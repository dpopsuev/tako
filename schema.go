package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type ArtifactSchema = circuit.ArtifactSchema
type FieldSchema = circuit.FieldSchema

// ValidateArtifact checks that an artifact's Raw() value conforms to the schema.
func ValidateArtifact(schema *ArtifactSchema, artifact Artifact) error {
	return circuit.ValidateArtifact(schema, artifact)
}
