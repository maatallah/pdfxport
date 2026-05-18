# 📦 PDFXport — Engineering Handover Guide

**Local path:** `M:\dev\cpt\PDFXport`\
**GitHub repo:** <https://github.com/maatallah/pdfxport>\
**Active branch:** `workspace-sync`\
**System type:** Three-component pipeline — Chrome extension + Go orchestrator + PDF parser

> **Quick start on a new PC:** run `.\setup_dev_env.ps1` from the project root.

---

# 1. 🧠 System Overview

PDFXport is a **three-component pipeline** that:

1. **Hassad** (Chrome extension, `hassad/`) — intercepts Sieval API responses
   while the operator browses Decoloop, silently captures order IDs + polygon IDs
   + JWT tokens, stages them in a local buffer, then submits jobs on demand.
2. **Moissonneuse-Serveur** (Go orchestrator) — receives jobs from the extension
   via HTTP, queues them in SQLite, calls the Sieval API to download each PDF.
3. **Moissonneuse-Moulin** (Go parser) — reads the downloaded PDFs, extracts
   structured order data (dimensions, client, reference, piece type…), and writes
   an Excel file ready for label printing.

---

## 🧩 Full System Architecture

```
┌──────────────────────────────────────────────────────────┐
│  LAYER 1 — Capture  (hassad/ — ACTIVE extension v3.5)    │
│  Chrome Extension "PDFXport Harvester" (MV3)             │
│  ├─ inject_xhr.js     hooks XHR to capture JSON API resp │
│  ├─ content_main.js   staging buffer + harvest trigger   │
│  ├─ background.js     legacy: Base64→PDF→/upload         │
│  ├─ popup.html/.js    operator UI (toggle, counter, btn) │
│  └─ manifest.json     v3.5, host: decoloop.com + sieval  │
│                                                          │
│  ⚠ trash/extension/  = old blob-intercept approach       │
│     (kept for reference, NOT loaded in Chrome)           │
└──────────────────────────────────────────────────────────┘
                         ↓ POST /add  (JSON job payload)
┌──────────────────────────────────────────────────────────┐
│  LAYER 2 — Orchestrator  (Moissonneuse-Serveur.exe)      │
│  Go module: pdfxport-orchestrator/                       │
│  ├─ cmd/main.go        systray + HTTP server + loop      │
│  ├─ internal/client/   Sieval API client (JWT auth)      │
│  ├─ internal/queue/    SQLite job queue                  │
│  ├─ internal/storage/  PDF file output                   │
│  ├─ internal/engine/   batch engine                      │
│  └─ internal/utils/    embedded tray icon                │
│                                                          │
│  Endpoints:                                              │
│  POST /ingest  ← receive job from extension              │
│  GET  /health  ← health probe                           │
│  GET  /metrics ← harvest stats                           │
│  POST /upload  ← legacy binary PDF upload               │
└──────────────────────────────────────────────────────────┘
                         ↓ PDFs → ./output/<OrderNum>/
┌──────────────────────────────────────────────────────────┐
│  LAYER 3 — Parser  (Moissonneuse-Moulin.exe / ptxrid.exe)│
│  Go module: root (module: pdfparser)                     │
│  ├─ main.go            CLI + server + processing loop    │
│  ├─ unc_windows.go     UNC path resolution (Win7 UAC)    │
│  ├─ unc_other.go       stub for non-Windows builds       │
│  └─ pdf/               git submodule (ledongthuc/pdf)    │
│                                                          │
│  Extraction pipeline per PDF:                            │
│  1. extractText()  — ledongthuc/pdf library              │
│  2. splitBlocks()  — split by Sur-mesure / Commande ID   │
│  3. parseBlock()   — regex-based field extraction        │
│  4. OCR fallback   — NAPS2.Console.exe + Tesseract       │
│  5. exportExcel()  — excelize, text format, archive copy │
└──────────────────────────────────────────────────────────┘
                         ↓ output.xlsx  +  archive/
┌──────────────────────────────────────────────────────────┐
│  LAYER 4 — Output                                        │
│  output.xlsx      → label printing system (Excel-based)  │
│  archive/         → timestamped copies per batch         │
│  extraction_log.tsv → per-file stats (items, OCR, moved) │
└──────────────────────────────────────────────────────────┘
```

