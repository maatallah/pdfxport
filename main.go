package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

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
		fmt.Println("❌ No PDF files found")
		return
	}

	jobs := make(chan string, len(files))
	results := make(chan struct {
		file string
		recs []Record
	}, len(files))

	var wg sync.WaitGroup

	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				recs := processPDF(f)
				results <- struct {
					file string
					recs []Record
				}{f, recs}
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	wg.Wait()
	close(results)

	var allRecs []Record
	var anyFile string
	for res := range results {
		if anyFile == "" {
			anyFile = res.file
		}
		allRecs = append(allRecs, res.recs...)
	}

	// Ensure output directory exists
	outPath := getOutputPath(*outdir, anyFile)
	if outPath == "" {
		outPath = "."
	}
	os.MkdirAll(outPath, 0755)

	if *excelFlag {
		exportExcel(allRecs, filepath.Join(outPath, "output.xlsx"))
	}
	if *csvFlag {
		exportCSV(allRecs, filepath.Join(outPath, "output.csv"))
	}
	if *jsonFlag {
		exportJSON(allRecs, filepath.Join(outPath, "output.json"))
	}

	fmt.Println("✅ Done")
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
func processPDF(path string) []Record {

	if DEBUG {
		logPath := strings.TrimSuffix(path, ".pdf") + "_debug.txt"
		f, _ := os.Create(logPath)
		debugFile = f
		defer f.Close()

		logDebug("FILE: %s", path)
		logDebug("==========================================")
	}

	text := extractText(path)

	if DEBUG {
		logDebug("\n========= RAW TEXT =========\n%s", text)
	}

	blocks := splitBlocks(text)
	if len(blocks) == 0 {
		logDebug("⚠️ WARNING: No blocks generated. Raw text length: %d", len(text))
	}

	var out []Record
	var lastCmd, lastCode, lastNom string

	for i, b := range blocks {

		logDebug("\n----------------------------------")
		logDebug("BLOCK #%d", i+1)
		logDebug("CONTENT:\n%s", b)

		if strings.Contains(strings.ToLower(b), "sur-mesure: doublure") {
			logDebug("⛔ SKIPPED (Doublure)")
			continue
		}

		// Carry over page-level headers to subsequent blocks
		if m := regexp.MustCompile(`(?i)(Commande:\s*\S+)`).FindString(b); m != "" {
			lastCmd = m
		}
		if m := regexp.MustCompile(`KLT-[\s\r\n]*\d+`).FindString(b); m != "" {
			lastCode = m
		}
		if m := regexp.MustCompile(`(?si)(Nom:\s*.*?\s*\(KLT-.*?\))`).FindString(b); m != "" {
			lastNom = m
		}

		missingHeaders := ""
		if !regexp.MustCompile(`(?i)Commande:`).MatchString(b) && lastCmd != "" {
			missingHeaders += lastCmd + "\n"
		}
		if !regexp.MustCompile(`KLT-`).MatchString(b) && lastCode != "" {
			missingHeaders += lastCode + "\n"
		}
		if !regexp.MustCompile(`(?i)Nom:`).MatchString(b) && lastNom != "" {
			missingHeaders += lastNom + "\n"
		}
		if missingHeaders != "" {
			b = missingHeaders + b
		}

		recs := parseBlock(b)

		out = append(out, recs...)
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
		logDebug("➡ RESULT: %+v", out[i])
	}

	return out
}

// -----------------------------
func extractText(path string) string {
	f, r, err := pdf.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		txt, err := p.GetPlainText(nil)
		if err != nil {
			logDebug("⚠️ ERROR parsing page %d: %v", i, err)
		}
		sb.WriteString(txt + "\n")
	}
	return sb.String()
}

