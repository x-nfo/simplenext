# BAB 6: MODUL 1 - ATOMIC NAMING SERIES GENERATOR (CORE ENGINE)

Halo semuanya! Memasuki logika bisnis pertama kita, kita akan memecahkan salah satu masalah klasik tersulit dalam pemrograman *multi-user*: **Race Condition pada Pembuatan Nomor Dokumen**.

Pernahkah Anda berbelanja di minimarket, dan di struk pembayaran Anda tertulis nomor transaksi (misal: `INV-2026-10-00001`)? Bagaimana jika ada 100 kasir di seluruh Indonesia yang menekan tombol "Bayar" secara bersamaan di detik yang sama persis? Jika sistem tidak dirancang dengan baik, 2 atau 3 kasir bisa mendapatkan nomor setruk yang sama persis! Ini adalah mimpi buruk akuntansi.

Di bab ini, kita akan membuat "Mesin Penomoran" (*Naming Series Engine*) yang **Atomik**, menjamin 100% tidak akan pernah ada nomor dokumen yang ganda.

---

## 6.1 Masalah Konkurensi (Race Condition) & Solusi Buruknya

Banyak programmer pemula menggunakan cara ini untuk mendapatkan nomor urut:
```sql
-- JANGAN GUNAKAN CARA INI!
SELECT MAX(id) FROM journal_entries;
-- Kemudian di kode: nomor_baru = hasil_max + 1
```
Ini sangat berbahaya! Jika User A dan User B menjalankan query `SELECT MAX(id)` secara bersamaan, keduanya akan mendapat angka yang sama (misal `50`). Lalu keduanya akan menyimpan dokumen baru dengan nomor urut `51`. Terjadilah duplikasi (*Data Collision*).

**Solusi SimpleNext:**
Kita menggunakan satu tabel khusus bernama `series_counters` dan memanfaatkan fitur `ON CONFLICT DO UPDATE` bawaan PostgreSQL yang dieksekusi secara **Atomik** (dikunci per baris oleh database langsung).

---

## 6.2 Membangun Logika Parse & Format Pola Naming Series

Sistem ERP modern (seperti ERPNext) mengizinkan pengguna mengatur pola nomor secara dinamis.
Contoh:
- `JV-.YYYY.-.MM.-.#####` akan menjadi `JV-2026-10-00001`
- `ACC-####` akan menjadi `ACC-0001`

Mari kita buat *Service* untuk menangani hal ini.

Buat file baru di dalam direktori `core`:
`internal/core/naming/service.go`
```go
package naming

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// 1. ParsePattern: Menerjemahkan pola teks menjadi Prefix (Awalan) dan Panjang Angka (PadLen).
// Contoh: "JV-.YYYY.-.#####" -> prefix: "JV-2026-", padLen: 5
func ParsePattern(seriesPattern string, now time.Time) (prefix string, padLen int) {
	padLen = 0
	trimmedPattern := seriesPattern
	
	// Menghitung jumlah '#' di akhir string
	for strings.HasSuffix(trimmedPattern, "#") {
		padLen++
		trimmedPattern = trimmedPattern[:len(trimmedPattern)-1]
	}
	if padLen == 0 {
		padLen = 5 // Default jika tidak ada simbol #
	}
	trimmedPattern = strings.TrimSuffix(trimmedPattern, ".")

	prefix = trimmedPattern
	
	// Mengganti token dinamis dengan nilai waktu saat ini
	prefix = strings.ReplaceAll(prefix, ".YYYY.", fmt.Sprintf("%04d", now.Year()))
	prefix = strings.ReplaceAll(prefix, ".YY.", fmt.Sprintf("%02d", now.Year()%100))
	prefix = strings.ReplaceAll(prefix, ".MM.", fmt.Sprintf("%02d", int(now.Month())))
	prefix = strings.ReplaceAll(prefix, ".DD.", fmt.Sprintf("%02d", now.Day()))

	prefix = strings.ReplaceAll(prefix, ".-.", "-")
	prefix = strings.ReplaceAll(prefix, "..", ".")
	
	return prefix, padLen
}

// 2. FormatSeries: Menggabungkan Prefix dan Angka Urut dengan tambahan Nol di depan.
func FormatSeries(prefix string, padLen int, currentValue int64) string {
	// %0*d artinya: Format angka desimal, tambahkan nol di depan sebanyak padLen
	return fmt.Sprintf("%s%0*d", prefix, padLen, currentValue)
}
```

---

## 6.3 Implementasi Repository SQL: Atomik "Upsert"

