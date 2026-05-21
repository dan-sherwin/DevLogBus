//go:build linux

package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/coreos/go-systemd/v22/journal"
)

type journaldHandler struct {
	attrs  []slog.Attr
	groups []string
}

type teeHandler struct {
	handlers []slog.Handler
}

var journaldMinLevel slog.Level = slog.LevelInfo

func initLogger() {
	level := parseLevel(LoggingLevel)
	journaldMinLevel = level

	handlers := []slog.Handler{&journaldHandler{}}
	if cliConfig.Verbose {
		handlers = append(handlers, slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	}

	var handler slog.Handler
	if len(handlers) == 1 {
		handler = handlers[0]
	} else {
		handler = &teeHandler{handlers: handlers}
	}
	setDefaultLogger(slog.New(handler))
}

func (h *journaldHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= journaldMinLevel
}

func (h *journaldHandler) Handle(_ context.Context, record slog.Record) error {
	fields := map[string]string{}
	for _, attr := range h.attrs {
		addJournalAttr(fields, h.groups, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		addJournalAttr(fields, h.groups, attr)
		return true
	})
	return journal.Send(record.Message, journalPriority(record.Level), fields)
}

func (h *journaldHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *journaldHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

func (h *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *teeHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &teeHandler{handlers: handlers}
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &teeHandler{handlers: handlers}
}

func journalPriority(level slog.Level) journal.Priority {
	switch {
	case level <= slog.LevelDebug:
		return journal.PriDebug
	case level < slog.LevelWarn:
		return journal.PriInfo
	case level < slog.LevelError:
		return journal.PriWarning
	default:
		return journal.PriErr
	}
}

func addJournalAttr(fields map[string]string, groups []string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := groups
		if attr.Key != "" {
			nextGroups = append(append([]string{}, groups...), attr.Key)
		}
		for _, groupAttr := range attr.Value.Group() {
			addJournalAttr(fields, nextGroups, groupAttr)
		}
		return
	}
	if attr.Key == "" {
		return
	}
	key := attr.Key
	for i := len(groups) - 1; i >= 0; i-- {
		key = groups[i] + "_" + key
	}
	fields[strings.ToUpper(key)] = fmt.Sprint(attr.Value.Any())
}
