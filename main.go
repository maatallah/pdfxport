package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	pdf "github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

// -----------------------------
type Record struct {
	OrderNumber string
	OrderItem   string
	ClientCode  string
	ClientName  string
	Reference   string
	Piece       string
	Size        string
}

// -----------------------------
type FileStat struct {
	FileName  string
	Processed int
	Extracted int
	Skipped   int
	Moved     string
}

// -----------------------------
var DEBUG bool
var debugFile *os.File

func logDebug(format string, a ...interface{}) {
	if DEBUG && debugFile != nil {
		fmt.Fprintf(debugFile, format+"\n", a...)
	}
}

// -----------------------------
// MAIN
// -----------------------------
func main() {

	input := flag.String("input", "", "")
	dir := flag.String("dir", "", "")
	outdir := flag.String("outdir", "", "")

	jsonFlag := flag.Bool("json", false, "")
	csvFlag := flag.Bool("csv", false, "")
	excelFlag := flag.Bool("excel", false, "")
	debug := flag.Bool("debug", false, "")

	flag.Parse()
	DEBUG = *debug
	if DEBUG {
		pdf.DebugOn = true
	}

	if !*jsonFlag && !*csvFlag && !*excelFlag {
		*excelFlag = true
	}

	files := collectFiles(*input, *dir, flag.Args())
	if len(files) == 0 {
		fmt.Println("❌ Aucun fichier PDF trouvé")
		return
	}

	var allRecs []Record
	var allStats []FileStat
	var anyFile string

	for _, f := range files {
		if anyFile == "" {
			anyFile = f
		}
		recs, stat := processPDF(f)
		allRecs = append(allRecs, recs...)
		allStats = append(allStats, stat)
	}

	// Ensure output directory exists
	outPath := getOutputPath(*outdir, anyFile)
	if outPath == "" {
		outPath = "."
	}
	os.MkdirAll(outPath, 0755)

	if *excelFlag {
		finalPath := filepath.Join(outPath, "output.xlsx")
		exportExcel(allRecs, finalPath)
	}
	if *csvFlag {
		exportCSV(allRecs, filepath.Join(outPath, "output.csv"))
	}
	if *jsonFlag {
		exportJSON(allRecs, filepath.Join(outPath, "output.json"))
	}

	writeLogFile(filepath.Join(outPath, "extraction_log.tsv"), allStats)

	fmt.Println("✅ Terminé")
}

// -----------------------------
func collectFiles(input, dir string, args []string) []string {
	var files []string
	files = append(files, args...)

	if input != "" {
		if strings.Contains(input, "*") {
			m, _ := filepath.Glob(input)
			files = append(files, m...)
		} else {
			files = append(files, input)
		}
	}

	if dir != "" {
		filepath.Walk(dir, func(p string, i os.FileInfo, e error) error {
			if e != nil {
				return nil
			}
			if i.IsDir() {
				name := strings.ToLower(i.Name())
				if name == "en_instance" || name == "out" {
					return filepath.SkipDir // Never scan these folders
				}
				return nil
			}
			if strings.HasSuffix(strings.ToLower(p), ".pdf") {
				files = append(files, p)
			}
			return nil
		})
	}

	return files
}

func getOutputPath(outdir, input string) string {
	if outdir != "" {
		return outdir
	}
	return filepath.Dir(input)
}

