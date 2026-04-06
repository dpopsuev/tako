// handler_reexport.go — type aliases for engine/handler sub-package.
// Existing engine.Transformer references continue to work.
package engine

import "github.com/dpopsuev/origami/engine/handler"

// Handler types — definitions live in engine/handler.
type (
	Transformer              = handler.Transformer
	DeterministicTransformer = handler.DeterministicTransformer
	TypedTransformer         = handler.TypedTransformer
	TransformerContext       = handler.TransformerContext
	TransformerRegistry      = handler.TransformerRegistry
	Extractor                = handler.Extractor
	ExtractorRegistry        = handler.ExtractorRegistry
	Renderer                 = handler.Renderer
	RendererRegistry         = handler.RendererRegistry
	Hook                     = handler.Hook
	HookRegistry             = handler.HookRegistry
	HookFunc                 = handler.HookFunc
)

// Handler constructors.
var (
	IsDeterministic = handler.IsDeterministic
	TransformerFunc = handler.TransformerFunc
	NewHookFunc     = handler.NewHookFunc
)

// Error sentinels stay in engine/errors.go — handler/ has its own copies
// for the sub-package. Both are valid sentinel vars.
