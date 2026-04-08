package docs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dpopsuev/origami/instruments/docs"
	"github.com/dpopsuev/origami/toolkit"
)

func TestDocsDriver_Handles(t *testing.T) {
	d := docs.NewDocsDriver()
	if got := d.Handles(); got != toolkit.SourceKindDoc {
		t.Errorf("Handles() = %q, want %q", got, toolkit.SourceKindDoc)
	}
}

func TestDocsDriver_Ensure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := docs.NewDocsDriver(docs.WithHTTPClient(srv.Client()))
	src := &toolkit.Source{Name: "test-docs", Kind: toolkit.SourceKindDoc, URI: srv.URL}

	if err := d.Ensure(context.Background(), src); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
}

func TestDocsDriver_Ensure_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d := docs.NewDocsDriver(docs.WithHTTPClient(srv.Client()))
	src := &toolkit.Source{Name: "test-docs", Kind: toolkit.SourceKindDoc, URI: srv.URL}

	if err := d.Ensure(context.Background(), src); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDocsDriver_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body>
<div class="result">
  <a href="/documentation/openshift/4.21/networking/ptp">PTP Configuration Guide</a>
  <p>Configure PTP for telco workloads</p>
</div>
<div class="result">
  <a href="/documentation/openshift/4.21/networking/sriov">SR-IOV Configuration</a>
  <p>Configure SR-IOV for high performance networking</p>
</div>
<div class="result">
  <a href="/other/page">Not a doc link</a>
</div>
</body></html>`))
	}))
	defer srv.Close()

	d := docs.NewDocsDriver(
		docs.WithHTTPClient(srv.Client()),
		docs.WithCacheDir(t.TempDir()),
	)

	src := &toolkit.Source{
		Name: "rh-docs",
		Kind: toolkit.SourceKindDoc,
		URI:  srv.URL,
		Tags: map[string]string{
			"product": "Red Hat OpenShift Container Platform",
			"version": "4.21",
		},
	}

	results, err := d.Search(context.Background(), src, "ptp", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) < 1 {
		t.Fatal("expected at least 1 search result")
	}

	for _, r := range results {
		if r.Source != "rh-docs" {
			t.Errorf("result source = %q, want %q", r.Source, "rh-docs")
		}
		if r.Path == "" {
			t.Error("result path is empty")
		}
	}
}

func TestDocsDriver_Search_Cached(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte(`<a href="/documentation/test">test doc</a>`))
	}))
	defer srv.Close()

	d := docs.NewDocsDriver(
		docs.WithHTTPClient(srv.Client()),
		docs.WithCacheTTL(5*time.Minute),
		docs.WithCacheDir(t.TempDir()),
	)

	src := &toolkit.Source{Name: "docs", Kind: toolkit.SourceKindDoc, URI: srv.URL}

	// First call hits server
	_, _ = d.Search(context.Background(), src, "test", 10)
	if callCount != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", callCount)
	}

	// Second call should be cached
	_, _ = d.Search(context.Background(), src, "test", 10)
	if callCount != 1 {
		t.Fatalf("expected 1 HTTP call (cached), got %d", callCount)
	}
}

func TestDocsDriver_Read(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><h1>PTP Guide</h1><p>Configure PTP using the following steps:</p><ol><li>Install the operator</li><li>Create a PtpConfig</li></ol></body></html>`))
	}))
	defer srv.Close()

	d := docs.NewDocsDriver(
		docs.WithHTTPClient(srv.Client()),
		docs.WithCacheDir(t.TempDir()),
	)

	src := &toolkit.Source{Name: "docs", Kind: toolkit.SourceKindDoc, URI: srv.URL}

	content, err := d.Read(context.Background(), src, "/networking/ptp")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	text := string(content)
	if text == "" {
		t.Fatal("expected non-empty content")
	}

	// Should contain text without HTML tags
	if !containsSubstring(text, "PTP Guide") {
		t.Errorf("expected text to contain 'PTP Guide', got:\n%s", text)
	}
	if containsSubstring(text, "<html>") || containsSubstring(text, "<body>") {
		t.Errorf("expected plain text, got HTML:\n%s", text)
	}
}

func TestDocsDriver_List(t *testing.T) {
	d := docs.NewDocsDriver()
	src := &toolkit.Source{Name: "docs", Kind: toolkit.SourceKindDoc, URI: "https://docs.example.com"}

	entries, err := d.List(context.Background(), src, ".", 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if entries != nil {
		t.Errorf("List should return nil for doc sources, got %v", entries)
	}
}

func TestDocsDriver_Interface(t *testing.T) {
	// Compile-time interface satisfaction check.
	var _ toolkit.Driver = docs.NewDocsDriver()
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || s != "" && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
