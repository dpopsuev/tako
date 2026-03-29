package containertest

import "errors"

var (
	// ErrTimeoutWaitingForHealthzOnPort is returned for: timeout waiting for healthz on port
	ErrTimeoutWaitingForHealthzOnPort = errors.New("timeout waiting for healthz on port")
)
