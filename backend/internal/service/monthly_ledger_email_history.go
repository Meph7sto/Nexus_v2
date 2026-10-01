package service

import (
	"context"
	"strconv"
	"time"
)

type MonthlyLedgerEmailRecord struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Month      string     `json:"month"`
	SourceType string     `json:"source_type"`
	Recipient  string     `json:"recipient"`
	Amount     float64    `json:"amount"`
	Subject    string     `json:"subject"`
	HTML       string     `json:"html"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	SentAt     *time.Time `json:"sent_at"`
}

type MonthlyLedgerEmailHistory struct {
	Items     []MonthlyLedgerEmailRecord `json:"items"`
	Total     int64                      `json:"total"`
	SentCount int64                      `json:"sent_count"`
}

type MonthlyLedgerEmailHistoryRepository interface {
	CreateEmailHistory(context.Context, MonthlyLedgerEmailRecord) (int64, error)
	FinishEmailHistory(context.Context, int64, bool) error
	ListEmailHistory(context.Context, string, int64, int, int) (*MonthlyLedgerEmailHistory, error)
}

func (s *MonthlyLedgerService) ListEmailHistory(ctx context.Context, month string, userID int64, page, size int) (*MonthlyLedgerEmailHistory, error) {
	period, err := s.resolvePeriod(month)
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, ErrMonthlyLedgerUserNotFound
	}
	repo, ok := s.repo.(MonthlyLedgerEmailHistoryRepository)
	if !ok {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return repo.ListEmailHistory(ctx, period.Month, userID, page, size)
}

func (s *MonthlyLedgerService) recordEmailHistory(input *NotificationEmailSendInput) {
	repo, ok := s.repo.(MonthlyLedgerEmailHistoryRepository)
	if !ok {
		return
	}
	var id int64
	input.BeforeSend = func(ctx context.Context, rendered NotificationEmailPreview) error {
		amount, err := strconv.ParseFloat(input.Variables["outstanding_amount"], 64)
		if err != nil {
			return err
		}
		id, err = repo.CreateEmailHistory(ctx, MonthlyLedgerEmailRecord{
			UserID: input.UserID, Month: input.Variables["billing_month"], SourceType: input.SourceType,
			Recipient: input.RecipientEmail, Amount: amount, Subject: rendered.Subject, HTML: rendered.HTML,
		})
		return err
	}
	input.AfterSend = func(ctx context.Context, sendErr error) error {
		// Keep bookkeeping possible even if the SMTP request timed out.
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return repo.FinishEmailHistory(recordCtx, id, sendErr == nil)
	}
}
