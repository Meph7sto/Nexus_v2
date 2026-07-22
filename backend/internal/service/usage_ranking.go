package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// UsageRankingParams is the service contract for the authenticated user leaderboard.
type UsageRankingParams struct {
	RankBy          usagestats.UserUsageRankingSort
	StartTime       time.Time
	EndTime         time.Time
	Page            int
	PageSize        int
	IncludeIdentity bool
}

// UsageRankingItem is the user-facing leaderboard row. UserID is deliberately
// optional so regular users never receive an identifier to correlate with a person.
type UsageRankingItem struct {
	Rank            int64   `json:"rank"`
	UserID          *int64  `json:"user_id,omitempty"`
	Nickname        string  `json:"nickname"`
	Email           string  `json:"email"`
	Requests        int64   `json:"requests"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalActualCost float64 `json:"total_actual_cost"`
}

// GetUserUsageRanking returns the global user leaderboard with server-side identity redaction.
func (s *UsageService) GetUserUsageRanking(ctx context.Context, params UsageRankingParams) ([]UsageRankingItem, *pagination.PaginationResult, error) {
	type usageRankingRepository interface {
		GetUserUsageRanking(
			ctx context.Context,
			params pagination.PaginationParams,
			rankBy usagestats.UserUsageRankingSort,
			startTime, endTime time.Time,
		) ([]usagestats.UserUsageRankingItem, *pagination.PaginationResult, error)
	}

	repo, ok := s.usageRepo.(usageRankingRepository)
	if !ok {
		return nil, nil, fmt.Errorf("usage ranking repository is not available")
	}

	rows, result, err := repo.GetUserUsageRanking(ctx, pagination.PaginationParams{
		Page:     params.Page,
		PageSize: params.PageSize,
	}, params.RankBy, params.StartTime, params.EndTime)
	if err != nil {
		return nil, nil, fmt.Errorf("get user usage ranking: %w", err)
	}

	items := make([]UsageRankingItem, 0, len(rows))
	for _, row := range rows {
		item := UsageRankingItem{
			Rank:            row.Rank,
			Nickname:        row.Nickname,
			Email:           row.Email,
			Requests:        row.Requests,
			TotalTokens:     row.TotalTokens,
			TotalActualCost: row.TotalActualCost,
		}
		if params.IncludeIdentity {
			userID := row.UserID
			item.UserID = &userID
		} else {
			item.Nickname = maskUsageRankingNickname(item.Nickname)
			item.Email = maskUsageRankingEmail(item.Email)
		}
		items = append(items, item)
	}

	return items, result, nil
}

func maskUsageRankingNickname(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "User"
	}
	runes := []rune(value)
	if len(runes) <= 2 {
		return string(runes[0]) + "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1])
}

func maskUsageRankingEmail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.SplitN(value, "@", 2)
	if len(parts) != 2 {
		return maskUsageRankingNickname(value)
	}
	local := []rune(parts[0])
	if len(local) == 0 {
		return "***@" + parts[1]
	}
	stars := len(local) - 1
	if stars < 3 {
		stars = 3
	}
	return string(local[0]) + strings.Repeat("*", stars) + "@" + parts[1]
}
