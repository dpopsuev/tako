package containertest_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/subprocess/containertest"
)

func repoRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

// TestContainerE2E_BuildImages validates that the deploy/ Dockerfiles
// produce valid OCI images. Gated by podman availability.
func TestContainerE2E_BuildImages(t *testing.T) {
	env := containertest.NewEnv(t)
	root := repoRoot()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	// Dockerfiles strip local replace directives before go mod download.
	// RCA container build requires published rh-dsr module — skip until available.
	images := []struct {
		dockerfile string
		tag        string
	}{
		{"deploy/Dockerfile.llm-worker", "origami-llm-worker-e2e"},
	}
	for _, img := range images {
		t.Run(img.tag, func(t *testing.T) {
			df := filepath.Join(root, img.dockerfile)
			env.BuildImageFromDockerfile(ctx, df, img.tag, root)
			t.Logf("built %s", img.tag)
		})
	}
}

// TestContainerE2E_MediatorDSR builds and starts the mediator +
// dsr containers on host network, then validates tool routing.
// Uses host networking so containers can reach each other via localhost.
//
// Requires: podman, not -short.
// NOTE: DSR (formerly dsr) has moved to github.com/dpopsuev/rh-dsr.
// This test is temporarily skipped until the Dockerfile is rebuilt for rh-dsr.
func TestContainerE2E_MediatorDSR(t *testing.T) {
	t.Skip("dsr moved to rh-dsr — container E2E needs Dockerfile update")
	env := containertest.NewEnv(t)
	root := repoRoot()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	t.Log("building dsr image...")
	env.BuildImageFromDockerfile(ctx,
		filepath.Join(root, "deploy/Dockerfile.dsr"),
		"origami-dsr-e2e", root)

	t.Log("building mediator image...")
	env.BuildImageFromDockerfile(ctx,
		filepath.Join(root, "deploy/Dockerfile.mediator"),
		"origami-mediator-e2e", root)

	knPort := 19100
	gwPort := 19000

	t.Log("starting dsr engine...")
	env.StartServiceWithConfig(ctx, containertest.ServiceConfig{
		Name:    "e2e-dsr",
		Image:   "origami-dsr-e2e",
		Port:    knPort,
		Network: "host",
		Args:    []string{"--port", fmt.Sprintf("%d", knPort)},
	})

	t.Log("starting mediator...")
	env.StartServiceWithConfig(ctx, containertest.ServiceConfig{
		Name:    "e2e-mediator",
		Image:   "origami-mediator-e2e",
		Port:    gwPort,
		Network: "host",
		Args: []string{
			"--port", fmt.Sprintf("%d", gwPort),
			"--backend", fmt.Sprintf("dsr=http://127.0.0.1:%d/mcp", knPort),
		},
	})

	t.Run("HealthProbes", func(t *testing.T) {
		for _, path := range []string{"/healthz", "/readyz"} {
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", gwPort, path))
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s = %d, want 200", path, resp.StatusCode)
			}
		}
	})

	t.Run("ToolRouting", func(t *testing.T) {
		transport := &sdkmcp.StreamableClientTransport{
			Endpoint: fmt.Sprintf("http://127.0.0.1:%d/mcp", gwPort),
		}
		client := sdkmcp.NewClient(
			&sdkmcp.Implementation{Name: "e2e-client", Version: "v0.1.0"},
			nil,
		)
		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		defer session.Close()

		tools, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}

		hasDSR := false
		for _, tool := range tools.Tools {
			if tool.Name == "dsr_search" || tool.Name == "dsr_read" {
				hasDSR = true
			}
		}
		if !hasDSR {
			t.Error("missing dsr tools through mediator")
		}
		t.Logf("discovered %d tools through mediator", len(tools.Tools))
	})
}
