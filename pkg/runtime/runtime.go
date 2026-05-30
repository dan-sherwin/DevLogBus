// Package runtime provides reconfigurable DevLogBus controls for Go applications.
package runtime

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/dan-sherwin/devlogbus/internal/slogattrs"
	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

const (
	defaultSource         = "unknown"
	defaultQueueSize      = 256
	defaultPublishTimeout = 250 * time.Millisecond
)

// Options configures a DevLogBus runtime.
type Options struct {
	Enabled        bool
	Endpoint       string
	Network        string
	Address        string
	SocketPath     string
	Source         string
	Level          slog.Leveler
	QueueSize      int
	PublishTimeout time.Duration
}

// Config is the mutable DevLogBus runtime configuration.
type Config struct {
	Enabled  bool
	Endpoint string
}

// Status reports the current runtime configuration and last publish error.
type Status struct {
	Enabled    bool
	Endpoint   string
	Source     string
	Generation uint64
	LastError  string
}

// Runtime owns the reconfigurable DevLogBus publisher used by its slog handler.
type Runtime struct {
	mu             sync.RWMutex
	enabled        bool
	endpoint       string
	network        string
	address        string
	socketPath     string
	source         string
	level          slog.Leveler
	publishTimeout time.Duration
	generation     uint64
	lastError      string

	queue     chan protocol.Record
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

type snapshot struct {
	enabled        bool
	endpoint       string
	network        string
	address        string
	socketPath     string
	source         string
	publishTimeout time.Duration
	generation     uint64
}

// New creates a reconfigurable DevLogBus runtime.
func New(options Options) *Runtime {
	if options.Source == "" {
		options.Source = defaultSource
	}
	if options.Level == nil {
		options.Level = slog.LevelDebug
	}
	if options.QueueSize <= 0 {
		options.QueueSize = defaultQueueSize
	}
	if options.PublishTimeout <= 0 {
		options.PublishTimeout = defaultPublishTimeout
	}

	r := &Runtime{
		enabled:        options.Enabled,
		endpoint:       strings.TrimSpace(options.Endpoint),
		network:        strings.TrimSpace(options.Network),
		address:        strings.TrimSpace(options.Address),
		socketPath:     strings.TrimSpace(options.SocketPath),
		source:         options.Source,
		level:          options.Level,
		publishTimeout: options.PublishTimeout,
		queue:          make(chan protocol.Record, options.QueueSize),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}
	go r.run()

	return r
}

// Handler returns a slog handler backed by this runtime.
func (r *Runtime) Handler() slog.Handler {
	return &Handler{runtime: r, attrs: map[string]any{}}
}

// Status returns the current runtime state.
func (r *Runtime) Status() Status {
	cfg := r.snapshot()

	r.mu.RLock()
	lastError := r.lastError
	r.mu.RUnlock()

	return Status{
		Enabled:    cfg.enabled,
		Endpoint:   cfg.client().Endpoint,
		Source:     cfg.source,
		Generation: cfg.generation,
		LastError:  lastError,
	}
}

// Configure applies the mutable DevLogBus runtime configuration.
func (r *Runtime) Configure(config Config) error {
	if err := validateEndpoint(config.Endpoint); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = config.Enabled
	r.endpoint = strings.TrimSpace(config.Endpoint)
	r.generation++
	return nil
}

// Enable turns DevLogBus publishing on.
func (r *Runtime) Enable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.enabled {
		return
	}
	r.enabled = true
	r.generation++
}

// Disable turns DevLogBus publishing off and closes the current publisher.
func (r *Runtime) Disable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.enabled {
		return
	}
	r.enabled = false
	r.generation++
}

// SetEndpoint changes the DevLogBus broker endpoint. An empty endpoint resets
// the runtime to its configured transport fields or the default local socket.
func (r *Runtime) SetEndpoint(endpoint string) error {
	if err := validateEndpoint(endpoint); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	next := strings.TrimSpace(endpoint)
	if r.endpoint == next {
		return nil
	}
	r.endpoint = next
	r.generation++
	return nil
}

// Close stops the runtime publisher.
func (r *Runtime) Close() error {
	r.closeOnce.Do(func() {
		close(r.stop)
		<-r.done
	})
	return nil
}

func (r *Runtime) snapshot() snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return snapshot{
		enabled:        r.enabled,
		endpoint:       r.endpoint,
		network:        r.network,
		address:        r.address,
		socketPath:     r.socketPath,
		source:         r.source,
		publishTimeout: r.publishTimeout,
		generation:     r.generation,
	}
}

func (r *Runtime) setLastError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		r.lastError = ""
		return
	}
	r.lastError = err.Error()
}

func (r *Runtime) run() {
	var publisher *client.Publisher
	var publisherGeneration uint64
	defer close(r.done)
	defer func() {
		if publisher != nil {
			_ = publisher.Close()
		}
	}()

	for {
		select {
		case <-r.stop:
			return
		case record := <-r.queue:
			cfg := r.snapshot()
			if !cfg.enabled {
				if publisher != nil {
					_ = publisher.Close()
					publisher = nil
				}
				continue
			}
			if publisher != nil && publisherGeneration != cfg.generation {
				_ = publisher.Close()
				publisher = nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.publishTimeout)
			if publisher == nil {
				next, err := cfg.client().OpenPublisher(ctx)
				if err != nil {
					r.setLastError(err)
					cancel()
					continue
				}
				publisher = next
				publisherGeneration = cfg.generation
			}
			if err := publisher.Publish(ctx, record); err != nil {
				r.setLastError(err)
				_ = publisher.Close()
				publisher = nil
			} else {
				r.setLastError(nil)
			}
			cancel()
		}
	}
}

func (cfg snapshot) client() *client.Client {
	return client.NewWithOptions(client.Options{
		Endpoint:   cfg.endpoint,
		Network:    cfg.network,
		Address:    cfg.address,
		SocketPath: cfg.socketPath,
	})
}

func validateEndpoint(endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}
	_, err := client.ParseEndpoint(endpoint)
	return err
}

// Handler implements slog.Handler for a Runtime.
type Handler struct {
	runtime *Runtime
	attrs   map[string]any
	groups  []string
}

// Enabled reports whether level is enabled.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	if h == nil || h.runtime == nil {
		return false
	}

	h.runtime.mu.RLock()
	defer h.runtime.mu.RUnlock()
	return h.runtime.enabled && level >= h.runtime.level.Level()
}

// Handle queues a record for publishing.
func (h *Handler) Handle(_ context.Context, record slog.Record) error {
	if h == nil || h.runtime == nil {
		return nil
	}

	cfg := h.runtime.snapshot()
	if !cfg.enabled {
		return nil
	}

	out := protocol.Record{
		Time:    record.Time,
		Level:   record.Level.String(),
		Source:  cfg.source,
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
	case <-h.runtime.stop:
	case h.runtime.queue <- out:
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
