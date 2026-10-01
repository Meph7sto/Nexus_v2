CREATE TABLE monthly_ledger_email_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    billing_month DATE NOT NULL,
    source_type TEXT NOT NULL,
    recipient TEXT NOT NULL,
    amount NUMERIC(30,2) NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    html TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('sending', 'sent', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);
CREATE INDEX monthly_ledger_email_history_month_user ON monthly_ledger_email_history(billing_month, user_id, id DESC);

-- Older automatic notices have amounts and send times, but no saved message body.
INSERT INTO monthly_ledger_email_history(user_id, billing_month, source_type, recipient, amount, status, created_at, sent_at)
SELECT d.user_id, d.billing_month, 'monthly_ledger', u.email, d.outstanding_amount, 'sent', d.sent_at, d.sent_at
FROM monthly_ledger_email_deliveries d JOIN users u ON u.id = d.user_id
WHERE d.sent_at IS NOT NULL;
