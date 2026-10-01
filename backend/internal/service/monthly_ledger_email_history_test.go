package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type ledgerHistoryRepo struct {
	manualLedgerEmailRepo
	records    []MonthlyLedgerEmailRecord
	failCreate bool
}

func (r *ledgerHistoryRepo) CreateEmailHistory(_ context.Context, record MonthlyLedgerEmailRecord) (int64, error) {
	if r.failCreate {
		return 0, errors.New("history unavailable")
	}
	record.Status = "sending"
	r.records = append(r.records, record)
	return int64(len(r.records)), nil
}
func (r *ledgerHistoryRepo) FinishEmailHistory(ctx context.Context, id int64, sent bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.records[id-1].Status = "failed"
	if sent {
		r.records[id-1].Status = "sent"
	}
	return nil
}
func (r *ledgerHistoryRepo) ListEmailHistory(context.Context, string, int64, int, int) (*MonthlyLedgerEmailHistory, error) {
	return &MonthlyLedgerEmailHistory{Items: r.records}, nil
}

func TestMonthlyLedgerEmailHistoryCapturesActualMessageAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	smtp := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settings.SetMultiple(ctx, smtp.settings()))
	notification := NewNotificationEmailService(settings, NewEmailService(settings, nil))
	repo := &ledgerHistoryRepo{manualLedgerEmailRepo: manualLedgerEmailRepo{limit: 10}}
	svc := NewMonthlyLedgerService(repo)
	svc.emailSend = notification.Send
	require.NoError(t, svc.SendManualEmail(ctx, manualLedgerInput()))
	require.Len(t, repo.records, 1)
	require.Equal(t, "sent", repo.records[0].Status)
	require.Equal(t, 42.35, repo.records[0].Amount)
	require.Contains(t, repo.records[0].HTML, "42.35")
	require.NotEmpty(t, repo.records[0].Subject)
	require.Equal(t, "monthly_ledger_manual", repo.records[0].SourceType)
	require.NoError(t, svc.SendManualEmail(ctx, manualLedgerInput()))
	require.Len(t, repo.records, 1)
	require.EqualValues(t, 1, smtp.messageCount())
	input := NotificationEmailSendInput{Event: NotificationEmailEventMonthlyLedger, UserID: 7, RecipientEmail: "auto@example.test", SourceType: "monthly_ledger", SourceID: "2026-08:7", Variables: map[string]string{"billing_month": "2026-08", "outstanding_amount": "20.00"}}
	require.NoError(t, svc.sendLimitedEmail(ctx, notification.Send, input))
	require.Len(t, repo.records, 2)
	require.Equal(t, "monthly_ledger", repo.records[1].SourceType)
	require.Equal(t, 20.0, repo.records[1].Amount)
	repo.failCreate = true
	input.SourceID = "2026-07:7"
	require.ErrorContains(t, svc.sendLimitedEmail(ctx, notification.Send, input), "history unavailable")
	require.EqualValues(t, 2, smtp.messageCount())
}

func TestMonthlyLedgerEmailHistoryFailureAfterCancellation(t *testing.T) {
	repo := &ledgerHistoryRepo{}
	svc := NewMonthlyLedgerService(repo)
	input := NotificationEmailSendInput{Variables: map[string]string{"outstanding_amount": "5.00", "billing_month": "2026-08"}}
	svc.recordEmailHistory(&input)
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, input.BeforeSend(ctx, NotificationEmailPreview{Subject: "Bill", HTML: "<p>$5</p>"}))
	cancel()
	require.NoError(t, input.AfterSend(ctx, context.DeadlineExceeded))
	require.Equal(t, "failed", repo.records[0].Status)
	svc.now = func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) }
	_, err := svc.ListEmailHistory(context.Background(), "bad", 7, 1, 20)
	require.ErrorIs(t, err, ErrMonthlyLedgerInvalidMonth)
	_, err = svc.ListEmailHistory(context.Background(), "2026-08", 0, 1, 20)
	require.ErrorIs(t, err, ErrMonthlyLedgerUserNotFound)
}
