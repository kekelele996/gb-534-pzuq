package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"fermentation-kinetics-deviation-analysis/backend/internal/repository"
	"fermentation-kinetics-deviation-analysis/backend/internal/timeseries"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
)

type missingSpec map[string][]int64

func createSeriesValidationFixture(t *testing.T) (
	*SensorSeriesService, repository.SensorSeriesRepository, model.FermentationVessel, model.CultureRecipe,
) {
	t.Helper()
	db := newTestDB(t)
	vesselRepo := repository.NewFermentationVesselRepository(db)
	recipeRepo := repository.NewCultureRecipeRepository(db)
	seriesRepo := repository.NewSensorSeriesRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	vessel := model.FermentationVessel{
		VesselCode: "FV-P1", Name: "Phase gate vessel", WorkingVolumeL: 500,
		SensorChannels: `["ph","do"]`, Location: "Pilot", OwnerTeam: "Process",
		VesselState: "active", CommissionedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := vesselRepo.Create(context.Background(), &vessel); err != nil {
		t.Fatal(err)
	}
	boundaries, references, tolerances := testRecipeConfig(t)
	recipe := model.CultureRecipe{
		VesselID: vessel.ID, RecipeCode: "PHASE-GATE-A", Version: 1, Organism: "Test organism",
		TargetDurationH: 8, PhaseBoundariesJSON: string(boundaries), ReferenceCurvesJSON: string(references),
		ToleranceProfileJSON: string(tolerances), RecipeState: "published",
		CreatedBy: 8, CreatedByName: "scientist", CreatedAt: now, UpdatedAt: now,
	}
	if err := recipeRepo.Create(context.Background(), &recipe); err != nil {
		t.Fatal(err)
	}
	svc := NewSensorSeriesService(seriesRepo, recipeRepo, vesselRepo, auditRepo)
	return svc, seriesRepo, vessel, recipe
}

// buildSeriesPoints creates hourly observations for hours 0..8 (nine points),
// covering lag 0-2, growth 2-4, production 4-6 and harvest 6-8. Boundary hours
// are shared between adjacent phases.
func buildSeriesPoints(t *testing.T, startedAt time.Time, missing missingSpec) []timeseries.Point {
	t.Helper()
	points := make([]timeseries.Point, 0, 9)
	for hour := int64(0); hour <= 8; hour++ {
		ph, do := 6.9-float64(hour)*0.05, 60-float64(hour)*1.2
		values := map[string]*float64{"ph": &ph, "do": &do}
		for _, missingHour := range missing["ph"] {
			if missingHour == hour {
				values["ph"] = nil
			}
		}
		for _, missingHour := range missing["do"] {
			if missingHour == hour {
				values["do"] = nil
			}
		}
		points = append(points, timeseries.Point{
			Timestamp: startedAt.Add(time.Duration(hour) * time.Hour), Values: values,
		})
	}
	return points
}

func persistImportedSeries(
	t *testing.T, repo repository.SensorSeriesRepository,
	vessel model.FermentationVessel, recipe model.CultureRecipe, runCode string,
	points []timeseries.Point, state string,
) model.SensorSeries {
	t.Helper()
	pointsJSON, err := timeseries.EncodePoints(points)
	if err != nil {
		t.Fatal(err)
	}
	now := points[0].Timestamp.Add(9 * time.Hour)
	series := model.SensorSeries{
		VesselID: vessel.ID, RecipeID: recipe.ID, RunCode: runCode, Channel: "multichannel",
		SampleIntervalS: 3600, PointsJSON: pointsJSON,
		StartedAt: points[0].Timestamp, EndedAt: points[len(points)-1].Timestamp,
		SourceChecksum: util.HashString(pointsJSON), SeriesState: state,
		QualitySummary: `{}`, NormalizationJSON: "{}",
		ImportedBy: 9, ImportedByName: "analyst", CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), &series); err != nil {
		t.Fatal(err)
	}
	return series
}

func decodeQuality(t *testing.T, series model.SensorSeries) map[string]any {
	t.Helper()
	quality := map[string]any{}
	if err := json.Unmarshal([]byte(series.QualitySummary), &quality); err != nil {
		t.Fatalf("decode quality summary %q: %v", series.QualitySummary, err)
	}
	return quality
}

