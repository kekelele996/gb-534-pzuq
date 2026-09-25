package service
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"fermentation-kinetics-deviation-analysis/backend/internal/repository"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"gorm.io/gorm"
)
type FermentationVesselService struct {
	vessels repository.FermentationVesselRepository
	audits  repository.AuditRepository
	now     func() time.Time
}
func NewFermentationVesselService(
	vessels repository.FermentationVesselRepository, audits repository.AuditRepository,
) *FermentationVesselService {
	return &FermentationVesselService{vessels: vessels, audits: audits, now: func() time.Time { return time.Now().UTC() }}
}
func (s *FermentationVesselService) Create(
	ctx context.Context, request dto.CreateFermentationVesselRequest, actor util.Actor,
) (dto.FermentationVesselResponse, error) {
	request.Normalize()
	channels, err := json.Marshal(request.SensorChannels)
	if err != nil {
		return dto.FermentationVesselResponse{}, util.WrapError(http.StatusBadRequest, util.CodeValidation, "sensor channels are invalid", err)
	}
	now := s.now()
	vessel := model.FermentationVessel{
		VesselCode: request.VesselCode, Name: request.Name, WorkingVolumeL: request.WorkingVolumeL,
		SensorChannels: string(channels), Location: request.Location, OwnerTeam: request.OwnerTeam,
		VesselState: "active", CommissionedAt: request.CommissionedAt.UTC(), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.vessels.Create(ctx, &vessel); err != nil {
		return dto.FermentationVesselResponse{}, util.WrapError(http.StatusConflict, util.CodeConflict, "vessel code already exists or is invalid", err)
	}
	if err := recordAudit(ctx, s.audits, actor, "fermentation_vessel", vessel.ID, "create", nil, vessel, "", "", 0); err != nil {
		return dto.FermentationVesselResponse{}, err
	}
	return dto.NewFermentationVesselResponse(vessel, model.FermentationVesselSummary{}), nil
}
func (s *FermentationVesselService) Get(ctx context.Context, id uint) (dto.FermentationVesselResponse, error) {
	vessel, err := s.vessels.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.FermentationVesselResponse{}, util.NotFound("fermentation vessel")
		}
		return dto.FermentationVesselResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load fermentation vessel", err)
	}
	summary, err := s.vessels.Summary(ctx, id)
	if err != nil {
		return dto.FermentationVesselResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load vessel analysis summary", err)
	}
	return dto.NewFermentationVesselResponse(vessel, summary), nil
}
func (s *FermentationVesselService) List(
	ctx context.Context, query dto.FermentationVesselQuery,
) (dto.FermentationVesselListResponse, error) {
	vessels, total, err := s.vessels.List(ctx, query)
	if err != nil {
		return dto.FermentationVesselListResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to list fermentation vessels", err)
	}
	response := dto.FermentationVesselListResponse{
		Items: make([]dto.FermentationVesselResponse, 0, len(vessels)), Total: total, Page: query.Page, Size: query.PageSize,
	}
	for _, vessel := range vessels {
		summary, summaryErr := s.vessels.Summary(ctx, vessel.ID)
		if summaryErr != nil {
			return dto.FermentationVesselListResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load vessel summary", summaryErr)
		}
		response.Items = append(response.Items, dto.NewFermentationVesselResponse(vessel, summary))
	}
	return response, nil
}
func (s *FermentationVesselService) Update(
	ctx context.Context, id uint, request dto.UpdateFermentationVesselRequest, actor util.Actor,
) (dto.FermentationVesselResponse, error) {
	request.Normalize()
	vessel, err := s.vessels.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.FermentationVesselResponse{}, util.NotFound("fermentation vessel")
		}
		return dto.FermentationVesselResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load fermentation vessel", err)
	}
	if !vessel.Active() {
		return dto.FermentationVesselResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition, "inactive vessels cannot be edited")
	}
	before := vessel
	if request.Name != nil {
		vessel.Name = *request.Name
	}
	if request.WorkingVolumeL != nil {
		vessel.WorkingVolumeL = *request.WorkingVolumeL
	}
	if request.SensorChannels != nil {
		encoded, marshalErr := json.Marshal(*request.SensorChannels)
		if marshalErr != nil {
			return dto.FermentationVesselResponse{}, util.WrapError(http.StatusBadRequest, util.CodeValidation, "sensor channels are invalid", marshalErr)
		}
		vessel.SensorChannels = string(encoded)
	}
	if request.Location != nil {
		vessel.Location = *request.Location
	}
	if request.OwnerTeam != nil {
		vessel.OwnerTeam = *request.OwnerTeam
	}
	if request.CommissionedAt != nil {
		vessel.CommissionedAt = request.CommissionedAt.UTC()
	}
	vessel.UpdatedAt = s.now()
	if err := s.vessels.Update(ctx, &vessel); err != nil {
		return dto.FermentationVesselResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to update fermentation vessel", err)
	}
	if err := recordAudit(ctx, s.audits, actor, "fermentation_vessel", vessel.ID, "update", before, vessel, "", "", 0); err != nil {
		return dto.FermentationVesselResponse{}, err
	}
	return s.Get(ctx, vessel.ID)
}
func (s *FermentationVesselService) Deactivate(
	ctx context.Context, id uint, actor util.Actor,
) (dto.FermentationVesselResponse, error) {
	vessel, err := s.vessels.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.FermentationVesselResponse{}, util.NotFound("fermentation vessel")
		}
		return dto.FermentationVesselResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load fermentation vessel", err)
	}
	if !vessel.Active() {
		return dto.FermentationVesselResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition, "fermentation vessel is already inactive")
	}
	before := vessel
	changed, err := s.vessels.Deactivate(ctx, id)
	if err != nil {
		return dto.FermentationVesselResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to deactivate fermentation vessel", err)
	}
	if !changed {
		return dto.FermentationVesselResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "fermentation vessel changed concurrently")
	}
	vessel.VesselState = "inactive"
	vessel.UpdatedAt = s.now()
	if err := recordAudit(ctx, s.audits, actor, "fermentation_vessel", id, "deactivate", before, vessel, "", "", 0); err != nil {
		return dto.FermentationVesselResponse{}, err
	}
	return s.Get(ctx, id)
}
func recordAudit(
	ctx context.Context, audits repository.AuditRepository, actor util.Actor,
	entity string, entityID uint, action string, before, after any,
	inputHash, algorithm string, durationMS int64,
) error {
	beforeJSON, err := util.CanonicalJSON(before)
	if err != nil {
		return util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to serialize audit before snapshot", err)
	}
	afterJSON, err := util.CanonicalJSON(after)
	if err != nil {
		return util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to serialize audit after snapshot", err)
	}
	err = audits.Record(ctx, model.AuditLog{
		RequestID: actor.RequestID, ActorID: actor.UserID, ActorName: actor.Username, ActorRole: actor.Role,
		EntityType: entity, EntityID: entityID, Action: action,
		BeforeSnapshot: beforeJSON, AfterSnapshot: afterJSON, InputHash: inputHash,
		Algorithm: algorithm, DurationMS: durationMS, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to record audit trail", err)
	}
	return nil
}
