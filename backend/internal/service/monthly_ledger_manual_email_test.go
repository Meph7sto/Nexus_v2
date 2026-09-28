package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type manualLedgerEmailRepo struct {
	ledgerEmailRepoStub
	limit, used int
	day         time.Time
}

func (r *manualLedgerEmailRepo) EmailQuota(_ context.Context, day time.Time) (*MonthlyLedgerEmailQuota, error) {
	r.day = day
	return &MonthlyLedgerEmailQuota{Date: day.Format("2006-01-02"), DailyLimit: r.limit, Used: r.used}, nil
}
func (r *manualLedgerEmailRepo) ReserveEmailAttempt(_ context.Context, day time.Time) error {
	r.day = day
	if r.used >= r.limit {
		return ErrMonthlyLedgerEmailDailyLimit
	}
	r.used++
	return nil
}
func (r *manualLedgerEmailRepo) SetEmailDailyLimit(_ context.Context, limit int, actor int64) error {
	r.limit = limit
	r.actor = actor
	return nil
}

func manualLedgerService() (*MonthlyLedgerService, *manualLedgerEmailRepo) {
	repo := &manualLedgerEmailRepo{limit: 5}
	svc := NewMonthlyLedgerService(repo)
	svc.location = time.FixedZone("UTC+8", 8*3600)
	svc.now = func() time.Time { return time.Date(2026, 9, 28, 20, 0, 0, 0, time.UTC) }
	svc.emailSend = func(context.Context, NotificationEmailSendInput) error { return nil }
	return svc, repo
}
func manualLedgerInput() MonthlyLedgerManualEmailInput {
	return MonthlyLedgerManualEmailInput{Month: "2026-09", UserID: 7, Amount: 42.35, RequestID: "85a94e2e-e5cb-4085-9869-7c3b1b5cb381"}
}

func TestMonthlyLedgerManualEmailExplicitAmountAndAnyMonth(t *testing.T) {
	for _, month := range []string{"2020-01", "2026-09", "2030-12"} {
		svc, repo := manualLedgerService()
		input := manualLedgerInput()
		input.Month = month
		var sent NotificationEmailSendInput
		svc.emailSend = func(ctx context.Context, in NotificationEmailSendInput) error {
			_, ok := ctx.Deadline()
			require.True(t, ok)
			sent = in
			return nil
		}
		require.NoError(t, svc.SendManualEmail(context.Background(), input))
		require.Equal(t, NotificationEmailEventMonthlyLedgerManual, sent.Event)
		require.Equal(t, "user@example.test", sent.RecipientEmail)
		require.Equal(t, map[string]string{"billing_month": month, "currency": "USD", "outstanding_amount": "42.35"}, sent.Variables)
		require.Equal(t, 1, repo.used)
		require.Equal(t, "2026-09-29", repo.day.Format("2006-01-02"))
	}
}

func TestMonthlyLedgerManualEmailValidationDoesNotConsumeQuota(t *testing.T) {
	for _, mutate := range []func(*MonthlyLedgerManualEmailInput){
		func(i *MonthlyLedgerManualEmailInput) { i.Month = "" },
		func(i *MonthlyLedgerManualEmailInput) { i.Month = "2026-13" },
		func(i *MonthlyLedgerManualEmailInput) { i.Month = "0000-01" },
		func(i *MonthlyLedgerManualEmailInput) { i.UserID = 0 },
		func(i *MonthlyLedgerManualEmailInput) { i.Amount = 0 },
		func(i *MonthlyLedgerManualEmailInput) { i.Amount = -1 },
		func(i *MonthlyLedgerManualEmailInput) { i.Amount = 1.001 },
		func(i *MonthlyLedgerManualEmailInput) { i.Amount = math.Inf(1) },
		func(i *MonthlyLedgerManualEmailInput) { i.RequestID = "" },
	} {
		svc, repo := manualLedgerService()
		input := manualLedgerInput()
		mutate(&input)
		require.Error(t, svc.SendManualEmail(context.Background(), input))
		require.Zero(t, repo.used)
	}
}

func TestMonthlyLedgerManualAndAutomaticEmailsShareDailyLimit(t *testing.T) {
	svc, repo := manualLedgerService()
	svc.emailSettings = &notificationEmailMemorySettingRepo{values: map[string]string{SettingKeyMonthlyLedgerEmailEnabled: "true"}}
	repo.used = 3
	repo.delivery = &MonthlyLedgerEmailDelivery{Month: "2026-08"}
	sent := 0
	svc.emailSend = func(context.Context, NotificationEmailSendInput) error { sent++; return nil }
	require.NoError(t, svc.processEmailNotifications(context.Background(), svc.emailSend))
	require.NoError(t, svc.SendManualEmail(context.Background(), manualLedgerInput()))
	require.ErrorIs(t, svc.SendManualEmail(context.Background(), manualLedgerInput()), ErrMonthlyLedgerEmailDailyLimit)
	repo.delivery = &MonthlyLedgerEmailDelivery{Month: "2026-07"}
	require.NoError(t, svc.processEmailNotifications(context.Background(), svc.emailSend))
	require.NotNil(t, repo.delivery)
	require.Equal(t, 2, sent)
	require.Equal(t, 5, repo.used)
}

func TestMonthlyLedgerManualEmailFailureCountsAndLimitCanChange(t *testing.T) {
	svc, repo := manualLedgerService()
	svc.emailSend = func(context.Context, NotificationEmailSendInput) error { return errors.New("SMTP timeout") }
	require.ErrorContains(t, svc.SendManualEmail(context.Background(), manualLedgerInput()), "SMTP timeout")
	require.Equal(t, 1, repo.used)
	require.ErrorIs(t, svc.SetEmailDailyLimit(context.Background(), -1, 99), ErrMonthlyLedgerEmailInvalidLimit)
	require.NoError(t, svc.SetEmailDailyLimit(context.Background(), 0, 99))
	require.EqualValues(t, 99, repo.actor)
	require.ErrorIs(t, svc.SendManualEmail(context.Background(), manualLedgerInput()), ErrMonthlyLedgerEmailDailyLimit)
	require.NoError(t, svc.SetEmailDailyLimit(context.Background(), 10, 99))
	quota, err := svc.EmailQuota(context.Background())
	require.NoError(t, err)
	require.Equal(t, 10, quota.DailyLimit)
	require.Equal(t, 1, quota.Used)
}

func TestMonthlyLedgerManualEmailTemplates(t *testing.T) {
	svc := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)
	for _, locale := range []string{"zh", "en"} {
		tmpl, err := svc.GetTemplate(context.Background(), NotificationEmailEventMonthlyLedgerManual, locale)
		require.NoError(t, err)
		rendered, err := renderNotificationEmail(NotificationEmailEventMonthlyLedgerManual, tmpl.Subject, tmpl.HTML, map[string]string{
			"site_name": "Test", "recipient_name": "Customer", "billing_month": "2026-09", "outstanding_amount": "42.35", "currency": "USD",
		}, nil)
		require.NoError(t, err)
		require.Contains(t, rendered.HTML, "42.35 USD")
		require.Contains(t, rendered.Subject, "2026-09")
		require.NotContains(t, rendered.HTML, "{{")
		require.NotContains(t, rendered.HTML, "120.00")
	}
}
