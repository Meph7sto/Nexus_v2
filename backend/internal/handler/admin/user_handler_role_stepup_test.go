package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupUserAuthorizationRouter(t *testing.T, subject middleware.AuthSubject, role string) (*gin.Engine, *stubAdminService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), subject)
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	adminSvc := newStubAdminService()
	// 追加一个已是管理员的目标用户，用于验证受限管理员不能管理其他管理员。
	adminSvc.users = append(adminSvc.users, service.User{
		ID:     2,
		Email:  "admin@example.com",
		Role:   service.RoleAdmin,
		Status: service.StatusActive,
	})

	h := NewUserHandler(adminSvc, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/users", h.Create)
	router.PUT("/api/v1/admin/users/:id", h.Update)
	return router, adminSvc
}

func doJSON(t *testing.T, router *gin.Engine, method, path string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

func TestUpdateUserPromoteToAdminRequiresStepUp(t *testing.T) {
	router, _ := setupUserAuthorizationRouter(t, middleware.AuthSubject{
		UserID:        9,
		PrincipalKind: middleware.PrincipalKindHuman,
	}, service.RoleAdmin)

	rec := doJSON(t, router, http.MethodPut, "/api/v1/admin/users/1", map[string]any{"role": "admin"})
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestLimitedAdminCanUpdateRegularUserWithoutRoleOrPermissions(t *testing.T) {
	router, _ := setupUserAuthorizationRouter(t, middleware.AuthSubject{
		UserID:        9,
		PrincipalKind: middleware.PrincipalKindHuman,
	}, service.RoleAdmin)

	rec := doJSON(t, router, http.MethodPut, "/api/v1/admin/users/1", map[string]any{"email": "u@example.com"})
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestLimitedAdminCannotUpdateAdministratorTarget(t *testing.T) {
	router, _ := setupUserAuthorizationRouter(t, middleware.AuthSubject{
		UserID:        9,
		PrincipalKind: middleware.PrincipalKindHuman,
	}, service.RoleAdmin)

	rec := doJSON(t, router, http.MethodPut, "/api/v1/admin/users/2", map[string]any{"email": "admin@example.com"})
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestLimitedAdminCannotReplacePermissions(t *testing.T) {
	router, _ := setupUserAuthorizationRouter(t, middleware.AuthSubject{
		UserID:        9,
		PrincipalKind: middleware.PrincipalKindHuman,
	}, service.RoleAdmin)

	rec := doJSON(t, router, http.MethodPut, "/api/v1/admin/users/1", map[string]any{
		"admin_permissions": []any{},
	})
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHumanSuperAdminCanUpdateAdministratorTarget(t *testing.T) {
	router, _ := setupUserAuthorizationRouter(t, middleware.AuthSubject{
		UserID:        9,
		PrincipalKind: middleware.PrincipalKindHuman,
	}, service.RoleSuperAdmin)

	rec := doJSON(t, router, http.MethodPut, "/api/v1/admin/users/2", map[string]any{"email": "admin@example.com"})
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestLimitedAdminCannotCreateAdministrator(t *testing.T) {
	router, _ := setupUserAuthorizationRouter(t, middleware.AuthSubject{
		UserID:        9,
		PrincipalKind: middleware.PrincipalKindHuman,
	}, service.RoleAdmin)

	rec := doJSON(t, router, http.MethodPost, "/api/v1/admin/users", map[string]any{
		"email": "new-admin@example.com", "password": "pass123", "role": "admin",
	})
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestLimitedAdminCanCreateRegularUser(t *testing.T) {
	router, _ := setupUserAuthorizationRouter(t, middleware.AuthSubject{
		UserID:        9,
		PrincipalKind: middleware.PrincipalKindHuman,
	}, service.RoleAdmin)

	rec := doJSON(t, router, http.MethodPost, "/api/v1/admin/users", map[string]any{
		"email": "new-user@example.com", "password": "pass123", "role": "user",
	})
	require.Equal(t, http.StatusOK, rec.Code)
}
