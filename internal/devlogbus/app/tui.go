package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
	"github.com/mattn/go-runewidth"
)

const (
	tuiViewMerged = "merged"
	tuiViewSource = "source"

	tuiLayoutTiled      = "tiled"
	tuiLayoutVertical   = "vertical"
	tuiLayoutHorizontal = "horizontal"

	tuiDefaultReplayPerSource = 1000
	tuiMaxPerSource           = 1000
	tuiDefaultPaneWidth       = 52
	tuiMinPaneWidth           = 30
	tuiRightEdgeGuard         = 12

	tuiReplayFallbackDelay = 300 * time.Millisecond
)

var (
	tuiLevels      = []string{"DEBUG", "INFO", "WARN", "ERROR"}
	tuiLayoutOrder = []string{tuiLayoutTiled, tuiLayoutVertical, tuiLayoutHorizontal}

	tuiBaseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("235"))
	tuiMutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	tuiHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("24")).Padding(0, 1)
	tuiFooterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("236")).Padding(0, 1)
	tuiPanelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	tuiPanelActive    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	tuiSelectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("238"))
	tuiDebugStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	tuiInfoStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("120"))
	tuiWarnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("222"))
	tuiErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	tuiSourceStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("195")).Bold(true)
	tuiSourceOffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
)

type (
	TUICommand struct {
		Endpoint        string `name:"endpoint" default:"${endpoint}" help:"Broker endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port"`
		ReplayPerSource int    `name:"replay-per-source" default:"1000" help:"Number of replay records to request per source when the TUI connects"`
		Replay          int    `name:"replay" hidden:"" help:"Deprecated alias for --replay-per-source"`
	}

	tuiRecord struct {
		protocol.Record
		Key string
	}

	tuiModel struct {
		endpoint        string
		replayPerSource int
		events          <-chan tea.Msg
		cancel          context.CancelFunc

		width  int
		height int

		connection string
		status     string
		errText    string

		initialReplayLoaded bool
		records             []tuiRecord
		knownSources        map[string]struct{}
		excluded            map[string]bool

		viewMode     string
		sourceLayout string
		paneWidth    int

		search       string
		searchActive bool

		sourceCursor  int
		mergedCursor  int
		mergedStart   int
		sourceCursors map[string]int
		sourceStarts  map[string]int

		mergedLevels   map[string]bool
		perSourceLevel map[string]map[string]bool

		mergedPaused  bool
		sourcePaused  map[string]bool
		mergedBottom  bool
		sourceBottom  map[string]bool
		mergedDetails bool
		sourceDetails map[string]bool

		pendingExpunge *tuiExpungeTarget
	}

	tuiExpungeTarget struct {
		All    bool
		Source string
	}

	tuiStatusMsg struct {
		State string
		Text  string
	}

	tuiRecordMsg struct {
		Record protocol.Record
	}

	tuiReplayLoadedMsg struct {
		Records []protocol.Record
	}

	tuiStreamErrMsg struct {
		Err error
	}

	tuiStreamClosedMsg struct{}

	tuiExpungeResultMsg struct {
		Target   tuiExpungeTarget
		Expunged int
		Err      error
	}

	tuiPane struct {
		Source  string
		Records []tuiRecord
		Total   int
	}
)

func (c *TUICommand) Run() error {
	replayPerSource := c.ReplayPerSource
	if c.Replay > 0 {
		replayPerSource = c.Replay
	}
	if replayPerSource <= 0 {
		replayPerSource = tuiDefaultReplayPerSource
	}

	brokerClient := newClient(c.Endpoint)
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan tea.Msg, 256)
	go streamTUIRecords(ctx, brokerClient, replayPerSource, events)

	program := tea.NewProgram(newTUIModel(brokerClient.Endpoint, replayPerSource, events, cancel), tea.WithAltScreen())
	_, err := program.Run()
	cancel()
	return err
}

func newTUIModel(endpoint string, replayPerSource int, events <-chan tea.Msg, cancel context.CancelFunc) tuiModel {
	return tuiModel{
		endpoint:        endpoint,
		replayPerSource: replayPerSource,
		events:          events,
		cancel:          cancel,
		connection:      "connecting",
		status:          "connecting",
		knownSources:    map[string]struct{}{},
		excluded:        map[string]bool{},
		viewMode:        tuiViewMerged,
		sourceLayout:    tuiLayoutTiled,
		paneWidth:       tuiDefaultPaneWidth,
		sourceCursors:   map[string]int{},
		sourceStarts:    map[string]int{},
		mergedLevels:    defaultTUILevels(),
		perSourceLevel:  map[string]map[string]bool{},
		sourcePaused:    map[string]bool{},
		mergedBottom:    true,
		sourceBottom:    map[string]bool{},
		sourceDetails:   map[string]bool{},
	}
}

func (m tuiModel) Init() tea.Cmd {
	return waitForTUIEvent(m.events)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampCursors()
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tuiStatusMsg:
		m.connection = msg.State
		m.status = msg.Text
		if msg.State == "online" {
			m.errText = ""
		}
		return m, waitForTUIEvent(m.events)
	case tuiStreamErrMsg:
		if msg.Err != nil {
			m.errText = msg.Err.Error()
			m.connection = "reconnecting"
			m.status = "reconnecting"
		}
		return m, waitForTUIEvent(m.events)
	case tuiRecordMsg:
		m.initialReplayLoaded = true
		m.handleRecord(msg.Record)
		m.clampCursors()
		return m, waitForTUIEvent(m.events)
	case tuiReplayLoadedMsg:
		for _, record := range msg.Records {
			m.handleRecord(record)
		}
		m.initialReplayLoaded = true
		m.connection = "online"
		m.status = "online"
		m.errText = ""
		m.clampCursors()
		return m, waitForTUIEvent(m.events)
	case tuiStreamClosedMsg:
		m.connection = "offline"
		m.status = "stream closed"
		return m, nil
	case tuiExpungeResultMsg:
		m.pendingExpunge = nil
		if msg.Err != nil {
			m.errText = msg.Err.Error()
			m.status = "expunge failed"
			return m, nil
		}
		if msg.Target.All {
			m.clearAll()
			m.status = fmt.Sprintf("expunged %d records", msg.Expunged)
		} else {
			m.clearSource(msg.Target.Source, true)
			m.status = fmt.Sprintf("expunged %d records for %s", msg.Expunged, msg.Target.Source)
		}
		m.clampCursors()
		return m, nil
	}
	return m, nil
}

