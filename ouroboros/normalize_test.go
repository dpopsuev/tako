package ouroboros

import (
	"math"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

func approxEq(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

func TestNormalizeProfile_MedianIsHalf(t *testing.T) {
	profiles := []ModelProfile{
		{
			Model: circuit.ModelIdentity{ModelName: "slow"},
			Dimensions: map[Dimension]float64{
				DimSpeed: 0.2,
			},
		},
		{
			Model: circuit.ModelIdentity{ModelName: "medium"},
			Dimensions: map[Dimension]float64{
				DimSpeed: 0.5,
			},
		},
		{
			Model: circuit.ModelIdentity{ModelName: "fast"},
			Dimensions: map[Dimension]float64{
				DimSpeed: 0.9,
			},
		},
	}

	target := &profiles[1]
	NormalizeProfile(target, profiles)

	// 2 out of 3 values are <= 0.5 (0.2 and 0.5), so percentile = 2/3
	want := 2.0 / 3.0
	if !approxEq(target.Dimensions[DimSpeed], want, 1e-9) {
		t.Errorf("DimSpeed percentile = %f, want %f", target.Dimensions[DimSpeed], want)
	}
}

func TestNormalizeProfile_TopIsOne(t *testing.T) {
	profiles := []ModelProfile{
		{Dimensions: map[Dimension]float64{DimSpeed: 0.1}},
		{Dimensions: map[Dimension]float64{DimSpeed: 0.5}},
		{Dimensions: map[Dimension]float64{DimSpeed: 0.9}},
	}

	target := &profiles[2]
	NormalizeProfile(target, profiles)

	if !approxEq(target.Dimensions[DimSpeed], 1.0, 1e-9) {
		t.Errorf("DimSpeed percentile = %f, want 1.0 (highest)", target.Dimensions[DimSpeed])
	}
}

func TestNormalizeProfile_SingleProfile_NoChange(t *testing.T) {
	profile := ModelProfile{
		Dimensions: map[Dimension]float64{DimSpeed: 0.7},
	}

	original := profile.Dimensions[DimSpeed]
	NormalizeProfile(&profile, []ModelProfile{profile})

	if profile.Dimensions[DimSpeed] != original {
		t.Errorf("single profile should not be normalized, got %f want %f",
			profile.Dimensions[DimSpeed], original)
	}
}

func TestIsStale_DifferentBattery(t *testing.T) {
	profile := ModelProfile{BatteryVersion: "ouroboros-v1"}
	if !IsStale(profile, "ouroboros-v2") {
		t.Error("should be stale with different battery version")
	}
}

func TestIsStale_SameBattery(t *testing.T) {
	profile := ModelProfile{BatteryVersion: "ouroboros-v1"}
	if IsStale(profile, "ouroboros-v1") {
		t.Error("should not be stale with same battery version")
	}
}

func TestCompareVersions_ShowsDelta(t *testing.T) {
	profiles := []ModelProfile{
		{
			Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Dimensions: map[Dimension]float64{
				DimSpeed:        0.3,
				DimEvidenceDepth: 0.5,
			},
		},
		{
			Timestamp: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			Dimensions: map[Dimension]float64{
				DimSpeed:        0.7,
				DimEvidenceDepth: 0.4,
			},
		},
	}

	deltas := CompareVersions(profiles)

	found := map[Dimension]DimensionDelta{}
	for _, d := range deltas {
		found[d.Dimension] = d
	}

	if d, ok := found[DimSpeed]; !ok {
		t.Error("missing DimSpeed delta")
	} else if !approxEq(d.Delta, 0.4, 1e-9) {
		t.Errorf("DimSpeed delta = %f, want 0.4", d.Delta)
	}

	if d, ok := found[DimEvidenceDepth]; !ok {
		t.Error("missing DimEvidenceDepth delta")
	} else if !approxEq(d.Delta, -0.1, 1e-9) {
		t.Errorf("DimEvidenceDepth delta = %f, want -0.1", d.Delta)
	}
}

func TestCompareVersions_TooFewProfiles(t *testing.T) {
	deltas := CompareVersions([]ModelProfile{{Dimensions: map[Dimension]float64{DimSpeed: 0.5}}})
	if deltas != nil {
		t.Errorf("expected nil for single profile, got %v", deltas)
	}
}
