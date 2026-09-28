package service

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var ErrIncomeInvalid = infraerrors.BadRequest("INCOME_INVALID", "invalid income fields, dates or amounts")
var ErrIncomeNotFound = infraerrors.NotFound("INCOME_NOT_FOUND", "income record not found")
var ErrIncomeHasPayments = infraerrors.BadRequest("INCOME_HAS_PAYMENTS", "remove payment records before deleting this entry")
var ErrIncomeScheduleEnded = infraerrors.BadRequest("INCOME_SCHEDULE_ENDED", "ended schedules cannot be changed")

type IncomeFields struct {
	Title    string  `json:"title"`
	Customer string  `json:"customer"`
	Category string  `json:"category"`
	Note     string  `json:"note"`
	Amount   float64 `json:"amount"`
	Cost     float64 `json:"cost"`
}
type IncomeEntryInput struct {
	IncomeFields
	BillingDate string `json:"billing_date"`
	DueDate     string `json:"due_date"`
}
type IncomeEntry struct {
	IncomeEntryInput
	ID                int64   `json:"id"`
	ScheduleID        *int64  `json:"schedule_id"`
	PaidAmount        float64 `json:"paid_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	OverpaidAmount    float64 `json:"overpaid_amount"`
	Profit            float64 `json:"profit"`
	Status            string  `json:"status"`
	CreatedBy         int64   `json:"created_by"`
	UpdatedBy         int64   `json:"updated_by"`
}
type IncomeScheduleInput struct {
	IncomeFields
	FirstDate      string  `json:"first_date"`
	IntervalMonths int     `json:"interval_months"`
	EndDate        *string `json:"end_date"`
}
type IncomeSchedule struct {
	IncomeScheduleInput
	ID        int64  `json:"id"`
	State     string `json:"state"`
	NextIndex int    `json:"next_index"`
	CreatedBy int64  `json:"created_by"`
	UpdatedBy int64  `json:"updated_by"`
}
type IncomePayment struct {
	MonthlyLedgerPaymentInput
	ID        int64 `json:"id"`
	EntryID   int64 `json:"entry_id"`
	CreatedBy int64 `json:"created_by"`
	UpdatedBy int64 `json:"updated_by"`
}
type IncomeSummary struct {
	ReceivableAmount  float64 `json:"receivable_amount"`
	PaidAmount        float64 `json:"paid_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	OverpaidAmount    float64 `json:"overpaid_amount"`
	Cost              float64 `json:"cost"`
	Profit            float64 `json:"profit"`
}
type IncomeListParams struct {
	Month, Query, Category, Status string
	Page, PageSize                 int
}
type IncomeList struct {
	Items      []IncomeEntry `json:"items"`
	Total      int64         `json:"total"`
	Summary    IncomeSummary `json:"summary"`
	Categories []string      `json:"categories"`
}
type IncomeRepository interface {
	ListIncome(context.Context, IncomeListParams) (*IncomeList, error)
	SaveIncomeEntry(context.Context, int64, IncomeEntryInput, int64) (*IncomeEntry, error)
	DeleteIncomeEntry(context.Context, int64, int64) error
	ListIncomePayments(context.Context, int64) ([]IncomePayment, error)
	SaveIncomePayment(context.Context, int64, int64, MonthlyLedgerPaymentInput, int64) (*IncomePayment, error)
	DeleteIncomePayment(context.Context, int64) error
	ListIncomeSchedules(context.Context) ([]IncomeSchedule, error)
	SaveIncomeSchedule(context.Context, int64, IncomeScheduleInput, int64, time.Time) (*IncomeSchedule, error)
	SetIncomeScheduleState(context.Context, int64, string, int64, time.Time) error
	ProcessIncomeSchedules(context.Context, time.Time) error
}

