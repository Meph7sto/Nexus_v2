package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.IncomeRepository = (*monthlyLedgerRepository)(nil)

const incomeCTE = `WITH amounts AS (
 SELECT e.*, COALESCE(p.paid_amount,0) AS paid_amount FROM monthly_ledger_income_entries e
 LEFT JOIN (SELECT entry_id,SUM(amount) AS paid_amount FROM monthly_ledger_income_payments GROUP BY entry_id) p ON p.entry_id=e.id
 WHERE e.deleted_at IS NULL
), income AS (
 SELECT amounts.*, GREATEST(amount-paid_amount,0) AS outstanding_amount,
 GREATEST(paid_amount-amount,0) AS overpaid_amount, amount-cost AS profit,
 CASE WHEN paid_amount=0 THEN 'unpaid' WHEN paid_amount<amount THEN 'partial' WHEN paid_amount=amount THEN 'settled' ELSE 'overpaid' END AS status
 FROM amounts
) `

func decodeIncomeRow(row *sql.Row, out any) error {
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrIncomeNotFound
		}
		return err
	}
	return json.Unmarshal(raw, out)
}
func (r *monthlyLedgerRepository) ListIncome(ctx context.Context, p service.IncomeListParams) (*service.IncomeList, error) {
	out := &service.IncomeList{}
	err := decodeIncomeRow(r.db.QueryRowContext(ctx, incomeCTE+`, monthly AS (
 SELECT * FROM income WHERE billing_date >= $1::date AND billing_date < ($1::date + INTERVAL '1 month')
 ), filtered AS (
 SELECT * FROM monthly WHERE ($2='' OR title ILIKE '%'||$2||'%' OR customer ILIKE '%'||$2||'%' OR note ILIKE '%'||$2||'%')
 AND ($3='' OR category=$3) AND ($4='' OR status=$4)
 ), page AS (SELECT * FROM filtered ORDER BY due_date DESC,id DESC LIMIT $5 OFFSET $6)
 SELECT json_build_object(
 'items',COALESCE((SELECT json_agg(page) FROM page),'[]'::json),
 'total',(SELECT COUNT(*) FROM filtered),
 'categories',COALESCE((SELECT json_agg(category ORDER BY category) FROM (SELECT DISTINCT category FROM monthly) c),'[]'::json),
 'summary',(SELECT json_build_object('receivable_amount',COALESCE(SUM(amount),0),'paid_amount',COALESCE(SUM(paid_amount),0),
 'outstanding_amount',COALESCE(SUM(outstanding_amount),0),'overpaid_amount',COALESCE(SUM(overpaid_amount),0),
 'cost',COALESCE(SUM(cost),0),'profit',COALESCE(SUM(profit),0)) FROM monthly))`, p.Month+"-01", p.Query, p.Category, p.Status, p.PageSize, (p.Page-1)*p.PageSize), out)
	return out, err
}
func lockIncomeEntry(ctx context.Context, tx *sql.Tx, id int64) error {
	var found int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM monthly_ledger_income_entries WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrIncomeNotFound
	}
	return err
}
func (r *monthlyLedgerRepository) SaveIncomeEntry(ctx context.Context, id int64, in service.IncomeEntryInput, actor int64) (*service.IncomeEntry, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	args := []any{in.Title, in.Customer, in.Category, in.Note, in.Amount, in.Cost, in.BillingDate, in.DueDate, actor}
	if id == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO monthly_ledger_income_entries(title,customer,category,note,amount,cost,billing_date,due_date,created_by,updated_by)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9) RETURNING id`, args...).Scan(&id)
	} else {
		if err = lockIncomeEntry(ctx, tx, id); err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE monthly_ledger_income_entries SET title=$1,customer=$2,category=$3,note=$4,amount=$5,cost=$6,billing_date=$7,due_date=$8,updated_by=$9,updated_at=NOW() WHERE id=$10`, append(args, id)...)
	}
	if err != nil {
		return nil, err
	}
	out := &service.IncomeEntry{}
	err = decodeIncomeRow(tx.QueryRowContext(ctx, incomeCTE+`SELECT row_to_json(income) FROM income WHERE id=$1`, id), out)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
