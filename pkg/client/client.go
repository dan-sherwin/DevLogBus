// Package client contains the Go client used to publish to and subscribe from a
// local DevLogBus broker.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"

	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

// DefaultSocketName is the filename used for the default Unix socket.
const DefaultSocketName = "devlogbus.sock"

// Client publishes records to and subscribes records from a DevLogBus broker.
type Client struct {
	SocketPath string
}

// Subscription is an active broker subscription.
type Subscription struct {
	Records <-chan protocol.Record
	Errors  <-chan error
	conn    net.Conn
}

// DefaultSocketPath returns the default local broker socket path.
func DefaultSocketPath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "devlogbus", DefaultSocketName)
	}
	return filepath.Join(os.TempDir(), "devlogbus", DefaultSocketName)
}

// New returns a client that uses socketPath, or the default socket path when
// socketPath is empty.
func New(socketPath string) *Client {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}
	return &Client{SocketPath: socketPath}
}

// Publish sends one structured record to the broker.
func (c *Client) Publish(ctx context.Context, record protocol.Record) error {
	if record.Time.IsZero() {
		return errors.New("record time is required")
	}
	record.Level = protocol.NormalizeLevel(record.Level)
	if err := record.Validate(); err != nil {
		return err
	}

	conn, err := dial(ctx, c.SocketPath)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	return json.NewEncoder(conn).Encode(protocol.Envelope{
		Type:   protocol.MessageTypeLog,
		Record: &record,
	})
}

// Subscribe opens a live subscription to broker records.
func (c *Client) Subscribe(ctx context.Context, sub protocol.Subscribe) (*Subscription, error) {
	conn, err := dial(ctx, c.SocketPath)
	if err != nil {
		return nil, err
	}

	if err := json.NewEncoder(conn).Encode(protocol.Envelope{
		Type:      protocol.MessageTypeSubscribe,
		Subscribe: &sub,
	}); err != nil {
		_ = conn.Close()
		return nil, err
	}

	records := make(chan protocol.Record, 128)
	errs := make(chan error, 1)
	go func() {
		defer close(records)
		defer close(errs)
		defer func() { _ = conn.Close() }()

		decoder := json.NewDecoder(conn)
		for {
			var env protocol.Envelope
			if err := decoder.Decode(&env); err != nil {
				errs <- err
				return
			}
			switch env.Type {
			case protocol.MessageTypeLog:
				if env.Record != nil {
					records <- *env.Record
				}
			case protocol.MessageTypeError:
				errs <- errors.New(env.Error)
				return
			}
		}
	}()

	return &Subscription{Records: records, Errors: errs, conn: conn}, nil
}

// Close closes the underlying broker connection.
func (s *Subscription) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func dial(ctx context.Context, socketPath string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, "unix", socketPath)
}
