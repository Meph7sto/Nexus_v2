package usagestats

// UserUsageRankingSort specifies the metric used by the global user leaderboard.
type UserUsageRankingSort string

const (
	UserUsageRankingByTokens UserUsageRankingSort = "tokens"
	UserUsageRankingByCost   UserUsageRankingSort = "cost"
)

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
