package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.MonthlyLedgerManualEmailRepository = (*monthlyLedgerRepository)(nil)

func (r *monthlyLedgerRepository) EmailQuota(ctx context.Context, day time.Time) (*service.MonthlyLedgerEmailQuota, error) {
	q := &service.MonthlyLedgerEmailQuota{Date: day.Format("2006-01-02")}
	err := r.db.QueryRowContext(ctx, `SELECT l.daily_limit, COALESCE(u.attempts, 0)
 FROM monthly_ledger_email_limits l LEFT JOIN monthly_ledger_email_daily_usage u ON u.send_date = $1::date
 WHERE l.id = TRUE`, q.Date).Scan(&q.DailyLimit, &q.Used)
	return q, err
}

func (r *monthlyLedgerRepository) SetEmailDailyLimit(ctx context.Context, limit int, actorID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE monthly_ledger_email_limits SET daily_limit = $1, updated_by = $2, updated_at = NOW() WHERE id = TRUE`, limit, actorID)
	return err
}

func (r *monthlyLedgerRepository) ReserveEmailAttempt(ctx context.Context, day time.Time) error {
	var used int
	err := r.db.QueryRowContext(ctx, `INSERT INTO monthly_ledger_email_daily_usage(send_date, attempts)
 SELECT $1::date, 1 FROM monthly_ledger_email_limits WHERE id = TRUE AND daily_limit > 0
 ON CONFLICT (send_date) DO UPDATE SET attempts = monthly_ledger_email_daily_usage.attempts + 1
 WHERE monthly_ledger_email_daily_usage.attempts < (SELECT daily_limit FROM monthly_ledger_email_limits WHERE id = TRUE)
 RETURNING attempts`, day.Format("2006-01-02")).Scan(&used)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrMonthlyLedgerEmailDailyLimit
	}
	return err
}

func (r *monthlyLedgerRepository) EmailRecipient(ctx context.Context, userID int64) (*service.MonthlyLedgerEmailPreference, error) {
	recipient := &service.MonthlyLedgerEmailPreference{}
	err := r.db.QueryRowContext(ctx, `SELECT id, email, COALESCE(username, '') FROM users
 WHERE id = $1 AND role = 'user' AND deleted_at IS NULL AND TRIM(email) <> ''`, userID).Scan(&recipient.UserID, &recipient.Email, &recipient.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMonthlyLedgerUserNotFound
	}
	return recipient, err
}
