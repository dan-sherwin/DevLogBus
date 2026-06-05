package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dan-sherwin/devlogbus/internal/buildinfo"
	"github.com/dan-sherwin/devlogbus/internal/devlogbusd/app/consts"
	devlogbusui "github.com/dan-sherwin/devlogbus/internal/devlogbusd/ui"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

type aboutResponse struct {
	API    apiAbout       `json:"api"`
	Broker brokerAbout    `json:"broker"`
	Build  buildinfo.Info `json:"build"`
}

type apiAbout struct {
	OK bool `json:"ok"`
}

type brokerAbout struct {
	Echo              bool   `json:"echo"`
	Endpoint          string `json:"endpoint"`
	HTTPListenAddress string `json:"httpListenAddress"`
	MaxRecords        int    `json:"maxRecords"`
	TCPListenAddress  string `json:"tcpListenAddress"`
}

func startHTTPServer(ctx context.Context, address string, b *broker) (func(), error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return func() {}, nil
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen http %q: %w", address, err)
	}

	mux := http.NewServeMux()
	auth := defaultAuthManager
	mux.HandleFunc("/api/auth/status", withCORSMethods(auth.handleHTTPAuthStatus, http.MethodGet))
	mux.HandleFunc("/api/auth/login", withCORSMethods(auth.handleHTTPAuthLogin, http.MethodPost))
	mux.HandleFunc("/api/auth/logout", withCORSMethods(auth.handleHTTPAuthLogout, http.MethodPost))
	mux.HandleFunc("/api/auth/settings", withCORSMethods(auth.withHTTPAuth(auth.handleHTTPAuthSettings), http.MethodPost))
	mux.HandleFunc("/api/auth/users", withCORSMethods(auth.withHTTPAuth(auth.handleHTTPAuthUsers), http.MethodGet, http.MethodPost))
	mux.HandleFunc("/api/auth/users/", withCORSMethods(auth.withHTTPAuth(auth.handleHTTPAuthUser), http.MethodDelete))
	mux.HandleFunc("/api/about", withCORS(auth.withHTTPAuth(handleHTTPAbout)))
	mux.HandleFunc("/api/health", withCORS(handleHTTPHealth))
	mux.HandleFunc("/api/records", withCORSMethods(auth.withHTTPAuth(b.handleHTTPRecords), http.MethodGet, http.MethodPost))
	mux.HandleFunc("/api/records/expunge", withCORSMethods(auth.withHTTPAuth(b.handleHTTPExpungeRecords), http.MethodDelete))
	mux.HandleFunc("/api/stream", withCORS(auth.withHTTPAuth(b.handleHTTPStream)))
	uiFS, err := devlogbusui.DistFS()
	if err != nil {
		return nil, fmt.Errorf("load ui assets: %w", err)
	}
	mux.HandleFunc("/", handleUI(uiFS))

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	done := make(chan struct{})

	go func() {
		defer close(done)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("devlogbusd http server failed", slog.String("error", err.Error()))
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	slog.Info("devlogbusd http listening", slog.String("address", listener.Addr().String()))
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = server.Close()
		}
	}, nil
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return withCORSMethods(next, http.MethodGet)
}

func withCORSMethods(next http.HandlerFunc, methods ...string) http.HandlerFunc {
	allowedMethods := map[string]struct{}{}
	for _, method := range methods {
		allowedMethods[method] = struct{}{}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(append(methods, http.MethodOptions), ", "))
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if _, ok := allowedMethods[r.Method]; !ok {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

func handleHTTPHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleHTTPAbout(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, aboutResponse{
		API: apiAbout{OK: true},
		Broker: brokerAbout{
			Echo:              Echo,
			Endpoint:          Endpoint,
			HTTPListenAddress: HTTPListenAddress,
			MaxRecords:        MaxRecords,
			TCPListenAddress:  TCPListenAddress,
		},
		Build: buildinfo.Read(consts.APPNAME, consts.Version, consts.Commit, consts.BuildDate),
	})
}

func handleUI(uiFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(uiFS))
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && uiFileExists(uiFS, name) {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFileFS(w, r, uiFS, "index.html")
	}
}