// -----------------------------
func splitBlocks(text string) []string {
	// Normalize text to make regex matching more robust before splitting
	text = strings.ReplaceAll(text, "\u00A0", " ")
	text = strings.ReplaceAll(text, "\r", "")

	re := regexp.MustCompile(`(?i)Sur-mesure:\s*(Tenture|Doublure)`)

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
	reCmd := regexp.MustCompile(`(?i)Commande:`)
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
	logDebug("EXTRACTPIECE INPUT: %.200s", block)

	// Try to find the 'Pièce:' label anywhere and extract until the next known label.
	rePiece := regexp.MustCompile(`(?i)(?:Pi[eéè]ce:|Piece:)`)
	if loc := rePiece.FindStringIndex(block); loc != nil {
		start := loc[1]
		rest := block[start:]
		reNext := regexp.MustCompile(`(?i)(Détails|Details|Nom:|Rue:|Référence:|Commande:|Pi[eéè]ce:|Piece:)`)
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

		if strings.HasPrefix(line, "Pièce:") {

			val := strings.TrimSpace(strings.TrimPrefix(line, "Pièce:"))

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
	labelRe := regexp.MustCompile(`(?i)(Commande:|Nom:|Rue:|Code postal:|Domicilié à:|Référence:|Pi[eéè]ce:|Détails tissu:|Détails:|Hauteur:|Largeur:|À gauche|À droite|Gauge:|Droite:)`)
	norm = labelRe.ReplaceAllString(norm, "\n$1")

	rec := Record{}

	var itemX, itemY int
	reCmd := regexp.MustCompile(`Commande:\s*(\S+)`)
	if m := reCmd.FindStringSubmatch(norm); m != nil {
		rec.OrderNumber, itemX, itemY = extractOrderFields(m[1])
	}

	reCode := regexp.MustCompile(`KLT-[\s\r\n]*(\d+)`)
	if cm := reCode.FindStringSubmatch(norm); len(cm) > 1 {
		rec.ClientCode = cm[1]
	}

	reNom := regexp.MustCompile(`(?si)Nom:\s*(.*?)\s*\(KLT-.*?\)`)
	if m := reNom.FindStringSubmatch(norm); m != nil {
		rec.ClientName = strings.Join(strings.Fields(m[1]), " ")
	}

	// The previous multi-line regex for Reference was too greedy and would
	// capture text between the reference and the next known label.
	// This simpler, single-line regex is more robust and avoids capturing
	// unrelated text from subsequent lines.
	reRef := regexp.MustCompile(`(?i)Référence:\s*([^\n\r]*)`)
	if m := reRef.FindStringSubmatch(norm); m != nil {
		rec.Reference = strings.TrimSpace(m[1])
	}

	rec.Piece = extractPiece(norm)

	// -----------------------------
	// SPLIT DEBUG
	// -----------------------------
	logDebug("\n--- SPLIT DEBUG ---")
	logDebug("Preview: %.200s", block)

	// Require a colon or whitespace after 'Hauteur' to avoid matching other
	// occurrences like 'grand hauteur... 10 cm' earlier in the block.
	reH := regexp.MustCompile(`(?i)Hauteur[:\s]*([\d.,]+)`)
	reLeftZero := regexp.MustCompile(`(?is)(?:À|A|à|a)\s*gauche[^0-9]*0`)
	reRightZero := regexp.MustCompile(`(?is)(?:À|A|à|a)\s*droite[^0-9]*0`)

	reGauge := regexp.MustCompile(`(?i)Gauge[:\s]*([\d.,]+)`)
	reDroite := regexp.MustCompile(`(?i)Droite[:\s]*([\d.,]+)`)

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

		logDebug("SPLIT RESULT COUNT: %d", len(results))

		return results
	}

	rec.OrderItem = fmt.Sprintf("%d/%d", itemX, itemY)
	rec.Size = extractSize(norm)
	return []Record{rec}
}

// -----------------------------
func extractSize(block string) string {

	reH := regexp.MustCompile(`Hauteur:\s*([\d.,]+)`)
	reW := regexp.MustCompile(`Largeur:\s*([\d.,]+)`)

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

	for i, rec := range r {
		values := []string{rec.OrderNumber, rec.OrderItem, rec.ClientCode, rec.ClientName, rec.Reference, rec.Piece, rec.Size}
		for j, v := range values {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			ex.SetCellStr(s, cell, v)
		}
	}

	ex.SaveAs(f)
}
