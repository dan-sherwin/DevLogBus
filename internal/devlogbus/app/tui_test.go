package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
		help   bool
	}{
		{name: "merged", mode: tuiViewMerged},
		{name: "source tiled", mode: tuiViewSource, layout: tuiLayoutTiled},
		{name: "source vertical", mode: tuiViewSource, layout: tuiLayoutVertical},
		{name: "source horizontal", mode: tuiViewSource, layout: tuiLayoutHorizontal},
		{name: "help", mode: tuiViewSource, layout: tuiLayoutTiled, help: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.viewMode = tc.mode
			if tc.layout != "" {
				m.sourceLayout = tc.layout
			}
			m.helpVisible = tc.help

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

func TestTUIHelpScreenRendersBeforeReplayAndCloses(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.width = 100
	m.height = 28

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Fatalf("help key returned command, want nil")
	}
	updated := next.(tuiModel)
	if !updated.helpVisible {
		t.Fatalf("helpVisible = false, want true")
	}

	view := updated.View()
	if !strings.Contains(view, "DevLogBus TUI Help") {
		t.Fatalf("help view = %q, want help title", view)
	}
	if !strings.Contains(view, "Source Groups") {
		t.Fatalf("help view = %q, want source groups section", view)
	}
	if strings.Contains(view, "Loading replay records") {
		t.Fatalf("help view should replace loading screen: %q", view)
	}

	next, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("close help returned command, want nil")
	}
	updated = next.(tuiModel)
	if updated.helpVisible {
		t.Fatalf("helpVisible after esc = true, want false")
	}
}

