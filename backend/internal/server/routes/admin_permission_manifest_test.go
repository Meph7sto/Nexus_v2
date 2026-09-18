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

	RegisterAdminRoutes(v1, handlers, adminAuth, permissions, auditLog, stepUp, nil, nil)
	RegisterPaymentRoutes(v1, nil, nil, (*adminhandler.PaymentHandler)(nil), middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }), adminAuth, permissions, auditLog, nil, nil)
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

func TestAdminRoutePermissionManifestMapsUpstreamAdminRoutes(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		resource service.AdminPermissionResource
		action   service.AdminPermissionAction
	}{
		{http.MethodGet, "/api/v1/admin/accounts/upstream-billing-rates", service.AdminResourceAccounts, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/settings/openai-images-oauth-unavailable-cooldown", service.AdminResourceSettings, service.AdminActionView},
		{http.MethodPut, "/api/v1/admin/settings/openai-images-oauth-unavailable-cooldown", service.AdminResourceSettings, service.AdminActionUpdate},
		{http.MethodGet, "/api/v1/admin/groups/live-capability", service.AdminResourceGroups, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/groups/:id/model-allowlist-candidates", service.AdminResourceGroups, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/accounts/:id/grok-media-eligibility", service.AdminResourceAccounts, service.AdminActionView},
		{http.MethodPut, "/api/v1/admin/accounts/:id/grok-media-eligibility", service.AdminResourceAccounts, service.AdminActionUpdate},
		{http.MethodPost, "/api/v1/admin/subscriptions/bulk-action", service.AdminResourceSubscriptions, service.AdminActionExecute},
		{http.MethodPost, "/api/v1/admin/accounts/usage/batch", service.AdminResourceAccounts, service.AdminActionView},
		{http.MethodPost, "/api/v1/admin/accounts/batch-delete", service.AdminResourceAccounts, service.AdminActionDelete},
		{http.MethodPost, "/api/v1/admin/openai/accounts/:id/quota/refresh", service.AdminResourceAccounts, service.AdminActionExecute},
		{http.MethodGet, "/api/v1/admin/grok/oauth/capabilities", service.AdminResourceAccounts, service.AdminActionView},
		{http.MethodPost, "/api/v1/admin/grok/oauth/sso-token", service.AdminResourceAccounts, service.AdminActionExecute},
		{http.MethodPost, "/api/v1/admin/grok/oauth/password", service.AdminResourceAccounts, service.AdminActionExecute},
		{http.MethodGet, "/api/v1/admin/cn-providers/accounts/:id/quota", service.AdminResourceAccounts, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/cn-providers/accounts/:id/balance", service.AdminResourceAccounts, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/settings/panel-rate-limit", service.AdminResourceSettings, service.AdminActionView},
		{http.MethodPut, "/api/v1/admin/settings/panel-rate-limit", service.AdminResourceSettings, service.AdminActionUpdate},
		{http.MethodGet, "/api/v1/admin/channel-monitor-v2/config", service.AdminResourceChannelMonitor, service.AdminActionView},
		{http.MethodPut, "/api/v1/admin/channel-monitor-v2/config", service.AdminResourceChannelMonitor, service.AdminActionUpdate},
		{http.MethodGet, "/api/v1/admin/channel-monitor-v2/dimensions", service.AdminResourceChannelMonitor, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/channel-monitor-v2/snapshot", service.AdminResourceChannelMonitor, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/channel-monitor-v2/models", service.AdminResourceChannelMonitor, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/channel-monitor-v2/matrix", service.AdminResourceChannelMonitor, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/channel-monitor-v2/errors", service.AdminResourceChannelMonitor, service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/channel-monitor-v2/users", service.AdminResourceChannelMonitor, service.AdminActionView},
	}

	for _, test := range tests {
		permission, ok := AdminRoutePermissionFor(test.method, test.path)
		require.Truef(t, ok, "%s %s", test.method, test.path)
		require.Equal(t, test.resource, permission.Resource, test.path)
		require.Equal(t, test.action, permission.Action, test.path)
	}
}

func TestAdminRoutePermissionManifestSeparatesUsageInteractionContentAndRaw(t *testing.T) {
	structured, ok := AdminRoutePermissionFor(http.MethodGet, "/api/v1/admin/usage/:id/interaction")
	require.True(t, ok)
	require.Equal(t, service.AdminResourceUsageInteractions, structured.Resource)
	require.Equal(t, service.AdminActionView, structured.Action)
	require.False(t, structured.HumanOnly)

	raw, ok := AdminRoutePermissionFor(http.MethodGet, "/api/v1/admin/usage/:id/interaction/raw")
	require.True(t, ok)
	require.Equal(t, service.AdminResourceUsageInteractionRaw, raw.Resource)
	require.Equal(t, service.AdminActionView, raw.Action)
	require.True(t, raw.HumanOnly)
}

func TestAdminRoutePermissionManifestMapsMonthlyLedgerActions(t *testing.T) {
	tests := []struct {
		method string
		path   string
		action service.AdminPermissionAction
	}{
		{http.MethodGet, "/api/v1/admin/monthly-ledger", service.AdminActionView},
		{http.MethodGet, "/api/v1/admin/monthly-ledger/:month/users/:user_id/payments", service.AdminActionView},
		{http.MethodPost, "/api/v1/admin/monthly-ledger/:month/users/:user_id/payments", service.AdminActionCreate},
		{http.MethodPut, "/api/v1/admin/monthly-ledger/:month/multipliers", service.AdminActionUpdate},
		{http.MethodPut, "/api/v1/admin/monthly-ledger/:month/users/:user_id/multiplier", service.AdminActionUpdate},
		{http.MethodPut, "/api/v1/admin/monthly-ledger/:month/users/:user_id/settlement", service.AdminActionUpdate},
		{http.MethodPut, "/api/v1/admin/monthly-ledger/payments/:id", service.AdminActionUpdate},
		{http.MethodDelete, "/api/v1/admin/monthly-ledger/payments/:id", service.AdminActionDelete},
	}

	for _, tt := range tests {
		permission, ok := AdminRoutePermissionFor(tt.method, tt.path)
		require.True(t, ok, "%s %s", tt.method, tt.path)
		require.Equal(t, service.AdminResourceMonthlyLedger, permission.Resource)
		require.Equal(t, tt.action, permission.Action)
	}
}
