package rehearsal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type SetupOption func(t *testing.T, dir string)

func WithGitRepo() SetupOption {
	return func(t *testing.T, dir string) {
		t.Helper()
		for _, args := range [][]string{
			{"init"},
			{"config", "user.email", "test@test.com"},
			{"config", "user.name", "Test"},
			{"add", "."},
			{"commit", "-m", "initial"},
		} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
	}
}

func WithFailingTest() SetupOption {
	return func(t *testing.T, dir string) {
		t.Helper()
		content := `package auth

import "testing"

func TestHandler_EmptySecret(t *testing.T) {
	h := NewHandler("")
	if h.Validate("") {
		t.Fatal("empty token should NOT pass with empty secret")
	}
}
`
		path := filepath.Join(dir, "auth", "handler_failing_test.go")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func WithExtraFiles(files map[string]string) SetupOption {
	return func(t *testing.T, dir string) {
		t.Helper()
		WriteFiles(t, dir, files)
	}
}

func WriteFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func SetupWorkspace(t *testing.T, opts ...SetupOption) string {
	t.Helper()
	dir := t.TempDir()

	WriteFiles(t, dir, map[string]string{
		"go.mod": "module example.com/testproject\n\ngo 1.21\n",
		"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
		"auth/handler.go": `package auth

type Handler struct {
	secret string
}

func NewHandler(secret string) *Handler {
	return &Handler{secret: secret}
}

func (h *Handler) Validate(token string) bool {
	return token == h.secret
}
`,
		"auth/handler_test.go": `package auth

import "testing"

func TestHandler_Validate(t *testing.T) {
	h := NewHandler("secret123")
	if !h.Validate("secret123") {
		t.Fatal("valid token should pass")
	}
	if h.Validate("wrong") {
		t.Fatal("invalid token should fail")
	}
}
`,
	})

	for _, opt := range opts {
		opt(t, dir)
	}

	return dir
}