func (m tuiModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "Starting DevLogBus TUI..."
	}

	renderModel := m
	renderModel.width = m.renderWidth()

	header := renderModel.renderHeader()
	footer := renderModel.renderFooter()
	if !renderModel.initialReplayLoaded {
		bodyHeight := renderModel.height - lipgloss.Height(header) - lipgloss.Height(footer)
		if bodyHeight < 0 {
			bodyHeight = 0
		}
		body := forceHeight(renderModel.renderLoadingBody(renderModel.width, bodyHeight), bodyHeight)
		frame := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
		return forceHeight(frame, renderModel.height)
	}

	sourceBar := renderModel.renderSourceBar()
	bodyHeight := renderModel.height - lipgloss.Height(header) - lipgloss.Height(sourceBar) - lipgloss.Height(footer)
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	body := forceHeight(renderModel.renderBody(renderModel.width, bodyHeight), bodyHeight)
	frame := lipgloss.JoinVertical(lipgloss.Left, header, sourceBar, body, footer)
	return forceHeight(frame, renderModel.height)
}

func (m tuiModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searchActive {
		switch msg.String() {
		case "esc", "enter":
			m.searchActive = false
		case "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case "backspace", "ctrl+h":
			if m.search != "" {
				m.search = string([]rune(m.search)[:len([]rune(m.search))-1])
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.search += string(msg.Runes)
			}
		}
		m.clampCursors()
		return m, nil
	}

	if m.pendingExpunge != nil {
		switch msg.String() {
		case "y", "Y":
			target := *m.pendingExpunge
			m.pendingExpunge = nil
			m.status = "expunging..."
			return m, expungeTUICmd(m.endpoint, target)
		case "n", "N", "esc":
			m.pendingExpunge = nil
			m.status = "expunge cancelled"
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c", "q":
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	case "/":
		m.searchActive = true
	case "esc":
		m.search = ""
	case "m":
		m.toggleViewMode()
	case "a":
		m.cycleLayout()
	case "tab":
		m.nextIncludedSource(1)
	case "shift+tab":
		m.nextIncludedSource(-1)
	case "[", "left", "h":
		m.moveSourceCursor(-1)
	case "]", "right":
		m.moveSourceCursor(1)
	case "up", "k":
		m.moveRecordCursor(-1)
	case "down", "j":
		m.moveRecordCursor(1)
	case "pgup":
		m.pageRecordCursor(-1)
	case "pgdown":
		m.pageRecordCursor(1)
	case "home", "g":
		m.moveRecordCursorToStart()
	case "end", "G":
		m.moveRecordCursorToEnd()
	case "1":
		m.toggleLevel("DEBUG")
	case "2":
		m.toggleLevel("INFO")
	case "3":
		m.toggleLevel("WARN")
	case "4":
		m.toggleLevel("ERROR")
	case "s":
		m.toggleFocusedSource()
	case "p":
		m.togglePause()
	case "b":
		m.toggleBottom()
	case "d":
		m.toggleDetails()
	case "c":
		m.clearFocused()
	case "x":
		m.queueExpunge()
	case "+", "=":
		m.paneWidth += 2
	case "-", "_":
		m.paneWidth -= 2
		if m.paneWidth < tuiMinPaneWidth {
			m.paneWidth = tuiMinPaneWidth
		}
	}

	m.clampCursors()
	return m, nil
}

func waitForTUIEvent(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return tuiStreamClosedMsg{}
		}
		return msg
	}
}

func streamTUIRecords(ctx context.Context, brokerClient *client.Client, replayPerSource int, events chan<- tea.Msg) {
	defer close(events)
	for {
		if !sendTUIEvent(ctx, events, tuiStatusMsg{State: "connecting", Text: "connecting"}) {
			return
		}
		dialCtx, cancelDial := context.WithTimeout(ctx, 2*time.Second)
		sub, err := brokerClient.Subscribe(dialCtx, protocol.Subscribe{MinLevel: "debug", ReplayPerSource: replayPerSource})
		cancelDial()
		if err != nil {
			if !sendTUIEvent(ctx, events, tuiStreamErrMsg{Err: err}) {
				return
			}
			if !sleepTUIReconnect(ctx) {
				return
			}
			continue
		}

		if !sendTUIEvent(ctx, events, tuiStatusMsg{State: "loading", Text: "loading replay"}) {
			_ = sub.Close()
			return
		}

		reconnect := false
		records, streamErr, ok := loadInitialTUIReplay(ctx, sub, &reconnect)
		if !ok {
			_ = sub.Close()
			return
		}
		if reconnect {
			if streamErr != nil && !sendTUIEvent(ctx, events, tuiStreamErrMsg{Err: streamErr}) {
				_ = sub.Close()
				return
			}
			_ = sub.Close()
			if !sleepTUIReconnect(ctx) {
				return
			}
			continue
		}
		if !sendTUIEvent(ctx, events, tuiReplayLoadedMsg{Records: records}) {
			_ = sub.Close()
			return
		}

		for !reconnect {
			select {
			case <-ctx.Done():
				_ = sub.Close()
				return
			case record, ok := <-sub.Records:
				if !ok {
					reconnect = true
					continue
				}
				if !sendTUIEvent(ctx, events, tuiRecordMsg{Record: record}) {
					_ = sub.Close()
					return
				}
			case err, ok := <-sub.Errors:
				if ok && err != nil && ctx.Err() == nil {
					_ = sendTUIEvent(ctx, events, tuiStreamErrMsg{Err: err})
				}
				reconnect = true
			}
		}
		_ = sub.Close()
		if !sleepTUIReconnect(ctx) {
			return
		}
	}
}

