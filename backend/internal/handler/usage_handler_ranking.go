package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Ranking handles the global usage leaderboard for authenticated users.
// GET /api/v1/usage/ranking
func (h *UsageHandler) Ranking(c *gin.Context) {
	rankByRaw := strings.TrimSpace(c.DefaultQuery("rank_by", string(usagestats.UserUsageRankingByTokens)))
	var rankBy usagestats.UserUsageRankingSort
	switch rankByRaw {
	case string(usagestats.UserUsageRankingByTokens):
		rankBy = usagestats.UserUsageRankingByTokens
	case string(usagestats.UserUsageRankingByCost):
		rankBy = usagestats.UserUsageRankingByCost
	default:
		response.BadRequest(c, "Invalid rank_by, allowed values: tokens, cost")
		return
	}

	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	startTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -1), userTZ)
	endTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	if startDateStr := strings.TrimSpace(c.Query("start_date")); startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		startTime = t
	}
	if endDateStr := strings.TrimSpace(c.Query("end_date")); endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		endTime = t.AddDate(0, 0, 1)
	}
	if !startTime.Before(endTime) {
		response.BadRequest(c, "start_date must be before or equal to end_date")
		return
	}

	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}

	role, _ := middleware2.GetUserRoleFromContext(c)
	includeIdentity := role == service.RoleAdmin || role == service.RoleSuperAdmin
	items, result, err := h.usageService.GetUserUsageRanking(c.Request.Context(), service.UsageRankingParams{
		RankBy:          rankBy,
		StartTime:       startTime,
		EndTime:         endTime,
		Page:            page,
		PageSize:        pageSize,
		IncludeIdentity: includeIdentity,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result == nil {
		response.Paginated(c, items, 0, page, pageSize)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}
