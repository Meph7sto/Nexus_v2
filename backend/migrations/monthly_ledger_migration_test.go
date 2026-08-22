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