// -----------------------------
// PROCESS PDF + DEBUG FILE
// -----------------------------
func processPDF(path string) ([]Record, FileStat) {

	base := filepath.Base(path)
	stat := FileStat{FileName: base, Moved: "N"}

	if DEBUG {
		logPath := strings.TrimSuffix(path, ".pdf") + "_debug.txt"
		f, _ := os.Create(logPath)
		debugFile = f
		defer f.Close()

		logDebug("FILE: %s", path)
		logDebug("==========================================")
	}

	text, err := extractText(path)
	if err != nil {
		return nil, stat // File couldn't be opened (likely locked). Skip moving.
	}

	if DEBUG {
		logDebug("\n========= RAW TEXT =========\n%s", text)
	}

	// A valid text-based order will always contain the word "Commande".
	// Scanned images (even with an embedded text barcode) will not.
	hasCommande := regexp.MustCompile(`(?i)Commande`).MatchString(text)

	if !hasCommande {
		fmt.Printf(" ➡ Déplacement de %s vers en_instance (Image/Vide - 'Commande' manquant)\n", base)
		moveToOCR(path)
		stat.Moved = "Y"
		return nil, stat
	}

	blocks := splitBlocks(text)
	stat.Processed = len(blocks)

	if len(blocks) == 0 {
		logDebug("⚠️ AVERTISSEMENT : Aucun bloc généré. Longueur du texte brut : %d", len(text))
	}

	var out []Record
	var lastCmd, lastCode, lastNom string

	for i, b := range blocks {

		logDebug("\n----------------------------------")
		logDebug("BLOC #%d", i+1)
		logDebug("CONTENU :\n%s", b)

		if strings.Contains(strings.ToLower(b), "sur-mesure: doublure") {
			logDebug("⛔ IGNORÉ (Doublure)")
			stat.Skipped++
			continue
		}

		// Carry over page-level headers to subsequent blocks
		if m := regexp.MustCompile(`(?i)(Commande\s*:\s*\S+)`).FindString(b); m != "" {
			lastCmd = m
		}
		if m := regexp.MustCompile(`KLT-[\s\r\n]*\d+`).FindString(b); m != "" {
			lastCode = m
		}
		if m := regexp.MustCompile(`(?si)(Nom\s*:\s*.*?\s*\(KLT-.*?\))`).FindString(b); m != "" {
			lastNom = m
		}

		missingHeaders := ""
		if !regexp.MustCompile(`(?i)Commande\s*:`).MatchString(b) && lastCmd != "" {
			missingHeaders += lastCmd + "\n"
		}
		if !regexp.MustCompile(`KLT-`).MatchString(b) && lastCode != "" {
			missingHeaders += lastCode + "\n"
		}
		if !regexp.MustCompile(`(?i)Nom\s*:`).MatchString(b) && lastNom != "" {
			missingHeaders += lastNom + "\n"
		}
		if missingHeaders != "" {
			b = missingHeaders + b
		}

		recs := parseBlock(b)

		// Only keep records that successfully extracted an Order Number
		for _, r := range recs {
			if r.OrderNumber != "" {
				out = append(out, r)
			}
		}
	}

	// Post-pass to find the maximum item X per OrderNumber (which becomes Z)
	maxZPerOrder := make(map[string]int)
	for _, r := range out {
		parts := strings.Split(r.OrderItem, "/")
		if len(parts) >= 1 {
			if x, err := strconv.Atoi(parts[0]); err == nil && x > maxZPerOrder[r.OrderNumber] {
				maxZPerOrder[r.OrderNumber] = x
			}
		}
	}

	for i := range out {
		z := maxZPerOrder[out[i].OrderNumber]
		out[i].OrderItem = fmt.Sprintf("%s/%d", out[i].OrderItem, z)
		logDebug("➡ RÉSULTAT : %+v", out[i])
	}

	stat.Extracted = len(out)
	return out, stat
}

// -----------------------------
func moveToOCR(path string) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ocrDir := filepath.Join(dir, "en_instance")

	// Create the en_instance directory if it doesn't exist
	os.MkdirAll(ocrDir, 0755)

	dest := filepath.Join(ocrDir, base)

	if filepath.Base(dir) == "en_instance" {
		return
	}

	// On Windows, os.Rename completely fails if the destination file already exists!
	os.Remove(dest)

	if err := os.Rename(path, dest); err == nil {
		fmt.Printf("   ✅ %s déplacé vers en_instance\n", base)
		return
	}

	// Fallback: Copy and Delete
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("   ❌ Échec du déplacement (Lecture) : %v\n", err)
		return
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		fmt.Printf("   ❌ Échec du déplacement (Écriture) : %v\n", err)
		return
	}

	for i := 0; i < 5; i++ {
		if err := os.Remove(path); err == nil {
			fmt.Printf("   ✅ %s copié et supprimé vers en_instance\n", base)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Printf("   ⚠️ %s copié vers en_instance, mais l'original est verrouillé et ne peut pas être supprimé.\n", base)
}

// -----------------------------
func extractText(path string) (string, error) {
	// Read entire file into RAM first. This guarantees the file handle on disk is closed instantly.
	fileData, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		txt, err := p.GetPlainText(nil)
		if err != nil {
			logDebug("⚠️ ERREUR lors de l'analyse de la page %d : %v", i, err)
		}
		sb.WriteString(txt + "\n")
	}
	return sb.String(), nil
}

