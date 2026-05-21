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
	"sync"
	"syscall"
	"time"

	"github.com/dan-sherwin/devlogbus/internal/recordfmt"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

type (
	RunCommand struct {
		SocketPath  string `name:"socket" default:"${socket_path}" help:"Unix socket path"`
		HTTPAddress string `name:"http" default:"${http_listen_address}" help:"HTTP listen address for browser clients; empty disables HTTP"`
		MaxRecords  int    `name:"max-records" default:"${max_records}" help:"Records to retain in memory"`
		Echo        bool   `name:"echo" default:"${echo}" help:"Print received records to stdout"`
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

	SocketPath = c.SocketPath
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

	return run(ctx, c.SocketPath, b)
}

func run(ctx context.Context, socketPath string, b *broker) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return err
	}
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	slog.Info("devlogbusd listening", slog.String("socket", socketPath))
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Warn("accept failed", slog.String("error", err.Error()))
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
	if len(b.ring) > b.maxRecords {
		copy(b.ring, b.ring[len(b.ring)-b.maxRecords:])
		b.ring = b.ring[:b.maxRecords]
	}
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

func (b *broker) stream(conn net.Conn, sub protocol.Subscribe) {
	encoder := json.NewEncoder(conn)
	for _, record := range b.replay(sub) {
		if err := encoder.Encode(protocol.Envelope{Type: protocol.MessageTypeLog, Record: &record}); err != nil {
			return
		}
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
	if sub.Replay > 0 && len(records) > sub.Replay {
		records = records[len(records)-sub.Replay:]
	}
	return records
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
