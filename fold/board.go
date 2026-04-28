package fold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/circuit/def"
	"gopkg.in/yaml.v3"
)

// BoardManifest is the parsed representation of a board YAML file.
// Flat structure — no K8s metadata/spec nesting. The board file IS the
// entry point: `tako assemble boards/ci-analysis.yaml`.
type BoardManifest struct {
	Kind        circuit.Kind      `yaml:"kind"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Uses        map[string]string `yaml:"uses,omitempty"`
	Bind        map[string]string `yaml:"bind,omitempty"`
	Domain      string            `yaml:"domain,omitempty"`
	Schema      string            `yaml:"schema,omitempty"`
	Scorecard   string            `yaml:"scorecard,omitempty"`
	Report      string            `yaml:"report,omitempty"`
	Prompts     map[string]string `yaml:"prompts,omitempty"`
	Instruments map[string]string `yaml:"instruments,omitempty"` // name → manifest path
	Compose     *ComposeRef       `yaml:"compose,omitempty"`
	Circuit     *CircuitBlock     `yaml:"circuit,omitempty"`
	Calibration *CalibBlock       `yaml:"calibration,omitempty"`
	Serve       *ServeBlock       `yaml:"serve,omitempty"`
	Params      []ParamDef        `yaml:"params,omitempty"`
}

// ComposeRef references another board for composition.
type ComposeRef struct {
	Base string `yaml:"base"`
}

// CircuitBlock holds the circuit composition section of a board.
// Import lists schematics whose circuits are composed. The remaining
// fields are passthrough YAML nodes — parsed by the engine, not fold.
type CircuitBlock struct {
	Import []string  `yaml:"import,omitempty"`
	Zones  yaml.Node `yaml:"zones,omitempty"`
	Nodes  yaml.Node `yaml:"nodes,omitempty"`
	Edges  yaml.Node `yaml:"edges,omitempty"`
	Wiring yaml.Node `yaml:"wiring,omitempty"`
	Ports  yaml.Node `yaml:"ports,omitempty"`
}

// CalibBlock holds calibration contract fields. Passthrough to calibrate/.
type CalibBlock struct {
	AdapterFields []string  `yaml:"adapter_fields,omitempty"`
	Outputs       yaml.Node `yaml:"outputs,omitempty"`
}

// ServeBlock configures the generated binary's HTTP server.
type ServeBlock struct {
	Port int `yaml:"port"`
}

// ParseBoardManifest unmarshals a flat board YAML file.
func ParseBoardManifest(data []byte) (*BoardManifest, error) {
	var bm BoardManifest
	if err := yaml.Unmarshal(data, &bm); err != nil {
		return nil, fmt.Errorf("parse board: %w", err)
	}
	if bm.Kind != circuit.KindBoard {
		return nil, fmt.Errorf("%w: expected %q, got %q", ErrBoardKindMismatch, circuit.KindBoard, bm.Kind)
	}
	if bm.Name == "" {
		return nil, ErrBoardNameRequired
	}
	return &bm, nil
}

// LoadBoardManifest reads and parses a board YAML file from disk.
func LoadBoardManifest(path string) (*BoardManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load board %s: %w", path, err)
	}
	return ParseBoardManifest(data)
}

// ValidateBoardPaths checks that every path referenced by the board exists
// and has the expected kind: header.
func ValidateBoardPaths(bm *BoardManifest, boardDir string) error {
	checks := make(map[string]circuit.Kind)
	if bm.Schema != "" {
		checks[bm.Schema] = circuit.KindStoreSchema
	}
	if bm.Scorecard != "" {
		checks[bm.Scorecard] = circuit.KindScorecard
	}
	if bm.Report != "" {
		checks[bm.Report] = circuit.KindReportTemplate
	}

	for relPath, expectedKind := range checks {
		absPath := filepath.Join(boardDir, relPath)
		if err := validateFileKindGraceful(absPath, expectedKind); err != nil {
			return fmt.Errorf("board %s: path %s: %w", bm.Name, relPath, err)
		}
	}
	return nil
}

// validateFileKindGraceful validates a file's kind header. Returns nil if the
// file doesn't exist (optional path) or has no kind header (migration).
// Returns error only on kind mismatch.
func validateFileKindGraceful(path string, expected circuit.Kind) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // file may not exist — optional
	}
	env, err := def.ParseEnvelope(data)
	if err != nil {
		return nil // unparseable — skip
	}
	if env.Kind == "" {
		return nil // no kind header — migration
	}
	if env.Kind != expected {
		return fmt.Errorf("%w: expected %q, got %q", ErrDomainKindMismatch, expected, env.Kind)
	}
	return nil
}
