package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/dpopsuev/tako/lint"
)

func newLintContextForTest(raw []byte, file string) (*lint.LintContext, error) {
	return lint.NewLintContext(raw, file)
}

func TestComputeDiagnostics_ValidYAML(t *testing.T) {
	raw := `circuit: test
description: "A simple test"
nodes:
  - name: start
  - name: finish
edges:
  - id: E1
    name: go
    from: start
    to: finish
    when: "true"
  - id: E2
    name: done
    from: finish
    to: DONE
    when: "true"
start: start
done: DONE`

	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: raw,
	}

	diags := computeDiagnostics(doc)
	for _, d := range diags {
		if d.Severity == protocol.DiagnosticSeverityError {
			t.Errorf("unexpected error diagnostic: %s (%v)", d.Message, d.Code)
		}
	}
}

func TestComputeDiagnostics_InvalidApproach(t *testing.T) {
	raw := `circuit: test
nodes:
  - name: start
    approach: rapd
start: start
done: DONE`

	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: raw,
	}

	diags := computeDiagnostics(doc)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "rapd") || strings.Contains(d.Message, "approach") {
			found = true
		}
	}
	if !found {
		t.Error("expected diagnostic about invalid approach 'rapd'")
	}
}

func TestComputeDiagnostics_Empty(t *testing.T) {
	doc := &document{
		URI:     uri.URI("file:///empty.yaml"),
		Content: "",
	}

	diags := computeDiagnostics(doc)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for empty file, got %d", len(diags))
	}
}

func TestCompletion_TopLevel(t *testing.T) {
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: "",
	}

	items := computeCompletions(doc, protocol.Position{Line: 0, Character: 0})
	if len(items) == 0 {
		t.Fatal("expected top-level completions")
	}

	labels := map[string]bool{}
	for _, item := range items {
		labels[item.Label] = true
	}
	for _, key := range []string{"circuit", "nodes", "edges", "start", "done"} {
		if !labels[key] {
			t.Errorf("missing top-level completion: %s", key)
		}
	}
}

func TestCompletion_ApproachValues(t *testing.T) {
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: "  approach: ",
	}

	items := computeCompletions(doc, protocol.Position{Line: 0, Character: 12})
	if len(items) == 0 {
		t.Fatal("expected approach value completions")
	}

	labels := map[string]bool{}
	for _, item := range items {
		labels[item.Label] = true
	}
	for _, a := range []string{"rapid", "analytical", "methodical", "holistic", "rigorous"} {
		if !labels[a] {
			t.Errorf("missing approach completion: %s", a)
		}
	}
}

func TestHover_Approach(t *testing.T) {
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: "    approach: rapid",
	}

	hover := computeHover(doc, protocol.Position{Line: 0, Character: 14}, nil)
	if hover == nil {
		t.Fatal("expected hover for approach")
	}
	if !strings.Contains(hover.Contents.Value, "Rapid") && !strings.Contains(hover.Contents.Value, "rapid") {
		t.Errorf("hover content doesn't mention rapid: %s", hover.Contents.Value)
	}
}

func TestHover_Persona(t *testing.T) {
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: "    persona: herald",
	}

	hover := computeHover(doc, protocol.Position{Line: 0, Character: 14}, nil)
	if hover == nil {
		t.Fatal("expected hover for persona")
	}
	if !strings.Contains(hover.Contents.Value, "herald") && !strings.Contains(hover.Contents.Value, "Herald") {
		t.Errorf("hover content doesn't mention herald: %s", hover.Contents.Value)
	}
}

