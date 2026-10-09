.PHONY: all run test test-unit test-integration docker-up docker-down migrate-up migrate-down seed

all: test run

# Jalankan aplikasi HTTP API server
run:
	go run ./cmd/api

# Jalankan seluruh pengujian (Unit & Integration test)
test:
	go test -v -count=1 ./...

# Jalankan hanya unit test
test-unit:
	go test -v -count=1 ./internal/core/naming ./internal/core/types

# Jalankan integration test siklus akuntansi
test-integration:
	go test -v -count=1 ./internal/accounting

# Jalankan PostgreSQL Docker container
docker-up:
	docker compose up -d

# Hentikan PostgreSQL Docker container
docker-down:
	docker compose down

# Eksekusi migrasi skema tabel
migrate-up:
	docker exec -i simplenext-postgres psql -U postgres -d simplenext < migrations/000001_init_schema.up.sql

# Rollback skema tabel
migrate-down:
	docker exec -i simplenext-postgres psql -U postgres -d simplenext < migrations/000001_init_schema.down.sql

# Seed data bagan akun (COA) standar
seed:
	docker exec -i simplenext-postgres psql -U postgres -d simplenext < migrations/000002_seed_accounts.sql
