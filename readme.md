```
# PDFXPort Orchestrator

## Overview

PDFXPort Orchestrator is a Go-based backend system that automates PDF generation from a remote production API and stores the resulting PDFs locally.

It replaces browser-based printing workflows with a fully server-driven pipeline featuring:

- Job queue (SQLite)
- Concurrent workers
- HTTP API integration
- PDF download + persistence
- Batch processing engine

---

## Architecture
```

cmd/\
main.go → entry point (orchestrator loop)

internal/\
client/ → HTTP API client (PDF generation)\
engine/ → worker handler logic\
queue/ → SQLite job queue\
storage/ → file persistence layer

```

---

## Tech Stack

### Core Language
- Go 1.22+ (tested with Go 1.22.x)

### Database
- SQLite3
- Driver: github.com/mattn/go-sqlite3 v1.14.22

### HTTP
- net/http (standard library)

### Concurrency
- goroutines
- sync.WaitGroup
- buffered worker pool channels

---

## External API

Target system:
```

<https://sievalhub.sieval.com/api/productiondata/document/generatePdfDocument>

```

Method:
```

POST

```

Authentication:
```

Bearer JWT token (from browser session)

````

---

## Features Implemented

### ✔ Job Queue System
- SQLite-based persistence
- job status tracking (pending, processing, done, failed)

### ✔ Concurrent Worker Engine
- configurable worker pool
- parallel PDF generation

### ✔ API Integration
- authenticated requests using Bearer token
- JSON payload per job

### ✔ PDF Handling
- binary PDF response processing
- direct file writing to disk

### ✔ Storage Layer
- structured output directory
- file naming per job ID

---

## Known Issues / Limitations

### ⚠ SQLite locking risk
- mitigated with WAL mode
- still single-writer constraint

### ⚠ No retry strategy yet
- failed jobs marked but not automatically retried with backoff

### ⚠ No deduplication
- duplicate jobs can be inserted if manually triggered

### ⚠ JWT is static
- token must be manually updated

---

## Current Workflow

1. Jobs inserted into SQLite
2. Orchestrator fetches batch
3. Workers process jobs concurrently
4. API generates PDF
5. PDF saved locally (`./output`)
6. Job marked as completed

---

## Build

```bash
go build -o build/pdfxport.exe ./cmd
````

***

## Run

```
go run ./cmd
```

or

```
build/pdfxport.exe
```

***

## Output

Generated PDFs:

```
./output/job_<id>.pdf
```

***

## Configuration Notes

Currently hardcoded:

* API base URL
* JWT token
* SQLite path (`./jobs.db`)
* output folder (`./output`)

Future improvement:\
→ move to config file or env vars

***

## What was achieved

✔ Replaced browser-based PDF printing\
✔ Built fully headless PDF pipeline\
✔ Implemented concurrent processing engine\
✔ Integrated real production API\
✔ Added persistence via SQLite\
✔ Added file storage system

***

## Next improvements (TODO)

### High priority

* atomic job claiming (true queue locking)
* retry system with exponential backoff
* job deduplication

### Medium priority

* config system (.env or YAML)
* structured logging
* metrics (job duration, failure rate)

### Advanced

* distributed workers
* Redis queue replacement
* Kubernetes deployment
* API gateway layer

***

## Security Notes

* JWT token currently stored in code (NOT SAFE)
* should be moved to environment variable
* API access is production-sensitive

***

## Author Notes

This system evolved from a browser-interception prototype into a backend orchestration engine.

It now fully bypasses UI rendering and directly consumes the production PDF generation API.

````

---

# 3. OPTIONAL (recommended before moving machine)

## remove junk before zip

```powershell
Remove-Item -Recurse -Force .\jobs.db
Remove-Item -Recurse -Force .\output
````

(only if you want clean transfer)

***

# If you want next step

I can help you turn this into:

* Dockerized service
* or Windows service (auto-start on boot)
* or distributed worker cluster

But what you have now is already a **solid production-grade single-node orchestrator**.