func uiFileExists(uiFS fs.FS, name string) bool {
	file, err := uiFS.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	return err == nil && !info.IsDir()
}

func (b *broker) handleHTTPRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		records, err := decodeHTTPPublishRecords(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, record := range records {
			b.publish(record)
		}
		writeJSON(w, http.StatusOK, map[string]int{"published": len(records)})
		return
	}
	writeJSON(w, http.StatusOK, b.replay(subscribeFromRequest(r)))
}

func decodeHTTPPublishRecords(w http.ResponseWriter, r *http.Request) ([]protocol.Record, error) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))

	var payload json.RawMessage
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode publish records: %w", err)
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return nil, fmt.Errorf("publish records payload is required")
	}

	var records []protocol.Record
	switch strings.TrimSpace(string(payload))[0] {
	case '[':
		if err := json.Unmarshal(payload, &records); err != nil {
			return nil, fmt.Errorf("decode records array: %w", err)
		}
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(payload, &object); err != nil {
			return nil, fmt.Errorf("decode record object: %w", err)
		}
		if rawRecords, ok := object["records"]; ok {
			if err := json.Unmarshal(rawRecords, &records); err != nil {
				return nil, fmt.Errorf("decode records: %w", err)
			}
			break
		}
		var record protocol.Record
		if err := json.Unmarshal(payload, &record); err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}
		records = []protocol.Record{record}
	default:
		return nil, fmt.Errorf("publish records payload must be an object or array")
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("at least one record is required")
	}
	if len(records) > 500 {
		return nil, fmt.Errorf("too many records: %d", len(records))
	}
	for i := range records {
		records[i].Level = protocol.NormalizeLevel(records[i].Level)
		if err := records[i].Validate(); err != nil {
			return nil, fmt.Errorf("record %d: %w", i, err)
		}
	}
	return records, nil
}

func (b *broker) handleHTTPExpungeRecords(w http.ResponseWriter, r *http.Request) {
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	writeJSON(w, http.StatusOK, map[string]int{"expunged": b.expunge(source)})
}

func (b *broker) handleHTTPStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	sub := subscribeFromRequest(r)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for _, record := range b.replay(sub) {
		if err := writeSSERecord(w, record); err != nil {
			return
		}
	}
	flusher.Flush()

	id, ch := b.addSubscriber(sub)
	defer b.removeSubscriber(id)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case record := <-ch:
			if err := writeSSERecord(w, record); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func subscribeFromRequest(r *http.Request) protocol.Subscribe {
	query := r.URL.Query()
	sub := protocol.Subscribe{
		MinLevel: firstNonEmpty(query.Get("level"), query.Get("minLevel")),
		Sources:  splitQueryValues(query["source"], query["sources"]),
	}
	if replay, err := strconv.Atoi(query.Get("replay")); err == nil && replay > 0 {
		sub.Replay = replay
	}
	if replayPerSource, err := strconv.Atoi(firstNonEmpty(query.Get("replayPerSource"), query.Get("replay-per-source"), query.Get("replay_per_source"))); err == nil && replayPerSource > 0 {
		sub.ReplayPerSource = replayPerSource
	}
	return sub
}

func splitQueryValues(values ...[]string) []string {
	var sources []string
	seen := map[string]struct{}{}
	for _, group := range values {
		for _, value := range group {
			for _, part := range strings.Split(value, ",") {
				source := strings.TrimSpace(part)
				if source == "" {
					continue
				}
				if _, ok := seen[source]; ok {
					continue
				}
				seen[source] = struct{}{}
				sources = append(sources, source)
			}
		}
	}
	return sources
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("http response encode failed", slog.String("error", err.Error()))
	}
}

func writeSSERecord(w http.ResponseWriter, record protocol.Record) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "event: record\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return nil
}
