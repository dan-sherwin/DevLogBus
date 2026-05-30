package runtime

import (
	"encoding/json"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

func TestNewDefaultsToDisabled(t *testing.T) {
	r := New(Options{Source: "test"})
	defer func() { _ = r.Close() }()

	status := r.Status()
	if status.Enabled {
		t.Fatal("expected runtime to default to disabled")
	}
	if status.Endpoint != "/tmp/devlogbus/devlogbus.sock" {
		t.Fatalf("endpoint = %q, want default socket path", status.Endpoint)
	}
}

func TestHandlerPublishesWhenEnabled(t *testing.T) {
	records := make(chan protocol.Record, 1)
	addr, stop := startRecordServer(t, records)
	defer stop()

	r := New(Options{
		Enabled:        true,
		Source:         "runtime-test",
		Endpoint:       addr,
		PublishTimeout: time.Second,
	})
	defer func() { _ = r.Close() }()

	logger := slog.New(r.Handler().WithGroup("request").WithAttrs([]slog.Attr{slog.String("id", "abc")}))
	logger.Info("hello", slog.Int("count", 3))

	record := waitRecord(t, records)
	if record.Source != "runtime-test" {
		t.Fatalf("source = %q, want runtime-test", record.Source)
	}
	if record.Message != "hello" {
		t.Fatalf("message = %q, want hello", record.Message)
	}
	if record.Attrs["request.id"] != "abc" {
		t.Fatalf("request.id = %v, want abc", record.Attrs["request.id"])
	}
	if record.Attrs["request.count"] != float64(3) {
		t.Fatalf("request.count = %v, want 3", record.Attrs["request.count"])
	}
}

func TestSetEndpointReconnectsPublisher(t *testing.T) {
	firstRecords := make(chan protocol.Record, 1)
	firstAddr, firstStop := startRecordServer(t, firstRecords)
	defer firstStop()

	secondRecords := make(chan protocol.Record, 1)
	secondAddr, secondStop := startRecordServer(t, secondRecords)
	defer secondStop()

	r := New(Options{
		Enabled:        true,
		Source:         "runtime-test",
		Endpoint:       firstAddr,
		PublishTimeout: time.Second,
	})
	defer func() { _ = r.Close() }()

	logger := slog.New(r.Handler())
	logger.Info("first")
	waitRecord(t, firstRecords)

	if err := r.SetEndpoint(secondAddr); err != nil {
		t.Fatalf("set endpoint: %v", err)
	}
	logger.Info("second")

	record := waitRecord(t, secondRecords)
	if record.Message != "second" {
		t.Fatalf("message = %q, want second", record.Message)
	}
	if status := r.Status(); status.Endpoint != secondAddr {
		t.Fatalf("status endpoint = %q, want %q", status.Endpoint, secondAddr)
	}
}

func TestDisableDropsRecords(t *testing.T) {
	records := make(chan protocol.Record, 1)
	addr, stop := startRecordServer(t, records)
	defer stop()

	r := New(Options{
		Enabled:        true,
		Source:         "runtime-test",
		Endpoint:       addr,
		PublishTimeout: time.Second,
	})
	defer func() { _ = r.Close() }()

	logger := slog.New(r.Handler())
	r.Disable()
	logger.Info("hidden")

	select {
	case record := <-records:
		t.Fatalf("unexpected record: %+v", record)
	case <-time.After(100 * time.Millisecond):
	}
}

func startRecordServer(t *testing.T, records chan<- protocol.Record) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				decoder := json.NewDecoder(conn)
				for {
					var env protocol.Envelope
					if err := decoder.Decode(&env); err != nil {
						return
					}
					if env.Record != nil {
						records <- *env.Record
					}
				}
			}()
		}
	}()

	return listener.Addr().String(), func() {
		_ = listener.Close()
		<-done
	}
}

func waitRecord(t *testing.T, records <-chan protocol.Record) protocol.Record {
	t.Helper()

	select {
	case record := <-records:
		return record
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for record")
		return protocol.Record{}
	}
}