---

# 2. 📁 Complete Project Structure

```
PDFXport/
│
├── ── BINARY 1: MOISSONNEUSE-SERVEUR ─────────────────────
│
├── pdfxport-orchestrator/              Go module: pdfxport-orchestrator
│   ├── cmd/
│   │   └── main.go                    entry: systray + HTTP + processing goroutine
│   ├── internal/
│   │   ├── client/sieval.go           API client: JWT auth, PDF download
│   │   ├── config/config.go           runtime config
│   │   ├── engine/batch.go            batch processing logic
│   │   ├── queue/sqlite.go            SQLite queue (FetchBatch/MarkDone/MarkFailed/Close)
│   │   ├── storage/file_store.go      PDF output persistence
│   │   └── utils/icon.go             embedded systray icon ([]byte)
│   ├── build/                         local build output (git-ignored)
│   ├── jobs.db                        live queue (git-ignored)
│   ├── output/                        downloaded PDFs (git-ignored)
│   ├── go.mod
│   └── go.sum
│
├── ── BINARY 2: MOISSONNEUSE-MOULIN ──────────────────────
│
├── main.go                            Go entry point — CLI flags + parse loop
├── unc_windows.go                     UNC path resolver (Windows 7 network drives)
├── unc_other.go                       build stub for non-Windows
├── go.mod                             module: pdfparser, Go 1.20
├── go.sum
│
├── pdf/                               git SUBMODULE — ledongthuc/pdf (forked)
│   ├── read.go                        PDF reader
│   ├── page.go                        page/text extraction
│   └── text.go                        text layer
│
├── App/                               OCR runtime (git-ignored)
│   ├── NAPS2.Console.exe              OCR engine CLI (NAPS2 portable)
│   └── tessdata/                      Tesseract language data (fra.traineddata etc.)
│
├── ── OPERATIONAL SCRIPTS ─────────────────────────────────
│
├── ptxrid.exe                         Compiled Moulin parser binary
├── ptxrid.bat                         Interactive launcher (prompts for in/out dirs)
├── start_server.bat                   Starts Moulin in -server mode (HTTP receiver)
├── update_codes.ps1                   Cross-checks codes.txt against downloaded PDFs
├── move_pdfs.ps1                      Helper to move PDFs between folders
│
├── ── RELEASE BUILDS ──────────────────────────────────────
│
├── release/                           Built binaries (git-ignored)
│   ├── Moissonneuse-Serveur.exe       orchestrator binary
│   ├── Moissonneuse-Moulin.exe        parser binary
│   ├── jobs.db / jobs.db-shm / -wal  live orchestrator queue
│   └── output/                        downloaded PDFs
│
├── ── ACTIVE CHROME EXTENSION ─────────────────────────────
│
├── hassad/                            ★ ACTIVE extension — load THIS in Chrome
│   ├── manifest.json                  MV3, v3.5 "PDFXport Harvester"
│   ├── inject_xhr.js                  page-context XHR hook (JSON interceptor)
│   ├── content_main.js                staging buffer + harvest trigger handler
│   ├── background.js                  service worker (legacy /upload path)
│   ├── popup.html                     operator UI (dark golden wheat theme)
│   └── popup.js                       health check + buffer counter + harvest btn
│
├── ── LEGACY / REFERENCE ──────────────────────────────────
│
├── trash/                             Old blob-intercept extension (DO NOT LOAD)
│   ├── extension/                     last working blob-intercept MV3 version
│   │   ├── manifest.json
│   │   ├── content_main.js
│   │   ├── background.js
│   │   ├── inject_xhr.js
│   │   ├── inject_fetch.js
│   │   └── inject_blob.js
│   └── BulkOrder/                     bulk-order variant of the old extension
│
├── ── DOCUMENTATION ───────────────────────────────────────
│
├── handover_guide.md                  ← this file
├── setup_dev_env.ps1                  new-PC automated setup script
├── walkthrough_fr.md                  French operational walkthrough
├── Drapeaux parseur.md                Moulin CLI flags reference
├── Rules_for_size_and_item_ordre.xlsx parsing business rules reference
└── test.http                          REST Client test file for HTTP endpoints
```

