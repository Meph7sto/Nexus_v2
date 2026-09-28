package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type ledgerEmailRepoStub struct {
	monthlyLedgerRepoStub
	months    []string
	prepared  []MonthlyLedgerPeriod
	delivery  *MonthlyLedgerEmailDelivery
	effective time.Time
	enabled   bool
	actor     int64
	batchIDs  []int64
}

func (r *ledgerEmailRepoStub) EmailQuota(context.Context, time.Time) (*MonthlyLedgerEmailQuota, error) {
	return &MonthlyLedgerEmailQuota{DailyLimit: 5}, nil
}
func (r *ledgerEmailRepoStub) SetEmailDailyLimit(context.Context, int, int64) error { return nil }
func (r *ledgerEmailRepoStub) ReserveEmailAttempt(context.Context, time.Time) error { return nil }
func (r *ledgerEmailRepoStub) EmailRecipient(context.Context, int64) (*MonthlyLedgerEmailPreference, error) {
	return &MonthlyLedgerEmailPreference{UserID: 7, Email: "user@example.test", Username: "User"}, nil
}

func (r *ledgerEmailRepoStub) ListEmailPreferences(context.Context, string, pagination.PaginationParams) ([]MonthlyLedgerEmailPreference, int64, error) {
	return nil, 0, nil
}
func (r *ledgerEmailRepoStub) SetEmailPreference(_ context.Context, _ int64, enabled bool, month time.Time, actor int64) error {
	r.effective, r.enabled, r.actor = month, enabled, actor
	return nil
}
func (r *ledgerEmailRepoStub) PendingEmailMonths(context.Context, time.Time) ([]string, error) {
	return r.months, nil
}
func (r *ledgerEmailRepoStub) HasPendingEmails(context.Context) (bool, error) {
	return r.delivery != nil, nil
}
func (r *ledgerEmailRepoStub) SetEmailPreferences(_ context.Context, ids []int64, enabled bool, month time.Time, actor int64) (int64, error) {
	r.batchIDs, r.enabled, r.effective, r.actor = ids, enabled, month, actor
	return int64(len(ids)), nil
}
func (r *ledgerEmailRepoStub) PrepareEmails(_ context.Context, p MonthlyLedgerPeriod) error {
	r.prepared = append(r.prepared, p)
	return nil
}
func (r *ledgerEmailRepoStub) DeliverNextEmail(ctx context.Context, send func(context.Context, MonthlyLedgerEmailDelivery) error) (bool, error) {
	if r.delivery == nil {
		return false, nil
	}
	d := *r.delivery
	r.delivery = nil
	return true, send(ctx, d)
}

func TestMonthlyLedgerEmailMonthBoundaryAndCatchup(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	repo := &ledgerEmailRepoStub{months: []string{"2026-07", "2026-08"}}
	svc := NewMonthlyLedgerService(repo)
	svc.emailSettings = &notificationEmailMemorySettingRepo{values: map[string]string{SettingKeyMonthlyLedgerEmailEnabled: "true"}}
	svc.location = loc
	now := time.Date(2026, 9, 1, 0, 5, 0, 0, loc)
	svc.now = func() time.Time { return now }
	send := func(context.Context, NotificationEmailSendInput) error { t.Fatal("no delivery expected"); return nil }
	require.NoError(t, svc.processEmailNotifications(context.Background(), send))
	require.Empty(t, repo.prepared)
	now = now.Add(15 * time.Minute)
	require.NoError(t, svc.processEmailNotifications(context.Background(), send))
	require.Len(t, repo.prepared, 2)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, loc), repo.prepared[1].Start)
	require.True(t, repo.prepared[1].CanRecordPayments)
}

