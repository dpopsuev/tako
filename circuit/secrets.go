package circuit

// Category: Processing & Support — secret reference expansion for circuit config.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// secretPattern matches ${VAR_NAME} and ${VAR_NAME:-default} references.
var secretPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// ErrSecretEnvNotSet is returned when a referenced env var is empty and no default is provided.
var ErrSecretEnvNotSet = fmt.Errorf("secret: env var not set")

// ResolveSecrets walks a map[string]any and expands secret references
// in string values. Supported patterns:
//
//	${VAR_NAME}          — os.Getenv, error if empty
//	${VAR_NAME:-default} — os.Getenv with default fallback
//	file:///path/to/file — read file contents, trim whitespace
//
// Returns a new map; the input is never mutated.
func ResolveSecrets(config map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(config))
	for k, v := range config {
		resolved, err := resolveValue(v)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
		result[k] = resolved
	}
	return result, nil
}

// resolveValue dispatches resolution based on the value's dynamic type.
func resolveValue(v any) (any, error) {
	switch val := v.(type) {
	case string:
		return resolveString(val)
	case map[string]any:
		return ResolveSecrets(val)
	case []any:
		return resolveSlice(val)
	default:
		return v, nil
	}
}

// resolveSlice resolves each element in a slice, returning a new slice.
func resolveSlice(items []any) ([]any, error) {
	out := make([]any, len(items))
	for i, item := range items {
		resolved, err := resolveValue(item)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		out[i] = resolved
	}
	return out, nil
}

// resolveString expands file:// references and ${VAR} patterns in a string.
func resolveString(s string) (string, error) {
	if strings.HasPrefix(s, "file://") {
		path := strings.TrimPrefix(s, "file://")
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read secret file %s: %w", path, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	var resolveErr error
	result := secretPattern.ReplaceAllStringFunc(s, func(match string) string {
		if resolveErr != nil {
			return match
		}
		inner := match[2 : len(match)-1] // strip ${ and }

		varName, defaultVal, hasDefault := strings.Cut(inner, ":-")
		val := os.Getenv(varName)
		if val != "" {
			return val
		}
		if hasDefault {
			return defaultVal
		}
		resolveErr = fmt.Errorf("%w: %s", ErrSecretEnvNotSet, varName)
		return match
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	return result, nil
}
