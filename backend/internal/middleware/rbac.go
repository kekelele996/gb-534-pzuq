package middleware
import (
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"github.com/gin-gonic/gin"
	"net/http"
)
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := ActorFromContext(c)
		if !ok {
			util.Fail(c, util.NewError(http.StatusUnauthorized, util.CodeUnauthorized, "authentication context is missing"))
			return
		}
		if !constants.HasPermission(constants.Role(actor.Role), permission) {
			util.Fail(c, util.NewError(http.StatusForbidden, util.CodeForbidden, "role does not have permission for this action"))
			return
		}
		c.Next()
	}
}
func RequireRoles(roles ...constants.Role) gin.HandlerFunc {
	allowed := make(map[constants.Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		actor, ok := ActorFromContext(c)
		if !ok {
			util.Fail(c, util.NewError(http.StatusUnauthorized, util.CodeUnauthorized, "authentication context is missing"))
			return
		}
		if _, ok := allowed[constants.Role(actor.Role)]; !ok {
			util.Fail(c, util.NewError(http.StatusForbidden, util.CodeForbidden, "role is not allowed to access this resource"))
			return
		}
		c.Next()
	}
}
