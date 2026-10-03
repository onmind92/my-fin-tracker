package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"finance-tracker/backend/internal/domain"
	"finance-tracker/backend/internal/store"
	"golang.org/x/crypto/argon2"
)

var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrInvalidSession = errors.New("invalid or expired session")
var ErrInvalidCSRF = errors.New("invalid csrf token")

type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

type Service struct {
	store           *store.Store
	sessionDuration time.Duration
}
type Tokens struct {
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
	SessionID    string
}
type Authenticated struct {
	User      domain.User
	SessionID string
	CSRFHash  []byte
	ExpiresAt time.Time
}

func New(st *store.Store, duration time.Duration) *Service {
	return &Service{store: st, sessionDuration: duration}
}
func NormalizeEmail(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || len(value) > 254 {
		return "", ValidationError{"invalid email"}
	}
	return value, nil
}
func ValidateTimezone(value string) error {
	if strings.TrimSpace(value) == "" {
		return ValidationError{"timezone is required"}
	}
	_, err := time.LoadLocation(value)
	if err != nil {
		return ValidationError{"timezone must be a valid IANA name"}
	}
	return nil
}
func ValidatePassword(value string) error {
	if len(value) < 12 {
		return ValidationError{"password must contain at least 12 characters"}
	}
	if len(value) > 128 {
		return ValidationError{"password must contain at most 128 characters"}
	}
	return nil
}

func (s *Service) Register(ctx context.Context, email, password string, displayName *string, timezone string, now time.Time) (domain.User, Tokens, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return domain.User{}, Tokens{}, err
	}
	if err = ValidatePassword(password); err != nil {
		return domain.User{}, Tokens{}, err
	}
	if err = ValidateTimezone(timezone); err != nil {
		return domain.User{}, Tokens{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return domain.User{}, Tokens{}, err
	}
	record, err := s.store.CreateUser(ctx, normalized, hash, displayName, timezone)
	if err != nil {
		return domain.User{}, Tokens{}, err
	}
	tokens, err := s.issue(ctx, record.ID, now)
	return record.User, tokens, err
}
func (s *Service) Login(ctx context.Context, email, password string, now time.Time) (domain.User, Tokens, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		dummyPasswordWork(password)
		return domain.User{}, Tokens{}, ErrInvalidCredentials
	}
	record, err := s.store.GetUserByEmail(ctx, normalized)
	if err != nil {
		dummyPasswordWork(password)
		if errors.Is(err, store.ErrNotFound) {
			return domain.User{}, Tokens{}, ErrInvalidCredentials
		}
		return domain.User{}, Tokens{}, err
	}
	if !VerifyPassword(record.PasswordHash, password) {
		return domain.User{}, Tokens{}, ErrInvalidCredentials
	}
	tokens, err := s.issue(ctx, record.ID, now)
	return record.User, tokens, err
}
func (s *Service) Authenticate(ctx context.Context, raw string, now time.Time) (Authenticated, error) {
	if raw == "" {
		return Authenticated{}, ErrInvalidSession
	}
	record, err := s.store.GetSession(ctx, hashToken(raw), now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Authenticated{}, ErrInvalidSession
		}
		return Authenticated{}, err
	}
	return Authenticated{User: record.User, SessionID: record.ID, CSRFHash: record.CSRFTokenHash, ExpiresAt: record.ExpiresAt}, nil
}
func (s *Service) RotateCSRF(ctx context.Context, sessionID string) (string, error) {
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	if err = s.store.RotateSessionCSRF(ctx, sessionID, hash); err != nil {
		return "", err
	}
	return raw, nil
}
func (s *Service) VerifyCSRF(session Authenticated, raw string) error {
	if raw == "" || subtle.ConstantTimeCompare(session.CSRFHash, hashToken(raw)) != 1 {
		return ErrInvalidCSRF
	}
	return nil
}
func (s *Service) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, hashToken(raw))
}
func (s *Service) UpdateProfile(ctx context.Context, userID, displayName, timezone string) (domain.User, error) {
	displayName = strings.TrimSpace(displayName)
	if err := ValidateTimezone(timezone); err != nil {
		return domain.User{}, err
	}
	return s.store.UpdateUserProfile(ctx, userID, displayName, timezone)
}
func (s *Service) issue(ctx context.Context, userID string, now time.Time) (Tokens, error) {
	sessionRaw, sessionHash, err := newToken()
	if err != nil {
		return Tokens{}, err
	}
	csrfRaw, csrfHash, err := newToken()
	if err != nil {
		return Tokens{}, err
	}
	expires := now.Add(s.sessionDuration)
	id, err := s.store.CreateSession(ctx, userID, sessionHash, csrfHash, expires)
	return Tokens{SessionToken: sessionRaw, CSRFToken: csrfRaw, ExpiresAt: expires, SessionID: id}, err
}
func newToken() (string, []byte, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(buffer)
	return raw, hashToken(raw), nil
}
func hashToken(raw string) []byte { sum := sha256.Sum256([]byte(raw)); return sum[:] }

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
func SessionValid(expires, now time.Time) bool { return expires.After(now) }
func dummyPasswordWork(password string) {
	_ = argon2.IDKey([]byte(password), []byte("0123456789abcdef"), 2, 19*1024, 1, 32)
}
