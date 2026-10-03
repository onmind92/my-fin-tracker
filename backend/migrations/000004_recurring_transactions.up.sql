CREATE TABLE recurring_transactions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('expense', 'income')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    description TEXT,
    frequency TEXT NOT NULL CHECK (frequency IN ('daily', 'weekly', 'monthly', 'yearly')),
    interval_value INTEGER NOT NULL DEFAULT 1 CHECK (interval_value >= 1),
    start_date DATE NOT NULL,
    next_run_at TIMESTAMPTZ NOT NULL,
    end_date DATE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date IS NULL OR end_date >= start_date)
);

ALTER TABLE transactions
    ADD COLUMN recurring_transaction_id UUID REFERENCES recurring_transactions(id) ON DELETE SET NULL,
    ADD COLUMN scheduled_for TIMESTAMPTZ;

CREATE UNIQUE INDEX transactions_recurring_schedule_unique
    ON transactions(recurring_transaction_id, scheduled_for)
    WHERE recurring_transaction_id IS NOT NULL AND scheduled_for IS NOT NULL;

CREATE INDEX recurring_transactions_due_idx
    ON recurring_transactions(next_run_at)
    WHERE is_active = true;

CREATE INDEX recurring_transactions_user_idx
    ON recurring_transactions(user_id, is_active, next_run_at);

