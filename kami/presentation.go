package kami

import (
	"encoding/json"
	"net/http"
)

// KabukiConfig provides section data for the Kabuki presentation engine.
// Consumers implement this interface to turn the Kami frontend into a branded,
// section-based presentation SPA. Methods that return nil skip that section.
//
// Kabuki is the presentation layer; Kami is the debugger. Agent intros and
// circuit nodes are derived from Theme (not duplicated here).
type KabukiConfig interface {
	Hero() *HeroSection
	Problem() *ProblemSection
	Results() *ResultsSection
	Competitive() []Competitor
	Architecture() *ArchitectureSection
	Roadmap() []Milestone
	Closing() *ClosingSection
	TransitionLine() string

	// SectionOrder returns the ordered section IDs for the presentation.
	// The frontend renders sections in this order instead of hardcoding.
	// Return nil to use the default order.
	SectionOrder() []string

	// CodeShowcases returns code blocks for DSL/code showcase sections.
	// Return nil if no code showcases are needed.
	CodeShowcases() []CodeShowcase

	// Concepts returns concept card groups for educational sections.
	// Return nil if no concept sections are needed.
	Concepts() []ConceptGroup
}

// HeroSection is the full-viewport opening slide.
type HeroSection struct {
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	Presenter string `json:"presenter,omitempty"`
	Logo      string `json:"logo,omitempty"`
	Framework string `json:"framework,omitempty"`
}

// ProblemSection describes the pain points being addressed.
type ProblemSection struct {
	Title      string   `json:"title"`
	Narrative  string   `json:"narrative"`
	BulletPoints []string `json:"bullet_points"`
	Stat       string   `json:"stat,omitempty"`
	StatLabel  string   `json:"stat_label,omitempty"`
}

// Metric is a single result data point with a 0-1 value.
type Metric struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Color string  `json:"color,omitempty"`
}

// SummaryCard is a small stat card (e.g. "19/21 Metrics passing").
type SummaryCard struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Color string `json:"color,omitempty"`
}

// ResultsSection presents calibration or benchmark outcomes.
type ResultsSection struct {
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Metrics     []Metric      `json:"metrics"`
	Summary     []SummaryCard `json:"summary,omitempty"`
}

// Competitor is one row in a competitive comparison table.
type Competitor struct {
	Name      string            `json:"name"`
	Fields    map[string]string `json:"fields"`
	Highlight bool              `json:"highlight,omitempty"`
}

// ArchitectureSection describes the system architecture.
type ArchitectureSection struct {
	Title      string          `json:"title"`
	Components []ArchComponent `json:"components"`
	Footer     string          `json:"footer,omitempty"`
}

// ArchComponent is a box in the architecture diagram.
type ArchComponent struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color,omitempty"`
}

// Milestone is a point on the roadmap timeline.
type Milestone struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"` // "done", "current", "future"
}

// ClosingSection is the final slide.
type ClosingSection struct {
	Headline string   `json:"headline"`
	Tagline  string   `json:"tagline,omitempty"`
	Lines    []string `json:"lines,omitempty"`
}

// CodeBlock is a syntax-highlighted code sample with optional annotation.
type CodeBlock struct {
	Language   string `json:"language"`
	Code       string `json:"code"`
	Annotation string `json:"annotation,omitempty"`
}

// CodeShowcase is a named group of code blocks for a showcase section.
type CodeShowcase struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Blocks []CodeBlock `json:"blocks"`
}

// ConceptCard is a single card in a concept grid.
type ConceptCard struct {
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description"`
	Color       string `json:"color,omitempty"`
}

// ConceptGroup is a titled group of concept cards.
type ConceptGroup struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Subtitle string        `json:"subtitle,omitempty"`
	Cards    []ConceptCard `json:"cards"`
}

// kabukiPayload is the JSON envelope for /api/kabuki.
type kabukiPayload struct {
	Hero           *HeroSection         `json:"hero,omitempty"`
	Problem        *ProblemSection      `json:"problem,omitempty"`
	Results        *ResultsSection      `json:"results,omitempty"`
	Competitive    []Competitor         `json:"competitive,omitempty"`
	Architecture   *ArchitectureSection `json:"architecture,omitempty"`
	Roadmap        []Milestone          `json:"roadmap,omitempty"`
	Closing        *ClosingSection      `json:"closing,omitempty"`
	TransitionLine string               `json:"transition_line,omitempty"`
	SectionOrder   []string             `json:"section_order,omitempty"`
	CodeShowcases  []CodeShowcase       `json:"code_showcases,omitempty"`
	Concepts       []ConceptGroup       `json:"concepts,omitempty"`
}

// themePayload is the JSON envelope for /api/theme.
type themePayload struct {
	Name               string            `json:"name"`
	AgentIntros        []AgentIntro      `json:"agent_intros"`
	NodeDescriptions   map[string]string `json:"node_descriptions"`
	CostumeAssets      map[string]string `json:"costume_assets"`
	CooperationDialogs []Dialog          `json:"cooperation_dialogs"`
}

// circuitPayload is the JSON envelope for /api/circuit.
type circuitPayload struct {
	Nodes map[string]string `json:"nodes"`
}

func (s *Server) handleThemeAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cfg.Theme == nil {
		_ = json.NewEncoder(w).Encode(themePayload{})
		return
	}
	_ = json.NewEncoder(w).Encode(themePayload{
		Name:               s.cfg.Theme.Name(),
		AgentIntros:        s.cfg.Theme.AgentIntros(),
		NodeDescriptions:   s.vocabOverlay(s.cfg.Theme.NodeDescriptions()),
		CostumeAssets:      s.cfg.Theme.CostumeAssets(),
		CooperationDialogs: s.cfg.Theme.CooperationDialogs(),
	})
}

func (s *Server) handleCircuitAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cfg.Theme == nil {
		_ = json.NewEncoder(w).Encode(circuitPayload{})
		return
	}
	_ = json.NewEncoder(w).Encode(circuitPayload{
		Nodes: s.vocabOverlay(s.cfg.Theme.NodeDescriptions()),
	})
}

// vocabOverlay enriches node descriptions with RichVocabulary descriptions
// when available. Falls back to Theme-provided descriptions for unknown nodes.
func (s *Server) vocabOverlay(base map[string]string) map[string]string {
	if s.cfg.Vocab == nil {
		return base
	}
	out := make(map[string]string, len(base))
	for code, desc := range base {
		if d := s.cfg.Vocab.Description(code); d != "" {
			out[code] = d
		} else {
			out[code] = desc
		}
	}
	return out
}

func (s *Server) handleKabukiAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cfg.Kabuki == nil {
		_ = json.NewEncoder(w).Encode(kabukiPayload{})
		return
	}
	p := s.cfg.Kabuki
	_ = json.NewEncoder(w).Encode(kabukiPayload{
		Hero:           p.Hero(),
		Problem:        p.Problem(),
		Results:        p.Results(),
		Competitive:    p.Competitive(),
		Architecture:   p.Architecture(),
		Roadmap:        p.Roadmap(),
		Closing:        p.Closing(),
		TransitionLine: p.TransitionLine(),
		SectionOrder:   p.SectionOrder(),
		CodeShowcases:  p.CodeShowcases(),
		Concepts:       p.Concepts(),
	})
}
