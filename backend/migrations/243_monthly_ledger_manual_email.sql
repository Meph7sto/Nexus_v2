CREATE TABLE monthly_ledger_email_limits (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    daily_limit INTEGER NOT NULL DEFAULT 5 CHECK (daily_limit >= 0),
    updated_by BIGINT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO monthly_ledger_email_limits (id) VALUES (TRUE);

CREATE TABLE monthly_ledger_email_daily_usage (
    send_date DATE PRIMARY KEY,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0)
);
