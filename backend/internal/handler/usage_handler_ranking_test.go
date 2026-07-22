package handler

import (
	"context"
	"encoding/json"
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

func newUsageRankingTestRouter(repo *usageRankingRepoCapture, userID *int64, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageService := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageService, nil, nil, nil)
	router := gin.New()
	if userID != nil {
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: *userID})
			if role != "" {
				c.Set(string(middleware2.ContextKeyUserRole), role)
			}
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

func TestUsageRankingRequiresAuthenticatedSubject(t *testing.T) {
	router := newUsageRankingTestRouter(&usageRankingRepoCapture{}, nil, "")

	rec := rankingRequest(router, "/usage/ranking")

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUsageRankingRequiresAuthenticatedUserID(t *testing.T) {
	userID := int64(0)
	router := newUsageRankingTestRouter(&usageRankingRepoCapture{}, &userID, "")

	rec := rankingRequest(router, "/usage/ranking")

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUsageRankingDefaultsToTokensAndClampsPageSize(t *testing.T) {
	userID := int64(42)
	repo := &usageRankingRepoCapture{}
	router := newUsageRankingTestRouter(repo, &userID, "")

	rec := rankingRequest(router, "/usage/ranking?page=2&page_size=101&start_date=2026-03-01&end_date=2026-03-01&timezone=UTC")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, usagestats.UserUsageRankingByTokens, repo.rankBy)
	require.Equal(t, 2, repo.params.Page)
	require.Equal(t, usagestats.UserUsageRankingMaxPageSize, repo.params.PageSize)
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
			userID := int64(42)
			repo := &usageRankingRepoCapture{rows: []usagestats.UserUsageRankingItem{{
				Rank: 1, UserID: 7, Nickname: "Alice", Email: "alice@example.test",
			}}}
			router := newUsageRankingTestRouter(repo, &userID, tc.role)

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

func TestUsageRankingValidatesNaturalDaysAndTimezone(t *testing.T) {
	userID := int64(42)
	cases := []struct {
		name string
		path string
		code int
	}{
		{
			name: "31 days across daylight saving time",
			path: "/usage/ranking?start_date=2026-03-08&end_date=2026-04-07&timezone=America/New_York",
			code: http.StatusOK,
		},
		{
			name: "32 natural days",
			path: "/usage/ranking?start_date=2026-03-08&end_date=2026-04-08&timezone=America/New_York",
			code: http.StatusBadRequest,
		},
		{name: "invalid timezone", path: "/usage/ranking?timezone=Not/AZone", code: http.StatusBadRequest},
		{name: "end before start", path: "/usage/ranking?start_date=2026-04-01&end_date=2026-03-31", code: http.StatusBadRequest},
		{name: "invalid rank", path: "/usage/ranking?rank_by=other", code: http.StatusBadRequest},
		{name: "invalid page", path: "/usage/ranking?page=0", code: http.StatusBadRequest},
		{name: "invalid page size", path: "/usage/ranking?page_size=zero", code: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &usageRankingRepoCapture{}
			router := newUsageRankingTestRouter(repo, &userID, "")

			rec := rankingRequest(router, tc.path)

			require.Equal(t, tc.code, rec.Code)
			if tc.code == http.StatusOK {
				loc, err := time.LoadLocation("America/New_York")
				require.NoError(t, err)
				require.Equal(t, time.Date(2026, 3, 8, 0, 0, 0, 0, loc), repo.startTime)
				require.Equal(t, time.Date(2026, 4, 8, 0, 0, 0, 0, loc), repo.endTime)
			}
		})
	}
}

func TestUsageRankingMapsQueryTimeoutToGatewayTimeout(t *testing.T) {
	userID := int64(42)
	repo := &usageRankingRepoCapture{err: service.ErrUsageRankingQueryTimeout}
	router := newUsageRankingTestRouter(repo, &userID, "")

	rec := rankingRequest(router, "/usage/ranking?start_date=2026-03-01&end_date=2026-03-01")

	require.Equal(t, http.StatusGatewayTimeout, rec.Code)

	var body struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, http.StatusGatewayTimeout, body.Code)
}
