package sumi

import (
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"

	tea "github.com/charmbracelet/bubbletea"
)

// --- TX/RX data types ---

func TestTxRxLog_PushAndAll(t *testing.T) {
	log := view.NewTxRxLog(5)
	now := time.Now()

	log.Push(view.TxRxEntry{Timestamp: now, Walker: "w1", Direction: view.TxDirection, Node: "recall", Content: "prompt"})
	log.Push(view.TxRxEntry{Timestamp: now, Walker: "w1", Direction: view.RxDirection, Node: "recall", Content: "response"})

	all := log.All()
	if len(all) != 2 {
		t.Fatalf("Len = %d, want 2", len(all))
	}
	if all[0].Direction != view.TxDirection || all[1].Direction != view.RxDirection {
		t.Error("directions should be tx, rx")
	}
}

func TestTxRxLog_Overflow(t *testing.T) {
	log := view.NewTxRxLog(3)
	now := time.Now()
	for i := 0; i < 5; i++ {
		log.Push(view.TxRxEntry{Timestamp: now, Content: string(rune('a' + i))})
	}
	if log.Len() != 3 {
		t.Fatalf("Len = %d, want 3", log.Len())
	}
	all := log.All()
	if all[0].Content != "c" || all[2].Content != "e" {
		t.Errorf("overflow entries: [%s, %s, %s], want [c, d, e]", all[0].Content, all[1].Content, all[2].Content)
	}
}

func TestTxRxLog_ForWalker(t *testing.T) {
	log := view.NewTxRxLog(10)
	now := time.Now()
	log.Push(view.TxRxEntry{Timestamp: now, Walker: "w1", Content: "a"})
	log.Push(view.TxRxEntry{Timestamp: now, Walker: "w2", Content: "b"})
	log.Push(view.TxRxEntry{Timestamp: now, Walker: "w1", Content: "c"})

	filtered := log.ForWalker("w1")
	if len(filtered) != 2 {
		t.Fatalf("filtered len = %d, want 2", len(filtered))
	}
}

func TestTxRxLog_LastTxRx(t *testing.T) {
	log := view.NewTxRxLog(10)
	now := time.Now()
	log.Push(view.TxRxEntry{Timestamp: now, Walker: "w1", Direction: view.TxDirection, Content: "p1"})
	log.Push(view.TxRxEntry{Timestamp: now, Walker: "w1", Direction: view.RxDirection, Content: "r1"})
	log.Push(view.TxRxEntry{Timestamp: now, Walker: "w1", Direction: view.TxDirection, Content: "p2"})

	tx, rx := log.LastTxRx("w1")
	if tx == nil || tx.Content != "p2" {
		t.Errorf("last TX = %v, want p2", tx)
	}
	if rx == nil || rx.Content != "r1" {
		t.Errorf("last RX = %v, want r1", rx)
	}
}

// --- TX/RX panel ---

func TestTxRxPanel_Interface(t *testing.T) {
	log := view.NewTxRxLog(10)
	p := NewTxRxPanel(log, true)
	var _ Panel = p
	if p.ID() != "txrx" {
		t.Errorf("ID = %q, want txrx", p.ID())
	}
	if !p.Focusable() {
		t.Error("TxRx panel should be focusable")
	}
}

func TestTxRxPanel_EmptyState(t *testing.T) {
	log := view.NewTxRxLog(10)
	p := NewTxRxPanel(log, true)

	content := p.View(Rect{0, 0, 50, 20})
	if !strings.Contains(content, "waiting for data") {
		t.Errorf("empty TX/RX should show waiting message, got: %s", content)
	}
}

func TestTxRxPanel_ShowsEntries(t *testing.T) {
	log := view.NewTxRxLog(10)
	log.Push(view.TxRxEntry{Timestamp: time.Now(), Walker: "w1", Direction: view.TxDirection, Content: "Hello prompt"})
	log.Push(view.TxRxEntry{Timestamp: time.Now(), Walker: "w1", Direction: view.RxDirection, Content: "Hello response"})

	p := NewTxRxPanel(log, true)
	content := p.View(Rect{0, 0, 60, 20})

	if !strings.Contains(content, "TX") {
		t.Error("should show TX section")
	}
	if !strings.Contains(content, "RX") {
		t.Error("should show RX section")
	}
	if !strings.Contains(content, "Hello prompt") {
		t.Error("should show prompt content")
	}
	if !strings.Contains(content, "Hello response") {
		t.Error("should show response content")
	}
}

