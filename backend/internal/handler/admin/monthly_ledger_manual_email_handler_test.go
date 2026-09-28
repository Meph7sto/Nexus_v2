package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type manualLedgerHandlerRepo struct {
	monthlyLedgerHandlerRepoStub
	limit   int
	updated bool
}

func (r *manualLedgerHandlerRepo) EmailQuota(context.Context, time.Time) (*service.MonthlyLedgerEmailQuota, error) {
	return &service.MonthlyLedgerEmailQuota{DailyLimit: r.limit, Used: 2, Date: "2026-09-28"}, nil
}
func (r *manualLedgerHandlerRepo) SetEmailDailyLimit(_ context.Context, limit int, _ int64) error {
	r.limit = limit
	r.updated = true
	return nil
}
func (r *manualLedgerHandlerRepo) ReserveEmailAttempt(context.Context, time.Time) error {
	return service.ErrMonthlyLedgerEmailDailyLimit
}
func (r *manualLedgerHandlerRepo) EmailRecipient(context.Context, int64) (*service.MonthlyLedgerEmailPreference, error) {
	return nil, service.ErrMonthlyLedgerUserNotFound
}

func TestMonthlyLedgerManualEmailHandlerLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		body   string
		status int
		limit  int
	}{
		{`{}`, 400, 5}, {`{"daily_limit":null}`, 400, 5}, {`{"daily_limit":-1}`, 400, 5}, {`{"daily_limit":1.5}`, 400, 5},
		{`{"daily_limit":0}`, 200, 0}, {`{"daily_limit":8}`, 200, 8},
	} {
		repo := &manualLedgerHandlerRepo{limit: 5}
		handler := NewMonthlyLedgerHandler(service.NewMonthlyLedgerService(repo))
		router := gin.New()
		router.PUT("/quota", handler.SetEmailDailyLimit)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/quota", bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		require.Equal(t, tc.status, rec.Code, rec.Body.String())
		require.Equal(t, tc.limit, repo.limit)
		require.Equal(t, tc.status == http.StatusOK, repo.updated)
	}
}

func TestMonthlyLedgerManualEmailHandlerRejectsInvalidRequests(t *testing.T) {
	handler := NewMonthlyLedgerHandler(service.NewMonthlyLedgerService(&manualLedgerHandlerRepo{limit: 5}))
	router := gin.New()
	router.POST("/emails", handler.SendManualEmail)
	router.GET("/quota", handler.EmailQuota)
	for _, body := range []string{`{}`, `{"month":"2026-09","user_id":7}`, `{"month":"2026-09","user_id":7,"amount":-1}`, `{"month":"2026-09","user_id":7,"amount":42,"request_id":"bad"}`} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/emails", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		require.Equal(t, 400, rec.Code, rec.Body.String())
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/quota", nil))
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"daily_limit":5`)
	require.Contains(t, rec.Body.String(), `"used":2`)
}
