CREATE TABLE transfers (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    from_account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    to_account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    description TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (from_account_id <> to_account_id)
);

CREATE INDEX transfers_user_occurred_idx ON transfers(user_id, occurred_at DESC);
CREATE INDEX transfers_from_account_idx ON transfers(from_account_id, occurred_at DESC);
CREATE INDEX transfers_to_account_idx ON transfers(to_account_id, occurred_at DESC);

CREATE TABLE budgets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    period TEXT NOT NULL DEFAULT 'monthly' CHECK (period IN ('monthly')),
    start_date DATE NOT NULL,
    end_date DATE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date IS NULL OR end_date >= start_date)
);

CREATE INDEX budgets_user_active_idx ON budgets(user_id, is_active, start_date);
CREATE UNIQUE INDEX budgets_user_category_active_unique ON budgets(user_id, category_id) WHERE is_active = true;

CREATE TABLE recurring_executions (
    id UUID PRIMARY KEY,
    recurring_transaction_id UUID NOT NULL REFERENCES recurring_transactions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scheduled_for TIMESTAMPTZ NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL,
    status TEXT NOT NULL CHECK (status IN ('success', 'skipped', 'failed')),
    error_message TEXT,
    UNIQUE (recurring_transaction_id, scheduled_for)
);

CREATE INDEX recurring_executions_user_idx ON recurring_executions(user_id, executed_at DESC);
CREATE INDEX recurring_executions_recurring_idx ON recurring_executions(recurring_transaction_id, scheduled_for DESC);
