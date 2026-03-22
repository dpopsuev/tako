package framework

// Category: Processing & Support — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

var (
	WithWalkerState      = engine.WithWalkerState
	WalkerStateFromContext = engine.WalkerStateFromContext
)

type Hook = engine.Hook
type HookRegistry = engine.HookRegistry
type HookFunc = engine.HookFunc

var NewHookFunc = engine.NewHookFunc

const BuiltinHookFileWrite = engine.BuiltinHookFileWrite

type FileWriteHook = engine.FileWriteHook
