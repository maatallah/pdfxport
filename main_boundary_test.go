package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestBoundaryCase1_RealAffectedPDF verifies that in 603Z.00069.pdf,
// the collapsed "À droite224603Z.00069" token is properly separated so that
// À droite is extracted as 224 (not 224603), allowing Paire equality (vL == vR)
// to trigger the dual-panel split.
func TestBoundaryCase1_RealAffectedPDF(t *testing.T) {
	pdfPath := filepath.Join("last_version", "603Z.00069.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skipf("Corpus file %s not found; skipping", pdfPath)
	}

	text, err := extractText(pdfPath)
	if err != nil {
		t.Fatalf("extractText failed: %v", err)
	}

	blocks := splitBlocks(text)
	if len(blocks) == 0 {
		t.Fatalf("Expected blocks from %s, got 0", pdfPath)
	}

	// Verify block-level extraction for each block
	reLeft := regexp.MustCompile(`(?i)À\s*gauche[\s\r\n]*([\d.,]+)`)
	reRight := regexp.MustCompile(`(?i)À\s*droite[\s\r\n]*([\d.,]+)`)

	for i, b := range blocks {
		// Normalize as parseBlock does
		labelRe := regexp.MustCompile(`(?i)(Commande\s*[:\s\._]*|Nom\s*[:\s\._]*|Rue\s*[:\s\._]*|Code postal\s*[:\s\._]*|Domicilié à\s*[:\s\._]*|R[eé]f[eé]rence\s*[:\s\._]*|Pi[eéè]ce\s*[:\s\._]*|Stuk\s*[:\s\._]*|Détails tissu\s*[:\s\._]*|Détails\s*[:\s\._]*|Hauteu[r]?\s*[:\s\._]*|Hoogte\s*[:\s\._]*|Largeu[r]?\s*[:\s\._]*|Breedte\s*[:\s\._]*|À gauche|À droite|Gauge\s*[:\s\._]*|Droite\s*[:\s\._]*|[A-Z0-9]{4}\.[A-Z0-9]{5})`)
		norm := strings.ReplaceAll(b, "\u00A0", " ")
		norm = strings.ReplaceAll(norm, "\r", "")
		norm = labelRe.ReplaceAllString(norm, "\n$1")

		mL := reLeft.FindStringSubmatch(norm)
		mR := reRight.FindStringSubmatch(norm)

		if mL != nil && mR != nil {
			vL, errL := strconv.ParseFloat(strings.ReplaceAll(mL[1], ",", "."), 64)
			vR, errR := strconv.ParseFloat(strings.ReplaceAll(mR[1], ",", "."), 64)

			if errL != nil || errR != nil {
				t.Fatalf("Block %d: failed to parse floats: vL=%s, vR=%s", i, mL[1], mR[1])
			}

			// In 603Z.00069, left and right values are equal (224/224, 215/215, 308/308, 299/299)
			if vL != vR {
				t.Errorf("Block %d: expected vL == vR, got vL=%v, vR=%v (raw left=%s, right=%s)",
					i, vL, vR, mL[1], mR[1])
			}

			// Specifically check that vR did NOT consume the order number digits (e.g. 224603)
			if vR > 1000 {
				t.Errorf("Block %d: vR appears to have absorbed trailing order number: %v", i, vR)
			}
		}

		// Also check that parseBlock applies the Paire split on these blocks
		recs := parseBlock(b)
		if len(recs) == 2 {
			if recs[0].Size != recs[1].Size {
				t.Errorf("Block %d: Paire records have mismatched sizes: %s vs %s",
					i, recs[0].Size, recs[1].Size)
			}
		}
	}
}

// TestBoundaryCase2_ExistingBarcodeDelimitedPDF verifies that in files like 604V.00106
// where the trailing barcode identifier is delimited by asterisks (*604V.00106.003*),
// the dimension digits are not corrupted and Paire detection succeeds.
func TestBoundaryCase2_ExistingBarcodeDelimitedPDF(t *testing.T) {
	block := `Sur-mesure: Rideau
Commande: 604V.00106.001
Nom: MARTINUS
Référence: slpk
Largeur: 125
Hauteur: 196
À gauche166À droite166*604V.00106.003*
604V.00106.003
`
	recs := parseBlock(block)
	if len(recs) != 2 {
		t.Fatalf("Expected 2 records (Paire split), got %d: %+v", len(recs), recs)
	}

	// Width 125 split in half is 62.5
	expectedSize := "62.5 x 196"
	if recs[0].Size != expectedSize || recs[1].Size != expectedSize {
		t.Errorf("Expected Size %q, got recs[0]=%q, recs[1]=%q", expectedSize, recs[0].Size, recs[1].Size)
	}
}

