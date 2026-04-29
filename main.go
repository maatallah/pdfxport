package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
	OCR       bool
}

// -----------------------------
var DEBUG = false
var debugFile *os.File

func logDebug(format string, a ...interface{}) {
	if DEBUG && debugFile != nil {
		fmt.Fprintf(debugFile, format+"\n", a...)
	}
}

// startServer launches a background HTTP server to receive PDFs directly from the Chrome Extension
func startServer() {
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		// Allow the Chrome Extension to talk to us
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusInternalServerError)
			return
		}

		// Save the file to the 'in/' folder
		inDir := filepath.Join(filepath.Dir(os.Args[0]), "in")
		os.MkdirAll(inDir, 0755) // Ensure directory exists

		filename := fmt.Sprintf("Commandes_%d.pdf", time.Now().Unix())
		outPath := filepath.Join(inDir, filename)

		err = os.WriteFile(outPath, body, 0644)
		if err != nil {
			fmt.Printf("\n[ERREUR SERVEUR] Impossible d'enregistrer %s: %v\n", filename, err)
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}

		fmt.Printf("\n[SERVEUR] Fichier reçu et enregistré avec succès : in\\%s\n", filename)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	fmt.Println("[SERVEUR] En écoute sur localhost:8765 (Sécurisé - Prêt pour l'extension Chrome)...")
	// Listen ONLY on localhost (127.0.0.1) so no external machines can hit this port
	err := http.ListenAndServe("127.0.0.1:8765", nil)
	if err != nil {
		fmt.Printf("[ERREUR SERVEUR] %v\n", err)
	}
}

