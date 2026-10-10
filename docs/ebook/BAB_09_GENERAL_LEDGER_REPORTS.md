# BAB 9: MODUL 4 - GENERAL LEDGER & LAPORAN KEUANGAN REALTIME

Halo semuanya! Selamat, Anda telah tiba di salah satu bab paling berharga di sistem finansial. Apa gunanya mencatat ratusan Jurnal Umum setiap hari jika Anda tidak bisa melihat status kesehatan perusahaan Anda?

Di bab ini, kita akan meracik kumpulan catatan mentah (*raw data*) di tabel `gl_entries` menjadi dua buah Laporan Keuangan standar internasional:
1. **Buku Besar (General Ledger):** Daftar riwayat mutasi pergerakan uang per satu akun.
2. **Neraca Saldo (Trial Balance):** Rekapitulasi total seluruh saldo perusahaan untuk melihat apakah pembukuan Anda benar-benar seimbang (Balance).

---

## 9.1 General Ledger & Menghitung Saldo Berjalan (Running Balance)

Buku Besar adalah arsip seluruh transaksi yang telah di-*Submit*. Di Backend, kita cukup membuat query sederhana untuk mengambil data dari `gl_entries` dan mengurutkannya berdasarkan tanggal.

Buka file `internal/accounting/repository.go` dan tambahkan metode ini:

