package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	authservice "finance-tracker/backend/internal/auth"
	"finance-tracker/backend/internal/domain"
	"finance-tracker/backend/internal/store"
)

const sessionCookieName = "finance_session"

type contextKey string

const authContextKey contextKey = "authenticated"

type authResponse struct {
	User      domain.User `json:"user"`
	CSRFToken string      `json:"csrf_token"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email       string  `json:"email"`
		Password    string  `json:"password"`
		DisplayName *string `json:"display_name"`
		Timezone    string  `json:"timezone"`
	}
	if !decode(w, r, &input) {
		return
	}
	user, tokens, err := s.auth.Register(r.Context(), input.Email, input.Password, input.DisplayName, input.Timezone, time.Now().UTC())
	if err != nil {
		var validation authservice.ValidationError
		if errors.Is(err, store.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email is already registered"})
		} else if errors.As(err, &validation) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		} else {
			s.fail(w, r, err)
		}
		return
	}
	s.setSessionCookie(w, tokens)
	writeJSON(w, http.StatusCreated, authResponse{User: user, CSRFToken: tokens.CSRFToken})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	user, tokens, err := s.auth.Login(r.Context(), input.Email, input.Password, time.Now().UTC())
	if err != nil {
		if errors.Is(err, authservice.ErrInvalidCredentials) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		} else {
			s.fail(w, r, err)
		}
		return
	}
	s.setSessionCookie(w, tokens)
	writeJSON(w, http.StatusOK, authResponse{User: user, CSRFToken: tokens.CSRFToken})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	session := currentAuth(r)
	csrf, err := s.auth.RotateCSRF(r.Context(), session.SessionID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: session.User, CSRFToken: csrf})
}
func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName string `json:"display_name"`
		Timezone    string `json:"timezone"`
	}
	if !decode(w, r, &input) {
		return
	}
	user, err := s.auth.UpdateProfile(r.Context(), userID(r), input.DisplayName, input.Timezone)
	if err != nil {
		var validation authservice.ValidationError
		if errors.As(err, &validation) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": validation.Error()})
		} else {
			s.fail(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, user)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookieName)
	if cookie != nil {
		if err := s.auth.Logout(r.Context(), cookie.Value); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.cookieSecure, SameSite: s.cookieSameSite, MaxAge: -1, Expires: time.Unix(0, 0)})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		session, err := s.auth.Authenticate(r.Context(), cookie.Value, time.Now().UTC())
		if err != nil {
			if errors.Is(err, authservice.ErrInvalidSession) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired or invalid"})
			} else {
				s.fail(w, r, err)
			}
			return
		}
		if writer, ok := w.(*statusWriter); ok {
			writer.userID = session.User.ID
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authContextKey, session)))
	})
}
func (s *Server) csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if !s.originAllowed(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin not allowed"})
			return
		}
		if err := s.auth.VerifyCSRF(currentAuth(r), r.Header.Get("X-CSRF-Token")); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid csrf token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) setSessionCookie(w http.ResponseWriter, tokens authservice.Tokens) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: tokens.SessionToken, Path: "/", HttpOnly: true, Secure: s.cookieSecure, SameSite: s.cookieSameSite, Expires: tokens.ExpiresAt, MaxAge: int(time.Until(tokens.ExpiresAt).Seconds())})
}

func parseSameSite(value string, secure bool) http.SameSite {
	if strings.EqualFold(value, "none") && secure {
		return http.SameSiteNoneMode
	}
	if strings.EqualFold(value, "strict") {
		return http.SameSiteStrictMode
	}
	return http.SameSiteLaxMode
}

func (s *Server) originAllowed(r *http.Request) bool {
	origin := strings.TrimRight(r.Header.Get("Origin"), "/")
	return origin == "" || strings.EqualFold(origin, s.origin)
}

func (s *Server) requireKnownOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.originAllowed(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin not allowed"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
func currentAuth(r *http.Request) authservice.Authenticated {
	value, _ := r.Context().Value(authContextKey).(authservice.Authenticated)
	return value
}
func userID(r *http.Request) string { return currentAuth(r).User.ID }
func userNow(r *http.Request) time.Time {
	location, err := time.LoadLocation(currentAuth(r).User.Timezone)
	if err != nil {
		location = time.UTC
	}
	return time.Now().In(location)
}
