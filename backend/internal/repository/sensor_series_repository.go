package repository
import (
	"context"
	"fmt"
	"strings"
	"time"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"gorm.io/gorm"
)
type SensorSeriesRepository interface {
	Create(context.Context, *model.SensorSeries) error
	GetByID(context.Context, uint, bool) (model.SensorSeries, error)
	List(context.Context, dto.SensorSeriesQuery) ([]model.SensorSeries, int64, error)
	Transition(context.Context, uint, string, string, string, string, time.Time) (bool, error)
	FindByRunCode(context.Context, string) (model.SensorSeries, error)
}
type sensorSeriesRepository struct{ db *gorm.DB }
func NewSensorSeriesRepository(db *gorm.DB) SensorSeriesRepository {
	return &sensorSeriesRepository{db: db}
}
func (r *sensorSeriesRepository) Create(ctx context.Context, series *model.SensorSeries) error {
	if err := r.db.WithContext(ctx).Create(series).Error; err != nil {
		return fmt.Errorf("create sensor series: %w", err)
	}
	return nil
}
func (r *sensorSeriesRepository) GetByID(ctx context.Context, id uint, preload bool) (model.SensorSeries, error) {
	var series model.SensorSeries
	query := r.db.WithContext(ctx)
	if preload {
		query = query.Preload("Vessel").Preload("Recipe")
	}
	if err := query.First(&series, id).Error; err != nil {
		return model.SensorSeries{}, fmt.Errorf("find sensor series %d: %w", id, err)
	}
	return series, nil
}
func (r *sensorSeriesRepository) FindByRunCode(ctx context.Context, runCode string) (model.SensorSeries, error) {
	var series model.SensorSeries
	if err := r.db.WithContext(ctx).Where("run_code = ?", runCode).First(&series).Error; err != nil {
		return model.SensorSeries{}, fmt.Errorf("find sensor series by run code: %w", err)
	}
	return series, nil
}
func (r *sensorSeriesRepository) List(ctx context.Context, query dto.SensorSeriesQuery) ([]model.SensorSeries, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.SensorSeries{})
	if query.VesselID != 0 {
		base = base.Where("vessel_id = ?", query.VesselID)
	}
	if query.RecipeID != 0 {
		base = base.Where("recipe_id = ?", query.RecipeID)
	}
	if query.Channel != "" {
		base = base.Where("channel = ?", query.Channel)
	}
	if query.State != "" {
		base = base.Where("series_state = ?", query.State)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		base = base.Where("LOWER(run_code) LIKE ?", "%"+strings.ToLower(search)+"%")
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count sensor series: %w", err)
	}
	var items []model.SensorSeries
	offset := (query.Page - 1) * query.PageSize
	if err := base.Preload("Vessel").Preload("Recipe").Order("started_at DESC, id DESC").
		Limit(query.PageSize).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list sensor series: %w", err)
	}
	return items, total, nil
}
func (r *sensorSeriesRepository) Transition(
	ctx context.Context, id uint, from, to, pointsJSON, metadataJSON string, updatedAt time.Time,
) (bool, error) {
	updates := map[string]any{"series_state": to, "updated_at": updatedAt}
	if pointsJSON != "" {
		updates["points_json"] = pointsJSON
	}
	if to == "validated" || to == "rejected" {
		updates["quality_summary"] = metadataJSON
	}
	if to == "normalized" {
		updates["normalization_json"] = metadataJSON
	}
	result := r.db.WithContext(ctx).Model(&model.SensorSeries{}).
		Where("id = ? AND series_state = ?", id, from).Updates(updates)
	if result.Error != nil {
		return false, fmt.Errorf("transition sensor series %d: %w", id, result.Error)
	}
	return result.RowsAffected == 1, nil
}