```go
// GetGeneralLedger mengambil daftar transaksi buku besar berdasarkan akun
func (r *Repository) GetGeneralLedger(ctx context.Context, account string) ([]GLEntry, error) {
	query := `
		SELECT 
			id, posting_date, account, debit, credit, 
			voucher_type, voucher_no, is_cancelled, COALESCE(remarks, ''), created_at
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
```

### Frontend Vue 3: Menghitung Running Balance
Daripada membebani database, kita bisa menghitung *Running Balance* (Saldo Berjalan) secara dinamis di *Frontend* ketika merender tabel!

`frontend/src/views/GeneralLedgerView.vue`
```vue
<template>
  <div>
    <h2>Laporan Buku Besar</h2>
    
    <!-- Filter Akun -->
    <select v-model="selectedAccount" @change="fetchLedger">
      <option value="">Semua Akun</option>
      <option v-for="acc in ledgerAccounts" :value="acc.name">{{ acc.name }}</option>
    </select>

    <!-- Tabel Mutasi -->
    <table class="w-full mt-4 border">
      <thead>
        <tr class="bg-gray-100">
          <th>Tanggal</th>
          <th>Voucher</th>
          <th>Debit</th>
          <th>Kredit</th>
          <th>Saldo Berjalan (Balance)</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in ledgerWithRunningBalance" :key="row.id">
          <td>{{ row.posting_date }}</td>
          <td>{{ row.voucher_no }}</td>
          <td class="text-green-600">Rp {{ row.debit }}</td>
          <td class="text-red-600">Rp {{ row.credit }}</td>
          <td class="font-bold">Rp {{ row.running_balance }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const entries = ref([]) // Data dari API Golang
const selectedAccount = ref('')

// Computed Property super reaktif untuk kalkulasi Running Balance
const ledgerWithRunningBalance = computed(() => {
  let balance = 0
  return entries.value.map(entry => {
    // Rumus saldo berjalan:
    balance = balance + Number(entry.debit) - Number(entry.credit)
    return {
      ...entry,
      running_balance: balance
    }
  })
})
</script>
```

> 💡 **[CATATAN/TIPS]**  
> Perhatikan pada tabel di atas. Dengan fitur *Computed Property* Vue 3, setiap kali variabel `entries` berubah (karena hasil *fetch API*), Vue akan otomatis me-*looping* array tersebut untuk menyuntikkan properti baru bernama `running_balance`. Ini sangat efisien dan instan!

---

## 9.2 Laporan Neraca Saldo (Trial Balance)

Tugas *Trial Balance* adalah menjumlahkan seluruh *Debit* dan *Kredit* dari setiap akun dari masa lalu hingga saat ini, lalu menampilkan "Saldo Akhir"-nya (*Net Balance*). 

Di sinilah kekuatan SQL berpadu dengan Golang. Kita akan menggunakan fungsi agregasi `SUM()` dari Postgres.

Di dalam `internal/accounting/repository.go`:

```go
// GetTrialBalance menghitung neraca saldo seluruh akun dari gl_entries
func (r *Repository) GetTrialBalance(ctx context.Context) ([]TrialBalanceRow, error) {
	// Query Agregasi SUM dan LEFT JOIN
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
			return nil, err
		}

		// Hitung Net Balance di Golang menggunakan fungsi helper
		row.NetBalance = CalculateNetBalance(row.RootType, row.TotalDebit, row.TotalCredit)

		results = append(results, row)
	}
	return results, nil
}
```

### Rahasia Akuntansi: Aturan "Saldo Normal" (Normal Balance)

Mengapa kita menggunakan fungsi pembantu `CalculateNetBalance`? Di dunia Akuntansi, Aset dan Beban itu berbeda sifat dengan Kewajiban, Ekuitas, dan Pendapatan.

Di `internal/accounting/service.go`, kita mendefinisikan aturannya:
```go
// CalculateNetBalance menghitung saldo bersih (Net Balance) akun berdasarkan jenis root normal balance
func CalculateNetBalance(rootType string, totalDebit, totalCredit float64) float64 {
	// Aset dan Beban bertambah di sisi Debit. (Saldo = Debit - Kredit)
	if rootType == "Asset" || rootType == "Expense" {
		return totalDebit - totalCredit
	}
	// Kewajiban, Ekuitas, dan Pendapatan bertambah di sisi Kredit. (Saldo = Kredit - Debit)
	return totalCredit - totalDebit
}
```

---

## 9.3 Frontend: Tampilan Trial Balance (Neraca Saldo)

Di Vue, kita akan merender array hasil balasan API tersebut ke dalam tabel dan menjumlahkan kolom total di paling bawah untuk membuktikan aplikasi kita "Balance".

`frontend/src/views/TrialBalanceView.vue`
```vue
<template>
  <div>
    <h2>Laporan Neraca Saldo (Trial Balance)</h2>
    
    <table class="w-full mt-4 border text-left">
      <thead class="bg-blue-800 text-white">
        <tr>
          <th>Akun</th>
          <th>Tipe Akar</th>
          <th>Total Debit</th>
          <th>Total Kredit</th>
          <th>Saldo Akhir (Net)</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.account" class="border-b">
          <td>{{ row.account_name }}</td>
          <td>{{ row.root_type }}</td>
          <td>Rp {{ row.total_debit }}</td>
          <td>Rp {{ row.total_credit }}</td>
          <td class="font-bold">Rp {{ row.net_balance }}</td>
        </tr>
        
        <!-- Baris Grand Total (Penentu Kebenaran Akuntansi) -->
        <tr class="bg-gray-200 font-bold">
          <td colspan="2" class="text-right pr-4">GRAND TOTAL:</td>
          <td>Rp {{ grandTotalDebit }}</td>
          <td>Rp {{ grandTotalCredit }}</td>
          <td :class="{'text-green-600': isBalanced, 'text-red-600': !isBalanced}">
            {{ isBalanced ? 'SEIMBANG (BALANCED)' : 'ERROR TIMPANG' }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const rows = ref([]) // Data API

const grandTotalDebit = computed(() => rows.value.reduce((sum, r) => sum + Number(r.total_debit), 0))
const grandTotalCredit = computed(() => rows.value.reduce((sum, r) => sum + Number(r.total_credit), 0))
const isBalanced = computed(() => grandTotalDebit.value === grandTotalCredit.value)
</script>
```

---

## 🔍 [PENGUJIAN LAPORAN]

Mari kita rayakan keberhasilan ini:
1. Buka halaman UI Trial Balance Anda.
2. Jika Anda telah men-submit sebuah *Journal Entry* (misal: "Kas Kecil" Debit 500.000 dan "Pendapatan" Kredit 500.000) pada bab sebelumnya.
3. Anda akan melihat tabel Trial Balance memuat kedua baris tersebut.
4. Di bagian paling bawah (Grand Total), Anda akan melihat teks hijau **"SEIMBANG (BALANCED)"** menyala terang karena Total Debit Perusahaan = Total Kredit Perusahaan.

Selamat! Anda baru saja menciptakan mesin pelaporan akuntansi yang bisa bersaing dengan produk bernilai ratusan juta! 🚀

---

➡️ **Langkah Berikutnya:** Tabel angka memang informatif, tapi bos akan lebih senang melihat *chart* dan grafik yang cantik. Di bab selanjutnya, kita akan membangun Dashboard visual yang indah. Lanjutkan ke [**BAB 10: MODUL 5 - DASHBOARD FINANSIAL & VISUALISASI METRIK**](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_10_DASHBOARD_METRICS.md).
