package dto
import (
	"encoding/json"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"sort"
	"strings"
	"time"
)
type CreateFermentationVesselRequest struct {
	VesselCode     string    `json:"vessel_code" binding:"required,min=2,max=40"`
	Name           string    `json:"name" binding:"required,min=2,max=160"`
	WorkingVolumeL float64   `json:"working_volume_l" binding:"required,gt=0,lte=10000000"`
	SensorChannels []string  `json:"sensor_channels" binding:"required,min=1,max=32,dive,required,max=64"`
	Location       string    `json:"location" binding:"required,min=2,max=160"`
	OwnerTeam      string    `json:"owner_team" binding:"required,min=2,max=120"`
	CommissionedAt time.Time `json:"commissioned_at" binding:"required"`
}
func (r *CreateFermentationVesselRequest) Normalize() {
	r.VesselCode = strings.ToUpper(strings.TrimSpace(r.VesselCode))
	r.Name = strings.TrimSpace(r.Name)
	r.Location = strings.TrimSpace(r.Location)
	r.OwnerTeam = strings.TrimSpace(r.OwnerTeam)
	r.SensorChannels = normalizeStrings(r.SensorChannels)
}
type UpdateFermentationVesselRequest struct {
	Name           *string    `json:"name" binding:"omitempty,min=2,max=160"`
	WorkingVolumeL *float64   `json:"working_volume_l" binding:"omitempty,gt=0,lte=10000000"`
	SensorChannels *[]string  `json:"sensor_channels" binding:"omitempty,min=1,max=32,dive,required,max=64"`
	Location       *string    `json:"location" binding:"omitempty,min=2,max=160"`
	OwnerTeam      *string    `json:"owner_team" binding:"omitempty,min=2,max=120"`
	CommissionedAt *time.Time `json:"commissioned_at"`
}
func (r *UpdateFermentationVesselRequest) Normalize() {
	r.Name = trimPointer(r.Name)
	r.Location = trimPointer(r.Location)
	r.OwnerTeam = trimPointer(r.OwnerTeam)
	if r.SensorChannels != nil {
		channels := normalizeStrings(*r.SensorChannels)
		r.SensorChannels = &channels
	}
}
type FermentationVesselQuery struct {
	Search, Location, OwnerTeam, State string
	Page, PageSize                     int
}
type FermentationVesselResponse struct {
	ID             uint                            `json:"id"`
	VesselCode     string                          `json:"vessel_code"`
	Name           string                          `json:"name"`
	WorkingVolumeL float64                         `json:"working_volume_l"`
	SensorChannels []string                        `json:"sensor_channels"`
	Location       string                          `json:"location"`
	OwnerTeam      string                          `json:"owner_team"`
	VesselState    string                          `json:"vessel_state"`
	CommissionedAt time.Time                       `json:"commissioned_at"`
	Summary        model.FermentationVesselSummary `json:"analysis_summary"`
	CreatedAt      time.Time                       `json:"created_at"`
	UpdatedAt      time.Time                       `json:"updated_at"`
}
type FermentationVesselListResponse struct {
	Items []FermentationVesselResponse `json:"items"`
	Total int64                        `json:"total"`
	Page  int                          `json:"page"`
	Size  int                          `json:"page_size"`
}
func NewFermentationVesselResponse(v model.FermentationVessel, summary model.FermentationVesselSummary) FermentationVesselResponse {
	var channels []string
	_ = json.Unmarshal([]byte(v.SensorChannels), &channels)
	return FermentationVesselResponse{
		ID: v.ID, VesselCode: v.VesselCode, Name: v.Name, WorkingVolumeL: v.WorkingVolumeL,
		SensorChannels: channels, Location: v.Location, OwnerTeam: v.OwnerTeam,
		VesselState: v.VesselState, CommissionedAt: v.CommissionedAt, Summary: summary,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}
func normalizeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
func trimPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
