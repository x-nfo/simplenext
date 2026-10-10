# BAB 5: DATABASE MIGRATION (DDL), PGX DRIVER, & SEEDER

Halo semuanya! Pada bab sebelumnya, kita sudah membahas cetak biru (*blueprint*) desain tabel di Bab 2. Sekarang, kita akan benar-benar "mengeksekusi" pembuatan tabel tersebut ke dalam PostgreSQL menggunakan metode **Database Migration as Code** dan menyambungkan backend Golang kita dengan koneksi berkecepatan tinggi!

---

## 5.1 Mengapa Menghindari Auto-Migrate ORM?

Dalam banyak framework seperti Laravel (Eloquent) atau GORM di Golang, developer sering dimanjakan dengan fitur *Auto-Migrate* (tabel otomatis dibuat dari *Struct* / Model). 

Namun, untuk sistem akuntansi berskala *Enterprise*, **Auto-Migrate sangat dihindari**. Mengapa?
1. Kita kehilangan kontrol atas *Index* spesifik dan *Constraint* database tingkat lanjut (seperti `ON CONFLICT` atau *Foreign Key Cascade*).
2. Tipe data finansial (seperti `NUMERIC(18,4)`) sering kali salah dipetakan menjadi `FLOAT` oleh ORM.
3. Jejak perubahan skema (*Versioning*) menjadi tidak jelas di *source code*.

Solusinya: Kita menulis file *Data Definition Language* (DDL) murni secara manual!

---

## 5.2 Menulis Skema Migrasi DDL Lengkap

Mari kita buat file SQL yang berisi kumpulan perintah pembuatan tabel (DDL). 

Buat file baru:
`migrations/000001_init_schema.up.sql`
```sql
-- 1. Enable pgcrypto untuk UUID generation bawaan Postgres
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 2. Naming Series Counter (Generator Nomor Atomik)
CREATE TABLE IF NOT EXISTS series_counters (
    prefix VARCHAR(50) PRIMARY KEY,
    current_value BIGINT NOT NULL DEFAULT 0
);

-- 3. Chart of Accounts (COA - Pohon Akun)
CREATE TABLE IF NOT EXISTS accounts (
    name VARCHAR(140) PRIMARY KEY,
    account_number VARCHAR(50),
    account_name VARCHAR(140) NOT NULL,
    account_type VARCHAR(50) NOT NULL, 
    root_type VARCHAR(50) NOT NULL,    
    is_group BOOLEAN NOT NULL DEFAULT FALSE,
    parent_account VARCHAR(140) REFERENCES accounts(name),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Optimasi Query: Mempercepat pencarian pohon akun
CREATE INDEX IF NOT EXISTS idx_accounts_parent ON accounts(parent_account);
CREATE INDEX IF NOT EXISTS idx_accounts_root_type ON accounts(root_type);

-- 4. Journal Entries (Header Transaksi)
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

-- 5. Journal Entry Accounts (Detail Baris Transaksi)
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

-- 6. GL Entries (Buku Besar - Append Only)
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
```

> 💡 **[CATATAN/TIPS]**  
> Perhatikan sintaks `gen_random_uuid()`. Ini adalah fungsi internal Postgres 13+ yang sangat cepat untuk menghasilkan UUID versi 4 tanpa perlu di-generate dari backend Golang.

---

## 5.3 Menghubungkan Backend ke PostgreSQL (`pgxpool`)

Sekarang saatnya menghubungkan Golang ke database. Kita akan menggunakan **Connection Pooling**. *Pool* memastikan aplikasi tidak membuat koneksi baru setiap kali ada request HTTP, melainkan mendaur ulang koneksi yang sudah ada untuk menghemat RAM dan mempercepat latensi.

Buat file konfigurasi database:

`internal/platform/database/postgres.go`
```go
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool menginisialisasi connection pool PostgreSQL
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("gagal parse connection string: %w", err)
	}

	// Tuning Performa untuk Production
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat database pool: %w", err)
	}

	// Lakukan Ping untuk memastikan database benar-benar hidup
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("gagal menghubungi PostgreSQL: %w", err)
	}

	return pool, nil
}
```

