package framework

// Category: Processing & Support — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

type Extractor = engine.Extractor
type ExtractorRegistry = engine.ExtractorRegistry

const (
	BuiltinExtractorJSONSchema = engine.BuiltinExtractorJSONSchema
	BuiltinExtractorRegex      = engine.BuiltinExtractorRegex
)

type JSONSchemaExtractor = engine.JSONSchemaExtractor

var NewRegexExtractor = engine.NewRegexExtractor
var MustRegexExtractor = engine.MustRegexExtractor
