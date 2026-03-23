package sumi

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DiffMsg wraps a StateDiff for delivery into the Bubble Tea update loop.
type DiffMsg view.StateDiff

// Model is the Bubble Tea model for Sumi.
// It holds the circuit definition, layout, store subscription, and UI state.
// The War Room layout uses a PanelRegistry for extensible multi-panel composition.
type Model struct {
	def     *circuit.CircuitDef
	store   *view.CircuitStore
	layout  view.CircuitLayout
	snap    view.CircuitSnapshot
	opts    RenderOpts
	subID   int
	subCh   <-chan view.StateDiff
	width   int
	height  int
	ready   bool

	// Interactive state
	selected   int
	nodeOrder  []string
	inspecting bool
	searching  bool
	searchBuf  string

	// Debug state
	debug      *DebugClient
	debugAvail bool

	// Kami agent state
	kamiStatus  KamiStatus
	chatOpen    bool
	chatMsgs    []ChatMessage
	chatInput   string

	// War Room layout
	registry     *PanelRegistry
	warLayout    WarRoomLayout
	graphHitMap  map[[2]int]string
	eventCount   int
	workerFilter string // selected worker ID for filtering, "" = all
	timeline     *TimelineRingBuffer
	helpOpen     bool

	// Animation state
	animFrame      int
	edgeHighlights map[string]int
	nodeFlash      map[string]int

	// ViewRecorder captures NoColor frames for agent MCP access and F12 dump.
	recorder    *ViewRecorder
	statusFlash string
}

// KamiStatus represents the Kami MCP connection state.
type KamiStatus int

const (
	KamiOffline   KamiStatus = iota
	KamiConnected
)

// ChatMessage is a single message in the agent chat panel.
type ChatMessage struct {
	Role    string // "user", "agent", "system"
	Content string
}

// Config holds initialization parameters for a Sumi Model.
type Config struct {
	Def      *circuit.CircuitDef
	Store    *view.CircuitStore
	Layout   view.CircuitLayout
	Opts     RenderOpts
	Debug    *DebugClient
	Recorder *ViewRecorder // optional; nil disables frame recording
}

// New creates a Sumi Model ready for Bubble Tea.
func New(cfg Config) Model {
	snap := cfg.Store.Snapshot()

	order := make([]string, 0, len(cfg.Def.Nodes))
	for _, nd := range cfg.Def.Nodes {
		order = append(order, nd.Name)
	}

	subID, subCh := cfg.Store.Subscribe()

	m := Model{
		def:            cfg.Def,
		store:          cfg.Store,
		layout:         cfg.Layout,
		snap:           snap,
		opts:           cfg.Opts,
		nodeOrder:      order,
		debug:          cfg.Debug,
		registry:       NewPanelRegistry(),
		timeline:       NewTimelineRingBuffer(timelineMaxEntries),
		edgeHighlights: make(map[string]int),
		nodeFlash:      make(map[string]int),
		recorder:       cfg.Recorder,
		subID:          subID,
		subCh:          subCh,
	}
	return m
}

type tickMsg time.Time

const animTickInterval = 100 * time.Millisecond

// Init returns the initial Cmd that starts listening for store diffs.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		waitForDiff(m.subCh),
		tea.Tick(animTickInterval, func(t time.Time) tea.Msg { return tickMsg(t) }),
	)
}

func waitForDiff(ch <-chan view.StateDiff) tea.Cmd {
	return func() tea.Msg {
		diff, ok := <-ch
		if !ok {
			return tea.Quit()
		}
		return DiffMsg(diff)
	}
}

// clearFlashMsg signals the status flash should be cleared.
type clearFlashMsg struct{}

