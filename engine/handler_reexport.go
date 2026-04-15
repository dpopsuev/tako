// handler_reexport.go — type aliases for engine/handler sub-package.
// Existing engine.Instrument references continue to work.
package engine

import "github.com/dpopsuev/origami/engine/handler"

// Handler types — definitions live in engine/handler.
type (
	Instrument              = handler.Instrument
	DeterministicInstrument = handler.DeterministicInstrument
	TypedInstrument         = handler.TypedInstrument
	StationLoggable         = handler.StationLoggable
	InstrumentContext       = handler.InstrumentContext
	InstrumentRegistry      = handler.InstrumentRegistry
	Extractor               = handler.Extractor
	ExtractorRegistry       = handler.ExtractorRegistry
	Renderer                = handler.Renderer
	RendererRegistry        = handler.RendererRegistry
	Hook                    = handler.Hook
	HookRegistry            = handler.HookRegistry
	HookFunc                = handler.HookFunc
)

// Handler constructors.
var (
	IsDeterministic = handler.IsDeterministic
	InstrumentFunc  = handler.InstrumentFunc
	NewHookFunc     = handler.NewHookFunc
)

// Error sentinels stay in engine/errors.go — handler/ has its own copies
// for the sub-package. Both are valid sentinel vars.
