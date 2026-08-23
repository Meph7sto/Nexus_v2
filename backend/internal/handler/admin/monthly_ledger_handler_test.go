package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type monthlyLedgerHandlerRepoStub struct {
	period              service.MonthlyLedgerPeriod
	listParams          service.MonthlyLedgerListParams
	batchUserIDs        []int64
	batchMultiplier     float64
	batchActorID        int64
	paymentBillingMonth time.Time
	created             *service.MonthlyLedgerPayment
}

func (s *monthlyLedgerHandlerRepoStub) List(_ context.Context, period service.MonthlyLedgerPeriod, params service.MonthlyLedgerListParams) ([]service.MonthlyLedgerRow, *service.MonthlyLedgerSummary, *pagination.PaginationResult, error) {
	s.period = period
	s.listParams = params
	return []service.MonthlyLedgerRow{}, &service.MonthlyLedgerSummary{}, &pagination.PaginationResult{Page: 1, PageSize: 20, Pages: 1}, nil
}

func (s *monthlyLedgerHandlerRepoStub) ListPayments(context.Context, time.Time, int64) ([]service.MonthlyLedgerPayment, error) {
	return []service.MonthlyLedgerPayment{}, nil
}

func (s *monthlyLedgerHandlerRepoStub) SetMultiplier(context.Context, time.Time, int64, float64, int64) (float64, error) {
	return 1, nil
}

func (s *monthlyLedgerHandlerRepoStub) SetMultipliers(_ context.Context, _ time.Time, userIDs []int64, multiplier float64, actorID int64) (int64, error) {
	s.batchUserIDs = append([]int64(nil), userIDs...)
	s.batchMultiplier = multiplier
	s.batchActorID = actorID
	return int64(len(userIDs)), nil
}

func (s *monthlyLedgerHandlerRepoStub) CreatePayment(_ context.Context, billingMonth time.Time, payment *service.MonthlyLedgerPayment) error {
	s.paymentBillingMonth = billingMonth
	payment.ID = 41
	copy := *payment
	s.created = &copy
	return nil
}

func (s *monthlyLedgerHandlerRepoStub) UpdatePayment(context.Context, int64, service.MonthlyLedgerPaymentUpdate, int64) (*service.MonthlyLedgerPayment, *service.MonthlyLedgerPayment, error) {
	return nil, nil, service.ErrMonthlyLedgerPaymentNotFound
}

func (s *monthlyLedgerHandlerRepoStub) DeletePayment(context.Context, int64) (*service.MonthlyLedgerPayment, error) {
	return nil, service.ErrMonthlyLedgerPaymentNotFound
}

func setupMonthlyLedgerHandlerRouter(repo service.MonthlyLedgerRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Next()
	})
	handler := NewMonthlyLedgerHandler(service.NewMonthlyLedgerService(repo))
	router.GET("/monthly-ledger", handler.List)
	router.PUT("/monthly-ledger/:month/multipliers", handler.SetMultipliers)
	router.POST("/monthly-ledger/:month/users/:user_id/payments", handler.CreatePayment)
	return router
}

func TestMonthlyLedgerHandlerListRejectsInvalidMonth(t *testing.T) {
	router := setupMonthlyLedgerHandlerRouter(&monthlyLedgerHandlerRepoStub{})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monthly-ledger?month=2026-8", nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestMonthlyLedgerHandlerListDefaultsToPreviousMonth(t *testing.T) {
	repo := &monthlyLedgerHandlerRepoStub{}
	router := setupMonthlyLedgerHandlerRouter(repo)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monthly-ledger", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, repo.period.CanRecordPayments)
	require.Equal(t, repo.period.Start.AddDate(0, 1, 0), repo.period.End)
}

func TestMonthlyLedgerHandlerListParsesExactUserID(t *testing.T) {
	repo := &monthlyLedgerHandlerRepoStub{}
	router := setupMonthlyLedgerHandlerRouter(repo)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monthly-ledger?user_id=7", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(7), repo.listParams.UserID)
}

func TestMonthlyLedgerHandlerListRejectsInvalidExactUserID(t *testing.T) {
	router := setupMonthlyLedgerHandlerRouter(&monthlyLedgerHandlerRepoStub{})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monthly-ledger?user_id=customer", nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestMonthlyLedgerHandlerCreatesHistoricalPayment(t *testing.T) {
	repo := &monthlyLedgerHandlerRepoStub{}
	router := setupMonthlyLedgerHandlerRouter(repo)
	month := time.Now().In(time.Local).AddDate(0, -1, 0).Format("2006-01")
	payload := map[string]any{
		"amount":  300.00,
		"paid_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		"note":    "paid in cash",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/monthly-ledger/"+month+"/users/7/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var responseBody struct {
		Data service.MonthlyLedgerPayment `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &responseBody))
	require.Equal(t, month, responseBody.Data.BillingMonth)
	require.NotNil(t, repo.created)
	require.Equal(t, month, repo.paymentBillingMonth.Format("2006-01"))
	require.Equal(t, 1, repo.paymentBillingMonth.Day())
	require.Equal(t, int64(7), repo.created.UserID)
	require.Equal(t, int64(99), repo.created.CreatedBy)
	require.Equal(t, 300.0, repo.created.Amount)
}

func TestMonthlyLedgerHandlerRejectsCurrentMonthPayment(t *testing.T) {
	router := setupMonthlyLedgerHandlerRouter(&monthlyLedgerHandlerRepoStub{})
	month := time.Now().Format("2006-01")
	body := bytes.NewBufferString(`{"amount":1,"paid_at":"2026-01-01T00:00:00Z"}`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/monthly-ledger/"+month+"/users/7/payments", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestMonthlyLedgerHandlerSetsMultipliers(t *testing.T) {
	repo := &monthlyLedgerHandlerRepoStub{}
	router := setupMonthlyLedgerHandlerRouter(repo)
	month := time.Now().In(time.Local).Format("2006-01")
	body := bytes.NewBufferString(`{"user_ids":[7,11,7],"multiplier":0.5}`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/monthly-ledger/"+month+"/multipliers", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{7, 11}, repo.batchUserIDs)
	require.Equal(t, 0.5, repo.batchMultiplier)
	require.Equal(t, int64(99), repo.batchActorID)
	var responseBody struct {
		Data service.MonthlyLedgerMultipliersUpdate `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &responseBody))
	require.Equal(t, int64(2), responseBody.Data.UpdatedCount)
	require.Equal(t, month, responseBody.Data.BillingMonth)
}

func TestMonthlyLedgerHandlerRejectsEmptyMultiplierBatch(t *testing.T) {
	router := setupMonthlyLedgerHandlerRouter(&monthlyLedgerHandlerRepoStub{})
	month := time.Now().In(time.Local).Format("2006-01")
	body := bytes.NewBufferString(`{"user_ids":[],"multiplier":0.5}`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/monthly-ledger/"+month+"/multipliers", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
