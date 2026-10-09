package accounting

import (
	"context"
	"fmt"
	"math"
	"time"

	"simplenext/internal/core/naming"
	"simplenext/internal/core/types"
)

type Service struct {
	repo          *Repository
	namingService *naming.Service
}

func NewService(repo *Repository, namingService *naming.Service) *Service {
	return &Service{
		repo:          repo,
		namingService: namingService,
	}
}

// BuildAccountTree menyusun daftar akun datar menjadi struktur pohon (Tree) berdasarkan ParentAccount
func BuildAccountTree(accounts []Account) []*Account {
	accountMap := make(map[string]*Account)
	for i := range accounts {
		acc := accounts[i]
		accountMap[acc.Name] = &acc
	}

	var rootAccounts []*Account
	for _, acc := range accountMap {
		if acc.ParentAccount == nil || *acc.ParentAccount == "" {
			rootAccounts = append(rootAccounts, acc)
		} else {
			parent, exists := accountMap[*acc.ParentAccount]
			if exists {
				parent.Children = append(parent.Children, acc)
			} else {
				rootAccounts = append(rootAccounts, acc)
			}
		}
	}
	return rootAccounts
}

// ValidateDebitCreditBalance memeriksa validitas baris akun transaksi jurnal dan memastikan Total Debit == Total Credit
func ValidateDebitCreditBalance(items []CreateJournalEntryItemRequest) (totalDebit float64, totalCredit float64, err error) {
	if len(items) < 2 {
		return 0, 0, fmt.Errorf("jurnal harus memiliki minimal 2 baris akun")
	}

	for i, item := range items {
		if item.Account == "" {
			return 0, 0, fmt.Errorf("baris ke-%d: akun tidak boleh kosong", i+1)
		}
		if item.Debit < 0 || item.Credit < 0 {
			return 0, 0, fmt.Errorf("baris ke-%d: nilai debit atau kredit tidak boleh negatif", i+1)
		}
		if item.Debit == 0 && item.Credit == 0 {
			return 0, 0, fmt.Errorf("baris ke-%d: salah satu dari debit atau kredit harus lebih besar dari 0", i+1)
		}
		if item.Debit > 0 && item.Credit > 0 {
			return 0, 0, fmt.Errorf("baris ke-%d: satu baris tidak boleh memiliki nilai debit dan kredit sekaligus", i+1)
		}

		totalDebit += item.Debit
		totalCredit += item.Credit
	}

	if math.Abs(totalDebit-totalCredit) > 0.0001 {
		return totalDebit, totalCredit, fmt.Errorf("jurnal tidak seimbang: Total Debit (%.2f) != Total Kredit (%.2f)", totalDebit, totalCredit)
	}

	return totalDebit, totalCredit, nil
}

// CalculateNetBalance menghitung saldo bersih (Net Balance) akun berdasarkan jenis root normal balance
func CalculateNetBalance(rootType string, totalDebit, totalCredit float64) float64 {
	if rootType == "Asset" || rootType == "Expense" {
		return totalDebit - totalCredit
	}
	return totalCredit - totalDebit
}

// GetAccountTree mengambil seluruh akun dalam format pohon (hierarki)
func (s *Service) GetAccountTree(ctx context.Context) ([]*Account, error) {
	accounts, err := s.repo.GetAllAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return BuildAccountTree(accounts), nil
}

// CreateJournalEntry memvalidasi dan menyimpan draft Journal Entry
func (s *Service) CreateJournalEntry(ctx context.Context, req *CreateJournalEntryRequest) (*JournalEntry, error) {
	totalDebit, totalCredit, err := ValidateDebitCreditBalance(req.Accounts)
	if err != nil {
		return nil, err
	}

	postingDate := time.Now()
	if req.PostingDate != "" {
		parsedDate, err := time.Parse("2006-01-02", req.PostingDate)
		if err != nil {
			return nil, fmt.Errorf("format posting_date tidak valid (gunakan YYYY-MM-DD): %w", err)
		}
		postingDate = parsedDate
	}

	var childRows []JournalEntryAccount

	for i, item := range req.Accounts {
		// Validasi akun di database: tidak boleh akun grup
		acc, err := s.repo.GetAccountByName(ctx, item.Account)
		if err != nil {
			return nil, fmt.Errorf("gagal memeriksa akun '%s': %w", item.Account, err)
		}
		if acc == nil {
			return nil, fmt.Errorf("akun '%s' tidak ditemukan dalam sistem", item.Account)
		}
		if acc.IsGroup {
			return nil, fmt.Errorf("transaksi tidak dapat dilakukan pada akun grup/induk ('%s')", item.Account)
		}

		childRows = append(childRows, JournalEntryAccount{
			Idx:        i + 1,
			Account:    item.Account,
			Debit:      item.Debit,
			Credit:     item.Credit,
			UserRemark: item.UserRemark,
		})
	}

	// Generate nomor dokumen menggunakan Naming Series ala ERPNext
	voucherNo, err := s.namingService.Generate(ctx, "JV-.YYYY.-.#####")
	if err != nil {
		return nil, fmt.Errorf("gagal generate nomor voucher: %w", err)
	}

	entry := &JournalEntry{
		Name:        voucherNo,
		PostingDate: postingDate,
		DocStatus:   types.DocStatusDraft,
		TotalDebit:  totalDebit,
		TotalCredit: totalCredit,
		UserRemark:  req.UserRemark,
		Accounts:    childRows,
	}

	if err := s.repo.CreateJournalEntry(ctx, entry); err != nil {
		return nil, err
	}

	return s.repo.GetJournalEntryByName(ctx, voucherNo)
}

// GetJournalEntry mengambil data voucher jurnal
func (s *Service) GetJournalEntry(ctx context.Context, name string) (*JournalEntry, error) {
	entry, err := s.repo.GetJournalEntryByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("dokumen '%s' tidak ditemukan", name)
	}
	return entry, nil
}

// SubmitJournalEntry melakukan submit dan posting ke Buku Besar
func (s *Service) SubmitJournalEntry(ctx context.Context, name string) error {
	return s.repo.SubmitJournalEntry(ctx, name)
}

// CancelJournalEntry membatalkan voucher dan memposting reversal entry
func (s *Service) CancelJournalEntry(ctx context.Context, name string) error {
	return s.repo.CancelJournalEntry(ctx, name)
}

// GetGeneralLedger mengambil transaksi buku besar
func (s *Service) GetGeneralLedger(ctx context.Context, account string) ([]GLEntry, error) {
	return s.repo.GetGeneralLedger(ctx, account)
}

// GetTrialBalance mengambil laporan Neraca Saldo
func (s *Service) GetTrialBalance(ctx context.Context) ([]TrialBalanceRow, error) {
	return s.repo.GetTrialBalance(ctx)
}
