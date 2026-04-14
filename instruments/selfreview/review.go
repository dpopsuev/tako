// Package selfreview validates that code changes address every requirement
// in a Scribe artifact. It reads the artifact via tool.Registry (MCPAdapter),
// greps modified files for evidence, and attaches stamps back to Scribe.
package selfreview

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/origami/engine/handler"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

// Sentinel errors.
var (
	ErrNoTaskID  = errors.New("self-review: no task_id found in walker state or config")
	ErrScribeGet = errors.New("self-review: failed to read Scribe artifact")
)

// Stamp status values.
const (
	StampVerified   = "verified"
	StampUnverified = "unverified"
)

const scribeToolName = "scribe.artifact"

// SelfReviewTransformer validates code changes against Scribe artifact
// requirements. For each requirement field (title, goal, sections), it
// greps the modified files for evidence and stamps them as verified or not.
type SelfReviewTransformer struct {
	registry *tool.Registry
	repoPath string

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// New creates a SelfReviewTransformer.
// registry must contain a "scribe.artifact" tool (via MCPAdapter).
func New(registry *tool.Registry, repoPath string) *SelfReviewTransformer {
	return &SelfReviewTransformer{
		registry: registry,
		repoPath: repoPath,
	}
}

// Name implements handler.Transformer.
func (s *SelfReviewTransformer) Name() string { return "self-review" }

// LastStationLog implements handler.StationLoggable.
func (s *SelfReviewTransformer) LastStationLog() trace.StationLogger {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastStationLog
}

// Transform implements handler.Transformer.
func (s *SelfReviewTransformer) Transform(ctx context.Context, tc *handler.TransformerContext) (any, error) {
	// 1. Resolve task ID from walker state or config.
	taskID := findTaskID(tc)
	if taskID == "" {
		return nil, ErrNoTaskID
	}

	// 2. Read Scribe artifact.
	art, err := s.getScribeArtifact(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrScribeGet, err)
	}

	// 3. Extract verifiable requirements.
	requirements := extractRequirements(art)

	// 4. Get modified files from prior fix output in walker state.
	modifiedFiles := findModifiedFiles(tc)

	// 5. Verify each requirement against modified files.
	stamps := make([]sdlctype.Stamp, 0, len(requirements))
	allVerified := true
	for _, req := range requirements {
		evidence := s.findEvidence(req.text, modifiedFiles)
		status := StampVerified
		if evidence == "" {
			status = StampUnverified
			allVerified = false
		}
		stamps = append(stamps, sdlctype.Stamp{
			Field:    req.field,
			Status:   status,
			Evidence: evidence,
		})
	}

	// 6. Attach stamps to Scribe artifact.
	stampsAttached := false
	if attachErr := s.attachStamps(ctx, taskID, stamps); attachErr == nil {
		stampsAttached = true
	}

	// 7. Record station log.
	verified := 0
	for _, st := range stamps {
		if st.Status == StampVerified {
			verified++
		}
	}
	s.mu.Lock()
	s.lastStationLog = &sdlctype.SelfReviewStationLog{
		RequirementsTotal:    len(requirements),
		RequirementsVerified: verified,
		AllVerified:          allVerified,
		StampsAttached:       stampsAttached,
	}
	s.mu.Unlock()

	return &sdlctype.SelfReviewResult{
		AllVerified: allVerified,
		Stamps:      stamps,
	}, nil
}

// --- Internal types ---

type requirement struct {
	field string
	text  string
}

// scribeArtifact is the subset of a Scribe artifact relevant to self-review.
// Handles both array and map section formats (real Scribe vs ToyScribeStore).
type scribeArtifact struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Goal     string          `json:"goal"`
	Sections json.RawMessage `json:"sections"`
}

// parsedSections handles both Scribe formats:
//   - real Scribe: [{"name":"context","text":"..."}]
//   - ToyScribeStore: {"context":"..."}
func (a *scribeArtifact) parsedSections() []requirement {
	// Try array format first (real Scribe).
	var arrSections []struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	if json.Unmarshal(a.Sections, &arrSections) == nil && len(arrSections) > 0 {
		reqs := make([]requirement, 0, len(arrSections))
		for _, sec := range arrSections {
			if sec.Text != "" {
				reqs = append(reqs, requirement{field: sec.Name, text: sec.Text})
			}
		}
		return reqs
	}

	// Try map format (ToyScribeStore).
	var mapSections map[string]string
	if json.Unmarshal(a.Sections, &mapSections) == nil {
		reqs := make([]requirement, 0, len(mapSections))
		for name, text := range mapSections {
			if text != "" {
				reqs = append(reqs, requirement{field: name, text: text})
			}
		}
		return reqs
	}

	return nil
}

