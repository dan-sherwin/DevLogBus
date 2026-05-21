package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

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
	mux.HandleFunc("/api/health", withCORS(handleHTTPHealth))
	mux.HandleFunc("/api/records", withCORS(b.handleHTTPRecords))
	mux.HandleFunc("/api/stream", withCORS(b.handleHTTPStream))

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
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

func handleHTTPHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (b *broker) handleHTTPRecords(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, b.replay(subscribeFromRequest(r)))
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
