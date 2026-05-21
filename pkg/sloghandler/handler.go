// Package sloghandler provides a non-blocking slog handler that publishes
// records to a local DevLogBus broker.
package sloghandler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

// Options configures a DevLogBus slog handler.
type Options struct {
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
		source:         options.Source,
		client:         client.New(options.SocketPath),
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
		Attrs:   copyAttrs(h.attrs),
	}

	record.Attrs(func(attr slog.Attr) bool {
		addAttr(out.Attrs, h.groups, attr)
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
	clone.attrs = copyAttrs(h.attrs)
	for _, attr := range attrs {
		addAttr(clone.attrs, h.groups, attr)
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
	for record := range s.queue {
		ctx, cancel := context.WithTimeout(context.Background(), s.publishTimeout)
		_ = s.client.Publish(ctx, record)
		cancel()
	}
}

func copyAttrs(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func addAttr(attrs map[string]any, groups []string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := groups
		if attr.Key != "" {
			nextGroups = append(append([]string{}, groups...), attr.Key)
		}
		for _, groupAttr := range attr.Value.Group() {
			addAttr(attrs, nextGroups, groupAttr)
		}
		return
	}
	if attr.Key == "" {
		return
	}

	key := attr.Key
	for i := len(groups) - 1; i >= 0; i-- {
		key = groups[i] + "." + key
	}
	attrs[key] = valueAny(attr.Value)
}

func valueAny(value slog.Value) any {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindUint64:
		return value.Uint64()
	default:
		return fmt.Sprint(value.Any())
	}
}
