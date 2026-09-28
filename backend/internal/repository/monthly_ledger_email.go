package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

var _ service.MonthlyLedgerEmailRepository = (*monthlyLedgerRepository)(nil)

func (r *monthlyLedgerRepository) HasPendingEmails(ctx context.Context) (bool, error) {
	var pending bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
 SELECT 1 FROM monthly_ledger_email_deliveries d
 JOIN monthly_ledger_email_preferences p ON p.user_id = d.user_id AND p.enabled AND p.effective_month <= d.billing_month
 JOIN users u ON u.id = d.user_id AND u.role = 'user' AND u.deleted_at IS NULL
 WHERE d.sent_at IS NULL AND NOT d.skipped AND d.outstanding_amount > 0 AND u.email <> '')`).Scan(&pending)
	return pending, err
}

func (r *monthlyLedgerRepository) ListEmailPreferences(ctx context.Context, q string, page pagination.PaginationParams) ([]service.MonthlyLedgerEmailPreference, int64, error) {
	page = normalizeMonthlyLedgerPagination(page)
	const filter = ` FROM users u LEFT JOIN monthly_ledger_email_preferences p ON p.user_id = u.id
 WHERE u.role = 'user' AND u.deleted_at IS NULL
 AND ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.username ILIKE '%' || $1 || '%' OR u.id::text = $1)`
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+filter, q).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT u.id, u.email, COALESCE(u.username, ''), COALESCE(p.enabled, FALSE), COALESCE(to_char(p.effective_month, 'YYYY-MM'), '')`+filter+` ORDER BY u.id LIMIT $2 OFFSET $3`, q, page.Limit(), page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]service.MonthlyLedgerEmailPreference, 0)
	for rows.Next() {
		var p service.MonthlyLedgerEmailPreference
		if err := rows.Scan(&p.UserID, &p.Email, &p.Username, &p.Enabled, &p.EffectiveMonth); err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	return items, total, rows.Err()
}

func (r *monthlyLedgerRepository) SetEmailPreference(ctx context.Context, userID int64, enabled bool, month time.Time, actorID int64) error {
	result, err := r.db.ExecContext(ctx, `INSERT INTO monthly_ledger_email_preferences(user_id, enabled, effective_month, updated_by)
 SELECT id, $2, $3::date, $4 FROM users WHERE id = $1 AND role = 'user' AND deleted_at IS NULL
 ON CONFLICT (user_id) DO UPDATE SET enabled = EXCLUDED.enabled,
 effective_month = CASE WHEN EXCLUDED.enabled AND NOT monthly_ledger_email_preferences.enabled
 THEN EXCLUDED.effective_month ELSE monthly_ledger_email_preferences.effective_month END,
 updated_by = EXCLUDED.updated_by, updated_at = NOW()`, userID, enabled, month.Format("2006-01-02"), actorID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrMonthlyLedgerUserNotFound
	}
	return err
}

func (r *monthlyLedgerRepository) SetEmailPreferences(ctx context.Context, userIDs []int64, enabled bool, month time.Time, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO monthly_ledger_email_preferences(user_id, enabled, effective_month, updated_by)
 SELECT id, $2, $3::date, $4 FROM users WHERE id = ANY($1::bigint[]) AND role = 'user' AND deleted_at IS NULL ORDER BY id
 ON CONFLICT (user_id) DO UPDATE SET enabled = EXCLUDED.enabled,
 effective_month = CASE WHEN EXCLUDED.enabled AND NOT monthly_ledger_email_preferences.enabled
 THEN EXCLUDED.effective_month ELSE monthly_ledger_email_preferences.effective_month END,
 updated_by = EXCLUDED.updated_by, updated_at = NOW()`, pq.Array(userIDs), enabled, month.Format("2006-01-02"), actorID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if count != int64(len(userIDs)) {
		return 0, service.ErrMonthlyLedgerUserNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *monthlyLedgerRepository) PendingEmailMonths(ctx context.Context, currentMonth time.Time) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT to_char(m.month, 'YYYY-MM') AS month
 FROM monthly_ledger_email_preferences p
 JOIN users u ON u.id = p.user_id AND u.role = 'user' AND u.deleted_at IS NULL
 CROSS JOIN LATERAL generate_series(p.effective_month::timestamp, $1::date - INTERVAL '1 month', INTERVAL '1 month') m(month)
 WHERE p.enabled AND NOT EXISTS (SELECT 1 FROM monthly_ledger_email_deliveries d WHERE d.user_id = p.user_id AND d.billing_month = m.month::date)
 ORDER BY month`, currentMonth.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var months []string
	for rows.Next() {
		var month string
		if err := rows.Scan(&month); err != nil {
			return nil, err
		}
		months = append(months, month)
	}
	return months, rows.Err()
}

