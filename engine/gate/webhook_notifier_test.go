package gate_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine/gate"
)

func TestWebhookNotifier_SendsCloudEvent(t *testing.T) {
	t.Parallel()

	var receivedBody []byte
	var receivedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier := gate.NewWebhookNotifier(srv.URL, "origami/test")

	item := gate.ApprovalItem{
		ID:         "apr-001",
		CircuitRun: "run-1",
		NodeName:   "deploy",
		Output:     json.RawMessage(`{"diff":"..."}`),
		ParkedAt:   time.Now(),
		Status:     gate.ApprovalPending,
	}

	err := notifier.Notify(context.Background(), item)
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	// Verify CloudEvents headers.
	if ct := receivedHeaders.Get("Content-Type"); ct != "application/cloudevents+json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if receivedHeaders.Get("Ce-Type") != "origami.gate.parked" {
		t.Errorf("Ce-Type = %q", receivedHeaders.Get("Ce-Type"))
	}
	if receivedHeaders.Get("Ce-Source") != "origami/test" {
		t.Errorf("Ce-Source = %q", receivedHeaders.Get("Ce-Source"))
	}

	// Verify body.
	var event struct {
		SpecVersion string `json:"specversion"`
		Type        string `json:"type"`
		Source      string `json:"source"`
		ID          string `json:"id"`
		Data        struct {
			NodeName string `json:"node_name"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(receivedBody, &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if event.SpecVersion != "1.0" {
		t.Errorf("specversion = %q", event.SpecVersion)
	}
	if event.Data.NodeName != "deploy" {
		t.Errorf("data.node_name = %q", event.Data.NodeName)
	}
}

func TestWebhookNotifier_HandlesErrorResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	notifier := gate.NewWebhookNotifier(srv.URL, "origami/test")

	err := notifier.Notify(context.Background(), gate.ApprovalItem{
		ID:       "apr-002",
		NodeName: "deploy",
		ParkedAt: time.Now(),
		Status:   gate.ApprovalPending,
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestWebhookNotifier_ImplementsNotifier(t *testing.T) {
	var _ gate.Notifier = (*gate.WebhookNotifier)(nil)
}
