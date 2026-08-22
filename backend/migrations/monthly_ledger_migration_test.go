package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMonthlyLedgerMigrationDefinesFinancialConstraints(t *testing.T) {
	raw, err := FS.ReadFile("188_monthly_ledger.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	for _, fragment := range []string{
		"create table if not exists monthly_ledger_multipliers",
		"unique (user_id, billing_month)",
		"multiplier >= 0",
		"create table if not exists monthly_ledger_payments",
		"amount > 0",
		"date_trunc('month', billing_month)::date = billing_month",
		"idx_monthly_ledger_payments_month_user",
	} {
		require.Contains(t, sql, fragment)
	}
}

func TestMonthlyLedgerUsageSnapshotMigrationBackfillsCompletedMonths(t *testing.T) {
	raw, err := FS.ReadFile("229_monthly_ledger_usage_snapshots.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	for _, fragment := range []string{
		"create table if not exists monthly_ledger_usage_snapshots",
		"unique (user_id, billing_month)",
		"sum(actual_cost)",
		"date_trunc('month', usage_logs.created_at)",
		"date_trunc('month', now())",
		"on conflict (user_id, billing_month) do nothing",
	} {
		require.Contains(t, sql, fragment)
	}
	require.Equal(t, 3, strings.Count(sql, "date_trunc('month', usage_logs.created_at)"))
}
