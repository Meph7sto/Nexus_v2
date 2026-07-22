package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.usageService == nil {
		response.InternalError(c, "Usage service not available")
		return
	}

	rankByRaw := strings.TrimSpace(c.Query("rank_by"))
	rankBy := usagestats.NormalizeUserUsageRankingSort(usagestats.UserUsageRankingSort(rankByRaw))
	if !rankBy.IsValid() {
		response.BadRequest(c, "Invalid rank_by, allowed values: tokens, cost")
		return
	}

	startTime, endTime, ok := parseUsageRankingRange(c)
	if !ok {
		return
	}
	page, pageSize, ok := parseUsageRankingPagination(c)
	if !ok {
		return
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
		if errors.Is(err, service.ErrUsageRankingQueryTimeout) {
			response.Error(c, http.StatusGatewayTimeout, "Usage ranking query timed out")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	if result == nil {
		response.Paginated(c, items, 0, page, pageSize)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func parseUsageRankingRange(c *gin.Context) (time.Time, time.Time, bool) {
	loc := timezone.Location()
	if userTZ := strings.TrimSpace(c.Query("timezone")); userTZ != "" {
		var err error
		loc, err = time.LoadLocation(userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid timezone")
			return time.Time{}, time.Time{}, false
		}
	}

	now := time.Now().In(loc)
	startTime := usageRankingStartOfDay(now.AddDate(0, 0, -1), loc)
	endTime := usageRankingStartOfDay(now.AddDate(0, 0, 1), loc)

	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		parsed, err := time.ParseInLocation("2006-01-02", startDate, loc)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		startTime = parsed
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		parsed, err := time.ParseInLocation("2006-01-02", endDate, loc)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		endTime = parsed.AddDate(0, 0, 1)
	}

	if !startTime.Before(endTime) {
		response.BadRequest(c, "start_date must be before or equal to end_date")
		return time.Time{}, time.Time{}, false
	}
	if usageRankingNaturalDays(startTime, endTime, loc) > usagestats.UserUsageRankingMaxRangeDays {
		response.BadRequest(c, "Date range cannot exceed 31 natural days")
		return time.Time{}, time.Time{}, false
	}
	return startTime, endTime, true
}

func usageRankingStartOfDay(value time.Time, loc *time.Location) time.Time {
	value = value.In(loc)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc)
}

// usageRankingNaturalDays counts calendar dates instead of elapsed hours, so DST cannot change the limit.
func usageRankingNaturalDays(startTime, endTime time.Time, loc *time.Location) int {
	startTime = startTime.In(loc)
	endTime = endTime.In(loc)
	startDate := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, time.UTC)
	endDate := time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 0, 0, 0, 0, time.UTC)
	return int(endDate.Sub(startDate) / (24 * time.Hour))
}

func parseUsageRankingPagination(c *gin.Context) (int, int, bool) {
	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			response.BadRequest(c, "Invalid page")
			return 0, 0, false
		}
		page = value
	}

	pageSize := usagestats.UserUsageRankingDefaultPageSize
	rawPageSize := strings.TrimSpace(c.Query("page_size"))
	if rawPageSize == "" {
		rawPageSize = strings.TrimSpace(c.Query("limit"))
	}
	if rawPageSize == "" {
		return page, pageSize, true
	}
	value, err := strconv.Atoi(rawPageSize)
	if err != nil || value < 1 {
		response.BadRequest(c, "Invalid page_size")
		return 0, 0, false
	}
	if value > usagestats.UserUsageRankingMaxPageSize {
		value = usagestats.UserUsageRankingMaxPageSize
	}
	return page, value, true
}
