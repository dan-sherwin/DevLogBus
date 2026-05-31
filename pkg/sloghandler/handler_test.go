package sloghandler

import (
	"encoding/json"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

func TestWithAttrsPreservesGroupBoundaries(t *testing.T) {
	handler := New(Options{Source: "test"})

	base := handler.WithAttrs([]slog.Attr{slog.String("outside", "one")})
	grouped := base.WithGroup("inside").WithAttrs([]slog.Attr{slog.String("value", "two")})

	h, ok := grouped.(*Handler)
	if !ok {
		t.Fatalf("expected *Handler, got %T", grouped)
	}
	if got := h.attrs["outside"]; got != "one" {
		t.Fatalf("outside attr = %v, want one", got)
	}
	if got := h.attrs["inside.value"]; got != "two" {
		t.Fatalf("inside attr = %v, want two", got)
	}
	if _, ok := h.attrs["inside.outside"]; ok {
		t.Fatalf("outside attr was incorrectly regrouped")
	}
}

func TestHandlerAppliesRedactorAndFilter(t *testing.T) {
	records := make(chan protocol.Record, 1)
	addr, stop := startRecordServer(t, records)
	defer stop()

	handler := New(Options{
		Source:         "test",
		Endpoint:       addr,
		PublishTimeout: time.Second,
		Filter: func(record protocol.Record) bool {
			return record.Message != "hidden"
		},
		Redactor: client.RedactAttrs("token"),
	})
	logger := slog.New(handler)

	logger.Info("hidden", slog.String("token", "drop-me"))
	logger.Info("visible", slog.String("token", "redact-me"))

	record := waitRecord(t, records)
	if record.Message != "visible" {
		t.Fatalf("message = %q, want visible", record.Message)
	}
	if record.Attrs["token"] != client.RedactedValue {
		t.Fatalf("token = %v, want redacted", record.Attrs["token"])
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
