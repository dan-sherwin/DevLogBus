package app

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
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

func TestHTTPRecordsPublishesSingleRecord(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	body := bytes.NewBufferString(`{"time":"2026-05-27T12:00:00Z","level":"warn","source":"chrome:tenant-ui","message":"POST /api/payment-methods -> 400","attrs":{"status":400}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/records", body)
	rr := httptest.NewRecorder()

	b.handleHTTPRecords(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var response map[string]int
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["published"] != 1 {
		t.Fatalf("published = %d, want 1", response["published"])
	}

	records := b.replay(protocol.Subscribe{})
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	if records[0].Source != "chrome:tenant-ui" || records[0].Level != "WARN" {
		t.Fatalf("record = %#v, want chrome:tenant-ui WARN", records[0])
	}
}

func TestHTTPRecordsPublishesBatch(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	body := bytes.NewBufferString(`{"records":[{"level":"info","source":"chrome:tenant-ui","message":"GET /api/session requested"},{"level":"error","source":"chrome:tenant-ui","message":"GET /api/session failed"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/records", body)
	rr := httptest.NewRecorder()

	b.handleHTTPRecords(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var response map[string]int
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["published"] != 2 {
		t.Fatalf("published = %d, want 2", response["published"])
	}

	records := b.replay(protocol.Subscribe{})
	if len(records) != 2 {
		t.Fatalf("records len = %d, want 2", len(records))
	}
	if records[0].Time.IsZero() || records[1].Time.IsZero() {
		t.Fatalf("published records should get daemon timestamps: %#v", records)
	}
}

func TestHTTPRecordsRejectsInvalidRecord(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/records", strings.NewReader(`{"level":"info","message":"missing source"}`))
	rr := httptest.NewRecorder()

	b.handleHTTPRecords(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if records := b.replay(protocol.Subscribe{}); len(records) != 0 {
		t.Fatalf("records len = %d, want 0", len(records))
	}
}

func TestHTTPExpungeRecordsRemovesServerRecords(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	now := time.Now()
	b.publish(protocol.Record{Time: now, Level: "info", Source: "ems", Message: "boot"})
	b.publish(protocol.Record{Time: now, Level: "warn", Source: "billing", Message: "catalog unavailable"})

	req := httptest.NewRequest(http.MethodDelete, "/api/records/expunge?source=billing", nil)
	rr := httptest.NewRecorder()

	b.handleHTTPExpungeRecords(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var response map[string]int
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["expunged"] != 1 {
		t.Fatalf("expunged = %d, want 1", response["expunged"])
	}
	records := b.replay(protocol.Subscribe{})
	if len(records) != 1 || records[0].Source != "ems" {
		t.Fatalf("records after expunge = %#v, want ems only", records)
	}
}

func TestHTTPAboutReportsBuildAndBrokerDetails(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/about", nil)
	rr := httptest.NewRecorder()

	handleHTTPAbout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var response aboutResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.API.OK {
		t.Fatalf("api ok = false, want true")
	}
	if response.Build.AppName != "devlogbusd" {
		t.Fatalf("app name = %q, want devlogbusd", response.Build.AppName)
	}
	if response.Broker.HTTPListenAddress == "" {
		t.Fatalf("http listen address should be reported")
	}
}

func TestSubscribeFromRequestParsesSources(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/stream?minLevel=info&source=ems,billing&source=tenant&replay=25&replayPerSource=50", nil)

	sub := subscribeFromRequest(req)

	if sub.MinLevel != "info" {
		t.Fatalf("MinLevel = %q, want info", sub.MinLevel)
	}
	if sub.Replay != 25 {
		t.Fatalf("Replay = %d, want 25", sub.Replay)
	}
	if sub.ReplayPerSource != 50 {
		t.Fatalf("ReplayPerSource = %d, want 50", sub.ReplayPerSource)
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

func TestUIServesIndexForRootAndFallback(t *testing.T) {
	uiFS := testingFS{
		"index.html":      "index",
		"assets/app.js":   "asset",
		"assets/app.css":  "style",
		"nested/page.txt": "nested",
	}
	handler := handleUI(uiFS)

	for _, target := range []string{"/", "/some/client/route"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rr := httptest.NewRecorder()
		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", target, rr.Code, http.StatusOK)
		}
		if rr.Body.String() != "index" {
			t.Fatalf("%s body = %q, want index", target, rr.Body.String())
		}
	}
}

func TestUIServesAssets(t *testing.T) {
	handler := handleUI(testingFS{"index.html": "index", "assets/app.js": "asset"})
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "asset" {
		t.Fatalf("body = %q, want asset", rr.Body.String())
	}
}

type testingFS map[string]string

func (t testingFS) Open(name string) (fs.File, error) {
	content, ok := t[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return testingFile{reader: strings.NewReader(content), name: name, size: int64(len(content))}, nil
}

type testingFile struct {
	reader *strings.Reader
	name   string
	size   int64
}

func (t testingFile) Stat() (fs.FileInfo, error) {
	return testingFileInfo{name: path.Base(t.name), size: t.size}, nil
}

func (t testingFile) Read(p []byte) (int, error) {
	return t.reader.Read(p)
}

func (t testingFile) Close() error {
	return nil
}

func (t testingFile) Seek(offset int64, whence int) (int64, error) {
	return t.reader.Seek(offset, whence)
}

type testingFileInfo struct {
	name string
	size int64
}

func (t testingFileInfo) Name() string       { return t.name }
func (t testingFileInfo) Size() int64        { return t.size }
func (t testingFileInfo) Mode() fs.FileMode  { return 0o444 }
func (t testingFileInfo) ModTime() time.Time { return time.Time{} }
func (t testingFileInfo) IsDir() bool        { return false }
func (t testingFileInfo) Sys() any           { return nil }

var _ fs.File = testingFile{}
var _ io.Seeker = testingFile{}
