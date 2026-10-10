# BAB 8: MODUL 3 - JOURNAL ENTRY & SIKLUS TRANSAKSI DOKUMEN

Halo semuanya! Tiba saatnya kita mengerjakan modul paling menantang sekaligus paling memuaskan: **Journal Entry (Jurnal Umum)**. Modul ini adalah jantung dari aplikasi finansial kita.

Pada bab ini, kita akan menggabungkan *Naming Series* yang kita buat di Bab 6 dan daftar *Bagan Akun* dari Bab 7 untuk menciptakan form transaksi yang dinamis. Lebih dari itu, kita akan mempelajari **Siklus Dokumen (Document Lifecycle)** yang mencegah data akuntansi dihapus secara sembarangan.

---

## 8.1 Siklus Hidup Dokumen (Document Lifecycle)

Di SimpleNext, dokumen tidak sekadar berstatus "Tersimpan". Dokumen melewati 3 fase `DocStatus`:

- **0 = Draft**: Dokumen baru disimpan sementara. Masih bisa diedit, dibongkar-pasang, dan **belum masuk ke Buku Besar (General Ledger)**.
- **1 = Submitted**: Dokumen telah diverifikasi dan dikunci (*Read-Only*). Saat status berubah ke `1`, sistem **otomatis mencetak baris ke tabel Buku Besar**.
- **2 = Cancelled**: Jika ada kesalahan pada dokumen yang sudah `Submitted`, kita **tidak boleh melakukan DELETE**. Kita mengubah statusnya ke `2`, lalu sistem otomatis membuat jurnal pembalik (*Reversal Entry*) di Buku Besar.

---

## 8.2 Backend: Validasi Ketat Akuntansi (Double-Entry)

Sebuah jurnal tidak boleh timpang sebelah. Mari kita tulis fungsi murni (tanpa pusing memikirkan database dulu) untuk mengecek perhitungan matematikanya.

Buka (atau perbarui) file `internal/accounting/service.go` dan tambahkan fungsi validasi ini:

```go
// ValidateDebitCreditBalance memastikan jurnal seimbang (Debit == Credit)
func ValidateDebitCreditBalance(items []CreateJournalEntryItemRequest) (totalDebit float64, totalCredit float64, err error) {
	// 1. Validasi minimal 2 baris transaksi (Satu yang memberi, satu yang menerima)
	if len(items) < 2 {
		return 0, 0, fmt.Errorf("jurnal harus memiliki minimal 2 baris akun")
	}

	for i, item := range items {
		if item.Account == "" {
			return 0, 0, fmt.Errorf("baris ke-%d: akun tidak boleh kosong", i+1)
		}
		if item.Debit < 0 || item.Credit < 0 {
			return 0, 0, fmt.Errorf("baris ke-%d: nilai tidak boleh negatif", i+1)
		}
		if item.Debit == 0 && item.Credit == 0 {
			return 0, 0, fmt.Errorf("baris ke-%d: salah satu dari debit/kredit harus lebih dari 0", i+1)
		}
		if item.Debit > 0 && item.Credit > 0 {
			return 0, 0, fmt.Errorf("baris ke-%d: satu baris tidak boleh memiliki nilai debit dan kredit sekaligus", i+1)
		}

		totalDebit += item.Debit
		totalCredit += item.Credit
	}

	// 2. Validasi Keseimbangan dengan toleransi presisi desimal
	if math.Abs(totalDebit-totalCredit) > 0.0001 {
		return totalDebit, totalCredit, fmt.Errorf("jurnal tidak seimbang: Total Debit (%.2f) != Total Kredit (%.2f)", totalDebit, totalCredit)
	}

	return totalDebit, totalCredit, nil
}
```

> 💡 **[CATATAN/TIPS]**  
> Perhatikan pada blok validasi akhir, kita tidak menggunakan `totalDebit != totalCredit`. Ini karena tipe data Float di Golang bisa jadi sedikit tidak akurat di angka puluhan desimal. Oleh karena itu, kita membandingkan nilai selisih absolutnya agar lebih kecil dari `0.0001`.

---

## 8.3 Backend: Menyimpan Draft Jurnal (Status 0)

