# BAB 3: PERSIAPAN LINGKUNGAN, DOCKER, & INISIALISASI PROYEK

Halo semuanya! Setelah kita memahami rancangan arsitektur dan database pada bab sebelumnya, sekarang saatnya kita mulai "*mengotori tangan*" kita dengan kode. 

Di bab ini, kita akan melakukan inisialisasi lingkungan kerja untuk keseluruhan proyek **SimpleNext**. Kita tidak akan menginstal database PostgreSQL secara manual di sistem operasi, melainkan menggunakan teknologi *containerization* **Docker** agar terisolasi dengan aman dan mudah dipindahkan.

---

## 3.1 Persyaratan Sistem & Tools

Pastikan Anda sudah menginstal daftar alat bantu berikut sebelum kita memulai pengetikan kode:

1. **Go (Golang)**: Minimal versi `1.22`. Unduh di [golang.org/dl](https://go.dev/dl/).
2. **Node.js**: Minimal versi `20+` LTS. Unduh di [nodejs.org](https://nodejs.org/).
3. **Docker Desktop** (atau Docker Engine + Docker Compose di Linux). Unduh di [docker.com](https://www.docker.com/).
4. **Git**: Untuk manajemen versi *source code*.
5. **Visual Studio Code (VS Code)**: Text editor yang direkomendasikan dengan *extension* **Go** dan **Vue - Official**.

---

## 3.2 Menyiapkan Lingkungan Proyek & Database (Docker)

Langkah pertama adalah membuat direktori kosong untuk proyek kita dan menata fondasinya.

Buka terminal kesayangan Anda dan jalankan perintah berikut:

```bash
mkdir simplenext
cd simplenext
```

### Membuat File Docker Compose untuk PostgreSQL
Agar kita tidak dipusingkan dengan konfigurasi PostgreSQL lokal, buatlah sebuah file bernama `docker-compose.yml` di dalam root direktori `simplenext/`.

`docker-compose.yml`
```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: simplenext-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: simplenext
    ports:
      - "5432:5432"
    volumes:
      - simplenext_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d simplenext"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  simplenext_pgdata:
```

> 💡 **[CATATAN/TIPS]**  
> Kita menggunakan tag `postgres:16-alpine` karena Alpine Linux adalah distribusi sistem operasi berukuran sangat kecil. Hal ini membuat proses *download image* jauh lebih cepat dibandingkan image reguler. 

---

## 3.3 Inisialisasi Proyek Backend (Golang)

Masih di terminal dalam direktori `simplenext`, sekarang kita inisialisasi manajemen dependensi Golang (Go Modules).

```bash
go mod init simplenext
```
Perintah ini akan membuat sebuah file bernama `go.mod`.

Selanjutnya, kita akan menginstal dua pustaka (*library*) eksternal andalan kita. Sesuai prinsip **SimpleNext**, kita menjaga dependensi agar setipis mungkin:
- `chi/v5` sebagai router HTTP berkecepatan tinggi.
- `pgx/v5` sebagai driver PostgreSQL murni yang sangat ngebut.

Jalankan perintah ini:
```bash
go get github.com/go-chi/chi/v5
go get github.com/jackc/pgx/v5
```

### Struktur Folder Clean Architecture
Sesuai konvensi standar proyek Go tingkat lanjut, buatlah folder-folder ini:

```bash
mkdir -p cmd/api
mkdir -p internal/accounting
mkdir -p internal/core
mkdir -p internal/platform/database
mkdir migrations
```

- `cmd/api`: Direktori tempat kode *startup* `main.go` yang akan dikompilasi.
- `internal`: Tempat menampung seluruh logika bisnis kita (*Clean Architecture*). Ingat, folder bernama `internal` di Go memiliki hak akses spesial (tidak dapat diimpor oleh proyek eksternal lain).
- `migrations`: Direktori untuk menyimpan file SQL migrasi struktur tabel (DDL).

---

## 3.4 Inisialisasi Proyek Frontend (Vue 3 + Vite)

Untuk *frontend*, kita akan menggunakan **Vue 3** berbasis **Composition API** agar mudah dibangun dan diintegrasikan. Kita menggunakan *build tool* **Vite** yang sangat cepat!

Buka terminal baru atau pastikan Anda berada di direktori `simplenext/`, kemudian ketik:

```bash
npm create vite@latest frontend -- --template vue
```

Kemudian, masuk ke folder frontend dan pasang package dasar yang kita butuhkan:
```bash
cd frontend
npm install

# Install Pinia (State Management) & Vue Router
npm install pinia vue-router@4

# Install Axios (HTTP Client untuk memanggil API Golang) & Lucide (Icons)
npm install axios lucide-vue-next
```

> ⚠️ **[PERINGATAN]**  
> Jangan lupa menjalankan `npm install` sebelum melakukan hal lain di dalam folder `frontend`, karena ini akan mengunduh ratusan dependensi yang tertulis di `package.json` Anda.

---

## 3.5 Pembuatan Makefile (Otomasi Developer)

Agar tidak perlu mengingat perintah panjang yang rumit, Go *developer* biasanya mengandalkan program utilitas bernama `make`. Buatlah sebuah file bernama `Makefile` *(tanpa ekstensi)* di root direktori `simplenext/`:

`Makefile`
```makefile
.PHONY: all run test test-unit test-integration docker-up docker-down migrate-up migrate-down seed

# Jalankan aplikasi HTTP API server backend
run:
	go run ./cmd/api

# Jalankan seluruh pengujian (Unit & Integration test)
test:
	go test -v -count=1 ./...

# Jalankan PostgreSQL Docker container
docker-up:
	docker compose up -d

# Hentikan PostgreSQL Docker container
docker-down:
	docker compose down

# Eksekusi migrasi skema tabel DDL
migrate-up:
	docker exec -i simplenext-postgres psql -U postgres -d simplenext < migrations/000001_init_schema.up.sql

# Seed data bagan akun (COA) standar ke dalam database
seed:
	docker exec -i simplenext-postgres psql -U postgres -d simplenext < migrations/000002_seed_accounts.sql
```

---

## 🔍 [PENGUJIAN LINGKUNGAN KERJA]

Mari kita uji hasil kerja keras kita di bab ini! Buka terminal Anda di direktori `simplenext/` dan ikuti tes berikut:

**Test 1: Menghidupkan Database**
Jalankan perintah ajaib dari Makefile kita:
```bash
make docker-up
```
*Tunggu proses download image selesai (tergantung koneksi internet Anda).*
Untuk mengecek apakah database hidup, jalankan: `docker ps`. Pastikan `simplenext-postgres` memiliki status *Up* dan tidak *Exit*.

**Test 2: Menghidupkan Frontend Vue 3**
Buka tab terminal baru, navigasi ke folder frontend:
```bash
cd frontend
npm run dev
```
Kunjungi URL yang muncul di terminal (biasanya `http://localhost:5173`). Jika muncul logo Vue yang dapat diklik, artinya frontend Anda siap!

Jika semuanya berjalan lancar, selamat! 🎉  
Anda telah berhasil membangun pondasi lingkungan (*Environment*) dari sistem ERP berskala enterprise. 

---

➡️ **Langkah Berikutnya:** Pada bab selanjutnya, kita akan membangun pondasi keamanan sistem melalui implementasi Login, Token JWT, dan Hak Akses (Role-Based Access Control). Lanjutkan ke [**BAB 4: OTENTIKASI, JWT, & ROLE-BASED ACCESS CONTROL (RBAC)**](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_04_AUTH_RBAC.md).