func (r *monthlyLedgerRepository) PrepareEmails(ctx context.Context, p service.MonthlyLedgerPeriod) error {
	if !p.CanRecordPayments {
		return service.ErrMonthlyLedgerOpenMonthPayment
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO monthly_ledger_usage_snapshots(user_id, billing_month, usage_amount)
 SELECT logs.user_id, $3::date, SUM(logs.actual_cost)
 FROM usage_logs logs JOIN monthly_ledger_email_preferences pref ON pref.user_id = logs.user_id
 JOIN users u ON u.id = logs.user_id AND u.role = 'user' AND u.deleted_at IS NULL
 WHERE logs.created_at >= $1 AND logs.created_at < $2 AND pref.enabled AND pref.effective_month <= $3::date
 GROUP BY logs.user_id ON CONFLICT(user_id, billing_month) DO NOTHING`, p.Start, p.End, p.Start.Format("2006-01-02"))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, monthlyLedgerBaseCTE+`
 INSERT INTO monthly_ledger_email_deliveries(user_id, billing_month, usage_amount, multiplier, receivable_amount, paid_amount, outstanding_amount, skipped)
 SELECT pref.user_id, $3::date, COALESCE(l.pricing_usage_amount, 0), COALESCE(l.multiplier, 1),
 COALESCE(l.receivable_amount, 0), COALESCE(l.paid_amount, 0), COALESCE(l.outstanding_amount, 0),
 COALESCE(l.outstanding_amount, 0) <= 0
 FROM monthly_ledger_email_preferences pref
 JOIN users u ON u.id = pref.user_id AND u.role = 'user' AND u.deleted_at IS NULL
 LEFT JOIN filtered l ON l.user_id = pref.user_id
 WHERE pref.enabled AND pref.effective_month <= $3::date
 ON CONFLICT (user_id, billing_month) DO NOTHING`, p.Start, p.End, p.Start.Format("2006-01-02"), true, "", "", int64(0))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Hold the row lock until SMTP and delivery bookkeeping finish so concurrent
// application instances cannot send the same monthly notice simultaneously.
func (r *monthlyLedgerRepository) DeliverNextEmail(ctx context.Context, send func(context.Context, service.MonthlyLedgerEmailDelivery) error) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var d service.MonthlyLedgerEmailDelivery
	err = tx.QueryRowContext(ctx, `SELECT d.user_id, to_char(d.billing_month, 'YYYY-MM'), u.email, COALESCE(u.username, ''),
 d.usage_amount, d.multiplier, d.receivable_amount, d.paid_amount, d.outstanding_amount
 FROM monthly_ledger_email_deliveries d
 JOIN monthly_ledger_email_preferences p ON p.user_id = d.user_id AND p.enabled AND p.effective_month <= d.billing_month
 JOIN users u ON u.id = d.user_id AND u.role = 'user' AND u.deleted_at IS NULL
 WHERE d.sent_at IS NULL AND NOT d.skipped AND d.next_attempt_at <= NOW() AND d.outstanding_amount > 0 AND u.email <> ''
 ORDER BY d.next_attempt_at, d.user_id LIMIT 1 FOR UPDATE OF d, p SKIP LOCKED`).Scan(
		&d.UserID, &d.Month, &d.Email, &d.Username, &d.UsageAmount, &d.Multiplier, &d.ReceivableAmount, &d.PaidAmount, &d.OutstandingAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	sendErr := send(sendCtx, d)
	cancel()
	if errors.Is(sendErr, service.ErrMonthlyLedgerEmailDailyLimit) {
		return false, nil
	}
	if sendErr != nil {
		slog.Warn("monthly ledger email failed; retry next day", "user_id", d.UserID, "month", d.Month, "error", sendErr)
	}
	now := timezone.Now()
	nextAttempt := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 15, 0, 0, now.Location())
	_, err = tx.ExecContext(ctx, `UPDATE monthly_ledger_email_deliveries SET attempts = attempts + 1,
 sent_at = CASE WHEN $3 THEN NOW() ELSE NULL END, next_attempt_at = $4
 WHERE user_id = $1 AND billing_month = $2::date`, d.UserID, d.Month+"-01", sendErr == nil, nextAttempt)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
