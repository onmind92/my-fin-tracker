CREATE TABLE users (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    type TEXT NOT NULL CHECK (type IN ('card', 'cash', 'savings', 'investment', 'other')),
    currency CHAR(3) NOT NULL CHECK (currency = 'RUB'),
    opening_balance_minor BIGINT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE categories (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    type TEXT NOT NULL CHECK (type IN ('expense', 'income')),
    icon TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, name, type)
);

CREATE TABLE transactions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    type TEXT NOT NULL CHECK (type IN ('expense', 'income')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    description TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX transactions_user_id_idx ON transactions(user_id);
CREATE INDEX transactions_account_id_idx ON transactions(account_id);
CREATE INDEX transactions_occurred_at_idx ON transactions(occurred_at DESC);
CREATE INDEX transactions_user_occurred_at_idx ON transactions(user_id, occurred_at DESC);

INSERT INTO users (id) VALUES ('00000000-0000-0000-0000-000000000001');

INSERT INTO categories (id, user_id, name, type, icon) VALUES
('10000000-0000-0000-0000-000000000001', NULL, 'Food', 'expense', '🍴'),
('10000000-0000-0000-0000-000000000002', NULL, 'Transport', 'expense', '🚕'),
('10000000-0000-0000-0000-000000000003', NULL, 'Shopping', 'expense', '🛒'),
('10000000-0000-0000-0000-000000000004', NULL, 'Entertainment', 'expense', '🎬'),
('10000000-0000-0000-0000-000000000005', NULL, 'Health', 'expense', '❤️'),
('10000000-0000-0000-0000-000000000006', NULL, 'Other', 'expense', '📦'),
('20000000-0000-0000-0000-000000000001', NULL, 'Salary', 'income', '💼'),
('20000000-0000-0000-0000-000000000002', NULL, 'Freelance', 'income', '🖥️'),
('20000000-0000-0000-0000-000000000003', NULL, 'Gift', 'income', '🎁'),
('20000000-0000-0000-0000-000000000004', NULL, 'Other', 'income', '✨');

