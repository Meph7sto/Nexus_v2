package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIncomePermissionDenied(t *testing.T) {
	for _, action := range []service.AdminPermissionAction{service.AdminActionView, service.AdminActionCreate, service.AdminActionUpdate, service.AdminActionDelete} {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(string(ContextKeyUserRole), service.RoleAdmin)
			c.Set(string(ContextKeyUser), AuthSubject{UserID: 2, PrincipalKind: PrincipalKindHuman})
			c.Next()
		})
		router.GET("/income", NewAdminPermissionMiddleware(permissionRepositoryStub{allowed: false})(service.AdminResourceMonthlyLedger, action), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/income", nil))
		require.Equal(t, http.StatusForbidden, rec.Code)
	}
}

func TestIncomeAuditDetails(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	SetAuditExtra(c, map[string]any{"income_record_id": int64(42), "income_cost": 80.0, "income_state": "paused", "new_amount": 100.0, "input": map[string]any{"customer": "private"}})
	v, ok := c.Get(auditCtxKeyExtra)
	require.True(t, ok)
	require.Equal(t, map[string]any{"income_record_id": int64(42), "income_cost": 80.0, "income_state": "paused", "new_amount": 100.0}, v)
	for route, action := range map[string]string{
		"POST /api/v1/admin/monthly-ledger/income-entries":              "income_entry.create",
		"PUT /api/v1/admin/monthly-ledger/income-entries/:id":           "income_entry.update",
		"DELETE /api/v1/admin/monthly-ledger/income-entries/:id":        "income_entry.delete",
		"POST /api/v1/admin/monthly-ledger/income-entries/:id/payments": "income_payment.create",
		"PUT /api/v1/admin/monthly-ledger/income-payments/:id":          "income_payment.update",
		"DELETE /api/v1/admin/monthly-ledger/income-payments/:id":       "income_payment.delete",
		"POST /api/v1/admin/monthly-ledger/income-schedules":            "income_schedule.create",
		"PUT /api/v1/admin/monthly-ledger/income-schedules/:id":         "income_schedule.update",
		"PUT /api/v1/admin/monthly-ledger/income-schedules/:id/state":   "income_schedule.state",
	} {
		require.Equal(t, "admin.monthly_ledger."+action, auditActionOverrides[route])
	}
}