Ini adalah inti dari anti-*Race Condition* kita. Kita menambahkan method `Generate` ke dalam struct `Service` di file yang sama.

Lanjutkan mengetik di bawah file `internal/core/naming/service.go`:

```go
// 3. Generate menghasilkan kode dokumen berikutnya secara atomik.
func (s *Service) Generate(ctx context.Context, seriesPattern string) (string, error) {
	prefix, padLen := ParsePattern(seriesPattern, time.Now())

	// QUERY SAKTI ANTI-BENTROK
	// INSERT jika prefix belum ada (nilai 1)
	// UPDATE (ditambah 1) jika prefix sudah ada
	// RETURNING langsung mengembalikan nilai terbaru di transaksi yang sama!
	query := `
		INSERT INTO series_counters (prefix, current_value)
		VALUES ($1, 1)
		ON CONFLICT (prefix)
		DO UPDATE SET current_value = series_counters.current_value + 1
		RETURNING current_value;
	`

	var currentValue int64
	err := s.db.QueryRow(ctx, query, prefix).Scan(&currentValue)
	if err != nil {
		return "", fmt.Errorf("gagal generate sequence untuk prefix '%s': %w", prefix, err)
	}

	return FormatSeries(prefix, padLen, currentValue), nil
}
```

> 💡 **[CATATAN/TIPS]**  
> Perintah `ON CONFLICT DO UPDATE` dalam PostgreSQL akan menggunakan *Row-Level Lock* secara otomatis. Jika ada dua permintaan datang di milidetik yang sama, Postgres akan menyuruh permintaan kedua mengantre (menunggu fraksi detik) sampai permintaan pertama selesai menambah urutan. Aman 100%!

---

## 6.4 Unit Testing Logika Parse (Tanpa Database)

Arsitektur yang baik harus mudah diuji. Karena fungsi `ParsePattern` dan `FormatSeries` adalah *Pure Function* (Fungsi murni tanpa butuh koneksi database), kita bisa membuat *Unit Test* super cepat.

Buat file baru (perhatikan akhiran `_test.go`):

`internal/core/naming/service_test.go`
```go
package naming

import (
	"testing"
	"time"
)

func TestParsePattern(t *testing.T) {
	// Kunci waktu agar test bisa diprediksi
	fixedTime := time.Date(2026, time.March, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		pattern        string
		expectedPrefix string
		expectedPadLen int
	}{
		{
			name:           "Standard JV Pattern",
			pattern:        "JV-.YYYY.-.#####",
			expectedPrefix: "JV-2026-",
			expectedPadLen: 5,
		},
		{
			name:           "Short Year and Month Pattern",
			pattern:        "INV-.YY.-.MM.-.####",
			expectedPrefix: "INV-26-03-",
			expectedPadLen: 4,
		},
		{
			name:           "No Date Tokens",
			pattern:        "ACC-#####",
			expectedPrefix: "ACC-",
			expectedPadLen: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prefix, padLen := ParsePattern(tc.pattern, fixedTime)
			if prefix != tc.expectedPrefix {
				t.Errorf("expected prefix '%s', got '%s'", tc.expectedPrefix, prefix)
			}
			if padLen != tc.expectedPadLen {
				t.Errorf("expected padLen %d, got %d", tc.expectedPadLen, padLen)
			}
		})
	}
}

func TestFormatSeries(t *testing.T) {
	got := FormatSeries("JV-2026-", 5, 42)
	expected := "JV-2026-00042"
	
	if got != expected {
		t.Errorf("expected '%s', got '%s'", expected, got)
	}
}
```

---

## 🔍 [PENGUJIAN & EKSEKUSI UNIT TEST]

Sangat penting untuk memastikan mesin penomoran kita tidak memiliki salah hitung.
Mari jalankan *Unit Test* murni kita melalui perintah terminal:

```bash
make test-unit
```
atau secara manual:
```bash
go test -v ./internal/core/naming
```

Jika terminal menampilkan output `PASS`, artinya logika penomoran dinamis Anda sudah benar. Anda baru saja membangun salah satu modul paling *core* di sistem ERP!

---

➡️ **Langkah Berikutnya:** Mesin penomoran kita siap digunakan. Selanjutnya, kita akan melompat ke ranah akuntansi sungguhan: Membuat struktur Pohon Bagan Akun (Chart of Accounts) secara hierarkis. Lanjutkan ke [**BAB 7: MODUL 2 - CHART OF ACCOUNTS (BAGAN AKUN HIERARKIS)**](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_07_CHART_OF_ACCOUNTS.md).
