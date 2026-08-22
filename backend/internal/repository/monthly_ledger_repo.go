package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type monthlyLedgerRepository struct {
	db *sql.DB
}

func NewMonthlyLedgerRepository(db *sql.DB) service.MonthlyLedgerRepository {
	return &monthlyLedgerRepository{db: db}
}

const monthlyLedgerBaseCTE = `
WITH usage_by_user AS (
	SELECT user_id, COALESCE(SUM(actual_cost), 0)::numeric AS usage_amount
	FROM usage_logs
	WHERE created_at >= $1 AND created_at < $2
	GROUP BY user_id
),
payments_by_user AS (
	SELECT
		user_id,
		COALESCE(SUM(amount), 0)::numeric AS paid_amount,
		COUNT(*)::bigint AS payment_count,
		MAX(paid_at) AS last_paid_at
	FROM monthly_ledger_payments
	WHERE billing_month = $3
	GROUP BY user_id
),
eligible_users AS (
	SELECT user_id FROM usage_by_user
	UNION
	SELECT user_id FROM payments_by_user
),
amounts AS (
	SELECT
		u.id AS user_id,
		u.email,
		COALESCE(u.username, '') AS username,
		(u.deleted_at IS NOT NULL) AS deleted,
		ROUND(COALESCE(usage.usage_amount, 0), 2) AS usage_amount,
		COALESCE(mult.multiplier, 1)::numeric AS multiplier,
		ROUND(COALESCE(usage.usage_amount, 0) * COALESCE(mult.multiplier, 1), 2) AS receivable_amount,
		ROUND(COALESCE(payments.paid_amount, 0), 2) AS paid_amount,
		COALESCE(payments.payment_count, 0) AS payment_count,
		payments.last_paid_at
	FROM eligible_users eligible
	JOIN users u ON u.id = eligible.user_id AND u.role = 'user'
	LEFT JOIN usage_by_user usage ON usage.user_id = u.id
	LEFT JOIN payments_by_user payments ON payments.user_id = u.id
	LEFT JOIN monthly_ledger_multipliers mult
		ON mult.user_id = u.id AND mult.billing_month = $3
),
ledger AS (
	SELECT
		amounts.*,
		GREATEST(receivable_amount - paid_amount, 0)::numeric AS outstanding_amount,
		GREATEST(paid_amount - receivable_amount, 0)::numeric AS overpaid_amount,
		CASE
			WHEN multiplier = 0 AND usage_amount > 0 AND paid_amount = 0 THEN 'waived'
			WHEN paid_amount > receivable_amount THEN 'overpaid'
			WHEN paid_amount = receivable_amount THEN 'settled'
			WHEN paid_amount = 0 THEN 'unpaid'
			ELSE 'partial'
		END AS status
	FROM amounts
),
filtered AS (
	SELECT *
	FROM ledger
	WHERE (
		$4 = '' OR email ILIKE '%' || $4 || '%' OR username ILIKE '%' || $4 || '%'
		OR user_id::text = $4
	)
	AND ($5 = '' OR status = $5)
)
`

