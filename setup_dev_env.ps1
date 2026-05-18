#Requires -Version 5.1
<#
.SYNOPSIS
    PDFXport — Dev Environment Setup & Verification Script
.DESCRIPTION
    Checks and installs all required dev tools for PDFXport on Windows.
    Verifies each step before proceeding. Ends with a full readiness summary.
    Also handles git repository initialization and sync.

.NOTES
    Run as Administrator for tool installations.
    Usage: .\setup_dev_env.ps1
           .\setup_dev_env.ps1 -ProjectPath "D:\myprojects\PDFXport"
           .\setup_dev_env.ps1 -SkipInstall   # check only, no installs
#>

param(
    [string]$ProjectPath = $PSScriptRoot,
    [switch]$SkipInstall
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ─────────────────────────────────────────────
#  CONSTANTS
# ─────────────────────────────────────────────
$REPO_URL      = "https://github.com/maatallah/pdfxport"
$BRANCH        = "workspace-sync"
$GO_MIN        = [Version]"1.22.0"   # minimum acceptable Go version
$ORCH_DIR      = Join-Path $ProjectPath "pdfxport-orchestrator"
$RESULTS       = @()   # collect final check results

# ─────────────────────────────────────────────
#  HELPERS
# ─────────────────────────────────────────────
function Write-Header([string]$text) {
    Write-Host ""
    Write-Host ("=" * 60) -ForegroundColor Cyan
    Write-Host "  $text" -ForegroundColor Cyan
    Write-Host ("=" * 60) -ForegroundColor Cyan
}

function Write-Step([string]$text) {
    Write-Host ""
    Write-Host ">> $text" -ForegroundColor Yellow
}

function Pass([string]$label, [string]$detail = "") {
    $msg = if ($detail) { "$label ($detail)" } else { $label }
    Write-Host "  [PASS] $msg" -ForegroundColor Green
    $script:RESULTS += [PSCustomObject]@{ Status="PASS"; Item=$label; Detail=$detail }
}

function Fail([string]$label, [string]$detail = "") {
    $msg = if ($detail) { "$label — $detail" } else { $label }
    Write-Host "  [FAIL] $msg" -ForegroundColor Red
    $script:RESULTS += [PSCustomObject]@{ Status="FAIL"; Item=$label; Detail=$detail }
}

function Warn([string]$label, [string]$detail = "") {
    $msg = if ($detail) { "$label — $detail" } else { $label }
    Write-Host "  [WARN] $msg" -ForegroundColor DarkYellow
    $script:RESULTS += [PSCustomObject]@{ Status="WARN"; Item=$label; Detail=$detail }
}

function Abort([string]$reason) {
    Write-Host ""
    Write-Host "ABORTED: $reason" -ForegroundColor Red
    Write-Host "Fix the issue above and re-run the script." -ForegroundColor Red
    exit 1
}

function CommandExists([string]$cmd) {
    return [bool](Get-Command $cmd -ErrorAction SilentlyContinue)
}

function Get-CommandVersion([string]$cmd, [string]$args) {
    try {
        $out = & $cmd $args.Split(" ") 2>&1 | Select-Object -First 1
        return "$out".Trim()
    } catch { return $null }
}

function Invoke-Winget([string]$id) {
    if ($SkipInstall) { return $false }
    if (-not (CommandExists "winget")) { return $false }
    Write-Host "  Installing $id via winget..." -ForegroundColor DarkCyan
    winget install --id $id --silent --accept-package-agreements --accept-source-agreements
    # Refresh PATH for this session
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" +
                [System.Environment]::GetEnvironmentVariable("Path","User")
    return $true
}

# ─────────────────────────────────────────────
#  BANNER
# ─────────────────────────────────────────────
Clear-Host
Write-Host @"

  ██████╗ ██████╗ ███████╗██╗  ██╗██████╗  ██████╗ ██████╗ ████████╗
  ██╔══██╗██╔══██╗██╔════╝╚██╗██╔╝██╔══██╗██╔═══██╗██╔══██╗╚══██╔══╝
  ██████╔╝██║  ██║█████╗   ╚███╔╝ ██████╔╝██║   ██║██████╔╝   ██║
  ██╔═══╝ ██║  ██║██╔══╝   ██╔██╗ ██╔═══╝ ██║   ██║██╔══██╗   ██║
  ██║     ██████╔╝██║     ██╔╝ ██╗██║     ╚██████╔╝██║  ██║   ██║
  ╚═╝     ╚═════╝ ╚═╝     ╚═╝  ╚═╝╚═╝      ╚═════╝ ╚═╝  ╚═╝   ╚═╝

  Dev Environment Setup — PDFXport
  Project path : $ProjectPath
  Repository   : $REPO_URL
  Branch       : $BRANCH

"@ -ForegroundColor DarkCyan

# ─────────────────────────────────────────────
#  STEP 1 — WINGET (package manager)
# ─────────────────────────────────────────────
Write-Header "STEP 1 — Package Manager (winget)"
Write-Step "Checking winget..."

if (CommandExists "winget") {
    $wv = Get-CommandVersion "winget" "--version"
    Pass "winget" $wv
} else {
    Warn "winget not found" "Install App Installer from Microsoft Store, or install tools manually"
}

# ─────────────────────────────────────────────
#  STEP 2 — GIT
# ─────────────────────────────────────────────
Write-Header "STEP 2 — Git"
Write-Step "Checking git..."

if (-not (CommandExists "git")) {
    Write-Host "  git not found. Attempting install..." -ForegroundColor DarkYellow
    $installed = Invoke-Winget "Git.Git"
    if (-not $installed) {
        Fail "git" "Not installed. Download from https://git-scm.com/download/win"
        Abort "git is required for all repository operations."
    }
}

if (CommandExists "git") {
    $gv = Get-CommandVersion "git" "--version"
    Pass "git" $gv

    # Configure git identity if not set
    $userName  = git config --global user.name  2>$null
    $userEmail = git config --global user.email 2>$null
    if (-not $userName) {
        Warn "git user.name not configured" "Run: git config --global user.name 'Your Name'"
    } else { Pass "git user.name" $userName }
    if (-not $userEmail) {
        Warn "git user.email not configured" "Run: git config --global user.email 'you@example.com'"
    } else { Pass "git user.email" $userEmail }
} else {
    Fail "git" "Still not found after install attempt"
    Abort "git is required."
}

# ─────────────────────────────────────────────
#  STEP 3 — GO
# ─────────────────────────────────────────────
Write-Header "STEP 3 — Go Language"
Write-Step "Checking Go installation..."

if (-not (CommandExists "go")) {
    Write-Host "  Go not found. Attempting install..." -ForegroundColor DarkYellow
    $installed = Invoke-Winget "GoLang.Go"
    if (-not $installed) {
        Fail "Go" "Not installed. Download from https://go.dev/dl/"
        Abort "Go is required to build the orchestrator."
    }
}

if (CommandExists "go") {
    $goVerStr = (go version) -replace "go version go",""
    $goVerNum = ($goVerStr -split " ")[0]
    try {
        $goVerParsed = [Version]$goVerNum
        if ($goVerParsed -ge $GO_MIN) {
            Pass "Go" "go$goVerNum (>= $GO_MIN required)"
        } else {
            Fail "Go version" "Found $goVerNum, need >= $GO_MIN"
            Abort "Please upgrade Go from https://go.dev/dl/"
        }
    } catch {
        Warn "Go version parse failed" $goVerStr
    }
    # Check GOPATH
    $gopath = go env GOPATH
    Pass "GOPATH" $gopath
} else {
    Fail "Go" "Still not found after install attempt"
    Abort "Go is required."
}

# ─────────────────────────────────────────────
#  STEP 4 — GCC / MinGW-W64  (CGO for go-sqlite3)
# ─────────────────────────────────────────────
Write-Header "STEP 4 — GCC / MinGW-W64 (CGO for go-sqlite3)"
Write-Step "Checking gcc..."

if (-not (CommandExists "gcc")) {
    Write-Host "  gcc not found. Attempting install via winget (MinGW-W64)..." -ForegroundColor DarkYellow
    $installed = Invoke-Winget "Msys2.Msys2"
    if (-not $installed) {
        Fail "gcc" "Not installed. Options:"
        Write-Host "    A) winget install --id MSYS2.MSYS2" -ForegroundColor Gray
        Write-Host "    B) winget install --id StrawberryPerl.StrawberryPerl  (bundles gcc)" -ForegroundColor Gray
        Write-Host "    C) https://github.com/niXman/mingw-builds-binaries/releases" -ForegroundColor Gray
        Abort "GCC is required for CGO (go-sqlite3 and systray packages)."
    }
}

if (CommandExists "gcc") {
    $gccVer = Get-CommandVersion "gcc" "--version"
    Pass "gcc (MinGW)" $gccVer
} else {
    Fail "gcc" "Not found — go-sqlite3 and systray will not compile without CGO"
    Abort "Install MinGW-W64 and add its bin folder to PATH."
}

# Verify CGO is actually usable
Write-Step "Verifying CGO is enabled..."
$cgoEnabled = go env CGO_ENABLED
if ($cgoEnabled -eq "1") {
    Pass "CGO_ENABLED" "1"
} else {
    Warn "CGO_ENABLED" "Currently $cgoEnabled — sqlite3 and systray require CGO=1"
    Write-Host "    Run: `$env:CGO_ENABLED='1'" -ForegroundColor Gray
}

# ─────────────────────────────────────────────
#  STEP 5 — NODE / NPM  (optional, browser extension tooling)
# ─────────────────────────────────────────────
Write-Header "STEP 5 — Node.js / npm (optional)"
Write-Step "Checking Node.js..."

if (CommandExists "node") {
    $nv = Get-CommandVersion "node" "--version"
    Pass "Node.js" $nv
    if (CommandExists "npm") {
        $npmv = Get-CommandVersion "npm" "--version"
        Pass "npm" $npmv
    } else {
        Warn "npm not found" "Usually bundled with Node.js"
    }
} else {
    Warn "Node.js not installed" "Only needed for browser extension tooling — not required for orchestrator"
    if (-not $SkipInstall) { Invoke-Winget "OpenJS.NodeJS.LTS" | Out-Null }
}

# ─────────────────────────────────────────────
#  STEP 6 — PROJECT FOLDER
# ─────────────────────────────────────────────
Write-Header "STEP 6 — Project Folder"
Write-Step "Checking project path: $ProjectPath"

if (-not (Test-Path $ProjectPath)) {
    Write-Host "  Project folder does not exist. Creating..." -ForegroundColor DarkYellow
    New-Item -ItemType Directory -Path $ProjectPath -Force | Out-Null
    Pass "Project folder created" $ProjectPath
} else {
    Pass "Project folder exists" $ProjectPath
}

# ─────────────────────────────────────────────
#  STEP 7 — GIT REPOSITORY
# ─────────────────────────────────────────────
Write-Header "STEP 7 — Git Repository"
Write-Step "Checking git repository state..."

Push-Location $ProjectPath

$isGitRepo = Test-Path (Join-Path $ProjectPath ".git")

if (-not $isGitRepo) {
    # Check if folder is empty enough to clone into
    $items = Get-ChildItem $ProjectPath -Force
    if ($items.Count -eq 0) {
        Write-Host "  Empty folder — cloning repository..." -ForegroundColor DarkCyan
        git clone $REPO_URL . --branch $BRANCH
        if ($LASTEXITCODE -ne 0) {
            Fail "git clone" "Failed to clone $REPO_URL"
            Abort "Check your internet connection and GitHub access."
        }
        Pass "Repository cloned" "$REPO_URL → $ProjectPath"
    } else {
        Write-Host "  Non-empty folder, no .git — initializing and linking remote..." -ForegroundColor DarkYellow
        git init
        git remote add origin $REPO_URL
        git fetch origin
        git checkout -b $BRANCH --track "origin/$BRANCH" 2>$null
        if ($LASTEXITCODE -ne 0) {
            git checkout $BRANCH 2>$null
        }
        Pass "Repository initialized" "Remote linked to $REPO_URL"
        Warn "Existing files" "Review with 'git status' — some files may be untracked"
    }
}

# ─────────────────────────────────────────────
#  STEP 8 — REPO STATUS & PULL
# ─────────────────────────────────────────────
Write-Header "STEP 8 — Repository Status & Sync"
Write-Step "Fetching latest from origin..."

git fetch origin 2>&1 | Out-Null

$currentBranch = git rev-parse --abbrev-ref HEAD 2>$null
Pass "Current branch" $currentBranch

# Check for uncommitted local changes
$statusLines = git status --porcelain 2>$null
if ($statusLines) {
    Warn "Uncommitted local changes detected" "Review with 'git status' before pulling"
    Write-Host ""
    Write-Host "  Local changes:" -ForegroundColor Gray
    $statusLines | ForEach-Object { Write-Host "    $_" -ForegroundColor Gray }
    Write-Host ""
    Write-Host "  Options:" -ForegroundColor Gray
    Write-Host "    Stash   : git stash" -ForegroundColor Gray
    Write-Host "    Discard : git restore ." -ForegroundColor Gray
    Write-Host "    Commit  : git add . && git commit -m 'WIP'" -ForegroundColor Gray
    Write-Host ""
    $answer = Read-Host "  Pull anyway? This is SAFE only if there are no conflicts (y/N)"
    if ($answer -ne "y" -and $answer -ne "Y") {
        Warn "Pull skipped" "Handle local changes manually then run: git pull origin $BRANCH"
    } else {
        git pull origin $BRANCH
        if ($LASTEXITCODE -eq 0) { Pass "git pull" "Merged from origin/$BRANCH" }
        else { Fail "git pull" "Merge conflict — resolve manually" }
    }
} else {
    # Check if behind remote
    $behind = git rev-list "HEAD..origin/$BRANCH" --count 2>$null
    if ($behind -and [int]$behind -gt 0) {
        Write-Host "  Branch is $behind commit(s) behind origin — pulling..." -ForegroundColor DarkCyan
        git pull origin $BRANCH
        if ($LASTEXITCODE -eq 0) { Pass "git pull" "$behind commit(s) pulled from origin/$BRANCH" }
        else { Fail "git pull" "Pull failed — check output above" }
    } else {
        Pass "Repository" "Up to date with origin/$BRANCH"
    }
}

Pop-Location

# ─────────────────────────────────────────────
#  STEP 8b — GIT SUBMODULE (pdf/) & CUSTOM PATCH
# ─────────────────────────────────────────────
Write-Header "STEP 8b — Git Submodule (pdf/ — ledongthuc/pdf fork)"
Write-Step "Checking pdf/ submodule..."

$submoduleDir = Join-Path $ProjectPath "pdf"
Push-Location $ProjectPath

# 1. Initialize submodule if missing
if (-not (Test-Path (Join-Path $submoduleDir ".git"))) {
    Write-Host "  pdf/ submodule not initialized — running git submodule update..." -ForegroundColor DarkCyan
    git submodule update --init --recursive 2>&1
    if ($LASTEXITCODE -eq 0) {
        Pass "git submodule" "pdf/ initialized"
    } else {
        Fail "git submodule" "Failed — run manually: git submodule update --init --recursive"
    }
} else {
    Pass "git submodule pdf/" "Already initialized"
}

# 2. Apply the custom PDF extraction patch automatically
Write-Step "Applying custom PDF parsing patch (pdf_page.go.patched)..."
$patchedSrc = Join-Path $ProjectPath "pdf_page.go.patched"
$patchedDst = Join-Path $submoduleDir "page.go"

if (Test-Path $patchedSrc) {
    Copy-Item -Path $patchedSrc -Destination $patchedDst -Force
    Pass "Submodule patch" "Successfully copied pdf_page.go.patched -> pdf/page.go"
} else {
    Fail "Submodule patch" "Missing pdf_page.go.patched in project root!"
}

Pop-Location


# ─────────────────────────────────────────────
#  STEP 9 — GO MODULE DEPENDENCIES
# ─────────────────────────────────────────────
Write-Header "STEP 9 — Go Module Dependencies"
Write-Step "Running go mod download in orchestrator..."


if (Test-Path $ORCH_DIR) {
    Push-Location $ORCH_DIR
    Write-Host "  go mod download..." -ForegroundColor DarkCyan
    go mod download 2>&1
    if ($LASTEXITCODE -eq 0) {
        Pass "go mod download" "All dependencies fetched"
    } else {
        Fail "go mod download" "See errors above"
    }

    Write-Host "  go mod verify..." -ForegroundColor DarkCyan
    go mod verify 2>&1
    if ($LASTEXITCODE -eq 0) {
        Pass "go mod verify" "Module checksums OK"
    } else {
        Fail "go mod verify" "Checksum mismatch"
    }
    Pop-Location
} else {
    Warn "pdfxport-orchestrator directory not found" "Expected at $ORCH_DIR"
}

# ─────────────────────────────────────────────
#  STEP 10 — BUILD VERIFICATION
# ─────────────────────────────────────────────
Write-Header "STEP 10 — Build Verification"

# 10a — Moissonneuse-Serveur (CGO required)
Write-Step "Attempting test build of Moissonneuse-Serveur (CGO)..."

if (Test-Path $ORCH_DIR) {
    Push-Location $ORCH_DIR
    $buildDir = Join-Path $ORCH_DIR "build"
    if (-not (Test-Path $buildDir)) { New-Item -ItemType Directory $buildDir | Out-Null }

    $env:CGO_ENABLED = "1"
    $buildOut = Join-Path $buildDir "Moissonneuse-Serveur.exe"

    Write-Host "  Building Serveur... (CGO=1, may take ~30s on first run)" -ForegroundColor DarkCyan
    go build -v -o $buildOut ./cmd/main.go 2>&1
    if ($LASTEXITCODE -eq 0 -and (Test-Path $buildOut)) {
        $size = [math]::Round((Get-Item $buildOut).Length / 1MB, 2)
        Pass "Serveur build" "build\Moissonneuse-Serveur.exe ($size MB)"
    } else {
        Fail "Serveur build failed" "Check errors above — common causes: missing gcc, CGO disabled"
    }
    Pop-Location
}

# 10b — Moissonneuse-Moulin (pure Go, no CGO)
Write-Step "Attempting test build of Moissonneuse-Moulin (pure Go)..."

$moulinBuildDir = Join-Path $ProjectPath "build"
if (-not (Test-Path $moulinBuildDir)) { New-Item -ItemType Directory $moulinBuildDir | Out-Null }
$moulinOut = Join-Path $moulinBuildDir "Moissonneuse-Moulin.exe"

push-location $ProjectPath
$env:CGO_ENABLED = "0"
Write-Host "  Building Moulin... (CGO=0, pure Go)" -ForegroundColor DarkCyan
go build -o $moulinOut . 2>&1
if ($LASTEXITCODE -eq 0 -and (Test-Path $moulinOut)) {
    $size = [math]::Round((Get-Item $moulinOut).Length / 1MB, 2)
    Pass "Moulin build" "build\Moissonneuse-Moulin.exe ($size MB)"
} else {
    Fail "Moulin build failed" "Check errors above"
}
$env:CGO_ENABLED = "1"   # restore
Pop-Location

# 10c — App/ OCR folder check
Write-Step "Checking OCR engine (App/ folder)..."
$appDir   = Join-Path $ProjectPath "App"
$naps2Exe = Join-Path $appDir "NAPS2.Console.exe"
if (Test-Path $naps2Exe) {
    Pass "NAPS2.Console.exe" "Found in App\"
} else {
    Warn "NAPS2.Console.exe missing" "Copy App\ from an existing installation for OCR support"
    Write-Host "    NAPS2 portable : https://www.naps2.com/download" -ForegroundColor Gray
    Write-Host "    Tessdata (fra) : https://github.com/tesseract-ocr/tessdata" -ForegroundColor Gray
}

# ─────────────────────────────────────────────
#  FINAL SUMMARY
# ─────────────────────────────────────────────
Write-Header "FINAL READINESS SUMMARY"

$passes = @($RESULTS | Where-Object { $_.Status -eq "PASS" })
$warns  = @($RESULTS | Where-Object { $_.Status -eq "WARN" })
$fails  = @($RESULTS | Where-Object { $_.Status -eq "FAIL" })

foreach ($r in $RESULTS) {
    $color = switch ($r.Status) {
        "PASS" { "Green" }
        "WARN" { "DarkYellow" }
        "FAIL" { "Red" }
    }
    $detail = if ($r.Detail) { " — $($r.Detail)" } else { "" }
    Write-Host "  [$($r.Status)] $($r.Item)$detail" -ForegroundColor $color
}

Write-Host ""
Write-Host ("  PASS: {0}   WARN: {1}   FAIL: {2}" -f $passes.Count, $warns.Count, $fails.Count) -ForegroundColor Cyan

if ($fails.Count -gt 0) {
    Write-Host ""
    Write-Host "  ⚠  Environment NOT ready — fix all FAIL items above." -ForegroundColor Red
    exit 1
} elseif ($warns.Count -gt 0) {
    Write-Host ""
    Write-Host "  ✔  Environment mostly ready — review WARN items above." -ForegroundColor DarkYellow
    exit 0
} else {
    Write-Host ""
    Write-Host "  ✔  Environment fully ready. Happy harvesting! 🌾" -ForegroundColor Green
    exit 0
}
