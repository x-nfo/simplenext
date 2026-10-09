package accounting

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"simplenext/internal/core/types"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetAllAccounts mengambil seluruh daftar akun
func (r *Repository) GetAllAccounts(ctx context.Context) ([]Account, error) {
	query := `
		SELECT name, COALESCE(account_number, ''), account_name, account_type, root_type, is_group, parent_account, created_at
		FROM accounts
		ORDER BY account_number ASC, name ASC;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		err := rows.Scan(
			&a.Name, &a.AccountNumber, &a.AccountName, &a.AccountType,
			&a.RootType, &a.IsGroup, &a.ParentAccount, &a.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// GetAccountByName mengambil detail akun berdasarkan nama uniknya
func (r *Repository) GetAccountByName(ctx context.Context, name string) (*Account, error) {
	query := `
		SELECT name, COALESCE(account_number, ''), account_name, account_type, root_type, is_group, parent_account, created_at
		FROM accounts
		WHERE name = $1;
	`
	var a Account
	err := r.db.QueryRow(ctx, query, name).Scan(
		&a.Name, &a.AccountNumber, &a.AccountName, &a.AccountType,
		&a.RootType, &a.IsGroup, &a.ParentAccount, &a.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("gagal query account by name: %w", err)
	}
	return &a, nil
}

// CreateJournalEntry menyimpan draft voucher jurnal dan child items-nya dalam transaksi DB
func (r *Repository) CreateJournalEntry(ctx context.Context, entry *JournalEntry) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gagal memulai db transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Simpan Header
	headerQuery := `
		INSERT INTO journal_entries (name, posting_date, docstatus, total_debit, total_credit, user_remark, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW());
	`
	_, err = tx.Exec(ctx, headerQuery,
		entry.Name, entry.PostingDate, entry.DocStatus, entry.TotalDebit, entry.TotalCredit, entry.UserRemark,
	)
	if err != nil {
		return fmt.Errorf("gagal insert journal_entries header: %w", err)
	}

	// 2. Simpan Child Items
	itemQuery := `
		INSERT INTO journal_entry_accounts (parent, idx, account, debit, credit, user_remark)
		VALUES ($1, $2, $3, $4, $5, $6);
	`
	for i, item := range entry.Accounts {
		_, err = tx.Exec(ctx, itemQuery,
			entry.Name, i+1, item.Account, item.Debit, item.Credit, item.UserRemark,
		)
		if err != nil {
			return fmt.Errorf("gagal insert journal_entry_accounts baris ke-%d: %w", i+1, err)
		}
	}

	return tx.Commit(ctx)
}

// GetJournalEntryByName mengambil data header dan seluruh baris jurnal
func (r *Repository) GetJournalEntryByName(ctx context.Context, name string) (*JournalEntry, error) {
	headerQuery := `
		SELECT name, posting_date, docstatus, total_debit, total_credit, COALESCE(user_remark, ''), created_at, updated_at
		FROM journal_entries
		WHERE name = $1;
	`
	var entry JournalEntry
	err := r.db.QueryRow(ctx, headerQuery, name).Scan(
		&entry.Name, &entry.PostingDate, &entry.DocStatus, &entry.TotalDebit,
		&entry.TotalCredit, &entry.UserRemark, &entry.CreatedAt, &entry.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("gagal query journal_entry: %w", err)
	}

	itemsQuery := `
		SELECT id, parent, idx, account, debit, credit, COALESCE(user_remark, '')
		FROM journal_entry_accounts
		WHERE parent = $1
		ORDER BY idx ASC;
	`
	rows, err := r.db.Query(ctx, itemsQuery, name)
	if err != nil {
		return nil, fmt.Errorf("gagal query journal_entry_accounts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item JournalEntryAccount
		err := rows.Scan(
			&item.ID, &item.Parent, &item.Idx, &item.Account,
			&item.Debit, &item.Credit, &item.UserRemark,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal scan item: %w", err)
		}
		entry.Accounts = append(entry.Accounts, item)
	}

	return &entry, nil
}

// SubmitJournalEntry mengubah status menjadi Submitted dan memposting ke Buku Besar (gl_entries)
func (r *Repository) SubmitJournalEntry(ctx context.Context, name string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi database: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Kunci dan validasi dokumen
	var docStatus types.DocStatus
	var postingDate time.Time
	var userRemark string

	lockQuery := `
		SELECT docstatus, posting_date, COALESCE(user_remark, '')
		FROM journal_entries
		WHERE name = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, lockQuery, name).Scan(&docStatus, &postingDate, &userRemark)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("dokumen '%s' tidak ditemukan", name)
		}
		return fmt.Errorf("gagal mengunci dokumen: %w", err)
	}

	if docStatus != types.DocStatusDraft {
		return fmt.Errorf("hanya dokumen berstatus Draft (0) yang dapat di-Submit. Status saat ini: %s", docStatus)
	}

	// 2. Ambil semua baris akun
	itemsQuery := `
		SELECT account, debit, credit, COALESCE(user_remark, '')
		FROM journal_entry_accounts
		WHERE parent = $1
		ORDER BY idx ASC;
	`
	rows, err := tx.Query(ctx, itemsQuery, name)
	if err != nil {
		return fmt.Errorf("gagal mengambil baris akun: %w", err)
	}
	defer rows.Close()

	type itemRow struct {
		account string
		debit   float64
		credit  float64
		remark  string
	}
	var items []itemRow
	for rows.Next() {
		var item itemRow
		if err := rows.Scan(&item.account, &item.debit, &item.credit, &item.remark); err != nil {
			return fmt.Errorf("gagal scan baris akun: %w", err)
		}
		items = append(items, item)
	}
	rows.Close()

	if len(items) < 2 {
		return fmt.Errorf("transaksi jurnal harus memiliki minimal 2 baris akun")
	}

	var sumDebit, sumCredit float64
	for _, item := range items {
		sumDebit += item.debit
		sumCredit += item.credit
	}
	if math.Abs(sumDebit-sumCredit) > 0.0001 {
		return fmt.Errorf("transaksi tidak seimbang: Total Debit (%.2f) != Total Kredit (%.2f)", sumDebit, sumCredit)
	}

	// 3. Update status dokumen menjadi Submitted (1)
	updateQuery := `
		UPDATE journal_entries
		SET docstatus = $1, updated_at = NOW()
		WHERE name = $2;
	`
	_, err = tx.Exec(ctx, updateQuery, types.DocStatusSubmitted, name)
	if err != nil {
		return fmt.Errorf("gagal update status dokumen: %w", err)
	}

	// 4. Posting ke gl_entries (Immutable Ledger)
	glInsertQuery := `
		INSERT INTO gl_entries (posting_date, account, debit, credit, voucher_type, voucher_no, is_cancelled, remarks, created_at)
		VALUES ($1, $2, $3, $4, 'Journal Entry', $5, FALSE, $6, NOW());
	`
	for _, item := range items {
		remarks := item.remark
		if remarks == "" {
			remarks = userRemark
		}
		_, err = tx.Exec(ctx, glInsertQuery,
			postingDate, item.account, item.debit, item.credit, name, remarks,
		)
		if err != nil {
			return fmt.Errorf("gagal posting ke gl_entries untuk akun '%s': %w", item.account, err)
		}
	}

	return tx.Commit(ctx)
}

