// Package sloghandler provides a non-blocking slog handler that publishes
// records to a local DevLogBus broker.
package sloghandler

import (
	"context"
	"log/slog"
	"time"

	"github.com/dan-sherwin/devlogbus/internal/slogattrs"
	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

// Options configures a DevLogBus slog handler.
type Options struct {
	Endpoint       string
	Network        string
	Address        string
	SocketPath     string
	Source         string
	Level          slog.Leveler
	QueueSize      int
	PublishTimeout time.Duration
}

// Handler implements slog.Handler for DevLogBus publishing.
type Handler struct {
	sink   *sink
	level  slog.Leveler
	attrs  map[string]any
	groups []string
}

type sink struct {
	source         string
	client         *client.Client
	queue          chan protocol.Record
	publishTimeout time.Duration
}

// New creates a non-blocking slog handler backed by DevLogBus.
func New(options Options) slog.Handler {
	if options.Source == "" {
		options.Source = "unknown"
	}
	if options.Level == nil {
		options.Level = slog.LevelDebug
	}
	if options.QueueSize <= 0 {
		options.QueueSize = 256
	}
	if options.PublishTimeout <= 0 {
		options.PublishTimeout = 250 * time.Millisecond
	}

	s := &sink{
		source: options.Source,
		client: client.NewWithOptions(client.Options{
			Endpoint:   options.Endpoint,
			Network:    options.Network,
			Address:    options.Address,
			SocketPath: options.SocketPath,
		}),
		queue:          make(chan protocol.Record, options.QueueSize),
		publishTimeout: options.PublishTimeout,
	}
	go s.run()

	return &Handler{sink: s, level: options.Level, attrs: map[string]any{}}
}

// Enabled reports whether level is enabled.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle queues a record for publishing.
func (h *Handler) Handle(_ context.Context, record slog.Record) error {
	out := protocol.Record{
		Time:    record.Time,
		Level:   record.Level.String(),
		Source:  h.sink.source,
		Message: record.Message,
		Attrs:   slogattrs.Copy(h.attrs),
	}

	record.Attrs(func(attr slog.Attr) bool {
		slogattrs.Add(out.Attrs, h.groups, attr)
		return true
	})
	if len(out.Attrs) == 0 {
		out.Attrs = nil
	}

	select {
	case h.sink.queue <- out:
	default:
	}
	return nil
}

// WithAttrs returns a handler with additional attributes.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = slogattrs.Copy(h.attrs)
	for _, attr := range attrs {
		slogattrs.Add(clone.attrs, h.groups, attr)
	}
	return &clone
}

// WithGroup returns a handler with a nested group.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

func (s *sink) run() {
	var publisher *client.Publisher
	defer func() {
		if publisher != nil {
			_ = publisher.Close()
		}
	}()
	for record := range s.queue {
		ctx, cancel := context.WithTimeout(context.Background(), s.publishTimeout)
		if publisher == nil {
			next, err := s.client.OpenPublisher(ctx)
			if err != nil {
				cancel()
				continue
			}
			publisher = next
		}
		if err := publisher.Publish(ctx, record); err != nil {
			_ = publisher.Close()
			publisher = nil
		}
		cancel()
	}
}
