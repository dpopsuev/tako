package sumi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/element"
	"github.com/dpopsuev/origami/view"
	"github.com/charmbracelet/lipgloss"
)

func resolveApproachToElement(approach string) string {
	e, _ := element.ResolveApproach(strings.ToLower(approach))
	return string(e)
}

const (
	nodeWidth  = 20
	nodeHeight = 3
	cellGapH   = 4
	cellGapV   = 1
)

// RenderGraph produces a static box-drawing representation of a circuit.
// It uses GridLayout positions for cell placement, draws zone borders,
// node rectangles with D/S badges, element colors, and edge lines.
func RenderGraph(def *circuit.CircuitDef, layout view.CircuitLayout, snap view.CircuitSnapshot, opts RenderOpts) string {
	result, _ := RenderGraphWithHitMap(def, layout, snap, opts)
	return result
}

// RenderGraphWithHitMap renders the graph and returns both the rendered string
// and a hit-map that maps canvas (x,y) coordinates to node names. The hit-map
// enables mouse interaction: given a click coordinate, look up the node.
func RenderGraphWithHitMap(def *circuit.CircuitDef, layout view.CircuitLayout, snap view.CircuitSnapshot, opts RenderOpts) (string, map[[2]int]string) {
	if len(layout.Grid) == 0 {
		return "(empty circuit)", nil
	}

	filteredGrid := filterDoneNode(layout.Grid, def)
	maxRow, maxCol := gridBounds(filteredGrid)

	routing := ComputeEdgeRouting(def, layout, def.Done)

	belowBaseY := (maxRow+1)*(nodeHeight+cellGapV) + cellGapV
	belowSpace := routing.Channels * 2
	if belowSpace > 0 {
		belowSpace++
	}
	loopSpace := len(routing.Loops) * 2
	if loopSpace > 0 {
		loopSpace++
	}

	canvasW := (maxCol+1)*(nodeWidth+cellGapH) + cellGapH
	canvasH := belowBaseY + belowSpace + loopSpace

	canvas := newCanvas(canvasW, canvasH)

	drawZones(canvas, def, layout, opts)
	drawInlineEdges(canvas, routing.Inline, layout, opts)
	drawBelowEdges(canvas, routing.Below, layout, belowBaseY, opts)
	loopBaseY := belowBaseY + belowSpace
	drawLoopEdges(canvas, routing.Loops, layout, loopBaseY, opts)
	drawNodes(canvas, def, layout, snap, opts)

	hitMap := buildHitMap(def, layout)

	return canvas.Render(opts), hitMap
}

// isVirtualDone returns true if the done node is a virtual terminal
// (not declared in the nodes list). Real nodes that happen to be the
// terminal (e.g. "report") should still be rendered.
func isVirtualDone(def *circuit.CircuitDef) bool {
	if def.Done == "" {
		return false
	}
	for _, nd := range def.Nodes {
		if nd.Name == def.Done {
			return false
		}
	}
	return true
}

func filterDoneNode(grid map[string]view.GridCell, def *circuit.CircuitDef) map[string]view.GridCell {
	if !isVirtualDone(def) {
		return grid
	}
	filtered := make(map[string]view.GridCell, len(grid))
	for name, gc := range grid {
		if name != def.Done {
			filtered[name] = gc
		}
	}
	return filtered
}

// buildHitMap maps every (x, y) cell inside a node box to the node's name.
func buildHitMap(def *circuit.CircuitDef, layout view.CircuitLayout) map[[2]int]string {
	hm := make(map[[2]int]string)
	virtualDone := isVirtualDone(def)
	for _, nd := range def.Nodes {
		if virtualDone && nd.Name == def.Done {
			continue
		}
		gc, ok := layout.Grid[nd.Name]
		if !ok {
			continue
		}
		ox, oy := cellOrigin(gc)
		for dy := 0; dy < nodeHeight; dy++ {
			for dx := 0; dx < nodeWidth; dx++ {
				hm[[2]int{ox + dx, oy + dy}] = nd.Name
			}
		}
	}
	return hm
}

