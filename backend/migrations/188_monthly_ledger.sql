-- Monthly manual billing ledger. Usage remains live in usage_logs; these
-- tables only persist per-user monthly multipliers and received payments.

CREATE TABLE IF NOT EXISTS monthly_ledger_multipliers (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  billing_month DATE NOT NULL,
  multiplier DECIMAL(10,4) NOT NULL DEFAULT 1,
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_monthly_ledger_multipliers_user_month UNIQUE (user_id, billing_month),
  CONSTRAINT chk_monthly_ledger_multipliers_month_start
    CHECK (date_trunc('month', billing_month)::date = billing_month),
  CONSTRAINT chk_monthly_ledger_multipliers_nonnegative
    CHECK (multiplier >= 0)
);

CREATE INDEX IF NOT EXISTS idx_monthly_ledger_multipliers_month
  ON monthly_ledger_multipliers(billing_month);

CREATE TABLE IF NOT EXISTS monthly_ledger_payments (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  billing_month DATE NOT NULL,
  amount DECIMAL(20,2) NOT NULL,
  paid_at TIMESTAMPTZ NOT NULL,
  note VARCHAR(500) NOT NULL DEFAULT '',
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_monthly_ledger_payments_month_start
    CHECK (date_trunc('month', billing_month)::date = billing_month),
  CONSTRAINT chk_monthly_ledger_payments_positive_amount
    CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_monthly_ledger_payments_month_user
  ON monthly_ledger_payments(billing_month, user_id);
CREATE INDEX IF NOT EXISTS idx_monthly_ledger_payments_user_month
  ON monthly_ledger_payments(user_id, billing_month);
CREATE INDEX IF NOT EXISTS idx_monthly_ledger_payments_paid_at
  ON monthly_ledger_payments(paid_at DESC);
