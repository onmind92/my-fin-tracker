package httpapi

import (
	"net/http"
	"time"

	"finance-tracker/backend/internal/exportdata"
	"finance-tracker/backend/internal/store"
)

func (s *Server) exportJSON(w http.ResponseWriter, r *http.Request) {
	archive, err := s.store.Export(r.Context(), currentAuth(r).User, userNow(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="finance-backup.json"`)
	writeJSON(w, http.StatusOK, archive)
}
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListTransactions(r.Context(), userID(r), store.TransactionFilter{Limit: 1_000_000})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	location, err := time.LoadLocation(currentAuth(r).User.Timezone)
	if err != nil {
		location = time.UTC
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="transactions.csv"`)
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	if err = exportdata.WriteTransactionsCSV(w, items, location); err != nil {
		s.log.Error("csv export failed", "error", err)
	}
}