func loadInitialTUIReplay(ctx context.Context, sub *client.Subscription, reconnect *bool) ([]protocol.Record, error, bool) {
	records := make([]protocol.Record, 0)
	fallback := time.NewTimer(tuiReplayFallbackDelay)
	defer fallback.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, false
		case <-sub.ReplayDone:
			return drainAvailableTUIRecords(records, sub.Records), nil, true
		case record, ok := <-sub.Records:
			if !ok {
				*reconnect = true
				return records, nil, true
			}
			records = append(records, record)
		case err, ok := <-sub.Errors:
			if ok && err != nil && ctx.Err() == nil {
				*reconnect = true
				return records, err, true
			}
			*reconnect = true
			return records, nil, true
		case <-fallback.C:
			return drainAvailableTUIRecords(records, sub.Records), nil, true
		}
	}
}

func drainAvailableTUIRecords(records []protocol.Record, ch <-chan protocol.Record) []protocol.Record {
	for {
		select {
		case record, ok := <-ch:
			if !ok {
				return records
			}
			records = append(records, record)
		default:
			return records
		}
	}
}

func sendTUIEvent(ctx context.Context, events chan<- tea.Msg, msg tea.Msg) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- msg:
		return true
	}
}

func sleepTUIReconnect(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func expungeTUICmd(endpoint string, target tuiExpungeTarget) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		source := target.Source
		if target.All {
			source = ""
		}
		expunged, err := newClient(endpoint).Expunge(ctx, source)
		return tuiExpungeResultMsg{Target: target, Expunged: expunged, Err: err}
	}
}

func (m *tuiModel) handleRecord(record protocol.Record) {
	if record.ID == "" {
		record.ID = fmt.Sprintf("%s:%s:%s", record.Time.Format(time.RFC3339Nano), record.Source, record.Message)
	}
	record.Level = protocol.NormalizeLevel(record.Level)
	if record.Source != "" {
		m.knownSources[record.Source] = struct{}{}
	}
	if m.viewMode == tuiViewMerged && m.mergedPaused {
		return
	}
	if m.sourcePaused[record.Source] {
		return
	}

	m.records = mergeTUIRecord(m.records, tuiRecord{Record: record, Key: record.ID})
	if m.viewMode == tuiViewMerged && m.mergedBottom {
		records := m.visibleMergedRecords()
		m.mergedCursor = len(records) - 1
		m.mergedStart = tuiEndRecordWindowStart(len(records), m.mergedRecordRows())
	}
	if sourceBottomEnabled(m.sourceBottom, record.Source) {
		records := m.visibleSourceRecords(record.Source)
		m.sourceCursors[record.Source] = len(records) - 1
		m.sourceStarts[record.Source] = tuiEndRecordWindowStart(len(records), m.sourceRecordRows(record.Source))
	}
}

func mergeTUIRecord(records []tuiRecord, record tuiRecord) []tuiRecord {
	index := -1
	for i, existing := range records {
		if existing.Key == record.Key {
			index = i
			break
		}
	}
	if index >= 0 {
		records[index] = record
	} else {
		records = append(records, record)
	}

	sourceCount := 0
	for _, existing := range records {
		if existing.Source == record.Source {
			sourceCount++
		}
	}
	drop := sourceCount - tuiMaxPerSource
	if drop <= 0 {
		return records
	}
	next := records[:0]
	for _, existing := range records {
		if existing.Source == record.Source && drop > 0 {
			drop--
			continue
		}
		next = append(next, existing)
	}
	return next
}

func (m tuiModel) renderHeader() string {
	visible := len(m.visibleRecords())
	contentWidth := paddedContentWidth(m.width)
	title := fmt.Sprintf(
		"DevLogBus TUI  %s  broker %s  %s  %d shown / %d buffered",
		m.endpoint,
		m.connection,
		m.viewSummary(),
		visible,
		len(m.records),
	)
	if m.search != "" || m.searchActive {
		marker := ""
		if m.searchActive {
			marker = "_"
		}
		title += fmt.Sprintf("  search:%q%s", m.search, marker)
	}
	return tuiHeaderStyle.Render(fitText(title, contentWidth))
}

func (m tuiModel) renderSourceBar() string {
	sources := m.sources()
	if len(sources) == 0 {
		return tuiMutedStyle.Width(m.width).Render("Sources: waiting for records")
	}

	parts := []string{"Sources:"}
	used := runewidth.StringWidth("Sources:")
	for i, source := range sources {
		plainLabel := "[" + source + "]"
		styledLabel := tuiSourceStyle.Render(plainLabel)
		if m.excluded[source] {
			plainLabel = "(" + source + ")"
			styledLabel = tuiSourceOffStyle.Render(plainLabel)
		}
		if i == m.sourceCursor {
			plainLabel = ">" + plainLabel
			styledLabel = ">" + styledLabel
		}
		nextWidth := used + 1 + runewidth.StringWidth(plainLabel)
		if nextWidth > m.width {
			if used+4 <= m.width {
				parts = append(parts, "...")
			}
			break
		}
		used = nextWidth
		parts = append(parts, styledLabel)
	}
	return tuiBaseStyle.Width(m.width).Render(strings.Join(parts, " "))
}

