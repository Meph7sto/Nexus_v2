//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMonthlyLedgerRepositoryAggregatesAndFiltersLiveMonthlyLedger(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewMonthlyLedgerRepository(integrationDB)
	suffix := uuid.NewString()

	actor := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("ledger-actor-%s@example.test", suffix),
		Role:  service.RoleAdmin,
	})
	account := mustCreateAccount(t, client, &service.Account{Name: "ledger-account-" + suffix})

	newUser := func(label string, role string) (*service.User, *service.APIKey) {
		user := mustCreateUser(t, client, &service.User{
			Email:    fmt.Sprintf("ledger-%s-%s@example.test", label, suffix),
			Username: "Ledger " + label,
			Role:     role,
		})
		key := mustCreateApiKey(t, client, &service.APIKey{
			UserID: user.ID,
			Key:    "sk-ledger-" + label + "-" + uuid.NewString(),
			Name:   "ledger-" + label,
		})
		return user, key
	}

	partialUser, partialKey := newUser("partial", service.RoleUser)
	settledUser, settledKey := newUser("settled", service.RoleUser)
	overpaidUser, overpaidKey := newUser("overpaid", service.RoleUser)
	waivedUser, waivedKey := newUser("waived", service.RoleUser)
	unpaidUser, unpaidKey := newUser("unpaid", service.RoleUser)
	zeroUser, zeroKey := newUser("zero", service.RoleUser)
	deletedUser, deletedKey := newUser("deleted", service.RoleUser)
	paymentOnlyUser, _ := newUser("payment-only", service.RoleUser)
	adminUser, adminKey := newUser("excluded-admin", service.RoleAdmin)

	users := []*service.User{
		partialUser, settledUser, overpaidUser, waivedUser, unpaidUser,
		zeroUser, deletedUser, paymentOnlyUser, adminUser, actor,
	}
	t.Cleanup(func() {
		for _, user := range users {
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM monthly_ledger_payments WHERE user_id = $1", user.ID)
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM monthly_ledger_multipliers WHERE user_id = $1", user.ID)
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM monthly_ledger_usage_snapshots WHERE user_id = $1", user.ID)
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM usage_logs WHERE user_id = $1", user.ID)
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE user_id = $1", user.ID)
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		}
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
	})

	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	period := service.MonthlyLedgerPeriod{
		Month:             "2026-07",
		Start:             time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		End:               time.Date(2026, 8, 1, 0, 0, 0, 0, loc),
		CanRecordPayments: true,
	}
	usageTime := period.Start.Add(12 * time.Hour)
	insertUsage := func(user *service.User, key *service.APIKey, amount float64, createdAt time.Time) {
		_, err := integrationDB.ExecContext(ctx, `
			INSERT INTO usage_logs (user_id, api_key_id, account_id, model, actual_cost, created_at)
			VALUES ($1, $2, $3, 'ledger-test', $4, $5)`,
			user.ID, key.ID, account.ID, amount, createdAt,
		)
		require.NoError(t, err)
	}
	insertUsage(partialUser, partialKey, 600, period.Start)
	insertUsage(partialUser, partialKey, 999, period.Start.Add(-time.Microsecond))
	insertUsage(partialUser, partialKey, 999, period.End)
	insertUsage(settledUser, settledKey, 100, usageTime)
	insertUsage(overpaidUser, overpaidKey, 50, usageTime)
	insertUsage(waivedUser, waivedKey, 10, usageTime)
	insertUsage(unpaidUser, unpaidKey, 20, usageTime)
	insertUsage(zeroUser, zeroKey, 0, usageTime)
	insertUsage(deletedUser, deletedKey, 30, usageTime)
	insertUsage(adminUser, adminKey, 999, usageTime)

	_, err = integrationDB.ExecContext(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", deletedUser.ID)
	require.NoError(t, err)

	previous, err := repo.SetMultiplier(ctx, period.Start, partialUser.ID, 0.5, actor.ID)
	require.NoError(t, err)
	require.Equal(t, 1.0, previous)
	_, err = repo.SetMultiplier(ctx, period.Start, waivedUser.ID, 0, actor.ID)
	require.NoError(t, err)

	createPayment := func(user *service.User, amount float64) *service.MonthlyLedgerPayment {
		payment := &service.MonthlyLedgerPayment{
			UserID: user.ID, BillingMonth: period.Month, Amount: amount,
			PaidAt: period.End.Add(24 * time.Hour), CreatedBy: actor.ID,
			CreatedAt: period.End.Add(24 * time.Hour),
		}
		require.NoError(t, repo.CreatePayment(ctx, period.Start, payment))
		return payment
	}
	partialPayment := createPayment(partialUser, 125.25)
	require.Equal(t, "2026-07", partialPayment.BillingMonth)
	var storedBillingMonth string
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT billing_month::text FROM monthly_ledger_payments WHERE id = $1",
		partialPayment.ID,
	).Scan(&storedBillingMonth))
	require.Equal(t, "2026-07-01", storedBillingMonth)
	createPayment(settledUser, 100)
	createPayment(overpaidUser, 70)
	createPayment(paymentOnlyUser, 5)

	items, summary, pageResult, err := repo.List(ctx, period, service.MonthlyLedgerListParams{
		Pagination: pagination.PaginationParams{
			Page: 1, PageSize: 3, SortBy: "receivable_amount", SortOrder: pagination.SortOrderDesc,
		},
	})
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, partialUser.ID, items[0].UserID)
	require.Equal(t, int64(8), pageResult.Total)
	require.Equal(t, 3, pageResult.Pages)
	require.InDelta(t, 810, summary.UsageAmount, 0.000001)
	require.InDelta(t, 500, summary.ReceivableAmount, 0.000001)
	require.InDelta(t, 300.25, summary.PaidAmount, 0.000001)
	require.InDelta(t, 224.75, summary.OutstandingAmount, 0.000001)
	require.InDelta(t, 25, summary.OverpaidAmount, 0.000001)
	require.Equal(t, int64(8), summary.UserCount)
	require.Equal(t, int64(2), summary.UnpaidCount)
	require.Equal(t, int64(1), summary.PartialCount)
	require.Equal(t, int64(2), summary.SettledCount)
	require.Equal(t, int64(2), summary.OverpaidCount)
	require.Equal(t, int64(1), summary.WaivedCount)

	filtered, filteredSummary, filteredPage, err := repo.List(ctx, period, service.MonthlyLedgerListParams{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 20},
		Query:      partialUser.Email,
		Status:     service.MonthlyLedgerStatusPartial,
	})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, partialUser.ID, filtered[0].UserID)
	require.Equal(t, int64(1), filteredPage.Total)
	require.Equal(t, int64(8), filteredSummary.UserCount, "summary must cover the full month")

	deletedRows, _, deletedPage, err := repo.List(ctx, period, service.MonthlyLedgerListParams{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 20},
		Query:      deletedUser.Email,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), deletedPage.Total)
	require.True(t, deletedRows[0].Deleted)

	insertUsage(partialUser, partialKey, 100, usageTime)
	filtered, summary, _, err = repo.List(ctx, period, service.MonthlyLedgerListParams{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 20},
		Query:      partialUser.Email,
	})
	require.NoError(t, err)
	require.InDelta(t, 700, filtered[0].UsageAmount, 0.000001)
	require.InDelta(t, 350, filtered[0].ReceivableAmount, 0.000001)
	require.InDelta(t, 550, summary.ReceivableAmount, 0.000001)

	previous, err = repo.SetMultiplier(ctx, period.Start, partialUser.ID, 0.8, actor.ID)
	require.NoError(t, err)
	require.Equal(t, 0.5, previous)
	filtered, summary, _, err = repo.List(ctx, period, service.MonthlyLedgerListParams{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 20},
		Query:      partialUser.Email,
	})
	require.NoError(t, err)
	require.InDelta(t, 560, filtered[0].ReceivableAmount, 0.000001)
	require.InDelta(t, 760, summary.ReceivableAmount, 0.000001)

	payments, err := repo.ListPayments(ctx, period.Start, partialUser.ID)
	require.NoError(t, err)
	require.Len(t, payments, 1)
	require.Equal(t, "2026-07", payments[0].BillingMonth)
	before, after, err := repo.UpdatePayment(ctx, partialPayment.ID, service.MonthlyLedgerPaymentUpdate{
		Amount: 130, PaidAt: partialPayment.PaidAt.Add(time.Hour), Note: "updated",
	}, actor.ID)
	require.NoError(t, err)
	require.InDelta(t, 125.25, before.Amount, 0.000001)
	require.InDelta(t, 130, after.Amount, 0.000001)
	require.Equal(t, "2026-07", before.BillingMonth)
	require.Equal(t, "2026-07", after.BillingMonth)
	deletedPayment, err := repo.DeletePayment(ctx, partialPayment.ID)
	require.NoError(t, err)
	require.InDelta(t, 130, deletedPayment.Amount, 0.000001)
	require.Equal(t, "2026-07", deletedPayment.BillingMonth)
}

