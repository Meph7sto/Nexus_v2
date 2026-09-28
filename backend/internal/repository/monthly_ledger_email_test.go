package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMonthlyLedgerEmailDeliveryCommitAndRetry(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "retry"}[success], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &monthlyLedgerRepository{db: db}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT d.user_id,.*FOR UPDATE OF d, p SKIP LOCKED").WillReturnRows(sqlmock.NewRows([]string{"id", "month", "email", "username", "usage", "multiplier", "receivable", "paid", "outstanding"}).AddRow(7, "2026-08", "user@example.test", "User", 120, .5, 60, 10, 50))
			mock.ExpectExec("UPDATE monthly_ledger_email_deliveries SET attempts").WithArgs(int64(7), "2026-08-01", success).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			processed, err := repo.DeliverNextEmail(context.Background(), func(ctx context.Context, d service.MonthlyLedgerEmailDelivery) error {
				_, hasDeadline := ctx.Deadline()
				require.True(t, hasDeadline)
				require.Equal(t, 50.0, d.OutstandingAmount)
				if !success {
					return errors.New("SMTP unavailable")
				}
				return nil
			})
			require.NoError(t, err)
			require.True(t, processed)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMonthlyLedgerEmailNoPendingDeliveryDoesNotSend(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT d.user_id").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	processed, err := (&monthlyLedgerRepository{db: db}).DeliverNextEmail(context.Background(), func(context.Context, service.MonthlyLedgerEmailDelivery) error {
		t.Fatal("unexpected email")
		return nil
	})
	require.NoError(t, err)
	require.False(t, processed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonthlyLedgerEmailQuotaRacePreservesPendingDelivery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT d.user_id").WillReturnRows(sqlmock.NewRows([]string{"id", "month", "email", "username", "usage", "multiplier", "receivable", "paid", "outstanding"}).AddRow(7, "2026-08", "user@example.test", "User", 120, .5, 60, 10, 50))
	mock.ExpectRollback()
	processed, err := (&monthlyLedgerRepository{db: db}).DeliverNextEmail(context.Background(), func(context.Context, service.MonthlyLedgerEmailDelivery) error {
		return service.ErrMonthlyLedgerEmailDailyLimit
	})
	require.NoError(t, err)
	require.False(t, processed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonthlyLedgerEmailPreferenceRejectsMissingUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("INSERT INTO monthly_ledger_email_preferences").WithArgs(int64(7), false, "2026-09-01", int64(99)).WillReturnResult(sqlmock.NewResult(0, 0))
	err = (&monthlyLedgerRepository{db: db}).SetEmailPreference(context.Background(), 7, false, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), 99)
	require.ErrorIs(t, err, service.ErrMonthlyLedgerUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