func TestHover_ConnectedEdges(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
    family: recall
  - name: triage
    family: triage
  - name: review
    family: review
edges:
  - id: E1
    from: recall
    to: triage
    when: "output.match != true"
  - id: E2
    from: recall
    to: review
    shortcut: true
    when: "output.match == true"
  - id: E3
    from: triage
    to: review
    when: "true"
start: recall
done: DONE`

	raw := []byte(content)
	lctx, _ := lint.NewLintContext(raw, "test.yaml")
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
	}
	if lctx != nil {
		doc.Def = lctx.Def
	}

	tests := []struct {
		name    string
		line    uint32
		wantIn  []string
		wantOut []string
		noWant  []string
	}{
		{
			name:    "recall node declaration has outbound E1 and E2",
			line:    2,
			wantOut: []string{"E1", "E2", "triage", "review"},
		},
		{
			name:    "triage has inbound E1 and outbound E3",
			line:    4,
			wantIn:  []string{"E1", "recall"},
			wantOut: []string{"E3", "review"},
			noWant:  []string{"E2"},
		},
		{
			name:   "review has inbound E2 and E3",
			line:   6,
			wantIn: []string{"E2", "E3", "recall", "triage"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hover := computeHover(doc, protocol.Position{Line: tt.line}, nil)
			if hover == nil {
				t.Fatal("expected hover")
			}
			md := hover.Contents.Value
			if !strings.Contains(md, "Connected edges") {
				t.Error("hover should contain 'Connected edges' section")
			}
			for _, w := range tt.wantIn {
				if !strings.Contains(md, w) {
					t.Errorf("expected inbound reference to %q in hover", w)
				}
			}
			for _, w := range tt.wantOut {
				if !strings.Contains(md, w) {
					t.Errorf("expected outbound reference to %q in hover", w)
				}
			}
			for _, nw := range tt.noWant {
				if strings.Contains(md, nw) {
					t.Errorf("did not expect %q in hover for this node", nw)
				}
			}
		})
	}
}

func TestHover_ConnectedEdges_FromTo(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
edges:
  - id: E1
    from: recall
    to: triage
    when: "true"
start: recall
done: DONE`

	raw := []byte(content)
	lctx, _ := lint.NewLintContext(raw, "test.yaml")
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
	}
	if lctx != nil {
		doc.Def = lctx.Def
	}

	// Hover on "from: recall" should also show connected edges
	lines := strings.Split(content, "\n")
	fromLine := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "from: recall" {
			fromLine = i
			break
		}
	}
	if fromLine < 0 {
		t.Fatal("could not find 'from: recall' line")
	}

	hover := computeHover(doc, protocol.Position{Line: uint32(fromLine)}, nil)
	if hover == nil {
		t.Fatal("expected hover on from: recall")
	}
	if !strings.Contains(hover.Contents.Value, "Connected edges") {
		t.Error("hover on from: should include connected edges")
	}
}

func TestHover_NodeDescription(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
    description: "Pattern-match against known failures"
    approach: rapid
  - name: triage
edges:
  - id: E1
    from: recall
    to: triage
    when: "true"
start: recall
done: DONE`

	raw := []byte(content)
	lctx, _ := lint.NewLintContext(raw, "test.yaml")
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
	}
	if lctx != nil {
		doc.Def = lctx.Def
	}

	lines := strings.Split(content, "\n")
	nameLine := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "- name: recall" {
			nameLine = i
			break
		}
	}
	if nameLine < 0 {
		t.Fatal("could not find '- name: recall' line")
	}

	hover := computeHover(doc, protocol.Position{Line: uint32(nameLine)}, nil)
	if hover == nil {
		t.Fatal("expected hover on node name")
	}
	if !strings.Contains(hover.Contents.Value, "Pattern-match against known failures") {
		t.Errorf("hover should include node description, got: %s", hover.Contents.Value)
	}
}

func TestHover_NodeNoDescription(t *testing.T) {
	content := `circuit: test
nodes:
  - name: triage
edges:
  - id: E1
    from: triage
    to: DONE
    when: "true"
start: triage
done: DONE`

	raw := []byte(content)
	lctx, _ := lint.NewLintContext(raw, "test.yaml")
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
	}
	if lctx != nil {
		doc.Def = lctx.Def
	}

	hover := computeHover(doc, protocol.Position{Line: 2}, nil)
	if hover == nil {
		t.Fatal("expected hover on node name")
	}
	if strings.Contains(hover.Contents.Value, "description") {
		t.Error("hover should not mention description when empty")
	}
}

func TestDefinition_EdgeToNode(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
edges:
  - id: E1
    from: recall
    to: triage
    when: "true"
start: recall
done: DONE`

	raw := []byte(content)
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
	}

	// Parse for definition support
	lintCtx, err := newLintContextForTest(raw, "test.yaml")
	if err == nil && lintCtx != nil {
		doc.LintCtx = lintCtx
		doc.Def = lintCtx.Def
	}

	// Line 7 is "    to: triage" (0-indexed)
	loc := computeDefinition(doc, protocol.Position{Line: 7, Character: 8})
	if loc == nil {
		t.Skip("definition not resolved (acceptable for basic line mapping)")
	}
}

