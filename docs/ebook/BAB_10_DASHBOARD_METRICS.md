# BAB 10: MODUL 5 - DASHBOARD FINANSIAL & VISUALISASI METRIK

Halo semuanya! Jika Bab 9 memanjakan Auditor dengan tabel angka panjang yang presisi, maka Bab 10 ini dikhususkan untuk memanjakan mata para Eksekutif dan Manajer. 

Seorang CEO tidak punya waktu untuk membaca ratusan baris Neraca Saldo. Mereka butuh ringkasan instan: *"Berapa uang tunai kita hari ini?"*, *"Berapa laba/rugi kita bulan ini?"*. 
Mari kita buat Halaman **Dashboard** reaktif untuk menjawab pertanyaan tersebut dengan cepat!

---

## 10.1 Backend: Endpoint Metrik Ringkasan Eksekutif

Karena arsitektur kita sudah memisahkan `root_type` (Tipe Akar) untuk setiap entri Buku Besar, menghitung Total Aset, Pendapatan, atau Beban sangatlah mudah. Kita hanya butuh satu atau dua query SQL.

Buka file `internal/accounting/repository.go` dan tambahkan metode untuk metrik:

```go
// GetDashboardMetrics menghitung agregasi cepat untuk halaman Dashboard utama
func (r *Repository) GetDashboardMetrics(ctx context.Context) (map[string]float64, error) {
	// Query agregasi total berdasarkan tipe akar dari tabel gl_entries
	query := `
		SELECT 
			a.root_type,
			COALESCE(SUM(g.debit), 0) AS total_debit,
			COALESCE(SUM(g.credit), 0) AS total_credit
		FROM gl_entries g
		JOIN accounts a ON g.account = a.name
		WHERE g.is_cancelled = FALSE
		GROUP BY a.root_type;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil metrik dashboard: %w", err)
	}
	defer rows.Close()

	metrics := map[string]float64{
		"TotalAsset":   0,
		"TotalIncome":  0,
		"TotalExpense": 0,
		"NetProfit":    0,
	}

	for rows.Next() {
		var rootType string
		var debit, credit float64
		
		if err := rows.Scan(&rootType, &debit, &credit); err != nil {
			return nil, err
		}

		// Hitung saldo normal seperti di Bab 9
		switch rootType {
		case "Asset":
			metrics["TotalAsset"] += (debit - credit)
		case "Income":
			metrics["TotalIncome"] += (credit - debit)
		case "Expense":
			metrics["TotalExpense"] += (debit - credit)
		}
	}

	// Laba Bersih = Pendapatan Kotor - Beban Pengeluaran
	metrics["NetProfit"] = metrics["TotalIncome"] - metrics["TotalExpense"]

	return metrics, nil
}
```

> 💡 **[CATATAN/TIPS]**  
> Perhatikan query `WHERE g.is_cancelled = FALSE`. Kita tidak ingin memasukkan dokumen yang berstatus "Dibatalkan" ke dalam statistik metrik kita.

Kemudian di file `handler.go`, buat endpoint sederhana yang me-return `metrics` ini ke dalam bentuk JSON `/api/v1/dashboard/metrics`.

---

## 10.2 Frontend: Widget Kartu Statistik (*KPI Cards*)

Kita akan membuat komponen kartu (*card*) yang cantik menggunakan Tailwind CSS dan ikon dari perpustakaan Lucide. 

Di dalam Vue 3 Anda, buat sebuah View baru untuk halaman utama Dashboard:

`frontend/src/views/DashboardView.vue`
```vue
<template>
  <div class="p-6">
    <h1 class="text-3xl font-bold text-gray-800 mb-6">Dashboard Keuangan</h1>

    <!-- Grid Kartu Statistik -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
      
      <!-- Kartu Total Aset -->
      <div class="bg-white rounded-xl shadow p-6 border-l-4 border-blue-500">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-500 font-semibold uppercase">Total Aset</p>
            <p class="text-2xl font-bold text-gray-900 mt-1">Rp {{ formatUang(metrics.TotalAsset) }}</p>
          </div>
          <div class="p-3 bg-blue-100 rounded-full">
            <BuildingIcon class="w-6 h-6 text-blue-600" />
          </div>
        </div>
      </div>

      <!-- Kartu Pendapatan -->
      <div class="bg-white rounded-xl shadow p-6 border-l-4 border-green-500">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-500 font-semibold uppercase">Pendapatan</p>
            <p class="text-2xl font-bold text-gray-900 mt-1">Rp {{ formatUang(metrics.TotalIncome) }}</p>
          </div>
          <div class="p-3 bg-green-100 rounded-full">
            <TrendingUpIcon class="w-6 h-6 text-green-600" />
          </div>
        </div>
      </div>

      <!-- Kartu Pengeluaran -->
      <div class="bg-white rounded-xl shadow p-6 border-l-4 border-red-500">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-500 font-semibold uppercase">Pengeluaran</p>
            <p class="text-2xl font-bold text-gray-900 mt-1">Rp {{ formatUang(metrics.TotalExpense) }}</p>
          </div>
          <div class="p-3 bg-red-100 rounded-full">
            <TrendingDownIcon class="w-6 h-6 text-red-600" />
          </div>
        </div>
      </div>

      <!-- Kartu Laba Bersih -->
      <div class="bg-white rounded-xl shadow p-6 border-l-4 border-yellow-500">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-500 font-semibold uppercase">Laba Bersih</p>
            <p class="text-2xl font-bold mt-1" :class="metrics.NetProfit >= 0 ? 'text-green-600' : 'text-red-600'">
              Rp {{ formatUang(metrics.NetProfit) }}
            </p>
          </div>
          <div class="p-3 bg-yellow-100 rounded-full">
            <WalletIcon class="w-6 h-6 text-yellow-600" />
          </div>
        </div>
      </div>

    </div>
    
    <!-- Area Tabel Aktivitas (Lanjut di bawah) -->
    <RecentActivityTable class="mt-8" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { BuildingIcon, TrendingUpIcon, TrendingDownIcon, WalletIcon } from 'lucide-vue-next'
