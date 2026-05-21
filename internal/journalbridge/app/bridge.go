package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dan-sherwin/devlogbus/internal/journalbridge/app/consts"
	"github.com/dan-sherwin/devlogbus/internal/recordfmt"
	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

type (
	RunCommand struct {
		Endpoint           string          `name:"endpoint" help:"DevLogBus endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port"`
		Network            string          `name:"network" help:"Broker network (unix|tcp); ignored when endpoint is set"`
		Address            string          `name:"address" help:"TCP broker address; ignored when endpoint is set"`
		SocketPath         string          `name:"socket" help:"Unix socket path; ignored when endpoint is set"`
		Source             string          `name:"source" default:"journald" help:"Fallback source when the journal entry has no unit or identifier"`
		SourceField        repeatedStrings `name:"source-field" help:"Journal field to prefer for the DevLogBus source; repeatable"`
		Match              repeatedStrings `name:"match" help:"Journal match in FIELD=VALUE form; repeatable"`
		ExcludeUnit        repeatedStrings `name:"exclude-unit" help:"Journal _SYSTEMD_UNIT to skip; repeatable"`
		ExcludeIdentifier  repeatedStrings `name:"exclude-identifier" help:"SYSLOG_IDENTIFIER or _COMM to skip; repeatable"`
		Since              string          `name:"since" default:"now" help:"Start point: now, all, or a duration like 10m"`
		Tail               uint64          `name:"tail" default:"0" help:"Replay this many entries from the journal tail before following"`
		Once               bool            `name:"once" help:"Publish available replay entries and exit instead of following"`
		AllFields          bool            `name:"all-fields" help:"Include all journal fields as record attributes"`
		Echo               bool            `name:"echo" help:"Print forwarded records locally"`
		PublishTimeout     time.Duration   `name:"publish-timeout" default:"2s" help:"Per-record publish timeout"`
		ReconnectLogWindow time.Duration   `name:"reconnect-log-window" default:"30s" help:"Minimum interval for repeated publish failure logs"`
	}
	bridgeOptions struct {
		BrokerClient       *client.Client
		Source             string
		SourceFields       []string
		Matches            []string
		ExcludeUnits       map[string]struct{}
		ExcludeIdentifiers map[string]struct{}
		Since              string
		Tail               uint64
		Once               bool
		AllFields          bool
		Echo               bool
		PublishTimeout     time.Duration
		ReconnectLogWindow time.Duration
	}
	journalEntry struct {
		Fields             map[string]string
		Cursor             string
		RealtimeTimestamp  uint64
		MonotonicTimestamp uint64
	}
	brokerPublisher struct {
		client      *client.Client
		timeout     time.Duration
		logWindow   time.Duration
		publisher   *client.Publisher
		lastErr     string
		lastLogTime time.Time
	}
)

var defaultSourceFields = []string{"_SYSTEMD_UNIT", "SYSLOG_IDENTIFIER", "_COMM"}

func (c *RunCommand) Run() error {
	options, err := c.options()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("journal bridge starting",
		slog.String("brokerNetwork", options.BrokerClient.Network),
		slog.String("brokerAddress", options.BrokerClient.Address),
		slog.String("since", options.Since),
		slog.Uint64("tail", options.Tail),
		slog.Bool("once", options.Once),
	)
	return runBridge(ctx, options)
}

func (c *RunCommand) options() (bridgeOptions, error) {
	broker, err := brokerClient(c.Endpoint, c.Network, c.Address, c.SocketPath)
	if err != nil {
		return bridgeOptions{}, err
	}

	source := strings.TrimSpace(c.Source)
	if source == "" {
		source = "journald"
	}
	sourceFields := trimStrings(c.SourceField)
	if len(sourceFields) == 0 {
		sourceFields = defaultSourceFields
	}

	return bridgeOptions{
		BrokerClient:       broker,
		Source:             source,
		SourceFields:       sourceFields,
		Matches:            trimStrings(c.Match),
		ExcludeUnits:       stringSet(defaultExcludeUnits(), trimStrings(c.ExcludeUnit)),
		ExcludeIdentifiers: stringSet(defaultExcludeIdentifiers(), trimStrings(c.ExcludeIdentifier)),
		Since:              strings.TrimSpace(c.Since),
		Tail:               c.Tail,
		Once:               c.Once,
		AllFields:          c.AllFields,
		Echo:               c.Echo,
		PublishTimeout:     positiveDuration(c.PublishTimeout, 2*time.Second),
		ReconnectLogWindow: positiveDuration(c.ReconnectLogWindow, 30*time.Second),
	}, nil
}

func runBridge(ctx context.Context, options bridgeOptions) error {
	publisher := &brokerPublisher{
		client:    options.BrokerClient,
		timeout:   options.PublishTimeout,
		logWindow: options.ReconnectLogWindow,
	}
	defer publisher.close()

	return streamJournal(ctx, journalStreamOptions{
		Since:   options.Since,
		Tail:    options.Tail,
		Once:    options.Once,
		Matches: options.Matches,
	}, func(entry journalEntry) error {
		if shouldSkipEntry(entry, options) {
			return nil
		}
		record := journalRecord(entry, options)
		if options.Echo {
			fmt.Println(recordfmt.Format(record))
		}
		if err := publisher.publish(ctx, record); err != nil {
			publisher.logPublishError(err)
		}
		return nil
	})
}

