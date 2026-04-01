package def

// Category: DSL & Build — vocabulary (code-to-name translation).

import (
	"sync"

	"gopkg.in/yaml.v3"
)

// Vocabulary translates machine codes to human-readable names.
// Implementations must be safe for concurrent use.
type Vocabulary interface {
	Name(code string) string
}

// VocabularyFunc adapts a plain function to the Vocabulary interface.
type VocabularyFunc func(string) string

func (f VocabularyFunc) Name(code string) string { return f(code) }

// MapVocabulary is a thread-safe, register-based vocabulary.
// Unknown codes are returned as-is (pass-through default).
type MapVocabulary struct {
	mu      sync.RWMutex
	entries map[string]string
}

// NewMapVocabulary returns an empty vocabulary ready for registration.
func NewMapVocabulary() *MapVocabulary {
	return &MapVocabulary{entries: make(map[string]string)}
}

// Register adds a single code -> name mapping. Returns the receiver for chaining.
func (v *MapVocabulary) Register(code, name string) *MapVocabulary {
	v.mu.Lock()
	v.entries[code] = name
	v.mu.Unlock()
	return v
}

// RegisterAll adds all entries from the map. Returns the receiver for chaining.
func (v *MapVocabulary) RegisterAll(entries map[string]string) *MapVocabulary {
	v.mu.Lock()
	for code, name := range entries {
		v.entries[code] = name
	}
	v.mu.Unlock()
	return v
}

// Name returns the human-readable name for code, or code itself if unregistered.
func (v *MapVocabulary) Name(code string) string {
	v.mu.RLock()
	name, ok := v.entries[code]
	v.mu.RUnlock()
	if ok {
		return name
	}
	return code
}

// NameWithCode formats as "Human Name (code)" for dual-audience contexts.
// If the vocabulary returns the code unchanged, only the code is returned.
// When v implements RichVocabulary and the entry has a Short field, the
// parenthetical uses Short instead of the raw code: "Recall (F0)".
func NameWithCode(v Vocabulary, code string) string {
	name := v.Name(code)
	if name == code {
		return code
	}
	paren := code
	if rv, ok := v.(RichVocabulary); ok {
		if s := rv.Short(code); s != "" && s != code {
			paren = s
		}
	}
	return name + " (" + paren + ")"
}

// ChainVocabulary tries multiple vocabularies in order. The first one that
// returns a value different from the input code wins. If none translates
// the code, the code is returned as-is.
type ChainVocabulary []Vocabulary

func (c ChainVocabulary) Name(code string) string {
	for _, v := range c {
		if name := v.Name(code); name != code {
			return name
		}
	}
	return code
}

// --- Rich Vocabulary ---

// VocabEntry holds structured metadata for a machine code:
// Short (abbreviation), Long (human name), and Description (tooltip/hover text).
// Supports YAML shorthand: a plain string value is treated as the Long name.
type VocabEntry struct {
	Short       string `yaml:"short"`
	Long        string `yaml:"long"`
	Description string `yaml:"description"`
}

// UnmarshalYAML supports both map form ({short: X, long: Y}) and shorthand
// string form ("Y") which sets Long only.
func (e *VocabEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		e.Long = value.Value
		return nil
	}
	type plain VocabEntry
	return value.Decode((*plain)(e))
}

// RichVocabulary extends Vocabulary with structured metadata access.
// Implementations must be safe for concurrent use.
type RichVocabulary interface {
	Vocabulary
	Entry(code string) (VocabEntry, bool)
	Short(code string) string
	Description(code string) string
}

// RichMapVocabulary is a thread-safe, register-based rich vocabulary.
// Name() returns Long, falling back to Short, falling back to code itself.
type RichMapVocabulary struct {
	mu      sync.RWMutex
	entries map[string]VocabEntry
}

// NewRichMapVocabulary returns an empty rich vocabulary ready for registration.
func NewRichMapVocabulary() *RichMapVocabulary {
	return &RichMapVocabulary{entries: make(map[string]VocabEntry)}
}

// RegisterEntry adds a single code -> VocabEntry mapping. Returns the receiver for chaining.
func (v *RichMapVocabulary) RegisterEntry(code string, entry VocabEntry) *RichMapVocabulary {
	v.mu.Lock()
	v.entries[code] = entry
	v.mu.Unlock()
	return v
}

// RegisterEntries adds multiple code -> VocabEntry mappings. Returns the receiver for chaining.
func (v *RichMapVocabulary) RegisterEntries(entries map[string]VocabEntry) *RichMapVocabulary {
	v.mu.Lock()
	for code, entry := range entries {
		v.entries[code] = entry
	}
	v.mu.Unlock()
	return v
}

// Name returns Long (falling back to Short, then code) for backward compatibility
// with the Vocabulary interface.
func (v *RichMapVocabulary) Name(code string) string {
	v.mu.RLock()
	e, ok := v.entries[code]
	v.mu.RUnlock()
	if !ok {
		return code
	}
	if e.Long != "" {
		return e.Long
	}
	if e.Short != "" {
		return e.Short
	}
	return code
}

// Entry returns the full VocabEntry for code, or (zero, false) if unregistered.
func (v *RichMapVocabulary) Entry(code string) (VocabEntry, bool) {
	v.mu.RLock()
	e, ok := v.entries[code]
	v.mu.RUnlock()
	return e, ok
}

// Short returns the short name for code, or "" if unregistered.
func (v *RichMapVocabulary) Short(code string) string {
	v.mu.RLock()
	e, ok := v.entries[code]
	v.mu.RUnlock()
	if ok {
		return e.Short
	}
	return ""
}

// Description returns the description for code, or "" if unregistered.
func (v *RichMapVocabulary) Description(code string) string {
	v.mu.RLock()
	e, ok := v.entries[code]
	v.mu.RUnlock()
	if ok {
		return e.Description
	}
	return ""
}

// RichChainVocabulary tries multiple RichVocabulary implementations in order.
// The first one that has an entry for the code wins.
type RichChainVocabulary []RichVocabulary

func (c RichChainVocabulary) Name(code string) string {
	for _, v := range c {
		if _, ok := v.Entry(code); ok {
			return v.Name(code)
		}
	}
	return code
}

func (c RichChainVocabulary) Entry(code string) (VocabEntry, bool) {
	for _, v := range c {
		if e, ok := v.Entry(code); ok {
			return e, true
		}
	}
	return VocabEntry{}, false
}

func (c RichChainVocabulary) Short(code string) string {
	for _, v := range c {
		if _, ok := v.Entry(code); ok {
			return v.Short(code)
		}
	}
	return ""
}

func (c RichChainVocabulary) Description(code string) string {
	for _, v := range c {
		if _, ok := v.Entry(code); ok {
			return v.Description(code)
		}
	}
	return ""
}
