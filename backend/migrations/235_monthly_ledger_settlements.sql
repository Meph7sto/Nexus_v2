CREATE TABLE IF NOT EXISTS monthly_ledger_settlements (
  user_id BIGINT NOT NULL REFERENCES users(id),
  billing_month DATE NOT NULL,
  manually_settled BOOLEAN NOT NULL DEFAULT FALSE,
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, billing_month),
  CONSTRAINT chk_monthly_ledger_settlements_month_start
    CHECK (date_trunc('month', billing_month)::date = billing_month)
);

CREATE INDEX IF NOT EXISTS idx_monthly_ledger_settlements_month
  ON monthly_ledger_settlements(billing_month);