---

# 3. ⚙️ Technology Stack

## Hassad — Chrome Extension

| Component | Details |
|-----------|---------|
| Standard | Chrome Extension Manifest V3 |
| Language | Vanilla JavaScript (ES2020) |
| Storage | `chrome.storage.local` (staging buffer) |
| Target host | `*.decoloop.com` + `sievalhub.sieval.com` |
| Server endpoint | `http://localhost:8765/add` (job submission) |

## Moissonneuse-Serveur (orchestrator)

| Component | Details |
|-----------|---------|
| Language | Go 1.26.2 |
| CGO | **Required** (go-sqlite3 + systray) |
| C compiler | MinGW-W64 GCC 15.x |
| DB | SQLite via `github.com/mattn/go-sqlite3 v1.14.44` |
| Tray | `github.com/getlantern/systray v1.2.2` |

## Moissonneuse-Moulin (parser)

| Component | Details |
|-----------|---------|
| Language | Go 1.20 (last version with official Win7 support) |
| CGO | **Not required** — pure Go |
| PDF library | `github.com/ledongthuc/pdf` (local submodule `./pdf`) |
| Excel output | `github.com/xuri/excelize/v2 v2.8.1` |
| OCR fallback | NAPS2.Console.exe + Tesseract (`fra` language model) |

---

# 4. 🌾 Hassad — Chrome Extension (Active)

**Folder:** `hassad/` — load this directory in Chrome as an unpacked extension.

> **Critical distinction:** `hassad/` does NOT intercept PDF blobs.
> It intercepts **JSON API responses** to silently harvest order metadata,
> then sends structured job payloads to the orchestrator on demand.
> The old blob approach (`trash/`) is no longer used.

## How it works

### Stage 1 — Passive interception (`inject_xhr.js`)

Injected into the page context of `*.decoloop.com`. Hooks `XMLHttpRequest` to:
- Capture the **Authorization header** (JWT Bearer token) from any outgoing request
- Intercept responses from:
  - `browse` endpoints → bulk list of `{projectNumber, id, polygons[]}`
  - `getProjectDetailsById` → single project details
- If polygon IDs are missing, performs a **background fetch** to
  `getProjectDetailsById?productionProjectId=<id>` using the captured JWT
- Emits `HARVESTED_ID` postMessage events to `content_main.js`

```javascript
// What is captured per order:
{
  orderNum:  "60FG.00063",   // e.g. projectNumber from browse response
  projectId: 210576,          // item.id
  polygons:  [573286, 573287],// polygon IDs
  token:     "eyJ..."        // stripped Bearer JWT
}
```

### Stage 2 — Staging buffer (`content_main.js`)

- Listens for `HARVESTED_ID` messages
- Stores entries keyed by `orderNum` in `chrome.storage.local` under `stagingBuffer`
- Buffer persists across page navigations
- Responds to `TRIGGER_HARVEST` messages from the popup

### Stage 3 — Manual harvest trigger (`popup.js` + `content_main.js`)

When the operator clicks **"Moissonner la Sélection"** in the popup:
1. `content_main.js` reads checked rows in the Decoloop table
   (`tbody tr[role="row"] mat-checkbox.mat-checkbox-checked`)
2. For each selected `orderNum`, looks up its entry in the staging buffer
3. POSTs to `http://localhost:8765/add`:

