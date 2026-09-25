package router
import (
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/handler"
	"fermentation-kinetics-deviation-analysis/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)
func RegisterDeviationAnalysisRoutes(
	api *gin.RouterGroup, h *handler.DeviationAnalysisHandler, runLimiter *middleware.RateLimiter,
) {
	group := api.Group("/deviation-analyses")
	group.GET("", middleware.RequirePermission(constants.PermissionRead), h.List)
	group.GET("/:id", middleware.RequirePermission(constants.PermissionRead), h.Get)
	group.POST("", middleware.RequirePermission(constants.PermissionAnalysisRun), runLimiter.Middleware("analysis-run"), h.Run)
	group.POST("/:id/transition", middleware.RequirePermission(constants.PermissionAnalysisReview), h.Transition)
	group.POST("/:id/replay", middleware.RequirePermission(constants.PermissionAnalysisRun), runLimiter.Middleware("analysis-replay"), h.Replay)
}
