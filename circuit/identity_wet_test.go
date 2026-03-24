//go:build wet

package framework_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dpopsuev/origami/models"
)

func TestWetIdentityProbe_CheckRegistry(t *testing.T) {
	raw := os.Getenv("IDENTITY_PROBE_JSON")
	if raw == "" {
		data, err := os.ReadFile("testdata/probe_result.json")
		if err != nil {
			t.Skip("no probe result available: set IDENTITY_PROBE_JSON or create testdata/probe_result.json")
		}
		raw = string(data)
	}

	var mi ModelIdentity
	if err := json.Unmarshal([]byte(raw), &mi); err != nil {
		t.Fatalf("failed to parse probe result: %v\nraw: %s", err, raw)
	}

	t.Logf("Probed identity: model_name=%q provider=%q version=%q wrapper=%q",
		mi.ModelName, mi.Provider, mi.Version, mi.Wrapper)
	t.Logf("String: %s", mi.String())
	t.Logf("Tag:    %s", mi.Tag())

	if models.IsWrapperName(mi.ModelName) {
		t.Fatalf("WRAPPER DETECTED as model_name: %q\n\n"+
			"The probe returned a wrapper identity, not the foundation model.\n"+
			"The probe prompt needs further hardening, or the model can't self-identify.\n"+
			"Wrapper: %q, Raw: %s",
			mi.ModelName, mi.ModelName, mi.Raw)
	}

	if mi.Wrapper == "" {
		t.Log("WARNING: wrapper field is empty — model may not know it's wrapped")
	}

	if !models.IsKnownModel(mi) {
		t.Fatalf("UNKNOWN FOUNDATION MODEL detected: %s\n\n"+
			"Add this entry to models/registry.go:\n\n"+
			"\t%q: {ModelName: %q, Provider: %q, Version: %q},\n",
			mi.String(), mi.ModelName, mi.ModelName, mi.Provider, mi.Version)
	}

	known, _ := models.LookupModel(mi.ModelName)
	if known.Provider != mi.Provider {
		t.Errorf("provider mismatch: registry has %q, probe returned %q",
			known.Provider, mi.Provider)
	}
	if known.Version != "" && mi.Version != "" && known.Version != mi.Version {
		t.Logf("version drift: registry has %q, probe returned %q", known.Version, mi.Version)
	}

	t.Logf("Foundation model %s is registered and verified", mi.String())
}

func TestWetIdentityProbe_RegistryNotEmpty(t *testing.T) {
	if len(models.DefaultModelRegistry().Models()) == 0 {
		t.Fatal("model registry is empty -- run a live probe first and populate it")
	}
}
