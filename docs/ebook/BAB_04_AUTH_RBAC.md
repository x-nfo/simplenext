# BAB 4: OTENTIKASI, JWT, & ROLE-BASED ACCESS CONTROL (RBAC)

Halo semuanya! Tidak ada sistem ERP yang membiarkan sembarang orang masuk dan mengubah data keuangan, bukan? Di bab ini, kita akan membangun "Gerbang Keamanan" alias sistem Autentikasi dan Otorisasi (Hak Akses) untuk aplikasi SimpleNext. 

Kita akan menggunakan **JSON Web Token (JWT)** dipadukan dengan **Bcrypt** untuk enkripsi password. Selain itu, kita akan membedakan hak akses (*Role-Based Access Control / RBAC*) antara peran `Admin`, `Accountant`, dan `Viewer`.

---

## 4.1 Desain Model User & Enkripsi Password (Bcrypt)

Data pengguna beserta otorisasi hak aksesnya akan kita simpan di tabel `users`. Dalam Golang, kita tidak boleh menyimpan password dalam bentuk teks murni (*plaintext*). Kita harus "mengacak"-nya menjadi *hash* menggunakan algoritma `Bcrypt`.

Agar backend kita bisa membaca tipe data tersebut, buat file model pengguna terlebih dahulu:

`internal/auth/model.go`
```go
package auth

import "time"

// User merepresentasikan tabel 'users' di database
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Disembunyikan dari output JSON (Tanda "-")
	FullName     string    `json:"full_name"`
	Role         string    `json:"role"` // "Admin", "Accountant", "Viewer"
	CreatedAt    time.Time `json:"created_at"`
}

// LoginRequest adalah Data Transfer Object (DTO) untuk menangkap input login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse adalah struktur balasan JSON setelah berhasil login
type LoginResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}
```

> 💡 **[CATATAN/TIPS]**  
> Perhatikan pada `PasswordHash string json:"-"`. Tanda minus (`-`) pada *struct tag* JSON di Go sangat krusial! Ini memastikan saat kita mengembalikan objek User ke Frontend, kolom password (meski sudah di-hash) tidak akan pernah ikut terkirim secara tidak sengaja.

---

## 4.2 Instalasi Pustaka JWT & Bcrypt

Sebelum kita bisa menerbitkan "Tiket Masuk" berupa JWT atau mengenkripsi password, kita butuh dua pustaka ini. Buka terminal Anda dan jalankan perintah:

```bash
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
```

- `golang-jwt/jwt/v5`: Digunakan untuk menandatangani (Sign) dan memverifikasi token JWT.
- `crypto/bcrypt`: Implementasi resmi Bcrypt di Go untuk hashing password.

---

## 4.3 Membangun Middleware Keamanan di Golang

Middleware adalah penjaga pintu. Setiap *Request HTTP* dari Frontend yang mencoba mengakses data keuangan (seperti *Jurnal* atau *Bagan Akun*), akan melewati middleware ini.

Buatlah file middleware:

`internal/auth/middleware.go`
```go
package auth

import (
	"context"
	"net/http"
	"strings"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecretKey = []byte("RahasiaSuperKuatSimpleNext2026") // Ingat, simpan ini di .env saat production!

// AuthMiddleware memeriksa apakah ada token JWT yang sah di header
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		
		// Parsing & Verifikasi Token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecretKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, `{"error": "Invalid Token"}`, http.StatusUnauthorized)
			return
		}

		// Mengambil informasi "Role" dari payload JWT
		claims, _ := token.Claims.(jwt.MapClaims)
		role := claims["role"].(string)

		// Simpan role ke Context HTTP agar bisa dibaca Handler
		ctx := context.WithValue(r.Context(), "userRole", role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RoleGuard adalah pelindung ekstra khusus untuk hak akses tertentu
func RoleGuard(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := r.Context().Value("userRole").(string)
			
			for _, allowed := range allowedRoles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return // Izinkan akses
				}
			}
			
			// Jika role tidak ada di daftar yang diizinkan
			http.Error(w, `{"error": "Forbidden: Anda tidak memiliki hak akses ini"}`, http.StatusForbidden)
		})
	}
}
```