func (r *monthlyLedgerRepository) DeleteIncomeEntry(ctx context.Context, id, actor int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockIncomeEntry(ctx, tx, id); err != nil {
		return err
	}
	var has bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM monthly_ledger_income_payments WHERE entry_id=$1)`, id).Scan(&has); err != nil {
		return err
	}
	if has {
		return service.ErrIncomeHasPayments
	}
	_, err = tx.ExecContext(ctx, `UPDATE monthly_ledger_income_entries SET deleted_at=NOW(),updated_at=NOW(),updated_by=$2 WHERE id=$1`, id, actor)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *monthlyLedgerRepository) ListIncomePayments(ctx context.Context, id int64) ([]service.IncomePayment, error) {
	var out []service.IncomePayment
	err := decodeIncomeRow(r.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT json_agg(p ORDER BY p.paid_at DESC,p.id DESC) FROM monthly_ledger_income_payments p WHERE p.entry_id=e.id),'[]'::json)
 FROM monthly_ledger_income_entries e WHERE e.id=$1 AND e.deleted_at IS NULL`, id), &out)
	return out, err
}
func (r *monthlyLedgerRepository) SaveIncomePayment(ctx context.Context, id, entry int64, in service.MonthlyLedgerPaymentInput, actor int64) (*service.IncomePayment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if id != 0 {
		if err = tx.QueryRowContext(ctx, `SELECT entry_id FROM monthly_ledger_income_payments WHERE id=$1`, id).Scan(&entry); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, service.ErrIncomeNotFound
			}
			return nil, err
		}
	}
	if err = lockIncomeEntry(ctx, tx, entry); err != nil {
		return nil, err
	}
	out := &service.IncomePayment{}
	if id == 0 {
		err = decodeIncomeRow(tx.QueryRowContext(ctx, `INSERT INTO monthly_ledger_income_payments(entry_id,amount,paid_at,note,created_by,updated_by) VALUES($1,$2,$3,$4,$5,$5) RETURNING row_to_json(monthly_ledger_income_payments)`, entry, in.Amount, in.PaidAt, in.Note, actor), out)
	} else {
		err = decodeIncomeRow(tx.QueryRowContext(ctx, `UPDATE monthly_ledger_income_payments SET amount=$1,paid_at=$2,note=$3,updated_by=$4,updated_at=NOW() WHERE id=$5 RETURNING row_to_json(monthly_ledger_income_payments)`, in.Amount, in.PaidAt, in.Note, actor, id), out)
	}
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
func (r *monthlyLedgerRepository) DeleteIncomePayment(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var entry int64
	err = tx.QueryRowContext(ctx, `SELECT entry_id FROM monthly_ledger_income_payments WHERE id=$1`, id).Scan(&entry)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrIncomeNotFound
	}
	if err != nil {
		return err
	}
	if err = lockIncomeEntry(ctx, tx, entry); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM monthly_ledger_income_payments WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrIncomeNotFound
	}
	return tx.Commit()
}
func (r *monthlyLedgerRepository) ListIncomeSchedules(ctx context.Context) ([]service.IncomeSchedule, error) {
	var out []service.IncomeSchedule
	err := decodeIncomeRow(r.db.QueryRowContext(ctx, `SELECT COALESCE(json_agg(s ORDER BY s.id DESC),'[]'::json) FROM monthly_ledger_income_schedules s`), &out)
	return out, err
}
func lockIncomeSchedule(ctx context.Context, tx *sql.Tx, id int64) (*service.IncomeSchedule, error) {
	out := &service.IncomeSchedule{}
	err := decodeIncomeRow(tx.QueryRowContext(ctx, `SELECT row_to_json(s) FROM monthly_ledger_income_schedules s WHERE id=$1 FOR UPDATE`, id), out)
	return out, err
}

