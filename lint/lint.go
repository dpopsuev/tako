// Package lint provides rule-based static analysis for Origami circuit YAML.
// It catches structural, semantic, and best-practice issues before runtime.
//
// The linter is designed for embedding: lint.Run() works without CLI, filesystem,
// or IO. The origami-lsp will embed this engine for editor-time feedback.
package lint

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	framework "github.com/dpopsuev/origami"
	"gopkg.in/yaml.v3"
)

// Severity levels for lint findings, ordered by importance.
type Severity int

const (
	SeverityError   Severity = 0
	SeverityWarning Severity = 1
	SeverityInfo    Severity = 2
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// Finding represents a single lint diagnostic.
type Finding struct {
	RuleID       string   `json:"rule_id"`
	Severity     Severity `json:"severity"`
	Message      string   `json:"message"`
	File         string   `json:"file,omitempty"`
	Line         int      `json:"line,omitempty"`
	Column       int      `json:"column,omitempty"`
	Suggestion   string   `json:"suggestion,omitempty"`
	FixAvailable bool     `json:"fix_available,omitempty"`
}

func (f Finding) String() string {
	var b strings.Builder
	if f.File != "" {
		b.WriteString(f.File)
		if f.Line > 0 {
			fmt.Fprintf(&b, ":%d", f.Line)
			if f.Column > 0 {
				fmt.Fprintf(&b, ":%d", f.Column)
			}
		}
		b.WriteString(": ")
	}
	fmt.Fprintf(&b, "%s: %s: %s", f.Severity, f.RuleID, f.Message)
	if f.Suggestion != "" {
		fmt.Fprintf(&b, " (%s)", f.Suggestion)
	}
	return b.String()
}

// Fix describes an auto-fix replacement for a finding.
type Fix struct {
	Finding     Finding `json:"finding"`
	Replacement string  `json:"replacement"`
	StartLine   int     `json:"start_line"`
	EndLine     int     `json:"end_line"`
}

// Rule is the interface every lint rule implements.
type Rule interface {
	ID() string
	Description() string
	Severity() Severity
	Tags() []string
	Check(ctx *LintContext) []Finding
}

// Fixable is an optional interface for rules that can auto-fix findings.
type Fixable interface {
	Rule
	Fix(ctx *LintContext) []Fix
}

// PromptFieldError describes an invalid field reference in a prompt template.
type PromptFieldError struct {
	Field   string
	Message string
}

// PromptValidator checks a prompt template's field references and returns
// errors for any that cannot be resolved against the expected parameter type.
// Implementations are provided by domain modules (e.g. modules/rca).
type PromptValidator func(content string) []PromptFieldError

// LintContext holds all data available to lint rules during checking.
type LintContext struct {
	Def             *framework.CircuitDef
	Raw             []byte
	File            string
	Registries      *framework.GraphRegistries
	PromptFS        fs.FS
	PromptValidator PromptValidator

	// ProjectFiles holds all YAML files in the project indexed by kind.
	// Populated when linting in project mode (directory or multiple files).
	// Cross-reference rules use this for multi-file validation.
	ProjectFiles map[string][]ProjectFile

	yamlRoot     *yaml.Node
	fieldLineMap map[string]int
}

// NewLintContext creates a LintContext from raw YAML bytes.
// It parses both the typed CircuitDef and the raw yaml.Node tree
// for line-number resolution.
func NewLintContext(raw []byte, file string) (*LintContext, error) {
	def, err := framework.LoadCircuit(raw)
	if err != nil {
		return nil, fmt.Errorf("parse circuit: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse YAML node tree: %w", err)
	}

	ctx := &LintContext{
		Def:  def,
		Raw:  raw,
		File: file,
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		ctx.yamlRoot = doc.Content[0]
	}
	ctx.fieldLineMap = buildFieldLineMap(ctx.yamlRoot)
	return ctx, nil
}

// NewGenericLintContext creates a LintContext from raw YAML bytes without
// requiring circuit parsing. Only the raw yaml.Node tree is available;
// Def is nil. Rules that need Def should check for nil and return early.
func NewGenericLintContext(raw []byte, file string) (*LintContext, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	ctx := &LintContext{
		Raw:  raw,
		File: file,
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		ctx.yamlRoot = doc.Content[0]
	}
	ctx.fieldLineMap = buildFieldLineMap(ctx.yamlRoot)
	return ctx, nil
}

// NewLintContextFromDef creates a LintContext from an already-parsed CircuitDef.
// Line numbers are unavailable (all zero).
func NewLintContextFromDef(def *framework.CircuitDef, file string) *LintContext {
	return &LintContext{Def: def, File: file}
}

// NodeLine returns the YAML source line for a node by name.
func (c *LintContext) NodeLine(name string) int {
	return c.fieldLineMap["node:"+name]
}

// PortLine returns the YAML source line for a port by name.
func (c *LintContext) PortLine(name string) int {
	return c.fieldLineMap["port:"+name]
}

// CalibrationFieldLine returns the YAML source line for a calibration input or output by index.
// section is "input" or "output", idx is the 0-based index.
func (c *LintContext) CalibrationFieldLine(section string, idx int) int {
	return c.fieldLineMap[fmt.Sprintf("calibration:%s:%d", section, idx)]
}

// EdgeLine returns the YAML source line for an edge by ID.
func (c *LintContext) EdgeLine(id string) int {
	return c.fieldLineMap["edge:"+id]
}

// WalkerLine returns the YAML source line for a walker by name.
func (c *LintContext) WalkerLine(name string) int {
	return c.fieldLineMap["walker:"+name]
}

// TopLevelLine returns the YAML source line for a top-level key.
func (c *LintContext) TopLevelLine(key string) int {
	return c.fieldLineMap["top:"+key]
}

// buildFieldLineMap walks the yaml.Node tree and maps DSL entities to
// their source line numbers. Keys follow the pattern "node:<name>",
// "edge:<id>", "walker:<name>", "top:<key>".
func buildFieldLineMap(root *yaml.Node) map[string]int {
	m := make(map[string]int)
	if root == nil || root.Kind != yaml.MappingNode {
		return m
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		val := root.Content[i+1]
		m["top:"+key.Value] = key.Line

		switch key.Value {
		case "nodes":
			mapSequenceByField(val, "name", "node:", m)
			mapInlineEdges(val, m)
		case "edges":
			mapSequenceByField(val, "id", "edge:", m)
		case "walkers":
			mapSequenceByField(val, "name", "walker:", m)
		case "ports":
			mapSequenceByField(val, "name", "port:", m)
		case "calibration":
			mapCalibrationFields(val, m)
		}
	}
	return m
}

// mapInlineEdges indexes edges nested under nodes[].edges in compact DSL form.
// Replicates the same ID generation as generateEdgeID: {from}-{name} or {from}-{to}.
func mapInlineEdges(nodesSeq *yaml.Node, m map[string]int) {
	if nodesSeq == nil || nodesSeq.Kind != yaml.SequenceNode {
		return
	}
	seen := make(map[string]int)
	for _, nodeItem := range nodesSeq.Content {
		if nodeItem.Kind != yaml.MappingNode {
			continue
		}
		var nodeName string
		var edgesNode *yaml.Node
		for j := 0; j+1 < len(nodeItem.Content); j += 2 {
			switch nodeItem.Content[j].Value {
			case "name":
				nodeName = nodeItem.Content[j+1].Value
			case "edges":
				edgesNode = nodeItem.Content[j+1]
			}
		}
		if nodeName == "" || edgesNode == nil || edgesNode.Kind != yaml.SequenceNode {
			continue
		}
		for _, edgeItem := range edgesNode.Content {
			var id string
			switch edgeItem.Kind {
			case yaml.ScalarNode:
				id = inlineEdgeID(nodeName, "", edgeItem.Value, seen)
			case yaml.MappingNode:
				var eName, eTo string
				for j := 0; j+1 < len(edgeItem.Content); j += 2 {
					switch edgeItem.Content[j].Value {
					case "name":
						eName = edgeItem.Content[j+1].Value
					case "to":
						eTo = edgeItem.Content[j+1].Value
					}
				}
				id = inlineEdgeID(nodeName, eName, eTo, seen)
			}
			if id != "" {
				m["edge:"+id] = edgeItem.Line
			}
		}
	}
}

func inlineEdgeID(from, name, to string, seen map[string]int) string {
	base := from + "-" + to
	if name != "" {
		base = from + "-" + strings.ReplaceAll(name, " ", "-")
	}
	seen[base]++
	if seen[base] > 1 {
		return fmt.Sprintf("%s-%d", base, seen[base])
	}
	return base
}

// mapSequenceByField extracts line numbers from a YAML sequence of mappings.
// Each mapping's identityField value is used as the key suffix.
func mapSequenceByField(seq *yaml.Node, identityField, prefix string, m map[string]int) {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			if item.Content[j].Value == identityField {
				name := item.Content[j+1].Value
				m[prefix+name] = item.Line
				break
			}
		}
	}
}

