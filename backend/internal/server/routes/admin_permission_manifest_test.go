package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminRoutePermissionManifestCoversRegisteredRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{}}
	adminAuth := middleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() })
	auditLog := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	permissions := allowAllAdminPermissionsForRouteTest()

	RegisterAdminRoutes(v1, handlers, adminAuth, permissions, auditLog, stepUp)
	RegisterPaymentRoutes(v1, nil, nil, (*adminhandler.PaymentHandler)(nil), middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }), adminAuth, permissions, auditLog, nil)
	handler.RegisterPageRoutes(v1, t.TempDir(), gin.HandlerFunc(func(c *gin.Context) { c.Next() }), gin.HandlerFunc(adminAuth), permissions, nil, nil)

	require.NoError(t, ValidateAdminRouteManifest(router.Routes()))
}

func TestAdminRoutePermissionManifestFailsClosedForUnknownRoute(t *testing.T) {
	err := ValidateAdminRouteManifest([]gin.RouteInfo{{Method: "GET", Path: "/api/v1/admin/not-registered"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing permission manifest entry")
}

func TestAdminRoutePermissionManifestSeparatesAPIKeyReads(t *testing.T) {
	for _, path := range []string{
		"/api/v1/admin/users/:id/api-keys",
		"/api/v1/admin/groups/:id/api-keys",
	} {
		permission, ok := AdminRoutePermissionFor(http.MethodGet, path)
		require.True(t, ok, path)
		require.Equal(t, service.AdminResourceAPIKeys, permission.Resource, path)
		require.Equal(t, service.AdminActionView, permission.Action, path)
	}
}

func TestAdminRoutePermissionManifestMapsOpsStorageView(t *testing.T) {
	permission, ok := AdminRoutePermissionFor(http.MethodGet, "/api/v1/admin/ops/storage")
	require.True(t, ok)
	require.Equal(t, service.AdminResourceOps, permission.Resource)
	require.Equal(t, service.AdminActionView, permission.Action)
}
