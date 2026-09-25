package timeseries

import (
	"encoding/json"
	"math"
	"testing"
)

func TestValidateSortsDeduplicatesAndReportsQuality(t *testing.T) {
	raw := []byte(`[
		{"timestamp":"2026-08-20T00:02:00Z","values":{"ph":6.7,"do":55}},
		{"timestamp":"2026-08-20T00:00:00Z","values":{"ph":6.9,"do":60}},
		{"timestamp":"2026-08-20T00:01:00Z","values":{"ph":6.8,"do":58}},
		{"timestamp":"2026-08-20T00:01:00Z","values":{"ph":6.75,"do":57}},
		{"timestamp":"2026-08-20T00:10:00Z","values":{"ph":null,"do":50}}
	]`)
	points, summary, err := Validate(raw, "multichannel", 60)
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
	_, summary, err := Validate(raw, "multichannel", 60)
	if err != nil {
		t.Fatalf("format should parse before quality rejection: %v", err)
	}
	if summary.Valid || summary.MissingRate["ph"] != 0.75 {
		t.Fatalf("quality=%+v, want invalid with 75%% missing", summary)
	}
}

func TestNormalizeUsesMedianAndIQRWithoutFillingMissing(t *testing.T) {
	raw := []byte(`[
		{"timestamp":"2026-08-20T00:00:00Z","values":{"ph":1}},
		{"timestamp":"2026-08-20T00:01:00Z","values":{"ph":2}},
		{"timestamp":"2026-08-20T00:02:00Z","values":{"ph":3}},
		{"timestamp":"2026-08-20T00:03:00Z","values":{"ph":100}},
		{"timestamp":"2026-08-20T00:04:00Z","values":{"ph":null}}
	]`)
	points, _, err := Validate(raw, "ph", 60)
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
