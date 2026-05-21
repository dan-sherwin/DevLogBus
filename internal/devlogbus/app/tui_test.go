package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

func TestTUIViewDoesNotExceedTerminalHeight(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.width = 80
	m.height = 24
	m.initialReplayLoaded = true

	for i := 0; i < 80; i++ {
		m.handleRecord(protocol.Record{
			ID:      fmt.Sprintf("%d", i+1),
			Time:    time.Unix(0, int64(i)*int64(time.Millisecond)),
			Level:   []string{"DEBUG", "INFO", "WARN", "ERROR"}[i%4],
			Source:  fmt.Sprintf("svc_%02d", i%12),
			Message: "record with enough text to pressure the terminal renderer",
			Attrs: map[string]any{
				"seq": i,
			},
		})
	}

	cases := []struct {
		name   string
		mode   string
		layout string
	}{
		{name: "merged", mode: tuiViewMerged},
		{name: "source tiled", mode: tuiViewSource, layout: tuiLayoutTiled},
		{name: "source vertical", mode: tuiViewSource, layout: tuiLayoutVertical},
		{name: "source horizontal", mode: tuiViewSource, layout: tuiLayoutHorizontal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.viewMode = tc.mode
			if tc.layout != "" {
				m.sourceLayout = tc.layout
			}

			view := m.View()
			if got := lipgloss.Height(view); got > m.height {
				t.Fatalf("rendered height = %d, want <= %d", got, m.height)
			}
			for lineNumber, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > m.width {
					t.Fatalf("line %d width = %d, want <= %d", lineNumber+1, got, m.width)
				}
			}
		})
	}
}

func TestTUILoadingScreenHidesPanelsUntilReplayLoaded(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "Loading replay records") {
		t.Fatalf("loading view = %q, want loading message", view)
	}
	if strings.Contains(view, "Details") {
		t.Fatalf("loading view should not render details panel: %q", view)
	}
}

func TestTUIReplayLoadedMessageAppliesRecordsAsBatch(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	next, _ := m.Update(tuiReplayLoadedMsg{Records: []protocol.Record{
		{
			ID:      "1",
			Time:    time.Unix(0, 0),
			Level:   "INFO",
			Source:  "svc",
			Message: "loaded",
		},
	}})
	updated := next.(tuiModel)
	if !updated.initialReplayLoaded {
		t.Fatalf("initialReplayLoaded = false, want true")
	}
	if len(updated.records) != 1 || updated.records[0].Message != "loaded" {
		t.Fatalf("records = %#v, want loaded record", updated.records)
	}
}

func TestTUIRecordWindowScrollsAtPaneEdges(t *testing.T) {
	const total = 20
	const height = 5

	if got := tuiEndRecordWindowStart(total, height); got != 15 {
		t.Fatalf("end start = %d, want 15", got)
	}
	if got := tuiKeepCursorVisible(total, 4, 0, height); got != 0 {
		t.Fatalf("cursor at visible bottom start = %d, want 0", got)
	}
	if got := tuiKeepCursorVisible(total, 5, 0, height); got != 1 {
		t.Fatalf("cursor below visible bottom start = %d, want 1", got)
	}
	if got := tuiKeepCursorVisible(total, 18, 15, height); got != 15 {
		t.Fatalf("one-up-from-end start = %d, want 15", got)
	}
	if got := tuiKeepCursorVisible(total, 14, 15, height); got != 14 {
		t.Fatalf("cursor above visible top start = %d, want 14", got)
	}
}

func TestTUIPanelRecordRowsLeavesRoomForTitle(t *testing.T) {
	if got := tuiPanelRecordRows(10); got != 7 {
		t.Fatalf("record rows = %d, want 7", got)
	}
	if got := tuiPanelRecordRows(2); got != 1 {
		t.Fatalf("small panel record rows = %d, want 1", got)
	}
}

func TestTUICursorMovesToPaneEdgesBeforeScrolling(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.width = 80
	m.height = 24

	for i := 0; i < 30; i++ {
		m.handleRecord(protocol.Record{
			ID:      fmt.Sprintf("%d", i+1),
			Time:    time.Unix(0, int64(i)*int64(time.Millisecond)),
			Level:   "INFO",
			Source:  "svc",
			Message: fmt.Sprintf("record %d", i+1),
		})
	}

	rows := m.mergedRecordRows()
	if rows < 3 {
		t.Fatalf("record rows = %d, want at least 3", rows)
	}

	m.moveRecordCursorToStart()
	for i := 1; i < rows; i++ {
		m.moveRecordCursor(1)
		if m.mergedStart != 0 {
			t.Fatalf("start after moving cursor to row %d = %d, want 0", i, m.mergedStart)
		}
	}
	m.moveRecordCursor(1)
	if m.mergedStart != 1 {
		t.Fatalf("start after moving past pane bottom = %d, want 1", m.mergedStart)
	}

	m.moveRecordCursorToEnd()
	wantEndStart := tuiEndRecordWindowStart(len(m.visibleMergedRecords()), rows)
	if m.mergedStart != wantEndStart {
		t.Fatalf("end start = %d, want %d", m.mergedStart, wantEndStart)
	}
	m.moveRecordCursor(-1)
	if m.mergedStart != wantEndStart {
		t.Fatalf("start after one-up from end = %d, want %d", m.mergedStart, wantEndStart)
	}
}

func TestTUIPgDownAndPgUpPageAtPaneEdges(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.width = 80
	m.height = 24

	for i := 0; i < 50; i++ {
		m.handleRecord(protocol.Record{
			ID:      fmt.Sprintf("%d", i+1),
			Time:    time.Unix(0, int64(i)*int64(time.Millisecond)),
			Level:   "INFO",
			Source:  "svc",
			Message: fmt.Sprintf("record %d", i+1),
		})
	}

	rows := m.mergedRecordRows()
	if rows < 3 {
		t.Fatalf("record rows = %d, want at least 3", rows)
	}

	m.moveRecordCursorToStart()
	m.pageRecordCursor(1)
	if m.mergedCursor != rows-1 || m.mergedStart != 0 {
		t.Fatalf("first pgdn cursor/start = %d/%d, want %d/0", m.mergedCursor, m.mergedStart, rows-1)
	}

	m.pageRecordCursor(1)
	if m.mergedCursor != rows*2-1 || m.mergedStart != rows {
		t.Fatalf("second pgdn cursor/start = %d/%d, want %d/%d", m.mergedCursor, m.mergedStart, rows*2-1, rows)
	}

	m.pageRecordCursor(-1)
	if m.mergedCursor != rows || m.mergedStart != rows {
		t.Fatalf("first pgup cursor/start = %d/%d, want %d/%d", m.mergedCursor, m.mergedStart, rows, rows)
	}

	m.pageRecordCursor(-1)
	if m.mergedCursor != 0 || m.mergedStart != 0 {
		t.Fatalf("second pgup cursor/start = %d/%d, want 0/0", m.mergedCursor, m.mergedStart)
	}
}
