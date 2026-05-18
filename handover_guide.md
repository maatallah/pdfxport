# 📦 PDFXport — Engineering Handover Guide

**Local path:** `M:\dev\cpt\PDFXport`\
**GitHub repo:** <https://github.com/maatallah/pdfxport>\
**Active branch:** `workspace-sync`\
**System type:** Headless Go API orchestrator (Phase 2 — no browser dependency)

> **Quick start on a new PC:** run `.\setup_dev_env.ps1` from the project root.

---

# 1. 🧠 System Overview

PDFXport is a **headless PDF batch generation pipeline** that calls the Sieval API
directly — no browser, no Chrome extension — and stores the resulting PDFs for
downstream parsing and structured data export.

It is now fully in **Phase 2**: the browser extension is deprecated (kept for
reference only).

---

## 🧩 Architecture (current — Phase 2)

```
[Go Orchestrator — Moissonneuse]
        │
        ├─ SQLite queue  (jobs.db)
        │     • pending → processing → done / failed
        │
        ├─ HTTP server  (port 8765)
        │     POST /ingest   ← browser extension (legacy)
        │     GET  /health   ← monitoring
        │     GET  /metrics  ← harvest stats
        │
        ├─ API client  → sievalhub.sieval.com
        │     POST /api/productiondata/document/generatePdfDocument
        │
        ├─ File storage  → ./output/<OrderNum>/
        │
        └─ System tray UI  (Windows systray)
              • live tooltip: job count / active order
              • "Reset Grenier" → safely wipe jobs.db
              • "Quit" → graceful shutdown
```

---

# 2. 📁 Actual Project Structure

```
PDFXport/
│
├── pdfxport-orchestrator/          ← GO ORCHESTRATOR (main deliverable)
│   ├── cmd/
│   │   └── main.go                 # entry point — systray + HTTP + processing loop
│   ├── internal/
│   │   ├── client/
│   │   │   └── sieval.go           # API client (JWT auth, PDF download)
│   │   ├── config/
│   │   │   └── config.go           # runtime config
│   │   ├── engine/
│   │   │   └── batch.go            # batch processing logic
│   │   ├── queue/
│   │   │   └── sqlite.go           # SQLite job queue (FetchBatch, MarkDone, MarkFailed, Close)
│   │   ├── storage/
│   │   │   └── file_store.go       # PDF file persistence
│   │   └── utils/
│   │       └── icon.go             # embedded systray icon ([]byte)
│   ├── build/                      # local build output (git-ignored)
│   ├── go.mod
│   └── go.sum
│
├── pdf/                            # git submodule — PDF parser
├── release/                        # compiled release binaries (git-ignored)
│   ├── Moissonneuse-Moulin.exe     # PDF parser binary
│   ├── Moissonneuse-Serveur.exe    # orchestrator binary
│   ├── jobs.db                     # live queue database
│   └── output/                     # harvested PDFs
│
├── handover_guide.md               ← this file
├── setup_dev_env.ps1               ← new-PC setup script
├── .gitignore
└── walkthrough_fr.md               # French-language operational walkthrough
```

---

# 3. ⚙️ Technology Stack

| Tool | Version | Purpose |
|------|---------|---------|
| Go | ≥ 1.22 (currently 1.26.2) | Orchestrator language |
| GCC / MinGW-W64 | 15.x | CGO — required by go-sqlite3 + systray |
| go-sqlite3 | v1.14.44 | Persistent job queue |
| getlantern/systray | v1.2.2 | Windows system tray UI |
| Git | ≥ 2.x | Version control |
| Node.js / npm | optional | Browser extension tooling only |

> **CGO is mandatory.** `go-sqlite3` and `systray` both require a C compiler.
> Without MinGW-W64 in `PATH`, the build will fail.

---

# 4. 🔄 Data Flow (Phase 2 — implemented)

```
1. Browser extension (legacy) OR future scheduler
         ↓ POST /ingest
2. HTTP server receives job metadata (OrderNum, ProjectID, DocumentID, Polygons, Lang, Token)
         ↓
3. SQLite queue stores job with status=pending
         ↓
4. Processing goroutine polls queue (FetchBatch)
         ↓
5. API client calls sievalhub.sieval.com with JWT Bearer token
         ↓
6. PDF binary returned → saved to ./output/<OrderNum>/
         ↓
7. queue.MarkDone(id) — job complete
         ↓
8. Systray tooltip updated with progress count
```

---

# 5. 🔐 API Reference

### Generate PDF
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

### Health check
```
GET http://localhost:8765/health
```

### Metrics
```
GET http://localhost:8765/metrics
```

### Ingest job (from extension or test)
```
POST http://localhost:8765/ingest
Content-Type: application/json

{
  "orderNum": "ORD-001",
  "projectId": 210576,
  "documentId": 12345,
  "polygons": [573286, 573287],
  "lang": "fr",
  "token": "eyJ..."
}
```

---

# 6. 🏗️ Build Instructions

### Prerequisites
```powershell
# Check everything is ready
.\setup_dev_env.ps1
```

### Build Moissonneuse-Serveur (orchestrator)
```powershell
cd pdfxport-orchestrator
$env:CGO_ENABLED = "1"
go build -o build\Moissonneuse-Serveur.exe ./cmd/main.go
```

### Build with version info (release)
```powershell
$ver = "1.3.0"
go build -ldflags "-X main.Version=$ver" -o ..\release\Moissonneuse-Serveur.exe ./cmd/main.go
```