func (m tuiModel) renderFooter() string {
	contentWidth := paddedContentWidth(m.width)
	if m.pendingExpunge != nil {
		target := "all broker replay records"
		if !m.pendingExpunge.All {
			target = "source " + m.pendingExpunge.Source
		}
		return tuiFooterStyle.Render(fitText("Confirm expunge "+target+"? y/n", contentWidth))
	}
	keys := "q quit  / search  m mode  a layout  tab focus  [] source  s include  1-4 levels  p pause  b bottom  d details  c clear  x expunge  +/- width"
	if m.errText != "" {
		keys = m.status + ": " + m.errText + " | " + keys
	} else if m.status != "" {
		keys = m.status + " | " + keys
	}
	return tuiFooterStyle.Render(fitText(keys, contentWidth))
}

func (m tuiModel) renderBody(width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if width >= 112 {
		detailWidth := width / 3
		if detailWidth < 34 {
			detailWidth = 34
		}
		if detailWidth > 46 {
			detailWidth = 46
		}
		mainWidth := width - detailWidth
		main := m.renderMain(mainWidth, height)
		detail := m.renderDetail(detailWidth, height)
		return forceHeight(lipgloss.JoinHorizontal(lipgloss.Top, main, detail), height)
	}

	detailHeight := 8
	if height < 16 {
		detailHeight = 5
	}
	if detailHeight > height {
		detailHeight = height / 2
	}
	mainHeight := height - detailHeight
	body := lipgloss.JoinVertical(lipgloss.Left, m.renderMain(width, mainHeight), m.renderDetail(width, detailHeight))
	return forceHeight(body, height)
}

func (m tuiModel) renderLoadingBody(width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	requested := fmt.Sprintf("Loading replay records from %s", m.endpoint)
	limit := fmt.Sprintf("requesting up to %d records per source", m.replayPerSource)
	if m.errText != "" {
		limit = m.errText
	}
	lines := make([]string, height)
	center := height / 2
	if center > 0 {
		center--
	}
	lines[center] = tuiHeaderStyle.Render(centerText(fitText(requested, width), width))
	if center+1 < height {
		lines[center+1] = tuiMutedStyle.Render(centerText(fitText(limit, width), width))
	}
	return strings.Join(lines, "\n")
}

func (m tuiModel) renderMain(width int, height int) string {
	if m.viewMode == tuiViewSource {
		return m.renderSourcePanes(width, height)
	}
	return m.renderMergedPane(width, height)
}

func (m tuiModel) renderMergedPane(width int, height int) string {
	records := m.visibleMergedRecords()
	title := fmt.Sprintf(
		"Merged %d/%d  levels %s  pause:%s bottom:%s details:%s",
		len(records),
		len(m.records),
		levelLabel(m.mergedLevels),
		boolLabel(m.mergedPaused),
		boolLabel(m.mergedBottom),
		boolLabel(m.mergedDetails),
	)
	lines := m.renderRecordWindow(records, m.mergedCursor, m.mergedStart, width-4, tuiPanelRecordRows(height), true, m.mergedDetails)
	return renderTUIPanel(title, lines, width, height, true)
}

func (m tuiModel) renderSourcePanes(width int, height int) string {
	panes := m.sourcePanes()
	if len(panes) == 0 {
		return renderTUIPanel("By source", []string{"Waiting for sources."}, width, height, true)
	}

	switch m.sourceLayout {
	case tuiLayoutVertical:
		return m.renderVerticalPanes(panes, width, height)
	case tuiLayoutHorizontal:
		return m.renderHorizontalPanes(panes, width, height)
	default:
		return m.renderTiledPanes(panes, width, height)
	}
}

func (m tuiModel) renderVerticalPanes(panes []tuiPane, width int, height int) string {
	heights := distributeSize(height, len(panes))
	rendered := make([]string, 0, len(panes))
	for i, pane := range panes {
		if heights[i] <= 0 {
			continue
		}
		rendered = append(rendered, m.renderSourcePane(pane, width, heights[i]))
	}
	return forceHeight(lipgloss.JoinVertical(lipgloss.Left, rendered...), height)
}

func (m tuiModel) renderHorizontalPanes(panes []tuiPane, width int, height int) string {
	widths := distributeSize(width, len(panes))
	rendered := make([]string, 0, len(panes))
	for i, pane := range panes {
		if widths[i] <= 0 {
			continue
		}
		rendered = append(rendered, m.renderSourcePane(pane, widths[i], height))
	}
	return forceHeight(lipgloss.JoinHorizontal(lipgloss.Top, rendered...), height)
}

func (m tuiModel) renderTiledPanes(panes []tuiPane, width int, height int) string {
	paneWidth := m.paneWidth
	if paneWidth < tuiMinPaneWidth {
		paneWidth = tuiMinPaneWidth
	}
	if paneWidth > width {
		paneWidth = width
	}
	columns := width / paneWidth
	if columns < 1 {
		columns = 1
	}
	if columns > len(panes) {
		columns = len(panes)
	}
	rows := (len(panes) + columns - 1) / columns
	rowHeights := distributeSize(height, rows)

	renderedRows := make([]string, 0, rows)
	for row := 0; row < rows; row++ {
		if rowHeights[row] <= 0 {
			continue
		}
		rowPanes := make([]string, 0, columns)
		widths := distributeSize(width, columns)
		for col := 0; col < columns; col++ {
			index := row*columns + col
			if index >= len(panes) {
				continue
			}
			if widths[col] <= 0 {
				continue
			}
			rowPanes = append(rowPanes, m.renderSourcePane(panes[index], widths[col], rowHeights[row]))
		}
		renderedRows = append(renderedRows, lipgloss.JoinHorizontal(lipgloss.Top, rowPanes...))
	}
	return forceHeight(lipgloss.JoinVertical(lipgloss.Left, renderedRows...), height)
}

