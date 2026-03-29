package ingest

import "errors"

var (
	// ErrDatasetManifestKindMustBeDatasetGot is returned for: dataset manifest: kind must be 'dataset', got
	ErrDatasetManifestKindMustBeDatasetGot = errors.New("dataset manifest: kind must be 'dataset', got")

	// ErrDatasetManifestMetadataScenarioIsRequired is returned for: dataset manifest: metadata.scenario is required
	ErrDatasetManifestMetadataScenarioIsRequired = errors.New("dataset manifest: metadata.scenario is required")

	// ErrPipelineSourceIsRequired is returned for: pipeline: Source is required
	ErrPipelineSourceIsRequired = errors.New("pipeline: Source is required")

	// ErrPipelineMatcherIsRequired is returned for: pipeline: Matcher is required
	ErrPipelineMatcherIsRequired = errors.New("pipeline: Matcher is required")
)
