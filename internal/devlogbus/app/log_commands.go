package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/dan-sherwin/devlogbus/internal/recordfmt"
	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

type (
	repeatedStrings []string
	EmitCommand     struct {
		Endpoint string          `name:"endpoint" default:"${endpoint}" help:"Broker endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port"`
		Source   string          `name:"source" default:"devlogbus" help:"Record source"`
		Level    string          `name:"level" default:"info" help:"Record level"`
		Message  string          `name:"message" default:"test log record" help:"Record message"`
		Attrs    repeatedStrings `name:"attr" help:"Attribute in key=value form; repeatable"`
	}
	TailCommand struct {
		Endpoint string          `name:"endpoint" default:"${endpoint}" help:"Broker endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port"`
		Level    string          `name:"level" default:"debug" help:"Minimum level"`
		Replay   int             `name:"replay" default:"100" help:"Number of matching records to replay"`
		Sources  repeatedStrings `name:"source" help:"Source to include; repeatable"`
	}
	ExpungeCommand struct {
		Endpoint string `name:"endpoint" default:"${endpoint}" help:"Broker endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port"`
		Source   string `name:"source" help:"Source to expunge"`
		All      bool   `name:"all" help:"Expunge all broker replay records"`
	}
	EndpointCommand struct {
		Endpoint string `name:"endpoint" default:"${endpoint}" help:"Broker endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port"`
	}
)

func (c *EmitCommand) Run() error {
	record := protocol.Record{
		Time:    time.Now(),
		Level:   c.Level,
		Source:  c.Source,
		Message: c.Message,
		Attrs:   parseAttrs(c.Attrs),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return newClient(c.Endpoint).Publish(ctx, record)
}

func (c *TailCommand) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sub, err := newClient(c.Endpoint).Subscribe(ctx, protocol.Subscribe{
		Sources:  c.Sources,
		MinLevel: c.Level,
		Replay:   c.Replay,
	})
	if err != nil {
		return err
	}
	defer func() { _ = sub.Close() }()

	for {
		select {
		case <-ctx.Done():
			return nil
		case record, ok := <-sub.Records:
			if !ok {
				return nil
			}
			fmt.Println(recordfmt.Format(record))
		case err := <-sub.Errors:
			if err != nil {
				slog.Error("tail failed", slog.String("error", err.Error()))
				return err
			}
		}
	}
}

func (c *ExpungeCommand) Run() error {
	source := strings.TrimSpace(c.Source)
	if c.All == (source != "") {
		return fmt.Errorf("set exactly one of --all or --source")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	expunged, err := newClient(c.Endpoint).Expunge(ctx, source)
	if err != nil {
		return err
	}
	if c.All {
		fmt.Printf("Expunged %d records\n", expunged)
		return nil
	}
	fmt.Printf("Expunged %d records for %s\n", expunged, source)
	return nil
}

func (c *EndpointCommand) Run() error {
	fmt.Println(newClient(c.Endpoint).Endpoint)
	return nil
}

func newClient(endpoint string) *client.Client {
	return client.New(endpoint)
}

func (r *repeatedStrings) String() string {
	return strings.Join(*r, ",")
}

func (r *repeatedStrings) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func parseAttrs(values []string) map[string]any {
	attrs := map[string]any{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok || key == "" {
			continue
		}
		attrs[key] = val
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}