// Update processes Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.warLayout = ComputeLayout(m.width, m.height)
		m.ready = true
		m.maybeRecordFrame()
		return m, nil

	case DiffMsg:
		diff := view.StateDiff(msg)
		m.applyDiff(diff)
		m.eventCount++
		m.timeline.Push(DiffToTimelineEntry(diff))
		m.snap = m.store.Snapshot()
		m.maybeRecordFrame()
		return m, waitForDiff(m.subCh)

	case tickMsg:
		m.animFrame++
		for k, v := range m.edgeHighlights {
			if v <= 1 {
				delete(m.edgeHighlights, k)
			} else {
				m.edgeHighlights[k] = v - 1
			}
		}
		for k, v := range m.nodeFlash {
			if v <= 1 {
				delete(m.nodeFlash, k)
			} else {
				m.nodeFlash[k] = v - 1
			}
		}
		return m, tea.Tick(animTickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })

	case clearFlashMsg:
		m.statusFlash = ""
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) applyDiff(diff view.StateDiff) {
	switch diff.Type {
	case view.DiffReset:
		m.rebuildFromStore()
		m.kamiStatus = KamiConnected
	case view.DiffNodeState:
		if diff.State == view.NodeActive {
			for i, name := range m.nodeOrder {
				if name == diff.Node {
					m.selected = i
					break
				}
			}
		}
		if diff.State == view.NodeCompleted {
			m.nodeFlash[diff.Node] = 8
		}
	case view.DiffWalkerMoved:
		if diff.Walker != "" {
			if wp, ok := m.snap.Walkers[diff.Walker]; ok {
				edgeKey := wp.Node + "->" + diff.Node
				m.edgeHighlights[edgeKey] = 8
			}
		}
	}
}

// rebuildFromStore reconstructs the Model's rendering state (def, layout,
// nodeOrder) from the store's current snapshot. Called on DiffReset when
// the SSE client reconnects after a session swap and rebootstraps.
func (m *Model) rebuildFromStore() {
	snap := m.store.Snapshot()
	if len(snap.Nodes) == 0 {
		return
	}

	def := m.store.Def()
	if def == nil {
		def = &circuit.CircuitDef{Circuit: snap.CircuitName}
		for name := range snap.Nodes {
			def.Nodes = append(def.Nodes, circuit.NodeDef{Name: name})
		}
	}

	engine := &view.GridLayout{}
	layout, err := engine.Layout(def)
	if err != nil {
		return
	}

	order := make([]string, 0, len(def.Nodes))
	for _, nd := range def.Nodes {
		order = append(order, nd.Name)
	}

	m.def = def
	m.layout = layout
	m.nodeOrder = order
	m.selected = 0
	m.snap = snap
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !IsClick(msg) {
		return m, nil
	}

	graphRect := m.warLayout.Center
	target := DispatchMouse(msg, m.warLayout, m.registry, m.graphHitMap, graphRect)

	if target.PanelID != "" {
		m.registry.SetFocusByID(target.PanelID)
	}

	if target.Node != "" {
		for i, name := range m.nodeOrder {
			if name == target.Node {
				m.selected = i
				break
			}
		}
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpOpen {
		switch msg.String() {
		case "?", "esc", "q":
			m.helpOpen = false
		}
		return m, nil
	}
	if m.searching {
		return m.handleSearchKey(msg)
	}
	if m.chatOpen {
		return m.handleChatKey(msg)
	}

	switch msg.String() {
	case "?":
		m.helpOpen = true
		return m, nil

	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		if len(m.nodeOrder) > 0 {
			m.selected = (m.selected + 1) % len(m.nodeOrder)
		}
		return m, nil

	case "shift+tab":
		if len(m.nodeOrder) > 0 {
			m.selected = (m.selected - 1 + len(m.nodeOrder)) % len(m.nodeOrder)
		}
		return m, nil

	case "up":
		m.selected = m.findAdjacentNode("up")
		return m, nil
	case "down":
		m.selected = m.findAdjacentNode("down")
		return m, nil
	case "left":
		m.selected = m.findAdjacentNode("left")
		return m, nil
	case "right":
		m.selected = m.findAdjacentNode("right")
		return m, nil

	case "enter":
		m.inspecting = !m.inspecting
		return m, nil

	case "esc":
		if m.inspecting {
			m.inspecting = false
		}
		return m, nil

	case "/":
		m.searching = true
		m.searchBuf = ""
		return m, nil

	case " ":
		return m.toggleBreakpoint()

	case "p":
		return m.pauseWalk()

	case "r":
		return m.resumeWalk()

	case "n":
		return m.stepNode()

	case "c":
		if m.kamiStatus == KamiConnected {
			m.chatOpen = !m.chatOpen
		}
		return m, nil

	case "f12":
		return m.dumpDebugSnapshot()
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchBuf = ""
		return m, nil
	case "enter":
		m.applySearch()
		m.searching = false
		return m, nil
	case "backspace":
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
		}
		return m, nil
	default:
		if len(msg.String()) == 1 {
			m.searchBuf += msg.String()
		}
		return m, nil
	}
}