// mapCalibrationFields indexes calibration inputs and outputs by sequence index.
func mapCalibrationFields(calNode *yaml.Node, m map[string]int) {
	if calNode == nil || calNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(calNode.Content); i += 2 {
		key := calNode.Content[i]
		val := calNode.Content[i+1]
		if key.Value != "inputs" && key.Value != "outputs" {
			continue
		}
		prefix := "calibration:" + key.Value + ":"
		if val == nil || val.Kind != yaml.SequenceNode {
			continue
		}
		for idx, item := range val.Content {
			if item != nil {
				m[prefix+fmt.Sprintf("%d", idx)] = item.Line
			}
		}
	}
}

// Profile defines a set of severity levels included in a lint run.
type Profile string

const (
	ProfileMin      Profile = "min"
	ProfileBasic    Profile = "basic"
	ProfileModerate Profile = "moderate"
	ProfileStrict   Profile = "strict"
)

func (p Profile) maxSeverity() Severity {
	switch p {
	case ProfileMin:
		return SeverityError
	case ProfileBasic:
		return SeverityWarning
	case ProfileModerate:
		return SeverityWarning
	case ProfileStrict:
		return SeverityInfo
	default:
		return SeverityWarning
	}
}

// Option configures a lint run.
type Option func(*runConfig)

