package model
import "time"
type FermentationVessel struct {
	ID             uint            `gorm:"primaryKey" json:"id"`
	VesselCode     string          `gorm:"size:40;not null;uniqueIndex" json:"vessel_code"`
	Name           string          `gorm:"size:160;not null" json:"name"`
	WorkingVolumeL float64         `gorm:"not null" json:"working_volume_l"`
	SensorChannels string          `gorm:"type:text;not null" json:"sensor_channels"`
	Location       string          `gorm:"size:160;not null;index" json:"location"`
	OwnerTeam      string          `gorm:"size:120;not null;index" json:"owner_team"`
	VesselState    string          `gorm:"size:24;not null;index" json:"vessel_state"`
	CommissionedAt time.Time       `gorm:"not null" json:"commissioned_at"`
	CreatedAt      time.Time       `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"not null" json:"updated_at"`
	CultureRecipes []CultureRecipe `gorm:"foreignKey:VesselID" json:"-"`
	SensorSeries   []SensorSeries  `gorm:"foreignKey:VesselID" json:"-"`
}
func (FermentationVessel) TableName() string { return "fermentation_vessels" }
func (v FermentationVessel) Active() bool    { return v.VesselState == "active" }
type FermentationVesselSummary struct {
	RecipeCount          int64   `json:"recipe_count"`
	SeriesCount          int64   `json:"series_count"`
	ReadySeriesCount     int64   `json:"ready_series_count"`
	LatestMissingRate    float64 `json:"latest_missing_rate"`
	LatestDeviationLevel string  `json:"latest_deviation_level"`
}
