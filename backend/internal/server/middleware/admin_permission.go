package middleware

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AdminPermissionMiddleware checks a registered resource/action pair after
// AdminAuth has established the principal. Unknown resource/action pairs fail
// closed instead of falling back to a dashboard-like permission.
type AdminPermissionMiddleware func(resource service.AdminPermissionResource, action service.AdminPermissionAction) gin.HandlerFunc

func NewAdminPermissionMiddleware(repo service.AdminPermissionRepository) AdminPermissionMiddleware {
	return func(resource service.AdminPermissionResource, action service.AdminPermissionAction) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !service.IsAdminPermissionRouteValid(resource, action) {
				AbortWithError(c, http.StatusForbidden, "ADMIN_PERMISSION_DENIED", "Admin permission denied")
				return
			}

			role, ok := GetUserRoleFromContext(c)
			if !ok {
				AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
				return
			}
			switch role {
			case service.RoleSuperAdmin:
				c.Next()
				return
			case service.RoleAdmin:
				subject, ok := GetAuthSubjectFromContext(c)
				if !ok || subject.UserID <= 0 || !subject.IsHuman() || repo == nil {
					AbortWithError(c, http.StatusForbidden, "ADMIN_PERMISSION_DENIED", "Admin permission denied")
					return
				}
				allowed, err := repo.HasPermission(c.Request.Context(), subject.UserID, resource, action)
				if err != nil {
					AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
					return
				}
				if !allowed {
					AbortWithError(c, http.StatusForbidden, "ADMIN_PERMISSION_DENIED", "Admin permission denied")
					return
				}
				c.Next()
				return
			default:
				AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
			}
		}
	}
}

// RequireHumanSuperAdmin protects operations that may only be performed by a
// real super-admin JWT session. Machine credentials have no user ID and are
// rejected with a distinct error code.
func RequireHumanSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := GetAuthSubjectFromContext(c)
		if !ok || !subject.IsHuman() {
			AbortWithError(c, http.StatusForbidden, "MACHINE_CREDENTIAL_FORBIDDEN", "This operation requires a human administrator")
			return
		}
		role, ok := GetUserRoleFromContext(c)
		if !ok || role != service.RoleSuperAdmin {
			AbortWithError(c, http.StatusForbidden, "SUPER_ADMIN_REQUIRED", "Super administrator access required")
			return
		}
		c.Next()
	}
}
