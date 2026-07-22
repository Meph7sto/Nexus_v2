package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageRankingRepoCapture struct {
	service.UsageLogRepository
	params    pagination.PaginationParams
	rankBy    usagestats.UserUsageRankingSort
	startTime time.Time
	endTime   time.Time
	rows      []usagestats.UserUsageRankingItem
	result    *pagination.PaginationResult
	err       error
}

func (s *usageRankingRepoCapture) GetUserUsageRanking(
	_ context.Context,
	params pagination.PaginationParams,
	rankBy usagestats.UserUsageRankingSort,
	startTime, endTime time.Time,
) ([]usagestats.UserUsageRankingItem, *pagination.PaginationResult, error) {
	s.params = params
	s.rankBy = rankBy
	s.startTime = startTime
	s.endTime = endTime
	if s.result == nil {
		s.result = &pagination.PaginationResult{Total: int64(len(s.rows)), Page: params.Page, PageSize: params.PageSize, Pages: 1}
	}
	return s.rows, s.result, s.err
}

func newUsageRankingTestRouter(repo *usageRankingRepoCapture, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageService := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageService, nil, nil, nil)
	router := gin.New()
	if role != "" {
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyUserRole), role)
			c.Next()
		})
	}
	router.GET("/usage/ranking", usageHandler.Ranking)
	return router
}

func rankingRequest(router *gin.Engine, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestUsageRankingDefaultsToTokensAndClampsPageSize(t *testing.T) {
	repo := &usageRankingRepoCapture{}
	router := newUsageRankingTestRouter(repo, "")

	rec := rankingRequest(router, "/usage/ranking?page=2&page_size=101&start_date=2026-03-01&end_date=2026-03-01&timezone=UTC")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, usagestats.UserUsageRankingByTokens, repo.rankBy)
	require.Equal(t, 2, repo.params.Page)
	require.Equal(t, 100, repo.params.PageSize)
}

func TestUsageRankingIdentityVisibility(t *testing.T) {
	roles := []struct {
		name          string
		role          string
		includeUserID bool
	}{
		{name: "regular user"},
		{name: "admin", role: service.RoleAdmin, includeUserID: true},
		{name: "super admin", role: service.RoleSuperAdmin, includeUserID: true},
	}

	for _, tc := range roles {
		t.Run(tc.name, func(t *testing.T) {
			repo := &usageRankingRepoCapture{rows: []usagestats.UserUsageRankingItem{{
				Rank: 1, UserID: 7, Nickname: "Alice", Email: "alice@example.test",
			}}}
			router := newUsageRankingTestRouter(repo, tc.role)

			rec := rankingRequest(router, "/usage/ranking?rank_by=cost&start_date=2026-03-01&end_date=2026-03-01&timezone=UTC")

			require.Equal(t, http.StatusOK, rec.Code)
			if tc.includeUserID {
				require.Contains(t, rec.Body.String(), `"user_id":7`)
				require.Contains(t, rec.Body.String(), "alice@example.test")
			} else {
				require.NotContains(t, rec.Body.String(), `"user_id"`)
				require.NotContains(t, rec.Body.String(), "alice@example.test")
				require.Contains(t, rec.Body.String(), "a****@example.test")
			}
		})
	}
}

func TestUsageRankingKeepsNexusDateRangeAndTimezoneSemantics(t *testing.T) {
	t.Run("accepts a range longer than 31 days", func(t *testing.T) {
		repo := &usageRankingRepoCapture{}
		router := newUsageRankingTestRouter(repo, "")

		rec := rankingRequest(router, "/usage/ranking?start_date=2026-03-08&end_date=2026-04-08&timezone=America/New_York")

		require.Equal(t, http.StatusOK, rec.Code)
		loc, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		require.Equal(t, time.Date(2026, 3, 8, 0, 0, 0, 0, loc), repo.startTime)
		require.Equal(t, time.Date(2026, 4, 9, 0, 0, 0, 0, loc), repo.endTime)
	})

	t.Run("falls back for an invalid timezone", func(t *testing.T) {
		repo := &usageRankingRepoCapture{}
		router := newUsageRankingTestRouter(repo, "")

		rec := rankingRequest(router, "/usage/ranking?timezone=Not/AZone")

		require.Equal(t, http.StatusOK, rec.Code)
		require.False(t, repo.startTime.IsZero())
		require.False(t, repo.endTime.IsZero())
	})

	t.Run("rejects an inverted date range", func(t *testing.T) {
		repo := &usageRankingRepoCapture{}
		router := newUsageRankingTestRouter(repo, "")

		rec := rankingRequest(router, "/usage/ranking?start_date=2026-04-01&end_date=2026-03-31")

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestUsageRankingKeepsNexusPaginationFallbacks(t *testing.T) {
	cases := []struct {
		name         string
		query        string
		wantPage     int
		wantPageSize int
	}{
		{name: "invalid page", query: "?page=0", wantPage: 1, wantPageSize: 20},
		{name: "invalid page size", query: "?page_size=zero", wantPage: 1, wantPageSize: 20},
		{name: "oversized page size", query: "?page_size=1001", wantPage: 1, wantPageSize: 20},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &usageRankingRepoCapture{}
			router := newUsageRankingTestRouter(repo, "")

			rec := rankingRequest(router, "/usage/ranking"+tc.query)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tc.wantPage, repo.params.Page)
			require.Equal(t, tc.wantPageSize, repo.params.PageSize)
		})
	}

	t.Run("invalid rank remains rejected", func(t *testing.T) {
		repo := &usageRankingRepoCapture{}
		router := newUsageRankingTestRouter(repo, "")

		rec := rankingRequest(router, "/usage/ranking?rank_by=other")

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
