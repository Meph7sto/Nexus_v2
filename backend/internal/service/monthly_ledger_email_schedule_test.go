package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMonthlyLedgerEmailSchedule(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	for _, tc := range []struct {
		name, now, want string
		pending         bool
	}{
		{"idle month", "2026-09-15 12:00", "2026-10-01 00:15", false},
		{"before month close window", "2026-10-01 00:05", "2026-10-01 00:15", false},
		{"finished monthly batch", "2026-10-01 00:15", "2026-11-01 00:15", false},
		{"quota exhausted", "2026-10-01 00:18", "2026-10-02 00:15", true},
		{"pending at startup before daily slot", "2026-10-02 00:05", "2026-10-02 00:15", true},
		{"year rollover", "2026-12-31 19:00", "2027-01-01 00:15", false},
		{"DST spring pending", "2026-03-08 00:15", "2026-03-09 00:15", true},
		{"DST fall pending", "2026-11-01 00:15", "2026-11-02 00:15", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.ParseInLocation("2006-01-02 15:04", tc.now, loc)
			require.NoError(t, err)
			want, err := time.ParseInLocation("2006-01-02 15:04", tc.want, loc)
			require.NoError(t, err)
			require.Equal(t, want, nextMonthlyLedgerEmailRun(now, tc.pending))
		})
	}
}

func TestMonthlyLedgerEmailPendingScheduleRespectsSwitch(t *testing.T) {
	repo := &ledgerEmailRepoStub{delivery: &MonthlyLedgerEmailDelivery{}}
	svc := NewMonthlyLedgerService(repo)
	settings := newNotificationEmailMemorySettingRepo()
	svc.emailSettings = settings
	pending, err := svc.hasPendingEmails(context.Background())
	require.NoError(t, err)
	require.False(t, pending)
	require.NoError(t, settings.Set(context.Background(), SettingKeyMonthlyLedgerEmailEnabled, "true"))
	pending, err = svc.hasPendingEmails(context.Background())
	require.NoError(t, err)
	require.True(t, pending)
	repo.delivery = nil
	pending, err = svc.hasPendingEmails(context.Background())
	require.NoError(t, err)
	require.False(t, pending)
}

func TestMonthlyLedgerEmailSchedulerStartupAndStop(t *testing.T) {
	svc := NewMonthlyLedgerService(&ledgerEmailRepoStub{})
	checked := make(chan struct{}, 1)
	svc.emailSettings = &ledgerEmailStartupSettings{checked: checked}
	// With notifications disabled the startup check cannot deliver any mail.
	svc.StartEmailNotifications(&NotificationEmailService{})
	select {
	case <-checked:
	case <-time.After(2 * time.Second):
		t.Fatal("startup did not check pending work")
	}
	done := make(chan struct{})
	go func() { svc.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not cancel its monthly timer")
	}
}

type ledgerEmailStartupSettings struct {
	SettingRepository
	checked chan struct{}
}

func (s *ledgerEmailStartupSettings) GetValue(context.Context, string) (string, error) {
	select {
	case s.checked <- struct{}{}:
	default:
	}
	return "false", nil
}
