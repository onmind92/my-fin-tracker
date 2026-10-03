package httpapi

import (
	"net/http"

	"finance-tracker/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) listTransfers(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListTransfers(r.Context(), userID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) getTransfer(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.GetTransfer(r.Context(), userID(r), chi.URLParam(r, "id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) createTransfer(w http.ResponseWriter, r *http.Request) {
	var in store.TransferInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.CreateTransfer(r.Context(), userID(r), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) updateTransfer(w http.ResponseWriter, r *http.Request) {
	var in store.TransferInput
	if !decode(w, r, &in) {
		return
	}
	item, err := s.store.UpdateTransfer(r.Context(), userID(r), chi.URLParam(r, "id"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) deleteTransfer(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTransfer(r.Context(), userID(r), chi.URLParam(r, "id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) accountHistory(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("range")
	if name == "" {
		name = "6m"
	}
	if !validRange(name) {
		s.fail(w, r, store.ValidationError{Message: "range must be 1m, 3m, 6m, 1y or all"})
		return
	}
	items, err := s.store.AccountHistory(r.Context(), userID(r), chi.URLParam(r, "id"), name, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