func (m *Model) applySearch() {
	if m.searchBuf == "" {
		return
	}
	q := strings.ToLower(m.searchBuf)
	for i, name := range m.nodeOrder {
		if strings.Contains(strings.ToLower(name), q) {
			m.selected = i
			return
		}
	}
	for zoneName, zd := range m.def.Zones {
		if strings.Contains(strings.ToLower(zoneName), q) {
			if len(zd.Nodes) > 0 {
				for i, name := range m.nodeOrder {
					if name == zd.Nodes[0] {
						m.selected = i
						return
					}
				}
			}
		}
	}
}

func (m Model) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.chatOpen = false
		return m, nil
	case "enter":
		if m.chatInput != "" {
			m.chatMsgs = append(m.chatMsgs, ChatMessage{Role: "user", Content: m.chatInput})
			m.chatMsgs = append(m.chatMsgs, ChatMessage{Role: "system", Content: "Agent: unavailable (MCP dispatch not connected)"})
			m.chatInput = ""
		}
		return m, nil
	case "backspace":
		if len(m.chatInput) > 0 {
			m.chatInput = m.chatInput[:len(m.chatInput)-1]
		}
		return m, nil
	default:
		if len(msg.String()) == 1 {
			m.chatInput += msg.String()
		}
		return m, nil
	}
}

