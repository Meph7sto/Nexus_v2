package usagestats

import "time"

// UserUsageRankingSort specifies the metric used by the global user leaderboard.
type UserUsageRankingSort string

const (
	UserUsageRankingByTokens UserUsageRankingSort = "tokens"
	UserUsageRankingByCost   UserUsageRankingSort = "cost"

	UserUsageRankingDefaultPageSize = 20
	UserUsageRankingMaxPageSize     = 100
	UserUsageRankingMaxRangeDays    = 31

	// These endpoint-specific limits were approved for the leaderboard contract.
	UserUsageRankingRateLimitRequests = 6
	UserUsageRankingRateLimitBurst    = 3
	UserUsageRankingRateLimitWindow   = time.Minute
	UserUsageRankingQueryTimeout      = 2 * time.Second
)

// IsValid reports whether the sort value is safe to map to a static SQL order clause.
func (s UserUsageRankingSort) IsValid() bool {
	switch s {
	case UserUsageRankingByTokens, UserUsageRankingByCost:
		return true
	default:
		return false
	}
}

// NormalizeUserUsageRankingSort preserves the API default for an omitted value.
func NormalizeUserUsageRankingSort(s UserUsageRankingSort) UserUsageRankingSort {
	if s == "" {
		return UserUsageRankingByTokens
	}
	return s
}

// UserUsageRankingItem represents one raw row in the global user leaderboard.
// Identity redaction is intentionally performed by the service layer.
type UserUsageRankingItem struct {
	Rank            int64   `json:"rank"`
	UserID          int64   `json:"user_id"`
	Nickname        string  `json:"nickname"`
	Email           string  `json:"email"`
	Requests        int64   `json:"requests"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalActualCost float64 `json:"total_actual_cost"`
}
