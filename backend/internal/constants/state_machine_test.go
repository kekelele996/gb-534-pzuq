package constants

import "testing"

func TestSeriesStateMachine(t *testing.T) {
	allowed := [][2]SeriesState{
		{SeriesImported, SeriesValidated}, {SeriesImported, SeriesRejected},
		{SeriesValidated, SeriesNormalized}, {SeriesValidated, SeriesRejected},
		{SeriesNormalized, SeriesReady}, {SeriesNormalized, SeriesRejected},
		{SeriesReady, SeriesSuperseded},
	}
	for _, transition := range allowed {
		if !CanTransitionSeries(transition[0], transition[1]) {
			t.Fatalf("expected series transition %s -> %s", transition[0], transition[1])
		}
	}
	if CanTransitionSeries(SeriesReady, SeriesValidated) || CanTransitionSeries(SeriesRejected, SeriesImported) {
		t.Fatal("terminal or backward series transition was accepted")
	}
}

func TestRecipeAndAnalysisStateMachines(t *testing.T) {
	if !CanTransitionRecipe(RecipeDraft, RecipeValidated) ||
		!CanTransitionRecipe(RecipeValidated, RecipePublished) ||
		!CanTransitionRecipe(RecipePublished, RecipeObsolete) {
		t.Fatal("required recipe lifecycle is incomplete")
	}
	if CanTransitionRecipe(RecipeObsolete, RecipeDraft) {
		t.Fatal("obsolete recipe was allowed to return to draft")
	}
	analysisPath := [][2]AnalysisState{
		{AnalysisQueued, AnalysisAnalyzing}, {AnalysisAnalyzing, AnalysisCompleted},
		{AnalysisCompleted, AnalysisReviewed}, {AnalysisReviewed, AnalysisConfirmed},
	}
	for _, transition := range analysisPath {
		if !CanTransitionAnalysis(transition[0], transition[1]) {
			t.Fatalf("expected analysis transition %s -> %s", transition[0], transition[1])
		}
	}
	if CanTransitionAnalysis(AnalysisQueued, AnalysisConfirmed) {
		t.Fatal("queued analysis bypassed required lifecycle")
	}
}

func TestRolePermissions(t *testing.T) {
	if !HasPermission(RoleDataAnalyst, PermissionAnalysisRun) {
		t.Fatal("data analyst cannot run analysis")
	}
	if HasPermission(RoleDataAnalyst, PermissionAnalysisConfirm) {
		t.Fatal("data analyst unexpectedly can confirm analysis")
	}
	if !HasPermission(RoleReviewer, PermissionAnalysisConfirm) {
		t.Fatal("reviewer cannot confirm analysis")
	}
	if HasPermission(RoleAuditor, PermissionVesselWrite) {
		t.Fatal("auditor unexpectedly has write access")
	}
}

func TestDeviationThresholds(t *testing.T) {
	cases := []struct {
		score float64
		level DeviationLevel
	}{
		{0.19, DeviationNormal}, {0.20, DeviationWatch},
		{0.40, DeviationMajor}, {0.65, DeviationCritical},
	}
	for _, item := range cases {
		if actual := DeviationLevelForScore(item.score); actual != item.level {
			t.Fatalf("score %.2f level = %s, want %s", item.score, actual, item.level)
		}
	}
}
