package dto
import (
	"encoding/json"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"strings"
	"time"
)
type CreateCultureRecipeRequest struct {
	VesselID             uint            `json:"vessel_id" binding:"required"`
	RecipeCode           string          `json:"recipe_code" binding:"required,min=2,max=60"`
	Organism             string          `json:"organism" binding:"required,min=2,max=160"`
	TargetDurationH      float64         `json:"target_duration_h" binding:"required,gt=0,lte=10000"`
	PhaseBoundariesJSON  json.RawMessage `json:"phase_boundaries_json" binding:"required"`
	ReferenceCurvesJSON  json.RawMessage `json:"reference_curves_json" binding:"required"`
	ToleranceProfileJSON json.RawMessage `json:"tolerance_profile_json" binding:"required"`
}
func (r *CreateCultureRecipeRequest) Normalize() {
	r.RecipeCode = strings.ToUpper(strings.TrimSpace(r.RecipeCode))
	r.Organism = strings.TrimSpace(r.Organism)
}
type UpdateCultureRecipeRequest struct {
	Version              int              `json:"version" binding:"required,gte=1"`
	Organism             *string          `json:"organism" binding:"omitempty,min=2,max=160"`
	TargetDurationH      *float64         `json:"target_duration_h" binding:"omitempty,gt=0,lte=10000"`
	PhaseBoundariesJSON  *json.RawMessage `json:"phase_boundaries_json"`
	ReferenceCurvesJSON  *json.RawMessage `json:"reference_curves_json"`
	ToleranceProfileJSON *json.RawMessage `json:"tolerance_profile_json"`
}
func (r *UpdateCultureRecipeRequest) Normalize() { r.Organism = trimPointer(r.Organism) }
type CultureRecipeTransitionRequest struct {
	ToState string `json:"to_state" binding:"required,oneof=draft validated published obsolete"`
	Version int    `json:"version" binding:"required,gte=1"`
	Comment string `json:"comment" binding:"omitempty,max=500"`
}
type CopyCultureRecipeRequest struct {
	VesselID uint   `json:"vessel_id"`
	Comment  string `json:"comment" binding:"omitempty,max=500"`
}
type CultureRecipeQuery struct {
	VesselID       uint
	Search, State  string
	Page, PageSize int
}
type CultureRecipeResponse struct {
	ID                   uint                        `json:"id"`
	VesselID             uint                        `json:"vessel_id"`
	Vessel               *FermentationVesselResponse `json:"vessel,omitempty"`
	RecipeCode           string                      `json:"recipe_code"`
	Version              int                         `json:"version"`
	Organism             string                      `json:"organism"`
	TargetDurationH      float64                     `json:"target_duration_h"`
	PhaseBoundariesJSON  json.RawMessage             `json:"phase_boundaries_json"`
	ReferenceCurvesJSON  json.RawMessage             `json:"reference_curves_json"`
	ToleranceProfileJSON json.RawMessage             `json:"tolerance_profile_json"`
	RecipeState          string                      `json:"recipe_state"`
	CreatedBy            uint                        `json:"created_by"`
	CreatedByName        string                      `json:"created_by_name"`
	CreatedAt            time.Time                   `json:"created_at"`
	UpdatedAt            time.Time                   `json:"updated_at"`
}
type CultureRecipeListResponse struct {
	Items []CultureRecipeResponse `json:"items"`
	Total int64                   `json:"total"`
	Page  int                     `json:"page"`
	Size  int                     `json:"page_size"`
}
func NewCultureRecipeResponse(recipe model.CultureRecipe) CultureRecipeResponse {
	response := CultureRecipeResponse{
		ID: recipe.ID, VesselID: recipe.VesselID, RecipeCode: recipe.RecipeCode, Version: recipe.Version,
		Organism: recipe.Organism, TargetDurationH: recipe.TargetDurationH,
		PhaseBoundariesJSON: rawJSON(recipe.PhaseBoundariesJSON), ReferenceCurvesJSON: rawJSON(recipe.ReferenceCurvesJSON),
		ToleranceProfileJSON: rawJSON(recipe.ToleranceProfileJSON), RecipeState: recipe.RecipeState,
		CreatedBy: recipe.CreatedBy, CreatedByName: recipe.CreatedByName,
		CreatedAt: recipe.CreatedAt, UpdatedAt: recipe.UpdatedAt,
	}
	if recipe.Vessel.ID != 0 {
		v := NewFermentationVesselResponse(recipe.Vessel, model.FermentationVesselSummary{})
		response.Vessel = &v
	}
	return response
}
func rawJSON(value string) json.RawMessage {
	if !json.Valid([]byte(value)) {
		return json.RawMessage("null")
	}
	return json.RawMessage(value)
}
