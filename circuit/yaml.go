package circuit

// Category: DSL & Build — YAML helpers (thin wrappers to avoid repeating the import).

import "gopkg.in/yaml.v3"

// Type aliases for yaml.v3 node types, used by UnmarshalYAML methods.
type yamlNode = yaml.Node

// YAML node kind constants.
const (
	yamlSequenceNode = yaml.SequenceNode
	yamlScalarNode   = yaml.ScalarNode
	yamlMappingNode  = yaml.MappingNode
)

// yamlUnmarshal wraps yaml.Unmarshal.
func yamlUnmarshal(in []byte, out any) error {
	return yaml.Unmarshal(in, out)
}

// yamlMarshal wraps yaml.Marshal.
func yamlMarshal(in any) ([]byte, error) {
	return yaml.Marshal(in)
}
