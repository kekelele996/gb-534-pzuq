package handler
import (
	"net/http"
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/service"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"github.com/gin-gonic/gin"
)
type DeviationAnalysisHandler struct {
	service *service.DeviationAnalysisService
}
func NewDeviationAnalysisHandler(value *service.DeviationAnalysisService) *DeviationAnalysisHandler {
	return &DeviationAnalysisHandler{service: value}
}
func (h *DeviationAnalysisHandler) List(c *gin.Context) {
	seriesID, ok := optionalUint(c, "sensor_series_id")
	if !ok {
		return
	}
	recipeID, ok := optionalUint(c, "recipe_id")
	if !ok {
		return
	}
	page, size := util.Pagination(c)
	result, err := h.service.List(c.Request.Context(), dto.DeviationAnalysisQuery{
		SensorSeriesID: seriesID, RecipeID: recipeID, State: c.Query("state"),
		Level: c.Query("deviation_level"), Initiator: c.Query("initiator"), Page: page, PageSize: size,
	})
	respond(c, http.StatusOK, result, err)
}
func (h *DeviationAnalysisHandler) Get(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	result, serviceErr := h.service.Get(c.Request.Context(), id)
	respond(c, http.StatusOK, result, serviceErr)
}
func (h *DeviationAnalysisHandler) Run(c *gin.Context) {
	var request dto.RunDeviationAnalysisRequest
	if !bindJSON(c, &request) {
		return
	}
	result, reused, err := h.service.Run(c.Request.Context(), request, c.GetHeader("Idempotency-Key"), mustActor(c))
	status := http.StatusCreated
	if reused {
		status = http.StatusOK
	}
	respond(c, status, result, err)
}
func (h *DeviationAnalysisHandler) Transition(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	var request dto.DeviationAnalysisTransitionRequest
	if !bindJSON(c, &request) {
		return
	}
	actor := mustActor(c)
	if request.ToState == string(constants.AnalysisConfirmed) &&
		!constants.HasPermission(constants.Role(actor.Role), constants.PermissionAnalysisConfirm) {
		util.Fail(c, util.NewError(http.StatusForbidden, util.CodeForbidden, "role cannot confirm analysis results"))
		return
	}
	result, serviceErr := h.service.Transition(c.Request.Context(), id, request, actor)
	respond(c, http.StatusOK, result, serviceErr)
}
func (h *DeviationAnalysisHandler) Replay(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	result, serviceErr := h.service.Replay(c.Request.Context(), id, mustActor(c))
	respond(c, http.StatusOK, result, serviceErr)
}
