package subprocess

import "errors"

var (
	// ErrContainer is returned for: container
	ErrContainer = errors.New("container")

	// ErrUnknownContainer is returned for: unknown container
	ErrUnknownContainer = errors.New("unknown container")

	// ErrUnknownSchematic is returned for: unknown schematic
	ErrUnknownSchematic = errors.New("unknown schematic")

	// ErrRemote is returned for: remote
	ErrRemote = errors.New("remote")

	// ErrSubprocessAlreadyStarted is returned for: subprocess already started
	ErrSubprocessAlreadyStarted = errors.New("subprocess already started")

	// ErrSubprocessNotStarted is returned for: subprocess not started
	ErrSubprocessNotStarted = errors.New("subprocess not started")

	// ErrTool is returned for: tool
	ErrTool = errors.New("tool")
)
