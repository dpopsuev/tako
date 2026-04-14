package telemetry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOTelObserver_SpanTree(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")
	obs := NewOTelObserver(tracer)

	obs.StartWalk("test-circuit", attribute.String("element", "fire"))

	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "recall", Elapsed: 100 * time.Millisecond})
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventTransition, Edge: "e1", Node: "triage"})
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage", Walker: "w1"})
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "triage", Elapsed: 200 * time.Millisecond})
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkComplete})

	spans := exporter.GetSpans()
	if len(spans) < 3 {
		t.Fatalf("expected at least 3 spans (walk + 2 nodes), got %d", len(spans))
	}

	var walkSpan, recallSpan, triageSpan *tracetest.SpanStub
	for i := range spans {
		switch spans[i].Name {
		case "circuit.walk":
			walkSpan = &spans[i]
		case "node.visit":
			for _, a := range spans[i].Attributes {
				if a.Key == "node" {
					switch a.Value.AsString() {
					case "recall":
						recallSpan = &spans[i]
					case "triage":
						triageSpan = &spans[i]
					}
				}
			}
		}
	}

	if walkSpan == nil {
		t.Fatal("missing circuit.walk root span")
	}
	if recallSpan == nil {
		t.Fatal("missing recall node span")
	}
	if triageSpan == nil {
		t.Fatal("missing triage node span")
	}

	// Node spans should be children of walk span
	if recallSpan.Parent.TraceID() != walkSpan.SpanContext.TraceID() {
		t.Error("recall span not child of walk span")
	}
	if triageSpan.Parent.TraceID() != walkSpan.SpanContext.TraceID() {
		t.Error("triage span not child of walk span")
	}

	// Walk span should have transition event
	foundTransition := false
	for _, ev := range walkSpan.Events {
		if ev.Name == "edge.transition" {
			foundTransition = true
		}
	}
	if !foundTransition {
		t.Error("walk span missing edge.transition event")
	}
}

func TestOTelObserver_WalkError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")
	obs := NewOTelObserver(tracer)

	obs.StartWalk("error-circuit")
	obs.OnEvent(&circuit.WalkEvent{
		Type:  circuit.EventWalkError,
		Error: fmt.Errorf("node failed"),
	})

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	foundError := false
	for _, ev := range spans[0].Events {
		if ev.Name == "exception" {
			foundError = true
		}
	}
	if !foundError {
		t.Error("walk error span missing recorded error event")
	}
}

func TestPrometheusCollector_Metrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	col := NewPrometheusCollector(reg)

	col.StartWalk("my-circuit")

	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall"})
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "recall", Elapsed: 150 * time.Millisecond})
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventTransition, Node: "recall", Edge: "e1",
		Metadata: map[string]any{"from": "recall", "to": "triage"}})
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage"})
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "triage", Elapsed: 200 * time.Millisecond})
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkComplete})

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	findMetric := func(name string) *dto.MetricFamily {
		for _, f := range families {
			if f.GetName() == name {
				return f
			}
		}
		return nil
	}

	// Node duration histogram should have 2 observations
	dur := findMetric("origami_walk_node_duration_seconds")
	if dur == nil {
		t.Fatal("missing origami_walk_node_duration_seconds")
	}
	totalCount := uint64(0)
	for _, m := range dur.GetMetric() {
		totalCount += m.GetHistogram().GetSampleCount()
	}
	if totalCount != 2 {
		t.Errorf("node duration sample count = %d, want 2", totalCount)
	}

	// Edge transitions
	edges := findMetric("origami_walk_edge_transitions_total")
	if edges == nil {
		t.Fatal("missing origami_walk_edge_transitions_total")
	}
	edgeTotal := 0.0
	for _, m := range edges.GetMetric() {
		edgeTotal += m.GetCounter().GetValue()
	}
	if edgeTotal != 1 {
		t.Errorf("edge transitions = %v, want 1", edgeTotal)
	}

	// Walk completed
	completed := findMetric("origami_walk_completed_total")
	if completed == nil {
		t.Fatal("missing origami_walk_completed_total")
	}
	completedTotal := 0.0
	for _, m := range completed.GetMetric() {
		completedTotal += m.GetCounter().GetValue()
	}
	if completedTotal != 1 {
		t.Errorf("walk completed = %v, want 1", completedTotal)
	}
}

func TestPrometheusCollector_ErrorStatus(t *testing.T) {
	reg := prometheus.NewRegistry()
	col := NewPrometheusCollector(reg)

	col.StartWalk("fail-circuit")
	col.OnEvent(&circuit.WalkEvent{
		Type:  circuit.EventWalkError,
		Error: fmt.Errorf("boom"),
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range families {
		if f.GetName() == "origami_walk_completed_total" {
			for _, m := range f.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "status" && lp.GetValue() == "error" {
						if m.GetCounter().GetValue() == 1 {
							return
						}
					}
				}
			}
		}
	}
	t.Error("expected walk_completed_total with status=error")
}

