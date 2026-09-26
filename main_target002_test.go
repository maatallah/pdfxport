package main

import (
	"fmt"
	"testing"
)

// TestPaireCondition verifies the TARGET-002 Paire rule:
// Trigger only when both À gauche and À droite are strictly positive AND equal.
func TestPaireCondition(t *testing.T) {
	testCases := []struct {
		leftStr     string
		rightStr    string
		expectPaire bool
	}{
		// Equal zero values: do not trigger
		{"0", "0", false},
		{"0.0", "0.0", false},

		// One zero: do not trigger
		{"0", "10", false},
		{"10", "0", false},
		{"0", "0.1", false},
		{"0.1", "0", false},

		// Different values: do not trigger
		{"3", "4", false},
		{"150", "151", false},
		{"0.1", "0.2", false},
		{"20", "25", false},
		{"100", "50", false},

		// Equal positive values: trigger
		{"0.1", "0.1", true},
		{"3", "3", true},
		{"20", "20", true},
		{"150", "150", true},
		{"649", "649", true},
		{"20.5", "20.5", true},
		{"0.01", "0.01", true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s", tc.leftStr, tc.rightStr), func(t *testing.T) {
			block := fmt.Sprintf(`Sur-mesure: Store
Commande: 9999.00001.001
Nom: TestClient
Référence: TestRef
Hauteur: 200
Largeur: 100
À gauche %s
À droite %s
`, tc.leftStr, tc.rightStr)

			recs := parseBlock(block)

			// When Paire split triggers, parseBlock produces 2 records with half-width (50 x 200)
			isPaireApplied := len(recs) == 2 && recs[0].Size == "50 x 200" && recs[1].Size == "50 x 200"

			if isPaireApplied != tc.expectPaire {
				t.Errorf("For À gauche=%s, À droite=%s: got isPaireApplied=%v, want %v (recs=%+v)",
					tc.leftStr, tc.rightStr, isPaireApplied, tc.expectPaire, recs)
			}
		})
	}
}

// TestExcelDimensionRounding verifies the TARGET-002 Excel dimension rounding rule:
// Ceil rounding applied independently to each dimension.
func TestExcelDimensionRounding(t *testing.T) {
	testCases := []struct {
		inputSize  string
		expectSize string
	}{
		// Decimal dimensions: ceil up to next integer
		{"164.01", "165"},
		{"164.25", "165"},
		{"164.75", "165"},
		{"164.99", "165"},

		// Integer dimensions: remain unchanged
		{"164.00", "164"},
		{"165.00", "165"},
		{"164", "164"},
		{"165", "165"},

		// Complete dimensions: rounded independently
		{"164.75 x 275.01", "165 x 276"},
		{"164.01 x 275.00", "165 x 275"},
		{"164.00 x 275.99", "164 x 276"},
		{"100 x 200", "100 x 200"},
		{"104.75 x 240", "105 x 240"},
		{"62.5 x 196", "63 x 196"},
		{"89.4 x 202.7", "90 x 203"},
		{"132.5 x 256", "133 x 256"},
		{"288.25 x 272", "289 x 272"},
		{"183.5 x 231.5", "184 x 232"},

		// Empty or edge cases
		{"", ""},
		{"   ", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.inputSize, func(t *testing.T) {
			got := formatSizeForExcel(tc.inputSize)
			if got != tc.expectSize {
				t.Errorf("formatSizeForExcel(%q) = %q; want %q", tc.inputSize, got, tc.expectSize)
			}
		})
	}
}
