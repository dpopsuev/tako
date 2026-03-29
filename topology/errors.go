package topology

import "errors"

var (
	// ErrUnknownTopology is returned for: unknown topology
	ErrUnknownTopology = errors.New("unknown topology")

	// ErrValidationFailed is returned when topology validation fails.
	ErrValidationFailed = errors.New("topology validation failed")

	// ErrTopologyAlreadyRegistered is returned when a topology is already registered.
	ErrTopologyAlreadyRegistered = errors.New("topology already registered")
)
