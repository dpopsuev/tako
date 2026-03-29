package circuit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSecrets_EnvVar(t *testing.T) {
	t.Setenv("ORIGAMI_TEST_SECRET", "hunter2")

	in := map[string]any{"token": "${ORIGAMI_TEST_SECRET}"}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if out["token"] != "hunter2" {
		t.Errorf("token = %q, want hunter2", out["token"])
	}
}

func TestResolveSecrets_EnvVarWithDefault(t *testing.T) {
	t.Setenv("ORIGAMI_TEST_UNSET_GUARD", "")
	os.Unsetenv("ORIGAMI_TEST_UNSET_GUARD")

	in := map[string]any{"region": "${ORIGAMI_TEST_UNSET_GUARD:-us-east-1}"}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if out["region"] != "us-east-1" {
		t.Errorf("region = %q, want us-east-1", out["region"])
	}
}

func TestResolveSecrets_EnvVarSetOverridesDefault(t *testing.T) {
	t.Setenv("ORIGAMI_TEST_REGION", "eu-west-1")

	in := map[string]any{"region": "${ORIGAMI_TEST_REGION:-us-east-1}"}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if out["region"] != "eu-west-1" {
		t.Errorf("region = %q, want eu-west-1", out["region"])
	}
}

func TestResolveSecrets_FileRef(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "api-key.txt")
	if err := os.WriteFile(p, []byte("  sk-12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	in := map[string]any{"api_key": "file://" + p}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if out["api_key"] != "sk-12345" {
		t.Errorf("api_key = %q, want sk-12345", out["api_key"])
	}
}

func TestResolveSecrets_NestedMap(t *testing.T) {
	t.Setenv("ORIGAMI_TEST_NESTED", "deep-value")

	in := map[string]any{
		"outer": map[string]any{
			"inner": "${ORIGAMI_TEST_NESTED}",
		},
	}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	inner, ok := out["outer"].(map[string]any)
	if !ok {
		t.Fatal("outer is not map[string]any")
	}
	if inner["inner"] != "deep-value" {
		t.Errorf("inner = %q, want deep-value", inner["inner"])
	}
}

func TestResolveSecrets_SliceValues(t *testing.T) {
	t.Setenv("ORIGAMI_TEST_SLICE_A", "alpha")
	t.Setenv("ORIGAMI_TEST_SLICE_B", "beta")

	in := map[string]any{
		"items": []any{"${ORIGAMI_TEST_SLICE_A}", "${ORIGAMI_TEST_SLICE_B}", 42},
	}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	items, ok := out["items"].([]any)
	if !ok {
		t.Fatal("items is not []any")
	}
	if items[0] != "alpha" {
		t.Errorf("items[0] = %v, want alpha", items[0])
	}
	if items[1] != "beta" {
		t.Errorf("items[1] = %v, want beta", items[1])
	}
	if items[2] != 42 {
		t.Errorf("items[2] = %v, want 42", items[2])
	}
}

func TestResolveSecrets_NonStringPassthrough(t *testing.T) {
	in := map[string]any{
		"count":   42,
		"enabled": true,
		"rate":    3.14,
		"nothing": nil,
	}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if out["count"] != 42 {
		t.Errorf("count = %v", out["count"])
	}
	if out["enabled"] != true {
		t.Errorf("enabled = %v", out["enabled"])
	}
	if out["rate"] != 3.14 {
		t.Errorf("rate = %v", out["rate"])
	}
	if out["nothing"] != nil {
		t.Errorf("nothing = %v", out["nothing"])
	}
}

func TestResolveSecrets_MissingEnvVarError(t *testing.T) {
	os.Unsetenv("ORIGAMI_TEST_MISSING_VAR_XYZ")

	in := map[string]any{"secret": "${ORIGAMI_TEST_MISSING_VAR_XYZ}"}
	_, err := ResolveSecrets(in)
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
	if !errors.Is(err, ErrSecretEnvNotSet) {
		t.Errorf("error = %v, want ErrSecretEnvNotSet", err)
	}
}

func TestResolveSecrets_DoesNotMutateInput(t *testing.T) {
	t.Setenv("ORIGAMI_TEST_MUTATE", "resolved")

	in := map[string]any{"key": "${ORIGAMI_TEST_MUTATE}"}
	_, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if in["key"] != "${ORIGAMI_TEST_MUTATE}" {
		t.Errorf("input mutated: key = %v", in["key"])
	}
}

func TestResolveSecrets_FileNotFound(t *testing.T) {
	in := map[string]any{"key": "file:///nonexistent/path/secret.txt"}
	_, err := ResolveSecrets(in)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestResolveSecrets_MultipleRefsInOneString(t *testing.T) {
	t.Setenv("ORIGAMI_TEST_HOST", "localhost")
	t.Setenv("ORIGAMI_TEST_PORT", "5432")

	in := map[string]any{"dsn": "postgres://${ORIGAMI_TEST_HOST}:${ORIGAMI_TEST_PORT}/db"}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if out["dsn"] != "postgres://localhost:5432/db" {
		t.Errorf("dsn = %q, want postgres://localhost:5432/db", out["dsn"])
	}
}

func TestResolveSecrets_EmptyMap(t *testing.T) {
	out, err := ResolveSecrets(map[string]any{})
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty map, got %v", out)
	}
}

func TestResolveSecrets_PlainStringUnchanged(t *testing.T) {
	in := map[string]any{"label": "no-secrets-here"}
	out, err := ResolveSecrets(in)
	if err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if out["label"] != "no-secrets-here" {
		t.Errorf("label = %q", out["label"])
	}
}
