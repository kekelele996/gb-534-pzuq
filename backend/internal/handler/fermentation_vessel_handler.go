package handler
import (
	"net/http"
	"strconv"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/middleware"
	"fermentation-kinetics-deviation-analysis/backend/internal/service"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"github.com/gin-gonic/gin"
)
type FermentationVesselHandler struct {
	service *service.FermentationVesselService
}
func NewFermentationVesselHandler(value *service.FermentationVesselService) *FermentationVesselHandler {
	return &FermentationVesselHandler{service: value}
}
func (h *FermentationVesselHandler) List(c *gin.Context) {
	page, size := util.Pagination(c)
	result, err := h.service.List(c.Request.Context(), dto.FermentationVesselQuery{
		Search: c.Query("search"), Location: c.Query("location"), OwnerTeam: c.Query("owner_team"),
		State: c.Query("state"), Page: page, PageSize: size,
	})
	respond(c, http.StatusOK, result, err)
}
func (h *FermentationVesselHandler) Get(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	result, serviceErr := h.service.Get(c.Request.Context(), id)
	respond(c, http.StatusOK, result, serviceErr)
}
func (h *FermentationVesselHandler) Create(c *gin.Context) {
	var request dto.CreateFermentationVesselRequest
	if !bindJSON(c, &request) {
		return
	}
	result, err := h.service.Create(c.Request.Context(), request, mustActor(c))
	respond(c, http.StatusCreated, result, err)
}
func (h *FermentationVesselHandler) Update(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	var request dto.UpdateFermentationVesselRequest
	if !bindJSON(c, &request) {
		return
	}
	result, serviceErr := h.service.Update(c.Request.Context(), id, request, mustActor(c))
	respond(c, http.StatusOK, result, serviceErr)
}
func (h *FermentationVesselHandler) Deactivate(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	result, serviceErr := h.service.Deactivate(c.Request.Context(), id, mustActor(c))
	respond(c, http.StatusOK, result, serviceErr)
}
func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		util.Fail(c, util.WrapError(http.StatusBadRequest, util.CodeValidation, "request body validation failed", err))
		return false
	}
	return true
}
func respond(c *gin.Context, status int, data any, err error) {
	if err != nil {
		util.Fail(c, err)
		return
	}
	util.Success(c, status, data)
}
func mustActor(c *gin.Context) util.Actor {
	actor, _ := middleware.ActorFromContext(c)
	return actor
}
func optionalUint(c *gin.Context, key string) (uint, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		util.Fail(c, util.NewError(http.StatusBadRequest, util.CodeBadRequest, key+" must be a positive integer"))
		return 0, false
	}
	return uint(value), true
}
