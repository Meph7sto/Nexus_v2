CREATE TABLE monthly_ledger_email_preferences (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  effective_month DATE NOT NULL,
  updated_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE monthly_ledger_email_deliveries (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  billing_month DATE NOT NULL,
  usage_amount NUMERIC(30,10) NOT NULL,
  multiplier NUMERIC(20,4) NOT NULL,
  receivable_amount NUMERIC(30,2) NOT NULL,
  paid_amount NUMERIC(30,2) NOT NULL,
  outstanding_amount NUMERIC(30,2) NOT NULL,
  skipped BOOLEAN NOT NULL DEFAULT FALSE,
  sent_at TIMESTAMPTZ,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  attempts INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, billing_month)
);
CREATE INDEX monthly_ledger_email_pending ON monthly_ledger_email_deliveries(next_attempt_at)
  WHERE sent_at IS NULL AND NOT skipped;