// -----------------------------
func splitBlocks(text string) []string {
	// Normalize text to make regex matching more robust before splitting
	text = strings.ReplaceAll(text, "\u00A0", " ")
	text = strings.ReplaceAll(text, "\r", "")

	re := regexp.MustCompile(`(?i)Sur-mesure\s*:\s*(Tenture|Doublure|Commande\s*Tissu)`)

	var blocks []string
	matches := re.FindAllStringSubmatchIndex(text, -1)

	if len(matches) > 0 {
		for i, m := range matches {
			kind := strings.ToLower(strings.TrimSpace(text[m[2]:m[3]]))

			start := 0
			if i > 0 {
				start = matches[i][0]
			}
			nextStart := len(text)
			if i+1 < len(matches) {
				nextStart = matches[i+1][0]
			}
			body := strings.TrimSpace(text[start:nextStart])
			if kind != "doublure" {
				blocks = append(blocks, body)
			}
		}
		return blocks
	}

	// Fallback 1: Split by Commande if no Sur-mesure label exists
	reCmd := regexp.MustCompile(`(?i)Commande\s*:`)
	cmdMatches := reCmd.FindAllStringSubmatchIndex(text, -1)
	if len(cmdMatches) > 0 {
		for i, m := range cmdMatches {
			start := m[0]
			nextStart := len(text)
			if i+1 < len(cmdMatches) {
				nextStart = cmdMatches[i+1][0]
			}
			body := strings.TrimSpace(text[start:nextStart])
			if !strings.Contains(strings.ToLower(body), "doublure") {
				blocks = append(blocks, body)
			}
		}
		return blocks
	}

	// Fallback 2: Entire text
	if strings.TrimSpace(text) != "" && !strings.Contains(strings.ToLower(text), "doublure") {
		blocks = append(blocks, text)
	}

	return blocks
}

// -----------------------------
func extractOrderFields(fullCmd string) (string, int, int) {
	parts := strings.Split(fullCmd, ".")

	orderNumber := parts[0]
	if len(parts) > 1 {
		orderNumber += "." + parts[1]
	}

	x, y := 0, 0
	if len(parts) >= 3 {
		x, _ = strconv.Atoi(parts[2])
	}
	if len(parts) >= 4 {
		y, _ = strconv.Atoi(parts[3])
	}

	return orderNumber, x, y
}

// -----------------------------
func extractPiece(block string) string {
	logDebug("ENTRÉE EXTRACTPIECE : %.200s", block)

	// Try to find the 'Pièce:' label anywhere and extract until the next known label.
	rePiece := regexp.MustCompile(`(?i)(?:Pi[eéè]ce\s*:|Piece\s*:)`)
	if loc := rePiece.FindStringIndex(block); loc != nil {
		start := loc[1]
		rest := block[start:]
		reNext := regexp.MustCompile(`(?i)(Détails|Details|Nom\s*:|Rue\s*:|Référence\s*:|Commande\s*:|Pi[eéè]ce\s*:|Piece\s*:)`)
		if nloc := reNext.FindStringIndex(rest); nloc != nil {
			val := strings.TrimSpace(rest[:nloc[0]])
			val = strings.Join(strings.Fields(val), " ")
			if val != "" {
				return val
			}
		} else {
			val := strings.TrimSpace(rest)
			val = strings.Join(strings.Fields(val), " ")
			if val != "" {
				return val
			}
		}
	}

	// Fallback: original line-based logic
	lines := strings.Split(block, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		lline := strings.ToLower(line)

		if strings.HasPrefix(lline, "pièce") || strings.HasPrefix(lline, "piece") {

			idx := strings.Index(line, ":")
			if idx == -1 {
				continue
			}
			val := strings.TrimSpace(line[idx+1:])

			if val == "" {
				return ""
			}

			words := strings.Fields(val)

			if len(words) > 0 && strings.Contains(words[0], ":") {
				return ""
			}

			var result []string

			for _, w := range words {
				if strings.Contains(w, ":") {
					break
				}
				result = append(result, w)
			}

			return strings.Join(result, " ")
		}
	}

	return ""
}