func (s *MonthlyLedgerService) incomeRepo() (IncomeRepository, error) {
	r, ok := s.repo.(IncomeRepository)
	if !ok {
		return nil, ErrMonthlyLedgerRepositoryNotReady
	}
	return r, nil
}
func validIncomeMoney(v float64, positive bool) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 999999999999.99 || (positive && v == 0) {
		return false
	}
	d := decimal.NewFromFloat(v)
	return d.Equal(d.Round(2))
}
func validateIncomeFields(f *IncomeFields) error {
	f.Title = strings.TrimSpace(f.Title)
	f.Customer = strings.TrimSpace(f.Customer)
	f.Category = strings.TrimSpace(f.Category)
	f.Note = strings.TrimSpace(f.Note)
	if f.Title == "" || f.Customer == "" || f.Category == "" || utf8.RuneCountInString(f.Title) > 200 || utf8.RuneCountInString(f.Customer) > 200 || utf8.RuneCountInString(f.Category) > 100 || utf8.RuneCountInString(f.Note) > 500 || !validIncomeMoney(f.Amount, true) || !validIncomeMoney(f.Cost, false) {
		return ErrIncomeInvalid
	}
	return nil
}
func IncomeDate(raw string, loc *time.Location) (time.Time, error) {
	d, e := time.ParseInLocation("2006-01-02", raw, loc)
	if e != nil || d.Year() < 1 || d.Format("2006-01-02") != raw {
		return time.Time{}, ErrIncomeInvalid
	}
	return d, nil
}

