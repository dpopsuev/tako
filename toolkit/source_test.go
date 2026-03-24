package toolkit

import (
	"encoding/json"
	"testing"
)

func TestSourceKind_Constants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		got  SourceKind
		want string
	}{
		{SourceKindRepo, "repo"},
		{SourceKindSpec, "spec"},
		{SourceKindDoc, "doc"},
		{SourceKindAPI, "api"},
	}
	for _, tc := range cases {
		if string(tc.got) != tc.want {
			t.Errorf("SourceKind = %q, want %q", tc.got, tc.want)
		}
	}
}

func TestReadPolicy_Constants(t *testing.T) {
	t.Parallel()
	if ReadAlways != "always" {
		t.Errorf("ReadAlways = %q, want %q", ReadAlways, "always")
	}
	if ReadConditional != "conditional" {
		t.Errorf("ReadConditional = %q, want %q", ReadConditional, "conditional")
	}
}

func TestResolutionStatus_AllValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		got  ResolutionStatus
		want string
	}{
		{Resolved, "resolved"},
		{Cached, "cached"},
		{Degraded, "degraded"},
		{Unavailable, "unavailable"},
		{Unknown, "unknown"},
	}
	for _, tc := range cases {
		if string(tc.got) != tc.want {
			t.Errorf("ResolutionStatus = %q, want %q", tc.got, tc.want)
		}
	}
}

func TestSource_IsAlwaysRead(t *testing.T) {
	t.Parallel()
	always := Source{Name: "a", ReadPolicy: ReadAlways}
	if !always.IsAlwaysRead() {
		t.Error("ReadAlways source should return true")
	}
	cond := Source{Name: "b", ReadPolicy: ReadConditional}
	if cond.IsAlwaysRead() {
		t.Error("ReadConditional source should return false")
	}
	empty := Source{Name: "c"}
	if empty.IsAlwaysRead() {
		t.Error("empty ReadPolicy source should return false")
	}
}

func TestSource_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	src := Source{
		Name:       "ptp-operator",
		Kind:       SourceKindRepo,
		URI:        "https://github.com/openshift/ptp-operator",
		Purpose:    "PTP operator source code",
		Branch:     "main",
		Tags:       map[string]string{"layer": "base"},
		ReadPolicy: ReadAlways,
		Resolution: Resolved,
	}
	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Source
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != src.Name || got.Kind != src.Kind || got.URI != src.URI {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
	if got.ReadPolicy != ReadAlways {
		t.Errorf("ReadPolicy = %q, want %q", got.ReadPolicy, ReadAlways)
	}
	if got.Resolution != Resolved {
		t.Errorf("Resolution = %q, want %q", got.Resolution, Resolved)
	}
}