// --- Requirement extraction ---

func extractRequirements(art *scribeArtifact) []requirement {
	var reqs []requirement
	if art.Title != "" {
		reqs = append(reqs, requirement{field: "title", text: art.Title})
	}
	if art.Goal != "" {
		reqs = append(reqs, requirement{field: "goal", text: art.Goal})
	}
	reqs = append(reqs, art.parsedSections()...)
	return reqs
}

// --- Task ID resolution ---

// findTaskID searches config, node extras, and walker state outputs for a task_id.
func findTaskID(tc *handler.TransformerContext) string {
	// Circuit vars.
	if tc.Config != nil {
		if id, ok := tc.Config["task_id"].(string); ok && id != "" {
			return id
		}
	}
	// Node config extra.
	if tc.NodeConfig != nil && tc.NodeConfig.Extra != nil {
		if id, ok := tc.NodeConfig.Extra["task_id"].(string); ok && id != "" {
			return id
		}
	}
	// Walk all prior node outputs.
	if tc.WalkerState != nil {
		for _, art := range tc.WalkerState.Outputs {
			if m, ok := art.Raw().(map[string]any); ok {
				if id, ok := m["task_id"].(string); ok && id != "" {
					return id
				}
			}
		}
	}
	return ""
}

// --- Modified file extraction ---

// findModifiedFiles extracts file paths from the prior fix output in walker state.
func findModifiedFiles(tc *handler.TransformerContext) []string {
	if tc.WalkerState == nil {
		return nil
	}
	for _, art := range tc.WalkerState.Outputs {
		raw := art.Raw()
		if fr, ok := raw.(*sdlctype.FixResult); ok {
			return fr.Fixed
		}
		if m, ok := raw.(map[string]any); ok {
			if fixed, ok := m["fixed"]; ok {
				return toStringSlice(fixed)
			}
		}
	}
	return nil
}

func toStringSlice(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// --- Evidence search ---

// findEvidence searches modified files for keywords from the requirement text.
// Returns "file:line" if found, empty string otherwise.
func (s *SelfReviewTransformer) findEvidence(reqText string, modifiedFiles []string) string {
	if len(modifiedFiles) == 0 {
		return ""
	}
	keywords := extractKeywords(reqText)
	if len(keywords) == 0 {
		return ""
	}
	for _, filePath := range modifiedFiles {
		fullPath := filePath
		if !filepath.IsAbs(filePath) {
			fullPath = filepath.Join(s.repoPath, filePath)
		}
		if line, found := grepFileForKeywords(fullPath, keywords); found {
			return fmt.Sprintf("%s:%d", filePath, line)
		}
	}
	return ""
}

// stopWords are filtered out from requirement text before keyword extraction.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"for": true, "and": true, "but": true, "or": true, "nor": true,
	"not": true, "so": true, "yet": true, "to": true, "of": true,
	"in": true, "on": true, "at": true, "by": true, "with": true,
	"from": true, "up": true, "about": true, "into": true, "through": true,
	"this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true,
}

// extractKeywords splits requirement text into meaningful search terms (3+ chars, no stop words).
func extractKeywords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}") // strip punctuation
		if len(w) >= 3 && !stopWords[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

// grepFileForKeywords returns the first line number where any keyword is found.
func grepFileForKeywords(path string, keywords []string) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.ToLower(scanner.Text())
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				return lineNum, true
			}
		}
	}
	return 0, false
}

// --- Scribe interaction ---

func (s *SelfReviewTransformer) getScribeArtifact(ctx context.Context, id string) (*scribeArtifact, error) {
	input, _ := json.Marshal(map[string]any{
		"action": "get",
		"id":     id,
	})
	result, err := s.registry.Execute(ctx, scribeToolName, input)
	if err != nil {
		return nil, err
	}
	var art scribeArtifact
	if err := json.Unmarshal([]byte(result), &art); err != nil {
		return nil, fmt.Errorf("parse artifact: %w", err)
	}
	return &art, nil
}

func (s *SelfReviewTransformer) attachStamps(ctx context.Context, id string, stamps []sdlctype.Stamp) error {
	stampsJSON, _ := json.Marshal(stamps)
	input, _ := json.Marshal(map[string]any{
		"action": "attach_section",
		"id":     id,
		"name":   "stamps",
		"text":   string(stampsJSON),
	})
	_, err := s.registry.Execute(ctx, scribeToolName, input)
	return err
}

// Compile-time interface checks.
var (
	_ handler.Transformer     = (*SelfReviewTransformer)(nil)
	_ handler.StationLoggable = (*SelfReviewTransformer)(nil)
)
