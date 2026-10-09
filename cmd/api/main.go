package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"simplenext/internal/accounting"
	"simplenext/internal/core/naming"
	"simplenext/internal/platform/database"
)

func main() {
	// 1. Konfigurasi
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/simplenext?sslmode=disable"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Inisialisasi Database Connection Pool
	log.Println("[INFO] Menghubungkan ke PostgreSQL...")
	dbPool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("[FATAL] Gagal inisialisasi koneksi database: %v", err)
	}
	defer dbPool.Close()
	log.Println("[INFO] Terhubung ke PostgreSQL dengan sukses.")

	// 3. Inisialisasi Services & Handlers
	namingService := naming.NewService(dbPool)
	accountingRepo := accounting.NewRepository(dbPool)
	accountingService := accounting.NewService(accountingRepo, namingService)
	accountingHandler := accounting.NewHandler(accountingService)

	// 4. Setup Router HTTP (Chi)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","app":"simplenext"}`))
	})

	// Register Domain Routes
	accountingHandler.RegisterRoutes(r)

	// 5. Jalankan HTTP Server dengan Graceful Shutdown
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("[INFO] Server SimpleNext mendengarkan pada port :%s\n", port)
		serverErrors <- server.ListenAndServe()
	}()

	// Menunggu sinyal OS untuk shutdown (Ctrl+C, SIGTERM)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("[INFO] Sinyal penonaktifan diterima (%v). Menghentikan server...\n", sig)
		ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()

		if err := server.Shutdown(ctxShutdown); err != nil {
			log.Fatalf("[FATAL] Gagal menghentikan server secara graceful: %v", err)
		}
		log.Println("[INFO] Server berhasil dimatikan dengan aman.")
	}
}