func (m tuiModel) renderSourcePane(pane tuiPane, width int, height int) string {
	active := m.focusedSource() == pane.Source && !m.excluded[pane.Source]
	levels := m.levelsForSource(pane.Source)
	title := fmt.Sprintf(
		"%s %d/%d  %s P:%s B:%s D:%s",
		pane.Source,
		len(pane.Records),
		pane.Total,
		levelLabel(levels),
		boolLabel(m.sourcePaused[pane.Source]),
		boolLabel(sourceBottomEnabled(m.sourceBottom, pane.Source)),
		boolLabel(m.sourceDetails[pane.Source]),
	)
	cursor := m.sourceCursors[pane.Source]
	lines := m.renderRecordWindow(pane.Records, cursor, m.sourceStarts[pane.Source], width-4, tuiPanelRecordRows(height), false, m.sourceDetails[pane.Source])
	return renderTUIPanel(title, lines, width, height, active)
}

func (m tuiModel) renderRecordWindow(records []tuiRecord, cursor int, start int, width int, height int, includeSource bool, details bool) []string {
	if width < 10 {
		width = 10
	}
	if height < 1 {
		height = 1
	}
	if len(records) == 0 {
		return []string{tuiMutedStyle.Render("No matching records.")}
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(records) {
		cursor = len(records) - 1
	}
	start = tuiKeepCursorVisible(len(records), cursor, start, height)
	end := start + height
	if end > len(records) {
		end = len(records)
	}

	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		lines = append(lines, renderTUIRecordLine(records[i], width, includeSource, details, i == cursor))
	}
	return lines
}

func renderTUIPanel(title string, lines []string, width int, height int, active bool) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if height == 1 || width < 5 {
		return fitText(title, width)
	}
	borderStyle := tuiPanelStyle
	if active {
		borderStyle = tuiPanelActive
	}
	contentWidth := width - 4
	contentHeight := height - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	renderedLines := make([]string, 0, contentHeight)
	renderedLines = append(renderedLines, fitANSI(title, contentWidth))
	for _, line := range lines {
		if len(renderedLines) >= contentHeight {
			break
		}
		renderedLines = append(renderedLines, fitANSI(line, contentWidth))
	}
	for len(renderedLines) < contentHeight {
		renderedLines = append(renderedLines, "")
	}

	horizontal := strings.Repeat("─", width-2)
	output := make([]string, 0, height)
	output = append(output, borderStyle.Render("╭"+horizontal+"╮"))
	for _, line := range renderedLines {
		fitted := fitANSI(line, contentWidth)
		output = append(output, borderStyle.Render("│")+" "+padANSI(fitted, contentWidth)+" "+borderStyle.Render("│"))
	}
	output = append(output, borderStyle.Render("╰"+horizontal+"╯"))
	return strings.Join(output, "\n")
}

func renderTUIRecordLine(record tuiRecord, width int, includeSource bool, details bool, selected bool) string {
	attrs := ""
	if details {
		attrs = attrSummary(record.Attrs)
		if attrs != "" {
			attrs = ` "` + attrs + `"`
		}
	}
	parts := []string{
		formatTUITime(record.Time),
		fmt.Sprintf("%-5s", protocol.NormalizeLevel(record.Level)),
	}
	if includeSource {
		parts = append(parts, fitText(record.Source, 20))
	}
	parts = append(parts, record.Message+attrs)
	line := fitText(strings.Join(parts, " "), width)

	style := styleForLevel(record.Level)
	if selected {
		style = tuiSelectedStyle
	}
	return style.Render(line)
}

func (m tuiModel) renderDetail(width int, height int) string {
	record := m.selectedRecord()
	if record == nil {
		return renderTUIPanel("Details", []string{"No record selected."}, width, height, false)
	}
	lines := []string{
		"level:  " + protocol.NormalizeLevel(record.Level),
		"time:   " + record.Time.Format("2006-01-02 15:04:05.000"),
		"source: " + record.Source,
		"id:     " + record.ID,
		"msg:    " + record.Message,
	}
	keys := make([]string, 0, len(record.Attrs))
	for key := range record.Attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, key+": "+attrValue(record.Attrs[key]))
	}
	return renderTUIPanel("Details", lines, width, height, false)
}

func (m tuiModel) visibleRecords() []tuiRecord {
	if m.viewMode == tuiViewSource {
		records := make([]tuiRecord, 0)
		for _, pane := range m.sourcePanes() {
			records = append(records, pane.Records...)
		}
		return records
	}
	return m.visibleMergedRecords()
}

func (m tuiModel) visibleMergedRecords() []tuiRecord {
	query := strings.ToLower(strings.TrimSpace(m.search))
	records := make([]tuiRecord, 0, len(m.records))
	for _, record := range m.records {
		if !m.mergedLevels[protocol.NormalizeLevel(record.Level)] {
			continue
		}
		if m.excluded[record.Source] {
			continue
		}
		if !recordMatchesTUISearch(record, query) {
			continue
		}
		records = append(records, record)
	}
	return records
}