func (m Model) findAdjacentNode(dir string) int {
	if len(m.nodeOrder) == 0 {
		return 0
	}
	currentName := m.nodeOrder[m.selected]
	currentGC, ok := m.layout.Grid[currentName]
	if !ok {
		return m.selected
	}

	bestIdx := m.selected
	bestDist := 999999

	for i, name := range m.nodeOrder {
		gc, ok := m.layout.Grid[name]
		if !ok || i == m.selected {
			continue
		}

		var match bool
		var dist int
		switch dir {
		case "up":
			match = gc.Row < currentGC.Row
			dist = (currentGC.Row-gc.Row)*10 + abs(gc.Col-currentGC.Col)
		case "down":
			match = gc.Row > currentGC.Row
			dist = (gc.Row-currentGC.Row)*10 + abs(gc.Col-currentGC.Col)
		case "left":
			match = gc.Col < currentGC.Col
			dist = (currentGC.Col-gc.Col)*10 + abs(gc.Row-currentGC.Row)
		case "right":
			match = gc.Col > currentGC.Col
			dist = (gc.Col-currentGC.Col)*10 + abs(gc.Row-currentGC.Row)
		}
		if match && dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}
	return bestIdx
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// --- Debug actions ---

func (m Model) toggleBreakpoint() (tea.Model, tea.Cmd) {
	if m.debug == nil || len(m.nodeOrder) == 0 {
		return m, nil
	}
	name := m.nodeOrder[m.selected]
	if m.snap.Breakpoints[name] {
		_ = m.debug.ClearBreakpoint(name)
	} else {
		_ = m.debug.SetBreakpoint(name)
	}
	return m, nil
}

func (m Model) pauseWalk() (tea.Model, tea.Cmd) {
	if m.debug != nil {
		_ = m.debug.Pause()
	}
	return m, nil
}

func (m Model) resumeWalk() (tea.Model, tea.Cmd) {
	if m.debug != nil {
		_ = m.debug.Resume()
	}
	return m, nil
}

func (m Model) stepNode() (tea.Model, tea.Cmd) {
	if m.debug != nil {
		_ = m.debug.AdvanceNode()
	}
	return m, nil
}

const debugSnapshotPath = ".sumi/debug-snapshot.txt"
const flashDuration = 2 * time.Second

func (m Model) dumpDebugSnapshot() (tea.Model, tea.Cmd) {
	if m.recorder == nil {
		return m, nil
	}
	flash := func(msg string) (tea.Model, tea.Cmd) {
		m.statusFlash = msg
		return m, tea.Tick(flashDuration, func(time.Time) tea.Msg { return clearFlashMsg{} })
	}

	f := m.recorder.Latest()
	if f == nil {
		return flash("No frames recorded")
	}

	if err := os.MkdirAll(".sumi", 0o755); err != nil {
		return flash(fmt.Sprintf("Error: %v", err))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Timestamp: %s\n", f.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(&sb, "Dimensions: %dx%d\n", f.Width, f.Height)
	fmt.Fprintf(&sb, "Layout tier: %s\n", f.LayoutTier)
	fmt.Fprintf(&sb, "Selected node: %s\n", f.SelectedNode)
	fmt.Fprintf(&sb, "Focused panel: %s\n", f.FocusedPanel)
	fmt.Fprintf(&sb, "Workers: %d\n", f.WorkerCount)
	fmt.Fprintf(&sb, "Events: %d\n", f.EventCount)
	sb.WriteString("---\n")
	sb.WriteString(f.ViewText)

	if err := os.WriteFile(debugSnapshotPath, []byte(sb.String()), 0o644); err != nil {
		return flash(fmt.Sprintf("Error: %v", err))
	}
	return flash("Snapshot saved")
}

// View renders the complete Sumi TUI.
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	opts := m.opts
	if m.selected < len(m.nodeOrder) {
		opts.SelectedNode = m.nodeOrder[m.selected]
	}
	opts.AnimFrame = m.animFrame
	opts.EdgeHighlights = m.edgeHighlights

	if m.warLayout.Tier >= TierCompact && !opts.NoColor {
		return m.viewWarRoom(opts)
	}
	return m.viewLegacy(opts)
}

func (m *Model) maybeRecordFrame() {
	if m.recorder == nil || !m.ready {
		return
	}
	opts := m.opts
	if m.selected < len(m.nodeOrder) {
		opts.SelectedNode = m.nodeOrder[m.selected]
	}
	m.recordFrame(opts)
}

func (m *Model) recordFrame(baseOpts RenderOpts) {
	ncOpts := baseOpts
	ncOpts.NoColor = true

	var viewText string
	if m.warLayout.Tier >= TierCompact {
		viewText = m.viewWarRoom(ncOpts)
	} else {
		viewText = m.viewLegacy(ncOpts)
	}

	tierName := "minimal"
	switch m.warLayout.Tier {
	case TierCompact:
		tierName = "compact"
	case TierStandard:
		tierName = "standard"
	case TierFull:
		tierName = "full"
	}

	focusedPanel := ""
	if m.registry != nil {
		focusedPanel = m.registry.FocusedID()
	}

	m.recorder.Record(view.RecordedFrame{
		Timestamp:    time.Now(),
		Width:        m.width,
		Height:       m.height,
		LayoutTier:   tierName,
		SelectedNode: baseOpts.SelectedNode,
		FocusedPanel: focusedPanel,
		WorkerCount:  len(m.snap.Walkers),
		EventCount:   m.eventCount,
		ViewText:     viewText,
	})
}

// viewWarRoom renders the multi-panel War Room layout using lipgloss
// composition instead of a rune canvas, ensuring ANSI escape codes
// do not corrupt the layout.
func (m *Model) viewWarRoom(opts RenderOpts) string {
	wl := m.warLayout

	graphStr, hitMap := RenderGraphWithHitMap(m.def, m.layout, m.snap, opts)
	m.graphHitMap = hitMap

	statusData := StatusBarDataFromModel(m)
	topBar := RenderWarRoomStatusBar(statusData)

	focusedID := m.registry.FocusedID()

	var middlePanels []string

	if wl.LeftSidebar.W > 0 && wl.LeftSidebar.H > 0 {
		casesContent := m.renderCasesContent()
		middlePanels = append(middlePanels,
			RenderPanelFrame("Cases", casesContent, wl.LeftSidebar, focusedID == "workers", opts.NoColor))
	}

	if wl.Center.W > 0 && wl.Center.H > 0 {
		innerW := wl.Center.W - 2
		innerH := wl.Center.H - 2
		if innerW > 0 && innerH > 0 {
			graphStr = lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, graphStr)
		}
		middlePanels = append(middlePanels,
			RenderPanelFrame("Circuit", graphStr, wl.Center, focusedID == "graph", opts.NoColor))
	}

	if wl.RightSidebar.W > 0 && wl.RightSidebar.H > 0 {
		inspectorContent := ""
		if m.selected < len(m.nodeOrder) {
			inspectorContent = m.renderInspectorContent()
		}
		middlePanels = append(middlePanels,
			RenderPanelFrame("Inspector", inspectorContent, wl.RightSidebar, focusedID == "inspector" || focusedID == "", opts.NoColor))
	}

	var rows []string
	rows = append(rows, topBar)

	if len(middlePanels) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, middlePanels...))
	}

	if wl.Bottom.W > 0 && wl.Bottom.H > 0 {
		timelineContent := m.renderTimelineContent()
		rows = append(rows,
			RenderPanelFrame("Events", timelineContent, wl.Bottom, focusedID == "timeline", opts.NoColor))
	}

	output := lipgloss.JoinVertical(lipgloss.Left, rows...)

	outputLines := strings.Split(output, "\n")
	for len(outputLines) < wl.Height {
		outputLines = append(outputLines, strings.Repeat(" ", wl.Width))
	}
	if len(outputLines) > wl.Height {
		outputLines = outputLines[:wl.Height]
	}

	if m.searching {
		searchBar := m.renderSearchBar()
		searchW := lipgloss.Width(searchBar)
		if searchW < wl.Width {
			searchBar += strings.Repeat(" ", wl.Width-searchW)
		}
		outputLines[wl.Height-1] = searchBar
	}

	output = strings.Join(outputLines, "\n")

	if m.helpOpen {
		helpOverlay := RenderHelpOverlay(wl.Width, wl.Height, opts.NoColor)
		output = overlayCenter(output, helpOverlay, wl.Width, wl.Height)
	}

	return output
}

