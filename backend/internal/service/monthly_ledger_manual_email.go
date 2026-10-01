package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

type MonthlyLedgerManualEmailInput struct {
	Month     string  `json:"month"`
	UserID    int64   `json:"user_id"`
	Amount    float64 `json:"amount"`
	RequestID string  `json:"request_id"`
}

func (s *MonthlyLedgerService) SendManualEmail(ctx context.Context, input MonthlyLedgerManualEmailInput) error {
	month, err := time.Parse("2006-01", input.Month)
	if err != nil || month.Year() < 1 || month.Format("2006-01") != input.Month {
		return ErrMonthlyLedgerInvalidMonth
	}
	if input.UserID <= 0 {
		return ErrMonthlyLedgerUserNotFound
	}
	if !validMoneyNumber(input.Amount) || input.Amount <= 0 || !hasAtMostDecimalPlaces(input.Amount, 2) {
		return ErrMonthlyLedgerInvalidAmount
	}
	requestID, err := uuid.Parse(input.RequestID)
	if err != nil {
		return infraerrors.BadRequest("MONTHLY_LEDGER_EMAIL_INVALID_REQUEST", "request_id must be a UUID")
	}
	repo, ok := s.repo.(MonthlyLedgerManualEmailRepository)
	if !ok || s.emailSend == nil {
		return ErrMonthlyLedgerRepositoryNotReady
	}
	recipient, err := repo.EmailRecipient(ctx, input.UserID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(recipient.Email) == "" {
		return ErrMonthlyLedgerUserNotFound
	}
	sendCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return s.sendLimitedEmail(sendCtx, s.emailSend, NotificationEmailSendInput{
		Event:  NotificationEmailEventMonthlyLedgerManual,
		UserID: recipient.UserID, RecipientEmail: recipient.Email, RecipientName: recipient.Username,
		SourceType: "monthly_ledger_manual", SourceID: fmt.Sprintf("%s:%d:%s:%s", input.Month, input.UserID, requestID, fmt.Sprintf("%.2f", input.Amount)),
		Variables: map[string]string{"billing_month": input.Month, "currency": "USD", "outstanding_amount": fmt.Sprintf("%.2f", input.Amount)},
	})
}

var (
	ErrMonthlyLedgerEmailDailyLimit   = infraerrors.New(http.StatusTooManyRequests, "MONTHLY_LEDGER_EMAIL_DAILY_LIMIT", "monthly ledger daily email limit reached")
	ErrMonthlyLedgerEmailInvalidLimit = infraerrors.BadRequest("MONTHLY_LEDGER_EMAIL_INVALID_LIMIT", "daily email limit must be a non-negative integer")
)

type MonthlyLedgerEmailQuota struct {
	Date       string `json:"date"`
	DailyLimit int    `json:"daily_limit"`
	Used       int    `json:"used"`
}

type MonthlyLedgerManualEmailRepository interface {
	EmailQuota(context.Context, time.Time) (*MonthlyLedgerEmailQuota, error)
	SetEmailDailyLimit(context.Context, int, int64) error
	ReserveEmailAttempt(context.Context, time.Time) error
	EmailRecipient(context.Context, int64) (*MonthlyLedgerEmailPreference, error)
}

func (s *MonthlyLedgerService) EmailQuota(ctx context.Context) (*MonthlyLedgerEmailQuota, error) {
	repo, ok := s.repo.(MonthlyLedgerManualEmailRepository)
	if !ok {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	return repo.EmailQuota(ctx, s.now().In(s.location))
}

func (s *MonthlyLedgerService) SetEmailDailyLimit(ctx context.Context, limit int, actorID int64) error {
	if limit < 0 || int64(limit) > 2147483647 {
		return ErrMonthlyLedgerEmailInvalidLimit
	}
	repo, ok := s.repo.(MonthlyLedgerManualEmailRepository)
	if !ok {
		return ErrMonthlyLedgerRepositoryNotReady
	}
	return repo.SetEmailDailyLimit(ctx, limit, actorID)
}

func (s *MonthlyLedgerService) sendLimitedEmail(ctx context.Context, send func(context.Context, NotificationEmailSendInput) error, input NotificationEmailSendInput) error {
	repo, ok := s.repo.(MonthlyLedgerManualEmailRepository)
	if !ok {
		return ErrMonthlyLedgerRepositoryNotReady
	}
	// Commit the reservation before SMTP: ambiguous delivery failures must not
	// allow retries or concurrent instances to exceed the daily sending cap.
	if err := repo.ReserveEmailAttempt(ctx, s.now().In(s.location)); err != nil {
		return err
	}
	s.recordEmailHistory(&input)
	return send(ctx, input)
}
