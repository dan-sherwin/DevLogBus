package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

type broker struct {
	mu          sync.RWMutex
	ring        []protocol.Record
	maxRecords  int
	subscribers map[int]subscriber
	nextSubID   int
	nextRecord  int64
	echo        bool
}

type subscriber struct {
	filter protocol.Subscribe
	ch     chan protocol.Record
}

func main() {
	socketPath := flag.String("socket", client.DefaultSocketPath(), "Unix socket path")
	maxRecords := flag.Int("max-records", 5000, "records to retain in memory")
	echo := flag.Bool("echo", true, "print received records to stdout")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	b := &broker{
		maxRecords:  *maxRecords,
		subscribers: map[int]subscriber{},
		echo:        *echo,
	}
	if err := run(ctx, *socketPath, b); err != nil {
		slog.Error("devlogbusd stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
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
	defer listener.Close()
	defer os.Remove(socketPath)

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
	defer conn.Close()

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
		fmt.Println(formatRecord(record))
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

	if sub, ok := b.subscribers[id]; ok {
		close(sub.ch)
		delete(b.subscribers, id)
	}
}

func formatRecord(record protocol.Record) string {
	keys := make([]string, 0, len(record.Attrs))
	for key := range record.Attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var fields []string
	for _, key := range keys {
		fields = append(fields, fmt.Sprintf("%s=%v", key, record.Attrs[key]))
	}
	suffix := ""
	if len(fields) > 0 {
		suffix = " " + strings.Join(fields, " ")
	}
	return fmt.Sprintf("%s %-5s %-24s %s%s", record.Time.Format("15:04:05.000"), record.Level, record.Source, record.Message, suffix)
}