### Run in dev mode (console visible)
```powershell
cd pdfxport-orchestrator
$env:CGO_ENABLED = "1"
go run ./cmd/main.go
```

---

# 7. 🗄️ SQLite Queue Schema

```sql
CREATE TABLE jobs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    order_num  TEXT,
    project_id INTEGER,
    document_id INTEGER,
    polygons   TEXT,       -- JSON array
    lang       TEXT,
    token      TEXT,
    status     TEXT DEFAULT 'pending',   -- pending | done | failed
    attempts   INTEGER DEFAULT 0,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Key operations:**
| Function | Description |
|----------|-------------|
| `FetchBatch(n)` | Atomically claim n pending jobs |
| `MarkDone(id)` | Set status=done |
| `MarkFailed(id, err)` | Set status=failed, increment attempts, store error |
| `Close()` | Safely close DB handle (needed before deleting jobs.db) |

---

# 8. 🖥️ Systray Menu

| Menu item | Action |
|-----------|--------|
| 🌾 Statut: En écoute (Port 8765) | Disabled label — shows server status |
| 🧹 Vider le Grenier | Calls `queue.Close()`, removes `jobs.db`, re-opens queue |
| 🚪 Quitter | Calls `systray.Quit()` → triggers `onExit()` |

**Tooltip states:**
- `En attente...` — idle
- `🚜 Récolte en cours : ORD-001` — active job
- `🌾 42 récoltés (Dernier: ORD-042)` — after completion

---

# 9. 🧪 Testing

### Health check
```powershell
Invoke-WebRequest http://localhost:8765/health
```

### Submit a test job
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
cd release   # or wherever jobs.db lives
sqlite3 jobs.db "SELECT id, order_num, status, attempts FROM jobs ORDER BY id DESC LIMIT 20;"
```

### Full smoke test
```powershell
# 1. Start server
Start-Process .\release\Moissonneuse-Serveur.exe

# 2. Verify health
Invoke-WebRequest http://localhost:8765/health | Select-Object StatusCode

# 3. Ingest test job (see above)

# 4. Watch output folder
Get-ChildItem .\release\output -Recurse | Sort-Object LastWriteTime -Descending | Select -First 5
```

---

# 10. ⚠️ Known Constraints & Gotchas

### CGO is non-negotiable
`go-sqlite3` and `getlantern/systray` are CGO packages.
Pure-Go builds (`CGO_ENABLED=0`) will fail at compile time.
MinGW-W64 must be in `PATH`.

### JWT token lifetime
Tokens expire. The orchestrator reads the token from the ingest payload —
no automatic refresh. The browser extension or job submitter must supply a
fresh token.

### jobs.db WAL mode
SQLite runs in WAL mode (`jobs.db-wal`, `jobs.db-shm` companion files).
Do NOT delete only `jobs.db` — delete all three, or use the tray "Reset" button
which calls `queue.Close()` first.

### Port 8765 conflict
If another process holds port 8765, the server silently continues without
the HTTP listener. Check with:
```powershell
netstat -ano | findstr 8765
```

### Submodule (pdf parser)
The `pdf/` directory is a git submodule. After cloning:
```powershell
git submodule update --init --recursive
```

---

# 11. 🔁 Git Workflow

```
Branch: workspace-sync  (primary working branch)
Remote: origin = https://github.com/maatallah/pdfxport
```

### Daily workflow
```powershell
git pull origin workspace-sync   # always pull before starting work
# ... make changes ...
git add <files>
git commit -m "feat|fix|refactor(scope): description"
git push
```

### What is ignored (`.gitignore`)
```
/in/ /out/ /App/ /dbg/ /BulkOrder/ /release/
*.pdf *.xlsx *.tsv *.exe *.exe~ *.7z *.txt
```

> **Do NOT commit** `jobs.db`, `*.exe`, or the `release/` folder.

---

# 12. 🆕 New PC Setup (summary)

```powershell
# 1. Open PowerShell as Administrator
# 2. Clone repo
git clone https://github.com/maatallah/pdfxport M:\dev\cpt\PDFXport
cd M:\dev\cpt\PDFXport

# 3. Run the automated setup script
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\setup_dev_env.ps1

# 4. Initialize submodule
git submodule update --init --recursive

# 5. Build
cd pdfxport-orchestrator
$env:CGO_ENABLED = "1"
go build -o ..\release\Moissonneuse-Serveur.exe ./cmd/main.go
```

Full script with step verification, tool installation, and readiness summary:
→ `setup_dev_env.ps1`

---

# 13. 📌 Current Status & Roadmap

### ✅ Done (Phase 2 complete)
- Headless Go orchestrator replaces browser-based capture
- SQLite job queue with atomic fetch, done, failed states
- Windows system tray UI with live status and reset capability
- HTTP ingestion endpoint (port 8765)
- File storage per order number
- `.gitignore` excludes all build/runtime artifacts

### 🔜 Next priorities
- **Automatic token refresh** — detect 401 and prompt for new token
- **Retry backoff** — exponential retry for `failed` jobs
- **Scheduler / CSV batch ingestion** — load job lists from a CSV file without browser
- **PDF parser integration** — pipe downloaded PDFs directly into Moulin parser
- **Dashboard** — lightweight local web UI to monitor queue and results

---

# 14. 🧭 Key Engineering Principle

> The browser is no longer part of the pipeline.
> The system is a deterministic backend engine driven by API calls and a persistent queue.