---

## 5.4 Database Seeder: Data Awal Standar Akuntansi (COA)

Agar kita tidak perlu mengetik ratusan akun pembukuan secara manual di aplikasi, kita harus menyuntikkan (*seed*) "Bagan Akun Standar" pada saat instalasi sistem.

Buat file seeder SQL:

`migrations/000002_seed_accounts.sql`
```sql
-- Hapus data lama agar aman jika dijalankan ulang
DELETE FROM accounts;

-- 1. Insert Akun Induk (Root / Groups)
INSERT INTO accounts (name, account_number, account_name, account_type, root_type, is_group, parent_account) VALUES
('Asset', '1', 'Asset', 'Asset', 'Asset', TRUE, NULL),
('Liability', '2', 'Liability', 'Liability', 'Liability', TRUE, NULL),
('Equity', '3', 'Equity', 'Equity', 'Equity', TRUE, NULL),
('Income', '4', 'Income', 'Income', 'Income', TRUE, NULL),
('Expense', '5', 'Expense', 'Expense', 'Expense', TRUE, NULL);

-- 2. Insert Sub-Grup Aset
INSERT INTO accounts (name, account_number, account_name, account_type, root_type, is_group, parent_account) VALUES
('Current Asset - Asset', '11', 'Current Asset', 'Asset', 'Asset', TRUE, 'Asset'),
('Bank and Cash - Asset', '111', 'Bank and Cash', 'Bank', 'Asset', TRUE, 'Current Asset - Asset');

-- 3. Insert Akun Buku Besar (Ledger) yang Bisa Menerima Transaksi Jurnal (is_group = FALSE)
INSERT INTO accounts (name, account_number, account_name, account_type, root_type, is_group, parent_account) VALUES
('1110 - Kas Kecil - Asset', '1110', 'Kas Kecil', 'Cash', 'Asset', FALSE, 'Bank and Cash - Asset'),
('1111 - Bank BCA - Asset', '1111', 'Bank BCA', 'Bank', 'Asset', FALSE, 'Bank and Cash - Asset'),
('5100 - Biaya Listrik - Expense', '5100', 'Biaya Listrik', 'Expense', 'Expense', FALSE, 'Expense');
```

---

## 🔍 [PENGUJIAN & EKSEKUSI MIGRASI]

Semuanya sudah siap! Mengingat pada Bab 3 kita sudah membuat `Makefile` super sakti, proses migrasi ini semudah membalikkan telapak tangan.

Buka terminal dan jalankan:

```bash
# 1. Pastikan docker database sedang menyala
make docker-up

# 2. Eksekusi Migrasi DDL (Membuat seluruh tabel)
make migrate-up

# 3. Suntikkan Data Seeder (Bagan Akun Awal)
make seed
```

Untuk memverifikasi hasilnya:
1. Buka *database client* favorit Anda (seperti DBeaver atau pgAdmin) atau jalankan di terminal: `docker exec -it simplenext-postgres psql -U postgres -d simplenext`
2. Jalankan perintah `\dt` untuk melihat daftar tabel.
3. Jalankan `SELECT * FROM accounts;`. Anda akan melihat struktur pohon bagan akun bersarang (*nested*) sudah siap digunakan!

---

➡️ **Langkah Berikutnya:** Struktur database yang handal telah berdiri tegak. Sekarang kita masuk ke *Modul 1: Mesin Pembuat Nomor Seri*! Di bab selanjutnya, kita akan memecahkan salah satu masalah tersulit dalam sistem *multi-user*: Menghasilkan nomor dokumen otomatis yang anti-bentrok. Lanjutkan ke [**BAB 6: MODUL 1 - ATOMIC NAMING SERIES ENGINE (CORE PLATFORM)**](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_06_NAMING_SERIES.md).
