package algorithm

import (
	"reflect"
	"testing"
	"time"

	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"fermentation-kinetics-deviation-analysis/backend/internal/timeseries"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
)

func TestDTWDeterministicFixture(t *testing.T) {
	distance, path, err := DTW([]float64{1, 2, 3, 4}, []float64{1, 2, 3, 4}, 2)
	if err != nil {
		t.Fatalf("DTW: %v", err)
	}
	if distance != 0 || len(path) != 4 {
		t.Fatalf("distance=%v path=%v", distance, path)
	}
}

func TestEvaluatorIsDeterministicAndReplayable(t *testing.T) {
	snapshot := evaluatorFixture(t)
	first, err := NewEvaluator().Evaluate(snapshot)
	if err != nil {
		t.Fatalf("first evaluate: %v", err)
	}
	second, err := NewEvaluator().Evaluate(snapshot)
	if err != nil {
		t.Fatalf("second evaluate: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("results differ:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if first.DeviationLevel != constants.DeviationNormal {
		t.Fatalf("identical fixture level=%s, want normal", first.DeviationLevel)
	}
	encoded, err := snapshot.Canonical()
	if err != nil {
		t.Fatalf("canonical snapshot: %v", err)
	}
	decoded, err := DecodeSnapshot(encoded)
	if err != nil || !reflect.DeepEqual(snapshot, decoded) {
		t.Fatalf("snapshot replay mismatch: %v", err)
	}
}

func evaluatorFixture(t *testing.T) Snapshot {
	t.Helper()
	started := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	points := make([]timeseries.Point, 0, 9)
	reference := map[string][]CurvePoint{"ph": {}}
	for hour := 0; hour <= 8; hour++ {
		value := 7.0 - float64(hour)*0.05
		valueCopy := value
		points = append(points, timeseries.Point{
			Timestamp: started.Add(time.Duration(hour) * time.Hour),
			Values:    map[string]*float64{"ph": &valueCopy},
		})
		reference["ph"] = append(reference["ph"], CurvePoint{ElapsedHour: float64(hour), Value: value})
	}
	pointsJSON, err := timeseries.EncodePoints(points)
	if err != nil {
		t.Fatal(err)
	}
	boundaries, _ := util.CanonicalJSON([]PhaseBoundary{
		{Phase: constants.PhaseLag, StartHour: 0, EndHour: 2},
		{Phase: constants.PhaseGrowth, StartHour: 2, EndHour: 4},
		{Phase: constants.PhaseProduction, StartHour: 4, EndHour: 6},
		{Phase: constants.PhaseHarvest, StartHour: 6, EndHour: 8},
	})
	references, _ := util.CanonicalJSON(reference)
	tolerances, _ := util.CanonicalJSON(map[string]ChannelTolerance{"ph": {Weight: 1, MaxDistance: 1}})
	series := model.SensorSeries{
		ID: 3, VesselID: 2, RecipeID: 4, RunCode: "FIXTURE", Channel: "ph",
		SampleIntervalS: 3600, PointsJSON: pointsJSON, SourceChecksum: util.HashString(pointsJSON),
		StartedAt: started, EndedAt: started.Add(8 * time.Hour),
	}
	recipe := model.CultureRecipe{
		ID: 4, Version: 2, TargetDurationH: 8, PhaseBoundariesJSON: boundaries,
		ReferenceCurvesJSON: references, ToleranceProfileJSON: tolerances,
	}
	return NewSnapshot(series, recipe)
}
