package service

import (
	"context"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

const (
	MonthlyLedgerStatusUnpaid   = "unpaid"
	MonthlyLedgerStatusPartial  = "partial"
	MonthlyLedgerStatusSettled  = "settled"
	MonthlyLedgerStatusOverpaid = "overpaid"
	MonthlyLedgerStatusWaived   = "waived"
)

var (
	ErrMonthlyLedgerInvalidMonth       = infraerrors.BadRequest("MONTHLY_LEDGER_INVALID_MONTH", "month must use YYYY-MM format")
	ErrMonthlyLedgerFutureMonth        = infraerrors.BadRequest("MONTHLY_LEDGER_FUTURE_MONTH", "future months are not available")
	ErrMonthlyLedgerOpenMonthPayment   = infraerrors.BadRequest("MONTHLY_LEDGER_OPEN_MONTH_PAYMENT", "payments can only be recorded for completed months")
	ErrMonthlyLedgerInvalidMultiplier  = infraerrors.BadRequest("MONTHLY_LEDGER_INVALID_MULTIPLIER", "multiplier must be a non-negative number with at most 4 decimal places")
	ErrMonthlyLedgerInvalidAmount      = infraerrors.BadRequest("MONTHLY_LEDGER_INVALID_AMOUNT", "payment amount must be greater than zero with at most 2 decimal places")
	ErrMonthlyLedgerFuturePaymentTime  = infraerrors.BadRequest("MONTHLY_LEDGER_FUTURE_PAYMENT_TIME", "payment time cannot be in the future")
	ErrMonthlyLedgerNoteTooLong        = infraerrors.BadRequest("MONTHLY_LEDGER_NOTE_TOO_LONG", "payment note cannot exceed 500 characters")
	ErrMonthlyLedgerInvalidStatus      = infraerrors.BadRequest("MONTHLY_LEDGER_INVALID_STATUS", "invalid monthly ledger status")
	ErrMonthlyLedgerUserNotFound       = infraerrors.NotFound("MONTHLY_LEDGER_USER_NOT_FOUND", "regular user not found")
	ErrMonthlyLedgerPaymentNotFound    = infraerrors.NotFound("MONTHLY_LEDGER_PAYMENT_NOT_FOUND", "monthly ledger payment not found")
	ErrMonthlyLedgerRepositoryNotReady = infraerrors.ServiceUnavailable("MONTHLY_LEDGER_UNAVAILABLE", "monthly ledger is unavailable")
)

type MonthlyLedgerPeriod struct {
	Month             string    `json:"month"`
	CurrentMonth      string    `json:"current_month"`
	Start             time.Time `json:"-"`
	End               time.Time `json:"-"`
	CanRecordPayments bool      `json:"can_record_payments"`
}

type MonthlyLedgerAmounts struct {
	ReceivableAmount  float64 `json:"receivable_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	OverpaidAmount    float64 `json:"overpaid_amount"`
	Status            string  `json:"status"`
}

type MonthlyLedgerRow struct {
	UserID            int64      `json:"user_id"`
	Email             string     `json:"email"`
	Username          string     `json:"username"`
	Deleted           bool       `json:"deleted"`
	UsageAmount       float64    `json:"usage_amount"`
	Multiplier        float64    `json:"multiplier"`
	ReceivableAmount  float64    `json:"receivable_amount"`
	PaidAmount        float64    `json:"paid_amount"`
	OutstandingAmount float64    `json:"outstanding_amount"`
	OverpaidAmount    float64    `json:"overpaid_amount"`
	Status            string     `json:"status"`
	PaymentCount      int64      `json:"payment_count"`
	LastPaidAt        *time.Time `json:"last_paid_at,omitempty"`
}

type MonthlyLedgerSummary struct {
	UsageAmount       float64 `json:"usage_amount"`
	ReceivableAmount  float64 `json:"receivable_amount"`
	PaidAmount        float64 `json:"paid_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	OverpaidAmount    float64 `json:"overpaid_amount"`
	UserCount         int64   `json:"user_count"`
	UnpaidCount       int64   `json:"unpaid_count"`
	PartialCount      int64   `json:"partial_count"`
	SettledCount      int64   `json:"settled_count"`
	OverpaidCount     int64   `json:"overpaid_count"`
	WaivedCount       int64   `json:"waived_count"`
}

type MonthlyLedgerListParams struct {
	Pagination pagination.PaginationParams
	Query      string
	Status     string
}

type MonthlyLedgerList struct {
	Items             []MonthlyLedgerRow    `json:"items"`
	Summary           *MonthlyLedgerSummary `json:"summary"`
	Total             int64                 `json:"total"`
	Page              int                   `json:"page"`
	PageSize          int                   `json:"page_size"`
	Pages             int                   `json:"pages"`
	Month             string                `json:"month"`
	CurrentMonth      string                `json:"current_month"`
	CanRecordPayments bool                  `json:"can_record_payments"`
}

type MonthlyLedgerPayment struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	BillingMonth   string    `json:"billing_month"`
	Amount         float64   `json:"amount"`
	PaidAt         time.Time `json:"paid_at"`
	Note           string    `json:"note"`
	CreatedBy      int64     `json:"created_by"`
	CreatedByEmail string    `json:"created_by_email"`
	UpdatedBy      int64     `json:"updated_by"`
	UpdatedByEmail string    `json:"updated_by_email"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MonthlyLedgerPaymentInput struct {
	Amount float64   `json:"amount"`
	PaidAt time.Time `json:"paid_at"`
	Note   string    `json:"note"`
}

type MonthlyLedgerPaymentUpdate = MonthlyLedgerPaymentInput

type MonthlyLedgerMultiplierChange struct {
	UserID             int64   `json:"user_id"`
	BillingMonth       string  `json:"billing_month"`
	PreviousMultiplier float64 `json:"previous_multiplier"`
	Multiplier         float64 `json:"multiplier"`
}

type MonthlyLedgerPaymentChange struct {
	Before *MonthlyLedgerPayment `json:"before,omitempty"`
	After  *MonthlyLedgerPayment `json:"after,omitempty"`
}

type MonthlyLedgerRepository interface {
	List(ctx context.Context, period MonthlyLedgerPeriod, params MonthlyLedgerListParams) ([]MonthlyLedgerRow, *MonthlyLedgerSummary, *pagination.PaginationResult, error)
	ListPayments(ctx context.Context, billingMonth time.Time, userID int64) ([]MonthlyLedgerPayment, error)
	SetMultiplier(ctx context.Context, billingMonth time.Time, userID int64, multiplier float64, actorID int64) (float64, error)
	CreatePayment(ctx context.Context, payment *MonthlyLedgerPayment) error
	UpdatePayment(ctx context.Context, paymentID int64, update MonthlyLedgerPaymentUpdate, actorID int64) (*MonthlyLedgerPayment, *MonthlyLedgerPayment, error)
	DeletePayment(ctx context.Context, paymentID int64) (*MonthlyLedgerPayment, error)
}

type MonthlyLedgerService struct {
	repo     MonthlyLedgerRepository
	now      func() time.Time
	location *time.Location
}

func NewMonthlyLedgerService(repo MonthlyLedgerRepository) *MonthlyLedgerService {
	return &MonthlyLedgerService{repo: repo, now: timezone.Now, location: timezone.Location()}
}

func ResolveMonthlyLedgerPeriod(raw string, now time.Time, loc *time.Location) (MonthlyLedgerPeriod, error) {
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)
	currentStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	monthStart := currentStart.AddDate(0, -1, 0)
	raw = strings.TrimSpace(raw)
	if raw != "" {
		if len(raw) != len("2006-01") {
			return MonthlyLedgerPeriod{}, ErrMonthlyLedgerInvalidMonth
		}
		parsed, err := time.ParseInLocation("2006-01", raw, loc)
		if err != nil || parsed.Format("2006-01") != raw {
			return MonthlyLedgerPeriod{}, ErrMonthlyLedgerInvalidMonth
		}
		monthStart = parsed
	}
	if monthStart.After(currentStart) {
		return MonthlyLedgerPeriod{}, ErrMonthlyLedgerFutureMonth
	}
	return MonthlyLedgerPeriod{
		Month:             monthStart.Format("2006-01"),
		CurrentMonth:      currentStart.Format("2006-01"),
		Start:             monthStart,
		End:               monthStart.AddDate(0, 1, 0),
		CanRecordPayments: monthStart.Before(currentStart),
	}, nil
}

func CalculateMonthlyLedgerAmounts(usageAmount, multiplier, paidAmount float64) MonthlyLedgerAmounts {
	usage := decimal.NewFromFloat(usageAmount)
	mult := decimal.NewFromFloat(multiplier)
	paid := decimal.NewFromFloat(paidAmount).Round(2)
	receivable := usage.Mul(mult).Round(2)
	difference := receivable.Sub(paid)
	zero := decimal.Zero

	amounts := MonthlyLedgerAmounts{ReceivableAmount: receivable.InexactFloat64()}
	switch {
	case multiplier == 0 && usage.GreaterThan(zero) && paid.Equal(zero):
		amounts.Status = MonthlyLedgerStatusWaived
	case paid.GreaterThan(receivable):
		amounts.Status = MonthlyLedgerStatusOverpaid
		amounts.OverpaidAmount = paid.Sub(receivable).InexactFloat64()
	case paid.Equal(receivable):
		amounts.Status = MonthlyLedgerStatusSettled
	case paid.Equal(zero):
		amounts.Status = MonthlyLedgerStatusUnpaid
	default:
		amounts.Status = MonthlyLedgerStatusPartial
	}
	if difference.GreaterThan(zero) {
		amounts.OutstandingAmount = difference.InexactFloat64()
	}
	return amounts
}

func (s *MonthlyLedgerService) resolvePeriod(raw string) (MonthlyLedgerPeriod, error) {
	if s == nil {
		return MonthlyLedgerPeriod{}, ErrMonthlyLedgerRepositoryNotReady
	}
	return ResolveMonthlyLedgerPeriod(raw, s.now(), s.location)
}

func (s *MonthlyLedgerService) List(ctx context.Context, month string, params MonthlyLedgerListParams) (*MonthlyLedgerList, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	period, err := s.resolvePeriod(month)
	if err != nil {
		return nil, err
	}
	params.Query = strings.TrimSpace(params.Query)
	params.Status = strings.TrimSpace(strings.ToLower(params.Status))
	if params.Status != "" && !isMonthlyLedgerStatus(params.Status) {
		return nil, ErrMonthlyLedgerInvalidStatus
	}
	items, summary, page, err := s.repo.List(ctx, period, params)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []MonthlyLedgerRow{}
	}
	if summary == nil {
		summary = &MonthlyLedgerSummary{}
	}
	if page == nil {
		page = &pagination.PaginationResult{Page: 1, PageSize: 50}
	}
	return &MonthlyLedgerList{
		Items: items, Summary: summary, Total: page.Total, Page: page.Page,
		PageSize: page.PageSize, Pages: page.Pages, Month: period.Month,
		CurrentMonth: period.CurrentMonth, CanRecordPayments: period.CanRecordPayments,
	}, nil
}

func (s *MonthlyLedgerService) ListPayments(ctx context.Context, month string, userID int64) ([]MonthlyLedgerPayment, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	period, err := s.resolvePeriod(month)
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, ErrMonthlyLedgerUserNotFound
	}
	payments, err := s.repo.ListPayments(ctx, period.Start, userID)
	if payments == nil && err == nil {
		payments = []MonthlyLedgerPayment{}
	}
	return payments, err
}

func (s *MonthlyLedgerService) SetMultiplier(ctx context.Context, month string, userID int64, multiplier float64, actorID int64) (*MonthlyLedgerMultiplierChange, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	period, err := s.resolvePeriod(month)
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, ErrMonthlyLedgerUserNotFound
	}
	if !validMoneyNumber(multiplier) || multiplier < 0 || !hasAtMostDecimalPlaces(multiplier, 4) {
		return nil, ErrMonthlyLedgerInvalidMultiplier
	}
	previous, err := s.repo.SetMultiplier(ctx, period.Start, userID, multiplier, actorID)
	if err != nil {
		return nil, err
	}
	return &MonthlyLedgerMultiplierChange{UserID: userID, BillingMonth: period.Month, PreviousMultiplier: previous, Multiplier: multiplier}, nil
}

func (s *MonthlyLedgerService) CreatePayment(ctx context.Context, month string, userID int64, input MonthlyLedgerPaymentInput, actorID int64) (*MonthlyLedgerPayment, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	period, err := s.resolvePeriod(month)
	if err != nil {
		return nil, err
	}
	if !period.CanRecordPayments {
		return nil, ErrMonthlyLedgerOpenMonthPayment
	}
	if userID <= 0 {
		return nil, ErrMonthlyLedgerUserNotFound
	}
	if err := s.validatePaymentInput(input); err != nil {
		return nil, err
	}
	now := s.now()
	payment := &MonthlyLedgerPayment{
		UserID: userID, BillingMonth: period.Month, Amount: input.Amount,
		PaidAt: input.PaidAt, Note: strings.TrimSpace(input.Note), CreatedBy: actorID,
		UpdatedBy: actorID, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return nil, err
	}
	return payment, nil
}

func (s *MonthlyLedgerService) UpdatePayment(ctx context.Context, paymentID int64, input MonthlyLedgerPaymentInput, actorID int64) (*MonthlyLedgerPaymentChange, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	if paymentID <= 0 {
		return nil, ErrMonthlyLedgerPaymentNotFound
	}
	if err := s.validatePaymentInput(input); err != nil {
		return nil, err
	}
	input.Note = strings.TrimSpace(input.Note)
	before, after, err := s.repo.UpdatePayment(ctx, paymentID, input, actorID)
	if err != nil {
		return nil, err
	}
	return &MonthlyLedgerPaymentChange{Before: before, After: after}, nil
}

func (s *MonthlyLedgerService) DeletePayment(ctx context.Context, paymentID int64) (*MonthlyLedgerPayment, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	if paymentID <= 0 {
		return nil, ErrMonthlyLedgerPaymentNotFound
	}
	return s.repo.DeletePayment(ctx, paymentID)
}

func (s *MonthlyLedgerService) validatePaymentInput(input MonthlyLedgerPaymentInput) error {
	if !validMoneyNumber(input.Amount) || input.Amount <= 0 || !hasAtMostDecimalPlaces(input.Amount, 2) {
		return ErrMonthlyLedgerInvalidAmount
	}
	if input.PaidAt.IsZero() {
		return ErrMonthlyLedgerFuturePaymentTime
	}
	if input.PaidAt.After(s.now()) {
		return ErrMonthlyLedgerFuturePaymentTime
	}
	if utf8.RuneCountInString(strings.TrimSpace(input.Note)) > 500 {
		return ErrMonthlyLedgerNoteTooLong
	}
	return nil
}

func isMonthlyLedgerStatus(status string) bool {
	switch status {
	case MonthlyLedgerStatusUnpaid, MonthlyLedgerStatusPartial, MonthlyLedgerStatusSettled, MonthlyLedgerStatusOverpaid, MonthlyLedgerStatusWaived:
		return true
	default:
		return false
	}
}

func validMoneyNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func hasAtMostDecimalPlaces(value float64, places int32) bool {
	return decimal.NewFromFloat(value).Equal(decimal.NewFromFloat(value).Round(places))
}
