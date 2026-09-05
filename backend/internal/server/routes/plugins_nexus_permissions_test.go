package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPluginRoutesRequireHumanSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	endpoints := []struct{ method, path string }{
		{"GET", "/api/v1/admin/plugins"},
		{"GET", "/api/v1/admin/plugins/:id"},
		{"POST", "/api/v1/admin/plugins/upload"},
		{"POST", "/api/v1/admin/plugins/:id/enable"},
		{"POST", "/api/v1/admin/plugins/:id/disable"},
		{"DELETE", "/api/v1/admin/plugins/:id"},
		{"GET", "/api/v1/admin/plugins/:id/config"},
		{"PUT", "/api/v1/admin/plugins/:id/config"},
		{"POST", "/api/v1/admin/plugins/:id/test"},
		{"POST", "/api/v1/admin/plugins/:id/ui-session"},
	}
	for _, endpoint := range endpoints {
		permission, ok := AdminRoutePermissionFor(endpoint.method, endpoint.path)
		require.True(t, ok, endpoint.path)
		require.Equal(t, service.AdminPermissionResource("plugins"), permission.Resource)
		require.True(t, permission.HumanOnly, endpoint.path)
	}
	for _, tc := range []struct {
		name, role string
		userID     int64
		want       int
	}{
		{"super admin", service.RoleSuperAdmin, 1, http.StatusNoContent},
		{"limited admin", service.RoleAdmin, 2, http.StatusForbidden},
		{"user", service.RoleUser, 3, http.StatusForbidden},
		{"machine", service.RoleSuperAdmin, 0, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				kind := middleware.PrincipalKindHuman
				if tc.userID == 0 {
					kind = middleware.PrincipalKindAdminAPIKey
				}
				c.Set(string(middleware.ContextKeyUserRole), tc.role)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: tc.userID, PrincipalKind: kind})
				c.Next()
			})
			router.GET("/api/v1/admin/plugins", RequireAdminRoutePermission(middleware.NewAdminPermissionMiddleware(nil)), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/plugins", nil))
			require.Equal(t, tc.want, response.Code, response.Body.String())
		})
	}
}
