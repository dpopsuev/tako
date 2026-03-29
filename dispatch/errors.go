package dispatch

import "errors"

var (
	// ErrBatchDispatchReturnedNoResults is returned for: batch dispatch returned no results
	ErrBatchDispatchReturnedNoResults = errors.New("batch dispatch returned no results")

	// ErrTimeoutAfter is returned for: timeout after
	ErrTimeoutAfter = errors.New("timeout after")

	// ErrResponderError is returned for: responder error
	ErrResponderError = errors.New("responder error")

	// ErrStaleArtifactToleranceExceeded is returned for: stale artifact tolerance exceeded
	ErrStaleArtifactToleranceExceeded = errors.New("stale artifact tolerance exceeded")

	// ErrArtifactAt is returned for: artifact at
	ErrArtifactAt = errors.New("artifact at")

	// ErrDispatchTimeoutAfter is returned for: dispatch timeout after
	ErrDispatchTimeoutAfter = errors.New("dispatch timeout after")

	// ErrDispatcherClosed is returned for: dispatcher closed
	ErrDispatcherClosed = errors.New("dispatcher closed")

	// ErrDispatchId is returned for: dispatch_id
	ErrDispatchId = errors.New("dispatch_id")

	// ErrUnknownDispatchId is returned for: unknown dispatch_id
	ErrUnknownDispatchId = errors.New("unknown dispatch_id")

	// ErrAborted is returned for: aborted
	ErrAborted = errors.New("aborted")

	// ErrNetworkClientGETNextStatus is returned for: network client: GET /next: status
	ErrNetworkClientGETNextStatus = errors.New("network client: GET /next: status")

	// ErrNetworkClientPOSTSubmitStatus is returned for: network client: POST /submit: status
	ErrNetworkClientPOSTSubmitStatus = errors.New("network client: POST /submit: status")

	// ErrNetworkClientPOSTSignalStatus is returned for: network client: POST /signal: status
	ErrNetworkClientPOSTSignalStatus = errors.New("network client: POST /signal: status")

	// ErrNetworkClientGETSignalsStatus is returned for: network client: GET /signals: status
	ErrNetworkClientGETSignalsStatus = errors.New("network client: GET /signals: status")

	// ErrCommandTimedOut is returned when a CLI command exceeds its timeout.
	ErrCommandTimedOut = errors.New("command timed out")

	// ErrCommandNoOutput is returned when a CLI command produces no output.
	ErrCommandNoOutput = errors.New("command produced no output")
)
