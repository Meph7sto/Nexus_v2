package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.MonthlyLedgerEmailHistoryRepository = (*monthlyLedgerRepository)(nil)

func (r *monthlyLedgerRepository) CreateEmailHistory(ctx context.Context, in service.MonthlyLedgerEmailRecord) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO monthly_ledger_email_history
 (user_id,billing_month,source_type,recipient,amount,subject,html,status)
 VALUES($1,$2::date,$3,$4,$5,$6,$7,'sending') RETURNING id`, in.UserID, in.Month+"-01", in.SourceType, in.Recipient, in.Amount, in.Subject, in.HTML).Scan(&id)
	return id, err
}

func (r *monthlyLedgerRepository) FinishEmailHistory(ctx context.Context, id int64, sent bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE monthly_ledger_email_history SET
 status=CASE WHEN $2 THEN 'sent' ELSE 'failed' END, sent_at=CASE WHEN $2 THEN NOW() ELSE NULL END WHERE id=$1`, id, sent)
	return err
}

func (r *monthlyLedgerRepository) ListEmailHistory(ctx context.Context, month string, userID int64, page, size int) (*service.MonthlyLedgerEmailHistory, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `WITH history AS (
 SELECT id,user_id,to_char(billing_month,'YYYY-MM') AS month,source_type,recipient,amount,subject,html,status,created_at,sent_at
 FROM monthly_ledger_email_history WHERE user_id=$1 AND billing_month=$2::date
 ), page AS (SELECT * FROM history ORDER BY id DESC LIMIT $3 OFFSET $4)
 SELECT json_build_object('items',COALESCE((SELECT json_agg(page ORDER BY id DESC) FROM page),'[]'::json),
 'total',(SELECT COUNT(*) FROM history),'sent_count',(SELECT COUNT(*) FROM history WHERE status='sent'))`, userID, month+"-01", size, (page-1)*size).Scan(&raw)
	if err != nil {
		return nil, err
	}
	out := &service.MonthlyLedgerEmailHistory{}
	err = json.Unmarshal(raw, out)
	return out, err
}
