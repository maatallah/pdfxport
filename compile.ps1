# Master PDFXport Compilation Script
# Compiles both Serveur (Orchestrator, CGO=1) and Moulin (Parser, CGO=0) into the 'release/' folder automatically.
# Usage: .\compile.ps1

$ProjectPath = Get-Item -LiteralPath . | Select-Object -ExpandProperty FullName
$ReleaseDir  = Join-Path $ProjectPath "release"

# Ensure release directory exists
if (-not (Test-Path $ReleaseDir)) {
    New-Item -ItemType Directory -LiteralPath $ReleaseDir -Force | Out-Null
}

Write-Host "PDFXport Compilation in progress..." -ForegroundColor Cyan
Write-Host "---------------------------------------------" -ForegroundColor Gray

# 1. Compile Moissonneuse-Serveur (Orchestrator)
Write-Host "1. Compilation: Moissonneuse-Serveur (Serveur)..." -ForegroundColor Yellow
$OrchDir = Join-Path $ProjectPath "pdfxport-orchestrator"
if (Test-Path $OrchDir) {
    Push-Location $OrchDir
    $env:CGO_ENABLED = "1"
    $OutPath = Join-Path $ReleaseDir "Moissonneuse-Serveur.exe"
    
    # -ldflags="-H=windowsgui" hides the console window on launch since it has a systray GUI
    go build -ldflags="-H=windowsgui" -o $OutPath ./cmd/main.go 2>&1
    
    if ($LASTEXITCODE -eq 0 -and (Test-Path $OutPath)) {
        $size = [math]::Round((Get-Item $OutPath).Length / 1MB, 2)
        Write-Host "   -> Moissonneuse-Serveur compiled successfully! ($size MB)" -ForegroundColor Green
        Write-Host "      Location: release\Moissonneuse-Serveur.exe" -ForegroundColor Gray
    } else {
        Write-Host "   [ERROR] Moissonneuse-Serveur compilation failed!" -ForegroundColor Red
        Pop-Location
        exit 1
    }
    Pop-Location
} else {
    Write-Host "   [ERROR] Folder 'pdfxport-orchestrator' not found!" -ForegroundColor Red
    exit 1
}

Write-Host "---------------------------------------------" -ForegroundColor Gray

# 2. Compile Moissonneuse-Moulin (Parser)
Write-Host "2. Compilation: Moissonneuse-Moulin (Moulin)..." -ForegroundColor Yellow
Push-Location $ProjectPath
$env:CGO_ENABLED = "0"
$OutPath = Join-Path $ReleaseDir "Moissonneuse-Moulin.exe"

go build -o $OutPath . 2>&1

if ($LASTEXITCODE -eq 0 -and (Test-Path $OutPath)) {
    $size = [math]::Round((Get-Item $OutPath).Length / 1MB, 2)
    Write-Host "   -> Moissonneuse-Moulin compiled successfully! ($size MB)" -ForegroundColor Green
    Write-Host "      Location: release\Moissonneuse-Moulin.exe" -ForegroundColor Gray
} else {
    Write-Host "   [ERROR] Moissonneuse-Moulin compilation failed!" -ForegroundColor Red
    Pop-Location
    exit 1
}
Pop-Location

Write-Host "---------------------------------------------" -ForegroundColor Gray
Write-Host "Compilation complete! Both servers are ready in 'release/' folder." -ForegroundColor Green