```json
{
  "orderNum":   "60FG.00063",
  "projectId":  210576,
  "documentId": 38,
  "polygons":   [573286, 573287],
  "lang":       "fr",
  "token":      "eyJ..."
}
```

> **Note:** `documentId: 38` is currently hardcoded — this is the Sieval
> document type ID for production order PDFs. Change if needed.

### Stage 4 — Popup UI (`popup.html` / `popup.js`)

| UI element | Function |
|------------|----------|
| Server status dot 🟡/🔴 | Polls `GET /health` every 3 seconds |
| "Récolte Active" toggle | Enables/disables XHR interception |
| Buffer counter | Shows `N commande(s) prête(s)` from `stagingBuffer` |
| "Moissonner la Sélection" | Sends selected checked rows to orchestrator |
| "Vider la Grange" | Clears `stagingBuffer` in `chrome.storage.local` |

### What `background.js` does in `hassad/`

`background.js` in `hassad/` handles the **legacy `UPLOAD_PDF` message** path —
it decodes a Base64 PDF and POSTs it to `/upload`. This path is no longer
the primary flow (superseded by the JSON interception approach) but is preserved
for backward compatibility.

## Installing the extension

```
1. Open Chrome → chrome://extensions
2. Enable "Developer mode" (top right)
3. Click "Load unpacked"
4. Select: M:\dev\cpt\PDFXport\hassad\
5. Extension appears as "PDFXport Harvester" v3.5
6. Pin it to toolbar for easy access
```

## Debugging the extension

```
# Content script logs:
Right-click page → Inspect → Console (filter: "Moissonneuse")

# Background service worker logs:
chrome://extensions → PDFXport Harvester → "Service Worker" link → Inspect

# Check staging buffer:
chrome://extensions → PDFXport Harvester → Service Worker → Console:
chrome.storage.local.get(['stagingBuffer'], console.log)
```

## Extension file summary

| File | Role |
|------|------|
| `manifest.json` | MV3, v3.5, declares host permissions and content script |
| `inject_xhr.js` | Page-context XHR hook — harvests order JSON + JWT |
| `content_main.js` | Injects hook, manages staging buffer, handles trigger |
| `background.js` | Service worker — legacy PDF upload path |
| `popup.html` | Dark golden-wheat themed operator dashboard |
| `popup.js` | Health check, buffer counter, harvest/clear buttons |

---

# 4. 🔄 Data Flow (end to end)

```
STEP 0 — Passive capture (Hassad extension, automatic while browsing)
  inject_xhr.js intercepts XHR responses on decoloop.com
    → captures orderNum + projectId + polygons[] + JWT token
    → stores in chrome.storage.local stagingBuffer

STEP 1 — Job submission (operator clicks "Moissonner la Sélection")
  content_main.js reads checked rows in Decoloop table
    → for each selected order, looks up stagingBuffer
    → POST /add  to Moissonneuse-Serveur (JSON payload)
    → job stored in jobs.db (status=pending)
  [Alternative: POST /ingest for direct API or testing use]

STEP 2 — PDF harvesting
  Moissonneuse-Serveur polling loop
    → FetchBatch(1) from SQLite
    → POST /api/productiondata/document/generatePdfDocument
         Host: sievalhub.sieval.com
         Authorization: Bearer <JWT>
    → PDF binary received
    → Saved to pdfxport-orchestrator/output/<OrderNum>/

STEP 3 — PDF parsing
  Moissonneuse-Moulin watching ./in folder (or -dir flag)
    → extractText() via ledongthuc/pdf library
    → splitBlocks(): split text by "Sur-mesure:" headers
       Fallback 1: split by "Commande:" labels
       Fallback 2: split by raw order ID (XXXX.XXXXX.XXX pattern)
       Fallback 3: treat whole file as one block
    → parseBlock(): regex extraction per block
       ├─ Order ID, Item number (X/Y/Z format)
       ├─ Client code (KLT-XXXXXX)
       ├─ Client name
       ├─ Reference
       ├─ Piece (product type)
       └─ Size (Hauteur x Largeur, multilingual FR/NL/EN)
    → Business rules applied:
       ├─ PAIRE RULE: Left+Right > 20cm → split 1 record into 2 (width ÷ 2 each)
       ├─ GAUGE/DROITE RULE: À gauche=0, À droite=0 → split by Gauge/Droite values
       ├─ TISSU FALLBACK: "Hauteur de coupe" / "Largeur de coupe" for fabric orders
       ├─ DOUBLURE SKIP: blocks with "sur-mesure: doublure" are ignored
       └─ RAIL SKIP: rail blocks without dimensions are ignored

STEP 4 — OCR fallback (automatic)
  Triggered when: 0 records extracted OR any record missing Size
    → NAPS2.Console.exe -i <pdf> -o <pdf> --ocr --ocrlang fra --dpi 300
    → Re-runs extractText() on the OCR-enhanced PDF
    → Second parse pass
    → OCR log saved to ./in/OCRed/<file>_ocr_raw.txt

STEP 5 — File management
  Success → PDF moved to ./in/processed/<OrderNum>.pdf
  Failure → PDF moved to ./in/en_instance/  (manual review queue)

STEP 6 — Output
  output.xlsx    → "Orders" sheet, all columns text-formatted (NumFmt 49)
  archive/output_YYYYMMDD_HHMMSS.xlsx → timestamped backup copy
  extraction_log.tsv → per-file: items processed/extracted/skipped/moved/OCR
```

