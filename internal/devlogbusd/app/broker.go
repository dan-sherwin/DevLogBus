package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dan-sherwin/devlogbus/internal/recordfmt"
	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

type (
	RunCommand struct {
		Endpoint    string `name:"endpoint" default:"${endpoint}" help:"Primary broker endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port"`
		TCPAddress  string `name:"tcp" default:"${tcp_listen_address}" help:"Additional TCP listen address for Go/CLI clients; empty disables the extra listener"`
		HTTPAddress string `name:"http" default:"${http_listen_address}" help:"HTTP listen address for browser clients; empty disables HTTP"`
		MaxRecords  int    `name:"max-records" default:"${max_records}" help:"Records to retain in memory"`
		Echo        bool   `name:"echo" default:"${echo}" help:"Print received records to stdout"`
	}
	brokerListener struct {
		net.Listener
		network string
		address string
		cleanup func()
	}
	broker struct {
		mu          sync.RWMutex
		ring        []protocol.Record
		maxRecords  int
		subscribers map[int]subscriber
		nextSubID   int
		nextRecord  int64
		echo        bool
	}
	subscriber struct {
		filter protocol.Subscribe
		ch     chan protocol.Record
	}
)

func (c *RunCommand) Run() error {
	if c.MaxRecords <= 0 {
		return fmt.Errorf("max-records must be greater than zero")
	}

	if err := setEndpoint(c.Endpoint); err != nil {
		return err
	}
	if err := setTCPListenAddress(c.TCPAddress); err != nil {
		return err
	}
	HTTPListenAddress = c.HTTPAddress
	MaxRecords = c.MaxRecords
	Echo = c.Echo

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cleanupSettingsRPC, err := startSettingsRPCServer(ctx)
	if err != nil {
		slog.Warn("settings rpc server disabled", slog.String("error", err.Error()))
	} else {
		defer cleanupSettingsRPC()
	}

	b := &broker{
		maxRecords:  c.MaxRecords,
		subscribers: map[int]subscriber{},
		echo:        c.Echo,
	}
	cleanupHTTP, err := startHTTPServer(ctx, c.HTTPAddress, b)
	if err != nil {
		return err
	}
	defer cleanupHTTP()

	return run(ctx, Endpoint, TCPListenAddress, b)
}

func run(ctx context.Context, endpoint string, tcpAddress string, b *broker) error {
	listeners, err := openBrokerListeners(endpoint, tcpAddress)
	if err != nil {
		return err
	}
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
			if listener.cleanup != nil {
				listener.cleanup()
			}
		}
	}()

	var wg sync.WaitGroup
	for _, listener := range listeners {
		listener := listener
		wg.Add(1)
		go func() {
			defer wg.Done()
			serveBrokerListener(ctx, listener, b)
		}()
	}

	<-ctx.Done()
	for _, listener := range listeners {
		_ = listener.Close()
	}
	wg.Wait()
	return nil
}

func openBrokerListeners(endpoint string, tcpAddress string) ([]brokerListener, error) {
	listeners := make([]brokerListener, 0, 2)

	listener, err := openBrokerListener(endpoint)
	if err != nil {
		return nil, err
	}
	listeners = append(listeners, listener)
	logBrokerListener(listener)

	tcpAddress = strings.TrimSpace(tcpAddress)
	if tcpAddress != "" {
		listener, err := openBrokerListener("tcp:" + tcpAddress)
		if err != nil {
			cleanupBrokerListeners(listeners)
			return nil, err
		}
		listeners = append(listeners, listener)
		logBrokerListener(listener)
	}

	return listeners, nil
}

func openBrokerListener(endpoint string) (brokerListener, error) {
	resolved, err := client.ParseEndpoint(endpoint)
	if err != nil {
		return brokerListener{}, err
	}

	switch resolved.Network {
	case client.NetworkUnix:
		socketPath := resolved.SocketPath
		if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
			return brokerListener{}, err
		}
		if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return brokerListener{}, err
		}
		listener, err := net.Listen(client.NetworkUnix, socketPath)
		if err != nil {
			return brokerListener{}, err
		}
		return brokerListener{
			Listener: listener,
			network:  client.NetworkUnix,
			address:  socketPath,
			cleanup:  func() { _ = os.Remove(socketPath) },
		}, nil
	case client.NetworkTCP:
		listener, err := net.Listen(client.NetworkTCP, resolved.Address)
		if err != nil {
			return brokerListener{}, err
		}
		return brokerListener{
			Listener: listener,
			network:  client.NetworkTCP,
			address:  listener.Addr().String(),
		}, nil
	default:
		return brokerListener{}, fmt.Errorf("unsupported broker endpoint network %q", resolved.Network)
	}
}

func cleanupBrokerListeners(listeners []brokerListener) {
	for _, listener := range listeners {
		_ = listener.Close()
		if listener.cleanup != nil {
			listener.cleanup()
		}
	}
}

func logBrokerListener(listener brokerListener) {
	slog.Info(
		"devlogbusd listening",
		slog.String("network", listener.network),
		slog.String("address", listener.address),
	)
}

func serveBrokerListener(ctx context.Context, listener brokerListener, b *broker) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn(
				"accept failed",
				slog.String("network", listener.network),
				slog.String("address", listener.address),
				slog.String("error", err.Error()),
			)
			continue
		}
		go b.handleConn(conn)
	}
}

