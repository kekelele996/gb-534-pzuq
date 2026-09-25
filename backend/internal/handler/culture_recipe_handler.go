package handler
import (
	"net/http"
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/dto"
	"fermentation-kinetics-deviation-analysis/backend/internal/service"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"github.com/gin-gonic/gin"
)
type CultureRecipeHandler struct{ service *service.CultureRecipeService }
func NewCultureRecipeHandler(value *service.CultureRecipeService) *CultureRecipeHandler {
	return &CultureRecipeHandler{service: value}
}
func (h *CultureRecipeHandler) List(c *gin.Context) {
	vesselID, ok := optionalUint(c, "vessel_id")
	if !ok {
		return
	}
	page, size := util.Pagination(c)
	result, err := h.service.List(c.Request.Context(), dto.CultureRecipeQuery{
		VesselID: vesselID, Search: c.Query("search"), State: c.Query("state"), Page: page, PageSize: size,
	})
	respond(c, http.StatusOK, result, err)
}
func (h *CultureRecipeHandler) Get(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	result, serviceErr := h.service.Get(c.Request.Context(), id)
	respond(c, http.StatusOK, result, serviceErr)
}
func (h *CultureRecipeHandler) Create(c *gin.Context) {
	var request dto.CreateCultureRecipeRequest
	if !bindJSON(c, &request) {
		return
	}
	result, err := h.service.Create(c.Request.Context(), request, mustActor(c))
	respond(c, http.StatusCreated, result, err)
}
func (h *CultureRecipeHandler) Update(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	var request dto.UpdateCultureRecipeRequest
	if !bindJSON(c, &request) {
		return
	}
	result, serviceErr := h.service.Update(c.Request.Context(), id, request, mustActor(c))
	respond(c, http.StatusOK, result, serviceErr)
}
func (h *CultureRecipeHandler) Transition(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	var request dto.CultureRecipeTransitionRequest
	if !bindJSON(c, &request) {
		return
	}
	actor := mustActor(c)
	if request.ToState == string(constants.RecipePublished) &&
		!constants.HasPermission(constants.Role(actor.Role), constants.PermissionRecipePublish) {
		util.Fail(c, util.NewError(http.StatusForbidden, util.CodeForbidden, "role cannot publish recipe versions"))
		return
	}
	result, serviceErr := h.service.Transition(c.Request.Context(), id, request, actor)
	respond(c, http.StatusOK, result, serviceErr)
}
func (h *CultureRecipeHandler) Copy(c *gin.Context) {
	id, err := util.ParseUintParam(c, "id")
	if err != nil {
		util.Fail(c, err)
		return
	}
	var request dto.CopyCultureRecipeRequest
	if !bindJSON(c, &request) {
		return
	}
	result, serviceErr := h.service.Copy(c.Request.Context(), id, request, mustActor(c))
	respond(c, http.StatusCreated, result, serviceErr)
}
