// Package client contains the Go client used to publish to and subscribe from a
// local DevLogBus broker.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

const (
	// NetworkUnix publishes over a Unix domain socket.
	NetworkUnix = "unix"
	// NetworkTCP publishes over a TCP socket.
	NetworkTCP = "tcp"
	// DefaultSocketName is the filename used for the default Unix socket.
	DefaultSocketName = "devlogbus.sock"
	// DefaultSocketDir is the stable local directory used for the default Unix socket.
	DefaultSocketDir = "/tmp/devlogbus"
)

// Options configures a broker client.
type Options struct {
	Endpoint   string
	Network    string
	Address    string
	SocketPath string
}

// Client publishes records to and subscribes records from a DevLogBus broker.
type Client struct {
	Endpoint   string
	Network    string
	Address    string
	SocketPath string
}

// ResolvedEndpoint is a concrete broker transport endpoint.
type ResolvedEndpoint struct {
	Network    string
	Address    string
	SocketPath string
}

// Publisher is a persistent broker publisher.
type Publisher struct {
	conn    net.Conn
	encoder *json.Encoder
}

// Subscription is an active broker subscription.
type Subscription struct {
	Records    <-chan protocol.Record
	Errors     <-chan error
	ReplayDone <-chan struct{}
	conn       net.Conn
}

// DefaultSocketPath returns the default local broker socket path.
func DefaultSocketPath() string {
	return filepath.Join(DefaultSocketDir, DefaultSocketName)
}

// New returns a client that uses endpoint, or the default socket path when
// endpoint is empty. Endpoint may be a Unix socket path or host:port TCP address.
func New(endpoint string) *Client {
	return NewWithOptions(Options{Endpoint: endpoint})
}

// NewWithOptions returns a client configured for Unix or TCP transport.
func NewWithOptions(options Options) *Client {
	endpoint := strings.TrimSpace(options.Endpoint)
	if endpoint != "" {
		resolved, err := ParseEndpoint(endpoint)
		if err == nil {
			return &Client{
				Endpoint:   resolved.String(),
				Network:    resolved.Network,
				Address:    resolved.Address,
				SocketPath: resolved.SocketPath,
			}
		}
		return &Client{Endpoint: endpoint}
	}

	network := strings.TrimSpace(options.Network)
	address := strings.TrimSpace(options.Address)
	socketPath := strings.TrimSpace(options.SocketPath)
	if network == "" {
		if address != "" {
			network = NetworkTCP
		} else {
			network = NetworkUnix
		}
	}
	if network == NetworkUnix {
		if address != "" && socketPath == "" {
			socketPath = address
		}
		if socketPath == "" {
			socketPath = DefaultSocketPath()
		}
		address = socketPath
	}
	resolved := ResolvedEndpoint{Network: network, Address: address, SocketPath: socketPath}
	return &Client{
		Endpoint:   resolved.String(),
		Network:    network,
		Address:    address,
		SocketPath: socketPath,
	}
}

// ParseEndpoint resolves a user-facing endpoint string into a transport.
func ParseEndpoint(raw string) (ResolvedEndpoint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ResolvedEndpoint{}, fmt.Errorf("devlogbus endpoint cannot be empty")
	}

	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "unix://"):
		return parseUnixEndpoint(raw[len("unix://"):])
	case strings.HasPrefix(lower, "unix:"):
		return parseUnixEndpoint(raw[len("unix:"):])
	case strings.HasPrefix(lower, "tcp://"):
		parsed, err := url.Parse(raw)
		if err != nil {
			return ResolvedEndpoint{}, fmt.Errorf("parse tcp devlogbus endpoint: %w", err)
		}
		return parseTCPEndpoint(parsed.Host)
	case strings.HasPrefix(lower, "tcp:"):
		return parseTCPEndpoint(raw[len("tcp:"):])
	}

	if endpoint, err := parseTCPEndpoint(raw); err == nil {
		return endpoint, nil
	}
	return parseUnixEndpoint(raw)
}

// String returns the user-facing endpoint value for the resolved endpoint.
func (e ResolvedEndpoint) String() string {
	if e.Network == NetworkTCP {
		return e.Address
	}
	return e.SocketPath
}

// Publish sends one structured record to the broker.
func (c *Client) Publish(ctx context.Context, record protocol.Record) error {
	publisher, err := c.OpenPublisher(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = publisher.Close() }()
	return publisher.Publish(ctx, record)
}

// OpenPublisher opens a persistent publisher connection.
func (c *Client) OpenPublisher(ctx context.Context) (*Publisher, error) {
	conn, err := c.Dial(ctx)
	if err != nil {
		return nil, err
	}
	return &Publisher{conn: conn, encoder: json.NewEncoder(conn)}, nil
}

