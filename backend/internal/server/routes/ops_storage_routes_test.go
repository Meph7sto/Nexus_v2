package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsStorageRouteRepository struct {
	service.OpsRepository
}

func (opsStorageRouteRepository) GetCurrentDatabaseSizeBytes(context.Context) (int64, error) {
	return 42, nil
}

type opsStoragePermissionRepository struct{}

func (opsStoragePermissionRepository) ListByUserID(context.Context, int64) ([]service.AdminPermission, error) {
	return nil, nil
}

func (opsStoragePermissionRepository) ReplaceForUser(context.Context, int64, []service.AdminPermission) error {
	return nil
}

func (opsStoragePermissionRepository) DeleteForUser(context.Context, int64) error {
	return nil
}

func (opsStoragePermissionRepository) HasPermission(_ context.Context, userID int64, resource service.AdminPermissionResource, action service.AdminPermissionAction) (bool, error) {
	return userID == 12 && resource == service.AdminResourceOps && action == service.AdminActionView, nil
}

func TestOpsStorageRouteRequiresOpsView(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("OPS_STORAGE_PATHS", "")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := service.NewOpsService(opsStorageRouteRepository{}, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Ops: adminhandler.NewOpsHandler(svc)}}
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		switch c.GetHeader("Authorization") {
		case "":
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
		case "Bearer user":
			setOpsStorageRoutePrincipal(c, service.RoleUser, 10)
		case "Bearer admin-without-ops":
			setOpsStorageRoutePrincipal(c, service.RoleAdmin, 11)
		case "Bearer admin-with-ops":
			setOpsStorageRoutePrincipal(c, service.RoleAdmin, 12)
		case "Bearer super-admin":
			setOpsStorageRoutePrincipal(c, service.RoleSuperAdmin, 13)
		default:
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
		}
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	permissions := servermiddleware.NewAdminPermissionMiddleware(opsStoragePermissionRepository{})
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, permissions, auditLog, stepUp)

	for _, testCase := range []struct {
		name       string
		authority  string
		wantStatus int
	}{
		{name: "unauthenticated", wantStatus: http.StatusUnauthorized},
		{name: "ordinary user", authority: "Bearer user", wantStatus: http.StatusForbidden},
		{name: "admin without ops view", authority: "Bearer admin-without-ops", wantStatus: http.StatusForbidden},
		{name: "admin with ops view", authority: "Bearer admin-with-ops", wantStatus: http.StatusOK},
		{name: "super admin", authority: "Bearer super-admin", wantStatus: http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/storage", nil)
			if testCase.authority != "" {
				request.Header.Set("Authorization", testCase.authority)
			}
			router.ServeHTTP(recorder, request)
			require.Equal(t, testCase.wantStatus, recorder.Code, recorder.Body.String())
		})
	}
}

func setOpsStorageRoutePrincipal(c *gin.Context, role string, userID int64) {
	c.Set(string(servermiddleware.ContextKeyUserRole), role)
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{
		UserID:        userID,
		PrincipalKind: servermiddleware.PrincipalKindHuman,
	})
	c.Next()
}
