package accounting

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mendaftarkan seluruh endpoint modul Accounting ke Chi router
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/accounts", h.ListAccounts)
	r.Post("/api/v1/journal-entries", h.CreateJournalEntry)
	r.Get("/api/v1/journal-entries/{name}", h.GetJournalEntry)
	r.Post("/api/v1/journal-entries/{name}/submit", h.SubmitJournalEntry)
	r.Post("/api/v1/journal-entries/{name}/cancel", h.CancelJournalEntry)

	r.Get("/api/v1/reports/general-ledger", h.GetGeneralLedger)
	r.Get("/api/v1/reports/trial-balance", h.GetTrialBalance)
}

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	asTree := r.URL.Query().Get("tree") == "true"

	if asTree {
		tree, err := h.service.GetAccountTree(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"data": tree})
		return
	}

	accounts, err := h.service.repo.GetAllAccounts(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": accounts})
}

func (h *Handler) CreateJournalEntry(w http.ResponseWriter, r *http.Request) {
	var req CreateJournalEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	entry, err := h.service.CreateJournalEntry(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Draft Journal Entry berhasil dibuat",
		"data":    entry,
	})
}

func (h *Handler) GetJournalEntry(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	entry, err := h.service.GetJournalEntry(r.Context(), name)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": entry})
}

func (h *Handler) SubmitJournalEntry(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.service.SubmitJournalEntry(r.Context(), name); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, _ := h.service.GetJournalEntry(r.Context(), name)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Journal Entry berhasil di-Submit dan diposting ke Buku Besar (GL)",
		"data":    entry,
	})
}

func (h *Handler) CancelJournalEntry(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.service.CancelJournalEntry(r.Context(), name); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, _ := h.service.GetJournalEntry(r.Context(), name)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Journal Entry berhasil di-Cancel dan jurnal balik (reversal) telah diposting",
		"data":    entry,
	})
}

func (h *Handler) GetGeneralLedger(w http.ResponseWriter, r *http.Request) {
	account := r.URL.Query().Get("account")
	entries, err := h.service.GetGeneralLedger(r.Context(), account)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": entries})
}

func (h *Handler) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	report, err := h.service.GetTrialBalance(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var totalDebit, totalCredit float64
	for _, row := range report {
		totalDebit += row.TotalDebit
		totalCredit += row.TotalCredit
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":         report,
		"total_debit":  totalDebit,
		"total_credit": totalCredit,
		"is_balanced":  totalDebit == totalCredit,
	})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": message,
	})
}