type runConfig struct {
	profile         Profile
	tags            []string
	registries      *framework.GraphRegistries
	promptFS        fs.FS
	promptValidator PromptValidator
	projectFiles    map[string][]ProjectFile
}

// WithProfile sets the lint profile.
func WithProfile(p Profile) Option {
	return func(c *runConfig) { c.profile = p }
}

// WithTags filters rules to those matching any of the given tags.
func WithTags(tags ...string) Option {
	return func(c *runConfig) { c.tags = tags }
}

// WithRegistries provides graph registries for deeper semantic checks.
func WithRegistries(reg *framework.GraphRegistries) Option {
	return func(c *runConfig) { c.registries = reg }
}

// WithPromptFS provides an fs.FS containing prompt templates for the
// P1/template-param-validity rule. Typically rca.DefaultPromptFS.
func WithPromptFS(fsys fs.FS) Option {
	return func(c *runConfig) { c.promptFS = fsys }
}

// WithPromptValidator provides a callback that validates prompt template
// field references. The callback is domain-specific; the lint package
// does not import domain modules.
func WithPromptValidator(v PromptValidator) Option {
	return func(c *runConfig) { c.promptValidator = v }
}

// WithProjectFiles provides parsed YAML files indexed by kind for
// cross-file validation rules (S30+). Built from LoadProjectFiles.
func WithProjectFiles(files map[string][]ProjectFile) Option {
	return func(c *runConfig) { c.projectFiles = files }
}

// Runner holds a set of rules and executes them against circuit definitions.
type Runner struct {
	rules []Rule
}

// NewRunner creates a Runner with the given rules.
func NewRunner(rules ...Rule) *Runner {
	return &Runner{rules: rules}
}

// DefaultRunner creates a Runner pre-loaded with all built-in rules.
func DefaultRunner() *Runner {
	return NewRunner(AllRules()...)
}

// Run executes all matching rules against a LintContext and returns findings
// sorted by severity (errors first) then line number.
func (r *Runner) Run(ctx *LintContext, opts ...Option) []Finding {
	cfg := runConfig{profile: ProfileModerate}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.registries != nil {
		ctx.Registries = cfg.registries
	}
	if cfg.promptFS != nil {
		ctx.PromptFS = cfg.promptFS
	}
	if cfg.promptValidator != nil {
		ctx.PromptValidator = cfg.promptValidator
	}
	if cfg.projectFiles != nil {
		ctx.ProjectFiles = cfg.projectFiles
	}

	maxSev := cfg.profile.maxSeverity()
	tagSet := make(map[string]bool, len(cfg.tags))
	for _, t := range cfg.tags {
		tagSet[t] = true
	}

	envelopeOnly := map[string]bool{"envelope": true}

	var findings []Finding
	for _, rule := range r.rules {
		if rule.Severity() > maxSev {
			continue
		}
		if len(tagSet) > 0 && !matchesTags(rule.Tags(), tagSet) {
			continue
		}
		if ctx.Def == nil && !matchesTags(rule.Tags(), envelopeOnly) {
			continue
		}
		findings = append(findings, rule.Check(ctx)...)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity < findings[j].Severity
		}
		return findings[i].Line < findings[j].Line
	})
	return findings
}

func matchesTags(ruleTags []string, want map[string]bool) bool {
	for _, t := range ruleTags {
		if want[t] {
			return true
		}
	}
	return false
}

// Run is the package-level entry point for embedding.
// It parses raw YAML, runs all built-in rules, and returns findings.
// For circuit files (kind: circuit or no kind), all rules run.
// For non-circuit YAML (kind is set but not "circuit"), only envelope rules run.
func Run(raw []byte, file string, opts ...Option) ([]Finding, error) {
	env, _ := framework.ParseEnvelope(raw)
	isCircuit := env == nil || env.Kind == "" || env.Kind == "circuit"

	if isCircuit {
		ctx, err := NewLintContext(raw, file)
		if err != nil {
			ctx, err = NewGenericLintContext(raw, file)
			if err != nil {
				return nil, err
			}
		}
		runner := DefaultRunner()
		return runner.Run(ctx, opts...), nil
	}

	ctx, err := NewGenericLintContext(raw, file)
	if err != nil {
		return nil, err
	}
	runner := DefaultRunner()
	return runner.Run(ctx, opts...), nil
}

// HasErrors returns true if any finding has Error severity.
func HasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any finding has Warning severity or higher.
func HasWarnings(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity <= SeverityWarning {
			return true
		}
	}
	return false
}
