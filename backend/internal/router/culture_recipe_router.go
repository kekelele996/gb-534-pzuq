package router
import (
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/handler"
	"fermentation-kinetics-deviation-analysis/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)
func RegisterCultureRecipeRoutes(api *gin.RouterGroup, h *handler.CultureRecipeHandler) {
	group := api.Group("/culture-recipes")
	group.GET("", middleware.RequirePermission(constants.PermissionRead), h.List)
	group.GET("/:id", middleware.RequirePermission(constants.PermissionRead), h.Get)
	group.POST("", middleware.RequirePermission(constants.PermissionRecipeWrite), h.Create)
	group.PUT("/:id", middleware.RequirePermission(constants.PermissionRecipeWrite), h.Update)
	group.POST("/:id/transition", middleware.RequirePermission(constants.PermissionRecipeWrite), h.Transition)
	group.POST("/:id/copy", middleware.RequirePermission(constants.PermissionRecipeWrite), h.Copy)
}