// Publish sends one structured record over the persistent publisher connection.
func (p *Publisher) Publish(ctx context.Context, record protocol.Record) error {
	if p == nil || p.conn == nil || p.encoder == nil {
		return errors.New("publisher is closed")
	}
	if err := validateRecord(&record); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = p.conn.SetWriteDeadline(deadline)
		defer func() { _ = p.conn.SetWriteDeadline(time.Time{}) }()
	}
	return p.encoder.Encode(protocol.Envelope{Type: protocol.MessageTypeLog, Record: &record})
}

// Close closes the persistent publisher connection.
func (p *Publisher) Close() error {
	if p == nil || p.conn == nil {
		return nil
	}
	return p.conn.Close()
}

// Subscribe opens a live subscription to broker records.
func (c *Client) Subscribe(ctx context.Context, sub protocol.Subscribe) (*Subscription, error) {
	conn, err := c.Dial(ctx)
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
	replayDone := make(chan struct{})
	go func() {
		replayDoneClosed := false
		closeReplayDone := func() {
			if !replayDoneClosed {
				close(replayDone)
				replayDoneClosed = true
			}
		}

		defer close(records)
		defer close(errs)
		defer closeReplayDone()
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
			case protocol.MessageTypeReplayComplete:
				closeReplayDone()
			case protocol.MessageTypeError:
				errs <- errors.New(env.Error)
				return
			}
		}
	}()

	return &Subscription{Records: records, Errors: errs, ReplayDone: replayDone, conn: conn}, nil
}

// Expunge removes replay-buffer records from the broker. When source is empty,
// every replay-buffer record is removed.
func (c *Client) Expunge(ctx context.Context, source string) (int, error) {
	conn, err := c.Dial(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
		defer func() { _ = conn.SetDeadline(time.Time{}) }()
	}

	if err := json.NewEncoder(conn).Encode(protocol.Envelope{
		Type:    protocol.MessageTypeExpunge,
		Expunge: &protocol.Expunge{Source: strings.TrimSpace(source)},
	}); err != nil {
		return 0, err
	}

	var env protocol.Envelope
	if err := json.NewDecoder(conn).Decode(&env); err != nil {
		return 0, err
	}
	switch env.Type {
	case protocol.MessageTypeExpungeResult:
		if env.ExpungeResult == nil {
			return 0, errors.New("broker returned empty expunge result")
		}
		return env.ExpungeResult.Expunged, nil
	case protocol.MessageTypeError:
		return 0, errors.New(env.Error)
	default:
		return 0, fmt.Errorf("unexpected broker response %q", env.Type)
	}
}

// Dial opens a raw connection to the configured broker endpoint.
func (c *Client) Dial(ctx context.Context) (net.Conn, error) {
	network, address, err := c.endpoint()
	if err != nil {
		return nil, err
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, address)
}

// Close closes the underlying broker connection.
func (s *Subscription) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (c *Client) endpoint() (string, string, error) {
	endpoint := strings.TrimSpace(c.Endpoint)
	if endpoint != "" && c.Network == "" && c.Address == "" && c.SocketPath == "" {
		resolved, err := ParseEndpoint(endpoint)
		if err != nil {
			return "", "", err
		}
		return resolved.Network, resolved.Address, nil
	}

	network := strings.TrimSpace(c.Network)
	address := strings.TrimSpace(c.Address)
	socketPath := strings.TrimSpace(c.SocketPath)
	if network == "" {
		if address != "" {
			network = NetworkTCP
		} else {
			network = NetworkUnix
		}
	}

	switch network {
	case NetworkUnix:
		if address == "" {
			address = socketPath
		}
		if address == "" {
			address = DefaultSocketPath()
		}
		return NetworkUnix, address, nil
	case NetworkTCP:
		if address == "" {
			return "", "", fmt.Errorf("tcp broker address is required")
		}
		return NetworkTCP, address, nil
	default:
		return "", "", fmt.Errorf("unsupported broker network %q", network)
	}
}

func validateRecord(record *protocol.Record) error {
	if record.Time.IsZero() {
		return errors.New("record time is required")
	}
	record.Level = protocol.NormalizeLevel(record.Level)
	return record.Validate()
}

func parseUnixEndpoint(socketPath string) (ResolvedEndpoint, error) {
	socketPath = strings.TrimSpace(socketPath)
	if socketPath == "" {
		return ResolvedEndpoint{}, fmt.Errorf("unix devlogbus endpoint requires a socket path")
	}
	socketPath = filepath.Clean(socketPath)
	return ResolvedEndpoint{
		Network:    NetworkUnix,
		Address:    socketPath,
		SocketPath: socketPath,
	}, nil
}

func parseTCPEndpoint(address string) (ResolvedEndpoint, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return ResolvedEndpoint{}, fmt.Errorf("tcp devlogbus endpoint requires host:port")
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return ResolvedEndpoint{}, fmt.Errorf("tcp devlogbus endpoint must be host:port: %w", err)
	}
	return ResolvedEndpoint{Network: NetworkTCP, Address: address}, nil
}
