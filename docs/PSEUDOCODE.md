# Pseudocode Arsitektur & Logika Bisnis SimpleNext

Dokumen ini berisi spesifikasi **Pseudocode** komprehensif untuk seluruh modul dan alur logika sistem **SimpleNext**. Pseudocode ini dirancang secara terstruktur dan formal agar memudahkan pemahaman alur kerja perangkat lunak, perancangan ulang (*re-implementation*), audit logika, maupun proses pembelajaran.

---

## Daftar Isi
1. [Konvensi Notasi Pseudocode](#1-konvensi-notasi-pseudocode)
2. [Definisi Tipe Data & Entitas Utama](#2-definisi-tipe-data--entitas-utama)
3. [Alur Startup & Lifecycle Aplikasi (cmd/api/main.go)](#3-alur-startup--lifecycle-aplikasi)
4. [Modul Core: Naming Series Generator Atomik](#4-modul-core-naming-series-generator-atomik)
5. [Modul Accounting: Bagan Akun (Chart of Accounts)](#5-modul-accounting-bagan-akun-chart-of-accounts)
6. [Modul Accounting: Transaksi Jurnal (Journal Entry)](#6-modul-accounting-transaksi-jurnal-journal-entry)
7. [Modul Accounting: Buku Besar & Neraca Saldo (Reports)](#7-modul-accounting-buku-besar--neraca-saldo-reports)
8. [Lapisan HTTP Controller / Transport](#8-lapisan-http-controller--transport)
9. [State Machine Dokumen (DocStatus Lifecycle)](#9-state-machine-dokumen-docstatus-lifecycle)

---

## 1. Konvensi Notasi Pseudocode

Notasi yang digunakan dalam dokumen ini mengikuti standar pseudocode prosedural dan modular:
- `FUNCTION / END FUNCTION`: Blok fungsi yang mengembalikan nilai (*return value*).
- `PROCEDURE / END PROCEDURE`: Blok prosedur yang mengeksekusi operasi tanpa nilai kembalian langsung.
- `INPUT / OUTPUT`: Spesifikasi argumen masukan dan luaran.
- `IF / ELSE IF / ELSE / END IF`: Percabangan logika kondisional.
- `FOR EACH / FOR / WHILE / END FOR / END WHILE`: Perulangan (*loop*).
- `BEGIN TRANSACTION / COMMIT / ROLLBACK`: Transaksi basis data ACID.
- `THROW ERROR`: Melempar pengecualian/kesalahan jika validasi gagal.
- `CONCURRENTLY DO`: Eksekusi asinkronus (setara dengan goroutine di Go).

---

## 2. Definisi Tipe Data & Entitas Utama

```pascal
ENUM DocStatus:
    DRAFT = 0       // Dokumen konsep, dapat diubah, belum memengaruhi pembukuan
    SUBMITTED = 1   // Dokumen final/terkunci, telah diposting ke Buku Besar
    CANCELLED = 2   // Dokumen dibatalkan, telah diposting jurnal balik (reversal)
END ENUM

STRUCT Account:
    name: STRING            // Primary Key (contoh: "1110 - Kas Operasional")
    account_number: STRING  // Kode akun (contoh: "1110")
    account_name: STRING    // Nama akun (contoh: "Kas Operasional")
    account_type: STRING    // Bank, Cash, Receivable, Payable, dll.
    root_type: STRING       // Asset, Liability, Equity, Income, Expense
    is_group: BOOLEAN       // TRUE jika akun induk (tidak bisa dijurnal)
    parent_account: STRING  // Referensi ke parent Account.name (nullable)
    children: LIST OF Account
END STRUCT

STRUCT JournalEntryAccount:
    id: UUID
    parent: STRING          // Referensi ke JournalEntry.name
    idx: INTEGER            // Urutan baris (1, 2, 3, ...)
    account: STRING         // Referensi ke Account.name
    debit: DECIMAL(18, 4)
    credit: DECIMAL(18, 4)
    user_remark: STRING
END STRUCT

STRUCT JournalEntry:
    name: STRING            // Nomor voucher unik (contoh: "JV-2026-00001")
    posting_date: DATE
    docstatus: DocStatus
    total_debit: DECIMAL(18, 4)
    total_credit: DECIMAL(18, 4)
    user_remark: STRING
    accounts: LIST OF JournalEntryAccount
END STRUCT

STRUCT GLEntry:
    id: UUID
    posting_date: DATE
    account: STRING
    debit: DECIMAL(18, 4)
    credit: DECIMAL(18, 4)
    voucher_type: STRING    // "Journal Entry"
    voucher_no: STRING      // Nomor voucher asal
    is_cancelled: BOOLEAN   // TRUE jika baris reversal/pembatalan
    remarks: STRING
END STRUCT

STRUCT TrialBalanceRow:
    account: STRING
    account_name: STRING
    root_type: STRING
    total_debit: DECIMAL(18, 4)
    total_credit: DECIMAL(18, 4)
    net_balance: DECIMAL(18, 4)
END STRUCT
```

---

## 3. Alur Startup & Lifecycle Aplikasi

File rujukan: [`cmd/api/main.go`](file:///home/sandi/workspace/simplenext/cmd/api/main.go)

```pascal
PROCEDURE MainApplication():
    // 1. Membaca Konfigurasi Lingkungan (Environment Variables)
    dbURL := GET_ENV("DATABASE_URL", DEFAULT: "postgres://postgres:postgres@localhost:5432/simplenext?sslmode=disable")
    port  := GET_ENV("PORT", DEFAULT: "8080")

    // 2. Inisialisasi Database Connection Pool
    TRY:
        dbPool := InitializePostgresPool(dbURL, max_conns=25, min_conns=5)
    CATCH err:
        LOG_FATAL("Gagal terhubung ke basis data:", err)
        TERMINATE_PROCESS()
    END TRY

    // 3. Dependency Injection (Wiring dari bawah ke atas)
    namingService     := NewNamingService(dbPool)
    accountingRepo    := NewAccountingRepository(dbPool)
    accountingService := NewAccountingService(accountingRepo, namingService)
    accountingHandler := NewAccountingHandler(accountingService)

    // 4. Setup HTTP Router & Middlewares
    router := NewHTTPRouter()
    router.USE(RequestIDMiddleware())
    router.USE(RealIPMiddleware())
    router.USE(LoggerMiddleware())
    router.USE(RecovererMiddleware())
    router.USE(TimeoutMiddleware(60_SECONDS))

    // 5. Registrasi Routes
    router.GET("/health", HandleHealthCheck)
    accountingHandler.RegisterRoutes(router)

    // 6. Jalankan HTTP Server Secara Asinkron (Non-blocking)
    server := NewHTTPServer(Address=":" + port, Handler=router)
    serverErrorsChannel := CREATE_CHANNEL()

    CONCURRENTLY DO:
        serverErrorsChannel <- server.ListenAndServe()
    END CONCURRENTLY

    // 7. Menangani Graceful Shutdown
    shutdownSignalChannel := LISTEN_OS_SIGNALS(SIGINT, SIGTERM)

    SELECT:
        CASE err := <-serverErrorsChannel:
            IF err != SERVER_CLOSED:
                LOG_FATAL("Server mengalami crash:", err)
            END IF

        CASE signal := <-shutdownSignalChannel:
            LOG_INFO("Menerima sinyal terminasi:", signal)
            
            // Berikan tenggat waktu maksimal 10 detik untuk menyelesaikan request berjalan
            shutdownContext := CREATE_TIMEOUT_CONTEXT(10_SECONDS)
            server.Shutdown(shutdownContext)
            dbPool.Close()
            LOG_INFO("Server dan koneksi database ditutup secara aman.")
    END SELECT
END PROCEDURE
```

---

## 4. Modul Core: Naming Series Generator Atomik

File rujukan: [`internal/core/naming/service.go`](file:///home/sandi/workspace/simplenext/internal/core/naming/service.go)

### 4.1 Parsing Pola Seri (`ParsePattern`)
```pascal
FUNCTION ParsePattern(seriesPattern: STRING, now: DATETIME) -> (prefix: STRING, padLen: INTEGER):
    padLen := 0
    trimmedPattern := seriesPattern

    // Hitung jumlah karakter '#' di akhir string untuk menentukan panjang digit
    WHILE ENDS_WITH(trimmedPattern, "#"):
        padLen := padLen + 1
        trimmedPattern := SUBSTRING(trimmedPattern, 0, LENGTH(trimmedPattern) - 1)
    END WHILE

    IF padLen == 0 THEN
        padLen := 5 // Default padding 5 digit jika tidak ditentukan
    END IF

    trimmedPattern := TRIM_TRAILING(trimmedPattern, ".")

    // Substitusi placeholder waktu dinamis
    prefix := trimmedPattern
    prefix := REPLACE_ALL(prefix, ".YYYY.", FORMAT_DATE(now, "YYYY"))
    prefix := REPLACE_ALL(prefix, ".YY.",   FORMAT_DATE(now, "YY"))
    prefix := REPLACE_ALL(prefix, ".MM.",   FORMAT_DATE(now, "MM"))
    prefix := REPLACE_ALL(prefix, ".DD.",   FORMAT_DATE(now, "DD"))
    prefix := REPLACE_ALL(prefix, ".-.",    "-")
    prefix := REPLACE_ALL(prefix, "..",     ".")

    RETURN (prefix, padLen)
END FUNCTION
```

### 4.2 Pemformatan Seri (`FormatSeries`)
```pascal
FUNCTION FormatSeries(prefix: STRING, padLen: INTEGER, currentValue: INTEGER) -> STRING:
    // Format angka currentValue dengan leading zero sejumlah padLen
    // Contoh: prefix="JV-2026-", padLen=5, currentValue=1 -> "JV-2026-00001"
    formattedNumber := PAD_LEFT(STRING(currentValue), padLen, CHARACTER='0')
    RETURN CONCAT(prefix, formattedNumber)
END FUNCTION
```

### 4.3 Generate Nomor Seri Atomik (`Generate`)
```pascal
FUNCTION GenerateVoucherNumber(seriesPattern: STRING) -> (voucherNo: STRING, err: ERROR):
    now := CURRENT_TIMESTAMP()
    (prefix, padLen) := ParsePattern(seriesPattern, now)

    // Eksekusi upsert atomik pada PostgreSQL untuk mencegah duplicate key saat konkurensi tinggi
    SQL_QUERY := "
        INSERT INTO series_counters (prefix, current_value)
        VALUES ($1, 1)
        ON CONFLICT (prefix)
        DO UPDATE SET current_value = series_counters.current_value + 1
        RETURNING current_value;
    "

    TRY:
        currentValue := EXECUTE_SQL_QUERY_ROW(SQL_QUERY, prefix)
    CATCH dbErr:
        RETURN ("", CREATE_ERROR("Gagal menghasilkan nomor urut: " + dbErr.MESSAGE))
    END TRY

    voucherNo := FormatSeries(prefix, padLen, currentValue)
    RETURN (voucherNo, NULL)
END FUNCTION
```

---

## 5. Modul Accounting: Bagan Akun (Chart of Accounts)

File rujukan: [`internal/accounting/service.go`](file:///home/sandi/workspace/simplenext/internal/accounting/service.go), [`internal/accounting/repository.go`](file:///home/sandi/workspace/simplenext/internal/accounting/repository.go)

### 5.1 Penyusunan Hierarki Pohon Akun (`BuildAccountTree`)
```pascal
FUNCTION BuildAccountTree(flatAccounts: LIST OF Account) -> LIST OF Account:
    INPUT: flatAccounts (Daftar seluruh akun dalam format relasional datar)
    OUTPUT: rootAccounts (Daftar akun tingkat teratas dengan anak-anak yang terhubung secara rekursif)

    accountMap := CREATE_HASH_MAP() // Key: Account.name, Value: Pointer ke Account

    // Langkah 1: Petakan seluruh akun ke dalam Hash Map
    FOR EACH acc IN flatAccounts:
        acc.children := EMPTY_LIST()
        accountMap.PUT(acc.name, acc)
    END FOR

    rootAccounts := EMPTY_LIST()

    // Langkah 2: Hubungkan akun anak ke akun induknya
    FOR EACH acc IN accountMap.VALUES():
        IF acc.parent_account IS NULL OR acc.parent_account == "" THEN
            // Akun tidak memiliki induk, merupakan akun Root
            rootAccounts.APPEND(acc)
        ELSE
            parentAcc := accountMap.GET(acc.parent_account)
            IF parentAcc IS NOT NULL THEN
                parentAcc.children.APPEND(acc)
            ELSE
                // Jika parent tidak ditemukan dalam set data, masukkan ke root sebagai fallback
                rootAccounts.APPEND(acc)
            END IF
        END IF
    END FOR

    RETURN rootAccounts
END FUNCTION
```

---

## 6. Modul Accounting: Transaksi Jurnal (Journal Entry)

File rujukan: [`internal/accounting/service.go`](file:///home/sandi/workspace/simplenext/internal/accounting/service.go), [`internal/accounting/repository.go`](file:///home/sandi/workspace/simplenext/internal/accounting/repository.go)

### 6.1 Validasi Keseimbangan Debit/Kredit (`ValidateDebitCreditBalance`)
```pascal
FUNCTION ValidateDebitCreditBalance(items: LIST OF CreateJournalEntryItemRequest) 
    -> (totalDebit: DECIMAL, totalCredit: DECIMAL, err: ERROR):
    
    // Syarat 1: Minimal 2 baris transaksi (prinsip double-entry)
    IF LENGTH(items) < 2 THEN
        RETURN (0, 0, CREATE_ERROR("Jurnal harus memiliki minimal 2 baris akun"))
    END IF

    totalDebit := 0.0
    totalCredit := 0.0

    FOR idx := 0 TO LENGTH(items) - 1:
        item := items[idx]

        // Syarat 2: Akun tidak boleh kosong
        IF TRIM(item.account) == "" THEN
            RETURN (0, 0, CREATE_ERROR("Baris ke-" + (idx+1) + ": akun tidak boleh kosong"))
        END IF

        // Syarat 3: Tidak boleh bernilai negatif
        IF item.debit < 0 OR item.credit < 0 THEN
            RETURN (0, 0, CREATE_ERROR("Baris ke-" + (idx+1) + ": nilai debit atau kredit tidak boleh negatif"))
        END IF

        // Syarat 4: Salah satu harus lebih besar dari 0
        IF item.debit == 0 AND item.credit == 0 THEN
            RETURN (0, 0, CREATE_ERROR("Baris ke-" + (idx+1) + ": salah satu dari debit atau kredit harus > 0"))
        END IF

        // Syarat 5: Satu baris tidak boleh memiliki debit dan kredit sekaligus
        IF item.debit > 0 AND item.credit > 0 THEN
            RETURN (0, 0, CREATE_ERROR("Baris ke-" + (idx+1) + ": satu baris tidak boleh memiliki debit dan kredit sekaligus"))
        END IF

        totalDebit += item.debit
        totalCredit += item.credit
    END FOR

    // Syarat 6: Total Debit harus seimbang dengan Total Kredit (toleransi floating point 0.0001)
    IF ABS(totalDebit - totalCredit) > 0.0001 THEN
        RETURN (totalDebit, totalCredit, CREATE_ERROR(
            "Jurnal tidak seimbang: Total Debit (" + totalDebit + ") != Total Kredit (" + totalCredit + ")"
        ))
    END IF

    RETURN (totalDebit, totalCredit, NULL)
END FUNCTION
```

### 6.2 Pembuatan Draft Journal Entry (`CreateJournalEntry`)
```pascal
FUNCTION CreateJournalEntry(request: CreateJournalEntryRequest) -> (entry: JournalEntry, err: ERROR):
    // 1. Validasi kesetimbangan debit dan kredit
    (totalDebit, totalCredit, valErr) := ValidateDebitCreditBalance(request.accounts)
    IF valErr IS NOT NULL THEN
        RETURN (NULL, valErr)
    END IF

    // 2. Tentukan Tanggal Posting
    postingDate := CURRENT_DATE()
    IF request.posting_date != "" THEN
        TRY:
            postingDate := PARSE_DATE(request.posting_date, "YYYY-MM-DD")
        CATCH parseErr:
            RETURN (NULL, CREATE_ERROR("Format posting_date tidak valid (gunakan YYYY-MM-DD)"))
        END TRY
    END IF

    // 3. Validasi Akun-Akun ke Database
    childRows := EMPTY_LIST()
    FOR idx := 0 TO LENGTH(request.accounts) - 1:
        item := request.accounts[idx]

        accountInDB := repository.GetAccountByName(item.account)
        IF accountInDB IS NULL THEN
            RETURN (NULL, CREATE_ERROR("Akun '" + item.account + "' tidak ditemukan dalam sistem"))
        END IF

        // Transaksi HANYA boleh diposting ke Akun Daun (Leaf/Ledger), BUKAN Akun Grup
        IF accountInDB.is_group == TRUE THEN
            RETURN (NULL, CREATE_ERROR("Transaksi tidak dapat dilakukan pada akun grup/induk ('" + item.account + "')"))
        END IF

        childRows.APPEND(JournalEntryAccount(
            idx=idx + 1,
            account=item.account,
            debit=item.debit,
            credit=item.credit,
            user_remark=item.user_remark
        ))
    END FOR

    // 4. Generate Nomor Voucher Unik via Naming Series
    (voucherNo, numErr) := namingService.Generate("JV-.YYYY.-.#####")
    IF numErr IS NOT NULL THEN
        RETURN (NULL, CREATE_ERROR("Gagal menghasilkan nomor voucher: " + numErr.MESSAGE))
    END IF

    // 5. Persiapkan Entitas Dokumen Baru (Status = DRAFT)
    newEntry := JournalEntry(
        name=voucherNo,
        posting_date=postingDate,
        docstatus=DocStatus.DRAFT,
        total_debit=totalDebit,
        total_credit=totalCredit,
        user_remark=request.user_remark,
        accounts=childRows
    )

    // 6. Simpan ke Database dalam Transaksi ACID
    BEGIN TRANSACTION
        TRY:
            EXECUTE_SQL("
                INSERT INTO journal_entries (name, posting_date, docstatus, total_debit, total_credit, user_remark, created_at, updated_at)
                VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
            ", newEntry.name, newEntry.posting_date, newEntry.docstatus, newEntry.total_debit, newEntry.total_credit, newEntry.user_remark)

            FOR EACH row IN childRows:
                EXECUTE_SQL("
                    INSERT INTO journal_entry_accounts (parent, idx, account, debit, credit, user_remark)
                    VALUES ($1, $2, $3, $4, $5, $6)
                ", newEntry.name, row.idx, row.account, row.debit, row.credit, row.user_remark)
            END FOR

            COMMIT TRANSACTION
        CATCH txErr:
            ROLLBACK TRANSACTION
            RETURN (NULL, txErr)
        END TRY

    RETURN (newEntry, NULL)
END FUNCTION
```

### 6.3 Submit & Posting ke Buku Besar (`SubmitJournalEntry`)
```pascal
PROCEDURE SubmitJournalEntry(voucherName: STRING):
    BEGIN TRANSACTION
        TRY:
            // 1. Kunci Baris Dokumen dengan Pessimistic Lock (FOR UPDATE)
            docRow := EXECUTE_SQL_QUERY_ROW("
                SELECT docstatus, posting_date, COALESCE(user_remark, '')
                FROM journal_entries
                WHERE name = $1
                FOR UPDATE;
            ", voucherName)

            IF docRow IS NULL THEN
                THROW ERROR("Dokumen '" + voucherName + "' tidak ditemukan")
            END IF

            // 2. Validasi Siklus Dokumen: Hanya Draft (0) yang boleh di-Submit
            IF docRow.docstatus != DocStatus.DRAFT THEN
                THROW ERROR("Hanya dokumen berstatus Draft (0) yang dapat di-Submit. Status saat ini: " + docRow.docstatus)
            END IF

            // 3. Ambil Detail Baris Akun Jurnal
            items := EXECUTE_SQL_QUERY_ALL("
                SELECT account, debit, credit, COALESCE(user_remark, '')
                FROM journal_entry_accounts
                WHERE parent = $1
                ORDER BY idx ASC;
            ", voucherName)

            IF LENGTH(items) < 2 THEN
                THROW ERROR("Transaksi jurnal harus memiliki minimal 2 baris akun")
            END IF

            // 4. Verifikasi Ulang Keseimbangan Debit/Kredit sebelum Posting
            sumDebit := 0.0
            sumCredit := 0.0
            FOR EACH item IN items:
                sumDebit += item.debit
                sumCredit += item.credit
            END FOR

            IF ABS(sumDebit - sumCredit) > 0.0001 THEN
                THROW ERROR("Transaksi tidak seimbang: Total Debit != Total Kredit")
            END IF

            // 5. Update Status Dokumen Menjadi SUBMITTED (1)
            EXECUTE_SQL("
                UPDATE journal_entries
                SET docstatus = $1, updated_at = NOW()
                WHERE name = $2;
            ", DocStatus.SUBMITTED, voucherName)

            // 6. Posting ke General Ledger (gl_entries - Immutable Append-Only Ledger)
            FOR EACH item IN items:
                remark := item.user_remark
                IF remark == "" THEN
                    remark := docRow.user_remark
                END IF

                EXECUTE_SQL("
                    INSERT INTO gl_entries (posting_date, account, debit, credit, voucher_type, voucher_no, is_cancelled, remarks, created_at)
                    VALUES ($1, $2, $3, $4, 'Journal Entry', $5, FALSE, $6, NOW());
                ", docRow.posting_date, item.account, item.debit, item.credit, voucherName, remark)
            END FOR

            COMMIT TRANSACTION
            LOG_INFO("Dokumen " + voucherName + " berhasil di-submit dan diposting ke GL.")

        CATCH err:
            ROLLBACK TRANSACTION
            THROW ERROR("Gagal melakukan submit journal entry: " + err.MESSAGE)
        END TRY
END PROCEDURE
```

### 6.4 Pembatalan & Posting Jurnal Balik (`CancelJournalEntry`)
```pascal
PROCEDURE CancelJournalEntry(voucherName: STRING):
    BEGIN TRANSACTION
        TRY:
            // 1. Kunci Dokumen dengan Pessimistic Lock (FOR UPDATE)
            docRow := EXECUTE_SQL_QUERY_ROW("
                SELECT docstatus, posting_date
                FROM journal_entries
                WHERE name = $1
                FOR UPDATE;
            ", voucherName)

            IF docRow IS NULL THEN
                THROW ERROR("Dokumen '" + voucherName + "' tidak ditemukan")
            END IF

            // 2. Validasi Siklus Dokumen: Hanya Submitted (1) yang boleh di-Cancel
            IF docRow.docstatus != DocStatus.SUBMITTED THEN
                THROW ERROR("Hanya dokumen berstatus Submitted (1) yang dapat di-Cancel. Status saat ini: " + docRow.docstatus)
            END IF

            // 3. Ambil Seluruh Baris Akun Asal
            items := EXECUTE_SQL_QUERY_ALL("
                SELECT account, debit, credit
                FROM journal_entry_accounts
                WHERE parent = $1
                ORDER BY idx ASC;
            ", voucherName)

            // 4. Update Status Dokumen Menjadi CANCELLED (2)
            EXECUTE_SQL("
                UPDATE journal_entries
                SET docstatus = $1, updated_at = NOW()
                WHERE name = $2;
            ", DocStatus.CANCELLED, voucherName)

            // 5. Posting Reversal Entry ke gl_entries (Menukar Debit <-> Kredit)
            reversalRemarks := "Pembatalan (Reversal) untuk voucher " + voucherName

            FOR EACH item IN items:
                // DEBIT DITUKAR MENJADI KREDIT, KREDIT DITUKAR MENJADI DEBIT
                reversedDebit  := item.credit
                reversedCredit := item.debit

                EXECUTE_SQL("
                    INSERT INTO gl_entries (posting_date, account, debit, credit, voucher_type, voucher_no, is_cancelled, remarks, created_at)
                    VALUES ($1, $2, $3, $4, 'Journal Entry', $5, TRUE, $6, NOW());
                ", docRow.posting_date, item.account, reversedDebit, reversedCredit, voucherName, reversalRemarks)
            END FOR

            COMMIT TRANSACTION
            LOG_INFO("Dokumen " + voucherName + " berhasil dibatalkan beserta jurnal pembalik.")

        CATCH err:
            ROLLBACK TRANSACTION
            THROW ERROR("Gagal membatalkan journal entry: " + err.MESSAGE)
        END TRY
END PROCEDURE
```

---

## 7. Modul Accounting: Buku Besar & Neraca Saldo (Reports)

File rujukan: [`internal/accounting/service.go`](file:///home/sandi/workspace/simplenext/internal/accounting/service.go), [`internal/accounting/repository.go`](file:///home/sandi/workspace/simplenext/internal/accounting/repository.go)

### 7.1 Kalkulasi Saldo Normal Bersih (`CalculateNetBalance`)
```pascal
FUNCTION CalculateNetBalance(rootType: STRING, totalDebit: DECIMAL, totalCredit: DECIMAL) -> DECIMAL:
    // Sesuai prinsip akuntansi umum:
    // - Asset & Expense bertambah di Debit (Saldo Normal: Debit)
    // - Liability, Equity, Income bertambah di Kredit (Saldo Normal: Kredit)
    IF rootType == "Asset" OR rootType == "Expense" THEN
        RETURN totalDebit - totalCredit
    ELSE
        RETURN totalCredit - totalDebit
    END IF
END FUNCTION
```

### 7.2 Laporan Buku Besar (`GetGeneralLedger`)
```pascal
FUNCTION GetGeneralLedger(filterAccount: STRING) -> LIST OF GLEntry:
    // Mengambil riwayat entri buku besar yang sudah diposting
    SQL_QUERY := "
        SELECT id, posting_date, account, debit, credit, voucher_type, voucher_no, is_cancelled, COALESCE(remarks, ''), created_at
        FROM gl_entries
        WHERE ($1 = '' OR account = $1)
        ORDER BY posting_date ASC, created_at ASC;
    "

    entries := EXECUTE_SQL_QUERY_ALL(SQL_QUERY, filterAccount)
    RETURN entries
END FUNCTION
```

### 7.3 Laporan Neraca Saldo (`GetTrialBalance`)
```pascal
FUNCTION GetTrialBalance() -> (rows: LIST OF TrialBalanceRow, totalDebit: DECIMAL, totalCredit: DECIMAL, isBalanced: BOOLEAN):
    // Agregasi saldo debit dan kredit dari gl_entries yang dikelompokkan per akun leaf
    SQL_QUERY := "
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
    "

    rawRows := EXECUTE_SQL_QUERY_ALL(SQL_QUERY)
    reportRows := EMPTY_LIST()

    grandTotalDebit := 0.0
    grandTotalCredit := 0.0

    FOR EACH r IN rawRows:
        netBalance := CalculateNetBalance(r.root_type, r.total_debit, r.total_credit)

        reportRow := TrialBalanceRow(
            account=r.account,
            account_name=r.account_name,
            root_type=r.root_type,
            total_debit=r.total_debit,
            total_credit=r.total_credit,
            net_balance=netBalance
        )
        reportRows.APPEND(reportRow)

        grandTotalDebit  += r.total_debit
        grandTotalCredit += r.total_credit
    END FOR

    isBalanced := (ABS(grandTotalDebit - grandTotalCredit) < 0.0001)

    RETURN (reportRows, grandTotalDebit, grandTotalCredit, isBalanced)
END FUNCTION
```

---

## 8. Lapisan HTTP Controller / Transport

File rujukan: [`internal/accounting/handler.go`](file:///home/sandi/workspace/simplenext/internal/accounting/handler.go)

```pascal
// Endpoint: GET /api/v1/accounts?tree=true|false
PROCEDURE HandleListAccounts(request: HTTPRequest, response: HTTPResponse):
    treeParam := request.QUERY_PARAMS.GET("tree")

    IF treeParam == "true" THEN
        tree := accountingService.GetAccountTree()
        response.WRITE_JSON(STATUS=200, BODY={"data": tree})
    ELSE
        accounts := accountingRepo.GetAllAccounts()
        response.WRITE_JSON(STATUS=200, BODY={"data": accounts})
    END IF
END PROCEDURE

// Endpoint: POST /api/v1/journal-entries
PROCEDURE HandleCreateJournalEntry(request: HTTPRequest, response: HTTPResponse):
    TRY:
        payload := DECODE_JSON_BODY(request.BODY, CreateJournalEntryRequest)
    CATCH err:
        response.WRITE_JSON(STATUS=400, BODY={"error": "Invalid JSON body: " + err.MESSAGE})
        RETURN
    END TRY

    (entry, err) := accountingService.CreateJournalEntry(payload)
    IF err IS NOT NULL THEN
        response.WRITE_JSON(STATUS=400, BODY={"error": err.MESSAGE})
        RETURN
    END IF

    response.WRITE_JSON(STATUS=201, BODY={
        "message": "Draft Journal Entry berhasil dibuat",
        "data": entry
    })
END PROCEDURE

// Endpoint: POST /api/v1/journal-entries/{name}/submit
PROCEDURE HandleSubmitJournalEntry(request: HTTPRequest, response: HTTPResponse):
    voucherName := request.PATH_PARAMS.GET("name")

    TRY:
        accountingService.SubmitJournalEntry(voucherName)
    CATCH err:
        response.WRITE_JSON(STATUS=400, BODY={"error": err.MESSAGE})
        RETURN
    END TRY

    updatedEntry := accountingService.GetJournalEntry(voucherName)
    response.WRITE_JSON(STATUS=200, BODY={
        "message": "Journal Entry berhasil di-Submit dan diposting ke Buku Besar (GL)",
        "data": updatedEntry
    })
END PROCEDURE

// Endpoint: POST /api/v1/journal-entries/{name}/cancel
PROCEDURE HandleCancelJournalEntry(request: HTTPRequest, response: HTTPResponse):
    voucherName := request.PATH_PARAMS.GET("name")

    TRY:
        accountingService.CancelJournalEntry(voucherName)
    CATCH err:
        response.WRITE_JSON(STATUS=400, BODY={"error": err.MESSAGE})
        RETURN
    END TRY

    updatedEntry := accountingService.GetJournalEntry(voucherName)
    response.WRITE_JSON(STATUS=200, BODY={
        "message": "Journal Entry berhasil di-Cancel dan jurnal balik (reversal) telah diposting",
        "data": updatedEntry
    })
END PROCEDURE

// Endpoint: GET /api/v1/reports/trial-balance
PROCEDURE HandleGetTrialBalance(request: HTTPRequest, response: HTTPResponse):
    (rows, totalDebit, totalCredit, isBalanced) := accountingService.GetTrialBalance()

    response.WRITE_JSON(STATUS=200, BODY={
        "data": rows,
        "total_debit": totalDebit,
        "total_credit": totalCredit,
        "is_balanced": isBalanced
    })
END PROCEDURE
```

---

## 9. State Machine Dokumen (DocStatus Lifecycle)

Diagram transisi status dokumen dan pengaruhnya terhadap basis data Buku Besar:

```mermaid
stateDiagram-v2
    [*] --> Draft : CreateJournalEntry()
    note right of Draft
        DocStatus = 0
        - Dapat diedit
        - gl_entries KOSONG
    end note

    Draft --> Submitted : SubmitJournalEntry()
    note right of Submitted
        DocStatus = 1
        - Terkunci (Read-Only)
        - INSERT gl_entries (Posting Asli)
    end note

    Submitted --> Cancelled : CancelJournalEntry()
    note right of Cancelled
        DocStatus = 2
        - Dibatalkan permanen
        - INSERT gl_entries (Reversal: Tukar Debit & Kredit)
    end note

    Cancelled --> [*]
```

### Ringkasan Validasi State Transisi

| Transisi | Kondisi Awal (`docstatus`) | Kondisi Akhir (`docstatus`) | Efek terhadap `gl_entries` |
|---|---|---|---|
| **Create Draft** | Belum ada | `0` (Draft) | Tidak ada entri yang dibuat |
| **Submit** | Harus `0` (Draft) | `1` (Submitted) | Menambahkan baris debit/kredit asli |
| **Cancel** | Harus `1` (Submitted) | `2` (Cancelled) | Menambahkan baris pembalik (*reversal*) dengan membalik debit dan kredit |
| **Modifikasi saat Submitted** | `1` (Submitted) | - | **Dilarang** (Dokumen terkunci) |
| **Re-Submit setelah Cancel** | `2` (Cancelled) | - | **Dilarang** (Siklus bersifat satu arah) |