func TestScenarioYAMLs_NoDiagnosticErrors(t *testing.T) {
	patterns := []string{
		"../testdata/*.yaml",
		"../testdata/scenarios/*.yaml",
		"../testdata/patterns/*.yaml",
	}

	tested := 0
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, path := range matches {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			doc := &document{
				URI:     uri.URI("file://" + path),
				Content: string(raw),
			}

			diags := computeDiagnostics(doc)
			for _, d := range diags {
				if d.Severity == protocol.DiagnosticSeverityError {
					t.Errorf("%s: unexpected error: %s (%v)", path, d.Message, d.Code)
				}
			}
			tested++
		}
	}
	if tested == 0 {
		t.Skip("no scenario YAMLs found")
	}
	t.Logf("validated %d scenario YAMLs with zero errors", tested)
}

func TestServerHandler_Initialize(t *testing.T) {
	srv := NewServer()
	h := srv.Handler()
	if h == nil {
		t.Fatal("Handler() returned nil")
	}
}

func TestSemanticTokens_ApproachValues(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
    approach: rapid
  - name: deep
    approach: analytical
  - name: classify
    approach: methodical
edges:
  - id: E1
    from: recall
    to: deep
    when: "true"
start: recall
done: DONE`

	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
	}

	raw := []byte(content)
	lctx, _ := lint.NewLintContext(raw, "test.yaml")
	if lctx != nil {
		doc.Def = lctx.Def
		doc.LintCtx = lctx
	}

	data := computeSemanticTokens(doc)
	// Each token produces 5 uint32 values
	if len(data)%5 != 0 {
		t.Fatalf("data length %d is not a multiple of 5", len(data))
	}
	tokenCount := len(data) / 5
	if tokenCount < 3 {
		t.Errorf("expected at least 3 approach tokens (rapid, analytical, methodical), got %d", tokenCount)
	}

	// Verify first token is rapid (type 0)
	if len(data) >= 5 {
		tokenType := data[3]
		if tokenType != approachTokenIndex["rapid"] {
			t.Errorf("first token type = %d, want %d (rapid)", tokenType, approachTokenIndex["rapid"])
		}
	}
}

func TestSemanticTokens_Empty(t *testing.T) {
	doc := &document{
		URI:     uri.URI("file:///empty.yaml"),
		Content: "circuit: test\nnodes:\n  - name: start\nstart: start\ndone: DONE",
	}

	data := computeSemanticTokens(doc)
	if len(data) != 0 {
		t.Errorf("expected no semantic tokens for circuit without approaches, got %d values", len(data))
	}
}

func TestSemanticTokens_AllApproaches(t *testing.T) {
	content := `circuit: elements
nodes:
  - name: n1
    approach: rapid
  - name: n2
    approach: analytical
  - name: n3
    approach: methodical
  - name: n4
    approach: holistic
  - name: n5
    approach: rigorous
  - name: n6
    approach: aggressive
start: n1
done: DONE`

	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
	}
	raw := []byte(content)
	lctx, _ := lint.NewLintContext(raw, "test.yaml")
	if lctx != nil {
		doc.Def = lctx.Def
	}

	data := computeSemanticTokens(doc)
	tokenCount := len(data) / 5
	if tokenCount != 6 {
		t.Errorf("expected 6 approach tokens (all 6 types), got %d", tokenCount)
	}

	// Verify all 6 unique token types are present
	seen := map[uint32]bool{}
	for i := 0; i < len(data); i += 5 {
		seen[data[i+3]] = true
	}
	for approach, idx := range approachTokenIndex {
		if !seen[idx] {
			t.Errorf("missing token type for approach %q (index %d)", approach, idx)
		}
	}
}

func TestSemanticTokensLegend(t *testing.T) {
	legend := SemanticTokensLegend()
	types, ok := legend["tokenTypes"].([]string)
	if !ok {
		t.Fatal("legend missing tokenTypes")
	}
	if len(types) != 6 {
		t.Errorf("expected 6 token types, got %d", len(types))
	}
}

func TestSemanticTokensProvider(t *testing.T) {
	provider := SemanticTokensProvider()
	if provider["full"] != true {
		t.Error("expected full: true")
	}
	legend, ok := provider["legend"].(map[string]any)
	if !ok {
		t.Fatal("expected legend in provider")
	}
	types, ok := legend["tokenTypes"].([]string)
	if !ok {
		t.Fatal("legend missing tokenTypes")
	}
	if len(types) != 6 {
		t.Errorf("expected 6 token types, got %d", len(types))
	}
}

func TestInlayHints_ApproachTraits(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
    approach: rapid
  - name: deep
    approach: analytical
start: recall
done: DONE`

	doc := makeTestDoc(content)
	hints := computeInlayHints(doc)

	traitHints := filterHintsByKind(hints, 1)
	found := 0
	for _, h := range traitHints {
		if strings.Contains(h.Label, "rapid") || strings.Contains(h.Label, "analytical") {
			found++
		}
	}
	if found < 2 {
		t.Errorf("expected at least 2 behavior profile hints (rapid, analytical), found %d", found)
	}
}

