package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMonthlyLedgerManualEmailReservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &monthlyLedgerRepository{db: db}
	day := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("INSERT INTO monthly_ledger_email_daily_usage").WithArgs("2026-09-28").WillReturnRows(sqlmock.NewRows([]string{"attempts"}).AddRow(5))
	require.NoError(t, repo.ReserveEmailAttempt(context.Background(), day))
	mock.ExpectQuery("INSERT INTO monthly_ledger_email_daily_usage").WithArgs("2026-09-28").WillReturnError(sql.ErrNoRows)
	require.ErrorIs(t, repo.ReserveEmailAttempt(context.Background(), day), service.ErrMonthlyLedgerEmailDailyLimit)
	require.NoError(t, mock.ExpectationsWereMet())
}
