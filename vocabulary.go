package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type Vocabulary = circuit.Vocabulary
type VocabularyFunc = circuit.VocabularyFunc
type MapVocabulary = circuit.MapVocabulary
type VocabEntry = circuit.VocabEntry
type RichVocabulary = circuit.RichVocabulary
type RichMapVocabulary = circuit.RichMapVocabulary

func NewMapVocabulary() *MapVocabulary        { return circuit.NewMapVocabulary() }
func NewRichMapVocabulary() *RichMapVocabulary { return circuit.NewRichMapVocabulary() }
func NameWithCode(v Vocabulary, code string) string { return circuit.NameWithCode(v, code) }

// chainVocabulary tries multiple vocabularies in order.
type chainVocabulary = circuit.ChainVocabulary

// richChainVocabulary tries multiple RichVocabulary implementations in order.
type richChainVocabulary = circuit.RichChainVocabulary