func (m tuiModel) visibleSourceRecords(source string) []tuiRecord {
	query := strings.ToLower(strings.TrimSpace(m.search))
	levels := m.levelsForSource(source)
	records := make([]tuiRecord, 0)
	for _, record := range m.records {
		if record.Source != source {
			continue
		}
		if !levels[protocol.NormalizeLevel(record.Level)] {
			continue
		}
		if !recordMatchesTUISearch(record, query) {
			continue
		}
		records = append(records, record)
	}
	return records
}

func (m tuiModel) sourcePanes() []tuiPane {
	sources := m.sources()
	panes := make([]tuiPane, 0, len(sources))
	for _, source := range sources {
		if m.excluded[source] {
			continue
		}
		total := 0
		for _, record := range m.records {
			if record.Source == source {
				total++
			}
		}
		panes = append(panes, tuiPane{
			Source:  source,
			Records: m.visibleSourceRecords(source),
			Total:   total,
		})
	}
	return panes
}

func (m tuiModel) selectedRecord() *tuiRecord {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source == "" || m.excluded[source] {
			return nil
		}
		records := m.visibleSourceRecords(source)
		if len(records) == 0 {
			return nil
		}
		cursor := clampInt(m.sourceCursors[source], 0, len(records)-1)
		return &records[cursor]
	}

	records := m.visibleMergedRecords()
	if len(records) == 0 {
		return nil
	}
	cursor := clampInt(m.mergedCursor, 0, len(records)-1)
	return &records[cursor]
}

func (m tuiModel) sources() []string {
	set := map[string]struct{}{}
	for source := range m.knownSources {
		if source != "" {
			set[source] = struct{}{}
		}
	}
	for _, record := range m.records {
		if record.Source != "" {
			set[record.Source] = struct{}{}
		}
	}
	sources := make([]string, 0, len(set))
	for source := range set {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return sources
}

func (m tuiModel) focusedSource() string {
	sources := m.sources()
	if len(sources) == 0 {
		return ""
	}
	cursor := clampInt(m.sourceCursor, 0, len(sources)-1)
	return sources[cursor]
}

func (m *tuiModel) levelsForSource(source string) map[string]bool {
	levels := m.perSourceLevel[source]
	if levels == nil {
		levels = defaultTUILevels()
		m.perSourceLevel[source] = levels
	}
	return levels
}

func (m *tuiModel) toggleViewMode() {
	if m.viewMode == tuiViewMerged {
		m.viewMode = tuiViewSource
		return
	}
	m.viewMode = tuiViewMerged
}

func (m *tuiModel) cycleLayout() {
	for i, layout := range tuiLayoutOrder {
		if m.sourceLayout == layout {
			m.sourceLayout = tuiLayoutOrder[(i+1)%len(tuiLayoutOrder)]
			return
		}
	}
	m.sourceLayout = tuiLayoutTiled
}

func (m *tuiModel) moveSourceCursor(delta int) {
	sources := m.sources()
	if len(sources) == 0 {
		m.sourceCursor = 0
		return
	}
	m.sourceCursor = (m.sourceCursor + delta + len(sources)) % len(sources)
}

func (m *tuiModel) nextIncludedSource(delta int) {
	sources := m.sources()
	if len(sources) == 0 {
		return
	}
	for i := 0; i < len(sources); i++ {
		m.moveSourceCursor(delta)
		if !m.excluded[m.focusedSource()] {
			return
		}
	}
}

func (m *tuiModel) moveRecordCursor(delta int) {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source == "" {
			return
		}
		records := m.visibleSourceRecords(source)
		m.sourceCursors[source] = clampInt(m.sourceCursors[source]+delta, 0, len(records)-1)
		m.sourceStarts[source] = tuiKeepCursorVisible(
			len(records),
			m.sourceCursors[source],
			m.sourceStarts[source],
			m.sourceRecordRows(source),
		)
		if delta != 0 {
			m.sourceBottom[source] = false
		}
		return
	}
	records := m.visibleMergedRecords()
	m.mergedCursor = clampInt(m.mergedCursor+delta, 0, len(records)-1)
	m.mergedStart = tuiKeepCursorVisible(
		len(records),
		m.mergedCursor,
		m.mergedStart,
		m.mergedRecordRows(),
	)
	if delta != 0 {
		m.mergedBottom = false
	}
}

func (m *tuiModel) pageRecordCursor(direction int) {
	if direction == 0 {
		return
	}
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source == "" {
			return
		}
		records := m.visibleSourceRecords(source)
		m.sourceCursors[source], m.sourceStarts[source] = tuiPageRecordWindow(
			len(records),
			m.sourceCursors[source],
			m.sourceStarts[source],
			m.sourceRecordRows(source),
			direction,
		)
		m.sourceBottom[source] = false
		return
	}

	records := m.visibleMergedRecords()
	m.mergedCursor, m.mergedStart = tuiPageRecordWindow(
		len(records),
		m.mergedCursor,
		m.mergedStart,
		m.mergedRecordRows(),
		direction,
	)
	m.mergedBottom = false
}

func (m *tuiModel) moveRecordCursorToStart() {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		m.sourceCursors[source] = 0
		m.sourceStarts[source] = 0
		m.sourceBottom[source] = false
		return
	}
	m.mergedCursor = 0
	m.mergedStart = 0
	m.mergedBottom = false
}

func (m *tuiModel) moveRecordCursorToEnd() {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		records := m.visibleSourceRecords(source)
		m.sourceCursors[source] = len(records) - 1
		m.sourceStarts[source] = tuiEndRecordWindowStart(len(records), m.sourceRecordRows(source))
		m.sourceBottom[source] = true
		return
	}
	records := m.visibleMergedRecords()
	m.mergedCursor = len(records) - 1
	m.mergedStart = tuiEndRecordWindowStart(len(records), m.mergedRecordRows())
	m.mergedBottom = true
}

