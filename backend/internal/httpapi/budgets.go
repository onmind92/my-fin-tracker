package httpapi

import (
	"net/http"

	"finance-tracker/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) listBudgets(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListBudgets(r.Context(), userID(r), userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) getBudget(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.GetBudget(r.Context(), userID(r), chi.URLParam(r, "id"), userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) createBudget(w http.ResponseWriter, r *http.Request) {
	var in store.BudgetInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.CreateBudget(r.Context(), userID(r), in, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) updateBudget(w http.ResponseWriter, r *http.Request) {
	var in store.BudgetInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.UpdateBudget(r.Context(), userID(r), chi.URLParam(r, "id"), in, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) deleteBudget(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DisableBudget(r.Context(), userID(r), chi.URLParam(r, "id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
