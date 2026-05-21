// Package protocol defines the wire records and filters used by DevLogBus.
package protocol

import (
	"fmt"
	"strings"
	"time"
)

// MessageType identifies an envelope payload kind.
type MessageType string

const (
	// MessageTypeLog carries a structured log record.
	MessageTypeLog MessageType = "log"
	// MessageTypeSubscribe carries a subscription request.
	MessageTypeSubscribe MessageType = "subscribe"
	// MessageTypeReplayComplete marks the end of the initial replay batch.
	MessageTypeReplayComplete MessageType = "replay_complete"
	// MessageTypeExpunge carries a request to delete replay-buffer records.
	MessageTypeExpunge MessageType = "expunge"
	// MessageTypeExpungeResult carries the count of deleted replay-buffer records.
	MessageTypeExpungeResult MessageType = "expunge_result"
	// MessageTypeError carries a broker-side error.
	MessageTypeError MessageType = "error"
)

// Envelope is one newline-delimited JSON message on the broker socket.
type Envelope struct {
	Type          MessageType    `json:"type"`
	Record        *Record        `json:"record,omitempty"`
	Subscribe     *Subscribe     `json:"subscribe,omitempty"`
	Expunge       *Expunge       `json:"expunge,omitempty"`
	ExpungeResult *ExpungeResult `json:"expungeResult,omitempty"`
	Error         string         `json:"error,omitempty"`
}

// Record is a structured log event.
type Record struct {
	ID      string         `json:"id,omitempty"`
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Source  string         `json:"source"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

// Subscribe describes a broker subscription filter.
type Subscribe struct {
	Sources         []string `json:"sources,omitempty"`
	MinLevel        string   `json:"minLevel,omitempty"`
	Replay          int      `json:"replay,omitempty"`
	ReplayPerSource int      `json:"replayPerSource,omitempty"`
}

// Expunge describes a broker replay-buffer delete request.
type Expunge struct {
	Source string `json:"source,omitempty"`
}

// ExpungeResult describes the outcome of a replay-buffer delete request.
type ExpungeResult struct {
	Expunged int `json:"expunged"`
}

// NormalizeLevel canonicalizes common log-level spellings.
func NormalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "dbg":
		return "DEBUG"
	case "warn", "warning":
		return "WARN"
	case "error", "err":
		return "ERROR"
	case "info", "":
		return "INFO"
	default:
		return strings.ToUpper(strings.TrimSpace(level))
	}
}

// LevelValue returns the slog-compatible numeric value for a level.
func LevelValue(level string) int {
	switch NormalizeLevel(level) {
	case "DEBUG":
		return -4
	case "INFO":
		return 0
	case "WARN":
		return 4
	case "ERROR":
		return 8
	default:
		return 0
	}
}

// Validate checks the required record fields.
func (r Record) Validate() error {
	if r.Source == "" {
		return fmt.Errorf("source is required")
	}
	if r.Message == "" {
		return fmt.Errorf("message is required")
	}
	return nil
}

// Matches reports whether record satisfies the subscription filter.
func (s Subscribe) Matches(record Record) bool {
	if s.MinLevel != "" && LevelValue(record.Level) < LevelValue(s.MinLevel) {
		return false
	}
	if len(s.Sources) == 0 {
		return true
	}
	for _, source := range s.Sources {
		if source == record.Source {
			return true
		}
	}
	return false
}
