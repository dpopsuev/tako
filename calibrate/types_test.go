package calibrate_test

import (
	"testing"

	"github.com/dpopsuev/tako/calibrate"
)

func sampleMetricSet() calibrate.MetricSet {
	return calibrate.MetricSet{
		Metrics: []calibrate.Metric{
			{ID: "M1", Name: "m1", Value: 0.80, Threshold: 0.80, Pass: true, Tier: calibrate.TierOutcome},
			{ID: "M2", Name: "m2", Value: 0.50, Threshold: 0.75, Pass: false, Tier: calibrate.TierOutcome},
			{ID: "M9", Name: "m9", Value: 1.00, Threshold: 0.70, Pass: true, Tier: calibrate.TierInvestigation},
			{ID: "M12", Name: "m12", Value: 0.60, Threshold: 0.60, Pass: true, DryCapped: true, Tier: calibrate.TierDetection},
			{ID: "M14", Name: "m14", Value: 0.70, Threshold: 0.60, Pass: true, Tier: calibrate.TierEfficiency},
			{ID: "M16", Name: "m16", Value: 0.65, Threshold: 0.60, Pass: true, Tier: calibrate.TierEfficiency},
			{ID: "M19", Name: "m19", Value: 0.80, Threshold: 0.65, Pass: true, Tier: calibrate.TierMeta},
			{ID: "M20", Name: "m20", Value: 0.05, Threshold: 0.15, Pass: true, Tier: calibrate.TierMeta},
		},
	}
}

func TestAllMetrics_ReturnsFlat(t *testing.T) {
	ms := sampleMetricSet()
	all := ms.AllMetrics()
	if got := len(all); got != 8 {
		t.Fatalf("AllMetrics: want 8, got %d", got)
	}
	ids := make([]string, len(all))
	for i, m := range all {
		ids[i] = m.ID
	}
	want := []string{"M1", "M2", "M9", "M12", "M14", "M16", "M19", "M20"}
	for i, w := range want {
		if ids[i] != w {
			t.Errorf("AllMetrics[%d]: want %s, got %s", i, w, ids[i])
		}
	}
}

func TestPassCount_ExcludesDryCapped(t *testing.T) {
	ms := sampleMetricSet()
	passed, total := ms.PassCount()

	// M12 is DryCapped so excluded. Remaining: M1(pass), M2(fail), M9(pass),
	// M14(pass), M16(pass), M19(pass), M20(pass) = 6 pass / 7 total.
	if passed != 6 || total != 7 {
		t.Fatalf("PassCount: want (6, 7), got (%d, %d)", passed, total)
	}
}

func TestPassCount_Empty(t *testing.T) {
	ms := calibrate.MetricSet{}
	passed, total := ms.PassCount()
	if passed != 0 || total != 0 {
		t.Fatalf("PassCount empty: want (0, 0), got (%d, %d)", passed, total)
	}
}

func TestAllMetrics_Empty(t *testing.T) {
	ms := calibrate.MetricSet{}
	if got := len(ms.AllMetrics()); got != 0 {
		t.Fatalf("AllMetrics empty: want 0, got %d", got)
	}
}

func TestByTier(t *testing.T) {
	ms := sampleMetricSet()
	byTier := ms.ByTier()
	if got := len(byTier[calibrate.TierOutcome]); got != 2 {
		t.Errorf("TierOutcome: want 2, got %d", got)
	}
	if got := len(byTier[calibrate.TierInvestigation]); got != 1 {
		t.Errorf("TierInvestigation: want 1, got %d", got)
	}
	if got := len(byTier[calibrate.TierEfficiency]); got != 2 {
		t.Errorf("TierEfficiency: want 2, got %d", got)
	}
	if got := len(byTier[calibrate.TierMeta]); got != 2 {
		t.Errorf("TierMeta: want 2, got %d", got)
	}
}

func TestByID(t *testing.T) {
	ms := sampleMetricSet()
	byID := ms.ByID()
	if m, ok := byID["M1"]; !ok || m.Value != 0.80 {
		t.Errorf("ByID[M1]: want value 0.80, got %v", m)
	}
	if _, ok := byID["MISSING"]; ok {
		t.Error("ByID[MISSING]: should not exist")
	}
}