// -----------------------------
func parseBlock(block string) []Record {
	// normalize block to make regex matching more robust (NBSPs, CRs, etc.)
	norm := strings.ReplaceAll(block, "\u00A0", " ")
	norm = strings.ReplaceAll(norm, "\r", "")
	// Ensure common field labels appear on their own line when PDFs collapse
	// spacing (e.g. "Référence:L0102...Pièce:Fenêtre G").
	labelRe := regexp.MustCompile(`(?i)(Commande\s*:|Nom\s*:|Rue\s*:|Code postal\s*:|Domicilié à\s*:|Référence\s*:|Pi[eéè]ce\s*:|Détails tissu\s*:|Détails\s*:|Hauteur\s*:|Largeur\s*:|À gauche|À droite|Gauge\s*:|Droite\s*:)`)
	norm = labelRe.ReplaceAllString(norm, "\n$1")

	rec := Record{}

	var itemX, itemY int
	reCmd := regexp.MustCompile(`(?i)Commande\s*:\s*(\S+)`)
	if m := reCmd.FindStringSubmatch(norm); m != nil {
		rec.OrderNumber, itemX, itemY = extractOrderFields(m[1])
	}

	reCode := regexp.MustCompile(`(?i)KLT-[\s\r\n]*(\d+)`)
	if cm := reCode.FindStringSubmatch(norm); len(cm) > 1 {
		rec.ClientCode = cm[1]
	}

	reNom := regexp.MustCompile(`(?si)Nom\s*:\s*(.*?)\s*\(KLT-.*?\)`)
	if m := reNom.FindStringSubmatch(norm); m != nil {
		rec.ClientName = strings.Join(strings.Fields(m[1]), " ")
	}

	// The previous multi-line regex for Reference was too greedy and would
	// capture text between the reference and the next known label.
	// This simpler, single-line regex is more robust and avoids capturing
	// unrelated text from subsequent lines.
	reRef := regexp.MustCompile(`(?i)Référence\s*:\s*([^\n\r]*)`)
	if m := reRef.FindStringSubmatch(norm); m != nil {
		rec.Reference = strings.TrimSpace(m[1])
	}

	rec.Piece = extractPiece(norm)

	// -----------------------------
	// SPLIT DEBUG
	// -----------------------------
	logDebug("\n--- DÉBOGAGE DE SÉPARATION ---")
	logDebug("Aperçu : %.200s", block)

	// Require a colon or whitespace after 'Hauteur' to avoid matching other
	// occurrences like 'grand hauteur... 10 cm' earlier in the block.
	reH := regexp.MustCompile(`(?i)Hauteur\s*[:\s]*([\d.,]+)`)
	reLeftZero := regexp.MustCompile(`(?is)(?:À|A|à|a)\s*gauche[^0-9]*0`)
	reRightZero := regexp.MustCompile(`(?is)(?:À|A|à|a)\s*droite[^0-9]*0`)

	reGauge := regexp.MustCompile(`(?i)Gauge\s*[:\s]*([\d.,]+)`)
	reDroite := regexp.MustCompile(`(?i)Droite\s*[:\s]*([\d.,]+)`)

	hStr := ""
	if m := reH.FindStringSubmatch(norm); m != nil {
		hStr = strings.ReplaceAll(m[1], ",", ".")
	}

	isLeftZero := reLeftZero.MatchString(norm)
	isRightZero := reRightZero.MatchString(norm)

	gStr := ""
	dStr := ""

	if m := reGauge.FindStringSubmatch(norm); m != nil {
		gStr = strings.ReplaceAll(m[1], ",", ".")
	}
	if m := reDroite.FindStringSubmatch(norm); m != nil {
		dStr = strings.ReplaceAll(m[1], ",", ".")
	}

	logDebug("Hauteur=%s", hStr)
	logDebug("LeftZero=%v RightZero=%v", isLeftZero, isRightZero)
	logDebug("Gauge=%s Droite=%s", gStr, dStr)

	// -----------------------------
	if isLeftZero && isRightZero && hStr != "" {

		var results []Record

		if gStr != "" {
			r1 := rec
			r1.OrderItem = fmt.Sprintf("%d/1", itemX)
			r1.Size = gStr + " x " + hStr
			results = append(results, r1)
		}

		if dStr != "" {
			r2 := rec
			r2.OrderItem = fmt.Sprintf("%d/2", itemX)
			r2.Size = dStr + " x " + hStr
			results = append(results, r2)
		}

		logDebug("NOMBRE DE RÉSULTATS DE SÉPARATION : %d", len(results))

		return results
	}

	rec.OrderItem = fmt.Sprintf("%d/%d", itemX, itemY)
	rec.Size = extractSize(norm)
	return []Record{rec}
}