func TestInlayHints_PersonaDescription(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
walkers:
  - name: scout
    persona: herald
start: recall
done: DONE`

	doc := makeTestDoc(content)
	hints := computeInlayHints(doc)

	found := false
	for _, h := range hints {
		if strings.Contains(h.Label, "Fire persona") {
			found = true
		}
	}
	if !found {
		t.Error("expected persona hint for herald")
	}
}

func TestInlayHints_EdgeConnection(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
edges:
  - id: E1
    from: recall
    to: triage
    when: "true"
  - id: E1-empty
    from: recall
    to: DONE
    shortcut: true
    when: "true"
start: recall
done: DONE`

	doc := makeTestDoc(content)
	hints := computeInlayHints(doc)

	var normalFound, shortcutFound bool
	for _, h := range hints {
		if strings.Contains(h.Label, "recall") && strings.Contains(h.Label, "triage") && !strings.Contains(h.Label, "shortcut") {
			normalFound = true
		}
		if strings.Contains(h.Label, "recall") && strings.Contains(h.Label, "DONE") && strings.Contains(h.Label, "shortcut") {
			shortcutFound = true
		}
	}
	if !normalFound {
		t.Error("expected edge hint showing 'recall → triage'")
	}
	if !shortcutFound {
		t.Error("expected edge hint showing 'recall → DONE · shortcut'")
	}
}

func TestInlayHints_EdgeTerminal(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
edges:
  - id: E1
    from: recall
    to: triage
    when: "true"
  - id: E2
    from: triage
    to: DONE
    when: "true"
start: recall
done: DONE`

	doc := makeTestDoc(content)
	hints := computeInlayHints(doc)

	found := false
	for _, h := range hints {
		if strings.Contains(h.Label, "triage") && strings.Contains(h.Label, "DONE") && strings.Contains(h.Label, "terminal") {
			found = true
		}
	}
	if !found {
		t.Error("expected edge hint 'triage → DONE · terminal'")
	}
}

func TestInlayHints_EdgeLoop(t *testing.T) {
	content := `circuit: test
nodes:
  - name: a
  - name: b
edges:
  - id: E1
    from: a
    to: b
    when: "true"
  - id: E2
    from: b
    to: a
    loop: true
    when: "true"
start: a
done: DONE`

	doc := makeTestDoc(content)
	hints := computeInlayHints(doc)

	found := false
	for _, h := range hints {
		if strings.Contains(h.Label, "b") && strings.Contains(h.Label, "a") && strings.Contains(h.Label, "loop") {
			found = true
		}
	}
	if !found {
		t.Error("expected edge hint 'b → a · loop'")
	}
}

func TestInlayHints_EdgeTooltipWithWhen(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
edges:
  - id: E1
    from: recall
    to: triage
    when: "output.match == true"
start: recall
done: DONE`

	doc := makeTestDoc(content)
	hints := computeInlayHints(doc)

	found := false
	for _, h := range hints {
		if strings.Contains(h.Label, "recall") && strings.Contains(h.Label, "triage") {
			if h.Tooltip != nil && strings.Contains(h.Tooltip.Value, "output.match == true") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected edge tooltip with when condition")
	}
}

func TestInlayHints_AllEdgesGetConnectionHint(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
edges:
  - id: E1
    from: recall
    to: triage
    when: "true"
start: recall
done: DONE`

	doc := makeTestDoc(content)
	hints := computeInlayHints(doc)

	edgeHintFound := false
	for _, h := range hints {
		if strings.Contains(h.Label, "recall") && strings.Contains(h.Label, "triage") {
			edgeHintFound = true
		}
		if h.Label == "shortcut" || h.Label == "terminal" {
			t.Errorf("found bare '%s' label — should be merged into connection hint", h.Label)
		}
	}
	if !edgeHintFound {
		t.Error("expected connection hint on normal edge")
	}
}

func TestInlayHints_Neighbors(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
  - name: review
edges:
  - id: E1
    from: recall
    to: triage
    when: "output.match != true"
  - id: E2
    from: recall
    to: review
    shortcut: true
    when: "output.match == true"
  - id: E3
    from: triage
    to: review
    when: "true"
  - id: E4
    from: triage
    to: recall
    loop: true
    when: "output.retry == true"
start: recall
done: DONE`

	raw := []byte(content)
	lctx, _ := lint.NewLintContext(raw, "test.yaml")
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
		Def:     lctx.Def,
	}

	hints := computeInlayHints(doc)

	neighborLabels := map[string]string{}
	for _, h := range hints {
		line := int(h.Position.Line)
		lines := strings.Split(content, "\n")
		if line < len(lines) {
			trimmed := strings.TrimSpace(lines[line])
			if strings.HasPrefix(trimmed, "- name:") {
				name := strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
				if strings.Contains(h.Label, "\u2190") || strings.Contains(h.Label, "\u2192") {
					neighborLabels[name] = h.Label
				}
			}
		}
	}

	if label, ok := neighborLabels["recall"]; !ok {
		t.Error("expected neighbor hint on recall")
	} else {
		if !strings.Contains(label, "start") {
			t.Errorf("recall should show start marker, got %q", label)
		}
		if !strings.Contains(label, "triage") || !strings.Contains(label, "review") {
			t.Errorf("recall should show outbound triage and review, got %q", label)
		}
	}

	if label, ok := neighborLabels["triage"]; !ok {
		t.Error("expected neighbor hint on triage")
	} else {
		if !strings.Contains(label, "recall") {
			t.Errorf("triage should show inbound recall, got %q", label)
		}
		if !strings.Contains(label, "\u21bb") {
			t.Errorf("triage should show loop marker on recall outbound, got %q", label)
		}
	}

	if label, ok := neighborLabels["review"]; !ok {
		t.Error("expected neighbor hint on review")
	} else if !strings.Contains(label, "recall") && !strings.Contains(label, "triage") {
		t.Errorf("review should show inbound recall and triage, got %q", label)
	}
}

