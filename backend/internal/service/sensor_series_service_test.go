package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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

// buildImportPoints creates hourly ph/do observations for hours 0..8. Hours
// listed in missingPh force ph to nil; the recipe phases are each two hours.
func buildImportPoints(missingPh ...int) json.RawMessage {
	base := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	points := make([]map[string]any, 0, 9)
	missing := map[int]bool{}
	for _, hour := range missingPh {
		missing[hour] = true
	}
	for hour := 0; hour <= 8; hour++ {
		phValue := any(6.9 - float64(hour)*0.02)
		if missing[hour] {
			phValue = nil
		}
		points = append(points, map[string]any{
			"timestamp": base.Add(time.Duration(hour) * time.Hour).Format(time.RFC3339),
			"values":    map[string]any{"ph": phValue, "do": 66 - float64(hour)},
		})
	}
	raw, _ := json.Marshal(points)
	return raw
}

func seedSeriesFixture(t *testing.T) (context.Context, *SensorSeriesService, util.Actor) {
	t.Helper()
	ctx := context.Background()
	db := newTestDB(t)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	vessel := model.FermentationVessel{
		VesselCode: "FV-Q1", Name: "Quality vessel", WorkingVolumeL: 300,
		SensorChannels: `["ph","do"]`, Location: "Lab", OwnerTeam: "Process",
		VesselState: "active", CommissionedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.NewFermentationVesselRepository(db).Create(ctx, &vessel); err != nil {
		t.Fatal(err)
	}
	boundaries, references, tolerances := testRecipeConfig(t)
	recipe := model.CultureRecipe{
		VesselID: vessel.ID, RecipeCode: "QUALITY-A", Version: 1, Organism: "Test organism",
		TargetDurationH: 8, PhaseBoundariesJSON: string(boundaries), ReferenceCurvesJSON: string(references),
		ToleranceProfileJSON: string(tolerances), RecipeState: string(constants.RecipePublished),
		CreatedBy: 8, CreatedByName: "scientist", CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.NewCultureRecipeRepository(db).Create(ctx, &recipe); err != nil {
		t.Fatal(err)
	}
	svc := NewSensorSeriesService(
		repository.NewSensorSeriesRepository(db), repository.NewCultureRecipeRepository(db),
		repository.NewFermentationVesselRepository(db), repository.NewAuditRepository(db),
	)
	actor := util.Actor{UserID: 9, Username: "analyst", Role: "data_analyst", RequestID: "req-quality"}
	return ctx, svc, actor
}

func containsAll(text string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			return false
		}
	}
	return true
}

func TestValidationRejectsGrowthBatchAndPreservesPhaseEvidence(t *testing.T) {
	ctx, svc, actor := seedSeriesFixture(t)
	// Growth spans hours 2-4 (three observations). Hours 3 and 4 missing is
	// ~22% whole-run but ~67% within growth, so the strict gate must reject.
	imported, err := svc.Import(ctx, dto.ImportSensorSeriesRequest{
		VesselID: 1, RecipeID: 1, RunCode: "RUN-Q1", Channel: "multichannel",
		SampleIntervalS: 3600, PointsJSON: buildImportPoints(3, 4),
	}, actor)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	_, err = svc.Transition(ctx, imported.ID, dto.SensorSeriesTransitionRequest{ToState: "validated"}, actor)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("transition error=%v, want 422 quality rejection", err)
	}
	if !containsAll(appErr.Message, "growth", "ph") {
		t.Fatalf("error=%q must name growth phase and ph channel", appErr.Message)
	}
	rejected, err := svc.Get(ctx, imported.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.SeriesState != string(constants.SeriesRejected) {
		t.Fatalf("state=%s, want rejected", rejected.SeriesState)
	}
	var parsed timeseries.QualitySummary
	if err := json.Unmarshal(rejected.QualitySummary, &parsed); err != nil {
		t.Fatalf("rejected series lost structured quality summary: %v", err)
	}
	if parsed.Valid {
		t.Fatal("retained summary must stay marked invalid")
	}
	var growth *timeseries.PhaseMissingSummary
	for i := range parsed.PhaseMissing {
		if parsed.PhaseMissing[i].Phase == "growth" {
			growth = &parsed.PhaseMissing[i]
		}
	}
	if growth == nil || growth.MissingRate["ph"] < 0.66 || growth.WorstChannel != "ph" {
		t.Fatalf("growth evidence=%+v, want ph at ~67%%", growth)
	}
	if !containsAll(parsed.RejectionReason, "growth", "ph") {
		t.Fatalf("rejection_reason=%q must retain phase and channel", parsed.RejectionReason)
	}
}

func TestValidationPassesWhenCriticalPhasesAreComplete(t *testing.T) {
	ctx, svc, actor := seedSeriesFixture(t)
	// A single lag gap stays under the 35% whole-run gate and does not touch
	// growth or production.
	imported, err := svc.Import(ctx, dto.ImportSensorSeriesRequest{
		VesselID: 1, RecipeID: 1, RunCode: "RUN-Q2", Channel: "multichannel",
		SampleIntervalS: 3600, PointsJSON: buildImportPoints(1),
	}, actor)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	validated, err := svc.Transition(ctx, imported.ID, dto.SensorSeriesTransitionRequest{ToState: "validated"}, actor)
	if err != nil {
		t.Fatalf("validation should pass with complete critical phases: %v", err)
	}
	if validated.SeriesState != string(constants.SeriesValidated) {
		t.Fatalf("state=%s, want validated", validated.SeriesState)
	}
	var parsed timeseries.QualitySummary
	if err := json.Unmarshal(validated.QualitySummary, &parsed); err != nil {
		t.Fatalf("validated quality summary: %v", err)
	}
	if len(parsed.PhaseMissing) != 4 {
		t.Fatalf("phase groups=%d, want four", len(parsed.PhaseMissing))
	}
}
