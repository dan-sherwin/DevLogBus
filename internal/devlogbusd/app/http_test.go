package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

func TestHTTPRecordsUsesReplayFilters(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	now := time.Now()
	b.publish(protocol.Record{Time: now, Level: "info", Source: "ems", Message: "boot"})
	b.publish(protocol.Record{Time: now, Level: "warn", Source: "billing", Message: "catalog unavailable"})
	b.publish(protocol.Record{Time: now, Level: "debug", Source: "ems", Message: "query finished"})

	req := httptest.NewRequest(http.MethodGet, "/api/records?level=warn&source=billing&replay=10", nil)
	rr := httptest.NewRecorder()

	b.handleHTTPRecords(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var records []protocol.Record
	if err := json.NewDecoder(rr.Body).Decode(&records); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	if records[0].Source != "billing" || records[0].Level != "WARN" {
		t.Fatalf("record = %#v, want billing WARN", records[0])
	}
}

func TestSubscribeFromRequestParsesSources(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/stream?minLevel=info&source=ems,billing&source=tenant&replay=25", nil)

	sub := subscribeFromRequest(req)

	if sub.MinLevel != "info" {
		t.Fatalf("MinLevel = %q, want info", sub.MinLevel)
	}
	if sub.Replay != 25 {
		t.Fatalf("Replay = %d, want 25", sub.Replay)
	}
	want := []string{"ems", "billing", "tenant"}
	if len(sub.Sources) != len(want) {
		t.Fatalf("Sources = %#v, want %#v", sub.Sources, want)
	}
	for i := range want {
		if sub.Sources[i] != want[i] {
			t.Fatalf("Sources = %#v, want %#v", sub.Sources, want)
		}
	}
}
