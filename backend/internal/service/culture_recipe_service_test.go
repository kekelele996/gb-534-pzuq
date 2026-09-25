package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"fermentation-kinetics-deviation-analysis/backend/internal/algorithm"
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"fermentation-kinetics-deviation-analysis/backend/internal/repository"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCultureRecipeLifecycleAndVersionCopy(t *testing.T) {
	db := newTestDB(t)
	vesselRepo := repository.NewFermentationVesselRepository(db)
	recipeRepo := repository.NewCultureRecipeRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	vessel := model.FermentationVessel{
		VesselCode: "FV-T1", Name: "Test vessel", WorkingVolumeL: 100,
		SensorChannels: `["ph"]`, Location: "Lab", OwnerTeam: "Process",
		VesselState: "active", CommissionedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := vesselRepo.Create(context.Background(), &vessel); err != nil {
		t.Fatal(err)
	}
	boundaries, references, tolerances := testRecipeConfig(t)
	svc := NewCultureRecipeService(recipeRepo, vesselRepo, auditRepo)
	actor := util.Actor{UserID: 1, Username: "scientist", Role: string(constants.RoleProcessScientist), RequestID: "req-test"}
	created, err := svc.Create(context.Background(), dto.CreateCultureRecipeRequest{
		VesselID: vessel.ID, RecipeCode: "TEST-A", Organism: "Test organism", TargetDurationH: 8,
		PhaseBoundariesJSON: boundaries, ReferenceCurvesJSON: references, ToleranceProfileJSON: tolerances,
	}, actor)
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	validated, err := svc.Transition(context.Background(), created.ID, dto.CultureRecipeTransitionRequest{
		ToState: "validated", Version: 1,
	}, actor)
	if err != nil || validated.RecipeState != "validated" {
		t.Fatalf("validate recipe: state=%s err=%v", validated.RecipeState, err)
	}
	published, err := svc.Transition(context.Background(), created.ID, dto.CultureRecipeTransitionRequest{
		ToState: "published", Version: 1,
	}, actor)
	if err != nil || published.RecipeState != "published" {
		t.Fatalf("publish recipe: state=%s err=%v", published.RecipeState, err)
	}
	copied, err := svc.Copy(context.Background(), created.ID, dto.CopyCultureRecipeRequest{}, actor)
	if err != nil {
		t.Fatalf("copy recipe: %v", err)
	}
	if copied.Version != 2 || copied.RecipeState != "draft" {
		t.Fatalf("copy version=%d state=%s, want 2 draft", copied.Version, copied.RecipeState)
	}
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.FermentationVessel{}, &model.CultureRecipe{},
		&model.SensorSeries{}, &model.DeviationAnalysis{}, &model.AuditLog{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func testRecipeConfig(t *testing.T) (json.RawMessage, json.RawMessage, json.RawMessage) {
	t.Helper()
	boundaries, err := json.Marshal([]algorithm.PhaseBoundary{
		{Phase: constants.PhaseLag, StartHour: 0, EndHour: 2},
		{Phase: constants.PhaseGrowth, StartHour: 2, EndHour: 4},
		{Phase: constants.PhaseProduction, StartHour: 4, EndHour: 6},
		{Phase: constants.PhaseHarvest, StartHour: 6, EndHour: 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	curves := map[string][]algorithm.CurvePoint{"ph": {}}
	for hour := 0; hour <= 8; hour++ {
		curves["ph"] = append(curves["ph"], algorithm.CurvePoint{ElapsedHour: float64(hour), Value: 7 - float64(hour)*0.05})
	}
	references, err := json.Marshal(curves)
	if err != nil {
		t.Fatal(err)
	}
	tolerances, err := json.Marshal(map[string]algorithm.ChannelTolerance{"ph": {Weight: 1, MaxDistance: 1}})
	if err != nil {
		t.Fatal(err)
	}
	return boundaries, references, tolerances
}