// --- Case result data types ---

func TestCaseResultSet_AddAndByID(t *testing.T) {
	s := view.NewCaseResultSet()
	s.Add(view.CaseResult{CaseID: "c1", DefectType: "env", Confidence: 0.85, Status: "pass"})
	s.Add(view.CaseResult{CaseID: "c2", DefectType: "code", Confidence: 0.72, Status: "fail"})

	if s.Len() != 2 {
		t.Fatalf("Len = %d, want 2", s.Len())
	}

	c := s.ByID("c1")
	if c == nil || c.DefectType != "env" {
		t.Errorf("ByID(c1) = %v, want env", c)
	}

	c = s.ByID("nonexistent")
	if c != nil {
		t.Error("ByID for unknown ID should return nil")
	}
}

func TestCaseResultSet_AddReplace(t *testing.T) {
	s := view.NewCaseResultSet()
	s.Add(view.CaseResult{CaseID: "c1", Status: "pending"})
	s.Add(view.CaseResult{CaseID: "c1", Status: "pass"})

	if s.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (replaced)", s.Len())
	}
	if s.Cases[0].Status != "pass" {
		t.Errorf("status = %q, want pass (replaced)", s.Cases[0].Status)
	}
}

func TestCaseResults_InSnapshot(t *testing.T) {
	snap := view.CircuitSnapshot{
		CaseResults: []view.CaseResult{
			{CaseID: "c1", Status: "pass"},
			{CaseID: "c2", Status: "fail"},
		},
	}
	if len(snap.CaseResults) != 2 {
		t.Fatalf("CaseResults len = %d, want 2", len(snap.CaseResults))
	}
}

// --- Cases panel ---

func TestCasesPanel_Interface(t *testing.T) {
	cases := view.NewCaseResultSet()
	p := NewCasesPanel(cases, true)
	var _ Panel = p
	if p.ID() != "cases" {
		t.Errorf("ID = %q, want cases", p.ID())
	}
	if !p.Focusable() {
		t.Error("Cases panel should be focusable")
	}
}

func TestCasesPanel_EmptyState(t *testing.T) {
	cases := view.NewCaseResultSet()
	p := NewCasesPanel(cases, true)

	content := p.View(Rect{0, 0, 80, 5})
	if !strings.Contains(content, "No case results") {
		t.Errorf("empty cases should show placeholder, got: %s", content)
	}
}

func TestCasesPanel_RendersTabs(t *testing.T) {
	cases := view.NewCaseResultSet()
	cases.Add(view.CaseResult{CaseID: "c1", Status: "pass", DefectType: "env", Confidence: 0.85, Summary: "Environment issue"})
	cases.Add(view.CaseResult{CaseID: "c2", Status: "fail", DefectType: "code", Confidence: 0.72, Summary: "Code bug"})

	p := NewCasesPanel(cases, true)
	content := p.View(Rect{0, 0, 80, 5})

	if !strings.Contains(content, "c1") {
		t.Error("should show case c1")
	}
	if !strings.Contains(content, "c2") {
		t.Error("should show case c2")
	}
}

func TestCasesPanel_NavigateWithKeys(t *testing.T) {
	cases := view.NewCaseResultSet()
	cases.Add(view.CaseResult{CaseID: "c1"})
	cases.Add(view.CaseResult{CaseID: "c2"})

	p := NewCasesPanel(cases, true)
	if p.SelectedCase() != "c1" {
		t.Fatalf("initial selection = %q, want c1", p.SelectedCase())
	}

	p.Update(tea.KeyMsg{Type: tea.KeyRight})
	if p.SelectedCase() != "c2" {
		t.Errorf("after right = %q, want c2", p.SelectedCase())
	}

	p.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if p.SelectedCase() != "c1" {
		t.Errorf("after left = %q, want c1", p.SelectedCase())
	}
}

// --- Sub-circuit navigation ---

func TestBreadcrumbBar_Visible(t *testing.T) {
	def := testCircuit()
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	bb := NewBreadcrumbBar("Root", def, layout, true)
	if bb.Visible() {
		t.Error("breadcrumb should be hidden at root")
	}
	if bb.Depth() != 1 {
		t.Errorf("depth = %d, want 1", bb.Depth())
	}
}

