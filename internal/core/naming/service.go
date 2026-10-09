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

// ParsePattern mem-parse pola seri dan mengembalikan prefix serta jumlah padding angka.
// Contoh: "JV-.YYYY.-.#####" -> prefix: "JV-2026-", padLen: 5
func ParsePattern(seriesPattern string, now time.Time) (prefix string, padLen int) {
	padLen = 0
	trimmedPattern := seriesPattern
	for strings.HasSuffix(trimmedPattern, "#") {
		padLen++
		trimmedPattern = trimmedPattern[:len(trimmedPattern)-1]
	}
	if padLen == 0 {
		padLen = 5
	}
	trimmedPattern = strings.TrimSuffix(trimmedPattern, ".")

	prefix = trimmedPattern
	prefix = strings.ReplaceAll(prefix, ".YYYY.", fmt.Sprintf("%04d", now.Year()))
	prefix = strings.ReplaceAll(prefix, ".YY.", fmt.Sprintf("%02d", now.Year()%100))
	prefix = strings.ReplaceAll(prefix, ".MM.", fmt.Sprintf("%02d", int(now.Month())))
	prefix = strings.ReplaceAll(prefix, ".DD.", fmt.Sprintf("%02d", now.Day()))

	prefix = strings.ReplaceAll(prefix, ".-.", "-")
	prefix = strings.ReplaceAll(prefix, "..", ".")
	return prefix, padLen
}

// FormatSeries memformat prefix, panjang padding nol, dan nilai urut angka menjadi string kode dokumen.
func FormatSeries(prefix string, padLen int, currentValue int64) string {
	return fmt.Sprintf("%s%0*d", prefix, padLen, currentValue)
}

// Generate menghasilkan kode dokumen berikutnya berdasarkan pola ERPNext secara atomik.
// Contoh pola: "JV-.YYYY.-.#####" -> "JV-2026-00001"
func (s *Service) Generate(ctx context.Context, seriesPattern string) (string, error) {
	prefix, padLen := ParsePattern(seriesPattern, time.Now())

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
