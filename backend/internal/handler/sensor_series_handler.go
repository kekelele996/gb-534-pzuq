package handler
import (
	"net/http"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/service"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"github.com/gin-gonic/gin"
)
type SensorSeriesHandler struct{ service *service.SensorSeriesService }
func NewSensorSeriesHandler(value *service.SensorSeriesService) *SensorSeriesHandler {
	return &SensorSeriesHandler{service: value}
}
func (h *SensorSeriesHandler) List(c *gin.Context) {
	vesselID, ok := optionalUint(c, "vessel_id")
	if !ok {
		return
	}
	recipeID, ok := optionalUint(c, "recipe_id")
	if !ok {
		return
	}
	page, size := util.Pagination(c)
	result, err := h.service.List(c.Request.Context(), dto.SensorSeriesQuery{
		VesselID: vesselID, RecipeID: recipeID, Search: c.Query("search"),
		Channel: c.Query("channel"), State: c.Query("state"), Page: page, PageSize: size,
	})
	respond(c, http.StatusOK, result, err)
}
func (h *SensorSeriesHandler) Get(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	result, serviceErr := h.service.Get(c.Request.Context(), id)
	respond(c, http.StatusOK, result, serviceErr)
}
func (h *SensorSeriesHandler) Import(c *gin.Context) {
	var request dto.ImportSensorSeriesRequest
	if !bindJSON(c, &request) {
		return
	}
	result, err := h.service.Import(c.Request.Context(), request, mustActor(c))
	respond(c, http.StatusCreated, result, err)
}
func (h *SensorSeriesHandler) Transition(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	var request dto.SensorSeriesTransitionRequest
	if !bindJSON(c, &request) {
		return
	}
	result, serviceErr := h.service.Transition(c.Request.Context(), id, request, mustActor(c))
	respond(c, http.StatusOK, result, serviceErr)
}
