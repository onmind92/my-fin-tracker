package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"finance-tracker/backend/internal/exportdata"
	"finance-tracker/backend/internal/importdata"
	"finance-tracker/backend/internal/store"
	"github.com/google/uuid"
)

type importSession struct {
	UserID, Kind string
	Expires      time.Time
	CSV          []byte
	Archive      *exportdata.Archive
}
type importSessions struct {
	mu     sync.Mutex
	values map[string]importSession
}

func newImportSessions() *importSessions { return &importSessions{values: map[string]importSession{}} }
func (m *importSessions) put(value importSession) string {
	m.mu.Lock()
	now := time.Now()
	for key, item := range m.values {
		if item.Expires.Before(now) {
			delete(m.values, key)
		}
	}
	token := uuid.NewString()
	value.Expires = now.Add(15 * time.Minute)
	m.values[token] = value
	m.mu.Unlock()
	time.AfterFunc(15*time.Minute, func() { m.remove(token) })
	return token
}
func (m *importSessions) get(token, userID, kind string) (importSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.values[token]
	if !ok || value.UserID != userID || value.Kind != kind || value.Expires.Before(time.Now()) {
		delete(m.values, token)
		return importSession{}, false
	}
	return value, true
}
func (m *importSessions) remove(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, token)
}

type csvPreviewRequest struct {
	Token   string             `json:"token"`
	Mapping importdata.Mapping `json:"mapping"`
}
type csvRowPreview struct {
	Index       int       `json:"index"`
	Date        time.Time `json:"date"`
	Type        string    `json:"type"`
	AmountMinor int64     `json:"amount_minor"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Status      string    `json:"status"`
}

func readLimited(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
}
func (s *Server) csvPreview(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var request csvPreviewRequest
		if !decode(w, r, &request) {
			return
		}
		session, ok := s.imports.get(request.Token, userID(r), "csv")
		if !ok {
			writeJSON(w, http.StatusGone, map[string]string{"error": "import preview expired"})
			return
		}
		headers, rows, err := importdata.Read(session.CSV)
		if err != nil {
			s.fail(w, r, store.ValidationError{Message: err.Error()})
			return
		}
		parsed, issues := importdata.Parse(headers, rows, request.Mapping, userNow(r).Location())
		preview := make([]csvRowPreview, 0, len(rows))
		seen := map[int]bool{}
		account, accountErr := s.store.GetAccount(r.Context(), userID(r), request.Mapping.AccountID)
		if accountErr != nil || !account.IsActive {
			issues = append(issues, "selected account is invalid or inactive")
		}
		for _, item := range parsed {
			seen[item.Index] = true
			description := item.Description
			categoryID, resolveErr := s.store.ResolveImportCategory(r.Context(), userID(r), item.Category, item.Type)
			status := "new"
			if accountErr != nil || !account.IsActive {
				status = "invalid"
			}
			if resolveErr != nil {
				status = "invalid"
				issues = append(issues, resolveErr.Error())
			}
			input := store.ImportTransaction{SourceIndex: item.Index, AccountID: request.Mapping.AccountID, CategoryID: categoryID, Type: item.Type, AmountMinor: item.AmountMinor, OccurredAt: item.Date}
			if description != "" {
				input.Description = &description
			}
			if status == "new" {
				duplicate, duplicateErr := s.store.IsPossibleDuplicate(r.Context(), userID(r), input)
				if duplicateErr != nil {
					s.fail(w, r, duplicateErr)
					return
				}
				if duplicate {
					status = "possible_duplicate"
				}
			}
			preview = append(preview, csvRowPreview{Index: item.Index, Date: item.Date, Type: item.Type, AmountMinor: item.AmountMinor, Description: item.Description, Category: item.Category, Status: status})
		}
		for index := range rows {
			if !seen[index] {
				preview = append(preview, csvRowPreview{Index: index, Status: "invalid"})
			}
		}
		sort.Slice(preview, func(i, j int) bool { return preview[i].Index < preview[j].Index })
		writeJSON(w, http.StatusOK, map[string]any{"token": request.Token, "headers": headers, "rows": preview, "issues": issues})
		return
	}
	data, err := readLimited(w, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "CSV is too large or unreadable"})
		return
	}
	headers, rows, err := importdata.Read(data)
	if err != nil {
		s.fail(w, r, store.ValidationError{Message: err.Error()})
		return
	}
	token := s.imports.put(importSession{UserID: userID(r), Kind: "csv", CSV: data})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "headers": headers, "rows": importdata.PreviewRows(rows, 10), "expires_in_seconds": 900})
}
func (s *Server) csvConfirm(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token      string             `json:"token"`
		Mapping    importdata.Mapping `json:"mapping"`
		RowIndexes []int              `json:"row_indexes"`
	}
	if !decode(w, r, &request) {
		return
	}
	session, ok := s.imports.get(request.Token, userID(r), "csv")
	if !ok {
		writeJSON(w, http.StatusGone, map[string]string{"error": "import preview expired"})
		return
	}
	headers, rows, err := importdata.Read(session.CSV)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	parsed, _ := importdata.Parse(headers, rows, request.Mapping, userNow(r).Location())
	selected := map[int]bool{}
	for _, index := range request.RowIndexes {
		selected[index] = true
	}
	items := []store.ImportTransaction{}
	for _, item := range parsed {
		if !selected[item.Index] {
			continue
		}
		categoryID, resolveErr := s.store.ResolveImportCategory(r.Context(), userID(r), item.Category, item.Type)
		if resolveErr != nil {
			s.fail(w, r, resolveErr)
			return
		}
		description := item.Description
		input := store.ImportTransaction{SourceIndex: item.Index, AccountID: request.Mapping.AccountID, CategoryID: categoryID, Type: item.Type, AmountMinor: item.AmountMinor, OccurredAt: item.Date}
		if description != "" {
			input.Description = &description
		}
		items = append(items, input)
	}
	count, err := s.store.ImportTransactions(r.Context(), userID(r), items)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.imports.remove(request.Token)
	writeJSON(w, http.StatusCreated, map[string]int{"imported": count})
}
func (s *Server) restorePreview(w http.ResponseWriter, r *http.Request) {
	data, err := readLimited(w, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "backup is too large or unreadable"})
		return
	}
	var archive exportdata.Archive
	if err = json.Unmarshal(data, &archive); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid JSON backup"})
		return
	}
	if archive.ExportVersion != 1 || archive.ArchiveID == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "unsupported or legacy export archive"})
		return
	}
	if _, err = uuid.Parse(archive.ArchiveID); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid archive_id"})
		return
	}
	token := s.imports.put(importSession{UserID: userID(r), Kind: "restore", Archive: &archive})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "archive_id": archive.ArchiveID, "already_imported": s.store.RestoreImported(r.Context(), userID(r), archive.ArchiveID), "counts": map[string]int{"accounts": len(archive.Data.Accounts), "transactions": len(archive.Data.Transactions), "transfers": len(archive.Data.Transfers), "goals": len(archive.Data.Goals), "templates": len(archive.Data.TransactionTemplates), "recurring": len(archive.Data.RecurringTransactions), "budgets": len(archive.Data.Budgets)}})
}
func (s *Server) restoreConfirm(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &request) {
		return
	}
	session, ok := s.imports.get(request.Token, userID(r), "restore")
	if !ok || session.Archive == nil {
		writeJSON(w, http.StatusGone, map[string]string{"error": "restore preview expired"})
		return
	}
	result, err := s.store.RestoreArchive(r.Context(), userID(r), session.Archive.ArchiveID, session.Archive.Data)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "this archive was already imported"})
			return
		}
		s.fail(w, r, err)
		return
	}
	s.imports.remove(request.Token)
	writeJSON(w, http.StatusCreated, result)
}