// -----------------------------
// MAIN
// -----------------------------
func main() {

	fmt.Println(">> Demarrage du programme Go...")

	runServer := flag.Bool("server", false, "")

	input := flag.String("input", "", "")
	dir := flag.String("dir", "", "")
	outdir := flag.String("outdir", "", "")

	jsonFlag := flag.Bool("json", false, "")
	csvFlag := flag.Bool("csv", false, "")
	excelFlag := flag.Bool("excel", false, "")
	debug := flag.Bool("debug", false, "")

	flag.Parse()

	if *runServer {
		fmt.Println(">> Démarrage en MODE SERVEUR. Gardez cette fenêtre ouverte en arrière-plan.")
		startServer() // Blocks forever
		return
	}
	DEBUG = *debug
	if DEBUG {
		pdf.DebugOn = true
	}

	if !*jsonFlag && !*csvFlag && !*excelFlag {
		*excelFlag = true
	}

	// Resolve mapped drive letters to UNC paths (fixes Windows 7 UAC elevation)
	if *dir != "" {
		*dir = resolveUNC(*dir)
	}
	if *outdir != "" {
		*outdir = resolveUNC(*outdir)
	}

	for {
		fmt.Printf("\n>> Dossier d'entrée configuré : %s\n", *dir)
		fmt.Println(">> Appuyez sur Entree pour traiter les fichiers (ou tapez 'q' pour quitter)...")
		
		userInput, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(strings.ToLower(userInput)) == "q" {
			break
		}

		fmt.Printf(">> Recherche de fichiers PDF dans : %s\n", *dir)

		files := collectFiles(*input, *dir, flag.Args())
		if len(files) == 0 {
			fmt.Println("[ERREUR] Aucun fichier PDF trouve")
			continue
		}

		var allRecs []Record
		var allStats []FileStat
		var anyFile string

		fmt.Printf(">> %d fichiers trouves. Debut de l'extraction...\n", len(files))

		for _, f := range files {
			fmt.Printf(".. Traitement de : %s...\n", filepath.Base(f))
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

		// Summary stats
		totalFiles := len(allStats)
		totalItems := 0
		totalOCR := 0
		totalFailed := 0
		for _, s := range allStats {
			totalItems += s.Extracted
			if s.OCR {
				totalOCR++
			}
			if s.Extracted == 0 {
				totalFailed++
			}
		}

		fmt.Printf("\n--- STATISTIQUES FINALES ---\n")
		fmt.Printf("Fichiers traites       : %d\n", totalFiles)
		fmt.Printf("Items extraits         : %d\n", totalItems)
		fmt.Printf("Conversions OCR (auto) : %d\n", totalOCR)
		fmt.Printf("Echecs (En instance)   : %d\n", totalFailed)
		fmt.Printf("----------------------------\n")
		fmt.Println("[OK] Traitement terminé pour ce lot.")
	}
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
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Printf("[ERREUR] Impossible de lire le dossier %s: %v\n", dir, err)
			return files
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue // Skip all subfolders (limit to 1 level only)
			}
			p := filepath.Join(dir, entry.Name())
			ext := strings.ToLower(filepath.Ext(p))
			if ext == ".pdf" || ext == ".txt" {
				files = append(files, p)
			}
		}
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
	dir := filepath.Dir(path)
	stat := FileStat{FileName: base, Moved: "N"}

	// Skip files that are already archived or in the error folder
	if filepath.Base(dir) == "en_instance" || filepath.Base(dir) == "processed" {
		return nil, stat
	}

	if DEBUG {
		logPath := strings.TrimSuffix(path, ".pdf") + "_debug.txt"
		f, _ := os.Create(logPath)
		debugFile = f
		defer f.Close()

		logDebug("FILE: %s", path)
		logDebug("==========================================")
	}

	ext := strings.ToLower(filepath.Ext(path))
	var text string
	var err error

	if ext == ".txt" {
		content, err2 := os.ReadFile(path)
		if err2 != nil {
			return nil, stat
		}
		text = string(content)
	} else {
		text, err = extractText(path)
		if err != nil {
			return nil, stat
		}
	}

	if DEBUG {
		logDebug("\n========= RAW TEXT =========\n%s", text)
	}

	// 1. Initial pass: try to extract text normally
	blocks := splitBlocks(text)
	recs := parseAndFilter(blocks, &stat)

	// --- NEW OCR TRIGGER ---
	// If no items found OR important fields are missing, try OCR as fallback
	needsOCR := len(recs) == 0
	if !needsOCR {
		for _, r := range recs {
			if r.Size == "" {
				needsOCR = true
				break
			}
		}
	}

	if needsOCR {
		fmt.Printf(" --> %s : Manque de donnees (Taille/Items). Tentative d'OCR avec NAPS2...\n", base)
		if err := performOCR(path); err == nil {
			stat.OCR = true
			text, _ = extractText(path)

			// LOG the OCR results for user inspection
			ocrLogPath := strings.TrimSuffix(path, ".pdf") + "_ocr_raw.txt"
			os.WriteFile(ocrLogPath, []byte(text), 0644)

			// Second pass with OCR text
			blocks = splitBlocks(text)
			recs = parseAndFilter(blocks, &stat)
		}
	} else if ext == ".txt" {
		// For .txt files, we go straight to parsing
		blocks = splitBlocks(text)
		recs = parseAndFilter(blocks, &stat)
	}

	// 4. Still no results? Move to en_instance
	if len(recs) == 0 {
		fmt.Printf(" --> Deplacement de %s vers en_instance (Echec extraction)\n", base)
		moveToOCR(path)
		stat.Moved = "Y"
		return nil, stat
	}

	stat.Extracted = len(recs)

	// 5. ARCHIVE SUCCESSFUL FILES
	if stat.Extracted > 0 {
		moveToProcessed(path)
		stat.Moved = "P"
	}

	return recs, stat
}

func performOCR(path string) error {
	// Path to NAPS2.Console.exe relative to current dir
	exePath := filepath.Join("App", "NAPS2.Console.exe")
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("NAPS2.Console.exe non trouve dans le dossier App")
	}

	// We overwrite the same file with the OCRed version
	cmd := exec.Command(exePath,
		"-i", path,
		"-o", path,
		"--ocr",
		"--ocrlang", "fra", // Using French for OCR
		"--dpi", "300", // Back to 300 DPI for standard clarity
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("erreur NAPS2: %v - %s", err, stderr.String())
	}
	return nil
}