func gridBounds(grid map[string]view.GridCell) (maxRow, maxCol int) {
	for _, gc := range grid {
		if gc.Row > maxRow {
			maxRow = gc.Row
		}
		if gc.Col > maxCol {
			maxCol = gc.Col
		}
	}
	return
}

func cellOrigin(gc view.GridCell) (x, y int) {
	x = gc.Col*(nodeWidth+cellGapH) + cellGapH
	y = gc.Row*(nodeHeight+cellGapV) + cellGapV
	return
}

// canvas is a 2D character buffer for compositing graph elements.
type canvas struct {
	cells  [][]rune
	styles [][]lipgloss.Style
	width  int
	height int
}

func newCanvas(w, h int) *canvas {
	cells := make([][]rune, h)
	styles := make([][]lipgloss.Style, h)
	for i := range cells {
		cells[i] = make([]rune, w)
		styles[i] = make([]lipgloss.Style, w)
		for j := range cells[i] {
			cells[i][j] = ' '
		}
	}
	return &canvas{cells: cells, styles: styles, width: w, height: h}
}

func (c *canvas) set(x, y int, ch rune, style lipgloss.Style) {
	if y >= 0 && y < c.height && x >= 0 && x < c.width {
		c.cells[y][x] = ch
		c.styles[y][x] = style
	}
}

