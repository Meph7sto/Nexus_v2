//go:build ledger_postgres

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestMonthlyLedgerEmailHistoryPostgres(t *testing.T) {
	dsn := os.Getenv("LEDGER_TEST_DSN")
	if dsn == "" {
		t.Skip("LEDGER_TEST_DSN is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	schema := fmt.Sprintf("ledger_history_test_%d", time.Now().UnixNano())
	_, err = db.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	defer db.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
	conn, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	defer conn.Close()
	exec := func(q string) { t.Helper(); _, err := conn.Exec(q); require.NoError(t, err) }
	exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,email TEXT,username TEXT,role TEXT,deleted_at TIMESTAMPTZ);
 CREATE TABLE usage_logs(user_id BIGINT,actual_cost NUMERIC,created_at TIMESTAMPTZ);
 INSERT INTO users VALUES(7,'test@example.test','Test','user',NULL),(8,'other@example.test','Other','user',NULL);`)
	for _, name := range []string{"188_monthly_ledger.sql", "229_monthly_ledger_usage_snapshots.sql", "235_monthly_ledger_settlements.sql", "241_monthly_ledger_email_notifications.sql"} {
		raw, err := os.ReadFile("../../migrations/" + name)
		require.NoError(t, err)
		exec(string(raw))
	}
	exec(`INSERT INTO monthly_ledger_email_deliveries(user_id,billing_month,usage_amount,multiplier,receivable_amount,paid_amount,outstanding_amount,sent_at)
 VALUES(7,'2026-08-01',100,1,100,0,100,'2026-09-01T00:00:00Z');
 INSERT INTO usage_logs VALUES(7,100,'2026-08-15T00:00:00Z');`)
	raw, err := os.ReadFile("../../migrations/244_monthly_ledger_email_history.sql")
	require.NoError(t, err)
	exec(string(raw))
	repo := &monthlyLedgerRepository{db: conn}
	ctx := context.Background()
	history, err := repo.ListEmailHistory(ctx, "2026-08", 7, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, history.SentCount)
	require.Empty(t, history.Items[0].HTML)
	record := service.MonthlyLedgerEmailRecord{UserID: 7, Month: "2026-08", SourceType: "monthly_ledger_manual", Recipient: "test@example.test", Amount: 42.35, Subject: "Saved bill", HTML: "<p>$42.35</p>"}
	sentID, err := repo.CreateEmailHistory(ctx, record)
	require.NoError(t, err)
	require.NoError(t, repo.FinishEmailHistory(ctx, sentID, true))
	failedID, err := repo.CreateEmailHistory(ctx, record)
	require.NoError(t, err)
	require.NoError(t, repo.FinishEmailHistory(ctx, failedID, false))
	history, err = repo.ListEmailHistory(ctx, "2026-08", 7, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 3, history.Total)
	require.EqualValues(t, 2, history.SentCount)
	require.Len(t, history.Items, 1)
	require.Equal(t, "failed", history.Items[0].Status)
	history, err = repo.ListEmailHistory(ctx, "2026-08", 7, 2, 1)
	require.NoError(t, err)
	require.Equal(t, record.HTML, history.Items[0].HTML)
	require.NotNil(t, history.Items[0].SentAt)
	empty, err := repo.ListEmailHistory(ctx, "2026-08", 8, 1, 20)
	require.NoError(t, err)
	require.Empty(t, empty.Items)
	empty, err = repo.ListEmailHistory(ctx, "2026-07", 7, 1, 20)
	require.NoError(t, err)
	require.Empty(t, empty.Items)
	period := service.MonthlyLedgerPeriod{Start: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), CanRecordPayments: true}
	rows, _, _, err := repo.List(ctx, period, service.MonthlyLedgerListParams{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 2, rows[0].EmailCount)
	require.Equal(t, 42.35, rows[0].LastEmailAmount)
}
