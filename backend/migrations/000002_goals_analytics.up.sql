CREATE TABLE goals (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    type TEXT NOT NULL CHECK (type IN ('global', 'current')),
    target_amount_minor BIGINT NOT NULL CHECK (target_amount_minor > 0),
    deadline DATE,
    linked_account_id UUID REFERENCES accounts(id) ON DELETE RESTRICT,
    manual_current_amount_minor BIGINT CHECK (manual_current_amount_minor >= 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (type = 'global' AND linked_account_id IS NULL AND manual_current_amount_minor IS NULL)
        OR
        (type = 'current' AND num_nonnulls(linked_account_id, manual_current_amount_minor) = 1)
    )
);

CREATE INDEX goals_user_status_idx ON goals(user_id, status, created_at);
CREATE INDEX goals_linked_account_idx ON goals(linked_account_id) WHERE linked_account_id IS NOT NULL;
CREATE INDEX accounts_user_created_at_idx ON accounts(user_id, created_at);

