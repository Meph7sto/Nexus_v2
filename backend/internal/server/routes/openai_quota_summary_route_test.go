package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIQuotaSummaryRouteServiceStub struct {
	service.AdminService
	calls int
}

func (s *openAIQuotaSummaryRouteServiceStub) GetOpenAIQuotaSummary(_ context.Context, input service.OpenAIQuotaSummaryInput) (*service.OpenAIQuotaSummaryResponse, error) {
	s.calls++
	return &service.OpenAIQuotaSummaryResponse{
		ProjectionAt: input.ProjectionAt,
		GeneratedAt:  input.GeneratedAt,
		Groups:       []service.OpenAIQuotaSummaryGroup{},
	}, nil
}

type openAIQuotaSummaryPermissionRepoStub struct {
	service.AdminPermissionRepository
	allowedUserIDs map[int64]bool
	calls          []openAIQuotaSummaryPermissionCall
}

type openAIQuotaSummaryPermissionCall struct {
	userID   int64
	resource service.AdminPermissionResource
	action   service.AdminPermissionAction
}

func (s *openAIQuotaSummaryPermissionRepoStub) HasPermission(_ context.Context, userID int64, resource service.AdminPermissionResource, action service.AdminPermissionAction) (bool, error) {
	s.calls = append(s.calls, openAIQuotaSummaryPermissionCall{userID: userID, resource: resource, action: action})
	return s.allowedUserIDs[userID], nil
}

func TestOpenAIQuotaSummaryRoutePermissionMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permission, ok := AdminRoutePermissionFor(http.MethodGet, "/api/v1/admin/openai/quota-summary")
	require.True(t, ok)
	require.Equal(t, service.AdminResourceAccounts, permission.Resource)
	require.Equal(t, service.AdminActionView, permission.Action)

	serviceStub := &openAIQuotaSummaryRouteServiceStub{}
	permissionRepo := &openAIQuotaSummaryPermissionRepoStub{allowedUserIDs: map[int64]bool{3: true}}
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		OpenAIOAuth: adminhandler.NewOpenAIOAuthHandler(nil, serviceStub, nil),
	}}
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		switch c.GetHeader("X-Test-Role") {
		case "":
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		case "user":
			setOpenAIQuotaSummaryRoutePrincipal(c, 1, service.RoleUser)
		case "admin-without-accounts-view":
			setOpenAIQuotaSummaryRoutePrincipal(c, 2, service.RoleAdmin)
		case "admin-with-accounts-view":
			setOpenAIQuotaSummaryRoutePrincipal(c, 3, service.RoleAdmin)
		case "super-admin":
			setOpenAIQuotaSummaryRoutePrincipal(c, 4, service.RoleSuperAdmin)
		default:
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		}
		c.Next()
	})
	RegisterAdminRoutes(
		router.Group("/api/v1"),
		handlers,
		adminAuth,
		servermiddleware.NewAdminPermissionMiddleware(permissionRepo),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }),
	)

	for _, testCase := range []struct {
		name       string
		role       string
		wantStatus int
	}{
		{name: "unauthenticated", wantStatus: http.StatusUnauthorized},
		{name: "normal user", role: "user", wantStatus: http.StatusForbidden},
		{name: "limited admin without accounts view", role: "admin-without-accounts-view", wantStatus: http.StatusForbidden},
		{name: "limited admin with accounts view", role: "admin-with-accounts-view", wantStatus: http.StatusOK},
		{name: "super admin", role: "super-admin", wantStatus: http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai/quota-summary", nil)
			if testCase.role != "" {
				request.Header.Set("X-Test-Role", testCase.role)
			}
			router.ServeHTTP(recorder, request)
			require.Equal(t, testCase.wantStatus, recorder.Code)
		})
	}

	require.Equal(t, 2, serviceStub.calls)
	require.Len(t, permissionRepo.calls, 2)
	for _, call := range permissionRepo.calls {
		require.Equal(t, service.AdminResourceAccounts, call.resource)
		require.Equal(t, service.AdminActionView, call.action)
	}
}

func setOpenAIQuotaSummaryRoutePrincipal(c *gin.Context, userID int64, role string) {
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{
		UserID:        userID,
		PrincipalKind: servermiddleware.PrincipalKindHuman,
	})
	c.Set(string(servermiddleware.ContextKeyUserRole), role)
}