func (m *tuiModel) toggleLevel(level string) {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source == "" {
			return
		}
		toggleTUILevel(m.levelsForSource(source), level)
		return
	}
	toggleTUILevel(m.mergedLevels, level)
}

func (m *tuiModel) toggleFocusedSource() {
	source := m.focusedSource()
	if source == "" {
		return
	}
	m.excluded[source] = !m.excluded[source]
}

func (m *tuiModel) togglePause() {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source != "" {
			m.sourcePaused[source] = !m.sourcePaused[source]
		}
		return
	}
	m.mergedPaused = !m.mergedPaused
}

func (m *tuiModel) toggleBottom() {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source != "" {
			m.sourceBottom[source] = !sourceBottomEnabled(m.sourceBottom, source)
			if m.sourceBottom[source] {
				m.moveRecordCursorToEnd()
			}
		}
		return
	}
	m.mergedBottom = !m.mergedBottom
	if m.mergedBottom {
		m.moveRecordCursorToEnd()
	}
}

func (m *tuiModel) toggleDetails() {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source != "" {
			m.sourceDetails[source] = !m.sourceDetails[source]
		}
		return
	}
	m.mergedDetails = !m.mergedDetails
}

func (m *tuiModel) clearFocused() {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source != "" {
			m.clearSource(source, false)
		}
		return
	}
	m.clearAll()
}

func (m *tuiModel) queueExpunge() {
	if m.viewMode == tuiViewSource {
		source := m.focusedSource()
		if source == "" {
			return
		}
		m.pendingExpunge = &tuiExpungeTarget{Source: source}
		return
	}
	m.pendingExpunge = &tuiExpungeTarget{All: true}
}

func (m *tuiModel) clearAll() {
	m.records = nil
	m.knownSources = map[string]struct{}{}
	m.excluded = map[string]bool{}
	m.sourcePaused = map[string]bool{}
	m.sourceCursors = map[string]int{}
	m.sourceStarts = map[string]int{}
	m.sourceCursor = 0
	m.mergedCursor = 0
	m.mergedStart = 0
}

func (m *tuiModel) clearSource(source string, forget bool) {
	next := m.records[:0]
	for _, record := range m.records {
		if record.Source != source {
			next = append(next, record)
		}
	}
	m.records = next
	m.sourceCursors[source] = 0
	if forget {
		delete(m.knownSources, source)
		delete(m.excluded, source)
		delete(m.perSourceLevel, source)
		delete(m.sourcePaused, source)
		delete(m.sourceBottom, source)
		delete(m.sourceDetails, source)
		delete(m.sourceCursors, source)
		delete(m.sourceStarts, source)
	} else if source != "" {
		m.knownSources[source] = struct{}{}
	}
}

func (m *tuiModel) clampCursors() {
	sources := m.sources()
	if len(sources) == 0 {
		m.sourceCursor = 0
	} else {
		m.sourceCursor = clampInt(m.sourceCursor, 0, len(sources)-1)
	}
	merged := m.visibleMergedRecords()
	m.mergedCursor = clampInt(m.mergedCursor, 0, len(merged)-1)
	m.mergedStart = tuiKeepCursorVisible(len(merged), m.mergedCursor, m.mergedStart, m.mergedRecordRows())
	for _, source := range sources {
		records := m.visibleSourceRecords(source)
		m.sourceCursors[source] = clampInt(m.sourceCursors[source], 0, len(records)-1)
		m.sourceStarts[source] = tuiKeepCursorVisible(len(records), m.sourceCursors[source], m.sourceStarts[source], m.sourceRecordRows(source))
	}
}

func (m tuiModel) viewSummary() string {
	if m.viewMode == tuiViewSource {
		text := "by source/" + m.sourceLayout
		if m.sourceLayout == tuiLayoutTiled {
			text += fmt.Sprintf(" width:%d", m.paneWidth)
		}
		return text
	}
	return "merged"
}

func (m tuiModel) renderWidth() int {
	width := m.width
	if width > tuiRightEdgeGuard+20 {
		return width - tuiRightEdgeGuard
	}
	if width > 1 {
		return width - 1
	}
	return width
}

func (m tuiModel) bodyHeight() int {
	renderModel := m
	renderModel.width = m.renderWidth()
	header := renderModel.renderHeader()
	sourceBar := renderModel.renderSourceBar()
	footer := renderModel.renderFooter()
	height := renderModel.height - lipgloss.Height(header) - lipgloss.Height(sourceBar) - lipgloss.Height(footer)
	if height < 0 {
		return 0
	}
	return height
}

func (m tuiModel) mainPaneHeight() int {
	height := m.bodyHeight()
	if m.renderWidth() >= 112 {
		return height
	}
	detailHeight := 8
	if height < 16 {
		detailHeight = 5
	}
	if detailHeight > height {
		detailHeight = height / 2
	}
	mainHeight := height - detailHeight
	if mainHeight < 0 {
		return 0
	}
	return mainHeight
}

func (m tuiModel) mergedRecordRows() int {
	return tuiPanelRecordRows(m.mainPaneHeight())
}

