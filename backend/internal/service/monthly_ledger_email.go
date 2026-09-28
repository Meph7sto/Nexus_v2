package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type MonthlyLedgerEmailPreference struct {
	UserID         int64  `json:"user_id"`
	Email          string `json:"email"`
	Username       string `json:"username"`
	Enabled        bool   `json:"enabled"`
	EffectiveMonth string `json:"effective_month"`
}

type MonthlyLedgerEmailDelivery struct {
	MonthlyLedgerRow
	Month string
}

type MonthlyLedgerEmailRepository interface {
	ListEmailPreferences(context.Context, string, pagination.PaginationParams) ([]MonthlyLedgerEmailPreference, int64, error)
	SetEmailPreference(context.Context, int64, bool, time.Time, int64) error
	SetEmailPreferences(context.Context, []int64, bool, time.Time, int64) (int64, error)
	PendingEmailMonths(context.Context, time.Time) ([]string, error)
	HasPendingEmails(context.Context) (bool, error)
	PrepareEmails(context.Context, MonthlyLedgerPeriod) error
	DeliverNextEmail(context.Context, func(context.Context, MonthlyLedgerEmailDelivery) error) (bool, error)
}

func (s *MonthlyLedgerService) ListEmailPreferences(ctx context.Context, query string, page pagination.PaginationParams) ([]MonthlyLedgerEmailPreference, int64, error) {
	repo, ok := s.repo.(MonthlyLedgerEmailRepository)
	if !ok {
		return nil, 0, ErrMonthlyLedgerRepositoryNotReady
	}
	return repo.ListEmailPreferences(ctx, query, page)
}

func (s *MonthlyLedgerService) SetEmailPreference(ctx context.Context, userID int64, enabled bool, actorID int64) error {
	repo, ok := s.repo.(MonthlyLedgerEmailRepository)
	if !ok {
		return ErrMonthlyLedgerRepositoryNotReady
	}
	if userID <= 0 {
		return ErrMonthlyLedgerUserNotFound
	}
	now := s.now().In(s.location)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.location)
	return repo.SetEmailPreference(ctx, userID, enabled, month, actorID)
}

