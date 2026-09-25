package service
import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"fermentation-kinetics-deviation-analysis/backend/internal/algorithm"
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"fermentation-kinetics-deviation-analysis/backend/internal/repository"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"gorm.io/gorm"
)
type CultureRecipeService struct {
	recipes repository.CultureRecipeRepository
	vessels repository.FermentationVesselRepository
	audits  repository.AuditRepository
	now     func() time.Time
}
func NewCultureRecipeService(
	recipes repository.CultureRecipeRepository,
	vessels repository.FermentationVesselRepository,
	audits repository.AuditRepository,
) *CultureRecipeService {
	return &CultureRecipeService{recipes: recipes, vessels: vessels, audits: audits, now: func() time.Time { return time.Now().UTC() }}
}
func (s *CultureRecipeService) Create(
	ctx context.Context, request dto.CreateCultureRecipeRequest, actor util.Actor,
) (dto.CultureRecipeResponse, error) {
	request.Normalize()
	vessel, err := s.vessels.GetByID(ctx, request.VesselID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.CultureRecipeResponse{}, util.NotFound("fermentation vessel")
		}
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load fermentation vessel", err)
	}
	if !vessel.Active() {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition, "cannot create a recipe for an inactive vessel")
	}
	if err := algorithm.ValidateRecipeConfiguration(
		request.PhaseBoundariesJSON, request.ReferenceCurvesJSON, request.ToleranceProfileJSON, request.TargetDurationH,
	); err != nil {
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "recipe kinetics configuration is invalid", err)
	}
	maxVersion, err := s.recipes.MaxVersion(ctx, request.VesselID, request.RecipeCode)
	if err != nil {
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to check recipe version", err)
	}
	if maxVersion != 0 {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "recipe code already exists; copy its version instead")
	}
	now := s.now()
	recipe := model.CultureRecipe{
		VesselID: request.VesselID, RecipeCode: request.RecipeCode, Version: 1, Organism: request.Organism,
		TargetDurationH: request.TargetDurationH, PhaseBoundariesJSON: string(request.PhaseBoundariesJSON),
		ReferenceCurvesJSON: string(request.ReferenceCurvesJSON), ToleranceProfileJSON: string(request.ToleranceProfileJSON),
		RecipeState: string(constants.RecipeDraft), CreatedBy: actor.UserID, CreatedByName: actor.Username,
		CreatedAt: now, UpdatedAt: now, Vessel: vessel,
	}
	if err := s.recipes.Create(ctx, &recipe); err != nil {
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusConflict, util.CodeConflict, "recipe code and version must be unique", err)
	}
	if err := recordAudit(ctx, s.audits, actor, "culture_recipe", recipe.ID, "create", nil, recipe, "", "", 0); err != nil {
		return dto.CultureRecipeResponse{}, err
	}
	return dto.NewCultureRecipeResponse(recipe), nil
}
func (s *CultureRecipeService) Get(ctx context.Context, id uint) (dto.CultureRecipeResponse, error) {
	recipe, err := s.recipes.GetByID(ctx, id, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.CultureRecipeResponse{}, util.NotFound("culture recipe")
		}
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load culture recipe", err)
	}
	return dto.NewCultureRecipeResponse(recipe), nil
}
func (s *CultureRecipeService) List(ctx context.Context, query dto.CultureRecipeQuery) (dto.CultureRecipeListResponse, error) {
	recipes, total, err := s.recipes.List(ctx, query)
	if err != nil {
		return dto.CultureRecipeListResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to list culture recipes", err)
	}
	response := dto.CultureRecipeListResponse{
		Items: make([]dto.CultureRecipeResponse, 0, len(recipes)), Total: total, Page: query.Page, Size: query.PageSize,
	}
	for _, recipe := range recipes {
		response.Items = append(response.Items, dto.NewCultureRecipeResponse(recipe))
	}
	return response, nil
}
func (s *CultureRecipeService) Update(
	ctx context.Context, id uint, request dto.UpdateCultureRecipeRequest, actor util.Actor,
) (dto.CultureRecipeResponse, error) {
	request.Normalize()
	recipe, err := s.recipes.GetByID(ctx, id, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.CultureRecipeResponse{}, util.NotFound("culture recipe")
		}
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load culture recipe", err)
	}
	if !recipe.Editable() {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition, "published or obsolete recipe versions cannot be edited")
	}
	if recipe.Version != request.Version {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "recipe version changed concurrently")
	}
	before := recipe
	if request.Organism != nil {
		recipe.Organism = *request.Organism
	}
	if request.TargetDurationH != nil {
		recipe.TargetDurationH = *request.TargetDurationH
	}
	if request.PhaseBoundariesJSON != nil {
		recipe.PhaseBoundariesJSON = string(*request.PhaseBoundariesJSON)
	}
	if request.ReferenceCurvesJSON != nil {
		recipe.ReferenceCurvesJSON = string(*request.ReferenceCurvesJSON)
	}
	if request.ToleranceProfileJSON != nil {
		recipe.ToleranceProfileJSON = string(*request.ToleranceProfileJSON)
	}
	if err := algorithm.ValidateRecipeConfiguration(
		[]byte(recipe.PhaseBoundariesJSON), []byte(recipe.ReferenceCurvesJSON),
		[]byte(recipe.ToleranceProfileJSON), recipe.TargetDurationH,
	); err != nil {
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "recipe kinetics configuration is invalid", err)
	}
	recipe.UpdatedAt = s.now()
	changed, err := s.recipes.UpdateWithVersion(ctx, &recipe, request.Version)
	if err != nil {
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to update culture recipe", err)
	}
	if !changed {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "recipe state or version changed concurrently")
	}
	if err := recordAudit(ctx, s.audits, actor, "culture_recipe", recipe.ID, "update", before, recipe, "", "", 0); err != nil {
		return dto.CultureRecipeResponse{}, err
	}
	return s.Get(ctx, recipe.ID)
}
func (s *CultureRecipeService) Transition(
	ctx context.Context, id uint, request dto.CultureRecipeTransitionRequest, actor util.Actor,
) (dto.CultureRecipeResponse, error) {
	recipe, err := s.recipes.GetByID(ctx, id, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.CultureRecipeResponse{}, util.NotFound("culture recipe")
		}
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load culture recipe", err)
	}
	from, to := constants.RecipeState(recipe.RecipeState), constants.RecipeState(request.ToState)
	if !constants.CanTransitionRecipe(from, to) {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition,
			"illegal recipe transition from "+recipe.RecipeState+" to "+request.ToState)
	}
	if request.Version != recipe.Version {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "recipe version changed concurrently")
	}
	if to == constants.RecipeValidated || to == constants.RecipePublished {
		if err := algorithm.ValidateRecipeConfiguration(
			[]byte(recipe.PhaseBoundariesJSON), []byte(recipe.ReferenceCurvesJSON),
			[]byte(recipe.ToleranceProfileJSON), recipe.TargetDurationH,
		); err != nil {
			return dto.CultureRecipeResponse{}, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "recipe kinetics configuration is invalid", err)
		}
	}
	before := recipe
	changed, err := s.recipes.Transition(ctx, id, recipe.RecipeState, request.ToState, request.Version, s.now())
	if err != nil {
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to transition culture recipe", err)
	}
	if !changed {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "recipe state or version changed concurrently")
	}
	recipe.RecipeState = request.ToState
	recipe.UpdatedAt = s.now()
	if err := recordAudit(ctx, s.audits, actor, "culture_recipe", id, "transition", before,
		map[string]any{"recipe": recipe, "comment": strings.TrimSpace(request.Comment)}, "", "", 0); err != nil {
		return dto.CultureRecipeResponse{}, err
	}
	return s.Get(ctx, id)
}
func (s *CultureRecipeService) Copy(
	ctx context.Context, id uint, request dto.CopyCultureRecipeRequest, actor util.Actor,
) (dto.CultureRecipeResponse, error) {
	source, err := s.recipes.GetByID(ctx, id, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.CultureRecipeResponse{}, util.NotFound("culture recipe")
		}
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load source recipe", err)
	}
	targetVesselID := request.VesselID
	if targetVesselID == 0 {
		targetVesselID = source.VesselID
	}
	vessel, err := s.vessels.GetByID(ctx, targetVesselID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.CultureRecipeResponse{}, util.NotFound("target fermentation vessel")
		}
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load target vessel", err)
	}
	if !vessel.Active() {
		return dto.CultureRecipeResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition, "cannot copy a recipe to an inactive vessel")
	}
	version, err := s.recipes.MaxVersion(ctx, targetVesselID, source.RecipeCode)
	if err != nil {
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to allocate recipe version", err)
	}
	now := s.now()
	copyRecipe := model.CultureRecipe{
		VesselID: targetVesselID, Vessel: vessel, RecipeCode: source.RecipeCode, Version: version + 1,
		Organism: source.Organism, TargetDurationH: source.TargetDurationH,
		PhaseBoundariesJSON: source.PhaseBoundariesJSON, ReferenceCurvesJSON: source.ReferenceCurvesJSON,
		ToleranceProfileJSON: source.ToleranceProfileJSON, RecipeState: string(constants.RecipeDraft),
		CreatedBy: actor.UserID, CreatedByName: actor.Username, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.recipes.Create(ctx, &copyRecipe); err != nil {
		return dto.CultureRecipeResponse{}, util.WrapError(http.StatusConflict, util.CodeConflict, "recipe version was allocated concurrently", err)
	}
	if err := recordAudit(ctx, s.audits, actor, "culture_recipe", copyRecipe.ID, "copy_version", source,
		map[string]any{"recipe": copyRecipe, "comment": strings.TrimSpace(request.Comment)}, "", "", 0); err != nil {
		return dto.CultureRecipeResponse{}, err
	}
	return dto.NewCultureRecipeResponse(copyRecipe), nil
}