// Compute from the original anchor so short months do not shift later dates.
func IncomeOccurrence(first time.Time, interval, index int) time.Time {
	month := time.Date(first.Year(), first.Month()+time.Month(interval*index), 1, 0, 0, 0, 0, first.Location())
	last := month.AddDate(0, 1, -1).Day()
	day := first.Day()
	if day > last {
		day = last
	}
	return month.AddDate(0, 0, day-1)
}
func (s *MonthlyLedgerService) validateSchedule(in *IncomeScheduleInput) error {
	if err := validateIncomeFields(&in.IncomeFields); err != nil {
		return err
	}
	first, err := IncomeDate(in.FirstDate, s.location)
	if err != nil || in.IntervalMonths < 1 || in.IntervalMonths > 1200 {
		return ErrIncomeInvalid
	}
	if in.EndDate != nil {
		end, e := IncomeDate(*in.EndDate, s.location)
		if e != nil || end.Before(first) {
			return ErrIncomeInvalid
		}
	}
	return nil
}
func (s *MonthlyLedgerService) PreviewIncomeSchedule(in IncomeScheduleInput) (int, error) {
	if err := s.validateSchedule(&in); err != nil {
		return 0, err
	}
	first, _ := IncomeDate(in.FirstDate, s.location)
	now := s.now().In(s.location)
	count := 0
	for {
		d := IncomeOccurrence(first, in.IntervalMonths, count)
		if d.After(now) || (in.EndDate != nil && d.Format("2006-01-02") > *in.EndDate) {
			break
		}
		count++
	}
	return count, nil
}
func (s *MonthlyLedgerService) ListIncome(ctx context.Context, p IncomeListParams) (*IncomeList, error) {
	period, err := s.resolvePeriod(p.Month)
	if err != nil {
		return nil, err
	}
	p.Month = period.Month
	if p.Status != "" && p.Status != "unpaid" && p.Status != "partial" && p.Status != "settled" && p.Status != "overpaid" {
		return nil, ErrIncomeInvalid
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 200 {
		p.PageSize = 200
	}
	r, e := s.incomeRepo()
	if e != nil {
		return nil, e
	}
	return r.ListIncome(ctx, p)
}
func (s *MonthlyLedgerService) SaveIncomeEntry(ctx context.Context, id int64, in IncomeEntryInput, actor int64) (*IncomeEntry, error) {
	if err := validateIncomeFields(&in.IncomeFields); err != nil {
		return nil, err
	}
	date, err := IncomeDate(in.BillingDate, s.location)
	if err != nil || date.After(s.now()) {
		return nil, ErrIncomeInvalid
	}
	if _, err = IncomeDate(in.DueDate, s.location); err != nil {
		return nil, err
	}
	r, e := s.incomeRepo()
	if e != nil {
		return nil, e
	}
	return r.SaveIncomeEntry(ctx, id, in, actor)
}
func (s *MonthlyLedgerService) DeleteIncomeEntry(ctx context.Context, id, actor int64) error {
	r, e := s.incomeRepo()
	if e != nil {
		return e
	}
	return r.DeleteIncomeEntry(ctx, id, actor)
}
func (s *MonthlyLedgerService) ListIncomePayments(ctx context.Context, id int64) ([]IncomePayment, error) {
	r, e := s.incomeRepo()
	if e != nil {
		return nil, e
	}
	return r.ListIncomePayments(ctx, id)
}
func (s *MonthlyLedgerService) SaveIncomePayment(ctx context.Context, id, entry int64, in MonthlyLedgerPaymentInput, actor int64) (*IncomePayment, error) {
	if !validIncomeMoney(in.Amount, true) || in.PaidAt.IsZero() || in.PaidAt.After(s.now()) || utf8.RuneCountInString(in.Note) > 500 {
		return nil, ErrIncomeInvalid
	}
	r, e := s.incomeRepo()
	if e != nil {
		return nil, e
	}
	return r.SaveIncomePayment(ctx, id, entry, in, actor)
}
func (s *MonthlyLedgerService) DeleteIncomePayment(ctx context.Context, id int64) error {
	r, e := s.incomeRepo()
	if e != nil {
		return e
	}
	return r.DeleteIncomePayment(ctx, id)
}
func (s *MonthlyLedgerService) ListIncomeSchedules(ctx context.Context) ([]IncomeSchedule, error) {
	r, e := s.incomeRepo()
	if e != nil {
		return nil, e
	}
	return r.ListIncomeSchedules(ctx)
}
func (s *MonthlyLedgerService) SaveIncomeSchedule(ctx context.Context, id int64, in IncomeScheduleInput, actor int64) (*IncomeSchedule, error) {
	if e := s.validateSchedule(&in); e != nil {
		return nil, e
	}
	r, e := s.incomeRepo()
	if e != nil {
		return nil, e
	}
	return r.SaveIncomeSchedule(ctx, id, in, actor, s.now().In(s.location))
}
func (s *MonthlyLedgerService) SetIncomeScheduleState(ctx context.Context, id int64, state string, actor int64) error {
	if state != "active" && state != "paused" && state != "ended" {
		return ErrIncomeInvalid
	}
	r, e := s.incomeRepo()
	if e != nil {
		return e
	}
	return r.SetIncomeScheduleState(ctx, id, state, actor, s.now().In(s.location))
}
func (s *MonthlyLedgerService) IncomeOverview(ctx context.Context, month string) (*IncomeSummary, error) {
	ledger, e := s.List(ctx, month, MonthlyLedgerListParams{})
	if e != nil {
		return nil, e
	}
	income, e := s.ListIncome(ctx, IncomeListParams{Month: ledger.Month})
	if e != nil {
		return nil, e
	}
	result := income.Summary
	add := func(a, b float64) float64 {
		return decimal.NewFromFloat(a).Add(decimal.NewFromFloat(b)).InexactFloat64()
	}
	result.ReceivableAmount = add(result.ReceivableAmount, ledger.Summary.ReceivableAmount)
	result.PaidAmount = add(result.PaidAmount, ledger.Summary.PaidAmount)
	result.OutstandingAmount = add(result.OutstandingAmount, ledger.Summary.OutstandingAmount)
	result.OverpaidAmount = add(result.OverpaidAmount, ledger.Summary.OverpaidAmount)
	return &result, nil
}
func (s *MonthlyLedgerService) StartIncomeSchedules() {
	ctx, cancel := context.WithCancel(context.Background())
	s.incomeCancel = cancel
	s.incomeDone = make(chan struct{})
	go func() {
		defer close(s.incomeDone)
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			run, done := context.WithTimeout(ctx, 4*time.Minute)
			r, e := s.incomeRepo()
			if e == nil {
				e = r.ProcessIncomeSchedules(run, s.now().In(s.location))
			}
			done()
			if e != nil && ctx.Err() == nil {
				slog.Error("income schedule processing failed", "error", e)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
