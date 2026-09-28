//go:build ledger_postgres

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestMonthlyLedgerManualEmailPostgresQuota(t *testing.T) {
	dsn := os.Getenv("LEDGER_TEST_DSN")
	if dsn == "" {
		t.Skip("LEDGER_TEST_DSN is not set")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer admin.Close()
	schema := fmt.Sprintf("ledger_manual_email_test_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	defer admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(10)
	migration, err := os.ReadFile("../../migrations/243_monthly_ledger_manual_email.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	repo := &monthlyLedgerRepository{db: db}
	ctx := context.Background()
	day := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	quota, err := repo.EmailQuota(ctx, day)
	require.NoError(t, err)
	require.Equal(t, 5, quota.DailyLimit)
	require.Zero(t, quota.Used)

	results := make(chan error, 30)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- repo.ReserveEmailAttempt(ctx, day) }()
	}
	wg.Wait()
	close(results)
	sent, limited := 0, 0
	for err := range results {
		switch {
		case err == nil:
			sent++
		case errors.Is(err, service.ErrMonthlyLedgerEmailDailyLimit):
			limited++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 5, sent)
	require.Equal(t, 25, limited)
	restarted := &monthlyLedgerRepository{db: db}
	quota, err = restarted.EmailQuota(ctx, day)
	require.NoError(t, err)
	require.Equal(t, 5, quota.Used)
	tomorrow := day.AddDate(0, 0, 1)
	quota, err = repo.EmailQuota(ctx, tomorrow)
	require.NoError(t, err)
	require.Zero(t, quota.Used)
	require.NoError(t, repo.ReserveEmailAttempt(ctx, tomorrow))
	require.NoError(t, repo.SetEmailDailyLimit(ctx, 0, 99))
	require.ErrorIs(t, repo.ReserveEmailAttempt(ctx, tomorrow), service.ErrMonthlyLedgerEmailDailyLimit)
	require.NoError(t, repo.SetEmailDailyLimit(ctx, 6, 99))
	require.NoError(t, repo.ReserveEmailAttempt(ctx, day))
	require.ErrorIs(t, repo.ReserveEmailAttempt(ctx, day), service.ErrMonthlyLedgerEmailDailyLimit)

	_, err = db.Exec(`CREATE TABLE users(id BIGINT, email TEXT, username TEXT, role TEXT, deleted_at TIMESTAMPTZ);
 INSERT INTO users VALUES (1,'user@example.test','User','user',NULL),(2,'admin@example.test','Admin','admin',NULL),(3,'deleted@example.test','','user',NOW()),(4,'   ','','user',NULL);`)
	require.NoError(t, err)
	recipient, err := repo.EmailRecipient(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "user@example.test", recipient.Email)
	for _, id := range []int64{2, 3, 4, 999} {
		_, err := repo.EmailRecipient(ctx, id)
		require.ErrorIs(t, err, service.ErrMonthlyLedgerUserNotFound)
	}
}
