package model
import "time"
type SensorSeries struct {
	ID                uint                `gorm:"primaryKey" json:"id"`
	VesselID          uint                `gorm:"not null;index" json:"vessel_id"`
	Vessel            FermentationVessel  `gorm:"foreignKey:VesselID" json:"vessel,omitempty"`
	RecipeID          uint                `gorm:"not null;index" json:"recipe_id"`
	Recipe            CultureRecipe       `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"`
	RunCode           string              `gorm:"size:80;not null;uniqueIndex" json:"run_code"`
	Channel           string              `gorm:"size:120;not null;index" json:"channel"`
	SampleIntervalS   int                 `gorm:"not null" json:"sample_interval_s"`
	PointsJSON        string              `gorm:"type:text;not null" json:"points_json"`
	StartedAt         time.Time           `gorm:"not null" json:"started_at"`
	EndedAt           time.Time           `gorm:"not null" json:"ended_at"`
	SourceChecksum    string              `gorm:"size:64;not null;index" json:"source_checksum"`
	SeriesState       string              `gorm:"size:24;not null;index" json:"series_state"`
	QualitySummary    string              `gorm:"type:text;not null" json:"quality_summary"`
	NormalizationJSON string              `gorm:"type:text;not null" json:"normalization_json"`
	ImportedBy        uint                `gorm:"not null;index" json:"imported_by"`
	ImportedByName    string              `gorm:"size:80;not null" json:"imported_by_name"`
	CreatedAt         time.Time           `gorm:"not null" json:"created_at"`
	UpdatedAt         time.Time           `gorm:"not null" json:"updated_at"`
	Analyses          []DeviationAnalysis `gorm:"foreignKey:SensorSeriesID" json:"-"`
}
func (SensorSeries) TableName() string { return "sensor_series" }
func (s SensorSeries) Ready() bool     { return s.SeriesState == "ready" }
