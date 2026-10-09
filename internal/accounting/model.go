package accounting

import (
	"time"

	"simplenext/internal/core/types"
)

// Account merepresentasikan satu baris dalam Bagan Akun (Chart of Accounts)
type Account struct {
	Name          string     `json:"name"`
	AccountNumber string     `json:"account_number"`
	AccountName   string     `json:"account_name"`
	AccountType   string     `json:"account_type"`
	RootType      string     `json:"root_type"`
	IsGroup       bool       `json:"is_group"`
	ParentAccount *string    `json:"parent_account"`
	CreatedAt     time.Time  `json:"created_at"`
	Children      []*Account `json:"children,omitempty"`
}

// JournalEntryAccount merepresentasikan satu baris detail debit/kredit (Child Table)
type JournalEntryAccount struct {
	ID         string  `json:"id"`
	Parent     string  `json:"parent"`
	Idx        int     `json:"idx"`
	Account    string  `json:"account"`
	Debit      float64 `json:"debit"`
	Credit     float64 `json:"credit"`
	UserRemark string  `json:"user_remark"`
}

// JournalEntry merepresentasikan dokumen voucher jurnal (Header)
type JournalEntry struct {
	Name        string                `json:"name"`
	PostingDate time.Time             `json:"posting_date"`
	DocStatus   types.DocStatus       `json:"docstatus"`
	TotalDebit  float64               `json:"total_debit"`
	TotalCredit float64               `json:"total_credit"`
	UserRemark  string                `json:"user_remark"`
	Accounts    []JournalEntryAccount `json:"accounts"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// GLEntry merepresentasikan baris buku besar permanen (General Ledger)
type GLEntry struct {
	ID          string    `json:"id"`
	PostingDate time.Time `json:"posting_date"`
	Account     string    `json:"account"`
	Debit       float64   `json:"debit"`
	Credit      float64   `json:"credit"`
	VoucherType string    `json:"voucher_type"`
	VoucherNo   string    `json:"voucher_no"`
	IsCancelled bool      `json:"is_cancelled"`
	Remarks     string    `json:"remarks"`
	CreatedAt   time.Time `json:"created_at"`
}

// DTO Requests & Responses
type CreateJournalEntryItemRequest struct {
	Account    string  `json:"account"`
	Debit      float64 `json:"debit"`
	Credit     float64 `json:"credit"`
	UserRemark string  `json:"user_remark"`
}

type CreateJournalEntryRequest struct {
	PostingDate string                          `json:"posting_date"` // YYYY-MM-DD
	UserRemark  string                          `json:"user_remark"`
	Accounts    []CreateJournalEntryItemRequest `json:"accounts"`
}

// TrialBalanceRow merepresentasikan baris dalam laporan Neraca Saldo
type TrialBalanceRow struct {
	Account     string  `json:"account"`
	AccountName string  `json:"account_name"`
	RootType    string  `json:"root_type"`
	TotalDebit  float64 `json:"total_debit"`
	TotalCredit float64 `json:"total_credit"`
	NetBalance  float64 `json:"net_balance"` // Debit - Credit untuk Aset/Beban; Credit - Debit untuk Kewajiban/Ekuitas/Pendapatan
}
