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
type DeviationAnalysisService struct {
	analyses  repository.DeviationAnalysisRepository
	recipes   repository.CultureRecipeRepository
	series    repository.SensorSeriesRepository
	audits    repository.AuditRepository
	evaluator *algorithm.Evaluator
	now       func() time.Time
}
func NewDeviationAnalysisService(
	analyses repository.DeviationAnalysisRepository,
	recipes repository.CultureRecipeRepository,
	series repository.SensorSeriesRepository,
	audits repository.AuditRepository,
	evaluator *algorithm.Evaluator,
) *DeviationAnalysisService {
	return &DeviationAnalysisService{
		analyses: analyses, recipes: recipes, series: series, audits: audits, evaluator: evaluator,
		now: func() time.Time { return time.Now().UTC() },
	}
}
func (s *DeviationAnalysisService) Run(
	ctx context.Context, request dto.RunDeviationAnalysisRequest, idempotencyKey string, actor util.Actor,
) (dto.DeviationAnalysisResponse, bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return dto.DeviationAnalysisResponse{}, false, util.NewError(http.StatusBadRequest, util.CodeIdempotency, "Idempotency-Key header is required")
	}
	if len(idempotencyKey) > 128 {
		return dto.DeviationAnalysisResponse{}, false, util.NewError(http.StatusBadRequest, util.CodeValidation, "Idempotency-Key must not exceed 128 characters")
	}
	series, err := s.series.GetByID(ctx, request.SensorSeriesID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.DeviationAnalysisResponse{}, false, util.NotFound("sensor series")
		}
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load sensor series", err)
	}
	if !series.Ready() {
		return dto.DeviationAnalysisResponse{}, false, util.NewError(http.StatusConflict, util.CodeStateTransition, "only ready sensor series may be analyzed")
	}
	recipe, err := s.recipes.GetByID(ctx, series.RecipeID, false)
	if err != nil {
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load frozen recipe version", err)
	}
	snapshot := algorithm.NewSnapshot(series, recipe)
	inputHash, err := snapshot.Hash()
	if err != nil {
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to hash analysis input", err)
	}
	if prior, findErr := s.analyses.FindByIdempotencyKey(ctx, idempotencyKey); findErr == nil {
		if prior.InputHash != inputHash {
			return dto.DeviationAnalysisResponse{}, false, util.NewError(http.StatusConflict, util.CodeConflict, "Idempotency-Key is already bound to a different input")
		}
		return dto.NewDeviationAnalysisResponse(prior), true, nil
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to check idempotency key", findErr)
	}
	if prior, findErr := s.analyses.FindByInput(ctx, inputHash, algorithm.Version); findErr == nil {
		return dto.NewDeviationAnalysisResponse(prior), true, nil
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to check frozen input", findErr)
	}
	snapshotJSON, err := snapshot.Canonical()
	if err != nil {
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to freeze analysis input", err)
	}
	now := s.now()
	analysis := model.DeviationAnalysis{
		SensorSeriesID: series.ID, RecipeID: recipe.ID, RecipeVersion: recipe.Version,
		AlgorithmVersion: algorithm.Version, InputHash: inputHash, InputSnapshot: snapshotJSON,
		PhaseScoresJSON: "[]", DeviationLevel: string(constants.DeviationNormal),
		AlignedCurveJSON: "[]", SuspectedCausesJSON: "[]", AnalysisState: string(constants.AnalysisQueued),
		Explanation: "Analysis is queued.", AnalyzedAt: now, InitiatedBy: actor.UserID,
		InitiatedByName: actor.Username, IdempotencyKey: idempotencyKey, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.analyses.Create(ctx, &analysis); err != nil {
		if prior, findErr := s.analyses.FindByInput(ctx, inputHash, algorithm.Version); findErr == nil {
			return dto.NewDeviationAnalysisResponse(prior), true, nil
		}
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusConflict, util.CodeConflict, "analysis was queued concurrently", err)
	}
	changed, err := s.analyses.Transition(ctx, analysis.ID, string(constants.AnalysisQueued), string(constants.AnalysisAnalyzing), nil)
	if err != nil || !changed {
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusConflict, util.CodeConflict, "analysis could not enter analyzing state", err)
	}
	started := time.Now()
	result, evaluateErr := s.evaluator.Evaluate(snapshot)
	duration := time.Since(started).Milliseconds()
	if evaluateErr != nil {
		_, transitionErr := s.analyses.Transition(ctx, analysis.ID, string(constants.AnalysisAnalyzing), string(constants.AnalysisFailed),
			map[string]any{"failure_reason": util.CompactText(evaluateErr.Error(), 1000), "duration_milliseconds": duration})
		if transitionErr != nil {
			return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "analysis failed and failure state could not be stored", transitionErr)
		}
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "deviation analysis failed", evaluateErr)
	}
	changed, err = s.analyses.Complete(ctx, analysis.ID, map[string]any{
		"phase_scores_json": result.PhaseScoresJSON, "deviation_level": string(result.DeviationLevel),
		"aligned_curve_json": result.AlignedCurveJSON, "suspected_causes_json": result.SuspectedCausesJSON,
		"explanation": result.Explanation, "analyzed_at": s.now(), "duration_milliseconds": duration,
	})
	if err != nil {
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to store analysis result", err)
	}
	if !changed {
		return dto.DeviationAnalysisResponse{}, false, util.NewError(http.StatusConflict, util.CodeConflict, "analysis state changed concurrently")
	}
	analysis, err = s.analyses.GetByID(ctx, analysis.ID, true)
	if err != nil {
		return dto.DeviationAnalysisResponse{}, false, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to reload deviation analysis", err)
	}
	if err := recordAudit(ctx, s.audits, actor, "deviation_analysis", analysis.ID, "run", nil, analysis,
		inputHash, algorithm.Version, duration); err != nil {
		return dto.DeviationAnalysisResponse{}, false, err
	}
	return dto.NewDeviationAnalysisResponse(analysis), false, nil
}
func (s *DeviationAnalysisService) Get(ctx context.Context, id uint) (dto.DeviationAnalysisResponse, error) {
	analysis, err := s.analyses.GetByID(ctx, id, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.DeviationAnalysisResponse{}, util.NotFound("deviation analysis")
		}
		return dto.DeviationAnalysisResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load deviation analysis", err)
	}
	return dto.NewDeviationAnalysisResponse(analysis), nil
}
func (s *DeviationAnalysisService) List(
	ctx context.Context, query dto.DeviationAnalysisQuery,
) (dto.DeviationAnalysisListResponse, error) {
	analyses, total, err := s.analyses.List(ctx, query)
	if err != nil {
		return dto.DeviationAnalysisListResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to list deviation analyses", err)
	}
	response := dto.DeviationAnalysisListResponse{
		Items: make([]dto.DeviationAnalysisResponse, 0, len(analyses)), Total: total, Page: query.Page, Size: query.PageSize,
	}
	for _, analysis := range analyses {
		response.Items = append(response.Items, dto.NewDeviationAnalysisResponse(analysis))
	}
	return response, nil
}
func (s *DeviationAnalysisService) Transition(
	ctx context.Context, id uint, request dto.DeviationAnalysisTransitionRequest, actor util.Actor,
) (dto.DeviationAnalysisResponse, error) {
	analysis, err := s.analyses.GetByID(ctx, id, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.DeviationAnalysisResponse{}, util.NotFound("deviation analysis")
		}
		return dto.DeviationAnalysisResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load deviation analysis", err)
	}
	from, to := constants.AnalysisState(analysis.AnalysisState), constants.AnalysisState(request.ToState)
	if !constants.CanTransitionAnalysis(from, to) {
		return dto.DeviationAnalysisResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition,
			"illegal analysis transition from "+analysis.AnalysisState+" to "+request.ToState)
	}
	if to == constants.AnalysisConfirmed && !analysis.ReviewerSeparated(actor.UserID) {
		return dto.DeviationAnalysisResponse{}, util.NewError(http.StatusConflict, util.CodeReviewerConflict,
			"analysis initiator cannot confirm their own result")
	}
	before := analysis
	updates := map[string]any{"review_comment": strings.TrimSpace(request.Comment)}
	if to == constants.AnalysisReviewed {
		updates["reviewed_by"] = actor.UserID
		updates["reviewed_by_name"] = actor.Username
	}
	changed, err := s.analyses.Transition(ctx, id, analysis.AnalysisState, request.ToState, updates)
	if err != nil {
		return dto.DeviationAnalysisResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to transition deviation analysis", err)
	}
	if !changed {
		return dto.DeviationAnalysisResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "analysis state changed concurrently")
	}
	analysis.AnalysisState = request.ToState
	analysis.ReviewComment = strings.TrimSpace(request.Comment)
	if to == constants.AnalysisReviewed {
		analysis.ReviewedBy = &actor.UserID
		analysis.ReviewedByName = actor.Username
	}
	if err := recordAudit(ctx, s.audits, actor, "deviation_analysis", id, "transition", before, analysis,
		analysis.InputHash, analysis.AlgorithmVersion, 0); err != nil {
		return dto.DeviationAnalysisResponse{}, err
	}
	return s.Get(ctx, id)
}
func (s *DeviationAnalysisService) Replay(
	ctx context.Context, id uint, actor util.Actor,
) (dto.DeviationAnalysisResponse, error) {
	analysis, err := s.analyses.GetByID(ctx, id, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.DeviationAnalysisResponse{}, util.NotFound("deviation analysis")
		}
		return dto.DeviationAnalysisResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to load deviation analysis", err)
	}
	if analysis.AnalysisState == string(constants.AnalysisQueued) || analysis.AnalysisState == string(constants.AnalysisAnalyzing) ||
		analysis.AnalysisState == string(constants.AnalysisFailed) {
		return dto.DeviationAnalysisResponse{}, util.NewError(http.StatusConflict, util.CodeStateTransition, "only completed analysis results can be replayed")
	}
	snapshot, err := algorithm.DecodeSnapshot(analysis.InputSnapshot)
	if err != nil {
		return dto.DeviationAnalysisResponse{}, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "frozen analysis snapshot is invalid", err)
	}
	result, err := s.evaluator.Evaluate(snapshot)
	if err != nil {
		return dto.DeviationAnalysisResponse{}, util.WrapError(http.StatusUnprocessableEntity, util.CodeValidation, "analysis replay failed", err)
	}
	passed := result.PhaseScoresJSON == analysis.PhaseScoresJSON &&
		string(result.DeviationLevel) == analysis.DeviationLevel &&
		result.AlignedCurveJSON == analysis.AlignedCurveJSON &&
		result.SuspectedCausesJSON == analysis.SuspectedCausesJSON &&
		result.Explanation == analysis.Explanation
	if err := s.analyses.SetReplayVerified(ctx, id, passed); err != nil {
		return dto.DeviationAnalysisResponse{}, util.WrapError(http.StatusInternalServerError, util.CodeInternal, "unable to store replay evidence", err)
	}
	if err := recordAudit(ctx, s.audits, actor, "deviation_analysis", id, "replay", analysis,
		map[string]any{"replay_verified": passed}, analysis.InputHash, analysis.AlgorithmVersion, 0); err != nil {
		return dto.DeviationAnalysisResponse{}, err
	}
	if !passed {
		return dto.DeviationAnalysisResponse{}, util.NewError(http.StatusConflict, util.CodeConflict, "replay result differs from the frozen historical result")
	}
	return s.Get(ctx, id)
}
