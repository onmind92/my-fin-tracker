CREATE TABLE restore_imports (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    archive_id UUID NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, archive_id)
);

CREATE INDEX restore_imports_user_idx ON restore_imports(user_id, imported_at DESC);