> ⚠️ **[PERINGATAN]**  
> Bedakan antara `401 Unauthorized` dan `403 Forbidden`!  
> - `401` berarti: "Saya tidak tahu siapa Anda, silakan login". (Gagal di `AuthMiddleware`).  
> - `403` berarti: "Saya tahu Anda siapa (misal: Viewer), tapi Anda dilarang mengakses halaman ini". (Gagal di `RoleGuard`).

---

## 4.4 Manajemen Auth State di Frontend Vue 3 (Pinia & Axios)

Sekarang kita berpindah ke sudut pandang **Frontend (Vue 3)**. Bagaimana cara web menyimpan token JWT yang dikirim backend? Kita akan menggunakan **Pinia Store** dan **Axios Interceptor** agar token tersebut selalu diselipkan di setiap panggilan API secara otomatis.

Buat file *Store* ini di dalam proyek Vue Anda:

`frontend/src/stores/auth.js`
```javascript
import { defineStore } from 'pinia'
import axios from 'axios'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || null,
    role: localStorage.getItem('role') || null,
  }),
  getters: {
    isAuthenticated: (state) => !!state.token,
    isAdmin: (state) => state.role === 'Admin',
  },
  actions: {
    async login(email, password) {
      try {
        const response = await axios.post('http://localhost:8080/api/v1/auth/login', {
          email,
          password
        })
        
        // Simpan token ke state & local storage
        this.token = response.data.token
        this.role = response.data.role
        localStorage.setItem('token', this.token)
        localStorage.setItem('role', this.role)
        
        // Atur header default Axios
        axios.defaults.headers.common['Authorization'] = `Bearer ${this.token}`
        
        return true
      } catch (error) {
        console.error("Login gagal:", error.response.data.error)
        return false
      }
    },
    logout() {
      this.token = null
      this.role = null
      localStorage.removeItem('token')
      localStorage.removeItem('role')
      delete axios.defaults.headers.common['Authorization']
    }
  }
})
```

### Vue Router Navigation Guard

Terakhir, mari kita cegah agar user tidak bisa iseng mengetikkan URL `/dashboard` atau `/journal-entry` di kolom *address bar browser* tanpa melakukan login.

Ubah konfigurasi *Vue Router* Anda:

`frontend/src/router/index.js`
```javascript
import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import LoginView from '../views/LoginView.vue'
import DashboardView from '../views/DashboardView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/login', component: LoginView },
    { 
      path: '/dashboard', 
      component: DashboardView,
      meta: { requiresAuth: true } // Penanda bahwa halaman ini dilindungi
    }
  ]
})

// Navigation Guard
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()
  
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    // Jika belum login, tendang balik ke halaman /login
    next('/login')
  } else {
    // Silakan lewat
    next()
  }
})

export default router
```

---

## 🔍 [PENGUJIAN ALUR KERJA LOGIN]

Mari kita tes logika keamanan sistem yang telah kita buat:

1. Buka browser dan paksa ketik URL `http://localhost:5173/dashboard`.
2. Jika *Vue Router Navigation Guard* Anda bekerja dengan baik, browser akan membatalkan *loading* halaman dashboard secara kilat dan segera mengalihkan *(redirect)* Anda ke halaman `http://localhost:5173/login`.
3. Setelah login, token akan tersimpan dengan aman di dalam *Local Storage*, dan backend Chi (Go) akan mengenalinya lewat `AuthMiddleware` setiap kali Anda mengambil data akuntansi.

Keren bukan? Gerbang keamanan aplikasi kita sudah sangat kokoh!

---

➡️ **Langkah Berikutnya:** Setelah gerbang diamankan, kita butuh "benda nyata" di dalam database untuk diakses. Pada bab selanjutnya, kita akan mengeksekusi fondasi sesungguhnya dari tabel-tabel tersebut. Lanjutkan ke [**BAB 5: DATABASE MIGRATION (DDL), PGX DRIVER, & SEEDER**](file:///home/sandi/workspace/simplenext/docs/ebook/BAB_05_MIGRATION_SEEDER.md).