import RecentActivityTable from '@/components/RecentActivityTable.vue'

const metrics = ref({
  TotalAsset: 0,
  TotalIncome: 0,
  TotalExpense: 0,
  NetProfit: 0
})

const formatUang = (angka) => {
  return new Intl.NumberFormat('id-ID').format(angka)
}

onMounted(async () => {
  try {
    const res = await axios.get('http://localhost:8080/api/v1/dashboard/metrics')
    metrics.value = res.data.data
  } catch (error) {
    console.error("Gagal memuat metrik dashboard", error)
  }
})
</script>
```

---

## 10.3 Frontend: Tabel Transaksi Jurnal Terakhir (Recent Activity)

Selain kartu metrik, kita juga membutuhkan tabel yang menunjukkan *"Apa transaksi yang baru saja terjadi di perusahaan?"*.
Buat komponen baru bernama `RecentActivityTable.vue` untuk memanggil endpoint *GET Journal Entries* dengan batasan limit 5 teratas.

`frontend/src/components/RecentActivityTable.vue`
```vue
<template>
  <div class="bg-white rounded-xl shadow overflow-hidden">
    <div class="p-4 border-b bg-gray-50">
      <h3 class="font-bold text-gray-700">Aktivitas Jurnal Terakhir</h3>
    </div>
    <table class="w-full text-left">
      <thead>
        <tr class="text-sm text-gray-500 border-b">
          <th class="p-4">Voucher</th>
          <th class="p-4">Tanggal</th>
          <th class="p-4">Keterangan</th>
          <th class="p-4">Nominal</th>
          <th class="p-4">Status</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="journal in recentJournals" :key="journal.name" class="border-b hover:bg-gray-50">
          <td class="p-4 font-semibold text-blue-600">{{ journal.name }}</td>
          <td class="p-4">{{ journal.posting_date }}</td>
          <td class="p-4 truncate max-w-xs">{{ journal.user_remark }}</td>
          <td class="p-4 font-bold">Rp {{ formatUang(journal.total_debit) }}</td>
          <td class="p-4">
            <span v-if="journal.docstatus === 0" class="bg-gray-200 text-gray-700 px-2 py-1 rounded text-xs">Draft</span>
            <span v-else-if="journal.docstatus === 1" class="bg-blue-100 text-blue-700 px-2 py-1 rounded text-xs">Submitted</span>
            <span v-else class="bg-red-100 text-red-700 px-2 py-1 rounded text-xs">Cancelled</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
```

> 🔍 **[PENGUJIAN DASHBOARD]**  
> Restart server Frontend Vue Anda dan buka halaman `/dashboard`. Anda akan disambut dengan warna-warni indikator performa keuangan perusahaan! Cobalah buat jurnal beban listrik sebesar Rp 1.000.000 dan tekan *Submit*. Anda akan melihat kotak "Pengeluaran" naik, dan kotak "Laba Bersih" otomatis menjadi minus (merah). Keren sekali bukan?

---

➡️ **Langkah Berikutnya:** Wah! Kode aplikasi kita sudah 100% tuntas! Namun aplikasi yang hebat tidak berguna jika hanya berjalan di laptop *local* Anda. Di bab terakhir, kita akan belajar cara men-*deploy* (merilis) aplikasi kita ke Server Produksi. Lanjutkan ke bab pamungkas: [**BAB 11: PRODUKSI, DOCKER MULTI-STAGE, NGINX, & DEPLOYMENT**](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_11_PRODUCTION_DEPLOYMENT.md).