func (m tuiModel) sourceRecordRows(source string) int {
	mainHeight := m.mainPaneHeight()
	panes := m.sourcePanes()
	if len(panes) == 0 {
		return tuiPanelRecordRows(mainHeight)
	}

	index := -1
	for i, pane := range panes {
		if pane.Source == source {
			index = i
			break
		}
	}
	if index < 0 {
		return tuiPanelRecordRows(mainHeight)
	}

	switch m.sourceLayout {
	case tuiLayoutVertical:
		heights := distributeSize(mainHeight, len(panes))
		return tuiPanelRecordRows(heights[index])
	case tuiLayoutHorizontal:
		return tuiPanelRecordRows(mainHeight)
	default:
		width := m.renderWidth()
		paneWidth := m.paneWidth
		if paneWidth < tuiMinPaneWidth {
			paneWidth = tuiMinPaneWidth
		}
		if paneWidth > width {
			paneWidth = width
		}
		columns := 1
		if paneWidth > 0 {
			columns = width / paneWidth
		}
		if columns < 1 {
			columns = 1
		}
		if columns > len(panes) {
			columns = len(panes)
		}
		rows := (len(panes) + columns - 1) / columns
		rowHeights := distributeSize(mainHeight, rows)
		row := index / columns
		if row < 0 || row >= len(rowHeights) {
			return tuiPanelRecordRows(mainHeight)
		}
		return tuiPanelRecordRows(rowHeights[row])
	}
}

func defaultTUILevels() map[string]bool {
	levels := map[string]bool{}
	for _, level := range tuiLevels {
		levels[level] = true
	}
	return levels
}

func toggleTUILevel(levels map[string]bool, level string) {
	level = protocol.NormalizeLevel(level)
	levels[level] = !levels[level]
}

func levelLabel(levels map[string]bool) string {
	labels := make([]string, 0, len(tuiLevels))
	for _, level := range tuiLevels {
		label := level[:1]
		if !levels[level] {
			label = strings.ToLower(label)
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, "")
}

func boolLabel(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func tuiPanelRecordRows(panelHeight int) int {
	rows := panelHeight - 3
	if rows < 1 {
		return 1
	}
	return rows
}

func tuiEndRecordWindowStart(total int, height int) int {
	if total <= 0 || height <= 0 || height >= total {
		return 0
	}
	return total - height
}

func tuiKeepCursorVisible(total int, cursor int, start int, height int) int {
	if total <= 0 || height <= 0 || height >= total {
		return 0
	}
	cursor = clampInt(cursor, 0, total-1)
	maxStart := tuiEndRecordWindowStart(total, height)
	start = clampInt(start, 0, maxStart)
	if cursor < start {
		return cursor
	}
	if cursor >= start+height {
		return cursor - height + 1
	}
	return start
}

func tuiPageRecordWindow(total int, cursor int, start int, height int, direction int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if height < 1 {
		height = 1
	}
	cursor = clampInt(cursor, 0, total-1)
	start = tuiKeepCursorVisible(total, cursor, start, height)
	if direction > 0 {
		bottom := start + height - 1
		if bottom >= total {
			bottom = total - 1
		}
		if cursor < bottom {
			return bottom, start
		}
		start = clampInt(start+height, 0, tuiEndRecordWindowStart(total, height))
		bottom = start + height - 1
		if bottom >= total {
			bottom = total - 1
		}
		return bottom, start
	}

	if cursor > start {
		return start, start
	}
	start = clampInt(start-height, 0, tuiEndRecordWindowStart(total, height))
	return start, start
}

func sourceBottomEnabled(values map[string]bool, source string) bool {
	value, ok := values[source]
	if !ok {
		return true
	}
	return value
}

func recordMatchesTUISearch(record tuiRecord, query string) bool {
	if query == "" {
		return true
	}
	parts := []string{
		record.Time.Format(time.RFC3339Nano),
		record.Level,
		record.Source,
		record.Message,
	}
	for key, value := range record.Attrs {
		parts = append(parts, key, attrValue(value))
	}
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), query)
}

func attrSummary(attrs map[string]any) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+attrValue(attrs[key]))
	}
	return strings.Join(parts, " ")
}

func attrValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatTUITime(t time.Time) string {
	if t.IsZero() {
		return "00:00:00.000"
	}
	return t.Format("15:04:05.000")
}

func styleForLevel(level string) lipgloss.Style {
	switch protocol.NormalizeLevel(level) {
	case "DEBUG":
		return tuiDebugStyle
	case "WARN":
		return tuiWarnStyle
	case "ERROR":
		return tuiErrorStyle
	default:
		return tuiInfoStyle
	}
}

func fitText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(value) <= width {
		return value
	}
	tail := "..."
	tailWidth := runewidth.StringWidth(tail)
	if width <= tailWidth {
		return runewidth.Truncate(value, width, "")
	}
	return runewidth.Truncate(value, width-tailWidth, "") + tail
}

func centerText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = fitText(value, width)
	padding := width - runewidth.StringWidth(value)
	if padding <= 0 {
		return value
	}
	left := padding / 2
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", padding-left)
}

func fitANSI(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= width {
		return value
	}
	tail := "..."
	tailWidth := ansi.StringWidth(tail)
	if width <= tailWidth {
		return ansi.Truncate(value, width, "")
	}
	return ansi.Truncate(value, width-tailWidth, "") + tail
}

func padANSI(value string, width int) string {
	padding := width - ansi.StringWidth(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func clampInt(value int, min int, max int) int {
	if max < min {
		return 0
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func paddedContentWidth(width int) int {
	if width <= 2 {
		return 0
	}
	return width - 2
}

func distributeSize(total int, count int) []int {
	if count <= 0 {
		return nil
	}
	sizes := make([]int, count)
	if total <= 0 {
		return sizes
	}
	base := total / count
	remainder := total % count
	for i := range sizes {
		sizes[i] = base
		if i < remainder {
			sizes[i]++
		}
	}
	return sizes
}

func forceHeight(value string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(value, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
