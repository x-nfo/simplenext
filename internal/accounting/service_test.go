package accounting

import (
	"strings"
	"testing"
)

func TestValidateDebitCreditBalance(t *testing.T) {
	tests := []struct {
		name        string
		items       []CreateJournalEntryItemRequest
		expectError bool
		errContains string
		wantDebit   float64
		wantCredit  float64
	}{
		{
			name: "Valid Balanced Transaction",
			items: []CreateJournalEntryItemRequest{
				{Account: "1111 - Kas Kecil", Debit: 500000, Credit: 0},
				{Account: "5110 - Beban Gaji", Debit: 0, Credit: 500000},
			},
			expectError: false,
			wantDebit:   500000,
			wantCredit:  500000,
		},
		{
			name: "Multi-line Balanced Transaction",
			items: []CreateJournalEntryItemRequest{
				{Account: "5110 - Beban Gaji", Debit: 300000, Credit: 0},
				{Account: "5130 - Beban Listrik", Debit: 200000, Credit: 0},
				{Account: "1111 - Kas Kecil", Debit: 0, Credit: 500000},
			},
			expectError: false,
			wantDebit:   500000,
			wantCredit:  500000,
		},
		{
			name: "Unbalanced Transaction (Debit > Credit)",
			items: []CreateJournalEntryItemRequest{
				{Account: "1111 - Kas Kecil", Debit: 500000, Credit: 0},
				{Account: "5110 - Beban Gaji", Debit: 0, Credit: 400000},
			},
			expectError: true,
			errContains: "jurnal tidak seimbang",
		},
		{
			name: "Less than 2 rows",
			items: []CreateJournalEntryItemRequest{
				{Account: "1111 - Kas Kecil", Debit: 500000, Credit: 0},
			},
			expectError: true,
			errContains: "minimal 2 baris akun",
		},
		{
			name: "Empty Account Name",
			items: []CreateJournalEntryItemRequest{
				{Account: "", Debit: 1000, Credit: 0},
				{Account: "1111 - Kas Kecil", Debit: 0, Credit: 1000},
			},
			expectError: true,
			errContains: "akun tidak boleh kosong",
		},
		{
			name: "Negative Value",
			items: []CreateJournalEntryItemRequest{
				{Account: "1111 - Kas Kecil", Debit: -100, Credit: 0},
				{Account: "5110 - Beban Gaji", Debit: 0, Credit: 100},
			},
			expectError: true,
			errContains: "tidak boleh negatif",
		},
		{
			name: "Both Debit and Credit Zero",
			items: []CreateJournalEntryItemRequest{
				{Account: "1111 - Kas Kecil", Debit: 0, Credit: 0},
				{Account: "5110 - Beban Gaji", Debit: 0, Credit: 0},
			},
			expectError: true,
			errContains: "harus lebih besar dari 0",
		},
		{
			name: "Both Debit and Credit Positive in Single Row",
			items: []CreateJournalEntryItemRequest{
				{Account: "1111 - Kas Kecil", Debit: 500, Credit: 500},
				{Account: "5110 - Beban Gaji", Debit: 0, Credit: 500},
			},
			expectError: true,
			errContains: "satu baris tidak boleh memiliki nilai debit dan kredit sekaligus",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			debit, credit, err := ValidateDebitCreditBalance(tc.items)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tc.errContains)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("expected error to contain '%s', got '%s'", tc.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if debit != tc.wantDebit {
					t.Errorf("expected debit %.2f, got %.2f", tc.wantDebit, debit)
				}
				if credit != tc.wantCredit {
					t.Errorf("expected credit %.2f, got %.2f", tc.wantCredit, credit)
				}
			}
		})
	}
}

func TestCalculateNetBalance(t *testing.T) {
	tests := []struct {
		name        string
		rootType    string
		totalDebit  float64
		totalCredit float64
		expectedNet float64
	}{
		{
			name:        "Asset normal balance is Debit",
			rootType:    "Asset",
			totalDebit:  1000000,
			totalCredit: 250000,
			expectedNet: 750000,
		},
		{
			name:        "Expense normal balance is Debit",
			rootType:    "Expense",
			totalDebit:  500000,
			totalCredit: 50000,
			expectedNet: 450000,
		},
		{
			name:        "Liability normal balance is Credit",
			rootType:    "Liability",
			totalDebit:  200000,
			totalCredit: 800000,
			expectedNet: 600000,
		},
		{
			name:        "Equity normal balance is Credit",
			rootType:    "Equity",
			totalDebit:  0,
			totalCredit: 5000000,
			expectedNet: 5000000,
		},
		{
			name:        "Income normal balance is Credit",
			rootType:    "Income",
			totalDebit:  100000,
			totalCredit: 1500000,
			expectedNet: 1400000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateNetBalance(tc.rootType, tc.totalDebit, tc.totalCredit)
			if got != tc.expectedNet {
				t.Errorf("expected net balance %.2f, got %.2f", tc.expectedNet, got)
			}
		})
	}
}

func TestBuildAccountTree(t *testing.T) {
	parent := "1000 - Aset"
	accounts := []Account{
		{Name: "1000 - Aset", IsGroup: true, ParentAccount: nil},
		{Name: "1100 - Aset Lancar", IsGroup: true, ParentAccount: &parent},
	}

	tree := BuildAccountTree(accounts)
	if len(tree) != 1 {
		t.Fatalf("expected 1 root account, got %d", len(tree))
	}
	if tree[0].Name != "1000 - Aset" {
		t.Errorf("expected root name '1000 - Aset', got '%s'", tree[0].Name)
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("expected 1 child in root, got %d", len(tree[0].Children))
	}
	if tree[0].Children[0].Name != "1100 - Aset Lancar" {
		t.Errorf("expected child name '1100 - Aset Lancar', got '%s'", tree[0].Children[0].Name)
	}
}
