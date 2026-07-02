package utils

import (
	"testing"
)

// ============================================================
// TESTS UNITAIRES - HELPERS MONÉTAIRES
// ============================================================

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected string
	}{
		// Cas normaux avec arrondi
		{"5499 → 55 FCFA", 5499, "55 FCFA"},
		{"5400 → 54 FCFA", 5400, "54 FCFA"},
		{"5450 → 55 FCFA", 5450, "55 FCFA"},
		{"5449 → 54 FCFA", 5449, "54 FCFA"},

		// Grands montants
		{"50000 FCFA", 5000000, "50 000 FCFA"},
		{"1234567 FCFA", 123456789, "1 234 568 FCFA"},
		{"100000 FCFA", 10000000, "100 000 FCFA"},

		// Petits montants
		{"1 FCFA", 100, "1 FCFA"},
		{"0 FCFA", 0, "0 FCFA"},
		{"50 centimes → 1 FCFA", 50, "1 FCFA"},
		{"49 centimes → 0 FCFA", 49, "0 FCFA"},

		// Montants exacts
		{"100 centimes → 1 FCFA", 100, "1 FCFA"},
		{"1000 centimes → 10 FCFA", 1000, "10 FCFA"},

		// Négatifs
		{"-5499 → -55 FCFA", -5499, "-55 FCFA"},
		{"-50000 FCFA", -5000000, "-50 000 FCFA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMoney(tt.cents)
			if result != tt.expected {
				t.Errorf("FormatMoney(%d) = %s, want %s", tt.cents, result, tt.expected)
			}
		})
	}
}

func TestFormatMoneyNoRound(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected string
	}{
		{"5499 → 54 FCFA (troncature)", 5499, "54 FCFA"},
		{"5400 → 54 FCFA", 5400, "54 FCFA"},
		{"5450 → 54 FCFA", 5450, "54 FCFA"},
		{"5000000 → 50000 FCFA", 5000000, "50 000 FCFA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMoneyNoRound(tt.cents)
			if result != tt.expected {
				t.Errorf("FormatMoneyNoRound(%d) = %s, want %s", tt.cents, result, tt.expected)
			}
		})
	}
}

func TestFormatMoneyWithDecimals(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected string
	}{
		{"5499 → 54.99 FCFA", 5499, "54.99 FCFA"},
		{"5400 → 54.00 FCFA", 5400, "54.00 FCFA"},
		{"5000000 → 50000.00 FCFA", 5000000, "50000.00 FCFA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMoneyWithDecimals(tt.cents)
			if result != tt.expected {
				t.Errorf("FormatMoneyWithDecimals(%d) = %s, want %s", tt.cents, result, tt.expected)
			}
		})
	}
}

func TestRoundMoney(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected int64
	}{
		{"5499 → 5500", 5499, 5500},
		{"5400 → 5400", 5400, 5400},
		{"5450 → 5500", 5450, 5500},
		{"5449 → 5400", 5449, 5400},
		{"0 → 0", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundMoney(tt.cents)
			if result != tt.expected {
				t.Errorf("RoundMoney(%d) = %d, want %d", tt.cents, result, tt.expected)
			}
		})
	}
}

func TestCentsToFCFA(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected int64
	}{
		{"5499 → 55", 5499, 55},
		{"5400 → 54", 5400, 54},
		{"5450 → 55", 5450, 55},
		{"100 → 1", 100, 1},
		{"0 → 0", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CentsToFCFA(tt.cents)
			if result != tt.expected {
				t.Errorf("CentsToFCFA(%d) = %d, want %d", tt.cents, result, tt.expected)
			}
		})
	}
}

func TestFCFAtoCents(t *testing.T) {
	tests := []struct {
		name     string
		fcfa     int64
		expected int64
	}{
		{"55 → 5500", 55, 5500},
		{"54 → 5400", 54, 5400},
		{"1 → 100", 1, 100},
		{"0 → 0", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FCFAtoCents(tt.fcfa)
			if result != tt.expected {
				t.Errorf("FCFAtoCents(%d) = %d, want %d", tt.fcfa, result, tt.expected)
			}
		})
	}
}

func TestFormatBalance(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected string
	}{
		{"Positif: 45000 → 450 FCFA", 45000, "450 FCFA"},
		{"Négatif: -45000 → -450 FCFA", -45000, "-450 FCFA"},
		{"Zéro: 0 → 0 FCFA", 0, "0 FCFA"},
		{"Grand négatif: -5000000 → -50 000 FCFA", -5000000, "-50 000 FCFA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBalance(tt.cents)
			if result != tt.expected {
				t.Errorf("FormatBalance(%d) = %s, want %s", tt.cents, result, tt.expected)
			}
		})
	}
}

func TestFormatCommission(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected string
	}{
		{"1833 → 18 FCFA", 1833, "18 FCFA"},
		{"1850 → 19 FCFA", 1850, "19 FCFA"},
		{"5499 → 55 FCFA", 5499, "55 FCFA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCommission(tt.cents)
			if result != tt.expected {
				t.Errorf("FormatCommission(%d) = %s, want %s", tt.cents, result, tt.expected)
			}
		})
	}
}