---

# 5. 🧮 Parsed Data Fields

| Field | Source in PDF | Notes |
|-------|--------------|-------|
| `OrderNumber` | `Commande: XXXX.XXXXX` | First two segments of the dot-separated ID |
| `OrderItem` | Dot ID parts 3+4, formatted `X/Z` | Z = max item in that order |
| `ClientCode` | `KLT-XXXXXX` | 3–7 digit numeric code |
| `ClientName` | `Nom: ...` | Free text |
| `Reference` | `Référence: ...` | Product reference string |
| `Piece` | `Pièce: ...` / positional OCR fallback | Room name, e.g. "Salon" |
| `Size` | `Hauteur/Largeur` (FR/NL/EN) in cm | `W x H` format, trailing .0 stripped |

---

# 6. 🖥️ Parser CLI Flags (Moulin)

```
MODES
  -server     Start HTTP mode (receives PDFs from Chrome extension on port 8765)
  -dump       Extract raw text only → saves to out/parsed.txt (diagnostic)

FILES / FOLDERS
  -input      Single PDF file path
  -dir        Input folder (default: ./in)
  -outdir     Output folder (default: ./out)

EXPORT FORMATS
  -excel      Generate output.xlsx (DEFAULT: true)
  -json       Generate output.json
  -csv        Generate output.csv

DIAGNOSTIC
  -debug      Verbose mode: logs every block parse, field match, rule trigger
              Also writes <filename>_debug.txt next to each PDF
```

**Examples:**
```powershell
# Normal batch processing (interactive, press Enter each run)
.\ptxrid.exe -dir ".\in" -outdir ".\out"

# Single file
.\ptxrid.exe -input ".\in\60FG.00063.pdf" -excel

# Diagnose a problematic PDF
.\ptxrid.exe -input ".\in\problem.pdf" -dump -debug

# Run as HTTP server to receive from Chrome extension
.\ptxrid.exe -server
```

---

# 7. 🔐 API Reference

### Generate PDF (Sieval)
```http
POST https://sievalhub.sieval.com/api/productiondata/document/generatePdfDocument
Authorization: Bearer <JWT>
Content-Type: application/json

{
  "productionProjectId": 210576,
  "productionPolygonIds": [573286, 573287],
  "languageCode": "fr"
}
```

