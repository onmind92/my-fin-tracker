package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"finance-tracker/backend/internal/auth"
	"finance-tracker/backend/internal/observability"
	"finance-tracker/backend/internal/quickentry"
	"finance-tracker/backend/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	store          *store.Store
	auth           *auth.Service
	log            *slog.Logger
	origin         string
	cookieSecure   bool
	cookieSameSite http.SameSite
	authLimiter    *rateLimiter
	imports        *importSessions
	metrics        *observability.Registry
}

func New(st *store.Store, authService *auth.Service, log *slog.Logger, origin string, cookieSecure bool, cookieSameSite, metricsToken string, metrics *observability.Registry) http.Handler {
	s := &Server{store: st, auth: authService, log: log, origin: strings.TrimRight(origin, "/"), cookieSecure: cookieSecure, cookieSameSite: parseSameSite(cookieSameSite, cookieSecure), authLimiter: newRateLimiter(10, time.Minute), imports: newImportSessions(), metrics: metrics}
	r := chi.NewRouter()
	r.Use(s.requestID, middleware.RealIP, middleware.Recoverer, s.observe, s.securityHeaders, s.cors)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/ready", s.ready)
	r.Handle("/metrics", metricsAuth(metrics.Handler(func() int64 { return st.CountActiveSessions(context.Background(), time.Now().UTC()) }), metricsToken))
	r.Route("/api/v1", func(r chi.Router) {
		r.With(s.requireKnownOrigin, s.limitAuth).Post("/auth/register", s.register)
		r.With(s.requireKnownOrigin, s.limitAuth).Post("/auth/login", s.login)
		r.Group(func(r chi.Router) {
			r.Use(s.authenticate, s.csrfProtection)
			r.Get("/auth/me", s.me)
			r.Patch("/auth/me", s.updateMe)
			r.Post("/auth/logout", s.logout)
			r.Get("/accounts", s.listAccounts)
			r.Post("/accounts", s.createAccount)
			r.Get("/accounts/{id}", s.getAccount)
			r.Get("/accounts/{id}/history", s.accountHistory)
			r.Patch("/accounts/{id}", s.updateAccount)
			r.Get("/transactions", s.listTransactions)
			r.Post("/transactions", s.createTransaction)
			r.Get("/transactions/{id}", s.getTransaction)
			r.Patch("/transactions/{id}", s.updateTransaction)
			r.Delete("/transactions/{id}", s.deleteTransaction)
			r.Get("/transfers", s.listTransfers)
			r.Post("/transfers", s.createTransfer)
			r.Get("/transfers/{id}", s.getTransfer)
			r.Patch("/transfers/{id}", s.updateTransfer)
			r.Delete("/transfers/{id}", s.deleteTransfer)
			r.Get("/budgets", s.listBudgets)
			r.Post("/budgets", s.createBudget)
			r.Get("/budgets/{id}", s.getBudget)
			r.Patch("/budgets/{id}", s.updateBudget)
			r.Delete("/budgets/{id}", s.deleteBudget)
			r.Get("/categories", s.listCategories)
			r.Get("/dashboard", s.dashboard)
			r.Get("/goals", s.listGoals)
			r.Post("/goals", s.createGoal)
			r.Get("/goals/{id}", s.getGoal)
			r.Patch("/goals/{id}", s.updateGoal)
			r.Delete("/goals/{id}", s.archiveGoal)
			r.Get("/analytics/net-worth", s.netWorthHistory)
			r.Get("/analytics/cash-flow", s.cashFlow)
			r.Get("/analytics/expenses-by-category", s.expensesByCategory)
			r.Get("/analytics/forecast", s.forecast)
			r.Get("/transaction-templates", s.listTemplates)
			r.Post("/transaction-templates", s.createTemplate)
			r.Patch("/transaction-templates/{id}", s.updateTemplate)
			r.Delete("/transaction-templates/{id}", s.deleteTemplate)
			r.Post("/quick-entry/parse", s.parseQuickEntry)
			r.Get("/recurring-transactions", s.listRecurring)
			r.Post("/recurring-transactions", s.createRecurring)
			r.Patch("/recurring-transactions/{id}", s.updateRecurring)
			r.Delete("/recurring-transactions/{id}", s.disableRecurring)
			r.Get("/recurring-transactions/{id}/executions", s.listRecurringExecutions)
			r.Get("/export/json", s.exportJSON)
			r.Get("/export/transactions.csv", s.exportCSV)
			r.Post("/import/csv/preview", s.csvPreview)
			r.Post("/import/csv/confirm", s.csvConfirm)
			r.Post("/restore/json/preview", s.restorePreview)
			r.Post("/restore/json/confirm", s.restoreConfirm)
		})
	})
	return r
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && strings.EqualFold(strings.TrimRight(origin, "/"), s.origin) {
			w.Header().Set("Access-Control-Allow-Origin", s.origin)
		}
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Content-Disposition")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var validation store.ValidationError
	switch {
	case errors.As(err, &validation):
		writeJSON(w, 422, map[string]string{"error": validation.Error()})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not found"})
	case errors.Is(err, store.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "resource already exists"})
	default:
		s.log.Error("request failed", "error", err, "path", r.URL.Path, "request_id", requestID(r))
		writeJSON(w, 500, map[string]string{"error": "internal server error"})
	}
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAccounts(r.Context(), userID(r), false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.GetAccount(r.Context(), userID(r), chi.URLParam(r, "id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var in store.CreateAccountInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.CreateAccount(r.Context(), userID(r), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request) {
	var in store.UpdateAccountInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.UpdateAccount(r.Context(), userID(r), chi.URLParam(r, "id"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func intQuery(r *http.Request, key string, def, max int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, store.ValidationError{Message: key + " must be a non-negative integer"}
	}
	if v > max {
		v = max
	}
	return v, nil
}
func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	limit, err := intQuery(r, "limit", 50, 100)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	offset, err := intQuery(r, "offset", 0, 1_000_000)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	typeFilter := r.URL.Query().Get("type")
	if typeFilter != "" && typeFilter != "income" && typeFilter != "expense" {
		s.fail(w, r, store.ValidationError{Message: "type must be income or expense"})
		return
	}
	items, err := s.store.ListTransactions(r.Context(), userID(r), store.TransactionFilter{Limit: limit, Offset: offset, AccountID: r.URL.Query().Get("account_id"), Type: typeFilter})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) getTransaction(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.GetTransaction(r.Context(), userID(r), chi.URLParam(r, "id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) {
	var in store.CreateTransactionInput
	if !decode(w, r, &in) {
		return
	}
	if in.Description != nil {
		v := strings.TrimSpace(*in.Description)
		in.Description = &v
	}
	item, err := s.store.CreateTransaction(r.Context(), userID(r), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) updateTransaction(w http.ResponseWriter, r *http.Request) {
	var in store.UpdateTransactionInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.UpdateTransaction(r.Context(), userID(r), chi.URLParam(r, "id"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTransaction(r.Context(), userID(r), chi.URLParam(r, "id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListCategories(r.Context(), userID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.Dashboard(r.Context(), userID(r), userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) listGoals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "active" && status != "completed" && status != "archived" {
		s.fail(w, r, store.ValidationError{Message: "invalid goal status"})
		return
	}
	items, err := s.store.ListGoals(r.Context(), userID(r), status, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) getGoal(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.GetGoal(r.Context(), userID(r), chi.URLParam(r, "id"), userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) createGoal(w http.ResponseWriter, r *http.Request) {
	var in store.CreateGoalInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.CreateGoal(r.Context(), userID(r), in, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) updateGoal(w http.ResponseWriter, r *http.Request) {
	var in store.UpdateGoalInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.UpdateGoal(r.Context(), userID(r), chi.URLParam(r, "id"), in, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) archiveGoal(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ArchiveGoal(r.Context(), userID(r), chi.URLParam(r, "id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validRange(value string) bool {
	return value == "1m" || value == "3m" || value == "6m" || value == "1y" || value == "all"
}
func rangeDates(r *http.Request, now time.Time) (time.Time, time.Time, error) {
	fromRaw, toRaw := r.URL.Query().Get("date_from"), r.URL.Query().Get("date_to")
	if fromRaw != "" || toRaw != "" {
		if fromRaw == "" || toRaw == "" {
			return time.Time{}, time.Time{}, store.ValidationError{Message: "date_from and date_to must be provided together"}
		}
		from, err := time.ParseInLocation("2006-01-02", fromRaw, now.Location())
		if err != nil {
			return time.Time{}, time.Time{}, store.ValidationError{Message: "date_from must use YYYY-MM-DD"}
		}
		to, err := time.ParseInLocation("2006-01-02", toRaw, now.Location())
		if err != nil {
			return time.Time{}, time.Time{}, store.ValidationError{Message: "date_to must use YYYY-MM-DD"}
		}
		to = to.AddDate(0, 0, 1)
		if !from.Before(to) {
			return time.Time{}, time.Time{}, store.ValidationError{Message: "date_from must be before date_to"}
		}
		return from, to, nil
	}
	rangeName := r.URL.Query().Get("range")
	if rangeName == "" {
		rangeName = "6m"
	}
	if !validRange(rangeName) {
		return time.Time{}, time.Time{}, store.ValidationError{Message: "range must be 1m, 3m, 6m, 1y or all"}
	}
	months := 6
	switch rangeName {
	case "1m":
		months = 1
	case "3m":
		months = 3
	case "1y":
		months = 12
	case "all":
		return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), now.AddDate(0, 0, 1), nil
	}
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -months+1, 0)
	return from, now.AddDate(0, 0, 1), nil
}
func (s *Server) netWorthHistory(w http.ResponseWriter, r *http.Request) {
	rangeName := r.URL.Query().Get("range")
	if rangeName == "" {
		rangeName = "6m"
	}
	if !validRange(rangeName) {
		s.fail(w, r, store.ValidationError{Message: "range must be 1m, 3m, 6m, 1y or all"})
		return
	}
	items, err := s.store.NetWorthHistory(r.Context(), userID(r), rangeName, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) cashFlow(w http.ResponseWriter, r *http.Request) {
	from, to, err := rangeDates(r, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, err := s.store.CashFlow(r.Context(), userID(r), from, to)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) expensesByCategory(w http.ResponseWriter, r *http.Request) {
	from, to, err := rangeDates(r, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, err := s.store.ExpensesByCategory(r.Context(), userID(r), from, to)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) forecast(w http.ResponseWriter, r *http.Request) {
	window, err := intQuery(r, "average_window_months", 6, 12)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if window != 3 && window != 6 && window != 12 {
		s.fail(w, r, store.ValidationError{Message: "average_window_months must be 3, 6 or 12"})
		return
	}
	item, err := s.store.Forecast(r.Context(), userID(r), window, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListTemplates(r.Context(), userID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createTemplate(w http.ResponseWriter, r *http.Request) {
	var in store.TemplateInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.CreateTemplate(r.Context(), userID(r), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) updateTemplate(w http.ResponseWriter, r *http.Request) {
	var in store.TemplateInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.UpdateTemplate(r.Context(), userID(r), chi.URLParam(r, "id"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTemplate(r.Context(), userID(r), chi.URLParam(r, "id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) parseQuickEntry(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &input) {
		return
	}
	categories, err := s.store.ListCategories(r.Context(), userID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	accounts, err := s.store.ListAccounts(r.Context(), userID(r), false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	catalog := quickentry.Catalog{Categories: make([]quickentry.Category, 0, len(categories)), Accounts: make([]quickentry.Account, 0, len(accounts))}
	for _, item := range categories {
		catalog.Categories = append(catalog.Categories, quickentry.Category{ID: item.ID, Name: item.Name, Type: item.Type})
	}
	for _, item := range accounts {
		catalog.Accounts = append(catalog.Accounts, quickentry.Account{ID: item.ID, Name: item.Name, Type: item.Type, Active: item.IsActive})
	}
	writeJSON(w, 200, quickentry.Parse(input.Text, catalog, userNow(r)))
}

func (s *Server) listRecurring(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRecurring(r.Context(), userID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createRecurring(w http.ResponseWriter, r *http.Request) {
	var in store.RecurringInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.CreateRecurring(r.Context(), userID(r), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) updateRecurring(w http.ResponseWriter, r *http.Request) {
	var in store.RecurringInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.UpdateRecurring(r.Context(), userID(r), chi.URLParam(r, "id"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) disableRecurring(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DisableRecurring(r.Context(), userID(r), chi.URLParam(r, "id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) listRecurringExecutions(w http.ResponseWriter, r *http.Request) {
	limit, err := intQuery(r, "limit", 20, 100)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, err := s.store.ListRecurringExecutions(r.Context(), userID(r), chi.URLParam(r, "id"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
