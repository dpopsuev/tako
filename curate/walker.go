package curate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/origami/circuit"
)

const (
	nodeFetch    = "fetch"
	nodeExtract  = "extract"
	nodeValidate = "validate"
	nodeEnrich   = "enrich"
)

// CurationWalker is a circuit.Walker that walks the curation circuit.
// It uses configured EvidenceSources and Extractors to fetch raw data, extract
// fields, validate against a Schema, and promote complete records.
type CurationWalker struct {
	identity   circuit.AgentIdentity
	state      *circuit.WalkerState
	schema     Schema
	sources    []EvidenceSource
	extractors []Extractor
	record     Record
	promoted   bool
}

// CurationWalkerConfig holds the configuration for constructing a CurationWalker.
type CurationWalkerConfig struct {
	RecordID      string
	Schema        Schema
	Sources       []EvidenceSource
	Extractors    []Extractor
	InitialRecord *Record
}

// NewCurationWalker creates a walker configured with sources, extractors,
// and a schema for validation.
func NewCurationWalker(cfg *CurationWalkerConfig) *CurationWalker {
	r := NewRecord(cfg.RecordID)
	if cfg.InitialRecord != nil {
		r = *cfg.InitialRecord
	}

	return &CurationWalker{
		identity: circuit.AgentIdentity{
			PersonaName: "curator",
			Alignment:   circuit.AlignmentThesis,
		},
		state:      circuit.NewWalkerState(cfg.RecordID),
		schema:     cfg.Schema,
		sources:    cfg.Sources,
		extractors: cfg.Extractors,
		record:     r,
	}
}

func (w *CurationWalker) Identity() circuit.AgentIdentity       { return w.identity }
func (w *CurationWalker) SetIdentity(id *circuit.AgentIdentity) { w.identity = *id }
func (w *CurationWalker) State() *circuit.WalkerState           { return w.state }

// Record returns the curated record after walking.
func (w *CurationWalker) Record() Record { return w.record }

// Promoted returns true if the record was promoted (all required fields present).
func (w *CurationWalker) Promoted() bool { return w.promoted }

// Handle processes each node in the curation circuit, producing
// CurationArtifact outputs that the edge evaluators use for routing.
func (w *CurationWalker) Handle(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	switch node.Name() {
	case nodeFetch:
		return w.handleFetch(ctx)
	case nodeExtract:
		return w.handleExtract(ctx, nc)
	case nodeValidate:
		return w.handleValidate()
	case nodeEnrich:
		return w.handleEnrich(ctx)
	case "promote":
		return w.handlePromote()
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownNode, node.Name())
	}
}

func (w *CurationWalker) handleFetch(ctx context.Context) (circuit.Artifact, error) {
	var lastRaw *RawEvidence
	for _, src := range w.sources {
		if !src.CanHandle(w.record.ID) {
			continue
		}
		raw, err := src.Fetch(ctx, w.record.ID)
		if err != nil {
			slog.WarnContext(ctx, circuit.LogSourceFetchFailed, slog.Any(circuit.LogKeySource, src.Type()), slog.Any(circuit.LogKeyRecord, w.record.ID), slog.Any(circuit.LogKeyError, err.Error()))
			continue
		}
		lastRaw = raw
		break
	}

	return &CurationArtifact{
		ArtifactType: nodeFetch,
		Rec:          &w.record,
		RawEvid:      lastRaw,
	}, nil
}

func (w *CurationWalker) handleExtract(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	var raw *RawEvidence
	if prior, ok := nc.PriorArtifact.(*CurationArtifact); ok {
		raw = prior.RawEvid
	}
	if raw == nil {
		return &CurationArtifact{ArtifactType: nodeExtract, Rec: &w.record}, nil
	}

	for _, ext := range w.extractors {
		fields, err := ext.Extract(ctx, raw)
		if err != nil {
			slog.WarnContext(ctx, circuit.LogExtractorFailed, slog.Any(circuit.LogKeyExtractor, ext.Type()), slog.Any(circuit.LogKeyError, err.Error()))
			continue
		}
		for _, f := range fields {
			w.record.Set(f)
		}
	}

	return &CurationArtifact{
		ArtifactType: nodeExtract,
		Rec:          &w.record,
	}, nil
}

func (w *CurationWalker) handleValidate() (circuit.Artifact, error) {
	cr := CheckCompleteness(w.record, w.schema)

	moreSources := false
	for _, src := range w.sources {
		if src.CanHandle(w.record.ID) {
			moreSources = true
			break
		}
	}

	return &CurationArtifact{
		ArtifactType: nodeValidate,
		Rec:          &w.record,
		Complete:     cr.Promotable,
		MoreSources:  moreSources && !cr.Promotable,
		Conf:         cr.Score,
	}, nil
}

func (w *CurationWalker) handleEnrich(_ context.Context) (circuit.Artifact, error) {
	cr := CheckCompleteness(w.record, w.schema)
	return &CurationArtifact{
		ArtifactType: nodeEnrich,
		Rec:          &w.record,
		Complete:     cr.Promotable,
		Conf:         cr.Score,
	}, nil
}

func (w *CurationWalker) handlePromote() (circuit.Artifact, error) {
	w.promoted = true
	slog.InfoContext(context.Background(), circuit.LogRecordPromoted, slog.Any(circuit.LogKeyRecordID, w.record.ID), slog.Any(circuit.LogKeyFields, len(w.record.Fields)))
	return &CurationArtifact{
		ArtifactType: "promote",
		Rec:          &w.record,
		Complete:     true,
		Conf:         1.0,
	}, nil
}

// Verify compile-time interface compliance.
var _ circuit.Walker = (*CurationWalker)(nil)