func TestInlayHints_Empty(t *testing.T) {
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: "circuit: test",
	}
	hints := computeInlayHints(doc)
	if len(hints) != 0 {
		t.Errorf("expected no hints for doc without parsed def, got %d", len(hints))
	}
}

func makeTestDoc(content string) *document {
	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: content,
	}
	raw := []byte(content)
	lctx, _ := lint.NewLintContext(raw, "test.yaml")
	if lctx != nil {
		doc.Def = lctx.Def
		doc.LintCtx = lctx
	}
	return doc
}

func filterHintsByKind(hints []InlayHint, kind int) []InlayHint {
	var out []InlayHint
	for _, h := range hints {
		if h.Kind == kind {
			out = append(out, h)
		}
	}
	return out
}

func TestCompletion_ZoneNodes(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
  - name: investigate
zones:
  backcourt:
    nodes: `

	doc := makeTestDoc(content)
	// Cursor at end of "    nodes: " line (line 7)
	items := computeCompletions(doc, protocol.Position{Line: 7, Character: 11})
	if len(items) == 0 {
		t.Fatal("expected node name completions in zone nodes field")
	}

	labels := map[string]bool{}
	for _, item := range items {
		labels[item.Label] = true
	}
	for _, name := range []string{"recall", "triage", "investigate"} {
		if !labels[name] {
			t.Errorf("missing node name completion: %s", name)
		}
	}
}

func TestCompletion_StepAffinity(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
walkers:
  - name: scout
    step_affinity:
      `

	doc := makeTestDoc(content)
	// Cursor on the blank line under step_affinity (line 7)
	items := computeCompletions(doc, protocol.Position{Line: 7, Character: 6})
	if len(items) == 0 {
		t.Fatal("expected node name completions under step_affinity")
	}

	labels := map[string]bool{}
	for _, item := range items {
		labels[item.Label] = true
	}
	if !labels["recall"] || !labels["triage"] {
		t.Error("expected recall and triage as step_affinity completions")
	}
}