func (b *broker) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	decoder := json.NewDecoder(bufio.NewReader(conn))
	for {
		var env protocol.Envelope
		if err := decoder.Decode(&env); err != nil {
			if !errors.Is(err, io.EOF) {
				slog.Debug("connection decode failed", slog.String("error", err.Error()))
			}
			return
		}

		switch env.Type {
		case protocol.MessageTypeLog:
			if env.Record == nil {
				continue
			}
			b.publish(*env.Record)
		case protocol.MessageTypeSubscribe:
			sub := protocol.Subscribe{}
			if env.Subscribe != nil {
				sub = *env.Subscribe
			}
			b.stream(conn, sub)
			return
		case protocol.MessageTypeExpunge:
			source := ""
			if env.Expunge != nil {
				source = strings.TrimSpace(env.Expunge.Source)
			}
			_ = json.NewEncoder(conn).Encode(protocol.Envelope{
				Type: protocol.MessageTypeExpungeResult,
				ExpungeResult: &protocol.ExpungeResult{
					Expunged: b.expunge(source),
				},
			})
			return
		default:
			_ = json.NewEncoder(conn).Encode(protocol.Envelope{
				Type:  protocol.MessageTypeError,
				Error: "unknown envelope type",
			})
			return
		}
	}
}

func (b *broker) publish(record protocol.Record) {
	if record.Time.IsZero() {
		record.Time = time.Now()
	}
	record.Level = protocol.NormalizeLevel(record.Level)

	b.mu.Lock()
	b.nextRecord++
	if record.ID == "" {
		record.ID = fmt.Sprintf("%d", b.nextRecord)
	}
	b.ring = append(b.ring, record)
	b.pruneSourceLocked(record.Source)
	subscribers := make([]subscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		if sub.filter.Matches(record) {
			subscribers = append(subscribers, sub)
		}
	}
	b.mu.Unlock()

	if b.echo {
		fmt.Println(recordfmt.Format(record))
	}
	for _, sub := range subscribers {
		select {
		case sub.ch <- record:
		default:
		}
	}
}

func (b *broker) pruneSourceLocked(source string) {
	sourceRecords := 0
	for _, record := range b.ring {
		if record.Source == source {
			sourceRecords++
		}
	}
	drop := sourceRecords - b.maxRecords
	if drop <= 0 {
		return
	}

	write := 0
	for _, record := range b.ring {
		if record.Source == source && drop > 0 {
			drop--
			continue
		}
		b.ring[write] = record
		write++
	}
	for i := write; i < len(b.ring); i++ {
		b.ring[i] = protocol.Record{}
	}
	b.ring = b.ring[:write]
}

func (b *broker) expunge(source string) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	if source == "" {
		expunged := len(b.ring)
		for i := range b.ring {
			b.ring[i] = protocol.Record{}
		}
		b.ring = nil
		return expunged
	}

	expunged := 0
	write := 0
	for _, record := range b.ring {
		if record.Source == source {
			expunged++
			continue
		}
		b.ring[write] = record
		write++
	}
	for i := write; i < len(b.ring); i++ {
		b.ring[i] = protocol.Record{}
	}
	b.ring = b.ring[:write]
	return expunged
}

func (b *broker) stream(conn net.Conn, sub protocol.Subscribe) {
	encoder := json.NewEncoder(conn)
	for _, record := range b.replay(sub) {
		if err := encoder.Encode(protocol.Envelope{Type: protocol.MessageTypeLog, Record: &record}); err != nil {
			return
		}
	}
	if err := encoder.Encode(protocol.Envelope{Type: protocol.MessageTypeReplayComplete}); err != nil {
		return
	}

	id, ch := b.addSubscriber(sub)
	defer b.removeSubscriber(id)

	for record := range ch {
		if err := encoder.Encode(protocol.Envelope{Type: protocol.MessageTypeLog, Record: &record}); err != nil {
			return
		}
	}
}

func (b *broker) replay(sub protocol.Subscribe) []protocol.Record {
	b.mu.RLock()
	defer b.mu.RUnlock()

	records := make([]protocol.Record, 0, len(b.ring))
	for _, record := range b.ring {
		if sub.Matches(record) {
			records = append(records, record)
		}
	}
	if sub.ReplayPerSource > 0 {
		records = limitReplayPerSource(records, sub.ReplayPerSource)
	}
	if sub.Replay > 0 && len(records) > sub.Replay {
		records = records[len(records)-sub.Replay:]
	}
	return records
}

func limitReplayPerSource(records []protocol.Record, limit int) []protocol.Record {
	if limit <= 0 || len(records) == 0 {
		return records
	}
	counts := map[string]int{}
	include := make([]bool, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		source := records[i].Source
		if counts[source] >= limit {
			continue
		}
		counts[source]++
		include[i] = true
	}

	next := records[:0]
	for i, record := range records {
		if include[i] {
			next = append(next, record)
		}
	}
	return next
}

func (b *broker) addSubscriber(sub protocol.Subscribe) (int, chan protocol.Record) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextSubID++
	id := b.nextSubID
	ch := make(chan protocol.Record, 512)
	b.subscribers[id] = subscriber{filter: sub, ch: ch}
	return id, ch
}

func (b *broker) removeSubscriber(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.subscribers, id)
}
