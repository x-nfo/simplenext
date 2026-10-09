-- Enable pgcrypto for UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Naming Series Counter
-- Digunakan untuk menghasilkan kode urut dokumen yang atomik dan aman dari konkurensi
CREATE TABLE IF NOT EXISTS series_counters (
    prefix VARCHAR(50) PRIMARY KEY,
    current_value BIGINT NOT NULL DEFAULT 0
);

-- 2. Chart of Accounts (COA)
-- Merepresentasikan pohon bagan akun dengan hierarki parent-child
CREATE TABLE IF NOT EXISTS accounts (
    name VARCHAR(140) PRIMARY KEY,
    account_number VARCHAR(50),
    account_name VARCHAR(140) NOT NULL,
    account_type VARCHAR(50) NOT NULL, -- Asset, Liability, Equity, Income, Expense, Bank, Cash, Receivable, Payable
    root_type VARCHAR(50) NOT NULL,    -- Asset, Liability, Equity, Income, Expense
    is_group BOOLEAN NOT NULL DEFAULT FALSE,
    parent_account VARCHAR(140) REFERENCES accounts(name),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index untuk hierarki akun
CREATE INDEX IF NOT EXISTS idx_accounts_parent ON accounts(parent_account);
CREATE INDEX IF NOT EXISTS idx_accounts_root_type ON accounts(root_type);

-- 3. Header Transaksi Jurnal (Journal Entry)
-- docstatus: 0 = Draft, 1 = Submitted, 2 = Cancelled
CREATE TABLE IF NOT EXISTS journal_entries (
    name VARCHAR(140) PRIMARY KEY,
    posting_date DATE NOT NULL,
    docstatus SMALLINT NOT NULL DEFAULT 0,
    total_debit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    total_credit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    user_remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_journal_entries_date ON journal_entries(posting_date);
CREATE INDEX IF NOT EXISTS idx_journal_entries_docstatus ON journal_entries(docstatus);

-- 4. Baris Akun Jurnal (Journal Entry Account - Child Table)
CREATE TABLE IF NOT EXISTS journal_entry_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent VARCHAR(140) NOT NULL REFERENCES journal_entries(name) ON DELETE CASCADE,
    idx INT NOT NULL,
    account VARCHAR(140) NOT NULL REFERENCES accounts(name),
    debit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    credit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    user_remark TEXT
);

CREATE INDEX IF NOT EXISTS idx_journal_entry_accounts_parent ON journal_entry_accounts(parent);
CREATE INDEX IF NOT EXISTS idx_journal_entry_accounts_account ON journal_entry_accounts(account);

-- 5. General Ledger Entry (Immutable Append-Only Ledger)
-- Setiap voucher yang di-submit akan memposting baris ke tabel ini
CREATE TABLE IF NOT EXISTS gl_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    posting_date DATE NOT NULL,
    account VARCHAR(140) NOT NULL REFERENCES accounts(name),
    debit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    credit NUMERIC(18, 4) NOT NULL DEFAULT 0,
    voucher_type VARCHAR(50) NOT NULL,
    voucher_no VARCHAR(140) NOT NULL,
    is_cancelled BOOLEAN NOT NULL DEFAULT FALSE,
    remarks TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gl_entries_account_date ON gl_entries(account, posting_date);
CREATE INDEX IF NOT EXISTS idx_gl_entries_voucher ON gl_entries(voucher_type, voucher_no);
