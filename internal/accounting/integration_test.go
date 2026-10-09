package accounting_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"simplenext/internal/accounting"
	"simplenext/internal/core/naming"
	"simplenext/internal/core/types"
	"simplenext/internal/platform/database"
)

func setupTestRouter(t *testing.T) (*chi.Mux, context.Context) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/simplenext?sslmode=disable"
	}

	ctx := context.Background()
	dbPool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Skipf("Melewati integration test karena PostgreSQL tidak tersedia: %v", err)
	}

	namingService := naming.NewService(dbPool)
	repo := accounting.NewRepository(dbPool)
	service := accounting.NewService(repo, namingService)
	handler := accounting.NewHandler(service)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	return r, ctx
}

// TestEndToEndJournalEntryLifecycle menguji skenario lengkap Langkah 5.2:
// 1. Buat Draft Journal Entry (Kas berkurang, Beban bertambah).
// 2. Coba Submit dengan saldo tidak balance -> Harus gagal (400 Bad Request).
// 3. Submit dengan saldo balance -> Berhasil, cek saldo di gl_entries.
// 4. Cancel Journal Entry -> Berhasil, pastikan saldo kembali ke posisi 0 di laporan buku besar.
func TestEndToEndJournalEntryLifecycle(t *testing.T) {
	router, _ := setupTestRouter(t)

	// =========================================================================
	// Langkah 5.2.2 (Uji Validasi): Kirim transaksi tidak balance -> Harus 400 Bad Request
	// =========================================================================
	t.Run("Step 2: Coba kirim saldo tidak balance (Debit != Credit)", func(t *testing.T) {
		unbalancedReq := accounting.CreateJournalEntryRequest{
			PostingDate: time.Now().Format("2006-01-02"),
			UserRemark:  "Transaksi Tidak Balance",
			Accounts: []accounting.CreateJournalEntryItemRequest{
				{Account: "5130 - Beban Listrik dan Air", Debit: 500000, Credit: 0},
				{Account: "1111 - Kas Kecil", Debit: 0, Credit: 300000}, // Selisih 200.000
			},
		}

		body, _ := json.Marshal(unbalancedReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/journal-entries", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("diharapkan status 400 Bad Request, didapat %d: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if errMsg, ok := resp["error"].(string); !ok || errMsg == "" {
			t.Errorf("respons harus memuat pesan error: %v", resp)
		}
	})

	// =========================================================================
	// Langkah 5.2.1: Buat Draft Journal Entry (Kas berkurang / Kredit, Beban bertambah / Debit)
	// =========================================================================
	var voucherName string
	testAmount := 750000.0

	t.Run("Step 1: Buat Draft Journal Entry", func(t *testing.T) {
		draftReq := accounting.CreateJournalEntryRequest{
			PostingDate: time.Now().Format("2006-01-02"),
			UserRemark:  "Pembayaran Listrik Operasional",
			Accounts: []accounting.CreateJournalEntryItemRequest{
				{
					Account:    "5130 - Beban Listrik dan Air",
					Debit:      testAmount,
					Credit:     0,
					UserRemark: "Beban listrik bulan berjalan",
				},
				{
					Account:    "1111 - Kas Kecil",
					Debit:      0,
					Credit:     testAmount,
					UserRemark: "Kas kecil berkurang",
				},
			},
		}

		body, _ := json.Marshal(draftReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/journal-entries", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("gagal membuat draft, status %d: %s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Message string                   `json:"message"`
			Data    *accounting.JournalEntry `json:"data"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("gagal unmarshal response: %v", err)
		}

		if resp.Data == nil {
			t.Fatalf("data voucher kosong dalam response")
		}

		voucherName = resp.Data.Name
		if voucherName == "" {
			t.Fatalf("voucher name tidak boleh kosong")
		}

		if resp.Data.DocStatus != types.DocStatusDraft {
			t.Errorf("status dokumen baru harus Draft (0), didapat: %d", resp.Data.DocStatus)
		}

		if resp.Data.TotalDebit != testAmount || resp.Data.TotalCredit != testAmount {
			t.Errorf("total debit/credit salah: Debit=%.2f, Credit=%.2f", resp.Data.TotalDebit, resp.Data.TotalCredit)
		}

		// Pastikan saat draft, belum ada record di general ledger
		glReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports/general-ledger?account="+url.QueryEscape("1111 - Kas Kecil"), nil)
		glRr := httptest.NewRecorder()
		router.ServeHTTP(glRr, glReq)

		var glResp struct {
			Data []accounting.GLEntry `json:"data"`
		}
		_ = json.Unmarshal(glRr.Body.Bytes(), &glResp)
		for _, row := range glResp.Data {
			if row.VoucherNo == voucherName {
				t.Errorf("voucher berstatus Draft tidak boleh muncul di gl_entries!")
			}
		}
	})

	// =========================================================================
	// Langkah 5.2.3: Submit dengan saldo balance -> Berhasil, cek saldo di gl_entries
	// =========================================================================
	t.Run("Step 3: Submit Journal Entry dan Verifikasi GL", func(t *testing.T) {
		if voucherName == "" {
			t.Fatal("voucherName belum dibuat pada Step 1")
		}

		submitURL := fmt.Sprintf("/api/v1/journal-entries/%s/submit", voucherName)
		req := httptest.NewRequest(http.MethodPost, submitURL, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("gagal submit voucher %s, status %d: %s", voucherName, rr.Code, rr.Body.String())
		}

		var submitResp struct {
			Message string                   `json:"message"`
			Data    *accounting.JournalEntry `json:"data"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &submitResp); err != nil {
			t.Fatalf("gagal unmarshal respons submit: %v", err)
		}

		if submitResp.Data.DocStatus != types.DocStatusSubmitted {
			t.Errorf("status setelah submit harus Submitted (1), didapat: %d", submitResp.Data.DocStatus)
		}

		// Verifikasi posting di General Ledger untuk Kas Kecil (Credit)
		glKasReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports/general-ledger?account="+url.QueryEscape("1111 - Kas Kecil"), nil)
		glKasRr := httptest.NewRecorder()
		router.ServeHTTP(glKasRr, glKasReq)

		var glKasResp struct {
			Data []accounting.GLEntry `json:"data"`
		}
		_ = json.Unmarshal(glKasRr.Body.Bytes(), &glKasResp)

		var kasEntryFound bool
		for _, entry := range glKasResp.Data {
			if entry.VoucherNo == voucherName && !entry.IsCancelled {
				kasEntryFound = true
				if entry.Credit != testAmount {
					t.Errorf("nilai kredit di GL Kas Kecil salah: diharapkan %.2f, didapat %.2f", testAmount, entry.Credit)
				}
				if entry.Debit != 0 {
					t.Errorf("nilai debit di GL Kas Kecil harus 0, didapat %.2f", entry.Debit)
				}
			}
		}
		if !kasEntryFound {
			t.Errorf("posting GL untuk akun Kas Kecil tidak ditemukan untuk voucher %s", voucherName)
		}

		// Verifikasi posting di General Ledger untuk Beban Listrik (Debit)
		glBebanReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports/general-ledger?account="+url.QueryEscape("5130 - Beban Listrik dan Air"), nil)
		glBebanRr := httptest.NewRecorder()
		router.ServeHTTP(glBebanRr, glBebanReq)

		var glBebanResp struct {
			Data []accounting.GLEntry `json:"data"`
		}
		_ = json.Unmarshal(glBebanRr.Body.Bytes(), &glBebanResp)

		var bebanEntryFound bool
		for _, entry := range glBebanResp.Data {
			if entry.VoucherNo == voucherName && !entry.IsCancelled {
				bebanEntryFound = true
				if entry.Debit != testAmount {
					t.Errorf("nilai debit di GL Beban Listrik salah: diharapkan %.2f, didapat %.2f", testAmount, entry.Debit)
				}
				if entry.Credit != 0 {
					t.Errorf("nilai kredit di GL Beban Listrik harus 0, didapat %.2f", entry.Credit)
				}
			}
		}
		if !bebanEntryFound {
			t.Errorf("posting GL untuk akun Beban Listrik tidak ditemukan untuk voucher %s", voucherName)
		}
	})

	// =========================================================================
	// Langkah 5.2.4: Cancel Journal Entry -> Berhasil, saldo kembali ke posisi 0
	// =========================================================================
	t.Run("Step 4: Cancel Journal Entry dan Verifikasi Reversal", func(t *testing.T) {
		if voucherName == "" {
			t.Fatal("voucherName belum dibuat pada Step 1")
		}

		cancelURL := fmt.Sprintf("/api/v1/journal-entries/%s/cancel", voucherName)
		req := httptest.NewRequest(http.MethodPost, cancelURL, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("gagal cancel voucher %s, status %d: %s", voucherName, rr.Code, rr.Body.String())
		}

		var cancelResp struct {
			Message string                   `json:"message"`
			Data    *accounting.JournalEntry `json:"data"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &cancelResp)

		if cancelResp.Data.DocStatus != types.DocStatusCancelled {
			t.Errorf("status setelah cancel harus Cancelled (2), didapat: %d", cancelResp.Data.DocStatus)
		}

		// Periksa jurnal balik (reversal entry) di General Ledger untuk Kas Kecil
		glKasReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports/general-ledger?account="+url.QueryEscape("1111 - Kas Kecil"), nil)
		glKasRr := httptest.NewRecorder()
		router.ServeHTTP(glKasRr, glKasReq)

		var glKasResp struct {
			Data []accounting.GLEntry `json:"data"`
		}
		_ = json.Unmarshal(glKasRr.Body.Bytes(), &glKasResp)

		var kasReversalFound bool
		var totalKasDebit, totalKasCredit float64
		for _, entry := range glKasResp.Data {
			if entry.VoucherNo == voucherName {
				totalKasDebit += entry.Debit
				totalKasCredit += entry.Credit
				if entry.IsCancelled {
					kasReversalFound = true
					// Reversal harus membalik kredit menjadi debit
					if entry.Debit != testAmount {
						t.Errorf("reversal debit untuk Kas Kecil harus %.2f, didapat %.2f", testAmount, entry.Debit)
					}
				}
			}
		}

		if !kasReversalFound {
			t.Errorf("jurnal pembalik (reversal) Kas Kecil tidak ditemukan")
		}

		// Saldo kumulatif voucher ini di Kas Kecil harus seimbang (Total Debit == Total Credit -> Net 0)
		if totalKasDebit != totalKasCredit {
			t.Errorf("setelah pembatalan, total debit dan kredit voucher %s harus seimbang: Debit=%.2f, Credit=%.2f",
				voucherName, totalKasDebit, totalKasCredit)
		}

		// Periksa Trial Balance tetap seimbang
		tbReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports/trial-balance", nil)
		tbRr := httptest.NewRecorder()
		router.ServeHTTP(tbRr, tbReq)

		var tbResp struct {
			TotalDebit  float64 `json:"total_debit"`
			TotalCredit float64 `json:"total_credit"`
			IsBalanced  bool    `json:"is_balanced"`
		}
		_ = json.Unmarshal(tbRr.Body.Bytes(), &tbResp)

		if !tbResp.IsBalanced {
			t.Errorf("Trial Balance harus tetap seimbang setelah reversal: TotalDebit=%.2f, TotalCredit=%.2f",
				tbResp.TotalDebit, tbResp.TotalCredit)
		}
	})
}
