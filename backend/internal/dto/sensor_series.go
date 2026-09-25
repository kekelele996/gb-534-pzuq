package dto
import (
	"encoding/json"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"strings"
	"time"
)
type ImportSensorSeriesRequest struct {
	VesselID        uint            `json:"vessel_id" binding:"required"`
	RecipeID        uint            `json:"recipe_id" binding:"required"`
	RunCode         string          `json:"run_code" binding:"required,min=2,max=80"`
	Channel         string          `json:"channel" binding:"required,min=1,max=120"`
	SampleIntervalS int             `json:"sample_interval_s" binding:"required,gte=1,lte=86400"`
	PointsJSON      json.RawMessage `json:"points_json" binding:"required"`
}
func (r *ImportSensorSeriesRequest) Normalize() {
	r.RunCode = strings.ToUpper(strings.TrimSpace(r.RunCode))
	r.Channel = strings.ToLower(strings.TrimSpace(r.Channel))
}
type SensorSeriesTransitionRequest struct {
	ToState string `json:"to_state" binding:"required,oneof=validated normalized ready rejected superseded"`
	Comment string `json:"comment" binding:"omitempty,max=500"`
}
type SensorSeriesQuery struct {
	VesselID, RecipeID     uint
	Search, Channel, State string
	Page, PageSize         int
}
type SensorSeriesResponse struct {
	ID                uint                        `json:"id"`
	VesselID          uint                        `json:"vessel_id"`
	RecipeID          uint                        `json:"recipe_id"`
	RunCode           string                      `json:"run_code"`
	Channel           string                      `json:"channel"`
	SampleIntervalS   int                         `json:"sample_interval_s"`
	PointsJSON        json.RawMessage             `json:"points_json"`
	StartedAt         time.Time                   `json:"started_at"`
	EndedAt           time.Time                   `json:"ended_at"`
	SourceChecksum    string                      `json:"source_checksum"`
	SeriesState       string                      `json:"series_state"`
	QualitySummary    json.RawMessage             `json:"quality_summary"`
	NormalizationJSON json.RawMessage             `json:"normalization_json"`
	ImportedBy        uint                        `json:"imported_by"`
	ImportedByName    string                      `json:"imported_by_name"`
	Vessel            *FermentationVesselResponse `json:"vessel,omitempty"`
	Recipe            *CultureRecipeResponse      `json:"recipe,omitempty"`
	CreatedAt         time.Time                   `json:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at"`
}
type SensorSeriesListResponse struct {
	Items []SensorSeriesResponse `json:"items"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Size  int                    `json:"page_size"`
}
func NewSensorSeriesResponse(series model.SensorSeries) SensorSeriesResponse {
	response := SensorSeriesResponse{
		ID: series.ID, VesselID: series.VesselID, RecipeID: series.RecipeID,
		RunCode: series.RunCode, Channel: series.Channel, SampleIntervalS: series.SampleIntervalS,
		PointsJSON: rawJSON(series.PointsJSON), StartedAt: series.StartedAt, EndedAt: series.EndedAt,
		SourceChecksum: series.SourceChecksum, SeriesState: series.SeriesState,
		QualitySummary: rawJSON(series.QualitySummary), NormalizationJSON: rawJSON(series.NormalizationJSON),
		ImportedBy: series.ImportedBy, ImportedByName: series.ImportedByName,
		CreatedAt: series.CreatedAt, UpdatedAt: series.UpdatedAt,
	}
	if series.Vessel.ID != 0 {
		v := NewFermentationVesselResponse(series.Vessel, model.FermentationVesselSummary{})
		response.Vessel = &v
	}
	if series.Recipe.ID != 0 {
		r := NewCultureRecipeResponse(series.Recipe)
		response.Recipe = &r
	}
	return response
}