func TestDefaultObservability_ReturnsTwoObservers(t *testing.T) {
	reg := prometheus.NewRegistry()
	obs := DefaultObservabilityWithRegistry(reg)
	if len(obs) != 2 {
		t.Fatalf("expected 2 observers, got %d", len(obs))
	}

	if _, ok := obs[0].(*OTelObserver); !ok {
		t.Error("first observer should be *OTelObserver")
	}
	if _, ok := obs[1].(*PrometheusCollector); !ok {
		t.Error("second observer should be *PrometheusCollector")
	}
}

func TestPrometheusCollector_TokenMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	col := NewPrometheusCollector(reg)

	col.StartWalk("alpha-circuit")
	col.RecordTokens("recall", "recall_node", 500, 200, 0.0045)
	col.RecordTokens("triage", "triage_node", 300, 100, 0.0024)

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	findMetric := func(name string) *dto.MetricFamily {
		for _, f := range families {
			if f.GetName() == name {
				return f
			}
		}
		return nil
	}

	tokens := findMetric("origami_tokens_total")
	if tokens == nil {
		t.Fatal("missing origami_tokens_total")
	}

	totalTokens := 0.0
	for _, m := range tokens.GetMetric() {
		totalTokens += m.GetCounter().GetValue()
	}
	if totalTokens != 1100 {
		t.Errorf("total tokens = %v, want 1100 (500+200+300+100)", totalTokens)
	}

	cost := findMetric("origami_tokens_cost_usd")
	if cost == nil {
		t.Fatal("missing origami_tokens_cost_usd")
	}
	totalCost := 0.0
	for _, m := range cost.GetMetric() {
		totalCost += m.GetCounter().GetValue()
	}
	expected := 0.0045 + 0.0024
	if totalCost < expected-0.0001 || totalCost > expected+0.0001 {
		t.Errorf("total cost = %v, want ~%v", totalCost, expected)
	}
}

func TestPrometheusCollector_DispatchMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	col := NewPrometheusCollector(reg)

	col.RecordDispatch("cursor", "recall", 2*time.Second, nil)
	col.RecordDispatch("cursor", "triage", 500*time.Millisecond, fmt.Errorf("timeout"))
	col.RecordDispatch("openai", "synthesis", 1*time.Second, nil)

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	findMetric := func(name string) *dto.MetricFamily {
		for _, f := range families {
			if f.GetName() == name {
				return f
			}
		}
		return nil
	}

	dur := findMetric("origami_dispatch_duration_seconds")
	if dur == nil {
		t.Fatal("missing origami_dispatch_duration_seconds")
	}
	totalObs := uint64(0)
	for _, m := range dur.GetMetric() {
		totalObs += m.GetHistogram().GetSampleCount()
	}
	if totalObs != 3 {
		t.Errorf("dispatch duration observations = %d, want 3", totalObs)
	}

	errs := findMetric("origami_dispatch_errors_total")
	if errs == nil {
		t.Fatal("missing origami_dispatch_errors_total")
	}
	errTotal := 0.0
	for _, m := range errs.GetMetric() {
		errTotal += m.GetCounter().GetValue()
	}
	if errTotal != 1 {
		t.Errorf("dispatch errors = %v, want 1", errTotal)
	}
}

func TestHasOTelEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if HasOTelEndpoint() {
		t.Error("HasOTelEndpoint() = true with empty env, want false")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	if !HasOTelEndpoint() {
		t.Error("HasOTelEndpoint() = false with env set, want true")
	}
}

func TestPrometheusCollector_AllNineMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	col := NewPrometheusCollector(reg)

	col.StartWalk("test-circuit")
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "a"})
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "a", Elapsed: 100 * time.Millisecond})
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventTransition, Node: "a",
		Metadata: map[string]any{"from": "a", "to": "b"}})
	col.OnEvent(&circuit.WalkEvent{Type: circuit.EventWalkComplete})
	col.RecordTokens("a", "node_a", 100, 50, 0.001)
	col.RecordDispatch("default", "a", 100*time.Millisecond, nil)
	col.RecordDispatch("default", "a", 50*time.Millisecond, fmt.Errorf("fail"))
	col.LoopsTotal.WithLabelValues("test-circuit", "a").Inc()

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"origami_walk_node_duration_seconds",
		"origami_walk_edge_transitions_total",
		"origami_walk_active",
		"origami_walk_completed_total",
		"origami_walk_loops_total",
		"origami_tokens_total",
		"origami_tokens_cost_usd",
		"origami_dispatch_duration_seconds",
		"origami_dispatch_errors_total",
	}

	found := make(map[string]bool)
	for _, f := range families {
		found[f.GetName()] = true
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("missing metric %q", name)
		}
	}
}