### Orchestrator endpoints (port 8765)
```http
GET  /health                  → 200 OK + CORS headers
GET  /metrics                 → harvest stats JSON
POST /ingest                  → submit a new job (JSON body below)
POST /upload                  → legacy: raw PDF binary stream
GET  /log  (POST)             → browser debug log receiver
```

### Ingest job payload
```json
{
  "orderNum":   "60FG.00063",
  "projectId":  210576,
  "documentId": 12345,
  "polygons":   [573286, 573287],
  "lang":       "fr",
  "token":      "eyJ..."
}
```

---

# 8. 🗄️ SQLite Queue Schema

```sql
CREATE TABLE jobs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    order_num   TEXT,
    project_id  INTEGER,
    document_id INTEGER,
    polygons    TEXT,           -- JSON array "[573286,573287]"
    lang        TEXT,
    token       TEXT,
    status      TEXT DEFAULT 'pending',  -- pending | done | failed
    attempts    INTEGER DEFAULT 0,
    last_error  TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

| Function | Description |
|----------|-------------|
| `FetchBatch(n)` | Atomically claims n pending jobs |
| `MarkDone(id)` | Sets status=done |
| `MarkFailed(id, err)` | Sets status=failed, increments attempts, stores error |
| `Close()` | Safely closes DB handle (must call before deleting jobs.db) |

---

# 9. 🖥️ Systray Menu (Serveur)

| Item | Action |
|------|--------|
| 🌾 Statut: En écoute (Port 8765) | Disabled label |
| 🧹 Vider le Grenier | `queue.Close()` → delete jobs.db → reopen queue |
| 🚪 Quitter | `systray.Quit()` → graceful shutdown |

**Tooltip states:**
- `En attente...` — idle
- `🚜 Récolte en cours : 60FG.00063` — processing
- `🌾 42 récoltés (Dernier: 60FG.00063)` — batch complete

---

# 10. 🏗️ Build Instructions

## Prerequisites
```powershell
.\setup_dev_env.ps1   # checks and installs all required tools
```

## Moissonneuse-Serveur (requires CGO + MinGW)
```powershell
cd pdfxport-orchestrator
$env:CGO_ENABLED = "1"
go build -o ..\release\Moissonneuse-Serveur.exe .\cmd\main.go
```

## Moissonneuse-Moulin / ptxrid (pure Go, no CGO needed)
```powershell
# From project root
go build -o release\Moissonneuse-Moulin.exe .
# or as the legacy name:
go build -o ptxrid.exe .
```

## Build with version tag
```powershell
$ver = "1.3.0"
# Serveur
cd pdfxport-orchestrator
go build -ldflags "-X main.Version=$ver" -o ..\release\Moissonneuse-Serveur.exe .\cmd\main.go
cd ..
# Moulin
go build -ldflags "-X main.Version=$ver" -o release\Moissonneuse-Moulin.exe .
```

## Run in dev mode
```powershell
# Serveur
cd pdfxport-orchestrator && $env:CGO_ENABLED="1" && go run .\cmd\main.go

# Moulin (interactive)
go run . -dir .\in -outdir .\out -debug

# Moulin (server mode)
go run . -server
```

---

# 11. 🔬 OCR Subsystem

The Moulin parser has an **automatic OCR fallback** for image-only or badly
scanned PDFs.

### Trigger conditions
- Zero records extracted from a PDF, OR
- Any extracted record is missing its `Size` field

### OCR Engine
- **NAPS2 Portable** (CLI) — must be present in `./App/NAPS2.Console.exe`
- **Tesseract** bundled with NAPS2 — French language model (`fra`)

### Process
```
1. NAPS2.Console.exe -i <pdf> -o <pdf> --ocr --ocrlang fra --dpi 300
   (overwrites the PDF with an OCR text layer)