// overlayCenter places the overlay string centered on the base string.
func overlayCenter(base, overlay string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	ovLines := strings.Split(overlay, "\n")
	ovH := len(ovLines)
	ovW := 0
	for _, l := range ovLines {
		if len([]rune(l)) > ovW {
			ovW = len([]rune(l))
		}
	}

	startY := (height - ovH) / 2
	startX := (width - ovW) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	for i, ovLine := range ovLines {
		y := startY + i
		if y >= len(baseLines) {
			break
		}
		baseRunes := []rune(baseLines[y])
		for len(baseRunes) < width {
			baseRunes = append(baseRunes, ' ')
		}
		ovRunes := []rune(ovLine)
		for j, ch := range ovRunes {
			x := startX + j
			if x < len(baseRunes) {
				baseRunes[x] = ch
			}
		}
		baseLines[y] = string(baseRunes)
	}

	return strings.Join(baseLines[:height], "\n")
}

// viewLegacy renders the original single-panel view for small terminals or --no-color.
func (m *Model) viewLegacy(opts RenderOpts) string {
	graphStr, hitMap := RenderGraphWithHitMap(m.def, m.layout, m.snap, opts)
	m.graphHitMap = hitMap

	var sections []string
	sections = append(sections, graphStr)

	if m.inspecting && m.selected < len(m.nodeOrder) {
		sections = append(sections, m.renderInspector())
	}

	if m.chatOpen {
		sections = append(sections, m.renderChatPanel())
	}

	sections = append(sections, m.renderStatusBar())

	if m.searching {
		sections = append(sections, m.renderSearchBar())
	}

	return strings.Join(sections, "\n")
}