func (r *monthlyLedgerRepository) List(ctx context.Context, period service.MonthlyLedgerPeriod, params service.MonthlyLedgerListParams) ([]service.MonthlyLedgerRow, *service.MonthlyLedgerSummary, *pagination.PaginationResult, error) {
	if r == nil || r.db == nil {
		return nil, nil, nil, service.ErrMonthlyLedgerRepositoryNotReady
	}
	pageParams := normalizeMonthlyLedgerPagination(params.Pagination)
	baseArgs := []any{period.Start, period.End, period.Start.Format("2006-01-02"), params.Query, params.Status}
	orderBy := monthlyLedgerOrderBy(pageParams.SortBy, pageParams.SortOrder)
	query := monthlyLedgerBaseCTE + `,
	summary AS (
		SELECT
			COALESCE(SUM(usage_amount), 0) AS usage_amount,
			COALESCE(SUM(receivable_amount), 0) AS receivable_amount,
			COALESCE(SUM(paid_amount), 0) AS paid_amount,
			COALESCE(SUM(outstanding_amount), 0) AS outstanding_amount,
			COALESCE(SUM(overpaid_amount), 0) AS overpaid_amount,
			COUNT(*)::bigint AS user_count,
			COUNT(*) FILTER (WHERE status = 'unpaid')::bigint AS unpaid_count,
			COUNT(*) FILTER (WHERE status = 'partial')::bigint AS partial_count,
			COUNT(*) FILTER (WHERE status = 'settled')::bigint AS settled_count,
			COUNT(*) FILTER (WHERE status = 'overpaid')::bigint AS overpaid_count,
			COUNT(*) FILTER (WHERE status = 'waived')::bigint AS waived_count
		FROM ledger
	),
	filtered_count AS (
		SELECT COUNT(*)::bigint AS total FROM filtered
	),
	page_items AS (
		SELECT filtered.*, ROW_NUMBER() OVER (ORDER BY ` + orderBy + `) AS row_position
		FROM filtered
		ORDER BY ` + orderBy + `
		LIMIT $6 OFFSET $7
	)
	SELECT
		summary.usage_amount, summary.receivable_amount, summary.paid_amount,
		summary.outstanding_amount, summary.overpaid_amount, summary.user_count,
		summary.unpaid_count, summary.partial_count, summary.settled_count,
		summary.overpaid_count, summary.waived_count, filtered_count.total,
		page_items.user_id, page_items.email, page_items.username, page_items.deleted,
		page_items.usage_amount, page_items.multiplier, page_items.receivable_amount,
		page_items.paid_amount, page_items.outstanding_amount, page_items.overpaid_amount,
		page_items.status, page_items.payment_count, page_items.last_paid_at
	FROM summary
	CROSS JOIN filtered_count
	LEFT JOIN page_items ON TRUE
	ORDER BY page_items.row_position NULLS LAST`
	rows, err := r.db.QueryContext(ctx, query, append(baseArgs, pageParams.Limit(), pageParams.Offset())...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("query monthly ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summary *service.MonthlyLedgerSummary
	var filteredTotal int64
	items := make([]service.MonthlyLedgerRow, 0)
	for rows.Next() {
		rowSummary := &service.MonthlyLedgerSummary{}
		var rowTotal int64
		var userID, paymentCount sql.NullInt64
		var email, username, status sql.NullString
		var deleted sql.NullBool
		var usageAmount, multiplier, receivableAmount sql.NullFloat64
		var paidAmount, outstandingAmount, overpaidAmount sql.NullFloat64
		var lastPaidAt sql.NullTime
		if err := rows.Scan(
			&rowSummary.UsageAmount, &rowSummary.ReceivableAmount, &rowSummary.PaidAmount,
			&rowSummary.OutstandingAmount, &rowSummary.OverpaidAmount, &rowSummary.UserCount,
			&rowSummary.UnpaidCount, &rowSummary.PartialCount, &rowSummary.SettledCount,
			&rowSummary.OverpaidCount, &rowSummary.WaivedCount, &rowTotal,
			&userID, &email, &username, &deleted, &usageAmount, &multiplier,
			&receivableAmount, &paidAmount, &outstandingAmount, &overpaidAmount,
			&status, &paymentCount, &lastPaidAt,
		); err != nil {
			return nil, nil, nil, fmt.Errorf("scan monthly ledger row: %w", err)
		}
		if summary == nil {
			summary = rowSummary
			filteredTotal = rowTotal
		}
		if !userID.Valid {
			continue
		}
		item := service.MonthlyLedgerRow{
			UserID: userID.Int64, Email: email.String, Username: username.String,
			Deleted: deleted.Bool, UsageAmount: usageAmount.Float64,
			Multiplier: multiplier.Float64, ReceivableAmount: receivableAmount.Float64,
			PaidAmount: paidAmount.Float64, OutstandingAmount: outstandingAmount.Float64,
			OverpaidAmount: overpaidAmount.Float64, Status: status.String,
			PaymentCount: paymentCount.Int64,
		}
		if lastPaidAt.Valid {
			value := lastPaidAt.Time
			item.LastPaidAt = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("iterate monthly ledger rows: %w", err)
	}
	if summary == nil {
		summary = &service.MonthlyLedgerSummary{}
	}
	return items, summary, paginationResultFromTotal(filteredTotal, pageParams), nil
}

func (r *monthlyLedgerRepository) ListPayments(ctx context.Context, billingMonth time.Time, userID int64) ([]service.MonthlyLedgerPayment, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrMonthlyLedgerRepositoryNotReady
	}
	var exists bool
	if err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'user')", userID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check monthly ledger user: %w", err)
	}
	if !exists {
		return nil, service.ErrMonthlyLedgerUserNotFound
	}
	query := `
		SELECT
			payments.id, payments.user_id, payments.billing_month, payments.amount,
			payments.paid_at, payments.note, payments.created_by,
			COALESCE(creators.email, ''), payments.updated_by,
			COALESCE(updaters.email, ''), payments.created_at, payments.updated_at
		FROM monthly_ledger_payments payments
		LEFT JOIN users creators ON creators.id = payments.created_by
		LEFT JOIN users updaters ON updaters.id = payments.updated_by
		WHERE payments.billing_month = $1 AND payments.user_id = $2
		ORDER BY payments.paid_at DESC, payments.id DESC`
	rows, err := r.db.QueryContext(ctx, query, billingMonth.Format("2006-01-02"), userID)
	if err != nil {
		return nil, fmt.Errorf("list monthly ledger payments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	payments := make([]service.MonthlyLedgerPayment, 0)
	for rows.Next() {
		payment, err := scanMonthlyLedgerPayment(rows.Scan)
		if err != nil {
			return nil, err
		}
		payments = append(payments, *payment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monthly ledger payments: %w", err)
	}
	return payments, nil
}

func (r *monthlyLedgerRepository) SetMultiplier(ctx context.Context, billingMonth time.Time, userID int64, multiplier float64, actorID int64) (float64, error) {
	if r == nil || r.db == nil {
		return 0, service.ErrMonthlyLedgerRepositoryNotReady
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin multiplier update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var previous float64
	query := `
		SELECT COALESCE((
			SELECT multiplier FROM monthly_ledger_multipliers
			WHERE user_id = users.id AND billing_month = $2
		), 1)
		FROM users
		WHERE id = $1 AND role = 'user'
		FOR UPDATE`
	if err := tx.QueryRowContext(ctx, query, userID, billingMonth.Format("2006-01-02")).Scan(&previous); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, service.ErrMonthlyLedgerUserNotFound
		}
		return 0, fmt.Errorf("get previous monthly multiplier: %w", err)
	}
	upsert := `
		INSERT INTO monthly_ledger_multipliers (
			user_id, billing_month, multiplier, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (user_id, billing_month) DO UPDATE SET
			multiplier = EXCLUDED.multiplier,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()`
	if _, err := tx.ExecContext(ctx, upsert, userID, billingMonth.Format("2006-01-02"), multiplier, actorID); err != nil {
		return 0, fmt.Errorf("set monthly multiplier: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit multiplier update: %w", err)
	}
	return previous, nil
}

func (r *monthlyLedgerRepository) CreatePayment(ctx context.Context, billingMonth time.Time, payment *service.MonthlyLedgerPayment) error {
	if r == nil || r.db == nil {
		return service.ErrMonthlyLedgerRepositoryNotReady
	}
	query := `
		INSERT INTO monthly_ledger_payments (
			user_id, billing_month, amount, paid_at, note, created_by, updated_by,
			created_at, updated_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $6, $7, $7
		FROM users
		WHERE id = $1 AND role = 'user'
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(
		ctx, query, payment.UserID, billingMonth.Format("2006-01-02"), payment.Amount,
		payment.PaidAt, payment.Note, payment.CreatedBy, payment.CreatedAt,
	).Scan(&payment.ID, &payment.CreatedAt, &payment.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrMonthlyLedgerUserNotFound
	}
	if err != nil {
		return fmt.Errorf("create monthly ledger payment: %w", err)
	}
	return nil
}

func (r *monthlyLedgerRepository) UpdatePayment(ctx context.Context, paymentID int64, update service.MonthlyLedgerPaymentUpdate, actorID int64) (*service.MonthlyLedgerPayment, *service.MonthlyLedgerPayment, error) {
	if r == nil || r.db == nil {
		return nil, nil, service.ErrMonthlyLedgerRepositoryNotReady
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin payment update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	before, err := getMonthlyLedgerPaymentForUpdate(ctx, tx, paymentID)
	if err != nil {
		return nil, nil, err
	}
	query := `
		UPDATE monthly_ledger_payments
		SET amount = $1, paid_at = $2, note = $3, updated_by = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING id, user_id, billing_month, amount, paid_at, note,
			created_by, ''::text, updated_by, ''::text, created_at, updated_at`
	after, err := scanMonthlyLedgerPayment(tx.QueryRowContext(ctx, query, update.Amount, update.PaidAt, update.Note, actorID, paymentID).Scan)
	if err != nil {
		return nil, nil, fmt.Errorf("update monthly ledger payment: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit payment update: %w", err)
	}
	return before, after, nil
}

func (r *monthlyLedgerRepository) DeletePayment(ctx context.Context, paymentID int64) (*service.MonthlyLedgerPayment, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrMonthlyLedgerRepositoryNotReady
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin payment delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	payment, err := getMonthlyLedgerPaymentForUpdate(ctx, tx, paymentID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM monthly_ledger_payments WHERE id = $1", paymentID); err != nil {
		return nil, fmt.Errorf("delete monthly ledger payment: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit payment delete: %w", err)
	}
	return payment, nil
}

func getMonthlyLedgerPaymentForUpdate(ctx context.Context, tx *sql.Tx, paymentID int64) (*service.MonthlyLedgerPayment, error) {
	query := `
		SELECT id, user_id, billing_month, amount, paid_at, note,
			created_by, ''::text, updated_by, ''::text, created_at, updated_at
		FROM monthly_ledger_payments
		WHERE id = $1
		FOR UPDATE`
	payment, err := scanMonthlyLedgerPayment(tx.QueryRowContext(ctx, query, paymentID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMonthlyLedgerPaymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get monthly ledger payment: %w", err)
	}
	return payment, nil
}

func scanMonthlyLedgerPayment(scan func(dest ...any) error) (*service.MonthlyLedgerPayment, error) {
	var payment service.MonthlyLedgerPayment
	var billingMonth time.Time
	if err := scan(
		&payment.ID, &payment.UserID, &billingMonth, &payment.Amount,
		&payment.PaidAt, &payment.Note, &payment.CreatedBy,
		&payment.CreatedByEmail, &payment.UpdatedBy, &payment.UpdatedByEmail,
		&payment.CreatedAt, &payment.UpdatedAt,
	); err != nil {
		return nil, err
	}
	payment.BillingMonth = billingMonth.Format("2006-01")
	return &payment, nil
}

func normalizeMonthlyLedgerPagination(params pagination.PaginationParams) pagination.PaginationParams {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 50
	}
	if params.PageSize > 200 {
		params.PageSize = 200
	}
	params.SortOrder = params.NormalizedSortOrder(pagination.SortOrderDesc)
	return params
}

func monthlyLedgerOrderBy(sortBy, sortOrder string) string {
	columns := map[string]string{
		"user":               "email",
		"usage_amount":       "usage_amount",
		"multiplier":         "multiplier",
		"receivable_amount":  "receivable_amount",
		"paid_amount":        "paid_amount",
		"outstanding_amount": "outstanding_amount",
		"last_paid_at":       "last_paid_at",
	}
	column, ok := columns[strings.TrimSpace(sortBy)]
	if !ok {
		column = "outstanding_amount"
	}
	order := strings.ToUpper(pagination.NormalizeSortOrder(sortOrder, pagination.SortOrderDesc))
	if column == "last_paid_at" {
		return column + " " + order + " NULLS LAST, user_id ASC"
	}
	return column + " " + order + ", user_id ASC"
}

var _ service.MonthlyLedgerRepository = (*monthlyLedgerRepository)(nil)
