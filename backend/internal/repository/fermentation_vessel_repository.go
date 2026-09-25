package repository
import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"gorm.io/gorm"
)
type FermentationVesselRepository interface {
	Create(context.Context, *model.FermentationVessel) error
	GetByID(context.Context, uint) (model.FermentationVessel, error)
	GetByCode(context.Context, string) (model.FermentationVessel, error)
	List(context.Context, dto.FermentationVesselQuery) ([]model.FermentationVessel, int64, error)
	Update(context.Context, *model.FermentationVessel) error
	Deactivate(context.Context, uint) (bool, error)
	Summary(context.Context, uint) (model.FermentationVesselSummary, error)
}
type fermentationVesselRepository struct{ db *gorm.DB }
func NewFermentationVesselRepository(db *gorm.DB) FermentationVesselRepository {
	return &fermentationVesselRepository{db: db}
}
func (r *fermentationVesselRepository) Create(ctx context.Context, vessel *model.FermentationVessel) error {
	if err := r.db.WithContext(ctx).Create(vessel).Error; err != nil {
		return fmt.Errorf("create fermentation vessel: %w", err)
	}
	return nil
}
func (r *fermentationVesselRepository) GetByID(ctx context.Context, id uint) (model.FermentationVessel, error) {
	var vessel model.FermentationVessel
	if err := r.db.WithContext(ctx).First(&vessel, id).Error; err != nil {
		return model.FermentationVessel{}, fmt.Errorf("find fermentation vessel %d: %w", id, err)
	}
	return vessel, nil
}
func (r *fermentationVesselRepository) GetByCode(ctx context.Context, code string) (model.FermentationVessel, error) {
	var vessel model.FermentationVessel
	if err := r.db.WithContext(ctx).Where("vessel_code = ?", code).First(&vessel).Error; err != nil {
		return model.FermentationVessel{}, fmt.Errorf("find fermentation vessel by code: %w", err)
	}
	return vessel, nil
}
func (r *fermentationVesselRepository) List(ctx context.Context, query dto.FermentationVesselQuery) ([]model.FermentationVessel, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.FermentationVessel{})
	if search := strings.TrimSpace(query.Search); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		base = base.Where("LOWER(vessel_code) LIKE ? OR LOWER(name) LIKE ? OR LOWER(location) LIKE ?", pattern, pattern, pattern)
	}
	if query.Location != "" {
		base = base.Where("location = ?", query.Location)
	}
	if query.OwnerTeam != "" {
		base = base.Where("owner_team = ?", query.OwnerTeam)
	}
	if query.State != "" {
		base = base.Where("vessel_state = ?", query.State)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count fermentation vessels: %w", err)
	}
	var vessels []model.FermentationVessel
	offset := (query.Page - 1) * query.PageSize
	if err := base.Order("vessel_code ASC").Limit(query.PageSize).Offset(offset).Find(&vessels).Error; err != nil {
		return nil, 0, fmt.Errorf("list fermentation vessels: %w", err)
	}
	return vessels, total, nil
}
func (r *fermentationVesselRepository) Update(ctx context.Context, vessel *model.FermentationVessel) error {
	result := r.db.WithContext(ctx).Model(&model.FermentationVessel{}).Where("id = ?", vessel.ID).Updates(map[string]any{
		"name": vessel.Name, "working_volume_l": vessel.WorkingVolumeL,
		"sensor_channels": vessel.SensorChannels, "location": vessel.Location,
		"owner_team": vessel.OwnerTeam, "commissioned_at": vessel.CommissionedAt,
		"updated_at": vessel.UpdatedAt,
	})
	if result.Error != nil {
		return fmt.Errorf("update fermentation vessel %d: %w", vessel.ID, result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *fermentationVesselRepository) Deactivate(ctx context.Context, id uint) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.FermentationVessel{}).
		Where("id = ? AND vessel_state = ?", id, "active").
		Updates(map[string]any{"vessel_state": "inactive", "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if result.Error != nil {
		return false, fmt.Errorf("deactivate fermentation vessel %d: %w", id, result.Error)
	}
	return result.RowsAffected == 1, nil
}
func (r *fermentationVesselRepository) Summary(ctx context.Context, id uint) (model.FermentationVesselSummary, error) {
	var summary model.FermentationVesselSummary
	if err := r.db.WithContext(ctx).Model(&model.CultureRecipe{}).Where("vessel_id = ?", id).
		Count(&summary.RecipeCount).Error; err != nil {
		return summary, fmt.Errorf("count vessel recipes: %w", err)
	}
	if err := r.db.WithContext(ctx).Model(&model.SensorSeries{}).Where("vessel_id = ?", id).
		Count(&summary.SeriesCount).Error; err != nil {
		return summary, fmt.Errorf("count vessel sensor series: %w", err)
	}
	if err := r.db.WithContext(ctx).Model(&model.SensorSeries{}).Where("vessel_id = ? AND series_state = ?", id, "ready").
		Count(&summary.ReadySeriesCount).Error; err != nil {
		return summary, fmt.Errorf("count ready vessel sensor series: %w", err)
	}
	var latestSeries model.SensorSeries
	err := r.db.WithContext(ctx).Where("vessel_id = ?", id).Order("started_at DESC").First(&latestSeries).Error
	if err == nil {
		var quality struct {
			MissingRate map[string]float64 `json:"missing_rate"`
		}
		if json.Unmarshal([]byte(latestSeries.QualitySummary), &quality) == nil {
			for _, rate := range quality.MissingRate {
				if rate > summary.LatestMissingRate {
					summary.LatestMissingRate = rate
				}
			}
		}
	} else if err != gorm.ErrRecordNotFound {
		return summary, fmt.Errorf("load latest vessel series: %w", err)
	}
	var latestAnalysis model.DeviationAnalysis
	err = r.db.WithContext(ctx).Model(&model.DeviationAnalysis{}).
		Joins("JOIN sensor_series ON sensor_series.id = deviation_analyses.sensor_series_id").
		Where("sensor_series.vessel_id = ?", id).
		Order("deviation_analyses.analyzed_at DESC").First(&latestAnalysis).Error
	if err == nil {
		summary.LatestDeviationLevel = latestAnalysis.DeviationLevel
	} else if err != gorm.ErrRecordNotFound {
		return summary, fmt.Errorf("load latest vessel analysis: %w", err)
	}
	return summary, nil
}
