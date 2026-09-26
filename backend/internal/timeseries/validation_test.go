package timeseries

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func testPhases() []PhaseWindow {
	return []PhaseWindow{
		{Phase: "lag", StartHour: 0, EndHour: 4},
		{Phase: "growth", StartHour: 4, EndHour: 10},
		{Phase: "production", StartHour: 10, EndHour: 20},
		{Phase: "harvest", StartHour: 20, EndHour: 24},
	}
}

func TestValidateSortsDeduplicatesAndReportsQuality(t *testing.T) {
	raw := []byte(`[
		{"timestamp":"2026-08-20T00:02:00Z","values":{"ph":6.7,"do":55}},
		{"timestamp":"2026-08-20T00:00:00Z","values":{"ph":6.9,"do":60}},
		{"timestamp":"2026-08-20T00:01:00Z","values":{"ph":6.8,"do":58}},
		{"timestamp":"2026-08-20T00:01:00Z","values":{"ph":6.75,"do":57}},
		{"timestamp":"2026-08-20T00:10:00Z","values":{"ph":null,"do":50}}
	]`)
	points, summary, err := Validate(raw, "multichannel", 60, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(points) != 4 || summary.DuplicateCount != 1 {
		t.Fatalf("unique=%d duplicate=%d, want 4 and 1", len(points), summary.DuplicateCount)
	}
	if !points[0].Timestamp.Before(points[1].Timestamp) {
		t.Fatal("points are not sorted by timestamp")
	}
	if value := *points[1].Values["ph"]; math.Abs(value-6.75) > 1e-9 {
		t.Fatalf("deduplicated value=%v, want last observation 6.75", value)
	}
	if summary.MissingRate["ph"] != 0.25 || summary.LongGapCount != 1 {
		t.Fatalf("quality=%+v", summary)
	}
	if !summary.Valid {
		t.Fatal("25% missing rate should remain reviewable")
	}
}

func TestValidateRejectsInsufficientQuality(t *testing.T) {
	raw := []byte(`[
		{"timestamp":"2026-08-20T00:00:00Z","values":{"ph":6.9,"do":60}},
		{"timestamp":"2026-08-20T00:01:00Z","values":{"ph":null,"do":58}},
		{"timestamp":"2026-08-20T00:02:00Z","values":{"ph":null,"do":57}},
		{"timestamp":"2026-08-20T00:03:00Z","values":{"ph":null,"do":55}}
	]`)
	_, summary, err := Validate(raw, "multichannel", 60, nil)
	if err != nil {
		t.Fatalf("format should parse before quality rejection: %v", err)
	}
	if summary.Valid || summary.MissingRate["ph"] != 0.75 {
		t.Fatalf("quality=%+v, want invalid with 75%% missing", summary)
	}
}

func TestValidateReportsMissingRateForAllFourPhases(t *testing.T) {
	// Growth spans hours 4,6,8,10 (boundary points are shared); losing do at
	// the two growth-only samples leaves a 50% growth-phase rate while the
	// overall rate is only 2/13 ~= 15%.
	raw := buildSeries(t, 2, map[string][]int{"do": {6, 8}})
	_, summary, err := Validate(raw, "multichannel", 7200, testPhases())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(summary.PhaseQuality) != 4 {
		t.Fatalf("phase reports=%d, want 4", len(summary.PhaseQuality))
	}
	growth := summary.PhaseQuality[1]
	if growth.Phase != "growth" || growth.SampleCount != 4 {
		t.Fatalf("growth phase=%+v, want 4 samples", growth)
	}
	if growth.MissingRate["do"] != 0.5 || growth.MissingRate["ph"] != 0 {
		t.Fatalf("growth rates=%+v", growth.MissingRate)
	}
	if growth.WorstChannel != "do" || growth.WorstMissingRate != 0.5 {
		t.Fatalf("worst=%s %.2f", growth.WorstChannel, growth.WorstMissingRate)
	}
	if !growth.Enforced {
		t.Fatal("growth phase must be enforced")
	}
	if summary.PhaseQuality[0].Enforced || summary.PhaseQuality[3].Enforced {
		t.Fatal("lag and harvest phases must not carry the 20% phase gate")
	}
}

func TestValidateRejectsGrowthPhaseLossEvenWhenOverallIsFine(t *testing.T) {
	raw := buildSeries(t, 2, map[string][]int{"do": {6, 8}})
	_, summary, err := Validate(raw, "multichannel", 7200, testPhases())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if summary.Valid {
		t.Fatal("growth-phase channel loss above 20% must reject even below the 35% overall limit")
	}
	if !strings.Contains(summary.RejectionReason, "growth") ||
		!strings.Contains(summary.RejectionReason, "do") ||
		!strings.Contains(summary.RejectionReason, "20%") {
		t.Fatalf("rejection reason=%q must name the growth phase, channel and limit", summary.RejectionReason)
	}
}

func TestValidateRejectsProductionPhaseLoss(t *testing.T) {
	// Production spans hours 10..20 (6 samples). Losing 2/6 = 33% rejects.
	raw := buildSeries(t, 2, map[string][]int{"do": {12, 16}})
	_, summary, err := Validate(raw, "multichannel", 7200, testPhases())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if summary.Valid {
		t.Fatal("production-phase channel loss above 20% must reject")
	}
	production := summary.PhaseQuality[2]
	if production.WorstChannel != "do" || math.Abs(production.WorstMissingRate-0.333333) > 1e-9 {
		t.Fatalf("production worst=%s %.4f", production.WorstChannel, production.WorstMissingRate)
	}
	if !strings.Contains(summary.RejectionReason, "production") {
		t.Fatalf("rejection reason=%q must name production", summary.RejectionReason)
	}
}

func TestValidateAllowsGrowthLossBelowTwentyPercent(t *testing.T) {
	// Hourly samples give each phase five observations: losing exactly one
	// growth sample is 1/5 = 20%, which must pass because only rates strictly
	// above 20% reject.
	hourlyPhases := []PhaseWindow{
		{Phase: "lag", StartHour: 0, EndHour: 4},
		{Phase: "growth", StartHour: 4, EndHour: 8},
		{Phase: "production", StartHour: 8, EndHour: 12},
		{Phase: "harvest", StartHour: 12, EndHour: 16},
	}
	raw := buildSeries(t, 1, map[string][]int{"do": {5}})
	_, summary, err := Validate(raw, "multichannel", 3600, hourlyPhases)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !summary.Valid {
		t.Fatalf("exactly 20%% growth loss must pass, reason=%s", summary.RejectionReason)
	}
	if got := summary.PhaseQuality[1].MissingRate["do"]; got != 0.2 {
		t.Fatalf("growth do rate=%.2f, want 0.20", got)
	}

	// Losing two growth samples (40%) must reject.
	raw = buildSeries(t, 1, map[string][]int{"do": {5, 6}})
	_, summary, err = Validate(raw, "multichannel", 3600, hourlyPhases)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if summary.Valid || !strings.Contains(summary.RejectionReason, "growth") {
		t.Fatalf("40%% growth loss must reject: %+v", summary)
	}
}

func TestValidateLagAndHarvestUseOnlyOverallLimit(t *testing.T) {
	// ph is null at hour 2 (lag-only) and hour 22 (harvest-only). Each phase
	// rate exceeds 20%, but lag/harvest only answer to the 35% overall limit;
	// overall ph missing is 2/13 ~= 15%, so the series stays valid.
	raw := buildSeries(t, 2, map[string][]int{"ph": {2, 22}})
	_, summary, err := Validate(raw, "multichannel", 7200, testPhases())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !summary.Valid {
		t.Fatalf("lag/harvest must only use the 35%% overall limit: %s", summary.RejectionReason)
	}
	if got := summary.PhaseQuality[0].MissingRate["ph"]; math.Abs(got-1.0/3.0) > 1e-6 {
		t.Fatalf("lag ph rate=%.4f, want 33%%", got)
	}
}

func TestValidateRejectsEnforcedPhaseWithoutObservations(t *testing.T) {
	raw := buildSeries(t, 2, nil)
	// No sample falls into the growth window, so growth cannot be aligned.
	phases := []PhaseWindow{
		{Phase: "lag", StartHour: 0, EndHour: 4},
		{Phase: "growth", StartHour: 26, EndHour: 30},
		{Phase: "production", StartHour: 30, EndHour: 32},
		{Phase: "harvest", StartHour: 32, EndHour: 34},
	}
	_, summary, err := Validate(raw, "multichannel", 7200, phases)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if summary.Valid {
		t.Fatal("an unobserved enforced phase must reject")
	}
	if !strings.Contains(summary.RejectionReason, "growth") {
		t.Fatalf("reason=%q must name the empty growth phase", summary.RejectionReason)
	}
}

func buildSeries(t *testing.T, stepHours int, missing map[string][]int) []byte {
	t.Helper()
	missingAt := map[int]map[string]struct{}{}
	for channel, hours := range missing {
		for _, hour := range hours {
			if missingAt[hour] == nil {
				missingAt[hour] = map[string]struct{}{}
			}
			missingAt[hour][channel] = struct{}{}
		}
	}
	type wire struct {
		Timestamp string              `json:"timestamp"`
		Values    map[string]*float64 `json:"values"`
	}
	start := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	points := make([]wire, 0)
	for hour := 0; hour <= 24; hour += stepHours {
		ph, do := 6.8-float64(hour)*0.02, 60-float64(hour)*1.2
		values := map[string]*float64{"ph": &ph, "do": &do}
		for channel := range missingAt[hour] {
			values[channel] = nil
		}
		points = append(points, wire{
			Timestamp: start.Add(time.Duration(hour) * time.Hour).Format(time.RFC3339),
			Values:    values,
		})
	}
	raw, err := json.Marshal(points)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestNormalizeUsesMedianAndIQRWithoutFillingMissing(t *testing.T) {
	raw := []byte(`[
		{"timestamp":"2026-08-20T00:00:00Z","values":{"ph":1}},
		{"timestamp":"2026-08-20T00:01:00Z","values":{"ph":2}},
		{"timestamp":"2026-08-20T00:02:00Z","values":{"ph":3}},
		{"timestamp":"2026-08-20T00:03:00Z","values":{"ph":100}},
		{"timestamp":"2026-08-20T00:04:00Z","values":{"ph":null}}
	]`)
	points, _, err := Validate(raw, "ph", 60, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	normalized, summary, err := Normalize(points)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	stats := summary.Channels["ph"]
	if stats.Median != 2.5 || stats.IQR != 25.5 {
		t.Fatalf("stats=%+v, want median 2.5 and IQR 25.5", stats)
	}
	if normalized[len(normalized)-1].Values["ph"] != nil {
		t.Fatal("normalization silently filled a missing value")
	}
	encoded, err := json.Marshal(summary)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("normalization summary is not serializable: %v", err)
	}
}