// CancelJournalEntry membatalkan voucher dan memposting reversal entry ke gl_entries
func (r *Repository) CancelJournalEntry(ctx context.Context, name string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi database: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Kunci dan validasi dokumen
	var docStatus types.DocStatus
	var postingDate time.Time

	lockQuery := `
		SELECT docstatus, posting_date
		FROM journal_entries
		WHERE name = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, lockQuery, name).Scan(&docStatus, &postingDate)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("dokumen '%s' tidak ditemukan", name)
		}
		return fmt.Errorf("gagal mengunci dokumen: %w", err)
	}

	if docStatus != types.DocStatusSubmitted {
		return fmt.Errorf("hanya dokumen berstatus Submitted (1) yang dapat di-Cancel. Status saat ini: %s", docStatus)
	}

	// 2. Ambil baris jurnal untuk dibalik
	itemsQuery := `
		SELECT account, debit, credit
		FROM journal_entry_accounts
		WHERE parent = $1
		ORDER BY idx ASC;
	`
	rows, err := tx.Query(ctx, itemsQuery, name)
	if err != nil {
		return fmt.Errorf("gagal mengambil baris akun: %w", err)
	}
	defer rows.Close()

	type itemRow struct {
		account string
		debit   float64
		credit  float64
	}
	var items []itemRow
	for rows.Next() {
		var item itemRow
		if err := rows.Scan(&item.account, &item.debit, &item.credit); err != nil {
			return fmt.Errorf("gagal scan baris akun: %w", err)
		}
		items = append(items, item)
	}
	rows.Close()

	// 3. Update status dokumen menjadi Cancelled (2)
	updateQuery := `
		UPDATE journal_entries
		SET docstatus = $1, updated_at = NOW()
		WHERE name = $2;
	`
	_, err = tx.Exec(ctx, updateQuery, types.DocStatusCancelled, name)
	if err != nil {
		return fmt.Errorf("gagal update status dokumen: %w", err)
	}

	// 4. Posting Reversal Entry ke gl_entries (Tukar Debit & Kredit)
	glInsertQuery := `
		INSERT INTO gl_entries (posting_date, account, debit, credit, voucher_type, voucher_no, is_cancelled, remarks, created_at)
		VALUES ($1, $2, $3, $4, 'Journal Entry', $5, TRUE, $6, NOW());
	`
	for _, item := range items {
		reversalRemarks := fmt.Sprintf("Pembatalan (Reversal) untuk voucher %s", name)
		// Debit dibalik jadi Credit, Credit dibalik jadi Debit
		_, err = tx.Exec(ctx, glInsertQuery,
			postingDate, item.account, item.credit, item.debit, name, reversalRemarks,
		)
		if err != nil {
			return fmt.Errorf("gagal posting reversal ke gl_entries untuk akun '%s': %w", item.account, err)
		}
	}

	return tx.Commit(ctx)
}

// GetGeneralLedger mengambil daftar transaksi buku besar
func (r *Repository) GetGeneralLedger(ctx context.Context, account string) ([]GLEntry, error) {
	query := `
		SELECT id, posting_date, account, debit, credit, voucher_type, voucher_no, is_cancelled, COALESCE(remarks, ''), created_at
		FROM gl_entries
		WHERE ($1 = '' OR account = $1)
		ORDER BY posting_date ASC, created_at ASC;
	`
	rows, err := r.db.Query(ctx, query, account)
	if err != nil {
		return nil, fmt.Errorf("gagal query gl_entries: %w", err)
	}
	defer rows.Close()

	var entries []GLEntry
	for rows.Next() {
		var e GLEntry
		err := rows.Scan(
			&e.ID, &e.PostingDate, &e.Account, &e.Debit, &e.Credit,
			&e.VoucherType, &e.VoucherNo, &e.IsCancelled, &e.Remarks, &e.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal scan gl_entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetTrialBalance menghitung neraca saldo seluruh akun dari gl_entries
func (r *Repository) GetTrialBalance(ctx context.Context) ([]TrialBalanceRow, error) {
	query := `
		SELECT 
			a.name AS account,
			a.account_name,
			a.root_type,
			COALESCE(SUM(g.debit), 0) AS total_debit,
			COALESCE(SUM(g.credit), 0) AS total_credit
		FROM accounts a
		LEFT JOIN gl_entries g ON a.name = g.account
		WHERE a.is_group = FALSE
		GROUP BY a.name, a.account_number, a.account_name, a.root_type
		HAVING COALESCE(SUM(g.debit), 0) > 0 OR COALESCE(SUM(g.credit), 0) > 0
		ORDER BY a.account_number ASC, a.name ASC;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal kalkulasi trial balance: %w", err)
	}
	defer rows.Close()

	var results []TrialBalanceRow
	for rows.Next() {
		var row TrialBalanceRow
		err := rows.Scan(&row.Account, &row.AccountName, &row.RootType, &row.TotalDebit, &row.TotalCredit)
		if err != nil {
			return nil, fmt.Errorf("gagal scan trial balance row: %w", err)
		}

		// Kalkulasi Net Balance sesuai normal balance
		row.NetBalance = CalculateNetBalance(row.RootType, row.TotalDebit, row.TotalCredit)

		results = append(results, row)
	}
	return results, nil
}
