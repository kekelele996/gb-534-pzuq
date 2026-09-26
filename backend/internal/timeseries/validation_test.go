package timeseries

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

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

func TestValidateReportsPerPhaseMissingRates(t *testing.T) {
	raw := buildPhasePoints(map[float64]map[string]*float64{})
	phases := fourPhases()
	_, summary, err := Validate(raw, "multichannel", 7200, phases)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(summary.PhaseMissing) != 4 {
		t.Fatalf("phase summaries=%d, want 4", len(summary.PhaseMissing))
	}
	expectedCounts := map[string]int{"lag": 3, "growth": 4, "production": 6, "harvest": 3}
	for _, phase := range summary.PhaseMissing {
		if phase.ObservedPointCount != expectedCounts[phase.Phase] {
			t.Fatalf("phase %s observed=%d, want %d", phase.Phase, phase.ObservedPointCount, expectedCounts[phase.Phase])
		}
		if phase.WorstChannel != "do" || phase.WorstMissingRate != 0 {
			t.Fatalf("phase %s worst=%s %.2f, want do at 0", phase.Phase, phase.WorstChannel, phase.WorstMissingRate)
		}
		for channel, rate := range phase.MissingRate {
			if rate != 0 {
				t.Fatalf("phase %s channel %s rate=%.2f, want 0", phase.Phase, channel, rate)
			}
		}
	}
	if !summary.Valid {
		t.Fatalf("complete observations must stay valid: %v", summary.Warnings)
	}
}

func TestValidateRejectsGrowthPhaseMissingBatch(t *testing.T) {
	// Two missing growth samples: whole-run rate is ~15% but growth ph is 50%.
	missing := map[float64]map[string]*float64{
		6: {"ph": nil},
		8: {"ph": nil},
	}
	raw := buildPhasePoints(missing)
	_, summary, err := Validate(raw, "multichannel", 7200, fourPhases())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if summary.MissingRate["ph"] > 0.35 {
		t.Fatalf("whole-run rate %.2f should remain under the 35%% gate", summary.MissingRate["ph"])
	}
	if summary.Valid {
		t.Fatal("growth phase channel above 20% must reject the series")
	}
	var growth *PhaseMissingSummary
	for i := range summary.PhaseMissing {
		if summary.PhaseMissing[i].Phase == "growth" {
			growth = &summary.PhaseMissing[i]
		}
	}
	if growth == nil || growth.MissingRate["ph"] != 0.5 || growth.WorstChannel != "ph" {
		t.Fatalf("growth summary=%+v, want ph 50%% as worst channel", growth)
	}
	if !containsWarning(summary.Warnings, "growth", "ph") {
		t.Fatalf("warnings=%v must name growth phase and ph channel", summary.Warnings)
	}
}

func TestValidateRejectsProductionPhaseMissingBatch(t *testing.T) {
	// Production window has six observations; two missing do values is 33%.
	missing := map[float64]map[string]*float64{
		12: {"do": nil},
		16: {"do": nil},
	}
	raw := buildPhasePoints(missing)
	_, summary, err := Validate(raw, "multichannel", 7200, fourPhases())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if summary.Valid || !containsWarning(summary.Warnings, "production", "do") {
		t.Fatalf("production do above 20%% must reject: valid=%v warnings=%v", summary.Valid, summary.Warnings)
	}
}

func TestValidateAllowsLagAndHarvestAboveCriticalLimit(t *testing.T) {
	// One missing lag sample is 33%: above the 20% critical-phase limit but
	// lag still relies on the whole-run 35% gate.
	missing := map[float64]map[string]*float64{2: {"ph": nil}}
	raw := buildPhasePoints(missing)
	_, summary, err := Validate(raw, "multichannel", 7200, fourPhases())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !summary.Valid {
		t.Fatalf("lag-phase gaps must not trip the critical-phase gate: %v", summary.Warnings)
	}
	for _, phase := range summary.PhaseMissing {
		if phase.Phase == "lag" && phase.MissingRate["ph"] < 0.33 {
			t.Fatalf("lag ph rate=%.2f, want ~33%% evidence retained", phase.MissingRate["ph"])
		}
	}
}

func TestValidateRejectsPhaseWithoutObservations(t *testing.T) {
	raw := []byte(`[
		{"timestamp":"2026-08-20T00:00:00Z","values":{"ph":6.9,"do":60}},
		{"timestamp":"2026-08-20T02:00:00Z","values":{"ph":6.8,"do":58}},
		{"timestamp":"2026-08-20T22:00:00Z","values":{"ph":6.5,"do":52}},
		{"timestamp":"2026-08-21T00:00:00Z","values":{"ph":6.4,"do":50}}
	]`)
	_, summary, err := Validate(raw, "multichannel", 7200, fourPhases())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if summary.Valid || !containsWarning(summary.Warnings, "growth", "") {
		t.Fatalf("an unobserved growth phase must reject: valid=%v warnings=%v", summary.Valid, summary.Warnings)
	}
}

func fourPhases() []PhaseWindow {
	return []PhaseWindow{
		{Phase: "lag", StartHour: 0, EndHour: 4},
		{Phase: "growth", StartHour: 4, EndHour: 10},
		{Phase: "production", StartHour: 10, EndHour: 20},
		{Phase: "harvest", StartHour: 20, EndHour: 24},
	}
}

// buildPhasePoints creates 13 two-hourly observations from 0h to 24h. The
// overrides map can force channel values to nil at selected elapsed hours.
func buildPhasePoints(overrides map[float64]map[string]*float64) []byte {
	points := make([]string, 0, 13)
	for hour := 0.0; hour <= 24; hour += 2 {
		ph, do := 6.8-hour*0.02, 68-hour*1.4
		phValue, doValue := fmt.Sprintf("%.2f", ph), fmt.Sprintf("%.2f", do)
		if override, ok := overrides[hour]; ok {
			if value, exists := override["ph"]; exists {
				phValue = nullText(value)
			}
			if value, exists := override["do"]; exists {
				doValue = nullText(value)
			}
		}
		timestamp := time.Date(2026, 8, 20, int(hour), 0, 0, 0, time.UTC).Format(time.RFC3339)
		points = append(points, fmt.Sprintf(`{"timestamp":%q,"values":{"ph":%s,"do":%s}}`, timestamp, phValue, doValue))
	}
	return []byte("[" + strings.Join(points, ",") + "]")
}

func nullText(value *float64) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprintf("%v", *value)
}

func containsWarning(warnings []string, phase, channel string) bool {
	for _, warning := range warnings {
		if !strings.Contains(warning, phase+" phase") {
			continue
		}
		if channel == "" || strings.Contains(warning, "channel "+channel) {
			return true
		}
	}
	return false
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
