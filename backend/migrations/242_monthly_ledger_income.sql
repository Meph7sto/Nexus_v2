CREATE TABLE IF NOT EXISTS monthly_ledger_income_schedules (
 id BIGSERIAL PRIMARY KEY,
 title VARCHAR(200) NOT NULL, customer VARCHAR(200) NOT NULL,
 category VARCHAR(100) NOT NULL, note VARCHAR(500) NOT NULL DEFAULT '',
 amount NUMERIC(20,2) NOT NULL CHECK(amount > 0),
 cost NUMERIC(20,2) NOT NULL DEFAULT 0 CHECK(cost >= 0),
 first_date DATE NOT NULL, interval_months INTEGER NOT NULL CHECK(interval_months BETWEEN 1 AND 1200),
 end_date DATE, next_index INTEGER NOT NULL DEFAULT 0 CHECK(next_index >= 0),
 state VARCHAR(10) NOT NULL DEFAULT 'active' CHECK(state IN ('active','paused','ended')),
 created_by BIGINT NOT NULL, updated_by BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK(end_date IS NULL OR end_date >= first_date)
);
CREATE TABLE IF NOT EXISTS monthly_ledger_income_entries (
 id BIGSERIAL PRIMARY KEY,
 title VARCHAR(200) NOT NULL, customer VARCHAR(200) NOT NULL,
 category VARCHAR(100) NOT NULL, note VARCHAR(500) NOT NULL DEFAULT '',
 amount NUMERIC(20,2) NOT NULL CHECK(amount > 0),
 cost NUMERIC(20,2) NOT NULL DEFAULT 0 CHECK(cost >= 0),
 billing_date DATE NOT NULL, due_date DATE NOT NULL,
 schedule_id BIGINT REFERENCES monthly_ledger_income_schedules(id), occurrence_date DATE,
 deleted_at TIMESTAMPTZ,
 created_by BIGINT NOT NULL, updated_by BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(schedule_id, occurrence_date),
 CHECK((schedule_id IS NULL) = (occurrence_date IS NULL))
);
CREATE INDEX IF NOT EXISTS idx_monthly_ledger_income_entries_month ON monthly_ledger_income_entries(billing_date) WHERE deleted_at IS NULL;
CREATE TABLE IF NOT EXISTS monthly_ledger_income_payments (
 id BIGSERIAL PRIMARY KEY,
 entry_id BIGINT NOT NULL REFERENCES monthly_ledger_income_entries(id),
 amount NUMERIC(20,2) NOT NULL CHECK(amount > 0), paid_at TIMESTAMPTZ NOT NULL,
 note VARCHAR(500) NOT NULL DEFAULT '',
 created_by BIGINT NOT NULL, updated_by BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_monthly_ledger_income_payments_entry ON monthly_ledger_income_payments(entry_id);