func moveToProcessed(path string) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	doneDir := filepath.Join(dir, "processed")
	os.MkdirAll(doneDir, 0755)

	dest := filepath.Join(doneDir, base)
	// On Windows, os.Rename fails if destination exists
	os.Remove(dest)

	if err := os.Rename(path, dest); err != nil {
		// Fallback for network drives
		data, err := os.ReadFile(path)
		if err == nil {
			os.WriteFile(dest, data, 0644)
			os.Remove(path)
		}
	}
	fmt.Printf("   [DONE] %s archive vers 'processed'\n", base)
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
		fmt.Printf("   [OK] %s deplace vers en_instance\n", base)
		return
	}

	// Fallback: Copy and Delete
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("   [ERREUR] Echec du deplacement (Lecture) : %v\n", err)
		return
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		fmt.Printf("   [ERREUR] Echec du deplacement (Ecriture) : %v\n", err)
		return
	}

	for i := 0; i < 5; i++ {
		if err := os.Remove(path); err == nil {
			fmt.Printf("   [OK] %s copie et supprime vers en_instance\n", base)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Printf("   [ATTENTION] %s copie vers en_instance, mais l'original est verrouille et ne peut pas etre supprime.\n", base)
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
			logDebug("[ATTENTION] ERREUR lors de l'analyse de la page %d : %v", i, err)
		}
		sb.WriteString(txt + "\n")
	}
	return sb.String(), nil
}