2. re-run extractText() on the enhanced PDF
3. raw OCR text saved to ./in/OCRed/<filename>_ocr_raw.txt
4. second parse pass with same block-splitting logic
```

### Setup on a new PC
```powershell
# Copy the App folder from an existing installation
# It contains NAPS2.Console.exe + tessdata/
# NAPS2 portable: https://www.naps2.com/download
# Tesseract language files: https://github.com/tesseract-ocr/tessdata
```

> **Note:** `App/` is in `.gitignore`. Copy it manually when setting up a new machine.

---

# 12. 📜 Operational Scripts

| Script | Purpose |
|--------|---------|
| `ptxrid.bat` | Interactive launcher — prompts for input/output dirs, runs Moulin |
| `start_server.bat` | Starts Moulin in `-server` mode (for Chrome extension pipeline) |
| `update_codes.ps1` | Compares `codes.txt` against downloaded PDFs in the output folder, marks which orders are present (`Y`/blank) |
| `move_pdfs.ps1` | Helper to batch-move PDFs between working folders |

### `update_codes.ps1` details
```powershell
# Cross-check codes.txt (tab-separated: Y/blank  TAB  OrderNum)
# against all PDFs in the orchestrator output folder.
.\update_codes.ps1 -CodesFile "M:\dev\cpt\PDFXport\codes.txt" `
                   -PdfDir    "M:\dev\cpt\PDFXport\pdfxport-orchestrator\output"
```

---

# 13. ⚠️ Known Constraints & Gotchas

### CGO is mandatory for Serveur
`go-sqlite3` and `getlantern/systray` are CGO packages.
Pure-Go builds (`CGO_ENABLED=0`) fail at compile time.
MinGW-W64 **must** be in `PATH`.

### Moulin requires Go 1.20 exactly for Windows 7 support
If building for modern Windows only, any Go version works.
Do not upgrade the root `go.mod` past `go 1.20` unless Win7 support is dropped.

### `pdf/` is a git submodule (forked `ledongthuc/pdf`)
After cloning:
```powershell
git submodule update --init --recursive
```
The fork patches are **local only** — the `go.mod` redirect `replace github.com/ledongthuc/pdf => ./pdf` makes this work.

### JWT token lifetime
Tokens expire. The orchestrator reads the token from the ingest payload — no
automatic refresh. The browser extension or job submitter must supply a fresh token.

### jobs.db WAL files
SQLite runs in WAL mode — three companion files exist (`jobs.db`, `jobs.db-shm`,
`jobs.db-wal`). **Never delete just `jobs.db`** — use the tray "Reset" button
which calls `queue.Close()` first, or delete all three files together.

### Port 8765 conflict
If port 8765 is taken, the HTTP server silently fails. Check:
```powershell
netstat -ano | findstr 8765
```

