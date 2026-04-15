package handler

import "errors"

// Sentinel errors for handler types.
var (
	ErrNode                     = errors.New("node")
	ErrExtractor                = errors.New("extractor")
	ErrInstrument              = errors.New("instrument")
	ErrHook                    = errors.New("hook")
	ErrRenderer                = errors.New("renderer")
	ErrGenerator               = errors.New("generator")
	ErrExtractorRegistryIsNil  = errors.New("extractor registry is nil")
	ErrHookRegistryIsNil       = errors.New("hook registry is nil")
	ErrInstrumentRegistryIsNil = errors.New("instrument registry is nil")
	ErrRendererRegistryIsNil    = errors.New("renderer registry is nil")
	ErrJSONExtractor            = errors.New("JSONExtractor")
	ErrRegexExtractor           = errors.New("RegexExtractor")
	ErrCodeBlockExtractor       = errors.New("CodeBlockExtractor")
	ErrLineSplitExtractor       = errors.New("LineSplitExtractor")
	ErrFileWriteHookNode        = errors.New("FileWriteHook: node config missing output_path")
)
