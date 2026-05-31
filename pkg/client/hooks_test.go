package client

import (
	"testing"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

func TestDropSourcesFiltersListedSources(t *testing.T) {
	filter := DropSources("hidden", " noisy ")

	if filter(protocol.Record{Source: "hidden"}) {
		t.Fatal("expected hidden source to be dropped")
	}
	if filter(protocol.Record{Source: "noisy"}) {
		t.Fatal("expected noisy source to be dropped")
	}
	if !filter(protocol.Record{Source: "visible"}) {
		t.Fatal("expected visible source to be published")
	}
}

func TestRedactAttrsRedactsKeyAndPathMatches(t *testing.T) {
	record := protocol.Record{
		Time:    time.Now(),
		Level:   "INFO",
		Source:  "test",
		Message: "hello",
		Attrs: map[string]any{
			"token": "abc",
			"request": map[string]any{
				"authorization": "Bearer abc",
				"id":            "req-1",
			},
			"user": map[string]any{
				"email": "dan@example.com",
			},
		},
	}

	redacted := RedactAttrs("token", "request.authorization")(record)

	if redacted.Attrs["token"] != RedactedValue {
		t.Fatalf("token = %v, want redacted", redacted.Attrs["token"])
	}
	request, ok := redacted.Attrs["request"].(map[string]any)
	if !ok {
		t.Fatalf("request attrs = %T, want map", redacted.Attrs["request"])
	}
	if request["authorization"] != RedactedValue {
		t.Fatalf("request.authorization = %v, want redacted", request["authorization"])
	}
	if request["id"] != "req-1" {
		t.Fatalf("request.id = %v, want req-1", request["id"])
	}
	user, ok := redacted.Attrs["user"].(map[string]any)
	if !ok {
		t.Fatalf("user attrs = %T, want map", redacted.Attrs["user"])
	}
	if user["email"] != "dan@example.com" {
		t.Fatalf("user.email = %v, want original", user["email"])
	}
}

func TestPrepareRecordFiltersBeforeValidation(t *testing.T) {
	_, publish, err := prepareRecord(protocol.Record{Source: "drop"}, DropSources("drop"), nil)
	if err != nil {
		t.Fatalf("prepare record returned error: %v", err)
	}
	if publish {
		t.Fatal("expected record to be filtered")
	}
}

func TestPrepareRecordRedactsBeforePublish(t *testing.T) {
	record := protocol.Record{
		Time:    time.Now(),
		Level:   "warn",
		Source:  "test",
		Message: "hello",
		Attrs:   map[string]any{"apiKey": "secret"},
	}

	prepared, publish, err := prepareRecord(record, nil, RedactAttrs("apiKey"))
	if err != nil {
		t.Fatalf("prepare record returned error: %v", err)
	}
	if !publish {
		t.Fatal("expected record to be published")
	}
	if prepared.Level != "WARN" {
		t.Fatalf("level = %q, want WARN", prepared.Level)
	}
	if prepared.Attrs["apiKey"] != RedactedValue {
		t.Fatalf("apiKey = %v, want redacted", prepared.Attrs["apiKey"])
	}
}
