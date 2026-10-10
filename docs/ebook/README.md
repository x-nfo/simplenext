# E-Book: Membangun Sistem ERP & Akuntansi "SimpleNext" dari Nol hingga Production
### *Panduan Langkah-demi-Langkah (Step-by-Step Project-Based) Menggunakan Golang, PostgreSQL, dan Vue 3*

---

Selamat datang di seri e-book tutorial pembangunan aplikasi web modern fullstack **SimpleNext**! E-book ini disusun dengan gaya penulisan praktis, ramah, dan *action-oriented* (ala **SantriKoding**), di mana setiap bab dirancang untuk memberikan progres nyata yang dapat langsung diuji coba.

---

## 🧭 Silabus & Daftar Isi Lengkap

| Bab | Judul Modul / Materi | Status | Dokumen File |
| :---: | :--- | :---: | :--- |
| **01** | **Pendahuluan & Gambaran Proyek SimpleNext** | ✅ Tersedia | [`BAB_01_PENDAHULUAN.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_01_PENDAHULUAN.md) |
| **02** | **Perancangan Database & Relasi (PostgreSQL)** | ✅ Tersedia | [`BAB_02_DATABASE_DESIGN.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_02_DATABASE_DESIGN.md) |
| **03** | **Persiapan Lingkungan, Docker, & Inisialisasi Proyek** | ✅ Tersedia | [`BAB_03_SETUP_PROJECT.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_03_SETUP_PROJECT.md) |
| **04** | **Otentikasi, JWT, & Role-Based Access Control (RBAC)** | ✅ Tersedia | [`BAB_04_AUTH_RBAC.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_04_AUTH_RBAC.md) |
| **05** | **Database Migration (DDL), pgx Driver, & Seeder** | ✅ Tersedia | [`BAB_05_MIGRATION_SEEDER.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_05_MIGRATION_SEEDER.md) |
| **06** | **Modul 1: Atomic Naming Series Engine (Core Platform)** | ✅ Tersedia | [`BAB_06_NAMING_SERIES.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_06_NAMING_SERIES.md) |
| **07** | **Modul 2: Chart of Accounts (Bagan Akun Hierarkis & Tree UI)** | ✅ Tersedia | [`BAB_07_CHART_OF_ACCOUNTS.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_07_CHART_OF_ACCOUNTS.md) |
| **08** | **Modul 3: Journal Entry & Siklus Transaksi Dokumen (Draft-Submit-Cancel)** | ✅ Tersedia | [`BAB_08_JOURNAL_ENTRY.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_08_JOURNAL_ENTRY.md) |
| **09** | **Modul 4: General Ledger & Laporan Keuangan Realtime (Buku Besar & Neraca)** | ✅ Tersedia | [`BAB_09_GENERAL_LEDGER_REPORTS.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_09_GENERAL_LEDGER_REPORTS.md) |
| **10** | **Modul 5: Dashboard Finansial & Visualisasi Metrik** | ✅ Tersedia | [`BAB_10_DASHBOARD_METRICS.md`](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_10_DASHBOARD_METRICS.md) |
| **11** | **Produksi, Docker Multi-Stage, Nginx, & Deployment** | ⏳ Segera | `BAB_11_PRODUCTION_DEPLOYMENT.md` |

---

## 🎯 Target Pembaca & Prasyarat

E-book ini ditujukan untuk **Programmer Pemula hingga Menengah** yang ingin:
1. Memahami bagaimana arsitektur aplikasi backend Golang berskala industri dirancang tanpa *framework bloatware*.
2. Mempelajari konsep inti sistem enterprise seperti *Document Lifecycle*, *Double-Entry Bookkeeping*, dan *Immutable Audit Trail*.
3. Membangun SPA (*Single Page Application*) modern dengan Vue 3 (Composition API), Vite, dan Pinia yang terhubung langsung ke RESTful API.

### Kebutuhan Sistem Minimum:
- **Go**: Versi `1.22` atau lebih baru
- **Node.js**: Versi `20+` LTS & `npm`
- **PostgreSQL**: Versi `16+` (dijalankan via Docker Compose)
- **Docker & Docker Compose**
- **Git** & Editor teks favorit (VS Code / Antigravity IDE)

---

## 💡 Konvensi Tanda & Callout

Sepanjang e-book ini, Anda akan sering menemui kotak penanda khusus berikut:

> 💡 **[CATATAN/TIPS]**  
> Berisi penjelasan trik efisiensi, standar konvensi industri, atau *best practice* yang penting untuk diingat.

> ⚠️ **[PERINGATAN]**  
> Peringatan error umum, potensi *bug*, atau hal-hal krusial yang tidak boleh Anda lewatkan saat mengikuti langkah tutorial.

> 🔍 **[PENGUJIAN]**  
> Panduan cara mengeksekusi, menjalankan perintah di terminal, dan memverifikasi hasil di browser atau REST client.

---

*Mari kita mulai petualangan membangun aplikasi SimpleNext dari bab pertama!*