func TestSeriesValidationRejectsGrowthPhaseLossAndKeepsReason(t *testing.T) {
	svc, repo, vessel, recipe := createSeriesValidationFixture(t)
	startedAt := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	// ph is null at hour 3: growth spans hours 2,3,4, so the growth rate is
	// 1/3 = 33% while the overall rate is only 1/9 ~= 11%.
	points := buildSeriesPoints(t, startedAt, missingSpec{"ph": {3}})
	series := persistImportedSeries(t, repo, vessel, recipe, "RUN-PHASE-REJECT", points, string(constants.SeriesImported))
	actor := util.Actor{UserID: 9, Username: "analyst", Role: "data_analyst", RequestID: "req-reject"}
	_, err := svc.Transition(context.Background(), series.ID, dto.SensorSeriesTransitionRequest{
		ToState: "validated",
	}, actor)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.Status != 422 || appErr.Code != util.CodeValidation {
		t.Fatalf("err=%v, want 422 validation error naming the rejection", err)
	}
	if !strings.Contains(appErr.Message, "growth") || !strings.Contains(appErr.Message, "ph") {
		t.Fatalf("error message=%q must name the growth phase and ph channel", appErr.Message)
	}
	stored, err := repo.GetByID(context.Background(), series.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SeriesState != string(constants.SeriesRejected) {
		t.Fatalf("state=%s, want rejected", stored.SeriesState)
	}
	quality := decodeQuality(t, stored)
	if valid, _ := quality["valid"].(bool); valid {
		t.Fatalf("quality=%v must be marked invalid", quality)
	}
	reason, _ := quality["rejection_reason"].(string)
	if !strings.Contains(reason, "growth") || !strings.Contains(reason, "ph") || !strings.Contains(reason, "20%") {
		t.Fatalf("rejection_reason=%q must name the growth phase, ph channel and 20%% limit", reason)
	}
	phases, ok := quality["phase_quality"].([]any)
	if !ok || len(phases) != 4 {
		t.Fatalf("phase_quality=%v must list four phases", quality["phase_quality"])
	}
}

func TestSeriesValidationAllowsLagHarvestLossUnderOverallLimit(t *testing.T) {
	svc, repo, vessel, recipe := createSeriesValidationFixture(t)
	startedAt := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	// ph is null at hours 1 (lag) and 7 (harvest): each phase rate is high,
	// but lag/harvest only answer to the 35% overall limit (2/9 ~= 22%).
	points := buildSeriesPoints(t, startedAt, missingSpec{"ph": {1, 7}})
	series := persistImportedSeries(t, repo, vessel, recipe, "RUN-PHASE-PASS", points, string(constants.SeriesImported))
	actor := util.Actor{UserID: 9, Username: "analyst", Role: "data_analyst", RequestID: "req-pass"}
	validated, err := svc.Transition(context.Background(), series.ID, dto.SensorSeriesTransitionRequest{
		ToState: "validated",
	}, actor)
	if err != nil {
		t.Fatalf("lag/harvest loss below the overall limit must pass: %v", err)
	}
	if validated.SeriesState != string(constants.SeriesValidated) {
		t.Fatalf("state=%s", validated.SeriesState)
	}
}

func TestSeriesRejectionLeavesReadyBatchUntouchedAndRecomputesNewBatch(t *testing.T) {
	svc, repo, vessel, recipe := createSeriesValidationFixture(t)
	startedAt := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	readyPoints := buildSeriesPoints(t, startedAt.Add(-48*time.Hour), nil)
	readySeries := persistImportedSeries(t, repo, vessel, recipe, "RUN-PHASE-READY", readyPoints, string(constants.SeriesReady))

	// hour 5 is production-only (hours 4,5,6): 1/3 = 33% rejects.
	badPoints := buildSeriesPoints(t, startedAt, missingSpec{"ph": {5}})
	badSeries := persistImportedSeries(t, repo, vessel, recipe, "RUN-PHASE-BAD", badPoints, string(constants.SeriesImported))
	actor := util.Actor{UserID: 9, Username: "analyst", Role: "data_analyst", RequestID: "req-mixed"}
	if _, err := svc.Transition(context.Background(), badSeries.ID, dto.SensorSeriesTransitionRequest{
		ToState: "validated",
	}, actor); err == nil {
		t.Fatal("production-phase loss must reject the new batch")
	}

	storedReady, err := repo.GetByID(context.Background(), readySeries.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if storedReady.SeriesState != string(constants.SeriesReady) {
		t.Fatalf("historical ready batch state=%s, must remain ready", storedReady.SeriesState)
	}
	storedBad, err := repo.GetByID(context.Background(), badSeries.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if storedBad.SeriesState != string(constants.SeriesRejected) {
		t.Fatalf("bad batch state=%s, want rejected", storedBad.SeriesState)
	}

	// A later, complete batch is recomputed independently and validates.
	nextPoints := buildSeriesPoints(t, startedAt.Add(24*time.Hour), nil)
	nextSeries := persistImportedSeries(t, repo, vessel, recipe, "RUN-PHASE-NEXT", nextPoints, string(constants.SeriesImported))
	validated, err := svc.Transition(context.Background(), nextSeries.ID, dto.SensorSeriesTransitionRequest{
		ToState: "validated",
	}, actor)
	if err != nil {
		t.Fatalf("subsequent complete batch must recompute and validate: %v", err)
	}
	quality := map[string]any{}
	if err := json.Unmarshal(validated.QualitySummary, &quality); err != nil {
		t.Fatal(err)
	}
	if valid, _ := quality["valid"].(bool); !valid {
		t.Fatalf("subsequent batch quality=%v must be valid", quality)
	}
}
