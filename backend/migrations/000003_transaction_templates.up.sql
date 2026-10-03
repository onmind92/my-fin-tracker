CREATE TABLE transaction_templates (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('expense', 'income')),
    amount_minor BIGINT CHECK (amount_minor > 0),
    account_id UUID REFERENCES accounts(id) ON DELETE SET NULL,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX transaction_templates_user_idx ON transaction_templates(user_id, updated_at DESC);