func (p *brokerPublisher) publish(ctx context.Context, record protocol.Record) error {
	if p.publisher == nil {
		openCtx, cancel := context.WithTimeout(ctx, p.timeout)
		defer cancel()
		publisher, err := p.client.OpenPublisher(openCtx)
		if err != nil {
			return err
		}
		p.publisher = publisher
	}

	publishCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	if err := p.publisher.Publish(publishCtx, record); err != nil {
		p.close()
		return err
	}
	p.lastErr = ""
	return nil
}

func (p *brokerPublisher) close() {
	if p.publisher != nil {
		_ = p.publisher.Close()
		p.publisher = nil
	}
}

func (p *brokerPublisher) logPublishError(err error) {
	if err == nil {
		return
	}
	errText := err.Error()
	now := time.Now()
	if errText == p.lastErr && now.Sub(p.lastLogTime) < p.logWindow {
		return
	}
	p.lastErr = errText
	p.lastLogTime = now
	slog.Warn("failed to publish journal record", slog.String("error", errText))
}

func journalRecord(entry journalEntry, options bridgeOptions) protocol.Record {
	fields := entry.Fields
	return protocol.Record{
		Time:    journalTime(entry),
		Level:   journalLevel(fields["PRIORITY"]),
		Source:  journalSource(fields, options),
		Message: journalMessage(fields),
		Attrs:   journalAttrs(entry, options.AllFields),
	}
}

func journalTime(entry journalEntry) time.Time {
	if entry.RealtimeTimestamp == 0 {
		return time.Now()
	}
	return time.Unix(0, int64(entry.RealtimeTimestamp)*int64(time.Microsecond))
}

func journalLevel(priority string) string {
	value, err := strconv.Atoi(strings.TrimSpace(priority))
	if err != nil {
		return "INFO"
	}
	switch {
	case value <= 3:
		return "ERROR"
	case value == 4:
		return "WARN"
	case value >= 7:
		return "DEBUG"
	default:
		return "INFO"
	}
}

func journalSource(fields map[string]string, options bridgeOptions) string {
	for _, field := range options.SourceFields {
		if value := strings.TrimSpace(fields[field]); value != "" {
			return value
		}
	}
	return options.Source
}

func journalMessage(fields map[string]string) string {
	if message := strings.TrimSpace(fields["MESSAGE"]); message != "" {
		return message
	}
	return "journal entry"
}

func journalAttrs(entry journalEntry, allFields bool) map[string]any {
	attrs := map[string]any{
		"journal.cursor":         entry.Cursor,
		"journal.realtime_usec":  entry.RealtimeTimestamp,
		"journal.monotonic_usec": entry.MonotonicTimestamp,
	}
	if allFields {
		for key, value := range entry.Fields {
			if key == "MESSAGE" {
				continue
			}
			attrs[key] = value
		}
		return attrs
	}

	for _, field := range selectedJournalFields {
		if value := strings.TrimSpace(entry.Fields[field]); value != "" {
			attrs[field] = value
		}
	}
	return attrs
}

var selectedJournalFields = []string{
	"PRIORITY",
	"SYSLOG_IDENTIFIER",
	"_SYSTEMD_UNIT",
	"_SYSTEMD_USER_UNIT",
	"_SYSTEMD_SLICE",
	"_SYSTEMD_INVOCATION_ID",
	"_HOSTNAME",
	"_MACHINE_ID",
	"_BOOT_ID",
	"_PID",
	"_UID",
	"_GID",
	"_COMM",
	"_EXE",
	"_CMDLINE",
	"_TRANSPORT",
	"CODE_FILE",
	"CODE_LINE",
	"CODE_FUNC",
	"ERRNO",
}

func shouldSkipEntry(entry journalEntry, options bridgeOptions) bool {
	if value := strings.TrimSpace(entry.Fields["_SYSTEMD_UNIT"]); value != "" {
		if _, ok := options.ExcludeUnits[value]; ok {
			return true
		}
	}
	for _, field := range []string{"SYSLOG_IDENTIFIER", "_COMM"} {
		value := strings.TrimSpace(entry.Fields[field])
		if value == "" {
			continue
		}
		if _, ok := options.ExcludeIdentifiers[value]; ok {
			return true
		}
	}
	return false
}

func defaultExcludeUnits() []string {
	return []string{consts.APPNAME + ".service"}
}

func defaultExcludeIdentifiers() []string {
	return []string{consts.APPNAME, "devlogbus-journ"}
}

func trimStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func stringSet(groups ...[]string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, group := range groups {
		for _, value := range group {
			value = strings.TrimSpace(value)
			if value != "" {
				set[value] = struct{}{}
			}
		}
	}
	return set
}

func positiveDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
