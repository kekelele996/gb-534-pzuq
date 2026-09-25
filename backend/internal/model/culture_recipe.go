package model
import "time"
type CultureRecipe struct {
	ID                   uint               `gorm:"primaryKey" json:"id"`
	VesselID             uint               `gorm:"not null;index;uniqueIndex:idx_recipe_code_version" json:"vessel_id"`
	Vessel               FermentationVessel `gorm:"foreignKey:VesselID" json:"vessel,omitempty"`
	RecipeCode           string             `gorm:"size:60;not null;uniqueIndex:idx_recipe_code_version" json:"recipe_code"`
	Version              int                `gorm:"not null;uniqueIndex:idx_recipe_code_version" json:"version"`
	Organism             string             `gorm:"size:160;not null" json:"organism"`
	TargetDurationH      float64            `gorm:"not null" json:"target_duration_h"`
	PhaseBoundariesJSON  string             `gorm:"type:text;not null" json:"phase_boundaries_json"`
	ReferenceCurvesJSON  string             `gorm:"type:text;not null" json:"reference_curves_json"`
	ToleranceProfileJSON string             `gorm:"type:text;not null" json:"tolerance_profile_json"`
	RecipeState          string             `gorm:"size:24;not null;index" json:"recipe_state"`
	CreatedBy            uint               `gorm:"not null;index" json:"created_by"`
	CreatedByName        string             `gorm:"size:80;not null" json:"created_by_name"`
	CreatedAt            time.Time          `gorm:"not null" json:"created_at"`
	UpdatedAt            time.Time          `gorm:"not null" json:"updated_at"`
	SensorSeries         []SensorSeries     `gorm:"foreignKey:RecipeID" json:"-"`
}
func (CultureRecipe) TableName() string { return "culture_recipes" }
func (r CultureRecipe) Editable() bool {
	return r.RecipeState == "draft" || r.RecipeState == "validated"
}