func TestTUIHelpKeepsHForNavigationWhenClosed(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.width = 100
	m.height = 24
	m.viewMode = tuiViewSource
	m.initialReplayLoaded = true
	m.handleRecord(protocol.Record{
		ID:      "a",
		Time:    time.Unix(0, 0),
		Level:   "INFO",
		Source:  "a",
		Message: "a",
	})
	m.handleRecord(protocol.Record{
		ID:      "b",
		Time:    time.Unix(0, int64(time.Millisecond)),
		Level:   "INFO",
		Source:  "b",
		Message: "b",
	})
	m.sourceCursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	updated := next.(tuiModel)
	if updated.sourceCursor != 0 {
		t.Fatalf("sourceCursor after h = %d, want 0", updated.sourceCursor)
	}

	updated.helpVisible = true
	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	updated = next.(tuiModel)
	if updated.helpVisible {
		t.Fatalf("helpVisible after h = true, want false")
	}
	if updated.sourceCursor != 0 {
		t.Fatalf("sourceCursor after closing help = %d, want unchanged 0", updated.sourceCursor)
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

func TestTUISourceGroupsDrillIntoChildSources(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.width = 120
	m.height = 30
	m.viewMode = tuiViewSource
	m.initialReplayLoaded = true

	for _, record := range []protocol.Record{
		{
			ID:      "client",
			Time:    time.Unix(0, 0),
			Level:   "INFO",
			Source:  "chrome:localhost:3010",
			Message: "client requested notifications",
			Attrs: map[string]any{
				"sourceGroup": "chrome:localhost:3010",
				"tabTitle":    "Spacelink Cloud Portal",
				"tabURL":      "http://localhost:3010/notifications",
			},
		},
		{
			ID:      "network",
			Time:    time.Unix(0, int64(time.Millisecond)),
			Level:   "WARN",
			Source:  "chrome:api.localhost:7423",
			Message: "GET /api/notifications returned 500",
			Attrs: map[string]any{
				"sourceGroup": "chrome:localhost:3010",
			},
		},
		{
			ID:      "server",
			Time:    time.Unix(0, int64(2*time.Millisecond)),
			Level:   "INFO",
			Source:  "ticket_management_svc",
			Message: "server handled request",
		},
	} {
		m.handleRecord(record)
	}

	scopes := m.sourceScopes()
	if len(scopes) != 2 {
		t.Fatalf("top-level scopes = %d, want 2", len(scopes))
	}
	if scopes[0].Kind != tuiScopeGroup || scopes[0].Key != tuiGroupKey("chrome:localhost:3010") {
		t.Fatalf("first scope = %#v, want chrome group", scopes[0])
	}
	if got := len(scopes[0].ChildSources); got != 2 {
		t.Fatalf("chrome child source count = %d, want 2", got)
	}
	if scopes[0].Label != "chrome:Spacelink Cloud Portal (localhost:3010)" {
		t.Fatalf("chrome label = %q", scopes[0].Label)
	}

	panes := m.sourcePanes()
	if len(panes) != 2 || panes[0].Kind != tuiScopeGroup || panes[0].Total != 2 {
		t.Fatalf("top-level panes = %#v, want grouped chrome pane plus server pane", panes)
	}

	m.drillFocusedGroup()
	if m.focusedGroup != "chrome:localhost:3010" {
		t.Fatalf("focusedGroup = %q, want chrome group", m.focusedGroup)
	}
	childScopes := m.sourceScopes()
	if len(childScopes) != 2 {
		t.Fatalf("child scopes = %d, want 2", len(childScopes))
	}
	for _, scope := range childScopes {
		if scope.Kind != tuiScopeSource {
			t.Fatalf("child scope kind = %q, want source", scope.Kind)
		}
	}
	if childScopes[0].Key != tuiSourceKey("chrome:api.localhost:7423") {
		t.Fatalf("first child scope = %#v", childScopes[0])
	}

	m.leaveFocusedGroup()
	if m.focusedGroup != "" {
		t.Fatalf("focusedGroup after leave = %q, want empty", m.focusedGroup)
	}
	if got := m.focusedSource(); got != tuiGroupKey("chrome:localhost:3010") {
		t.Fatalf("focused source after leave = %q, want chrome group", got)
	}
}

func TestTUIGroupExclusionFiltersMergedRecords(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.handleRecord(protocol.Record{
		ID:      "client",
		Time:    time.Unix(0, 0),
		Level:   "INFO",
		Source:  "chrome:localhost:3010",
		Message: "client requested notifications",
		Attrs: map[string]any{
			"sourceGroup": "chrome:localhost:3010",
		},
	})
	m.handleRecord(protocol.Record{
		ID:      "server",
		Time:    time.Unix(0, int64(time.Millisecond)),
		Level:   "INFO",
		Source:  "ticket_management_svc",
		Message: "server handled request",
	})

	m.excluded[tuiGroupKey("chrome:localhost:3010")] = true
	records := m.visibleMergedRecords()
	if len(records) != 1 || records[0].Source != "ticket_management_svc" {
		t.Fatalf("visible records = %#v, want only server record", records)
	}
}

func TestTUIGroupExpungeQueuesChildSources(t *testing.T) {
	m := newTUIModel("127.0.0.1:7422", 100, nil, nil)
	m.viewMode = tuiViewSource
	m.handleRecord(protocol.Record{
		ID:      "client",
		Time:    time.Unix(0, 0),
		Level:   "INFO",
		Source:  "chrome:localhost:3010",
		Message: "client requested notifications",
		Attrs: map[string]any{
			"sourceGroup": "chrome:localhost:3010",
		},
	})
	m.handleRecord(protocol.Record{
		ID:      "network",
		Time:    time.Unix(0, int64(time.Millisecond)),
		Level:   "WARN",
		Source:  "chrome:api.localhost:7423",
		Message: "GET /api/notifications returned 500",
		Attrs: map[string]any{
			"sourceGroup": "chrome:localhost:3010",
		},
	})

	m.queueExpunge()
	if m.pendingExpunge == nil {
		t.Fatalf("pendingExpunge = nil, want grouped target")
	}
	got := strings.Join(m.pendingExpunge.Sources, ",")
	want := "chrome:api.localhost:7423,chrome:localhost:3010"
	if got != want {
		t.Fatalf("expunge sources = %q, want %q", got, want)
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
