package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type monthlyLedgerRepoStub struct {
	listPeriod          MonthlyLedgerPeriod
	multiplier          float64
	paymentBillingMonth time.Time
	payment             *MonthlyLedgerPayment
}

func (s *monthlyLedgerRepoStub) List(_ context.Context, period MonthlyLedgerPeriod, _ MonthlyLedgerListParams) ([]MonthlyLedgerRow, *MonthlyLedgerSummary, *pagination.PaginationResult, error) {
	s.listPeriod = period
	return []MonthlyLedgerRow{}, &MonthlyLedgerSummary{}, &pagination.PaginationResult{}, nil
}

func (s *monthlyLedgerRepoStub) ListPayments(context.Context, time.Time, int64) ([]MonthlyLedgerPayment, error) {
	return []MonthlyLedgerPayment{}, nil
}

func (s *monthlyLedgerRepoStub) SetMultiplier(_ context.Context, _ time.Time, _ int64, multiplier float64, _ int64) (float64, error) {
	previous := s.multiplier
	s.multiplier = multiplier
	return previous, nil
}

func (s *monthlyLedgerRepoStub) CreatePayment(_ context.Context, billingMonth time.Time, payment *MonthlyLedgerPayment) error {
	s.paymentBillingMonth = billingMonth
	copy := *payment
	copy.ID = 17
	s.payment = &copy
	payment.ID = copy.ID
	return nil
}

func (s *monthlyLedgerRepoStub) UpdatePayment(context.Context, int64, MonthlyLedgerPaymentUpdate, int64) (*MonthlyLedgerPayment, *MonthlyLedgerPayment, error) {
	return nil, nil, nil
}

func (s *monthlyLedgerRepoStub) DeletePayment(context.Context, int64) (*MonthlyLedgerPayment, error) {
	return nil, nil
}

func TestResolveMonthlyLedgerPeriod(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, loc)

	period, err := ResolveMonthlyLedgerPeriod("", now, loc)
	require.NoError(t, err)
	require.Equal(t, "2026-07", period.Month)
	require.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, loc), period.Start)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, loc), period.End)
	require.True(t, period.CanRecordPayments)

	period, err = ResolveMonthlyLedgerPeriod("2026-08", now, loc)
	require.NoError(t, err)
	require.False(t, period.CanRecordPayments)

	_, err = ResolveMonthlyLedgerPeriod("2026-09", now, loc)
	require.ErrorIs(t, err, ErrMonthlyLedgerFutureMonth)

	for _, value := range []string{"2026-8", "2026-08-01", "not-a-month"} {
		_, err = ResolveMonthlyLedgerPeriod(value, now, loc)
		require.ErrorIs(t, err, ErrMonthlyLedgerInvalidMonth, value)
	}
}

func TestCalculateMonthlyLedgerAmounts(t *testing.T) {
	amounts := CalculateMonthlyLedgerAmounts(600, 0.5, 0)
	require.Equal(t, 300.0, amounts.ReceivableAmount)
	require.Equal(t, 300.0, amounts.OutstandingAmount)
	require.Equal(t, MonthlyLedgerStatusUnpaid, amounts.Status)

	amounts = CalculateMonthlyLedgerAmounts(600, 0.5, 125.25)
	require.Equal(t, 174.75, amounts.OutstandingAmount)
	require.Equal(t, MonthlyLedgerStatusPartial, amounts.Status)

	amounts = CalculateMonthlyLedgerAmounts(600, 0.5, 300)
	require.Equal(t, MonthlyLedgerStatusSettled, amounts.Status)

	amounts = CalculateMonthlyLedgerAmounts(600, 0.5, 310)
	require.Equal(t, 10.0, amounts.OverpaidAmount)
	require.Equal(t, MonthlyLedgerStatusOverpaid, amounts.Status)

	amounts = CalculateMonthlyLedgerAmounts(600, 0, 0)
	require.Equal(t, MonthlyLedgerStatusWaived, amounts.Status)

	amounts = CalculateMonthlyLedgerAmounts(1.005, 1, 0)
	require.Equal(t, 1.01, amounts.ReceivableAmount)
}

func TestMonthlyLedgerServiceCreatePaymentValidation(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, loc)
	repo := &monthlyLedgerRepoStub{}
	svc := NewMonthlyLedgerService(repo)
	svc.now = func() time.Time { return now }
	svc.location = loc

	_, err = svc.CreatePayment(context.Background(), "2026-08", 7, MonthlyLedgerPaymentInput{Amount: 1, PaidAt: now}, 99)
	require.ErrorIs(t, err, ErrMonthlyLedgerOpenMonthPayment)

	_, err = svc.CreatePayment(context.Background(), "2026-07", 7, MonthlyLedgerPaymentInput{Amount: 0, PaidAt: now}, 99)
	require.ErrorIs(t, err, ErrMonthlyLedgerInvalidAmount)

	_, err = svc.CreatePayment(context.Background(), "2026-07", 7, MonthlyLedgerPaymentInput{Amount: 1, PaidAt: now.Add(time.Minute)}, 99)
	require.ErrorIs(t, err, ErrMonthlyLedgerFuturePaymentTime)

	_, err = svc.CreatePayment(context.Background(), "2026-07", 7, MonthlyLedgerPaymentInput{Amount: 1, PaidAt: now, Note: string(make([]byte, 501))}, 99)
	require.ErrorIs(t, err, ErrMonthlyLedgerNoteTooLong)

	payment, err := svc.CreatePayment(context.Background(), "2026-07", 7, MonthlyLedgerPaymentInput{Amount: 125.25, PaidAt: now, Note: "first payment"}, 99)
	require.NoError(t, err)
	require.Equal(t, int64(17), payment.ID)
	require.Equal(t, "2026-07", payment.BillingMonth)
	require.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, loc), repo.paymentBillingMonth)
	require.Equal(t, int64(99), payment.CreatedBy)
}

func TestMonthlyLedgerServiceSetMultiplier(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, loc)
	repo := &monthlyLedgerRepoStub{multiplier: 1}
	svc := NewMonthlyLedgerService(repo)
	svc.now = func() time.Time { return now }
	svc.location = loc

	_, err = svc.SetMultiplier(context.Background(), "2026-07", 7, -0.1, 99)
	require.ErrorIs(t, err, ErrMonthlyLedgerInvalidMultiplier)

	change, err := svc.SetMultiplier(context.Background(), "2026-08", 7, 0.5, 99)
	require.NoError(t, err)
	require.Equal(t, 1.0, change.PreviousMultiplier)
	require.Equal(t, 0.5, change.Multiplier)
}
