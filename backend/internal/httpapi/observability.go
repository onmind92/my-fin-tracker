package httpapi

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type requestIDKey struct{}

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func requestID(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey{}).(string)
	return value
}
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !safeRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	userID string
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		path := chi.RouteContext(r.Context()).RoutePattern()
		if path == "" {
			path = "unmatched"
		}
		duration := time.Since(started)
		s.metrics.ObserveHTTP(r.Method, path, status, duration)
		s.log.Info("request", "request_id", requestID(r), "method", r.Method, "path", path, "status", status, "duration_ms", duration.Milliseconds(), "user_id", wrapped.userID)
	})
}
