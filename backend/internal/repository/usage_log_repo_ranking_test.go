//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestGetUserUsageRankingUsesStaticOrderAndGlobalRanks(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 31)

	tests := []struct {
		name        string
		rankBy      usagestats.UserUsageRankingSort
		orderBy     string
		page        int
		pageSize    int
		expectedLim int
		expectedOff int
	}{
		{
			name:        "tokens with stable tie breakers",
			rankBy:      usagestats.UserUsageRankingByTokens,
			orderBy:     "total_tokens DESC, total_actual_cost DESC, user_id ASC",
			page:        2,
			pageSize:    2,
			expectedLim: 2,
			expectedOff: 2,
		},
		{
			name:        "cost preserves page size for the handler to cap",
			rankBy:      usagestats.UserUsageRankingByCost,
			orderBy:     "total_actual_cost DESC, total_tokens DESC, user_id ASC",
			page:        1,
			pageSize:    101,
			expectedLim: 101,
			expectedOff: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := &usageLogRepository{sql: db}

			mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
				WithArgs(start, end).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
			mock.ExpectQuery(regexp.QuoteMeta("ROW_NUMBER() OVER (ORDER BY "+tc.orderBy+")")).
				WithArgs(start, end, tc.expectedLim, tc.expectedOff).
				WillReturnRows(sqlmock.NewRows([]string{
					"rank", "user_id", "nickname", "email", "requests", "total_tokens", "total_actual_cost",
				}).
					AddRow(3, 11, "User #11", "", 4, 800, 12.5).
					AddRow(4, 12, "maria", "maria@example.test", 3, 800, 12.5))

			rows, page, err := repo.GetUserUsageRanking(context.Background(), pagination.PaginationParams{
				Page:     tc.page,
				PageSize: tc.pageSize,
			}, tc.rankBy, start, end)

			require.NoError(t, err)
			require.Equal(t, int64(3), rows[0].Rank)
			require.Equal(t, int64(11), rows[0].UserID)
			require.Equal(t, "User #11", rows[0].Nickname)
			require.Equal(t, "", rows[0].Email)
			require.Equal(t, int64(5), page.Total)
			require.Equal(t, tc.expectedLim, page.PageSize)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetUserUsageRankingEmptyAndInvalidSort(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)

	t.Run("empty ranking", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := &usageLogRepository{sql: db}
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WithArgs(start, end).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		rows, page, err := repo.GetUserUsageRanking(context.Background(), pagination.PaginationParams{}, "", start, end)

		require.NoError(t, err)
		require.Empty(t, rows)
		require.Equal(t, 1, page.Page)
		require.Equal(t, 20, page.PageSize)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("unknown sort keeps the Nexus token ordering", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := &usageLogRepository{sql: db}
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WithArgs(start, end).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(regexp.QuoteMeta("ROW_NUMBER() OVER (ORDER BY total_tokens DESC, total_actual_cost DESC, user_id ASC)")).
			WithArgs(start, end, 20, 0).
			WillReturnRows(sqlmock.NewRows([]string{
				"rank", "user_id", "nickname", "email", "requests", "total_tokens", "total_actual_cost",
			}).AddRow(1, 11, "User #11", "", 4, 800, 12.5))

		_, _, err := repo.GetUserUsageRanking(context.Background(), pagination.PaginationParams{}, "tokens DESC; DROP TABLE usage_logs", start, end)

		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