// -----------------------------
func extractSize(block string) string {

	reH := regexp.MustCompile(`(?i)Hauteur\s*:\s*([\d.,]+)`)
	reW := regexp.MustCompile(`(?i)Largeur\s*:\s*([\d.,]+)`)

	h := ""
	w := ""

	if m := reH.FindStringSubmatch(block); m != nil {
		h = strings.ReplaceAll(m[1], ",", ".")
	}
	if m := reW.FindStringSubmatch(block); m != nil {
		w = strings.ReplaceAll(m[1], ",", ".")
	}

	if w != "" && h != "" {
		return w + " x " + h
	}

	// Fallback for cases like 'Commande Tissu' where only 'Hauteur de coupe' exists
	reHC := regexp.MustCompile(`(?i)Hauteur de coupe\s*[:]?\s*([\d.,]+)`)
	if m := reHC.FindStringSubmatch(block); m != nil {
		return strings.ReplaceAll(m[1], ",", ".")
	}

	return ""
}

// -----------------------------
func exportJSON(r []Record, f string) {
	file, _ := os.Create(f)
	defer file.Close()
	json.NewEncoder(file).Encode(r)
}

func exportCSV(r []Record, f string) {
	file, _ := os.Create(f)
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	for _, x := range r {
		w.Write([]string{x.OrderNumber, x.OrderItem, x.ClientCode, x.ClientName, x.Reference, x.Piece, x.Size})
	}
}

func exportExcel(r []Record, f string) {
	ex := excelize.NewFile()
	s := ex.GetSheetName(0)

	headers := []string{"OrderNumber", "OrderItem", "ClientCode", "ClientName", "Reference", "Piece", "Size"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		ex.SetCellValue(s, cell, h)
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	for i, rec := range r {
		values := []string{rec.OrderNumber, rec.OrderItem, rec.ClientCode, rec.ClientName, rec.Reference, rec.Piece, rec.Size}
		for j, v := range values {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			ex.SetCellStr(s, cell, v)
			if len(v) > colWidths[j] {
				colWidths[j] = len(v)
			}
		}
	}

	for i, w := range colWidths {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		// Add a little padding to the calculated max width
		ex.SetColWidth(s, colName, colName, float64(w)+2.0)
	}

	if err := ex.SaveAs(f); err != nil {
		fmt.Printf("❌ ERREUR lors de la sauvegarde du fichier Excel (est-il ouvert ?) : %v\n", err)
	} else {
		fmt.Printf("✅ %d lignes écrites avec succès dans Excel !\n", len(r))
	}
}

// -----------------------------
func writeLogFile(logPath string, stats []FileStat) {
	fileExists := false
	if _, err := os.Stat(logPath); err == nil {
		fileExists = true
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ ERREUR lors de l'ouverture du fichier journal: %v\n", err)
		return
	}
	defer f.Close()

	if !fileExists {
		f.WriteString("Timestamp\tFile name\tProcessed items\tExtracted items\tSkipped items\tMoved Y/N\n")
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	for _, s := range stats {
		f.WriteString(fmt.Sprintf("%s\t%s\t%d\t%d\t%d\t%s\n", timestamp, s.FileName, s.Processed, s.Extracted, s.Skipped, s.Moved))
	}
}
