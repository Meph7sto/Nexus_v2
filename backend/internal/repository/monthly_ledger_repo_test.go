package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMonthlyLedgerListUsesOneDatasetWithFullSummaryAndFilteredTotal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	loc := time.FixedZone("UTC+8", 8*60*60)
	period := service.MonthlyLedgerPeriod{
		Month:             "2026-07",
		Start:             time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		End:               time.Date(2026, 8, 1, 0, 0, 0, 0, loc),
		CanRecordPayments: true,
	}
	columns := []string{
		"summary_usage_amount", "summary_receivable_amount", "summary_paid_amount",
		"summary_outstanding_amount", "summary_overpaid_amount", "summary_user_count",
		"summary_unpaid_count", "summary_partial_count", "summary_settled_count",
		"summary_overpaid_count", "summary_waived_count", "filtered_total",
		"user_id", "email", "username", "deleted", "usage_amount", "pricing_usage_amount", "multiplier",
		"receivable_amount", "paid_amount", "outstanding_amount", "overpaid_amount",
		"status", "payment_count", "last_paid_at",
	}
	rows := sqlmock.NewRows(columns).AddRow(
		810.0, 500.0, 300.25, 224.75, 25.0, int64(8),
		int64(2), int64(1), int64(2), int64(2), int64(1), int64(1),
		int64(7), "match@example.test", "Match", false, 600.01, 600.005, 0.5,
		300.0, 125.25, 174.75, 0.0, "partial", int64(1), period.Start.Add(24*time.Hour),
	)
	mock.ExpectQuery(`(?s)WITH live_usage_by_user AS.*\(\$7 = 0 OR user_id = \$7\)`).
		WithArgs(period.Start, period.End, "2026-07-01", true, "match", "partial", int64(7), 20, 0).
		WillReturnRows(rows)

	repo := NewMonthlyLedgerRepository(db)
	items, summary, page, err := repo.List(context.Background(), period, service.MonthlyLedgerListParams{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 20},
		Query:      "match",
		Status:     "partial",
		UserID:     7,
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, 600.005, items[0].PricingUsageAmount)
	require.Equal(t, int64(8), summary.UserCount)
	require.Equal(t, 500.0, summary.ReceivableAmount)
	require.Equal(t, 300.25, summary.PaidAmount)
	require.Equal(t, int64(1), page.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonthlyLedgerOrderByUsesAllowlist(t *testing.T) {
	require.Equal(t, "paid_amount ASC, user_id ASC", monthlyLedgerOrderBy("paid_amount", pagination.SortOrderAsc))
	require.Equal(t, "last_paid_at DESC NULLS LAST, user_id ASC", monthlyLedgerOrderBy("last_paid_at", pagination.SortOrderDesc))
	require.Equal(t, "outstanding_amount DESC, user_id ASC", monthlyLedgerOrderBy("paid_amount DESC; DROP TABLE users", "desc"))
}

func TestNormalizeMonthlyLedgerPagination(t *testing.T) {
	params := normalizeMonthlyLedgerPagination(pagination.PaginationParams{Page: 0, PageSize: 500, SortOrder: "invalid"})
	require.Equal(t, 1, params.Page)
	require.Equal(t, 200, params.PageSize)
	require.Equal(t, pagination.SortOrderDesc, params.SortOrder)
}

func TestMonthlyLedgerCreatePaymentWritesMonthStartDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	paidAt := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	createdAt := paidAt.Add(time.Minute)
	billingMonth := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("INSERT INTO monthly_ledger_payments").
		WithArgs(int64(7), "2026-07-01", 125.25, paidAt, "received", int64(99), createdAt).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(int64(42), createdAt, createdAt))

	payment := &service.MonthlyLedgerPayment{
		UserID:       7,
		BillingMonth: "2026-07",
		Amount:       125.25,
		PaidAt:       paidAt,
		Note:         "received",
		CreatedBy:    99,
		CreatedAt:    createdAt,
	}
	repo := NewMonthlyLedgerRepository(db)
	require.NoError(t, repo.CreatePayment(context.Background(), billingMonth, payment))
	require.Equal(t, int64(42), payment.ID)
	require.Equal(t, "2026-07", payment.BillingMonth)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonthlyLedgerSetMultipliersUpdatesAllTargetsAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	billingMonth := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO monthly_ledger_multipliers.*SELECT DISTINCT unnest.*RETURNING user_id`).
		WithArgs(sqlmock.AnyArg(), "2026-07-01", 0.5, int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(7)).AddRow(int64(11)))
	mock.ExpectCommit()

	repo := NewMonthlyLedgerRepository(db)
	updated, err := repo.SetMultipliers(context.Background(), billingMonth, []int64{7, 11}, 0.5, 99)
	require.NoError(t, err)
	require.Equal(t, int64(2), updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonthlyLedgerSetMultipliersRollsBackWhenTargetIsMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	billingMonth := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO monthly_ledger_multipliers.*RETURNING user_id`).
		WithArgs(sqlmock.AnyArg(), "2026-07-01", 0.5, int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(7)))
	mock.ExpectRollback()

	repo := NewMonthlyLedgerRepository(db)
	updated, err := repo.SetMultipliers(context.Background(), billingMonth, []int64{7, 404}, 0.5, 99)
	require.ErrorIs(t, err, service.ErrMonthlyLedgerUserNotFound)
	require.Zero(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}