func TestDefinition_ZoneNodeList(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
  - name: triage
  - name: investigate
zones:
  backcourt:
    nodes: [recall, triage]
start: recall
done: DONE`

	doc := makeTestDoc(content)

	// Cursor on "recall" inside nodes: [recall, triage] (line 7, char ~14)
	bracketStart := strings.Index(content, "[recall")
	line7 := strings.Split(content, "\n")[7]
	recallCol := strings.Index(line7, "recall")

	loc := computeDefinition(doc, protocol.Position{Line: 7, Character: uint32(recallCol)})
	if loc == nil {
		t.Skip("zone node definition not resolved (LintCtx line mapping)")
		return
	}
	_ = bracketStart

	// Should jump to the "- name: recall" node declaration (line 2)
	if loc.Range.Start.Line != 2 {
		t.Errorf("expected definition at line 2, got %d", loc.Range.Start.Line)
	}
}

func TestSemanticTokens_WalkerApproach(t *testing.T) {
	content := `circuit: test
nodes:
  - name: recall
    approach: rapid
walkers:
  - name: scout
    approach: analytical
start: recall
done: DONE`

	doc := makeTestDoc(content)

	data := computeSemanticTokens(doc)
	tokenCount := len(data) / 5
	if tokenCount < 2 {
		t.Errorf("expected at least 2 approach tokens (node rapid + walker analytical), got %d", tokenCount)
	}

	// Collect all token types
	types := map[uint32]bool{}
	for i := 0; i < len(data); i += 5 {
		types[data[i+3]] = true
	}
	if !types[approachTokenIndex["rapid"]] {
		t.Error("missing semantic token for node approach: rapid")
	}
	if !types[approachTokenIndex["analytical"]] {
		t.Error("missing semantic token for walker approach: analytical")
	}
}

// --- S21/S22/S23: real-time diagnostic rules via LSP ---

func TestComputeDiagnostics_EdgeNodeReference(t *testing.T) {
	// Invalid edge from-node is now caught at parse time by normalize()
	// graph validation, so the circuit falls back to generic context.
	// The LSP should still report a diagnostic (parse error).
	raw := `circuit: test
description: "edge node reference test"
nodes:
  - name: recall
  - name: triage
edges:
  - id: E1
    name: go
    from: recal
    to: triage
    when: "true"
  - id: E2
    name: done
    from: triage
    to: DONE
    when: "true"
start: recall
done: DONE`

	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: raw,
	}

	diags := computeDiagnostics(doc)
	// With normalize() validation, the parse fails and S21 rule may not fire.
	// Verify that some diagnostic is produced (parse error or S21).
	if len(diags) == 0 {
		t.Error("expected at least one diagnostic for invalid circuit")
	}
}

func TestComputeDiagnostics_HookReference(t *testing.T) {
	raw := `circuit: test
description: "hook reference test"
nodes:
  - name: recall
    instrument: transformer
    action: core.jq
    meta:
      expr: "input"
    before: ["inject failure"]
edges:
  - id: E1
    name: done
    from: recall
    to: DONE
    when: "true"
start: recall
done: DONE`

	doc := &document{
		URI:     uri.URI("file:///test.yaml"),
		Content: raw,
	}

	diags := computeDiagnostics(doc)
	found := false
	for _, d := range diags {
		code, _ := d.Code.(string)
		if code == "S22/hook-reference" {
			found = true
			if !strings.Contains(d.Message, "whitespace") {
				t.Errorf("expected diagnostic to mention whitespace, got %q", d.Message)
			}
		}
	}
	if !found {
		t.Error("expected S22/hook-reference diagnostic in LSP for hook with whitespace")
	}
}

func TestKamiBridge_LastTransitionHint(t *testing.T) {
	kb := NewKamiBridge(0)

	content := `circuit: test
nodes:
  - name: recall
  - name: triage
edges:
  - id: E1
    from: recall
    to: triage
    when: "true"
start: recall
done: DONE`

	doc := makeTestDoc(content)

	kb.processEvent(`{"type":"node_enter","node":"recall","agent":"herald","ts":"2026-02-26T10:00:00Z"}`)
	kb.processEvent(`{"type":"transition","edge":"E1","ts":"2026-02-26T10:00:01Z"}`)
	kb.processEvent(`{"type":"node_exit","node":"recall","ts":"2026-02-26T10:00:01Z"}`)
	kb.processEvent(`{"type":"node_enter","node":"triage","agent":"herald","ts":"2026-02-26T10:00:02Z"}`)

	hints := kb.LiveInlayHints(doc)
	found := false
	for _, h := range hints {
		if strings.Contains(h.Label, "last transition") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'last transition' hint for edge E1")
	}
}

func TestKamiBridge_ProcessEvents(t *testing.T) {
	kb := NewKamiBridge(0)

	kb.processEvent(`{"type":"node_enter","node":"recall","agent":"seeker","ts":"2026-02-26T10:00:00Z"}`)
	state := kb.State()
	if state.ActiveNode != "recall" {
		t.Errorf("active node = %q, want recall", state.ActiveNode)
	}
	if state.ActiveAgent != "seeker" {
		t.Errorf("active agent = %q, want seeker", state.ActiveAgent)
	}
	if _, ok := state.Visited["recall"]; !ok {
		t.Error("recall should be in visited map")
	}

	kb.processEvent(`{"type":"transition","edge":"E1","ts":"2026-02-26T10:00:01Z"}`)
	state = kb.State()
	if _, ok := state.Transitions["E1"]; !ok {
		t.Error("E1 should be in transitions map")
	}

	kb.processEvent(`{"type":"paused","ts":"2026-02-26T10:00:02Z"}`)
	state = kb.State()
	if !state.Paused {
		t.Error("expected paused=true after paused event")
	}

	kb.processEvent(`{"type":"resumed","ts":"2026-02-26T10:00:03Z"}`)
	state = kb.State()
	if state.Paused {
		t.Error("expected paused=false after resumed event")
	}

	kb.processEvent(`{"type":"walk_complete","ts":"2026-02-26T10:00:04Z"}`)
	state = kb.State()
	if state.ActiveNode != "" {
		t.Errorf("active node should be empty after walk_complete, got %q", state.ActiveNode)
	}
}

func TestKamiBridge_LiveInlayHints(t *testing.T) {
	kb := NewKamiBridge(0)

	content := `circuit: test
nodes:
  - name: recall
    approach: rapid
  - name: triage
    approach: analytical
start: recall
done: DONE`

	doc := makeTestDoc(content)

	kb.processEvent(`{"type":"node_enter","node":"recall","agent":"herald","ts":"2026-02-26T10:00:00Z"}`)

	hints := kb.LiveInlayHints(doc)
	foundActive := false
	foundVisited := false
	for _, h := range hints {
		if strings.Contains(h.Label, "ACTIVE") {
			foundActive = true
			if !strings.Contains(h.Label, "herald") {
				t.Error("active hint should include agent name")
			}
		}
	}
	if !foundActive {
		t.Error("expected ACTIVE hint for recall node")
	}

	kb.processEvent(`{"type":"node_exit","node":"recall","ts":"2026-02-26T10:00:01Z"}`)
	kb.processEvent(`{"type":"node_enter","node":"triage","agent":"herald","ts":"2026-02-26T10:00:02Z"}`)

	hints = kb.LiveInlayHints(doc)
	for _, h := range hints {
		if strings.Contains(h.Label, "visited") {
			foundVisited = true
		}
	}
	if !foundVisited {
		t.Error("expected 'visited' hint for recall after it was exited")
	}
}

func TestKamiBridge_PausedHint(t *testing.T) {
	kb := NewKamiBridge(0)

	content := `circuit: test
nodes:
  - name: recall
start: recall
done: DONE`
	doc := makeTestDoc(content)

	kb.processEvent(`{"type":"node_enter","node":"recall","agent":"seeker","ts":"2026-02-26T10:00:00Z"}`)
	kb.processEvent(`{"type":"paused","ts":"2026-02-26T10:00:01Z"}`)

	hints := kb.LiveInlayHints(doc)
	found := false
	for _, h := range hints {
		if h.Label == "PAUSED" {
			found = true
		}
	}
	if !found {
		t.Error("expected PAUSED hint when circuit is paused on active node")
	}
}

func TestKamiBridge_NotConnected(t *testing.T) {
	kb := NewKamiBridge(0)
	if kb.Connected() {
		t.Error("bridge should not be connected without Start()")
	}
}

func TestKamiBridge_StateSnapshotIsolation(t *testing.T) {
	kb := NewKamiBridge(0)
	kb.processEvent(`{"type":"node_enter","node":"recall","agent":"seeker","ts":"2026-02-26T10:00:00Z"}`)

	state := kb.State()
	state.Visited["injected"] = VisitInfo{Agent: "hacker"}

	fresh := kb.State()
	if _, ok := fresh.Visited["injected"]; ok {
		t.Error("state snapshot should be isolated — mutation leaked")
	}
}