// renderCasesContent produces the case list content (panel body only).
// Shows all cases with their lifecycle state, not just active walkers.
func (m Model) renderCasesContent() string {
	if len(m.snap.Cases) == 0 {
		if len(m.snap.Walkers) == 0 {
			return "No cases"
		}
		var sb strings.Builder
		for _, wp := range m.snap.Walkers {
			sb.WriteString(fmt.Sprintf("● %s @ %s\n", wp.WalkerID, wp.Node))
		}
		return strings.TrimRight(sb.String(), "\n")
	}

	ids := make([]string, 0, len(m.snap.Cases))
	for id := range m.snap.Cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var sb strings.Builder
	for _, id := range ids {
		ci := m.snap.Cases[id]
		icon := caseStateIcon(ci.State)
		line := fmt.Sprintf("%s %s @ %s", icon, ci.CaseID, ci.Node)
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

func caseStateIcon(s view.CaseVisualState) string {
	switch s {
	case view.CaseActive:
		return "▶"
	case view.CaseCompleted:
		return "✓"
	case view.CaseError:
		return "✗"
	default:
		return "○"
	}
}

// renderInspectorContent produces the inspector body without the border frame.
func (m Model) renderInspectorContent() string {
	if m.selected >= len(m.nodeOrder) {
		return ""
	}
	name := m.nodeOrder[m.selected]
	var nd *circuit.NodeDef
	for i := range m.def.Nodes {
		if m.def.Nodes[i].Name == name {
			nd = &m.def.Nodes[i]
			break
		}
	}
	if nd == nil {
		return ""
	}

	ns := m.snap.Nodes[name]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name:    %s\n", nd.Name))
	sb.WriteString(fmt.Sprintf("Approach: %s\n", nd.Approach))
	sb.WriteString(fmt.Sprintf("State:   %s\n", ns.State))
	handler := nd.EffectiveHandler()
	ht := nd.EffectiveHandlerType(m.def.HandlerType)
	if handler != "" && handler != nd.Name {
		sb.WriteString(fmt.Sprintf("Handler: %s\n", handler))
	}
	if ht != "" {
		sb.WriteString(fmt.Sprintf("Type:    %s\n", ht))
	}
	badge := DSBadge(handler)
	if badge != "" {
		sb.WriteString(fmt.Sprintf("D/S:     %s\n", badge))
	}
	zone := ns.Zone
	if zone == "" {
		zone = "(none)"
	}
	sb.WriteString(fmt.Sprintf("Zone:    %s\n", zone))

	for _, wp := range m.snap.Walkers {
		if wp.Node == nd.Name {
			sb.WriteString(fmt.Sprintf("Walker:  %s\n", wp.WalkerID))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderTimelineContent produces the event timeline body from the ring buffer.
// Events are shown newest-first so the most recent events are visible
// when the panel clips to its inner height.
func (m Model) renderTimelineContent() string {
	entries := m.timeline.Filtered(m.workerFilter)
	if len(entries) == 0 {
		return "Waiting for events..."
	}
	var sb strings.Builder
	for i := len(entries) - 1; i >= 0; i-- {
		sb.WriteString(entries[i].FormatEntry(m.opts.NoColor))
		if i > 0 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}


func (m Model) renderStatusBar() string {
	parts := []string{}

	// Walker progress
	for _, wp := range m.snap.Walkers {
		completed := 0
		for _, ns := range m.snap.Nodes {
			if ns.State == view.NodeCompleted {
				completed++
			}
		}
		total := len(m.snap.Nodes)
		bar := progressBar(completed, total, 10)
		elemStyle := ElementFg(wp.Element)
		if m.opts.NoColor {
			elemStyle = lipgloss.NewStyle()
		}
		parts = append(parts, elemStyle.Render(fmt.Sprintf("Walker: %s @ %s %s %d/%d", wp.WalkerID, wp.Node, bar, completed, total)))
	}

	// Breakpoints
	bps := []string{}
	for name, set := range m.snap.Breakpoints {
		if set {
			bps = append(bps, name)
		}
	}
	if len(bps) > 0 {
		parts = append(parts, fmt.Sprintf("BP: %s", strings.Join(bps, ", ")))
	}

	// Kami status
	switch m.kamiStatus {
	case KamiConnected:
		parts = append(parts, "🟢 Kami: connected")
	default:
		parts = append(parts, "⚫ Kami: offline")
	}

	// Paused / completed / error
	if m.snap.Paused {
		parts = append(parts, "[PAUSED]")
	}
	if m.snap.Completed {
		parts = append(parts, "[DONE]")
	}
	if m.snap.Error != "" {
		parts = append(parts, fmt.Sprintf("[ERROR: %s]", m.snap.Error))
	}

	// Selected node
	if m.selected < len(m.nodeOrder) {
		parts = append(parts, fmt.Sprintf("Selected: %s", m.nodeOrder[m.selected]))
	}

	status := strings.Join(parts, "  │  ")
	if m.opts.NoColor {
		return status
	}
	return StyleStatusBar.Render(status)
}

func (m Model) renderSearchBar() string {
	prompt := fmt.Sprintf("/ %s", m.searchBuf)
	if m.opts.NoColor {
		return prompt
	}
	return StyleSearchBar.Render(prompt)
}

func (m Model) renderInspector() string {
	if m.selected >= len(m.nodeOrder) {
		return ""
	}
	name := m.nodeOrder[m.selected]
	var nd *circuit.NodeDef
	for i := range m.def.Nodes {
		if m.def.Nodes[i].Name == name {
			nd = &m.def.Nodes[i]
			break
		}
	}
	if nd == nil {
		return ""
	}

	ns := m.snap.Nodes[name]

	var sb strings.Builder
	sb.WriteString("┌─── Inspector ─────────────────┐\n")
	sb.WriteString(fmt.Sprintf("│ Name:        %-16s │\n", nd.Name))
	sb.WriteString(fmt.Sprintf("│ Approach:    %-16s │\n", nd.Approach))
	sb.WriteString(fmt.Sprintf("│ State:       %-16s │\n", ns.State))
	handler2 := nd.EffectiveHandler()
	ht2 := nd.EffectiveHandlerType(m.def.HandlerType)
	if handler2 != "" && handler2 != nd.Name {
		sb.WriteString(fmt.Sprintf("│ Handler:     %-16s │\n", handler2))
	}
	if ht2 != "" {
		sb.WriteString(fmt.Sprintf("│ Type:        %-16s │\n", ht2))
	}
	badge := DSBadge(handler2)
	if badge != "" {
		sb.WriteString(fmt.Sprintf("│ D/S:         %-16s │\n", badge))
	}
	zone := ns.Zone
	if zone == "" {
		zone = "(none)"
	}
	sb.WriteString(fmt.Sprintf("│ Zone:        %-16s │\n", zone))
	sb.WriteString("└───────────────────────────────┘")
	return sb.String()
}

func (m Model) renderChatPanel() string {
	var sb strings.Builder
	sb.WriteString("┌─── Agent Chat ─────────────────┐\n")
	for _, msg := range m.chatMsgs {
		prefix := ""
		switch msg.Role {
		case "user":
			prefix = "You: "
		case "agent":
			prefix = "Agent: "
		case "system":
			prefix = "  "
		}
		sb.WriteString(fmt.Sprintf("│ %s%s\n", prefix, msg.Content))
	}
	sb.WriteString(fmt.Sprintf("│ > %s\n", m.chatInput))
	sb.WriteString("└────────────────────────────────┘")
	return sb.String()
}

func progressBar(current, total, width int) string {
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := (current * width) / total
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}