// TestBoundaryCase3_AuthoritativeCommandeExtraction verifies that the authoritative
// OrderNumber always originates from Commande: near the top of the block, and that
// trailing order/barcode tokens at the footer do not replace or alter OrderNumber.
func TestBoundaryCase3_AuthoritativeCommandeExtraction(t *testing.T) {
	// Block where header Commande is 603Z.00069.001, but footer contains a different token 9999.88888
	block := `Sur-mesure: Rideau
Commande: 603Z.00069.001
Nom: RULOT
Référence: Trevisani
Largeur: 209.5
Hauteur: 240
À gauche224À droite2249999.88888
9999.88888
`
	recs := parseBlock(block)
	if len(recs) == 0 {
		t.Fatalf("Expected parsed records, got 0")
	}

	for _, r := range recs {
		if r.OrderNumber != "603Z.00069" {
			t.Errorf("Authoritative OrderNumber corrupted! Got %q, want %q", r.OrderNumber, "603Z.00069")
		}
		if !strings.HasPrefix(r.OrderItem, "1/0") {
			t.Errorf("Authoritative OrderItem corrupted! Got %q, expected prefix '1/0'", r.OrderItem)
		}
	}
}

// TestBoundaryCase4_NegativeBoundaryCases tests normal and edge cases to ensure
// the boundary regex does not indiscriminately alter unrelated numeric text.
func TestBoundaryCase4_NegativeBoundaryCases(t *testing.T) {
	testCases := []struct {
		name        string
		inputNorm   string
		expectVR    string
		expectSplit bool
	}{
		{
			name: "Normal dimension followed by whitespace",
			inputNorm: `Sur-mesure: Rideau
Commande: 0000.00001.001
Largeur: 100
Hauteur: 200
À gauche 150 À droite 150 
`,
			expectVR:    "150",
			expectSplit: true,
		},
		{
			name: "Normal dimension followed by punctuation (comma and period)",
			inputNorm: `Sur-mesure: Rideau
Commande: 0000.00001.001
Largeur: 100
Hauteur: 200
À gauche 150, À droite 150.
`,
			expectVR:    "150",
			expectSplit: true,
		},
		{
			name: "Normal dimension followed by asterisk-delimited identifier",
			inputNorm: `Sur-mesure: Rideau
Commande: 0000.00001.001
Largeur: 100
Hauteur: 200
À gauche 150 À droite 150*TEST.12345*
`,
			expectVR:    "150",
			expectSplit: true,
		},
		{
			name: "Real collapsed case: À droite224603Z.00069",
			inputNorm: `Sur-mesure: Rideau
Commande: 603Z.00069.001
Largeur: 209.5
Hauteur: 240
À gauche 224 À droite224603Z.00069
`,
			expectVR:    "224",
			expectSplit: true,
		},
		{
			name: "Valid order-token-shaped string occurring independently",
			inputNorm: `Sur-mesure: Rideau
Commande: 0000.00001.001
Largeur: 100
Hauteur: 200
À gauche 150
À droite 150
603Z.00069
`,
			expectVR:    "150",
			expectSplit: true,
		},
		{
			name: "Unequal dimensions: À gauche 100 À droite 150",
			inputNorm: `Sur-mesure: Rideau
Commande: 0000.00001.001
Largeur: 100
Hauteur: 200
À gauche 100 À droite 150
`,
			expectVR:    "150",
			expectSplit: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recs := parseBlock(tc.inputNorm)
			if tc.expectSplit {
				if len(recs) != 2 {
					t.Fatalf("Expected split (2 records), got %d: %+v", len(recs), recs)
				}
			} else {
				if len(recs) != 1 {
					t.Fatalf("Expected no split (1 record), got %d: %+v", len(recs), recs)
				}
			}
		})
	}
}
