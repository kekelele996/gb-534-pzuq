package router
import (
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/handler"
	"fermentation-kinetics-deviation-analysis/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)
func RegisterSensorSeriesRoutes(
	api *gin.RouterGroup, h *handler.SensorSeriesHandler, importLimiter *middleware.RateLimiter,
) {
	group := api.Group("/sensor-series")
	group.GET("", middleware.RequirePermission(constants.PermissionRead), h.List)
	group.GET("/:id", middleware.RequirePermission(constants.PermissionRead), h.Get)
	group.POST("", middleware.RequirePermission(constants.PermissionSeriesImport), importLimiter.Middleware("series-import"), h.Import)
	group.POST("/:id/transition", middleware.RequirePermission(constants.PermissionSeriesProcess), h.Transition)
}
