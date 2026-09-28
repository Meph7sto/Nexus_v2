package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIncomeHandlerValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewMonthlyLedgerHandler(service.NewMonthlyLedgerService(nil))
	for _, tc := range []struct {
		path, body string
		handler    gin.HandlerFunc
	}{
		{"/income-entries", `{"title":"Top-up","customer":"External","category":"Other","amount":0,"billing_date":"2026-09-01","due_date":"2026-09-01"}`, h.SaveIncomeEntry},
		{"/income-entries", `{"title":"Top-up","customer":"External","category":"Other","amount":10,"cost":-1,"billing_date":"2026-09-01","due_date":"2026-09-01"}`, h.SaveIncomeEntry},
		{"/income-entries/1/payments", `{"amount":1.001,"paid_at":"2026-09-01T00:00:00Z"}`, h.SaveIncomePayment},
		{"/income-entries/1/payments", `{"amount":10,"paid_at":"2999-09-01T00:00:00Z"}`, h.SaveIncomePayment},
		{"/income-schedules", `{"title":"Top-up","customer":"External","category":"Other","amount":10,"first_date":"2026-01-31","interval_months":0}`, h.SaveIncomeSchedule},
	} {
		router := gin.New()
		path := tc.path
		if strings.Contains(path, "/payments") {
			path = "/income-entries/:id/payments"
		}
		router.POST(path, tc.handler)
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
