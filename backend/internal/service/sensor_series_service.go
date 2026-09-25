package service
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"fermentation-kinetics-deviation-analysis/backend/internal/repository"
	"fermentation-kinetics-deviation-analysis/backend/internal/timeseries"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"gorm.io/gorm"
)
type SensorSeriesService struct {
	series  repository.SensorSeriesRepository
	recipes repository.CultureRecipeRepository
	vessels repository.FermentationVesselRepository
	audits  repository.AuditRepository
	now     func() time.Time
}
func NewSensorSeriesService(
	series repository.SensorSeriesRepository,
	recipes repository.CultureRecipeRepository,
	vessels repository.FermentationVesselRepository,
	audits repository.AuditRepository,
) *SensorSeriesService {
	return &SensorSeriesService{
		series: series, recipes: recipes, vessels: vessels, audits: audits,
		now: func() time.Time { return time.Now().UTC() },
	}
}
func (s *SensorSeriesService) Import(
	ctx context.Context, request dto.ImportSensorSeriesRequest, actor util.Actor,
) (dto.SensorSeriesResponse, error) {
	request.Normalize()
	vessel, err := s.vessels.GetByID(ctx, request.VesselID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.SensorSeriesResponse{}, util.NotFound("fermentation vessel")
		}
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load fermentation vessel", err)
	}
	if !vessel.Active() {
		return dto.SensorSeriesResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition, "cannot import data for an inactive vessel")
	}
	recipe, err := s.recipes.GetByID(ctx, request.RecipeID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.SensorSeriesResponse{}, util.NotFound("culture recipe")
		}
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load culture recipe", err)
	}
	if recipe.VesselID != vessel.ID {
		return dto.SensorSeriesResponse{}, util.NewError(http.StatusUnprocessableEntity, util.CodeValidation, "recipe does not belong to the selected vessel")
	}
	if recipe.RecipeState != string(constants.RecipePublished) {
		return dto.SensorSeriesResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition, "sensor series can only target a published recipe")
	}
	points, quality, err := timeseries.Validate(request.PointsJSON, request.Channel, request.SampleIntervalS)
	if err != nil {
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "time series format is invalid", err)
	}
	canonical, err := timeseries.EncodePoints(points)
	if err != nil {
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to canonicalize time series", err)
	}
	qualityJSON, err := json.Marshal(quality)
	if err != nil {
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to serialize quality summary", err)
	}
	now := s.now()
	series := model.SensorSeries{
		VesselID: vessel.ID, Vessel: vessel, RecipeID: recipe.ID, Recipe: recipe,
		RunCode: request.RunCode, Channel: request.Channel, SampleIntervalS: request.SampleIntervalS,
		PointsJSON: canonical, StartedAt: points[0].Timestamp, EndedAt: points[len(points)-1].Timestamp,
		SourceChecksum: util.HashString(canonical), SeriesState: string(constants.SeriesImported),
		QualitySummary: string(qualityJSON), NormalizationJSON: "{}",
		ImportedBy: actor.UserID, ImportedByName: actor.Username, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.series.Create(ctx, &series); err != nil {
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusConflict, util.CodeConflict, "run code already exists or series is invalid", err)
	}
	if err := recordAudit(ctx, s.audits, actor, "sensor_series", series.ID, "import", nil, series,
		series.SourceChecksum, "", 0); err != nil {
		return dto.SensorSeriesResponse{}, err
	}
	return dto.NewSensorSeriesResponse(series), nil
}
func (s *SensorSeriesService) Get(ctx context.Context, id uint) (dto.SensorSeriesResponse, error) {
	series, err := s.series.GetByID(ctx, id, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.SensorSeriesResponse{}, util.NotFound("sensor series")
		}
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load sensor series", err)
	}
	return dto.NewSensorSeriesResponse(series), nil
}
func (s *SensorSeriesService) List(ctx context.Context, query dto.SensorSeriesQuery) (dto.SensorSeriesListResponse, error) {
	items, total, err := s.series.List(ctx, query)
	if err != nil {
		return dto.SensorSeriesListResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to list sensor series", err)
	}
	response := dto.SensorSeriesListResponse{
		Items: make([]dto.SensorSeriesResponse, 0, len(items)), Total: total, Page: query.Page, Size: query.PageSize,
	}
	for _, item := range items {
		response.Items = append(response.Items, dto.NewSensorSeriesResponse(item))
	}
	return response, nil
}
func (s *SensorSeriesService) Transition(
	ctx context.Context, id uint, request dto.SensorSeriesTransitionRequest, actor util.Actor,
) (dto.SensorSeriesResponse, error) {
	series, err := s.series.GetByID(ctx, id, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.SensorSeriesResponse{}, util.NotFound("sensor series")
		}
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load sensor series", err)
	}
	from, to := constants.SeriesState(series.SeriesState), constants.SeriesState(request.ToState)
	if !constants.CanTransitionSeries(from, to) {
		return dto.SensorSeriesResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition,
			"illegal sensor series transition from "+series.SeriesState+" to "+request.ToState)
	}
	before := series
	pointsJSON, metadataJSON := "", ""
	if to == constants.SeriesValidated {
		points, quality, validateErr := timeseries.Validate([]byte(series.PointsJSON), series.Channel, series.SampleIntervalS)
		if validateErr != nil {
			return s.rejectAfterValidation(ctx, series, before, actor, validateErr.Error())
		}
		pointsJSON, err = timeseries.EncodePoints(points)
		if err != nil {
			return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to canonicalize validated series", err)
		}
		qualityBytes, marshalErr := json.Marshal(quality)
		if marshalErr != nil {
			return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to encode series quality", marshalErr)
		}
		metadataJSON = string(qualityBytes)
		if !quality.Valid {
			return s.rejectAfterValidation(ctx, series, before, actor, strings.Join(quality.Warnings, "; "))
		}
	}
	if to == constants.SeriesNormalized {
		points, decodeErr := timeseries.DecodePoints(series.PointsJSON)
		if decodeErr != nil {
			return dto.SensorSeriesResponse{}, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "validated series cannot be decoded", decodeErr)
		}
		_, normalization, normalizeErr := timeseries.Normalize(points)
		if normalizeErr != nil {
			return dto.SensorSeriesResponse{}, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "series cannot be robustly normalized", normalizeErr)
		}
		metadataJSON, err = timeseries.EncodeNormalization(normalization)
		if err != nil {
			return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to encode normalization evidence", err)
		}
	}
	if to == constants.SeriesRejected {
		metadataJSON, _ = util.CanonicalJSON(map[string]any{"valid": false, "warnings": []string{strings.TrimSpace(request.Comment)}})
	}
	changed, err := s.series.Transition(ctx, id, series.SeriesState, request.ToState, pointsJSON, metadataJSON, s.now())
	if err != nil {
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to transition sensor series", err)
	}
	if !changed {
		return dto.SensorSeriesResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "sensor series state changed concurrently")
	}
	series.SeriesState = request.ToState
	if pointsJSON != "" {
		series.PointsJSON = pointsJSON
	}
	if to == constants.SeriesValidated || to == constants.SeriesRejected {
		series.QualitySummary = metadataJSON
	}
	if to == constants.SeriesNormalized {
		series.NormalizationJSON = metadataJSON
	}
	series.UpdatedAt = s.now()
	if err := recordAudit(ctx, s.audits, actor, "sensor_series", id, "transition", before,
		map[string]any{"series": series, "comment": strings.TrimSpace(request.Comment)}, series.SourceChecksum, "", 0); err != nil {
		return dto.SensorSeriesResponse{}, err
	}
	return s.Get(ctx, id)
}
func (s *SensorSeriesService) rejectAfterValidation(
	ctx context.Context, series, before model.SensorSeries, actor util.Actor, reason string,
) (dto.SensorSeriesResponse, error) {
	metadata, _ := util.CanonicalJSON(map[string]any{"valid": false, "warnings": []string{reason}})
	changed, err := s.series.Transition(ctx, series.ID, series.SeriesState, string(constants.SeriesRejected), "", metadata, s.now())
	if err != nil {
		return dto.SensorSeriesResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to reject low-quality series", err)
	}
	if !changed {
		return dto.SensorSeriesResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "sensor series state changed concurrently")
	}
	series.SeriesState = string(constants.SeriesRejected)
	series.QualitySummary = metadata
	series.UpdatedAt = s.now()
	if err := recordAudit(ctx, s.audits, actor, "sensor_series", series.ID, "quality_rejected", before, series,
		series.SourceChecksum, "", 0); err != nil {
		return dto.SensorSeriesResponse{}, err
	}
	return dto.SensorSeriesResponse{}, util.NewError(http.StatusUnprocessableEntity, util.CodeValidation,
		"series quality is insufficient and the series was rejected: "+util.CompactText(reason, 240))
}
