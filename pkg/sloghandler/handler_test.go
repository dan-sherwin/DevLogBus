package sloghandler

import (
	"log/slog"
	"testing"
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