// Caller holds the schedule lock; inserts and cursor advancement commit together.
func materializeIncome(ctx context.Context, tx *sql.Tx, s *service.IncomeSchedule, now time.Time, skip bool) error {
	first, err := service.IncomeDate(s.FirstDate, now.Location())
	if err != nil {
		return err
	}
	for {
		date := service.IncomeOccurrence(first, s.IntervalMonths, s.NextIndex)
		key := date.Format("2006-01-02")
		if s.EndDate != nil && key > *s.EndDate {
			s.State = "ended"
			break
		}
		if date.After(now) {
			break
		}
		if !skip {
			_, err = tx.ExecContext(ctx, `INSERT INTO monthly_ledger_income_entries(title,customer,category,note,amount,cost,billing_date,due_date,schedule_id,occurrence_date,created_by,updated_by)
 VALUES($1,$2,$3,$4,$5,$6,$7,$7,$8,$7,$9,$9) ON CONFLICT(schedule_id,occurrence_date) DO NOTHING`, s.Title, s.Customer, s.Category, s.Note, s.Amount, s.Cost, key, s.ID, s.UpdatedBy)
			if err != nil {
				return err
			}
		}
		s.NextIndex++
	}
	_, err = tx.ExecContext(ctx, `UPDATE monthly_ledger_income_schedules SET next_index=$2,state=$3,updated_at=NOW() WHERE id=$1`, s.ID, s.NextIndex, s.State)
	return err
}
func (r *monthlyLedgerRepository) SaveIncomeSchedule(ctx context.Context, id int64, in service.IncomeScheduleInput, actor int64, now time.Time) (*service.IncomeSchedule, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if id != 0 {
		old, e := lockIncomeSchedule(ctx, tx, id)
		if e != nil {
			return nil, e
		}
		if old.State == "ended" {
			return nil, service.ErrIncomeScheduleEnded
		}
		if old.FirstDate != in.FirstDate || old.IntervalMonths != in.IntervalMonths {
			return nil, service.ErrIncomeInvalid
		}
		if old.State == "active" {
			if e = materializeIncome(ctx, tx, old, now, false); e != nil {
				return nil, e
			}
			if old.State == "ended" {
				return nil, service.ErrIncomeScheduleEnded
			}
		}
	}
	args := []any{in.Title, in.Customer, in.Category, in.Note, in.Amount, in.Cost, in.FirstDate, in.IntervalMonths, in.EndDate, actor}
	if id == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO monthly_ledger_income_schedules(title,customer,category,note,amount,cost,first_date,interval_months,end_date,created_by,updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10) RETURNING id`, args...).Scan(&id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE monthly_ledger_income_schedules SET title=$1,customer=$2,category=$3,note=$4,amount=$5,cost=$6,first_date=$7,interval_months=$8,end_date=$9,updated_by=$10,updated_at=NOW() WHERE id=$11`, append(args, id)...)
	}
	if err != nil {
		return nil, err
	}
	out, err := lockIncomeSchedule(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if out.State == "active" {
		if err = materializeIncome(ctx, tx, out, now, false); err != nil {
			return nil, err
		}
	}
	return out, tx.Commit()
}
func (r *monthlyLedgerRepository) SetIncomeScheduleState(ctx context.Context, id int64, state string, actor int64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	s, err := lockIncomeSchedule(ctx, tx, id)
	if err != nil {
		return err
	}
	if s.State == "ended" {
		return service.ErrIncomeScheduleEnded
	}
	if s.State == "active" {
		err = materializeIncome(ctx, tx, s, now, false)
	} else if state == "active" {
		err = materializeIncome(ctx, tx, s, now, true)
	}
	if err != nil {
		return err
	}
	if s.State != "ended" {
		s.State = state
	}
	_, err = tx.ExecContext(ctx, `UPDATE monthly_ledger_income_schedules SET state=$2,updated_by=$3,updated_at=NOW() WHERE id=$1`, id, s.State, actor)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *monthlyLedgerRepository) ProcessIncomeSchedules(ctx context.Context, now time.Time) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM monthly_ledger_income_schedules WHERE state='active' ORDER BY id`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		err = func() error {
			tx, e := r.db.BeginTx(ctx, nil)
			if e != nil {
				return e
			}
			defer tx.Rollback()
			s, e := lockIncomeSchedule(ctx, tx, id)
			if e != nil {
				return e
			}
			if s.State == "active" {
				if e = materializeIncome(ctx, tx, s, now, false); e != nil {
					return e
				}
			}
			return tx.Commit()
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
