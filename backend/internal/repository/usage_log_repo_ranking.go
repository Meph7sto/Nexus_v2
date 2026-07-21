package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var userUsageRankingQueryTimeout = usagestats.UserUsageRankingQueryTimeout

// GetUserUsageRanking returns a paginated global user leaderboard for the requested metric.
func (r *usageLogRepository) GetUserUsageRanking(
	ctx context.Context,
	params pagination.PaginationParams,
	rankBy usagestats.UserUsageRankingSort,
	startTime, endTime time.Time,
) (results []usagestats.UserUsageRankingItem, page *pagination.PaginationResult, err error) {
	params = normalizeUserUsageRankingPagination(params)
	rankBy = usagestats.NormalizeUserUsageRankingSort(rankBy)
	orderBy, ok := userUsageRankingOrderBy(rankBy)
	if !ok {
		return nil, nil, fmt.Errorf("invalid user usage ranking sort %q", rankBy)
	}

	queryCtx, cancel := context.WithTimeout(ctx, userUsageRankingQueryTimeout)
	defer cancel()

	var total int64
	const countQuery = `
		SELECT COUNT(*)
		FROM (
			SELECT ul.user_id
			FROM usage_logs ul
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			GROUP BY ul.user_id
		) ranked_users
	`
	if err := scanSingleRow(queryCtx, r.sql, countQuery, []any{startTime, endTime}, &total); err != nil {
		return nil, nil, userUsageRankingQueryError(queryCtx, err)
	}
	if total == 0 {
		return []usagestats.UserUsageRankingItem{}, paginationResultFromTotal(total, params), nil
	}

	query := fmt.Sprintf(`
		WITH user_usage AS (
			SELECT
				ul.user_id,
				COALESCE(
					NULLIF(TRIM(us.username), ''),
					NULLIF(SPLIT_PART(COALESCE(us.email, ''), '@', 1), ''),
					'User #' || ul.user_id::text
				) AS nickname,
				COALESCE(us.email, '') AS email,
				COUNT(*) AS requests,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS total_tokens,
				COALESCE(SUM(ul.actual_cost), 0) AS total_actual_cost
			FROM usage_logs ul
			LEFT JOIN users us ON us.id = ul.user_id
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			GROUP BY ul.user_id, us.username, us.email
		), ranked AS (
			SELECT
				ROW_NUMBER() OVER (ORDER BY %s) AS rank,
				user_id,
				nickname,
				email,
				requests,
				total_tokens,
				total_actual_cost
			FROM user_usage
		)
		SELECT
			rank,
			user_id,
			nickname,
			email,
			requests,
			total_tokens,
			total_actual_cost
		FROM ranked
		ORDER BY rank ASC
		LIMIT $3 OFFSET $4
	`, orderBy)

	rows, err := r.sql.QueryContext(queryCtx, query, startTime, endTime, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, userUsageRankingQueryError(queryCtx, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = userUsageRankingQueryError(queryCtx, closeErr)
			results = nil
			page = nil
		}
	}()

	results = make([]usagestats.UserUsageRankingItem, 0)
	for rows.Next() {
		var row usagestats.UserUsageRankingItem
		if err = rows.Scan(
			&row.Rank,
			&row.UserID,
			&row.Nickname,
			&row.Email,
			&row.Requests,
			&row.TotalTokens,
			&row.TotalActualCost,
		); err != nil {
			return nil, nil, userUsageRankingQueryError(queryCtx, err)
		}
		results = append(results, row)
	}
	if err = rows.Err(); err != nil {
		return nil, nil, userUsageRankingQueryError(queryCtx, err)
	}

	return results, paginationResultFromTotal(total, params), nil
}

func normalizeUserUsageRankingPagination(params pagination.PaginationParams) pagination.PaginationParams {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = usagestats.UserUsageRankingDefaultPageSize
	}
	if params.PageSize > usagestats.UserUsageRankingMaxPageSize {
		params.PageSize = usagestats.UserUsageRankingMaxPageSize
	}
	return params
}

func userUsageRankingOrderBy(rankBy usagestats.UserUsageRankingSort) (string, bool) {
	switch rankBy {
	case usagestats.UserUsageRankingByTokens:
		return "total_tokens DESC, total_actual_cost DESC, user_id ASC", true
	case usagestats.UserUsageRankingByCost:
		return "total_actual_cost DESC, total_tokens DESC, user_id ASC", true
	default:
		return "", false
	}
}

func userUsageRankingQueryError(queryCtx context.Context, err error) error {
	if errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
		if err == nil {
			return service.ErrUsageRankingQueryTimeout
		}
		return fmt.Errorf("%w: %w", service.ErrUsageRankingQueryTimeout, err)
	}
	return err
}
