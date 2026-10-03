package store

import (
	"context"
	"errors"
	"time"

	"finance-tracker/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type UserRecord struct {
	domain.User
	PasswordHash string
}
type SessionRecord struct {
	ID            string
	User          domain.User
	CSRFTokenHash []byte
	ExpiresAt     time.Time
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string, displayName *string, timezone string) (UserRecord, error) {
	id := uuid.NewString()
	var item UserRecord
	err := s.db.QueryRow(ctx, `INSERT INTO users(id,email,password_hash,display_name,timezone) VALUES($1,$2,$3,$4,$5) RETURNING id,email,password_hash,display_name,timezone,created_at,updated_at`, id, email, passwordHash, displayName, timezone).Scan(&item.ID, &item.Email, &item.PasswordHash, &item.DisplayName, &item.Timezone, &item.CreatedAt, &item.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return item, ErrConflict
	}
	return item, err
}
func (s *Store) GetUserByEmail(ctx context.Context, email string) (UserRecord, error) {
	var item UserRecord
	err := s.db.QueryRow(ctx, `SELECT id,email,password_hash,display_name,timezone,created_at,updated_at FROM users WHERE lower(email)=lower($1)`, email).Scan(&item.ID, &item.Email, &item.PasswordHash, &item.DisplayName, &item.Timezone, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}
func (s *Store) UpdateUserProfile(ctx context.Context, userID, displayName, timezone string) (domain.User, error) {
	var item domain.User
	var name *string
	if displayName != "" {
		name = &displayName
	}
	err := s.db.QueryRow(ctx, `UPDATE users SET display_name=$2,timezone=$3,updated_at=now() WHERE id=$1 RETURNING id,email,display_name,timezone,created_at,updated_at`, userID, name, timezone).Scan(&item.ID, &item.Email, &item.DisplayName, &item.Timezone, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}
func (s *Store) GetUserTimezone(ctx context.Context, userID string) (string, error) {
	var value string
	err := s.db.QueryRow(ctx, `SELECT timezone FROM users WHERE id=$1`, userID).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}
func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash, csrfHash []byte, expires time.Time) (string, error) {
	id := uuid.NewString()
	_, err := s.db.Exec(ctx, `INSERT INTO sessions(id,user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,$4,$5)`, id, userID, tokenHash, csrfHash, expires)
	return id, err
}
func (s *Store) GetSession(ctx context.Context, tokenHash []byte, now time.Time) (SessionRecord, error) {
	var item SessionRecord
	err := s.db.QueryRow(ctx, `SELECT s.id,u.id,u.email,u.display_name,u.timezone,u.created_at,u.updated_at,s.csrf_token_hash,s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>$2`, tokenHash, now).Scan(&item.ID, &item.User.ID, &item.User.Email, &item.User.DisplayName, &item.User.Timezone, &item.User.CreatedAt, &item.User.UpdatedAt, &item.CSRFTokenHash, &item.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	if err == nil {
		_, _ = s.db.Exec(ctx, `UPDATE sessions SET last_used_at=$2 WHERE id=$1`, item.ID, now)
	}
	return item, err
}
func (s *Store) RotateSessionCSRF(ctx context.Context, sessionID string, csrfHash []byte) error {
	tag, err := s.db.Exec(ctx, `UPDATE sessions SET csrf_token_hash=$2 WHERE id=$1`, sessionID, csrfHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash)
	return err
}
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE expires_at<=$1`, now)
	return err
}
func (s *Store) Ping(ctx context.Context) error { return s.db.Ping(ctx) }
func (s *Store) CountActiveSessions(ctx context.Context, now time.Time) int64 {
	var count int64
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE expires_at>$1`, now).Scan(&count)
	return count
}
