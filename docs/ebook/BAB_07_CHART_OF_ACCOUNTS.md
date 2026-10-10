# BAB 7: MODUL 2 - CHART OF ACCOUNTS (BAGAN AKUN HIERARKIS)

Halo semuanya! Jika modul *Naming Series* di bab sebelumnya bertugas memberi nomor urut, maka modul **Chart of Accounts (Bagan Akun/COA)** bertugas sebagai "peta" atau "kerangka tulang" dari sistem keuangan kita.

Tanpa Bagan Akun yang rapi, kita tidak akan tahu apakah uang perusahaan digunakan untuk membayar sewa gedung, membeli bahan baku, atau membayar gaji karyawan. Di bab ini, kita akan merancang struktur COA yang dinamis, tidak datar (*flat*), melainkan berbentuk pohon bercabang (*Hierarchical Tree*).

---

## 7.1 Konsep Pohon Akun: Group vs Ledger

Di aplikasi ERP, kita mengenal dua bentuk akun:
1. **Group Account (Akun Grup):** Berfungsi seperti "folder" atau "map". Akun ini tidak boleh menampung transaksi jurnal secara langsung. Tujuannya hanya untuk mengelompokkan akun-akun di bawahnya agar laporan keuangan mudah dibaca.
2. **Ledger Account (Akun Buku Besar):** Berfungsi seperti "file" di dalam folder. Akun inilah yang benar-benar kita gunakan saat membuat transaksi keuangan (Debit/Kredit).

**Contoh Visualisasinya:**
```text
📂 Asset (Group / Root)
 └── 📂 Current Asset (Group)
     └── 📂 Bank and Cash (Group)
         ├── 📄 1110 - Kas Kecil (Ledger) ---> Bisa di-Jurnal
         └── 📄 1111 - Bank BCA (Ledger) ---> Bisa di-Jurnal
```

---

## 7.2 Backend: Model Data (Struct) & DTO

Pertama, mari kita terjemahkan skema SQL `accounts` kita ke dalam *Struct* Golang. Ingat, karena kita ingin membentuk JSON berupa pohon (nested array `children`), kita menyelipkan atribut `Children []*Account` di dalam struct.

Buka (atau perbarui) file: `internal/accounting/model.go`
```go
package accounting

import "time"

// Account merepresentasikan satu baris dalam Bagan Akun
type Account struct {
	Name          string     `json:"name"`           // Primary Key (Contoh: "1110 - Kas Kecil")
	AccountNumber string     `json:"account_number"` // "1110"
	AccountName   string     `json:"account_name"`   // "Kas Kecil"
	AccountType   string     `json:"account_type"`   // "Cash", "Bank", "Expense"
	RootType      string     `json:"root_type"`      // "Asset", "Liability", "Equity", "Income", "Expense"
	IsGroup       bool       `json:"is_group"`       // true jika "folder", false jika "ledger"
	ParentAccount *string    `json:"parent_account"` // Pointer ke nama akun induk
	CreatedAt     time.Time  `json:"created_at"`
	
	// Atribut Rekursif untuk JSON Frontend (dihilangkan jika kosong berkat "omitempty")
	Children      []*Account `json:"children,omitempty"` 
}
```

---

## 7.3 Backend: Service - Logika Pembentuk Pohon (Tree Builder)

Database SQL standar membaca data dalam bentuk baris yang datar (*flat rows*). Oleh karena itu, kita harus menulis sebuah algoritma cerdas di Golang untuk mengubah ratusan baris data pipih (*flat*) tersebut menjadi struktur pohon bersarang (*nested JSON*).

Buat file *Business Logic* kita:
`internal/accounting/service.go`
```go
package accounting

import (
	"context"
	"fmt"
)

// BuildTree mengubah array linear Account menjadi struktur pohon bersarang.
// Algoritma ini berjalan dalam kompleksitas O(N) menggunakan Hash Map (Map di Go).
func BuildTree(accounts []Account) []*Account {
	// Buat peta (lookup map) pointer ke setiap akun agar mudah dirangkai
	accountMap := make(map[string]*Account)
	for i := range accounts {
		accountMap[accounts[i].Name] = &accounts[i]
	}

	var rootAccounts []*Account

	for i := range accounts {
		acc := &accounts[i]
		// Jika akun tidak memiliki induk, ia adalah cabang utama (Root)
		if acc.ParentAccount == nil {
			rootAccounts = append(rootAccounts, acc)
		} else {
			// Jika akun memiliki induk, cari induknya di peta, lalu masukkan diri ke dalam 'Children' induk tersebut
			parent, exists := accountMap[*acc.ParentAccount]
			if exists {
				parent.Children = append(parent.Children, acc)
			}
		}
	}

	return rootAccounts
}
```

