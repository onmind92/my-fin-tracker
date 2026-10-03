ALTER TABLE users
    ADD COLUMN email TEXT,
    ADD COLUMN password_hash TEXT,
    ADD COLUMN display_name TEXT,
    ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';

CREATE UNIQUE INDEX users_email_lower_unique ON users(lower(email)) WHERE email IS NOT NULL;

CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    csrf_token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_idx ON sessions(user_id);
CREATE INDEX sessions_expires_idx ON sessions(expires_at);

