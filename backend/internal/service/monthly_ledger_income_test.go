package service

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
)

func TestIncomeOccurrenceAnchors(t *testing.T) {
	for _, tc := range []struct {
		first         string
		months, index int
		want          string
	}{
		{"2026-01-31", 1, 1, "2026-02-28"}, {"2026-01-31", 1, 2, "2026-03-31"},
		{"2024-01-31", 1, 1, "2024-02-29"}, {"2024-02-29", 12, 1, "2025-02-28"},
		{"2024-02-29", 12, 4, "2028-02-29"}, {"2026-01-31", 3, 1, "2026-04-30"},
	} {
		first, e := IncomeDate(tc.first, time.UTC)
		require.NoError(t, e)
		require.Equal(t, tc.want, IncomeOccurrence(first, tc.months, tc.index).Format("2006-01-02"))
	}
}
func TestIncomeMoneyValidation(t *testing.T) {
	for _, v := range []float64{-1, 0.001, 1.999, math.NaN(), math.Inf(1), 1e15} {
		require.False(t, validIncomeMoney(v, false))
	}
	require.False(t, validIncomeMoney(0, true))
	require.True(t, validIncomeMoney(0, false))
	require.True(t, validIncomeMoney(100.01, true))
}
func TestIncomePreviewTimezoneAndInclusiveEnd(t *testing.T) {
	loc := time.FixedZone("UTC-5", -5*3600)
	s := NewMonthlyLedgerService(nil)
	s.location = loc
	s.now = func() time.Time { return time.Date(2026, 3, 1, 4, 59, 0, 0, time.UTC) }
	input := IncomeScheduleInput{IncomeFields: IncomeFields{Title: "Top-up", Customer: "Outside customer", Category: "Service", Amount: 100, Cost: 120}, FirstDate: "2026-01-31", IntervalMonths: 1}
	n, e := s.PreviewIncomeSchedule(input)
	require.NoError(t, e)
	require.Equal(t, 2, n)
	end := "2026-01-31"
	input.EndDate = &end
	n, e = s.PreviewIncomeSchedule(input)
	require.NoError(t, e)
	require.Equal(t, 1, n)
	input.IntervalMonths = 0
	_, e = s.PreviewIncomeSchedule(input)
	require.ErrorIs(t, e, ErrIncomeInvalid)
	input.IntervalMonths = 1
	input.FirstDate = "2026-02-30"
	_, e = s.PreviewIncomeSchedule(input)
	require.ErrorIs(t, e, ErrIncomeInvalid)
}