func (c *canvas) putString(x, y int, s string, style lipgloss.Style) {
	col := 0
	for _, ch := range s {
		c.set(x+col, y, ch, style)
		col++
	}
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func truncateRunes(s string, max int) string {
	i := 0
	for pos := range s {
		if i >= max {
			return s[:pos]
		}
		i++
	}
	return s
}

func (c *canvas) Render(opts RenderOpts) string {
	var sb strings.Builder
	for y := 0; y < c.height; y++ {
		line := c.renderLine(y, opts)
		sb.WriteString(strings.TrimRight(line, " "))
		sb.WriteByte('\n')
	}
	return sb.String()
}

func (c *canvas) renderLine(y int, opts RenderOpts) string {
	if opts.NoColor {
		return string(c.cells[y])
	}
	var sb strings.Builder
	for x := 0; x < c.width; x++ {
		ch := string(c.cells[y][x])
		st := c.styles[y][x]
		sb.WriteString(st.Render(ch))
	}
	return sb.String()
}

// RenderOpts controls rendering behavior.
type RenderOpts struct {
	NoColor        bool
	Compact        bool
	Width          int
	SelectedNode   string // node name to highlight with selection border
	AnimFrame      int    // monotonic frame counter for animations
	EdgeHighlights map[string]int // "from->to" -> frames remaining
}

// --- Node drawing ---

func drawNodes(c *canvas, def *circuit.CircuitDef, layout view.CircuitLayout, snap view.CircuitSnapshot, opts RenderOpts) {
	nodeMap := make(map[string]*circuit.NodeDef, len(def.Nodes))
	for i := range def.Nodes {
		nodeMap[def.Nodes[i].Name] = &def.Nodes[i]
	}

	virtualDone := isVirtualDone(def)
	for name, gc := range layout.Grid {
		if virtualDone && name == def.Done {
			continue
		}
		nd := nodeMap[name]
		if nd == nil {
			continue
		}
		ns := snap.Nodes[name]
		x, y := cellOrigin(gc)
		drawNode(c, x, y, nd, ns, snap, opts, def.HandlerType)
	}
}

func drawNode(c *canvas, x, y int, nd *circuit.NodeDef, ns view.NodeState, snap view.CircuitSnapshot, opts RenderOpts, circuitHandlerType string) {
	selected := opts.SelectedNode == nd.Name
	style := nodeStyle(ns, opts)
	elemStyle := ElementFg(ns.Element)
	if opts.NoColor {
		elemStyle = lipgloss.NewStyle()
		style = lipgloss.NewStyle()
	}

	if ns.State == view.NodeActive && !opts.NoColor && !selected {
		style = ElementFg(ns.Element).Bold(true)
	}

	if selected && !opts.NoColor {
		style = StyleSelected
	}

	badge := ""
	if nd.EffectiveHandlerType(circuitHandlerType) == circuit.HandlerTypeTransformer {
		badge = DSBadge(nd.EffectiveHandler())
	}
	stateIcon := stateIndicator(ns.State, opts.AnimFrame)

	walkerMark := ""
	for _, wp := range snap.Walkers {
		if wp.Node == nd.Name {
			walkerMark = "●"
			break
		}
	}

	bpMark := ""
	if snap.Breakpoints[nd.Name] {
		bpMark = "◉"
	}

	// Selected nodes use double-line borders for visibility
	topCh, botCh, side := "┌", "└", '│'
	hBar := "─"
	if selected {
		topCh, botCh, side = "╔", "╚", '║'
		hBar = "═"
	}

	topBorder := topCh + strings.Repeat(hBar, nodeWidth-2) + mirrorCorner(topCh)
	c.putString(x, y, topBorder, style)

	label := nd.Name
	if badge != "" {
		label = badge + " " + label
	}
	prefix := ""
	if bpMark != "" {
		prefix = bpMark + " "
	}
	suffix := ""
	if walkerMark != "" {
		suffix += " " + walkerMark
	}
	if stateIcon != "" {
		suffix += " " + stateIcon
	}

	content := prefix + label + suffix
	inner := nodeWidth - 2
	contentLen := runeLen(content)
	if contentLen > inner {
		content = truncateRunes(content, inner)
		contentLen = inner
	}
	padded := content + strings.Repeat(" ", inner-contentLen)

	c.set(x, y+1, side, style)
	if !opts.NoColor {
		c.putString(x+1, y+1, padded, elemStyle)
	} else {
		c.putString(x+1, y+1, padded, lipgloss.NewStyle())
	}
	c.set(x+nodeWidth-1, y+1, side, style)

	bottomBorder := botCh + strings.Repeat(hBar, nodeWidth-2) + mirrorCorner(botCh)
	c.putString(x, y+2, bottomBorder, style)
}

func mirrorCorner(corner string) string {
	switch corner {
	case "┌":
		return "┐"
	case "└":
		return "┘"
	case "╔":
		return "╗"
	case "╚":
		return "╝"
	default:
		return corner
	}
}

func nodeStyle(ns view.NodeState, opts RenderOpts) lipgloss.Style {
	if opts.NoColor {
		return lipgloss.NewStyle()
	}
	switch ns.State {
	case view.NodeActive:
		return StyleActive
	case view.NodeCompleted:
		return StyleCompleted
	case view.NodeError:
		return StyleError
	default:
		return StyleIdle
	}
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func stateIndicator(state view.NodeVisualState, animFrame int) string {
	switch state {
	case view.NodeActive:
		if animFrame > 0 {
			return spinnerChars[animFrame%len(spinnerChars)]
		}
		return "▶"
	case view.NodeCompleted:
		return "✓"
	case view.NodeError:
		return "✗"
	default:
		return ""
	}
}

// --- Edge routing ---

// RoutedEdge is a deduplicated edge with routing metadata.
type RoutedEdge struct {
	From     string
	To       string
	Shortcut bool
	Loop     bool
	Channel  int // >=0 for below-path channel; -1 for inline
}

// EdgeRouting holds classified, deduplicated edges ready for rendering.
type EdgeRouting struct {
	Inline   []RoutedEdge // adjacent same-row or multi-row forward edges
	Below    []RoutedEdge // non-adjacent same-row forward edges (routed below)
	Loops    []RoutedEdge // backward (loop) edges
	Channels int          // number of below-path channels needed
}

// ComputeEdgeRouting deduplicates edges by (from,to) and classifies them
// into inline (adjacent), below-path (non-adjacent forward), and loop categories.
// Below-path edges get channel assignments via interval graph coloring.
func ComputeEdgeRouting(def *circuit.CircuitDef, layout view.CircuitLayout, doneNode string) EdgeRouting {
	virtualDone := isVirtualDone(def)

	type edgeKey struct{ from, to string }
	type dedupEntry struct {
		key      edgeKey
		shortcut bool
		loop     bool
	}
	seen := make(map[edgeKey]*dedupEntry)
	var order []edgeKey

	for _, e := range def.Edges {
		if virtualDone && (e.To == doneNode || e.From == doneNode) {
			continue
		}
		if e.From == e.To {
			continue
		}
		key := edgeKey{e.From, e.To}
		if de, ok := seen[key]; ok {
			if !e.Shortcut {
				de.shortcut = false
			}
			if e.Loop {
				de.loop = true
			}
			continue
		}
		seen[key] = &dedupEntry{key: key, shortcut: e.Shortcut, loop: e.Loop}
		order = append(order, key)
	}

	var result EdgeRouting
	type belowCandidate struct {
		edge    RoutedEdge
		fromCol int
		toCol   int
	}
	var belowCandidates []belowCandidate

	for _, key := range order {
		de := seen[key]
		re := RoutedEdge{From: de.key.from, To: de.key.to, Shortcut: de.shortcut, Loop: de.loop, Channel: -1}
		fromGC, fromOK := layout.Grid[re.From]
		toGC, toOK := layout.Grid[re.To]
		if !fromOK || !toOK {
			continue
		}

		if re.Loop || (fromGC.Row == toGC.Row && toGC.Col < fromGC.Col) {
			result.Loops = append(result.Loops, re)
		} else if fromGC.Row == toGC.Row && toGC.Col-fromGC.Col == 1 {
			result.Inline = append(result.Inline, re)
		} else if fromGC.Row == toGC.Row && toGC.Col-fromGC.Col > 1 {
			belowCandidates = append(belowCandidates, belowCandidate{re, fromGC.Col, toGC.Col})
		} else {
			result.Inline = append(result.Inline, re)
		}
	}

	// Channel assignment: interval graph coloring, shortest span first
	sort.Slice(belowCandidates, func(i, j int) bool {
		si := belowCandidates[i].toCol - belowCandidates[i].fromCol
		sj := belowCandidates[j].toCol - belowCandidates[j].fromCol
		if si != sj {
			return si < sj
		}
		return belowCandidates[i].fromCol < belowCandidates[j].fromCol
	})

	type interval struct{ from, to int }
	channels := make([][]interval, 0)
	for i := range belowCandidates {
		bc := &belowCandidates[i]
		iv := interval{bc.fromCol, bc.toCol}
		assigned := false
		for ch := range channels {
			overlap := false
			for _, ex := range channels[ch] {
				if iv.from < ex.to && iv.to > ex.from {
					overlap = true
					break
				}
			}
			if !overlap {
				channels[ch] = append(channels[ch], iv)
				bc.edge.Channel = ch
				assigned = true
				break
			}
		}
		if !assigned {
			channels = append(channels, []interval{iv})
			bc.edge.Channel = len(channels) - 1
		}
	}

	result.Channels = len(channels)
	for _, bc := range belowCandidates {
		result.Below = append(result.Below, bc.edge)
	}
	return result
}

// --- Edge drawing ---

func edgeStyle(re RoutedEdge, opts RenderOpts) lipgloss.Style {
	if opts.NoColor {
		return lipgloss.NewStyle()
	}
	edgeKey := re.From + "->" + re.To
	if opts.EdgeHighlights[edgeKey] > 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	}
	if re.Shortcut {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	}
	if re.Loop {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	}
	return lipgloss.NewStyle().Faint(true)
}

// drawInlineEdges draws adjacent same-row and multi-row forward edges
// using horizontal/vertical connectors.
func drawInlineEdges(c *canvas, edges []RoutedEdge, layout view.CircuitLayout, opts RenderOpts) {
	for _, re := range edges {
		fromGC := layout.Grid[re.From]
		toGC := layout.Grid[re.To]
		fromX, fromY := cellOrigin(fromGC)
		toX, toY := cellOrigin(toGC)

		style := edgeStyle(re, opts)

		startX := fromX + nodeWidth
		startY := fromY + nodeHeight/2
		endX := toX
		endY := toY + nodeHeight/2

		if startY == endY {
			for x := startX; x < endX; x++ {
				c.set(x, startY, '─', style)
			}
			c.set(endX-1, startY, '▸', style)
		} else {
			midX := (startX + endX) / 2
			for x := startX; x <= midX; x++ {
				c.set(x, startY, '─', style)
			}
			if endY > startY {
				c.set(midX, startY, '┐', style)
				for y := startY + 1; y < endY; y++ {
					c.set(midX, y, '│', style)
				}
				c.set(midX, endY, '└', style)
			} else {
				c.set(midX, startY, '┘', style)
				for y := endY + 1; y < startY; y++ {
					c.set(midX, y, '│', style)
				}
				c.set(midX, endY, '┌', style)
			}
			for x := midX + 1; x < endX; x++ {
				c.set(x, endY, '─', style)
			}
			c.set(endX-1, endY, '▸', style)
		}
	}
}

// drawBelowEdges draws non-adjacent forward edges as arcs below the main path.
// Each edge exits from the source node's bottom, runs horizontally along its
// assigned channel, and terminates with an arrowhead below the target node.
func drawBelowEdges(c *canvas, edges []RoutedEdge, layout view.CircuitLayout, baseY int, opts RenderOpts) {
	if len(edges) == 0 {
		return
	}

	exitIdx := make(map[string]int)
	entryIdx := make(map[string]int)
	exitCount := make(map[string]int)
	entryCount := make(map[string]int)
	for _, re := range edges {
		exitCount[re.From]++
		entryCount[re.To]++
	}

	for i, re := range edges {
		fromGC := layout.Grid[re.From]
		toGC := layout.Grid[re.To]
		fromX, fromY := cellOrigin(fromGC)
		toX, _ := cellOrigin(toGC)

		style := edgeStyle(re, opts)

		eIdx := exitIdx[re.From]
		exitIdx[re.From]++
		eCnt := exitCount[re.From]
		exitX := fromX + nodeWidth/2
		if eCnt > 1 {
			spacing := (nodeWidth - 4) / eCnt
			if spacing < 2 {
				spacing = 2
			}
			exitX = fromX + 2 + eIdx*spacing
		}

		nIdx := entryIdx[re.To]
		entryIdx[re.To]++
		nCnt := entryCount[re.To]
		entryX := toX + nodeWidth/2
		if nCnt > 1 {
			spacing := (nodeWidth - 4) / nCnt
			if spacing < 2 {
				spacing = 2
			}
			entryX = toX + 2 + nIdx*spacing
		}

		_ = i
		exitTopY := fromY + nodeHeight
		channelY := baseY + re.Channel*2

		ch := '╌'
		if !re.Shortcut {
			ch = '─'
		}

		// Vertical connector from source bottom to channel
		for y := exitTopY; y < channelY; y++ {
			c.set(exitX, y, '│', style)
		}
		c.set(exitX, channelY, '└', style)

		// Horizontal along channel
		for x := exitX + 1; x < entryX; x++ {
			c.set(x, channelY, ch, style)
		}

		// Arrowhead at target
		c.set(entryX, channelY, '▴', style)
	}
}

// drawLoopEdges draws backward (loop) edges below the shortcut channels.
// Each loop exits from the bottom-right area of the source node, runs down
// and left, then enters the left side of the target node with ◀.
func drawLoopEdges(c *canvas, edges []RoutedEdge, layout view.CircuitLayout, baseY int, opts RenderOpts) {
	for i, re := range edges {
		fromGC := layout.Grid[re.From]
		toGC := layout.Grid[re.To]
		fromX, fromY := cellOrigin(fromGC)
		toX, _ := cellOrigin(toGC)

		style := edgeStyle(re, opts)

		exitX := fromX + nodeWidth - 3
		exitTopY := fromY + nodeHeight
		loopY := baseY + i*2

		// Down from source bottom-right area
		for y := exitTopY; y < loopY; y++ {
			c.set(exitX, y, '│', style)
		}
		c.set(exitX, loopY, '┘', style)

		// Horizontal left to target
		targetX := toX
		for x := targetX + 1; x < exitX; x++ {
			c.set(x, loopY, '─', style)
		}
		c.set(targetX, loopY, '◀', style)
	}
}

// --- Abstract renderer ---

// RenderAbstract produces a simplified 2D grid visualization of a circuit.
// Nodes are rendered as * (or [*] for composite nodes with Meta["composite"]=true).
// Edges use ASCII connectors: - for horizontal, | for vertical, + for corners.
// Below-path edges and loops are drawn below the node grid.
// This output is stable across cosmetic rendering changes and is used for
// algorithmic validation in tests.
func RenderAbstract(def *circuit.CircuitDef, layout view.CircuitLayout) string {
	if len(layout.Grid) == 0 {
		return "(empty)"
	}

	filteredGrid := filterDoneNode(layout.Grid, def)
	if len(filteredGrid) == 0 {
		return "(empty)"
	}

	compositeNodes := make(map[string]bool)
	nodeByName := make(map[string]*circuit.NodeDef, len(def.Nodes))
	for i := range def.Nodes {
		nd := &def.Nodes[i]
		nodeByName[nd.Name] = nd
		if nd.Meta != nil {
			if v, ok := nd.Meta["composite"]; ok {
				if b, ok := v.(bool); ok && b {
					compositeNodes[nd.Name] = true
				}
			}
		}
	}

	maxRow, maxCol := gridBounds(filteredGrid)
	routing := ComputeEdgeRouting(def, layout, def.Done)

	gridW := (maxCol+1)*2 - 1
	if gridW < 1 {
		gridW = 1
	}
	gridH := (maxRow+1)*2 - 1
	if gridH < 1 {
		gridH = 1
	}

	belowLines := routing.Channels
	loopLines := len(routing.Loops)

	totalH := gridH
	if belowLines > 0 {
		totalH += belowLines
	}
	if loopLines > 0 {
		totalH += loopLines
	}

	grid := make([][]rune, totalH)
	for i := range grid {
		grid[i] = make([]rune, gridW)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	// Draw edges first, then nodes on top (nodes always win).
	for _, re := range routing.Inline {
		fromGC, fromOK := filteredGrid[re.From]
		toGC, toOK := filteredGrid[re.To]
		if !fromOK || !toOK {
			continue
		}

		fromGX, fromGY := fromGC.Col*2, fromGC.Row*2
		toGX, toGY := toGC.Col*2, toGC.Row*2

		if fromGY == toGY {
			for x := fromGX + 1; x < toGX; x++ {
				if x < len(grid[fromGY]) {
					grid[fromGY][x] = '-'
				}
			}
		} else {
			midGX := fromGX + 1
			if midGX < len(grid[fromGY]) {
				grid[fromGY][midGX] = '-'
			}
			if toGY > fromGY {
				for y := fromGY + 1; y < toGY; y++ {
					if midGX < len(grid[y]) {
						grid[y][midGX] = '|'
					}
				}
				if midGX < len(grid[fromGY]) {
					grid[fromGY][midGX] = '+'
				}
				if midGX < len(grid[toGY]) {
					grid[toGY][midGX] = '+'
				}
			} else {
				for y := toGY + 1; y < fromGY; y++ {
					if midGX < len(grid[y]) {
						grid[y][midGX] = '|'
					}
				}
				if midGX < len(grid[fromGY]) {
					grid[fromGY][midGX] = '+'
				}
				if midGX < len(grid[toGY]) {
					grid[toGY][midGX] = '+'
				}
			}
			for x := midGX + 1; x < toGX; x++ {
				if x < len(grid[toGY]) {
					grid[toGY][x] = '-'
				}
			}
		}
	}

	belowBaseY := gridH
	for _, re := range routing.Below {
		fromGC, fromOK := filteredGrid[re.From]
		toGC, toOK := filteredGrid[re.To]
		if !fromOK || !toOK {
			continue
		}
		fromGX := fromGC.Col * 2
		toGX := toGC.Col * 2
		channelY := belowBaseY + re.Channel
		for x := fromGX; x <= toGX; x++ {
			if x < len(grid[channelY]) {
				grid[channelY][x] = '-'
			}
		}
		if fromGX < len(grid[channelY]) {
			grid[channelY][fromGX] = '+'
		}
		if toGX < len(grid[channelY]) {
			grid[channelY][toGX] = '+'
		}
	}

	loopBaseY := belowBaseY + belowLines
	for i, re := range routing.Loops {
		fromGC, fromOK := filteredGrid[re.From]
		toGC, toOK := filteredGrid[re.To]
		if !fromOK || !toOK {
			continue
		}
		fromGX := fromGC.Col * 2
		toGX := toGC.Col * 2
		loopY := loopBaseY + i
		for x := toGX; x <= fromGX; x++ {
			if x < len(grid[loopY]) {
				grid[loopY][x] = '-'
			}
		}
		if toGX < len(grid[loopY]) {
			grid[loopY][toGX] = '<'
		}
		if fromGX < len(grid[loopY]) {
			grid[loopY][fromGX] = '+'
		}
	}

	// Draw nodes last so they overwrite edge characters.
	// Regular nodes: *, composite nodes: @
	for name, gc := range filteredGrid {
		gx := gc.Col * 2
		gy := gc.Row * 2
		if compositeNodes[name] {
			grid[gy][gx] = '@'
		} else {
			grid[gy][gx] = '*'
		}
	}

	var sb strings.Builder
	for _, row := range grid {
		line := strings.TrimRight(string(row), " ")
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// --- Zone drawing ---

type zoneBounds struct {
	name    string
	element string
	minRow  int
	maxRow  int
	minCol  int
	maxCol  int
}

func drawZones(c *canvas, def *circuit.CircuitDef, layout view.CircuitLayout, opts RenderOpts) {
	zones := make(map[string]*zoneBounds)

	sortedZoneNames := make([]string, 0, len(def.Zones))
	for name := range def.Zones {
		sortedZoneNames = append(sortedZoneNames, name)
	}
	sort.Strings(sortedZoneNames)

	for _, zoneName := range sortedZoneNames {
		zd := def.Zones[zoneName]
		for _, nodeName := range zd.Nodes {
			gc, ok := layout.Grid[nodeName]
			if !ok {
				continue
			}
			zb, exists := zones[zoneName]
			if !exists {
				zb = &zoneBounds{
					name: zoneName, element: resolveApproachToElement(zd.Approach),
					minRow: gc.Row, maxRow: gc.Row,
					minCol: gc.Col, maxCol: gc.Col,
				}
				zones[zoneName] = zb
			}
			if gc.Row < zb.minRow {
				zb.minRow = gc.Row
			}
			if gc.Row > zb.maxRow {
				zb.maxRow = gc.Row
			}
			if gc.Col < zb.minCol {
				zb.minCol = gc.Col
			}
			if gc.Col > zb.maxCol {
				zb.maxCol = gc.Col
			}
		}
	}

	for _, zoneName := range sortedZoneNames {
		if zb, ok := zones[zoneName]; ok {
			drawZoneBorder(c, zb, opts)
		}
	}
}

func drawZoneBorder(c *canvas, zb *zoneBounds, opts RenderOpts) {
	x1, y1 := cellOrigin(view.GridCell{Row: zb.minRow, Col: zb.minCol})
	x2, y2 := cellOrigin(view.GridCell{Row: zb.maxRow, Col: zb.maxCol})

	x1 -= 1
	y1 -= 1
	x2 += nodeWidth
	y2 += nodeHeight

	style := StyleZoneBorder
	if !opts.NoColor {
		if ec, ok := ElementColor[zb.element]; ok {
			style = lipgloss.NewStyle().Faint(true).Foreground(ec)
		}
	} else {
		style = lipgloss.NewStyle()
	}

	// Top + bottom
	for x := x1; x <= x2; x++ {
		c.set(x, y1, '─', style)
		c.set(x, y2, '─', style)
	}
	// Left + right
	for y := y1; y <= y2; y++ {
		c.set(x1, y, '│', style)
		c.set(x2, y, '│', style)
	}
	// Corners
	c.set(x1, y1, '┌', style)
	c.set(x2, y1, '┐', style)
	c.set(x1, y2, '└', style)
	c.set(x2, y2, '┘', style)

	// Zone label
	label := fmt.Sprintf(" %s ", zb.name)
	c.putString(x1+2, y1, label, style)
}
