package router
import (
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/handler"
	"fermentation-kinetics-deviation-analysis/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)
func RegisterFermentationVesselRoutes(api *gin.RouterGroup, h *handler.FermentationVesselHandler) {
	group := api.Group("/fermentation-vessels")
	group.GET("", middleware.RequirePermission(constants.PermissionRead), h.List)
	group.GET("/:id", middleware.RequirePermission(constants.PermissionRead), h.Get)
	group.POST("", middleware.RequirePermission(constants.PermissionVesselWrite), h.Create)
	group.PUT("/:id", middleware.RequirePermission(constants.PermissionVesselWrite), h.Update)
	group.POST("/:id/deactivate", middleware.RequirePermission(constants.PermissionVesselWrite), h.Deactivate)
}