// -----------------------------
func parseAndFilter(blocks []string, stat *FileStat) []Record {
	var out []Record
	var lastCmd, lastCode, lastNom string
	processedIDs := make(map[string]bool)

	for i, b := range blocks {
		logDebug("\n----------------------------------")
		logDebug("BLOC #%d", i+1)

		if strings.Contains(strings.ToLower(b), "sur-mesure: doublure") {
			logDebug("[SKIP] IGNORE (Doublure)")
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

		// Re-inject missing headers to ensure parseBlock has full context
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
			// Avoid exact duplicates in the same file (footer barcodes)
			uniqueKey := r.OrderNumber + "|" + r.OrderItem + "|" + r.Size
			if r.OrderNumber != "" && !processedIDs[uniqueKey] {
				out = append(out, r)
				processedIDs[uniqueKey] = true
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
	}

	return out
}

// -----------------------------
func splitBlocks(text string) []string {
	// Normalize text to make regex matching more robust before splitting
	text = strings.ReplaceAll(text, "\u00A0", " ")
	text = strings.ReplaceAll(text, "\r", "")

	// Match any "Sur-mesure: <type>" header. We accept ALL types and only
	// exclude "Doublure" blocks below. This avoids silently dropping new
	// order types (e.g. "Store bateau") that aren't in a whitelist.
	re := regexp.MustCompile(`(?i)Sur-mesure\s*:\s*\S+`)

	var blocks []string
	matches := re.FindAllStringSubmatchIndex(text, -1)

	if len(matches) > 0 {
		for i, m := range matches {
			matchedText := strings.ToLower(strings.TrimSpace(text[m[0]:m[1]]))

			start := 0
			if i > 0 {
				start = matches[i][0]
			}
			nextStart := len(text)
			if i+1 < len(matches) {
				nextStart = matches[i+1][0]
			}
			body := strings.TrimSpace(text[start:nextStart])
			if !strings.Contains(matchedText, "doublure") {
				blocks = append(blocks, body)
			}
		}
		return blocks
	}

	// Fallback 1: Split by Commande (Labeled)
	reCmd := regexp.MustCompile(`(?i)Commande\s*:`)
	cmdMatches := reCmd.FindAllStringSubmatchIndex(text, -1)
	if len(cmdMatches) > 1 {
		for i, m := range cmdMatches {
			start := m[0]
			nextStart := len(text)
			if i+1 < len(cmdMatches) {
				nextStart = cmdMatches[i+1][0]
			}
			body := strings.TrimSpace(text[start:nextStart])
			blocks = append(blocks, body)
		}
		return blocks
	}

	// Fallback 2: Split by Raw Order Number (XXXX.XXXXX.XXX) - Common in OCR
	reRawOrder := regexp.MustCompile(`(?m)^[A-Z0-9]{4}\.[A-Z0-9]{5}\.[A-Z0-9]{3}\s*$`)
	rawMatches := reRawOrder.FindAllStringSubmatchIndex(text, -1)
	if len(rawMatches) > 1 {
		seen := make(map[string]bool)
		for i, m := range rawMatches {
			start := m[0]
			id := strings.TrimSpace(text[m[0]:m[1]])

			// 1. Skip if it looks like a barcode (asterisks)
			if start > 0 && text[start-1] == '*' {
				continue
			}
			// 2. Skip if we just saw this ID (likely same record barcode footer)
			if seen[id] {
				continue
			}
			seen[id] = true

			nextStart := len(text)
			if i+1 < len(rawMatches) {
				nextStart = rawMatches[i+1][0]
			}
			body := strings.TrimSpace(text[start:nextStart])

			// 3. Skip tiny blocks
			if len(strings.Split(body, "\n")) < 5 {
				continue
			}

			blocks = append(blocks, body)
		}
		return blocks
	}

	// Fallback 3: Entire text
	if strings.TrimSpace(text) != "" {
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
func isValValidPiece(val string) bool {
	if val == "" {
		return false
	}
	lval := strings.ToLower(val)
	// Blacklist: Not a piece if it contains these "noise" words
	blacklist := []string{"klt-", "d[eéè]tails", "commande", "sur-mesure", "tissu", "nombre", "m[eéè]chanisme", "child-safety", "benodigd", "cm", "mm"}
	for _, b := range blacklist {
		if regexp.MustCompile("(?i)" + b).MatchString(lval) {
			return false
		}
	}
	// Check if it looks like an Order Number or Item ID (e.g. 0000.03980.001)
	if regexp.MustCompile(`\d{4}\.\d{5}`).MatchString(val) {
		return false
	}
	// Check if it looks like an address/street
	if regexp.MustCompile(`^\d+\s+\w+`).MatchString(val) {
		return false
	}
	return true
}

// -----------------------------
func extractPiece(block string) string {
	logDebug("ENTRÉE EXTRACTPIECE : %.200s", block)

	// 1. Label-based search
	rePiece := regexp.MustCompile(`(?i)(?:Pi[eéè]ce\s*:|Piece\s*:|Stuk\s*:)`)
	if loc := rePiece.FindStringIndex(block); loc != nil {
		start := loc[1]
		rest := block[start:]
		reNext := regexp.MustCompile(`(?i)(D[eéè]tails|Nom\s*:|Rue\s*:|R[eé]f[eé]rence\s*:|Commande\s*:|Pi[eéè]ce\s*:|Stuk\s*:|Tissu)`)
		if nloc := reNext.FindStringIndex(rest); nloc != nil {
			val := strings.TrimSpace(rest[:nloc[0]])
			val = strings.Join(strings.Fields(val), " ")
			if isValValidPiece(val) {
				return val
			}
		}
	}

	// 2. OCR Line fallback
	lines := strings.Split(block, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if isValValidPiece(line) {
			// We only take long lines which aren't other labels
			if len(line) > 5 && !strings.Contains(line, ":") {
				return line
			}
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
	// spacing. Added Dutch labels and fuzzy support for OCR typos (Hauteu, etc.)
	labelRe := regexp.MustCompile(`(?i)(Commande\s*[:\s\._]*|Nom\s*[:\s\._]*|Rue\s*[:\s\._]*|Code postal\s*[:\s\._]*|Domicilié à\s*[:\s\._]*|R[eé]f[eé]rence\s*[:\s\._]*|Pi[eéè]ce\s*[:\s\._]*|Stuk\s*[:\s\._]*|Détails tissu\s*[:\s\._]*|Détails\s*[:\s\._]*|Hauteu[r]?\s*[:\s\._]*|Hoogte\s*[:\s\._]*|Largeu[r]?\s*[:\s\._]*|Breedte\s*[:\s\._]*|À gauche|À droite|Gauge\s*[:\s\._]*|Droite\s*[:\s\._]*)`)
	norm = labelRe.ReplaceAllString(norm, "\n$1")

	rec := Record{}

	var itemX, itemY int
	// Support 3 or 4 part IDs (e.g. 60FG.00001.001.001)
	reCmd := regexp.MustCompile(`(?i)Commande\s*[:\s]*([A-Z0-9]{4}\.[A-Z0-9]{5}(?:\.[A-Z0-9]{3,4})+)`)
	reRawCmd := regexp.MustCompile(`([A-Z0-9]{4}\.[A-Z0-9]{5}(?:\.[A-Z0-9]{3,4})+)`)

	if m := reCmd.FindStringSubmatch(norm); m != nil {
		rec.OrderNumber, itemX, itemY = extractOrderFields(m[1])
	} else if m := reRawCmd.FindStringSubmatch(norm); m != nil {
		rec.OrderNumber, itemX, itemY = extractOrderFields(m[0])
	}

	reRef := regexp.MustCompile(`(?i)R[eé]f[eé]rence\s*[:\s]+([^\n\r]*)`)
	if m := reRef.FindStringSubmatch(norm); m != nil {
		rec.Reference = strings.TrimSpace(m[1])
	}

	reCode := regexp.MustCompile(`(?i)KLT-[\s\r\n]*(\d+)`)
	if cm := reCode.FindStringSubmatch(norm); len(cm) > 1 {
		rec.ClientCode = cm[1]
	} else {
		reRawCode := regexp.MustCompile(`\(KLT-(\d+)\)`)
		if cm := reRawCode.FindStringSubmatch(norm); len(cm) > 1 {
			rec.ClientCode = cm[1]
		}
	}

	reNom := regexp.MustCompile(`(?si)Nom\s*:\s*(.*?)\s*\(KLT-.*?\)`)
	if m := reNom.FindStringSubmatch(norm); m != nil {
		rec.ClientName = strings.Join(strings.Fields(m[1]), " ")
	}

	rec.Piece = extractPiece(norm)

	// --- OCR POSITIONAL FALLBACK ---
	// If Piece was blank or invalid, try to find it avoiding Name/Address fields
	if rec.Piece == "" {
		lines := strings.Split(norm, "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if isValValidPiece(l) {
				rec.Piece = l
				break
			}
		}
	}

	// -----------------------------
	// SPLIT DEBUG
	// -----------------------------
	logDebug("\n--- DÉBOGAGE DE SÉPARATION ---")
	logDebug("Aperçu : %.200s", block)

	// --- POSITIONAL FALLBACK FOR LABEL-LESS LAYOUTS ---
	// If standard labels failed, we use the known order:
	// Line 1: ID, Line 2: Ref, Line 3: Piece
	if rec.Reference == "" || rec.Piece == "" {
		lines := strings.Split(norm, "\n")
		var cleanLines []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				cleanLines = append(cleanLines, l)
			}
		}

		if len(cleanLines) >= 3 {
			// Check if first line is our ID pattern
			if regexp.MustCompile(`\d{4}\.\d{5}\.\d{3}`).MatchString(cleanLines[0]) {
				if rec.Reference == "" {
					rec.Reference = cleanLines[1]
				}
				if rec.Piece == "" {
					rec.Piece = cleanLines[2]
				}
			}
		}
	}

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

		logDebug("NOMBRE DE RESULTATS DE SEPARATION : %d", len(results))

		return results
	}

	// Regex to check if values are > 0 (Permissive for newlines and Dutch)
	reLeftVal := regexp.MustCompile(`(?is)(?:gauche|links)[\r\n\s\.:\-_]*([1-9]\d*)`)
	reRightVal := regexp.MustCompile(`(?is)(?:droite|rechts)[\r\n\s\.:\-_]*([1-9]\d*)`)
	isLeftPos := reLeftVal.MatchString(norm)
	isRightPos := reRightVal.MatchString(norm)

	// CASE A: Gauche/Droite values found (The "Paire" rule)
	if isLeftPos && isRightPos && hStr != "" {
		sz := extractSize(norm)
		if sz != "" {
			// Clean "cm" or other junk from the size string before math
			sz = regexp.MustCompile(`(?i)\s*cm`).ReplaceAllString(sz, "")
			parts := strings.Split(sz, " x ")
			if len(parts) == 2 {
				wStr := strings.TrimSpace(parts[0])
				hVal := strings.TrimSpace(parts[1])
				
				wVal, err := strconv.ParseFloat(strings.ReplaceAll(wStr, ",", "."), 64)
				if err == nil && wVal > 0 {
					halfW := wVal / 2
					
					r1 := rec
					r1.OrderItem = fmt.Sprintf("%d/1", itemX)
					r1.Size = cleanDim(strconv.FormatFloat(halfW, 'f', -1, 64)) + " x " + cleanDim(hVal)
					
					r2 := rec
					r2.OrderItem = fmt.Sprintf("%d/2", itemX)
					r2.Size = cleanDim(strconv.FormatFloat(halfW, 'f', -1, 64)) + " x " + cleanDim(hVal)
					
					logDebug("RÈGLE PAIRE APPLIQUÉE: %s x %s", cleanDim(strconv.FormatFloat(halfW, 'f', -1, 64)), cleanDim(hVal))
					return []Record{r1, r2}
				}
			}
		}
	}

	rec.OrderItem = fmt.Sprintf("%d/%d", itemX, itemY)
	rec.Size = extractSize(norm)

	// If Client Info is still missing, we might have a block that missed the header
	// but contains the data elsewhere. One last check:
	if rec.ClientName == "" {
		if m := regexp.MustCompile(`(?i)Nom\s*[:\s]+([^\n\r\(]*)`).FindStringSubmatch(norm); m != nil {
			rec.ClientName = strings.TrimSpace(m[1])
		}
	}

	return []Record{rec}
}

func extractSize(block string) string {
	h, w := "", ""

	// 1. Strict Label-based search (Requires colon - ignores stray "Largeur 235" junk)
	reHStrict := regexp.MustCompile(`(?i)(?:Hauteu[r]?|Hoogte|Height)\s*:\s*([\d.,]+)`)
	reWStrict := regexp.MustCompile(`(?i)(?:Largeu[r]?|Breedte|Width)\s*:\s*([\d.,]+)`)

	if m := reHStrict.FindStringSubmatch(block); m != nil {
		h = strings.ReplaceAll(m[1], ",", ".")
	}
	if m := reWStrict.FindStringSubmatch(block); m != nil {
		w = strings.ReplaceAll(m[1], ",", ".")
	}

	// 2. Permissive Label-based search (Fallback if OCR lost the colon)
	if h == "" {
		reH := regexp.MustCompile(`(?i)(?:Hauteu[r]?|Hoogte|Height)[\s\.:\-_=]*([\d.,]+)`)
		if m := reH.FindStringSubmatch(block); m != nil {
			h = strings.ReplaceAll(m[1], ",", ".")
		}
	}
	if w == "" {
		reW := regexp.MustCompile(`(?i)(?:Largeu[r]?|Breedte|Width)[\s\.:\-_=]*([\d.,]+)`)
		if m := reW.FindStringSubmatch(block); m != nil {
			w = strings.ReplaceAll(m[1], ",", ".")
		}
	}

	// 3. OCR Fallback (No labels, just "Width x Height")
	if h == "" || w == "" {
		// IMPORTANT: Ignore the "Benodigd" line which often contains "Quantity x Width"
		cleanOCR := regexp.MustCompile(`(?i).*Benodigd.*`).ReplaceAllString(block, "")

		reOCR := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[xX]\s*(\d+(?:\.\d+)?)`)
		if m := reOCR.FindStringSubmatch(cleanOCR); m != nil {
			v1, _ := strconv.ParseFloat(m[1], 64)
			v2, _ := strconv.ParseFloat(m[2], 64)

			// Heuristic: If one number is very small (1, 2, 3...) it's likely a quantity, not a size.
			if v1 > 5 && v2 > 5 {
				return m[1] + " x " + m[2]
			}
		}
		// Special heuristic for "154.5 cm \n 90.5 cm" appearance
		reCM := regexp.MustCompile(`([\d.,]+)\s*cm`)
		matches := reCM.FindAllStringSubmatch(block, -1)
		if len(matches) >= 2 {
			hRaw := matches[len(matches)-2][1]
			wRaw := matches[len(matches)-1][1]
			h = strings.ReplaceAll(hRaw, ",", ".")
			w = strings.ReplaceAll(wRaw, ",", ".")
		}
	}

	// 4. Special fallback for "Hauteur de coupe" (Commande Tissu)
	if h == "" {
		reHC := regexp.MustCompile(`(?i)Hauteur de coupe\s*[:]?\s*([\d.,]+)`)
		if m := reHC.FindStringSubmatch(block); m != nil {
			h = strings.ReplaceAll(m[1], ",", ".")
		}
	}
	if w == "" {
		reWC := regexp.MustCompile(`(?i)Largeur de coupe\s*[:]?\s*([\d.,]+)`)
		if m := reWC.FindStringSubmatch(block); m != nil {
			w = strings.ReplaceAll(m[1], ",", ".")
		}
	}

	if w != "" && h != "" {
		// IMPORTANT: DO NOT divide by 2 here. The 'Paire' rule division 
		// is explicitly handled in parseBlock. Doing it here causes a double-division.
		return cleanDim(w) + " x " + cleanDim(h)
	}
	if w != "" {
		return cleanDim(w)
	}
	if h != "" {
		return cleanDim(h)
	}
	return ""
}

// Helper to remove trailing .0 from dimensions
func cleanDim(s string) string {
	s = strings.ReplaceAll(s, ",", ".")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	// 'f' with -1 precision automatically removes unnecessary trailing zeros
	return strconv.FormatFloat(f, 'f', -1, 64)
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
	s := "Orders"
	ex.SetSheetName(ex.GetSheetName(0), s)

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
		fmt.Printf("\n[ERREUR FATALE] Impossible de sauvegarder le fichier Excel : %v\n", err)
		fmt.Printf("--> VERIFIEZ QUE LE FICHIER N'EST PAS OUVERT DANS EXCEL !\n")
		fmt.Println("\nAppuyez sur Entree pour quitter...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	} else {
		fmt.Printf("[OK] %d lignes ecrites avec succes dans Excel !\n", len(r))

		// ARCHIVE: Create a timestamped copy in an 'archive' subfolder
		dir := filepath.Dir(f)
		archiveDir := filepath.Join(dir, "archive")
		os.MkdirAll(archiveDir, 0755)

		timestamp := time.Now().Format("20060102_150405")
		archivePath := filepath.Join(archiveDir, fmt.Sprintf("output_%s.xlsx", timestamp))
		
		if err := ex.SaveAs(archivePath); err == nil {
			fmt.Printf("[ARCHIVE] Copie sauvegardee : %s\n", filepath.Base(archivePath))
		}
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
		fmt.Printf("[ERREUR] ERREUR lors de l'ouverture du fichier journal: %v\n", err)
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