### Windows 7 network drive UAC bug
`unc_windows.go` resolves mapped drive letters (`M:\`) to UNC paths
(`\\server\share\`) before any file operation — this bypasses the UAC elevation
bug where mapped drives are invisible to elevated processes.

### `App/` folder is not in git
NAPS2 + Tesseract must be copied manually. See §11 above.

---

# 14. 🔁 Git Workflow

```
Branch: workspace-sync  (primary working branch)
Remote: origin = https://github.com/maatallah/pdfxport
Submodule: pdf/ → forked ledongthuc/pdf
```

### Daily workflow
```powershell
git pull origin workspace-sync       # always pull before starting work
# ... edit ...
git add <files>
git commit -m "feat|fix|refactor(scope): description"
git push
```

### After cloning (first time)
```powershell
git submodule update --init --recursive   # initialize pdf/ submodule
```

### What is **not** committed (`.gitignore`)
```
/in/  /out/  /App/  /dbg/  /BulkOrder/  /release/
*.pdf  *.xlsx  *.tsv  *.exe  *.exe~  *.7z  *.txt
jobs.db  jobs.db-shm  jobs.db-wal
```

---

# 15. 🆕 New PC Setup (full procedure)

```powershell
# Step 1 — Clone
git clone https://github.com/maatallah/pdfxport M:\dev\cpt\PDFXport
cd M:\dev\cpt\PDFXport

# Step 2 — Automated env check + tool install
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\setup_dev_env.ps1

# Step 3 — Initialize submodule
git submodule update --init --recursive

# Step 4 — Build Moulin (no CGO needed)
go build -o release\Moissonneuse-Moulin.exe .

# Step 5 — Build Serveur (CGO required)
cd pdfxport-orchestrator
$env:CGO_ENABLED = "1"
go build -o ..\release\Moissonneuse-Serveur.exe .\cmd\main.go
cd ..

# Step 6 — Copy App/ folder from another machine (NAPS2 + Tesseract)
#   Source: any existing installation's App\ directory
#   Destination: M:\dev\cpt\PDFXport\App\

# Step 7 — Create working folders
New-Item -ItemType Directory release\in, release\out -Force
```

Full automated steps 1–2: see `setup_dev_env.ps1`

---

# 16. 🧪 Testing

### Health check
```powershell
Invoke-WebRequest http://localhost:8765/health
```

### Submit a test ingest job
```powershell
$body = @{
    orderNum   = "TEST-001"
    projectId  = 210576
    documentId = 12345
    polygons   = @(573286, 573287)
    lang       = "fr"
    token      = "YOUR_JWT_HERE"
} | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8765/ingest -Method POST `
    -ContentType "application/json" -Body $body
```

### Check queue state
```powershell
sqlite3 release\jobs.db "SELECT id,order_num,status,attempts FROM jobs ORDER BY id DESC LIMIT 20;"
```

### Diagnose a bad PDF
```powershell
.\ptxrid.exe -input ".\in\problem.pdf" -dump -debug
# Check: .\in\problem_debug.txt  and  .\in\parsed.txt
```

### Smoke test — full pipeline
```powershell
# 1. Start Serveur
Start-Process .\release\Moissonneuse-Serveur.exe

# 2. Health check
Invoke-WebRequest http://localhost:8765/health | Select-Object StatusCode

# 3. Ingest test job (see above)

# 4. Watch output
Get-ChildItem .\release\output -Recurse | Sort-Object LastWriteTime -Descending | Select -First 5

# 5. Run Moulin on harvested PDFs
.\release\Moissonneuse-Moulin.exe -dir .\release\output -outdir .\release\out
```

---

# 17. 📌 Current Status & Roadmap

### ✅ Implemented
- Headless Go orchestrator (Serveur) with SQLite queue and systray UI
- Windows system tray: live status, reset queue, quit
- HTTP ingestion endpoint compatible with Chrome extension AND direct API use
- PDF parser (Moulin) with 4-pass extraction strategy
- Multi-language dimension support (FR/NL/EN labels)
- Paire rule (paired curtains → width ÷ 2, 2 records)
- Gauge/Droite rule (one-sided blind → split by individual panel widths)
- Tissu fallback (fabric orders → "Hauteur/Largeur de coupe")
- Automatic OCR fallback via NAPS2 + Tesseract
- Excel output with text formatting + timestamped archive copy
- UNC path resolution for Windows 7 network drives
- `update_codes.ps1` for cross-checking order completion
- `.gitignore` covering all runtime/build artifacts

### 🔜 Next priorities
- **Automatic JWT refresh** — detect 401 and prompt or re-auth
- **Retry with exponential backoff** — automatic re-queue for failed jobs
- **CSV batch ingestion** — load job lists from CSV without browser
- **Moulin → Serveur integration** — pipe downloaded PDFs directly into parsing
- **Dashboard** — local web UI for queue monitoring and result inspection

---

# 18. 🧭 Key Engineering Principles

> **Serveur** is a deterministic backend engine driven by API calls and a
> persistent queue — no browser required.

> **Moulin** is a defensive parser: every extraction has multiple fallback
> passes (text → OCR → positional), and files that cannot be parsed are
> quarantined to `en_instance/` rather than silently dropped.

> **The two binaries are independent** — they share a folder convention
> (`output/`) but have no compile-time dependency on each other.