func TestMonthlyLedgerRepositoryManualSettlementLifecycle(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewMonthlyLedgerRepository(integrationDB)
	suffix := uuid.NewString()
	user := mustCreateUser(t, client, &service.User{Email: "settlement-" + suffix + "@example.test", Role: service.RoleUser})
	other := mustCreateUser(t, client, &service.User{Email: "settlement-other-" + suffix + "@example.test", Role: service.RoleUser})
	account := mustCreateAccount(t, client, &service.Account{Name: "settlement-" + suffix})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-settlement-" + suffix, Name: "settlement"})
	t.Cleanup(func() {
		for _, id := range []int64{user.ID, other.ID} {
			for _, table := range []string{"monthly_ledger_settlements", "monthly_ledger_payments", "monthly_ledger_multipliers", "usage_logs", "api_keys", "users"} {
				column := "user_id"
				if table == "users" {
					column = "id"
				}
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM "+table+" WHERE "+column+" = $1", id)
			}
		}
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
	})
	month := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	period := service.MonthlyLedgerPeriod{Month: "2026-07", Start: month, End: month.AddDate(0, 1, 0), CanRecordPayments: true}
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_logs (user_id, api_key_id, account_id, model, actual_cost, created_at) VALUES ($1, $2, $3, 'settlement-test', 1001, $4)`, user.ID, key.ID, account.ID, month.Add(time.Hour))
	require.NoError(t, err)
	params := service.MonthlyLedgerListParams{UserID: user.ID, Pagination: pagination.PaginationParams{Page: 1, PageSize: 20}}
	read := func() (service.MonthlyLedgerRow, *service.MonthlyLedgerSummary) {
		items, summary, _, err := repo.List(ctx, period, params)
		require.NoError(t, err)
		require.Len(t, items, 1)
		return items[0], summary
	}
	for _, amount := range []float64{1000, 1001, 1002} {
		payment := &service.MonthlyLedgerPayment{UserID: user.ID, BillingMonth: period.Month, Amount: amount, PaidAt: period.End, CreatedAt: period.End, CreatedBy: user.ID}
		require.NoError(t, repo.CreatePayment(ctx, month, payment))
		before, summaryBefore := read()
		require.False(t, before.ManuallySettled)
		_, err := repo.SetSettlement(ctx, month, user.ID, true, user.ID)
		require.NoError(t, err)
		after, summaryAfter := read()
		require.True(t, after.ManuallySettled)
		require.Equal(t, "settled", after.Status)
		require.Equal(t, 1001.0, after.ReceivableAmount)
		require.Equal(t, amount, after.PaidAmount)
		require.Zero(t, after.OutstandingAmount)
		require.Zero(t, after.OverpaidAmount)
		require.Equal(t, summaryBefore.OutstandingAmount-before.OutstandingAmount, summaryAfter.OutstandingAmount)
		require.Equal(t, summaryBefore.OverpaidAmount-before.OverpaidAmount, summaryAfter.OverpaidAmount)
		expectedSettled := summaryBefore.SettledCount
		if before.Status != "settled" {
			expectedSettled++
		}
		require.Equal(t, expectedSettled, summaryAfter.SettledCount)
		params.Status = "settled"
		read()
		params.Status = "partial"
		items, _, page, err := repo.List(ctx, period, params)
		require.NoError(t, err)
		require.Empty(t, items)
		require.Zero(t, page.Total)
		params.Status = ""
		_, err = repo.SetSettlement(ctx, month, user.ID, false, user.ID)
		require.NoError(t, err)
		restored, _ := read()
		require.Equal(t, before, restored)
		_, err = repo.DeletePayment(ctx, payment.ID)
		require.NoError(t, err)
	}
	_, err = repo.SetSettlement(ctx, month, user.ID, true, user.ID)
	require.NoError(t, err)
	payment := &service.MonthlyLedgerPayment{UserID: user.ID, Amount: 1000, PaidAt: period.End, CreatedAt: period.End}
	require.NoError(t, repo.CreatePayment(ctx, month, payment))
	_, _, err = repo.UpdatePayment(ctx, payment.ID, service.MonthlyLedgerPaymentUpdate{Amount: 1002, PaidAt: period.End}, user.ID)
	require.NoError(t, err)
	_, err = repo.SetMultiplier(ctx, month, user.ID, 2, user.ID)
	require.NoError(t, err)
	row, _ := read()
	require.True(t, row.ManuallySettled)
	require.Equal(t, 2002.0, row.ReceivableAmount)
	_, err = repo.DeletePayment(ctx, payment.ID)
	require.NoError(t, err)
	row, _ = read()
	require.True(t, row.ManuallySettled)
	require.Equal(t, "settled", row.Status)
	require.Zero(t, row.PaidAmount)
	previous, err := repo.SetSettlement(ctx, month.AddDate(0, 1, 0), user.ID, false, user.ID)
	require.NoError(t, err)
	require.False(t, previous)
	previous, err = repo.SetSettlement(ctx, month, other.ID, false, user.ID)
	require.NoError(t, err)
	require.False(t, previous)
	row, _ = read()
	require.True(t, row.ManuallySettled)
}

func TestMonthlyLedgerRepositoryBatchMultiplierIsAtomicAndMonthScoped(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewMonthlyLedgerRepository(integrationDB)
	suffix := uuid.NewString()
	actor := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("ledger-batch-actor-%s@example.test", suffix),
		Role:  service.RoleAdmin,
	})
	first := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("ledger-batch-first-%s@example.test", suffix),
		Role:  service.RoleUser,
	})
	second := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("ledger-batch-second-%s@example.test", suffix),
		Role:  service.RoleUser,
	})
	t.Cleanup(func() {
		for _, user := range []*service.User{first, second, actor} {
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM monthly_ledger_multipliers WHERE user_id = $1", user.ID)
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		}
	})

	june := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	_, err := repo.SetMultiplier(ctx, june, first.ID, 0.9, actor.ID)
	require.NoError(t, err)

	updated, err := repo.SetMultipliers(ctx, july, []int64{first.ID, second.ID}, 0.5, actor.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), updated)
	for _, user := range []*service.User{first, second} {
		var multiplier float64
		var updatedBy int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `
			SELECT multiplier, updated_by FROM monthly_ledger_multipliers
			WHERE user_id = $1 AND billing_month = $2`, user.ID, july).Scan(&multiplier, &updatedBy))
		require.Equal(t, 0.5, multiplier)
		require.Equal(t, actor.ID, updatedBy)
	}
	var juneMultiplier float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT multiplier FROM monthly_ledger_multipliers
		WHERE user_id = $1 AND billing_month = $2`, first.ID, june).Scan(&juneMultiplier))
	require.Equal(t, 0.9, juneMultiplier)

	_, err = repo.SetMultipliers(ctx, july, []int64{first.ID, actor.ID}, 0.8, actor.ID)
	require.ErrorIs(t, err, service.ErrMonthlyLedgerUserNotFound)
	var unchanged float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT multiplier FROM monthly_ledger_multipliers
		WHERE user_id = $1 AND billing_month = $2`, first.ID, july).Scan(&unchanged))
	require.Equal(t, 0.5, unchanged)
}

func TestMonthlyLedgerRepositoryKeepsCompletedMonthAfterManualUsageCleanup(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	ledgerRepo := NewMonthlyLedgerRepository(integrationDB)
	cleanupRepo := NewUsageCleanupRepository(client, integrationDB)
	suffix := uuid.NewString()

	user := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("ledger-snapshot-%s@example.test", suffix),
		Role:  service.RoleUser,
	})
	key := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-ledger-snapshot-" + suffix,
		Name:   "ledger-snapshot",
	})
	account := mustCreateAccount(t, client, &service.Account{Name: "ledger-snapshot-" + suffix})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM monthly_ledger_usage_snapshots WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM usage_logs WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
	})

	period := service.MonthlyLedgerPeriod{
		Month:             "2026-07",
		Start:             time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		End:               time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		CanRecordPayments: true,
	}
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO usage_logs (user_id, api_key_id, account_id, model, actual_cost, created_at)
		VALUES ($1, $2, $3, 'ledger-snapshot-test', 1.005, $4),
		       ($1, $2, $3, 'ledger-snapshot-test', 2.000, $5)`,
		user.ID, key.ID, account.ID, period.Start.Add(time.Hour), period.Start.Add(2*time.Hour),
	)
	require.NoError(t, err)

	list := func() ([]service.MonthlyLedgerRow, *service.MonthlyLedgerSummary) {
		items, summary, _, listErr := ledgerRepo.List(ctx, period, service.MonthlyLedgerListParams{
			Pagination: pagination.PaginationParams{Page: 1, PageSize: 20},
			Query:      user.Email,
		})
		require.NoError(t, listErr)
		require.Len(t, items, 1)
		return items, summary
	}
	beforeItems, beforeSummary := list()

	deleted, err := cleanupRepo.DeleteUsageLogsBatch(ctx, service.UsageCleanupFilters{
		StartTime: period.Start,
		EndTime:   period.End.Add(-time.Nanosecond),
		UserID:    &user.ID,
	}, 100)
	require.NoError(t, err)
	require.Equal(t, int64(2), deleted)

	afterItems, afterSummary := list()
	require.Equal(t, beforeItems, afterItems)
	require.Equal(t, beforeSummary, afterSummary)
	require.InDelta(t, 3.005, afterItems[0].PricingUsageAmount, 0.000000001)
	require.InDelta(t, 3.01, afterItems[0].UsageAmount, 0.000000001)
}
