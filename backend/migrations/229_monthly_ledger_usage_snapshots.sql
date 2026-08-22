-- Preserve the exact per-user usage basis for completed monthly ledgers before
-- raw usage_logs retention or an administrator cleanup removes source rows.
-- The migration connection uses the application's configured session timezone,
-- matching the month boundaries used by MonthlyLedgerService.

CREATE TABLE IF NOT EXISTS monthly_ledger_usage_snapshots (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  billing_month DATE NOT NULL,
  usage_amount DECIMAL(30,10) NOT NULL DEFAULT 0,
  captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_monthly_ledger_usage_snapshots_user_month UNIQUE (user_id, billing_month),
  CONSTRAINT chk_monthly_ledger_usage_snapshots_month_start
    CHECK (date_trunc('month', billing_month)::date = billing_month)
);

CREATE INDEX IF NOT EXISTS idx_monthly_ledger_usage_snapshots_month_user
  ON monthly_ledger_usage_snapshots(billing_month, user_id);

INSERT INTO monthly_ledger_usage_snapshots (user_id, billing_month, usage_amount)
SELECT
  usage_logs.user_id,
  date_trunc('month', created_at)::date AS billing_month,
  COALESCE(SUM(actual_cost), 0)::numeric AS usage_amount
FROM usage_logs
JOIN users ON users.id = usage_logs.user_id AND users.role = 'user'
WHERE date_trunc('month', created_at) < date_trunc('month', NOW())
GROUP BY usage_logs.user_id, date_trunc('month', created_at)::date
ON CONFLICT (user_id, billing_month) DO NOTHING;
