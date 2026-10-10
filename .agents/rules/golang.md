---
trigger: "glob:**/*.go"
description: "Aturan penulisan dan konvensi Golang untuk proyek SimpleNext"
---

# Aturan Penulisan Golang — SimpleNext

Saat membaca, menulis, atau memodifikasi kode Go di repositori SimpleNext, patuhi aturan-aturan berikut:

## 1. Arsitektur & Lapisan (Layering)
- **Unidirectional Flow**: `cmd/api` -> `handler` -> `service` -> `repository` -> `database pool`.
- `handler.go`: Hanya urusan HTTP & JSON decoding/response. Tidak boleh ada SQL atau kalkulasi bisnis akuntansi.
- `service.go`: Logika bisnis domain murni (validasi double-entry debit==credit, tree builder, orkestrasi). Bebas dari `net/http`.
- `repository.go`: Query SQL eksplisit (`pgx`), scanning baris, transaksi ACID, locking `FOR UPDATE`.
- `model.go`: Struct domain, DTO, custom types, dan JSON tag (`snake_case`).

## 2. Penamaan & Idiom Go
- **Package**: Lowercase tunggal tanpa underscore/hyphen (`accounting`, `naming`, `types`).
- **Identifiers**: `PascalCase` untuk exported, `camelCase` untuk unexported. Tidak menggunakan `snake_case` pada variabel atau fungsi Go.
- **Acronyms/Initialisms**: Wajib huruf besar semua jika kapital (`URL`, `ID`, `HTTP`, `SQL`, `GL`, `COA`, `DTO`, `DB`). Contoh: `dbURL`, `GetAccountByID`, `GLEntry`.
- **Receiver Name**: 1-2 huruf singkatan struct yang konsisten (`s *Service`, `r *Repository`, `h *Handler`, `s DocStatus`). Dilarang memakai `this` atau `self`.
- **Getter**: Tanpa prefix `Get` untuk field murni (`doc.Status()`). Boleh pakai `Get...` untuk query database (`repo.GetAllAccounts`).

## 3. Error Handling & Context
- Selalu tangani error dengan `if err != nil`. Dilarang mengabaikan error dengan `_`.
- Dilarang memakai `panic` di logika aplikasi (hanya di `main.go` bootstrap jika fatal).
- Gunakan error wrapping dengan verb `%w`: `fmt.Errorf("gagal query: %w", err)`.
- Format pesan error: diawali huruf kecil dan tanpa titik di akhir.
- Selalu letakkan `ctx context.Context` sebagai parameter pertama fungsi I/O / DB.
- Jangan pernah menyimpan `ctx context.Context` di dalam field struct.

## 4. Database & Transaksi (`pgx`)
- Selalu gunakan parameterized queries (`$1, $2`). Dilarang merangkai query string manual (`fmt.Sprintf`).
- Transaksi ACID: Selalu gunakan pola `defer tx.Rollback(ctx)` setelah `r.db.Begin(ctx)` dan commit eksplisit `return tx.Commit(ctx)`.
- Selalu gunakan `defer rows.Close()` setelah `Query(ctx, ...)` berhasil.
- Gunakan `SELECT ... FOR UPDATE` saat mutasi dokumen untuk mencegah race condition.

## 5. Dependency Injection
- Gunakan constructor manual `NewService(...)`, `NewRepository(...)`, `NewHandler(...)`.
- Tidak menggunakan global variables untuk database pool atau service.
