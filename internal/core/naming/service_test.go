package naming

import (
	"testing"
	"time"
)

func TestParsePattern(t *testing.T) {
	fixedTime := time.Date(2026, time.March, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		pattern        string
		expectedPrefix string
		expectedPadLen int
	}{
		{
			name:           "Standard JV Pattern",
			pattern:        "JV-.YYYY.-.#####",
			expectedPrefix: "JV-2026-",
			expectedPadLen: 5,
		},
		{
			name:           "Short Year and Month Pattern",
			pattern:        "INV-.YY.-.MM.-.####",
			expectedPrefix: "INV-26-03-",
			expectedPadLen: 4,
		},
		{
			name:           "Full Date Pattern",
			pattern:        "PO-.YYYY.-.MM.-.DD.-.###",
			expectedPrefix: "PO-2026-03-15-",
			expectedPadLen: 3,
		},
		{
			name:           "No Date Tokens",
			pattern:        "ACC-#####",
			expectedPrefix: "ACC-",
			expectedPadLen: 5,
		},
		{
			name:           "Default PadLen When No Hash",
			pattern:        "DOC-",
			expectedPrefix: "DOC-",
			expectedPadLen: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prefix, padLen := ParsePattern(tc.pattern, fixedTime)
			if prefix != tc.expectedPrefix {
				t.Errorf("expected prefix '%s', got '%s'", tc.expectedPrefix, prefix)
			}
			if padLen != tc.expectedPadLen {
				t.Errorf("expected padLen %d, got %d", tc.expectedPadLen, padLen)
			}
		})
	}
}

func TestFormatSeries(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		padLen       int
		currentValue int64
		expected     string
	}{
		{
			name:         "5 digit padding single digit",
			prefix:       "JV-2026-",
			padLen:       5,
			currentValue: 1,
			expected:     "JV-2026-00001",
		},
		{
			name:         "5 digit padding double digits",
			prefix:       "JV-2026-",
			padLen:       5,
			currentValue: 42,
			expected:     "JV-2026-00042",
		},
		{
			name:         "3 digit padding overflow",
			prefix:       "ACC-",
			padLen:       3,
			currentValue: 1234,
			expected:     "ACC-1234",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatSeries(tc.prefix, tc.padLen, tc.currentValue)
			if got != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, got)
			}
		})
	}
}
