package admin

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

type ledgerHistoryHandlerRepo struct {
	monthlyLedgerHandlerRepoStub
	service.MonthlyLedgerEmailHistoryRepository
	user       int64
	month      string
	page, size int
}

func (r *ledgerHistoryHandlerRepo) ListEmailHistory(_ context.Context, month string, userID int64, page, size int) (*service.MonthlyLedgerEmailHistory, error) {
	r.user, r.month, r.page, r.size = userID, month, page, size
	return &service.MonthlyLedgerEmailHistory{Items: []service.MonthlyLedgerEmailRecord{{ID: 1, Amount: 42.35, Subject: "Bill", HTML: "<p>Saved body</p>", Status: "sent"}}, Total: 1, SentCount: 1}, nil
}
func TestMonthlyLedgerEmailHistoryHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &ledgerHistoryHandlerRepo{}
	h := NewMonthlyLedgerHandler(service.NewMonthlyLedgerService(repo))
	router := gin.New()
	router.GET("/:month/users/:user_id/emails", h.ListEmailHistory)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/2020-08/users/7/emails?page=2&page_size=10", nil))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.EqualValues(t, 7, repo.user)
	require.Equal(t, "2020-08", repo.month)
	require.Equal(t, 2, repo.page)
	require.Equal(t, 10, repo.size)
	require.Contains(t, rec.Body.String(), `"sent_count":1`)
	require.Contains(t, rec.Body.String(), `"amount":42.35`)
	for _, path := range []string{"/bad/users/7/emails", "/2020-08/users/0/emails", "/2020-08/users/no/emails"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 400, rec.Code)
	}
}
