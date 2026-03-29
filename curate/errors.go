package curate

import "errors"

var (
	// ErrInvalidDatasetName is returned for: curate: invalid dataset name
	ErrInvalidDatasetName = errors.New("curate: invalid dataset name")

	// ErrDataset is returned for: curate/memory: dataset
	ErrDataset = errors.New("curate/memory: dataset")

	// ErrDatasetNameMustNotBeEmpty is returned for: curate/memory: dataset name must not be empty
	ErrDatasetNameMustNotBeEmpty = errors.New("curate/memory: dataset name must not be empty")

	// ErrUnknownNode is returned for: curate walker: unknown node
	ErrUnknownNode = errors.New("curate walker: unknown node")
)
