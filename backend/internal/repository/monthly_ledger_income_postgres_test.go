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
	"sync"
	"testing"
	"time"
)

func TestIncomePostgres(t *testing.T) {
	dsn := os.Getenv("LEDGER_TEST_DSN")
	if dsn == "" {
		t.Skip("LEDGER_TEST_DSN is not set")
	}
	admin, e := sql.Open("postgres", dsn)
	require.NoError(t, e)
	defer admin.Close()
	schema := fmt.Sprintf("income_test_%d", time.Now().UnixNano())
	_, e = admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, e)
	defer admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
	db, e := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, e)
	defer db.Close()
	raw, e := os.ReadFile("../../migrations/242_monthly_ledger_income.sql")
	require.NoError(t, e)
	_, e = db.Exec(string(raw))
	require.NoError(t, e)
	repo := &monthlyLedgerRepository{db: db}
	ctx := context.Background()
	_, e = db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,email TEXT,username TEXT,role TEXT,deleted_at TIMESTAMPTZ);
	CREATE TABLE usage_logs(user_id BIGINT,actual_cost NUMERIC(30,10),created_at TIMESTAMPTZ);`)
	require.NoError(t, e)
	for _, name := range []string{"188_monthly_ledger.sql", "229_monthly_ledger_usage_snapshots.sql", "235_monthly_ledger_settlements.sql"} {
		data, err := os.ReadFile("../../migrations/" + name)
		require.NoError(t, err)
		_, err = db.Exec(string(data))
		require.NoError(t, err)
	}
	_, e = db.Exec(`INSERT INTO users VALUES(1,'relay@example.test','Relay','user',NULL);
	INSERT INTO usage_logs VALUES(1,30,'2026-01-15T12:00:00Z');`)
	require.NoError(t, e)
	fields := service.IncomeFields{Title: "Top-up", Customer: "External", Category: "Service", Amount: 100, Cost: 120}
	entry, e := repo.SaveIncomeEntry(ctx, 0, service.IncomeEntryInput{IncomeFields: fields, BillingDate: "2026-01-15", DueDate: "2026-01-16"}, 99)
	require.NoError(t, e)
	require.Equal(t, -20.0, entry.Profit)
	list := func(month string) *service.IncomeList {
		out, e := repo.ListIncome(ctx, service.IncomeListParams{Month: month, Page: 1, PageSize: 20})
		require.NoError(t, e)
		return out
	}
	paidAt := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	payment, e := repo.SaveIncomePayment(ctx, 0, entry.ID, service.MonthlyLedgerPaymentInput{Amount: 40, PaidAt: paidAt}, 99)
	require.NoError(t, e)
	require.Equal(t, "partial", list("2026-01").Items[0].Status)
	require.Equal(t, 40.0, list("2026-01").Summary.PaidAmount)
	secondPayment, e := repo.SaveIncomePayment(ctx, 0, entry.ID, service.MonthlyLedgerPaymentInput{Amount: 60, PaidAt: paidAt}, 99)
	require.NoError(t, e)
	require.Equal(t, "settled", list("2026-01").Items[0].Status)
	require.NoError(t, repo.DeleteIncomePayment(ctx, secondPayment.ID))
	overview, e := service.NewMonthlyLedgerService(repo).IncomeOverview(ctx, "2026-01")
	require.NoError(t, e)
	require.Equal(t, 130.0, overview.ReceivableAmount)
	require.Equal(t, 90.0, overview.OutstandingAmount)
	require.Equal(t, -20.0, overview.Profit)
	require.Zero(t, list("2026-02").Total)
	require.ErrorIs(t, repo.DeleteIncomeEntry(ctx, entry.ID, 99), service.ErrIncomeHasPayments)
	_, e = repo.SaveIncomePayment(ctx, payment.ID, 0, service.MonthlyLedgerPaymentInput{Amount: 110, PaidAt: paidAt}, 99)
	require.NoError(t, e)
	require.Equal(t, "overpaid", list("2026-01").Items[0].Status)
	require.Equal(t, 10.0, list("2026-01").Summary.OverpaidAmount)
	second, e := repo.SaveIncomeEntry(ctx, 0, service.IncomeEntryInput{IncomeFields: fields, BillingDate: "2026-01-20", DueDate: "2026-01-20"}, 99)
	require.NoError(t, e)
	filtered, e := repo.ListIncome(ctx, service.IncomeListParams{Month: "2026-01", Status: "unpaid", Page: 1, PageSize: 1})
	require.NoError(t, e)
	require.EqualValues(t, 1, filtered.Total)
	require.Equal(t, 100.0, filtered.Summary.OutstandingAmount)
	require.Equal(t, 10.0, filtered.Summary.OverpaidAmount)
	updated := second.IncomeEntryInput
	updated.Amount = 50
	updated.Cost = 0
	_, e = repo.SaveIncomeEntry(ctx, second.ID, updated, 98)
	require.NoError(t, e)
	require.Equal(t, 50.0, list("2026-01").Summary.OutstandingAmount)
	require.NoError(t, repo.DeleteIncomePayment(ctx, payment.ID))
	require.NoError(t, repo.DeleteIncomeEntry(ctx, entry.ID, 99))
	require.NoError(t, repo.DeleteIncomeEntry(ctx, second.ID, 99))
	require.Zero(t, list("2026-01").Total)

	at := func(date string) time.Time {
		v, e := service.IncomeDate(date, time.UTC)
		require.NoError(t, e)
		return v
	}
	input := service.IncomeScheduleInput{IncomeFields: fields, FirstDate: "2026-01-31", IntervalMonths: 1}
	schedule, e := repo.SaveIncomeSchedule(ctx, 0, input, 99, at("2026-03-31"))
	require.NoError(t, e)
	require.Equal(t, 3, schedule.NextIndex)
	require.Equal(t, "2026-02-28", list("2026-02").Items[0].BillingDate)
	// Concurrent workers and a replay after deleting an occurrence cannot duplicate it.
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- repo.ProcessIncomeSchedules(ctx, at("2026-04-30")) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	april := list("2026-04")
	require.EqualValues(t, 1, april.Total)
	require.NoError(t, repo.DeleteIncomeEntry(ctx, april.Items[0].ID, 99))
	require.NoError(t, repo.ProcessIncomeSchedules(ctx, at("2026-04-30")))
	require.Zero(t, list("2026-04").Total)
	// Pause first catches up May; resume skips June and July, retaining the anchor.
	require.NoError(t, repo.SetIncomeScheduleState(ctx, schedule.ID, "paused", 99, at("2026-05-31")))
	require.EqualValues(t, 1, list("2026-05").Total)
	require.NoError(t, repo.ProcessIncomeSchedules(ctx, at("2026-06-30")))
	require.Zero(t, list("2026-06").Total)
	require.NoError(t, repo.SetIncomeScheduleState(ctx, schedule.ID, "active", 99, at("2026-07-31")))
	require.Zero(t, list("2026-07").Total)
	input.Amount = 200
	end := "2026-08-31"
	input.EndDate = &end
	_, e = repo.SaveIncomeSchedule(ctx, schedule.ID, input, 98, at("2026-08-01"))
	require.NoError(t, e)
	require.NoError(t, repo.ProcessIncomeSchedules(ctx, at("2026-10-31")))
	require.Equal(t, 200.0, list("2026-08").Items[0].Amount)
	require.Equal(t, 100.0, list("2026-03").Items[0].Amount)
	require.Zero(t, list("2026-09").Total)
	schedules, e := repo.ListIncomeSchedules(ctx)
	require.NoError(t, e)
	require.Equal(t, "ended", schedules[0].State)
	require.ErrorIs(t, repo.SetIncomeScheduleState(ctx, schedule.ID, "active", 99, at("2026-11-01")), service.ErrIncomeScheduleEnded)
	// Database constraints reject negative amounts even if service validation is bypassed.
	_, e = db.Exec(`UPDATE monthly_ledger_income_entries SET amount=-1 WHERE id=$1`, entry.ID)
	require.Error(t, e)
	_, e = db.Exec(`UPDATE monthly_ledger_income_entries SET cost=-1 WHERE id=$1`, entry.ID)
	require.Error(t, e)
	// A failed catch-up must not leave either a template or a partial first bill.
	_, e = db.Exec(`CREATE FUNCTION fail_income_test() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN IF NEW.customer='Rollback' AND NEW.billing_date='2026-02-28' THEN RAISE EXCEPTION 'test failure'; END IF; RETURN NEW; END $$;
	CREATE TRIGGER fail_income_test BEFORE INSERT ON monthly_ledger_income_entries FOR EACH ROW EXECUTE FUNCTION fail_income_test();`)
	require.NoError(t, e)
	rollbackInput := input
	rollbackInput.Customer = "Rollback"
	_, e = repo.SaveIncomeSchedule(ctx, 0, rollbackInput, 99, at("2026-03-31"))
	require.Error(t, e)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM monthly_ledger_income_entries WHERE customer='Rollback'`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM monthly_ledger_income_schedules WHERE customer='Rollback'`).Scan(&count))
	require.Zero(t, count)
}