func TestBreadcrumbBar_PushAndPop(t *testing.T) {
	def := testCircuit()
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	bb := NewBreadcrumbBar("Root", def, layout, true)

	subDef := &circuit.CircuitDef{Circuit: "sub"}
	bb.Push("SubCircuit", subDef, layout)

	if !bb.Visible() {
		t.Error("breadcrumb should be visible after push")
	}
	if bb.Depth() != 2 {
		t.Errorf("depth = %d, want 2", bb.Depth())
	}
	if bb.Current().Label != "SubCircuit" {
		t.Errorf("current = %q, want SubCircuit", bb.Current().Label)
	}

	popped := bb.Pop()
	if popped == nil {
		t.Fatal("pop should return the popped entry")
	}
	if popped.Label != "SubCircuit" {
		t.Errorf("popped = %q, want SubCircuit", popped.Label)
	}
	if bb.Visible() {
		t.Error("breadcrumb should be hidden after pop to root")
	}
}

func TestBreadcrumbBar_PopAtRootReturnsNil(t *testing.T) {
	def := testCircuit()
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	bb := NewBreadcrumbBar("Root", def, layout, true)
	if bb.Pop() != nil {
		t.Error("pop at root should return nil")
	}
}

func TestBreadcrumbBar_PopTo(t *testing.T) {
	def := testCircuit()
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	bb := NewBreadcrumbBar("Root", def, layout, true)
	bb.Push("L1", def, layout)
	bb.Push("L2", def, layout)

	entry := bb.PopTo(0)
	if entry == nil || entry.Label != "Root" {
		t.Errorf("PopTo(0) should return root, got %v", entry)
	}
	if bb.Depth() != 1 {
		t.Errorf("depth after PopTo(0) = %d, want 1", bb.Depth())
	}
}

func TestBreadcrumbBar_View(t *testing.T) {
	def := testCircuit()
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	bb := NewBreadcrumbBar("Root", def, layout, true)
	bb.Push("Sub", def, layout)

	v := bb.View(80)
	if !strings.Contains(v, "Root") || !strings.Contains(v, "Sub") {
		t.Errorf("view should contain crumbs, got: %s", v)
	}
	if !strings.Contains(v, ">") {
		t.Errorf("view should contain separator, got: %s", v)
	}
}

func TestBreadcrumbBar_CrumbAtX(t *testing.T) {
	def := testCircuit()
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	bb := NewBreadcrumbBar("Root", def, layout, true)
	bb.Push("Sub", def, layout)

	idx := bb.CrumbAtX(0)
	if idx != 0 {
		t.Errorf("CrumbAtX(0) = %d, want 0 (Root)", idx)
	}

	// "Root" = 4 chars, " > " = 3 chars, "Sub" starts at position 7
	idx = bb.CrumbAtX(7)
	if idx != 1 {
		t.Errorf("CrumbAtX(7) = %d, want 1 (Sub)", idx)
	}

	idx = bb.CrumbAtX(100)
	if idx != -1 {
		t.Errorf("CrumbAtX(100) = %d, want -1 (miss)", idx)
	}
}

func TestDrillDownRequest_Type(t *testing.T) {
	req := DrillDownRequest{
		ParentNode: "marble-node",
		CircuitDef: testCircuit(),
	}
	if req.ParentNode != "marble-node" {
		t.Errorf("ParentNode = %q, want marble-node", req.ParentNode)
	}
}

// --- Layout accommodation ---

func TestLayout_AccommodatesExtensionPanels(t *testing.T) {
	reg := NewPanelRegistry()
	log := view.NewTxRxLog(10)
	cases := view.NewCaseResultSet()

	reg.Register(&stubPanel{id: "graph", focusable: true}, SlotCenter)
	reg.Register(&stubPanel{id: "inspector", focusable: true}, SlotRightSidebar)
	reg.Register(&stubPanel{id: "timeline", focusable: true}, SlotBottom)
	reg.Register(NewTxRxPanel(log, true), SlotLeftSidebar)
	reg.Register(NewCasesPanel(cases, true), SlotBottomTabs)

	layout := ComputeLayout(160, 50)
	for _, e := range reg.All() {
		rect := layout.RectFor(e.Slot)
		if rect.W == 0 && rect.H == 0 && e.Slot != SlotLeftSidebar && e.Slot != SlotBottomTabs {
			continue
		}
		content := e.Panel.View(rect)
		if content == "" && rect.W > 2 && rect.H > 2 {
			t.Errorf("panel %s rendered empty in %v", e.Panel.ID(), rect)
		}
	}
}
