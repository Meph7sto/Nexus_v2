package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type permissionRepositoryStub struct {
	allowed bool
	err     error
}

func (s permissionRepositoryStub) ListByUserID(context.Context, int64) ([]service.AdminPermission, error) {
	return nil, nil
}

func (s permissionRepositoryStub) ReplaceForUser(context.Context, int64, []service.AdminPermission) error {
	return nil
}

func (s permissionRepositoryStub) DeleteForUser(context.Context, int64) error {
	return nil
}

func (s permissionRepositoryStub) HasPermission(context.Context, int64, service.AdminPermissionResource, service.AdminPermissionAction) (bool, error) {
	return s.allowed, s.err
}

func TestAdminPermissionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		role       string
		subject    AuthSubject
		repository service.AdminPermissionRepository
		wantStatus int
	}{
		{
			name:       "super admin bypasses limited permissions",
			role:       service.RoleSuperAdmin,
			subject:    AuthSubject{UserID: 1, PrincipalKind: PrincipalKindHuman},
			repository: permissionRepositoryStub{allowed: false},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "limited admin with grant is allowed",
			role:       service.RoleAdmin,
			subject:    AuthSubject{UserID: 2, PrincipalKind: PrincipalKindHuman},
			repository: permissionRepositoryStub{allowed: true},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "limited admin without grant is denied",
			role:       service.RoleAdmin,
			subject:    AuthSubject{UserID: 2, PrincipalKind: PrincipalKindHuman},
			repository: permissionRepositoryStub{allowed: false},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "ordinary user is denied",
			role:       service.RoleUser,
			subject:    AuthSubject{UserID: 3, PrincipalKind: PrincipalKindHuman},
			repository: permissionRepositoryStub{allowed: true},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyUserRole), tt.role)
				c.Set(string(ContextKeyUser), tt.subject)
				c.Next()
			})
			router.GET("/admin/users", NewAdminPermissionMiddleware(tt.repository)(service.AdminResourceUsers, service.AdminActionView), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/users", nil))
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tt.wantStatus, response.Body.String())
			}
		})
	}
}

func TestRequireHumanSuperAdminRejectsMachinePrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUserRole), service.RoleSuperAdmin)
		c.Set(string(ContextKeyUser), AuthSubject{PrincipalKind: PrincipalKindAdminAPIKey})
		c.Next()
	})
	router.GET("/admin/settings/admin-api-key", RequireHumanSuperAdmin(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/settings/admin-api-key", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
}
