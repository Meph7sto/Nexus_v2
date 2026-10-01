//go:build ledger_postgres

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Run against an isolated local PostgreSQL with LEDGER_TEST_DSN and
// go test -tags ledger_postgres ./internal/repository -run MonthlyLedgerEmailPostgres.
func TestMonthlyLedgerEmailPostgres(t *testing.T) {
	dsn := os.Getenv("LEDGER_TEST_DSN")
	if dsn == "" {
		t.Skip("LEDGER_TEST_DSN is not set")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer admin.Close()
	schema := fmt.Sprintf("ledger_email_test_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	defer admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	defer db.Close()
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		require.NoError(t, err)
	}
	exec(`CREATE TABLE users(id BIGINT PRIMARY KEY, email TEXT, username TEXT, role TEXT, deleted_at TIMESTAMPTZ);
 CREATE TABLE usage_logs(user_id BIGINT, actual_cost NUMERIC(30,10), created_at TIMESTAMPTZ);`)
	for _, name := range []string{"188_monthly_ledger.sql", "229_monthly_ledger_usage_snapshots.sql", "235_monthly_ledger_settlements.sql", "241_monthly_ledger_email_notifications.sql", "244_monthly_ledger_email_history.sql"} {
		data, err := os.ReadFile("../../migrations/" + name)
		require.NoError(t, err)
		exec(string(data))
	}
	exec(`INSERT INTO users(id, email, username, role) SELECT n, 'user' || n || '@example.test', 'User ' || n, 'user' FROM generate_series(1,6) n;
 INSERT INTO usage_logs VALUES (1,120.01,'2026-08-15T00:00:00Z'),(2,100,'2026-08-15T00:00:00Z'),(4,100,'2026-08-15T00:00:00Z'),(5,100,'2026-08-15T00:00:00Z');
 INSERT INTO monthly_ledger_multipliers(user_id,billing_month,multiplier) VALUES(1,'2026-08-01',0.5);
 INSERT INTO monthly_ledger_payments(user_id,billing_month,amount,paid_at) VALUES(1,'2026-08-01',10,'2026-09-01T00:00:00Z');
 INSERT INTO monthly_ledger_settlements(user_id,billing_month,manually_settled) VALUES(4,'2026-08-01',true);`)
	repo := &monthlyLedgerRepository{db: db}
	ctx := context.Background()
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	items, total, err := repo.ListEmailPreferences(ctx, "", pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 6, total)
	for _, item := range items {
		require.False(t, item.Enabled)
	}
	for _, id := range []int64{1, 3, 4, 5, 6} {
		require.NoError(t, repo.SetEmailPreference(ctx, id, true, month, 99))
	}
	exec(`UPDATE users SET deleted_at=NOW() WHERE id=5`)
	// Re-saving an enabled preference must not move its effective month forward.
	require.NoError(t, repo.SetEmailPreference(ctx, 1, true, month.AddDate(0, 1, 0), 99))
	months, err := repo.PendingEmailMonths(ctx, month.AddDate(0, 1, 0))
	require.NoError(t, err)
	require.Equal(t, []string{"2026-08"}, months)
	p := service.MonthlyLedgerPeriod{Month: "2026-08", Start: month, End: month.AddDate(0, 1, 0), CanRecordPayments: true}
	require.NoError(t, repo.PrepareEmails(ctx, p))
	months, err = repo.PendingEmailMonths(ctx, p.End)
	require.NoError(t, err)
	require.Empty(t, months)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM monthly_ledger_email_deliveries`).Scan(&count))
	require.Equal(t, 4, count) // Disabled and deleted users are excluded; zero/settled months are completed without mail.
	pending, err := repo.HasPendingEmails(ctx)
	require.NoError(t, err)
	require.True(t, pending)
	exec(`UPDATE monthly_ledger_multipliers SET multiplier=2 WHERE user_id=1`)
	require.NoError(t, repo.PrepareEmails(ctx, p))
	processed, err := repo.DeliverNextEmail(ctx, func(ctx context.Context, d service.MonthlyLedgerEmailDelivery) error {
		require.EqualValues(t, 1, d.UserID)
		require.Equal(t, 50.01, d.OutstandingAmount)
		require.Equal(t, 0.5, d.Multiplier)
		other, err := repo.DeliverNextEmail(ctx, func(context.Context, service.MonthlyLedgerEmailDelivery) error {
			t.Fatal("concurrent duplicate delivery")
			return nil
		})
		require.NoError(t, err)
		require.False(t, other)
		return errors.New("test SMTP failure")
	})
	require.NoError(t, err)
	require.True(t, processed)
	var nextAttempt time.Time
	require.NoError(t, db.QueryRow(`SELECT next_attempt_at FROM monthly_ledger_email_deliveries WHERE user_id=1`).Scan(&nextAttempt))
	now := timezone.Now()
	require.True(t, nextAttempt.Equal(time.Date(now.Year(), now.Month(), now.Day()+1, 0, 15, 0, 0, now.Location())))
	processed, err = repo.DeliverNextEmail(ctx, func(context.Context, service.MonthlyLedgerEmailDelivery) error {
		t.Fatal("retry must wait")
		return nil
	})
	require.NoError(t, err)
	require.False(t, processed)
	exec(`UPDATE monthly_ledger_email_deliveries SET next_attempt_at=NOW() WHERE user_id=1`)
	require.NoError(t, repo.SetEmailPreference(ctx, 1, false, month, 99))
	pending, err = repo.HasPendingEmails(ctx)
	require.NoError(t, err)
	require.False(t, pending)
	processed, err = repo.DeliverNextEmail(ctx, func(context.Context, service.MonthlyLedgerEmailDelivery) error {
		t.Fatal("disabled recipient")
		return nil
	})
	require.NoError(t, err)
	require.False(t, processed)
	require.NoError(t, repo.SetEmailPreference(ctx, 1, true, month, 99))
	processed, err = repo.DeliverNextEmail(ctx, func(context.Context, service.MonthlyLedgerEmailDelivery) error { return nil })
	require.NoError(t, err)
	require.True(t, processed)
	processed, err = repo.DeliverNextEmail(ctx, func(context.Context, service.MonthlyLedgerEmailDelivery) error {
		t.Fatal("already delivered")
		return nil
	})
	require.NoError(t, err)
	require.False(t, processed)
	require.NoError(t, repo.SetEmailPreference(ctx, 1, false, month, 99))
	pending, err = repo.HasPendingEmails(ctx)
	require.NoError(t, err)
	require.False(t, pending)
	require.NoError(t, repo.SetEmailPreference(ctx, 1, true, month.AddDate(0, 1, 0), 99))
	var effective string
	require.NoError(t, db.QueryRow(`SELECT to_char(effective_month,'YYYY-MM') FROM monthly_ledger_email_preferences WHERE user_id=1`).Scan(&effective))
	require.Equal(t, "2026-09", effective)
	// Batch writes keep existing opt-in dates, and never partially apply missing users.
	countUpdated, err := repo.SetEmailPreferences(ctx, []int64{1, 2}, true, month.AddDate(0, 2, 0), 99)
	require.NoError(t, err)
	require.EqualValues(t, 2, countUpdated)
	require.NoError(t, db.QueryRow(`SELECT to_char(effective_month,'YYYY-MM') FROM monthly_ledger_email_preferences WHERE user_id=1`).Scan(&effective))
	require.Equal(t, "2026-09", effective)
	require.NoError(t, db.QueryRow(`SELECT to_char(effective_month,'YYYY-MM') FROM monthly_ledger_email_preferences WHERE user_id=2`).Scan(&effective))
	require.Equal(t, "2026-10", effective)
	for _, missing := range []int64{5, 999} {
		_, err = repo.SetEmailPreferences(ctx, []int64{1, missing}, false, month, 99)
		require.ErrorIs(t, err, service.ErrMonthlyLedgerUserNotFound)
		var enabled bool
		require.NoError(t, db.QueryRow(`SELECT enabled FROM monthly_ledger_email_preferences WHERE user_id=1`).Scan(&enabled))
		require.True(t, enabled)
	}
	_, err = repo.SetEmailPreferences(ctx, []int64{1, 2}, false, month.AddDate(0, 2, 0), 99)
	require.NoError(t, err)
	_, err = repo.SetEmailPreferences(ctx, []int64{1, 2}, true, month.AddDate(0, 3, 0), 99)
	require.NoError(t, err)
	for _, id := range []int64{1, 2} {
		require.NoError(t, db.QueryRow(`SELECT to_char(effective_month,'YYYY-MM') FROM monthly_ledger_email_preferences WHERE user_id=$1`, id).Scan(&effective))
		require.Equal(t, "2026-11", effective)
	}
}