> 💡 **[CATATAN/TIPS]**  
> Banyak developer menggunakan cara perulangan bersarang `for` di dalam `for` (O(N²)) atau fungsi rekursif database (CTE) yang lambat untuk menyusun pohon. Namun, menggunakan *Hash Map Pointer* seperti contoh di atas membuat proses *Tree Building* di Golang super kilat meskipun perusahaan memiliki 10.000 bagan akun!

---

## 7.4 Backend: Handler & Endpoint REST API

Agar data ini dapat dikonsumsi oleh Vue 3, kita butuh sebuah *Controller* (Handler HTTP).

Buat file: `internal/accounting/handler.go`
```go
package accounting

import (
	"encoding/json"
	"net/http"
)

// Meniru struktur router chi
func (h *Handler) GetAccountTree(w http.ResponseWriter, r *http.Request) {
	// 1. Ambil semua akun dari database melalui Repository (h.repo.GetAllAccounts)
	accounts, err := h.repo.GetAllAccounts(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Gagal mengambil data akun"}`, http.StatusInternalServerError)
		return
	}

	// 2. Susun menjadi pohon
	tree := BuildTree(accounts)

	// 3. Kembalikan balasan dalam format JSON Envelope
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Berhasil memuat bagan akun",
		"data":    tree,
	})
}
```

---

## 7.5 Frontend: Komponen Rekursif Vue 3 (Treeview UI)

Menerima JSON pohon bersarang adalah tantangan besar di antarmuka pengguna (*Frontend*). Bagaimana cara mencetak menu lipat jika kita tidak tahu seberapa dalam cabangnya?

Jawabannya adalah: **Komponen Vue yang memanggil dirinya sendiri (Rekursif)**.

Buat komponen di Vue: `frontend/src/components/TreeNode.vue`
```vue
<template>
  <div class="ml-6 border-l-2 border-gray-200 pl-4 py-1">
    <!-- Header Node -->
    <div 
      class="flex items-center gap-2 cursor-pointer hover:bg-blue-50 p-2 rounded"
      @click="toggleExpand"
    >
      <!-- Ikon Folder vs Dokumen -->
      <FolderIcon v-if="node.is_group" class="w-5 h-5 text-yellow-500" />
      <FileTextIcon v-else class="w-5 h-5 text-gray-500" />
      
      <span :class="{'font-bold text-gray-800': node.is_group, 'text-gray-600': !node.is_group}">
        {{ node.name }}
      </span>
      
      <!-- Label Tipe Akun -->
      <span class="text-xs bg-gray-100 px-2 py-0.5 rounded text-gray-500 ml-auto">
        {{ node.account_type }}
      </span>
    </div>

    <!-- Recursive Children (Memanggil Diri Sendiri) -->
    <div v-if="isExpanded && node.children && node.children.length > 0">
      <TreeNode 
        v-for="child in node.children" 
        :key="child.name" 
        :node="child" 
      />
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { FolderIcon, FileTextIcon } from 'lucide-vue-next'

const props = defineProps({
  node: { type: Object, required: true }
})

const isExpanded = ref(false)

const toggleExpand = () => {
  if (props.node.is_group) {
    isExpanded.value = !isExpanded.value
  }
}
</script>
```

> ⚠️ **[PERINGATAN]**  
> Saat menulis komponen rekursif di Vue, pastikan selalu ada kondisi penutup (seperti `v-if="node.children && node.children.length > 0"`). Jika tidak ada batasannya, Vue akan merender *looping* tanpa henti hingga browser Anda *crash* / *Out of Memory*!

---

## 🔍 [PENGUJIAN & VERIFIKASI COA]

Sekarang, mari kita tes logika integrasi kita:

1. **Jalankan Backend Golang:** Buka terminal dan ketik `make run`. Server akan mendengarkan di port `8080`.
2. **Uji API via cURL / Postman / Browser:**  
   Kunjungi URL `http://localhost:8080/api/v1/accounts/tree`.  
   Jika algoritma `BuildTree` kita benar, Anda akan melihat JSON di mana "Kas Kecil" berada di dalam array `children` milik "Bank and Cash", yang berada di dalam `children` milik "Current Asset", dan seterusnya.
3. **Jalankan Frontend Vue:** Ketik `npm run dev` dan buka halaman Chart of Accounts. Klik ikon folder kuning untuk melihat folder tersebut terbuka dengan mulus berkat komponen rekursif `TreeNode`!

---

➡️ **Langkah Berikutnya:** Bagan akun sudah siap! Sekarang saatnya kita mulai memasukkan uang (fiktif) ke dalam sistem melalui mekanisme "Journal Entry". Kita akan belajar bagaimana memvalidasi agar Total Debit selalu sama dengan Total Kredit. Lanjutkan ke [**BAB 8: MODUL 3 - JOURNAL ENTRY & SIKLUS TRANSAKSI DOKUMEN**](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_08_JOURNAL_ENTRY.md).
