package constants
type SeriesState string
const (
	SeriesImported   SeriesState = "imported"
	SeriesValidated  SeriesState = "validated"
	SeriesNormalized SeriesState = "normalized"
	SeriesReady      SeriesState = "ready"
	SeriesRejected   SeriesState = "rejected"
	SeriesSuperseded SeriesState = "superseded"
)
var seriesTransitions = map[SeriesState]map[SeriesState]struct{}{
	SeriesImported:   {SeriesValidated: {}, SeriesRejected: {}},
	SeriesValidated:  {SeriesNormalized: {}, SeriesRejected: {}},
	SeriesNormalized: {SeriesReady: {}, SeriesRejected: {}},
	SeriesReady:      {SeriesSuperseded: {}},
	SeriesRejected:   {},
	SeriesSuperseded: {},
}
func (s SeriesState) Valid() bool {
	_, ok := seriesTransitions[s]
	return ok
}
func CanTransitionSeries(from, to SeriesState) bool {
	allowed, ok := seriesTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}
func SeriesStateValues() []string {
	return []string{"imported", "validated", "normalized", "ready", "rejected", "superseded"}
}
type RecipeState string
const (
	RecipeDraft     RecipeState = "draft"
	RecipeValidated RecipeState = "validated"
	RecipePublished RecipeState = "published"
	RecipeObsolete  RecipeState = "obsolete"
)
var recipeTransitions = map[RecipeState]map[RecipeState]struct{}{
	RecipeDraft:     {RecipeValidated: {}},
	RecipeValidated: {RecipeDraft: {}, RecipePublished: {}},
	RecipePublished: {RecipeObsolete: {}},
	RecipeObsolete:  {},
}
func (s RecipeState) Valid() bool {
	_, ok := recipeTransitions[s]
	return ok
}
func CanTransitionRecipe(from, to RecipeState) bool {
	allowed, ok := recipeTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}
func RecipeStateValues() []string {
	return []string{"draft", "validated", "published", "obsolete"}
}
type AnalysisState string
const (
	AnalysisQueued        AnalysisState = "queued"
	AnalysisAnalyzing     AnalysisState = "analyzing"
	AnalysisCompleted     AnalysisState = "completed"
	AnalysisFailed        AnalysisState = "failed"
	AnalysisReviewed      AnalysisState = "reviewed"
	AnalysisConfirmed     AnalysisState = "confirmed"
	AnalysisInvestigating AnalysisState = "investigating"
	AnalysisVoided        AnalysisState = "voided"
)
var analysisTransitions = map[AnalysisState]map[AnalysisState]struct{}{
	AnalysisQueued:        {AnalysisAnalyzing: {}},
	AnalysisAnalyzing:     {AnalysisCompleted: {}, AnalysisFailed: {}},
	AnalysisCompleted:     {AnalysisReviewed: {}, AnalysisVoided: {}},
	AnalysisFailed:        {AnalysisVoided: {}},
	AnalysisReviewed:      {AnalysisConfirmed: {}, AnalysisInvestigating: {}, AnalysisVoided: {}},
	AnalysisConfirmed:     {AnalysisVoided: {}},
	AnalysisInvestigating: {AnalysisReviewed: {}, AnalysisVoided: {}},
	AnalysisVoided:        {},
}
func (s AnalysisState) Valid() bool {
	_, ok := analysisTransitions[s]
	return ok
}
func CanTransitionAnalysis(from, to AnalysisState) bool {
	allowed, ok := analysisTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}
func AnalysisStateValues() []string {
	return []string{"queued", "analyzing", "completed", "failed", "reviewed", "confirmed", "investigating", "voided"}
}
