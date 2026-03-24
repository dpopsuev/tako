package agentport

import (
	"github.com/dpopsuev/bugle/dispatch"
	"github.com/dpopsuev/bugle/dispatch/guard"
)

// Type aliases — definitions live in bugle/dispatch.
type (
	Dispatcher         = dispatch.Dispatcher
	Context            = dispatch.Context
	PullHints          = dispatch.PullHints
	ExternalDispatcher = dispatch.ExternalDispatcher

	CLIDispatcher   = dispatch.CLIDispatcher
	HTTPDispatcher  = dispatch.HTTPDispatcher
	StdinDispatcher = dispatch.StdinDispatcher
	StaticDispatcher = dispatch.StaticDispatcher

	CLIOption  = dispatch.CLIOption
	HTTPOption = dispatch.HTTPOption

	Finalizer = dispatch.Finalizer
	Unwrapper = dispatch.Unwrapper

	StdinTemplate = dispatch.StdinTemplate
)

// Constructors.
var (
	NewCLIDispatcher   = dispatch.NewCLIDispatcher
	NewHTTPDispatcher  = dispatch.NewHTTPDispatcher
	NewStdinDispatcher = dispatch.NewStdinDispatcher
	NewStaticDispatcher = dispatch.NewStaticDispatcher
	NewStdinDispatcherWithTemplate = dispatch.NewStdinDispatcherWithTemplate
	DefaultStdinTemplate           = dispatch.DefaultStdinTemplate
)

// CLI options.
var (
	WithCLIArgs    = dispatch.WithCLIArgs
	WithCLITimeout = dispatch.WithCLITimeout
	WithCLILogger  = dispatch.WithCLILogger
)

// HTTP options.
var (
	WithModel     = dispatch.WithModel
	WithAPIKeyEnv = dispatch.WithAPIKeyEnv
	WithHTTPClient = dispatch.WithHTTPClient
	WithHTTPLogger = dispatch.WithHTTPLogger
)

// Utility functions.
var (
	UnwrapFinalizer = dispatch.UnwrapFinalizer
	DiscardLogger   = dispatch.DiscardLogger
)

// Guard decorators — resilience and token tracking.
type (
	TokenTrackingDispatcher = guard.TokenTrackingDispatcher
	DispatchHook            = guard.Hook
)

var NewTokenTrackingDispatcher = guard.NewTokenTrackingDispatcher
