package containertest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/domainfs"
	"github.com/dpopsuev/origami/domainserve"
	"github.com/dpopsuev/origami/gateway"
	dsr "github.com/dpopsuev/rh-dsr"
	mcpserver "github.com/dpopsuev/rh-rca/mcpconfig"
	"github.com/dpopsuev/origami/subprocess/containertest"
)

// These tests use httptest servers to simulate the four-service architecture
// (Gateway, RCA engine, Harvester, Asterisk domain) without requiring real
// container images. They validate Gateway routing, health probes, and
// concurrency patterns that container E2E tests would exercise.
//
// Real container E2E tests (requiring podman + built images) are gated by
// RequirePodman and will be added when Dockerfiles are generated.

func domainTestdataFS() fs.FS {
	_, f, _, _ := runtime.Caller(0)
	return os.DirFS(filepath.Join(filepath.Dir(f), "..", "..", "schematics", "rca", "mcpconfig", "testdata"))
}

func newDomainServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler := domainserve.New(domainTestdataFS(), domainserve.Config{
		Name:    "asterisk",
		Version: "v0.1.0-test",
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

type domainSessionCaller struct {
	session *sdkmcp.ClientSession
}

func (s *domainSessionCaller) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	return s.session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
}

func connectDomainFS(t *testing.T, domainSrvURL string) *domainfs.MCPRemoteFS {
	t.Helper()
	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: domainSrvURL + "/mcp"}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-engine-domain", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to domain server: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return domainfs.New(&domainSessionCaller{session: session}).
		WithTimeout(5 * time.Second)
}

func newHarvesterServer(t *testing.T) *httptest.Server {
	t.Helper()
	router := dsr.NewRouter()
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "test-harvester", Version: "v0.1.0"},
		nil,
	)
	dsr.RegisterTools(server, router)

	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func newRCAServer(t *testing.T, opts ...mcpserver.ServerOption) *httptest.Server {
	t.Helper()
	srv := mcpserver.NewServer("test-rca", opts...)
	t.Cleanup(srv.Shutdown)

	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return srv.MCPServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: false},
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if srv.MCPServer != nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func newFourServiceGateway(t *testing.T) (domainSrv, knSrv, rcaSrv, gwSrv *httptest.Server, gw *gateway.Gateway) {
	t.Helper()
	domainSrv = newDomainServer(t)
	knSrv = newHarvesterServer(t)

	remoteFS := connectDomainFS(t, domainSrv.URL)
	rcaSrv = newRCAServer(t, mcpserver.WithDomainFS(remoteFS))

	gw = gateway.New([]gateway.BackendConfig{
		{Name: "rca", Endpoint: rcaSrv.URL + "/mcp"},
		{Name: "harvester", Endpoint: knSrv.URL + "/mcp"},
		{Name: "asterisk", Endpoint: domainSrv.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start gateway: %v", err)
	}
	t.Cleanup(func() { gw.Stop(context.Background()) })

	gwSrv = httptest.NewServer(gw.Handler())
	t.Cleanup(gwSrv.Close)
	return
}

func connectMCP(t *testing.T, endpoint string) *sdkmcp.ClientSession {
	t.Helper()
	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: endpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect MCP: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func TestE2E_FourServices_ToolRouting(t *testing.T) {
	_, _, _, gwSrv, _ := newFourServiceGateway(t)
	ctx := t.Context()
	session := connectMCP(t, gwSrv.URL+"/mcp")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	hasRCA := false
	hasHarvester := false
	hasDomain := false
	for _, tool := range tools.Tools {
		switch tool.Name {
		case "start_circuit", "get_next_step", "submit_step":
			hasRCA = true
		case "harvester_search", "harvester_read":
			hasHarvester = true
		case "domain_info", "domain_read", "domain_list":
			hasDomain = true
		}
	}
	if !hasRCA {
		t.Error("gateway missing RCA tools")
	}
	if !hasHarvester {
		t.Error("gateway missing harvester tools")
	}
	if !hasDomain {
		t.Error("gateway missing domain tools (domain_info, domain_read, domain_list)")
	}
}

func TestE2E_FourServices_HealthProbes(t *testing.T) {
	_, _, _, gwSrv, _ := newFourServiceGateway(t)

	resp, err := http.Get(gwSrv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz = %d, want 200", resp.StatusCode)
	}

	resp, err = http.Get(gwSrv.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("readyz = %d, want 200", resp.StatusCode)
	}
}

func TestE2E_HarvesterFailure_ReadyzDegrades(t *testing.T) {
	_, knSrv, _, gwSrv, _ := newFourServiceGateway(t)

	resp, _ := http.Get(gwSrv.URL + "/readyz")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("readyz initially = %d, want 200", resp.StatusCode)
	}

	knSrv.Close()
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(gwSrv.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz after kill: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("readyz after harvester kill = %d, want 503", resp.StatusCode)
	}
}

func TestE2E_Concurrency_MultipleSessions(t *testing.T) {
	_, _, _, gwSrv, _ := newFourServiceGateway(t)
	ctx := t.Context()

	const n = 5
	var wg sync.WaitGroup
	errors := make(chan error, n)

	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			transport := &sdkmcp.StreamableClientTransport{Endpoint: gwSrv.URL + "/mcp"}
			client := sdkmcp.NewClient(
				&sdkmcp.Implementation{Name: fmt.Sprintf("worker-%d", i), Version: "v0.1.0"},
				nil,
			)
			session, err := client.Connect(ctx, transport, nil)
			if err != nil {
				errors <- fmt.Errorf("worker %d connect: %w", i, err)
				return
			}
			defer session.Close()

			tools, err := session.ListTools(ctx, nil)
			if err != nil {
				errors <- fmt.Errorf("worker %d ListTools: %w", i, err)
				return
			}
			if len(tools.Tools) == 0 {
				errors <- fmt.Errorf("worker %d: no tools found", i)
				return
			}

			if err := session.Ping(ctx, nil); err != nil {
				errors <- fmt.Errorf("worker %d Ping: %w", i, err)
				return
			}
			errors <- nil
		}(i)
	}

	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestContainerTestHelper_RequirePodman(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	containertest.RequirePodman(t)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