Setelah lulus perhitungan, kita validasi kebenaran akun dari database dan membuat nomor urut menggunakan mesin dari Bab 6.

Lanjutkan mengetik di `internal/accounting/service.go`:

```go
// CreateJournalEntry memvalidasi dan menyimpan draft Journal Entry
func (s *Service) CreateJournalEntry(ctx context.Context, req *CreateJournalEntryRequest) (*JournalEntry, error) {
	
	// 1. Validasi Keseimbangan
	totalDebit, totalCredit, err := ValidateDebitCreditBalance(req.Accounts)
	if err != nil {
		return nil, err
	}

	// 2. Siapkan array untuk menampung baris (Child Table)
	var childRows []JournalEntryAccount

	for i, item := range req.Accounts {
		// Validasi akun di database: tidak boleh menggunakan akun Folder/Group!
		acc, err := s.repo.GetAccountByName(ctx, item.Account)
		if err != nil || acc == nil {
			return nil, fmt.Errorf("akun '%s' tidak ditemukan", item.Account)
		}
		if acc.IsGroup {
			return nil, fmt.Errorf("transaksi tidak dapat dilakukan pada akun grup ('%s')", item.Account)
		}

		childRows = append(childRows, JournalEntryAccount{
			Idx:        i + 1,
			Account:    item.Account,
			Debit:      item.Debit,
			Credit:     item.Credit,
			UserRemark: item.UserRemark,
		})
	}

	// 3. Generate nomor dokumen menggunakan Naming Series ala ERPNext
	voucherNo, err := s.namingService.Generate(ctx, "JV-.YYYY.-.#####")
	if err != nil {
		return nil, fmt.Errorf("gagal generate nomor voucher: %w", err)
	}

	// 4. Siapkan Entitas (Model)
	entry := &JournalEntry{
		Name:        voucherNo,
		PostingDate: postingDate,
		DocStatus:   types.DocStatusDraft, // 0 = Draft
		TotalDebit:  totalDebit,
		TotalCredit: totalCredit,
		UserRemark:  req.UserRemark,
		Accounts:    childRows,
	}

	// 5. Simpan ke database melalui Repository
	err = s.repo.CreateJournalEntry(ctx, entry)
	return entry, err
}
```

---

## 8.4 Backend: Transaksi Submit & Audit Trail Otomatis

Bagian paling krusial! Saat status berpindah dari Draft (`0`) ke Submitted (`1`), kita harus menyimpan Jurnal tersebut sekaligus menyalinnya ke Buku Besar (`gl_entries`) secara **Atomik**. Jika terjadi *error* di tengah jalan, seluruhnya harus di-*rollback*.

Di `internal/accounting/repository.go`, metode Submit harus menggunakan **Database Transaction (`pgx.Tx`)**:

```go
// SubmitJournalEntry mengubah status dan memposting ke General Ledger
func (r *Repository) SubmitJournalEntry(ctx context.Context, voucherNo string) error {
	// Memulai Transaksi SQL
	tx, err := r.db.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx) // Akan batal otomatis jika tidak ada tx.Commit()

	// 1. Ubah docstatus menjadi 1 (Submitted)
	updateQuery := `UPDATE journal_entries SET docstatus = 1 WHERE name = $1 AND docstatus = 0`
	commandTag, err := tx.Exec(ctx, updateQuery, voucherNo)
	if err != nil || commandTag.RowsAffected() == 0 {
		return fmt.Errorf("jurnal tidak ditemukan atau sudah tersubmit")
	}

	// 2. Ambil baris detail dari jurnal tersebut
	// ... (kode SELECT dari journal_entry_accounts) ...
	
	// 3. Posting semua baris transaksi ke tabel Buku Besar Murni (gl_entries)
	insertGLQuery := `
		INSERT INTO gl_entries (posting_date, account, debit, credit, voucher_type, voucher_no, remarks)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	for _, row := range childRows {
		_, err = tx.Exec(ctx, insertGLQuery, 
			postingDate, row.Account, row.Debit, row.Credit, "Journal Entry", voucherNo, row.UserRemark,
		)
		if err != nil { return err }
	}

	// Semuanya berhasil? Kunci database!
	return tx.Commit(ctx)
}
```

---

## 8.5 Frontend: Form Dinamis Vue 3 (Live Calculator)

Di Vue 3, form Jurnal sangat interaktif. Pengguna bisa memencet tombol "Tambah Baris" dan total debit/kredit akan terhitung otomatis (reaktif).

`frontend/src/views/JournalEntryForm.vue`
```vue
<template>
  <div>
    <h2>Buat Jurnal Umum</h2>
    
    <!-- Tabel Baris Transaksi -->
    <table class="w-full mb-4">
      <thead>
        <tr>
          <th>Akun</th>
          <th>Debit</th>
          <th>Kredit</th>
          <th>Aksi</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, index) in formData.accounts" :key="index">
          <td>
            <select v-model="row.account">
              <option v-for="acc in ledgerAccounts" :value="acc.name">{{ acc.name }}</option>
            </select>
          </td>
          <td><input type="number" v-model="row.debit" min="0" /></td>
          <td><input type="number" v-model="row.credit" min="0" /></td>
          <td>
            <button @click="removeRow(index)" class="text-red-500">Hapus</button>
          </td>
        </tr>
      </tbody>
    </table>

    <button @click="addRow" class="bg-gray-200 px-3 py-1 mb-4">Tambah Baris</button>

    <!-- Kalkulator Live -->
    <div class="flex justify-between bg-gray-50 p-4 font-bold">
      <span :class="{'text-red-500': totalDebit !== totalCredit}">
        Total Debit: Rp {{ totalDebit }}
      </span>
      <span :class="{'text-red-500': totalDebit !== totalCredit}">
        Total Kredit: Rp {{ totalCredit }}
      </span>
    </div>

    <!-- Tombol Simpan -->
    <button 
      @click="saveDraft" 
      :disabled="totalDebit !== totalCredit || formData.accounts.length < 2"
      class="bg-blue-600 text-white px-4 py-2 mt-4 w-full disabled:opacity-50"
    >
      Simpan Draft
    </button>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const formData = ref({
  user_remark: '',
  accounts: [
    { account: '', debit: 0, credit: 0 },
    { account: '', debit: 0, credit: 0 }
  ]
})

// Fungsi Computed super reaktif! 
// Otomatis terhitung tiap kali input diubah.
const totalDebit = computed(() => {
  return formData.value.accounts.reduce((sum, row) => sum + Number(row.debit), 0)
})

const totalCredit = computed(() => {
  return formData.value.accounts.reduce((sum, row) => sum + Number(row.credit), 0)
})

const addRow = () => {
  formData.value.accounts.push({ account: '', debit: 0, credit: 0 })
}

const removeRow = (index) => {
  formData.value.accounts.splice(index, 1)
}

const saveDraft = async () => {
  // Panggil endpoint API Axios
  // ...
}
</script>
```

---

## 🔍 [PENGUJIAN SIKLUS DOKUMEN]

Uji dari Vue Frontend atau Postman Anda:
1. Buat Draft Jurnal dengan Debit 10.000 dan Kredit 5.000.  
   **Ekspektasi:** Gagal dengan pesan "jurnal tidak seimbang".
2. Samakan Debit dan Kredit menjadi 10.000, lalu klik Simpan Draft.  
   **Ekspektasi:** Berhasil, status menjadi `0`, dokumen tersimpan di `journal_entries` namun `gl_entries` masih kosong.
3. Buka halaman detail jurnal tersebut, klik **Submit**.  
   **Ekspektasi:** Berhasil, status menjadi `1`, dan jika Anda mengecek database, tabel `gl_entries` sekarang terisi baris mutasi!

Luar biasa! Sistem pergerakan uang yang anti-bocor (*bulletproof*) sudah di tangan Anda.

---

➡️ **Langkah Berikutnya:** Setelah dokumen disubmit, data masuk ke Buku Besar. Tapi, bagaimana cara membacanya menjadi laporan keuangan yang dapat dimengerti manusia? Kita akan mempelajarinya di [**BAB 9: MODUL 4 - GENERAL LEDGER & LAPORAN KEUANGAN REALTIME**](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_09_GENERAL_LEDGER_REPORTS.md).