func TestMonthlyLedgerEmailBatchPreferences(t *testing.T) {
	repo := &ledgerEmailRepoStub{}
	svc := NewMonthlyLedgerService(repo)
	svc.location = time.FixedZone("UTC+8", 8*3600)
	svc.now = func() time.Time { return time.Date(2026, 8, 31, 20, 0, 0, 0, time.UTC) }
	for _, enabled := range []bool{true, false} {
		count, err := svc.SetEmailPreferences(context.Background(), []int64{7, 8, 7}, enabled, 99)
		require.NoError(t, err)
		require.EqualValues(t, 2, count)
		require.Equal(t, []int64{7, 8}, repo.batchIDs)
		require.Equal(t, enabled, repo.enabled)
		require.EqualValues(t, 99, repo.actor)
		require.Equal(t, "2026-09-01", repo.effective.Format("2006-01-02"))
	}
	_, err := svc.SetEmailPreferences(context.Background(), nil, true, 99)
	require.ErrorIs(t, err, ErrMonthlyLedgerNoUsersSelected)
	_, err = svc.SetEmailPreferences(context.Background(), []int64{7, 0}, true, 99)
	require.ErrorIs(t, err, ErrMonthlyLedgerInvalidUserIDs)
}

func TestMonthlyLedgerEmailGlobalSwitchDefaultsOffAndPreservesPending(t *testing.T) {
	for _, value := range []string{"", "false", "invalid"} {
		t.Run(value, func(t *testing.T) {
			repo := &ledgerEmailRepoStub{months: []string{"2026-08"}, delivery: &MonthlyLedgerEmailDelivery{}, enabled: true}
			svc := NewMonthlyLedgerService(repo)
			settings := newNotificationEmailMemorySettingRepo()
			if value != "" {
				require.NoError(t, settings.Set(context.Background(), SettingKeyMonthlyLedgerEmailEnabled, value))
			}
			svc.emailSettings = settings
			require.NoError(t, svc.processEmailNotifications(context.Background(), func(context.Context, NotificationEmailSendInput) error {
				t.Fatal("mail while globally disabled")
				return nil
			}))
			require.Empty(t, repo.prepared)
			require.NotNil(t, repo.delivery)
			require.True(t, repo.enabled)
		})
	}
}

func TestMonthlyLedgerEmailGlobalSwitchStopsRunningBatchAndResumes(t *testing.T) {
	repo := &ledgerEmailRepoStub{delivery: &MonthlyLedgerEmailDelivery{Month: "2026-08"}}
	svc := NewMonthlyLedgerService(repo)
	svc.now = func() time.Time { return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC) }
	settings := newNotificationEmailMemorySettingRepo()
	svc.emailSettings = settings
	ctx := context.Background()
	require.NoError(t, settings.Set(ctx, SettingKeyMonthlyLedgerEmailEnabled, "true"))
	sent := 0
	require.NoError(t, svc.processEmailNotifications(ctx, func(context.Context, NotificationEmailSendInput) error {
		sent++
		repo.delivery = &MonthlyLedgerEmailDelivery{Month: "2026-08"}
		return settings.Set(ctx, SettingKeyMonthlyLedgerEmailEnabled, "false")
	}))
	require.Equal(t, 1, sent)
	require.NotNil(t, repo.delivery)
	require.NoError(t, settings.Set(ctx, SettingKeyMonthlyLedgerEmailEnabled, "true"))
	require.NoError(t, svc.processEmailNotifications(ctx, func(context.Context, NotificationEmailSendInput) error { sent++; return nil }))
	require.Equal(t, 2, sent)
	require.Nil(t, repo.delivery)
}

type ledgerEmailBrokenSettings struct{ SettingRepository }

func (ledgerEmailBrokenSettings) GetValue(context.Context, string) (string, error) {
	return "", errors.New("settings unavailable")
}

func TestMonthlyLedgerEmailGlobalSwitchReadFailurePreventsSending(t *testing.T) {
	svc := NewMonthlyLedgerService(&ledgerEmailRepoStub{delivery: &MonthlyLedgerEmailDelivery{}})
	svc.emailSettings = ledgerEmailBrokenSettings{}
	err := svc.processEmailNotifications(context.Background(), func(context.Context, NotificationEmailSendInput) error { t.Fatal("unexpected email"); return nil })
	require.ErrorContains(t, err, "settings unavailable")
}