func (s *MonthlyLedgerService) SetEmailPreferences(ctx context.Context, userIDs []int64, enabled bool, actorID int64) (int64, error) {
	if len(userIDs) == 0 {
		return 0, ErrMonthlyLedgerNoUsersSelected
	}
	ids := make([]int64, 0, len(userIDs))
	seen := make(map[int64]bool, len(userIDs))
	for _, id := range userIDs {
		if id <= 0 {
			return 0, ErrMonthlyLedgerInvalidUserIDs
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	repo, ok := s.repo.(MonthlyLedgerEmailRepository)
	if !ok {
		return 0, ErrMonthlyLedgerRepositoryNotReady
	}
	now := s.now().In(s.location)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.location)
	return repo.SetEmailPreferences(ctx, ids, enabled, month, actorID)
}

func (s *MonthlyLedgerService) StartEmailNotifications(email *NotificationEmailService) {
	ctx, cancel := context.WithCancel(context.Background())
	s.emailCancel = cancel
	s.emailDone = make(chan struct{})
	go func() {
		defer close(s.emailDone)
		for {
			startedAt := s.now().In(s.location)
			runCtx, runCancel := context.WithTimeout(ctx, 4*time.Minute)
			err := s.processEmailNotifications(runCtx, email.Send)
			if err != nil && ctx.Err() == nil {
				slog.Error("monthly ledger email processing failed", "error", err)
			}
			runCancel()
			if ctx.Err() != nil {
				return
			}
			checkCtx, checkCancel := context.WithTimeout(ctx, 10*time.Second)
			pending, checkErr := s.hasPendingEmails(checkCtx)
			checkCancel()
			if checkErr != nil {
				slog.Error("monthly ledger email pending check failed", "error", checkErr)
			}
			// Empty queues sleep until next month; unfinished work resumes the next day.
			// Keep a month boundary crossed during catch-up from being skipped.
			next := nextMonthlyLedgerEmailRun(startedAt, pending || err != nil || checkErr != nil)
			timer := time.NewTimer(next.Sub(s.now()))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (s *MonthlyLedgerService) hasPendingEmails(ctx context.Context) (bool, error) {
	enabled, err := s.emailNotificationsEnabled(ctx)
	if err != nil || !enabled {
		return false, err
	}
	repo, ok := s.repo.(MonthlyLedgerEmailRepository)
	if !ok {
		return false, ErrMonthlyLedgerRepositoryNotReady
	}
	return repo.HasPendingEmails(ctx)
}

func nextMonthlyLedgerEmailRun(now time.Time, pending bool) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 15, 0, 0, now.Location())
	if now.Before(today) && (now.Day() == 1 || pending) {
		return today
	}
	if pending {
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 15, 0, 0, now.Location())
	}
	return time.Date(now.Year(), now.Month()+1, 1, 0, 15, 0, 0, now.Location())
}

func (s *MonthlyLedgerService) Stop() {
	if s.incomeCancel != nil {
		s.incomeCancel()
		<-s.incomeDone
	}
	if s.emailCancel != nil {
		s.emailCancel()
		<-s.emailDone
	}
}

func (s *MonthlyLedgerService) processEmailNotifications(ctx context.Context, send func(context.Context, NotificationEmailSendInput) error) error {
	if enabled, err := s.emailNotificationsEnabled(ctx); err != nil || !enabled {
		return err
	}
	repo, ok := s.repo.(MonthlyLedgerEmailRepository)
	if !ok {
		return ErrMonthlyLedgerRepositoryNotReady
	}
	period, err := s.resolvePeriod("")
	if err != nil {
		return err
	}
	// Allow in-flight usage records at the month boundary to reach storage.
	if s.now().Before(period.End.Add(15 * time.Minute)) {
		return nil
	}
	months, err := repo.PendingEmailMonths(ctx, period.End)
	if err != nil {
		return err
	}
	for _, month := range months {
		p, err := s.resolvePeriod(month)
		if err != nil {
			return err
		}
		if !p.CanRecordPayments {
			continue
		}
		if err := repo.PrepareEmails(ctx, p); err != nil {
			return err
		}
	}
	for ctx.Err() == nil {
		quota, err := s.EmailQuota(ctx)
		if err != nil {
			return err
		}
		if quota.Used >= quota.DailyLimit {
			return nil
		}
		// Re-read before each delivery so switching off also stops a running batch.
		if enabled, err := s.emailNotificationsEnabled(ctx); err != nil || !enabled {
			return err
		}
		limitReached := false
		processed, err := repo.DeliverNextEmail(ctx, func(ctx context.Context, d MonthlyLedgerEmailDelivery) error {
			err := s.sendLimitedEmail(ctx, send, NotificationEmailSendInput{
				Event: NotificationEmailEventMonthlyLedger, UserID: d.UserID,
				RecipientEmail: d.Email, RecipientName: d.Username,
				SourceType: "monthly_ledger", SourceID: fmt.Sprintf("%s:%d", d.Month, d.UserID),
				Variables: map[string]string{
					"billing_month": d.Month, "currency": "USD",
					"usage_amount":       fmt.Sprintf("%.2f", d.UsageAmount),
					"multiplier":         strconv.FormatFloat(d.Multiplier, 'f', -1, 64),
					"receivable_amount":  fmt.Sprintf("%.2f", d.ReceivableAmount),
					"paid_amount":        fmt.Sprintf("%.2f", d.PaidAmount),
					"outstanding_amount": fmt.Sprintf("%.2f", d.OutstandingAmount),
				},
			})
			limitReached = errors.Is(err, ErrMonthlyLedgerEmailDailyLimit)
			return err
		})
		if err != nil {
			return err
		}
		if !processed || limitReached {
			return nil
		}
	}
	return ctx.Err()
}

func (s *MonthlyLedgerService) emailNotificationsEnabled(ctx context.Context) (bool, error) {
	if s.emailSettings == nil {
		return false, nil
	}
	value, err := s.emailSettings.GetValue(ctx, SettingKeyMonthlyLedgerEmailEnabled)
	if errors.Is(err, ErrSettingNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return value == "true", nil
}
