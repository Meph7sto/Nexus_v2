package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/lib/pq"
)

type monthlyLedgerSnapshotExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// snapshotMonthlyLedgerUsageForBatch freezes the complete user-month totals
// touched by one cleanup batch. Existing snapshots are immutable: they either
// came from the deployment backfill or an earlier cleanup of the same month.
func snapshotMonthlyLedgerUsageForBatch(
	ctx context.Context,
	exec monthlyLedgerSnapshotExecutor,
	whereClause string,
	args []any,
) error {
	limitPosition := len(args)
	timezonePosition := limitPosition + 1
	query := fmt.Sprintf(`
		WITH target AS (
			SELECT
				user_id,
				date_trunc('month', created_at AT TIME ZONE $%d)::date AS billing_month
			FROM usage_logs
			WHERE %s
			ORDER BY created_at ASC, id ASC
			LIMIT $%d
		),
		affected_user_months AS (
			SELECT DISTINCT user_id, billing_month
			FROM target
			WHERE billing_month < date_trunc('month', NOW() AT TIME ZONE $%d)::date
		)
		INSERT INTO monthly_ledger_usage_snapshots (user_id, billing_month, usage_amount)
		SELECT
			logs.user_id,
			affected.billing_month,
			COALESCE(SUM(logs.actual_cost), 0)::numeric
		FROM affected_user_months affected
		JOIN usage_logs logs
			ON logs.user_id = affected.user_id
			AND date_trunc('month', logs.created_at AT TIME ZONE $%d)::date = affected.billing_month
		JOIN users ON users.id = logs.user_id AND users.role = 'user'
		GROUP BY logs.user_id, affected.billing_month
		ON CONFLICT (user_id, billing_month) DO NOTHING
	`, timezonePosition, whereClause, limitPosition, timezonePosition, timezonePosition)
	_, err := exec.ExecContext(ctx, query, append(args, timezone.Name())...)
	return err
}

func snapshotMonthlyLedgerUsageForPartition(
	ctx context.Context,
	exec monthlyLedgerSnapshotExecutor,
	partitionName string,
) error {
	query := fmt.Sprintf(`
		WITH affected_user_months AS (
			SELECT DISTINCT
				user_id,
				date_trunc('month', created_at AT TIME ZONE $1)::date AS billing_month
			FROM %s
			WHERE date_trunc('month', created_at AT TIME ZONE $1)::date
				< date_trunc('month', NOW() AT TIME ZONE $1)::date
		)
		INSERT INTO monthly_ledger_usage_snapshots (user_id, billing_month, usage_amount)
		SELECT
			logs.user_id,
			affected.billing_month,
			COALESCE(SUM(logs.actual_cost), 0)::numeric
		FROM affected_user_months affected
		JOIN usage_logs logs
			ON logs.user_id = affected.user_id
			AND date_trunc('month', logs.created_at AT TIME ZONE $1)::date = affected.billing_month
		JOIN users ON users.id = logs.user_id AND users.role = 'user'
		GROUP BY logs.user_id, affected.billing_month
		ON CONFLICT (user_id, billing_month) DO NOTHING
	`, pq.QuoteIdentifier(partitionName))
	_, err := exec.ExecContext(ctx, query, timezone.Name())
	return err
}