func TestMonthlyLedgerEmailSettingParsesDefaultAndExplicitValues(t *testing.T) {
	svc := &SettingService{cfg: &config.Config{}}
	require.False(t, svc.parseSettings(map[string]string{}).MonthlyLedgerEmailEnabled)
	for _, value := range []string{"true", "false"} {
		settings := svc.parseSettings(map[string]string{SettingKeyMonthlyLedgerEmailEnabled: value})
		require.Equal(t, value == "true", settings.MonthlyLedgerEmailEnabled)
		updates, err := svc.buildSystemSettingsUpdates(context.Background(), settings)
		require.NoError(t, err)
		require.Equal(t, value, updates[SettingKeyMonthlyLedgerEmailEnabled])
	}
}

func TestMonthlyLedgerEmailUsesFrozenAmountsAndStableIdentity(t *testing.T) {
	repo := &ledgerEmailRepoStub{delivery: &MonthlyLedgerEmailDelivery{Month: "2026-08", MonthlyLedgerRow: MonthlyLedgerRow{
		UserID: 7, Email: "ledger@example.test", Username: "Customer", UsageAmount: 120.01, Multiplier: 0.5, ReceivableAmount: 60.01, PaidAmount: 10, OutstandingAmount: 50.01,
	}}}
	svc := NewMonthlyLedgerService(repo)
	svc.emailSettings = &notificationEmailMemorySettingRepo{values: map[string]string{SettingKeyMonthlyLedgerEmailEnabled: "true"}}
	svc.now = func() time.Time { return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC) }
	var input NotificationEmailSendInput
	require.NoError(t, svc.processEmailNotifications(context.Background(), func(_ context.Context, in NotificationEmailSendInput) error { input = in; return nil }))
	require.Equal(t, "billing.monthly_ledger", input.Event)
	require.Equal(t, "2026-08:7", input.SourceID)
	require.Equal(t, "50.01", input.Variables["outstanding_amount"])
	require.Equal(t, "60.01", input.Variables["receivable_amount"])
	require.Equal(t, "10.00", input.Variables["paid_amount"])
	require.Equal(t, "USD", input.Variables["currency"])
}

func TestMonthlyLedgerEmailPreferenceUsesCurrentLocalMonth(t *testing.T) {
	repo := &ledgerEmailRepoStub{}
	svc := NewMonthlyLedgerService(repo)
	svc.location = time.FixedZone("UTC+8", 8*3600)
	svc.now = func() time.Time { return time.Date(2026, 8, 31, 20, 0, 0, 0, time.UTC) }
	require.NoError(t, svc.SetEmailPreference(context.Background(), 7, true, 99))
	require.Equal(t, "2026-09-01", repo.effective.Format("2006-01-02"))
	require.True(t, repo.enabled)
	require.EqualValues(t, 99, repo.actor)
	require.ErrorIs(t, svc.SetEmailPreference(context.Background(), 0, true, 99), ErrMonthlyLedgerUserNotFound)
}

func TestMonthlyLedgerEmailTemplatesRenderBothLocales(t *testing.T) {
	svc := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)
	for _, locale := range []string{"en", "zh"} {
		tmpl, err := svc.GetTemplate(context.Background(), NotificationEmailEventMonthlyLedger, locale)
		require.NoError(t, err)
		result, err := svc.PreviewTemplate(context.Background(), NotificationEmailPreviewInput{Event: tmpl.Event, Locale: locale, Subject: tmpl.Subject, HTML: tmpl.HTML})
		require.NoError(t, err)
		require.Contains(t, result.HTML, "50.00 USD")
		require.Contains(t, result.Subject, "2026-08")
		require.NotContains(t, result.HTML, "{{")
	}
}
