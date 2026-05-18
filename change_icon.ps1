# Master PDFXport Icon Changer Script
# Allows you to easily swap the system tray icon of Moissonneuse-Serveur.
# Usage: .\change_icon.ps1 <1 | 2 | 3>

param (
    [Parameter(Position = 0)]
    [ValidateSet(1, 2, 3)]
    [int]$IconNumber
)

$ProjectPath = Get-Item -Path . | Select-Object -ExpandProperty FullName
$OrchDir     = Join-Path $ProjectPath "pdfxport-orchestrator"
$IconGoFile  = Join-Path $OrchDir "internal/utils/icon.go"

# If no parameter is provided, show menu
if ($null -eq $IconNumber -or $IconNumber -eq 0) {
    Write-Host "=============================================" -ForegroundColor Cyan
    Write-Host "      🌾 PDFXport - Change System Tray Icon 🌾" -ForegroundColor Cyan
    Write-Host "=============================================" -ForegroundColor Cyan
    Write-Host "Select an icon style to apply:" -ForegroundColor Gray
    Write-Host " [1] Icon 1 (Green/gold leaf harvester style)" -ForegroundColor Yellow
    Write-Host " [2] Icon 2 (Sleek professional harvester/gear logo)" -ForegroundColor Yellow
    Write-Host " [3] Icon 3 (Minimal high-visibility 16x16 tractor)" -ForegroundColor Yellow
    Write-Host ""
    
    $choice = Read-Host "Enter icon choice (1, 2, or 3)"
    if ($choice -match '^[1-3]$') {
        $IconNumber = [int]$choice
    } else {
        Write-Host "[ERROR] Invalid choice. Exiting." -ForegroundColor Red
        exit 1
    }
}

$IcoFile = Join-Path $ProjectPath "icon_$IconNumber.ico"
if (-not (Test-Path $IcoFile)) {
    Write-Host "[ERROR] Icon asset '$IcoFile' not found!" -ForegroundColor Red
    exit 1
}

Write-Host "---------------------------------------------" -ForegroundColor Gray
Write-Host "Applying Icon $IconNumber..." -ForegroundColor Cyan

# 1. Read bytes and generate Go file
$bytes = [System.IO.File]::ReadAllBytes($IcoFile)
$hex = $bytes | ForEach-Object { "0x{0:x2}" -f $_ }
$goCode = "package utils`r`n`r`nvar IconData = []byte{ " + ($hex -join ", ") + " }`r`n"
[System.IO.File]::WriteAllText($IconGoFile, $goCode)
Write-Host "   -> Embedded Icon $IconNumber into icon.go successfully." -ForegroundColor Green

# 2. Stop running server process if running (to prevent file lock during compilation)
$process = Get-Process -Name "Moissonneuse-Serveur" -ErrorAction SilentlyContinue
$wasRunning = $false
if ($process) {
    Write-Host "   -> Found running Moissonneuse-Serveur. Stopping process..." -ForegroundColor Yellow
    $wasRunning = $true
    $process | Stop-Process -Force
    Start-Sleep -Seconds 1
}

# 3. Trigger compilation using compile.ps1
Write-Host "   -> Triggering compilation..." -ForegroundColor Yellow
if (Test-Path "compile.ps1") {
    & .\compile.ps1
} else {
    Write-Host "[ERROR] compile.ps1 not found in root directory!" -ForegroundColor Red
    exit 1
}

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Compilation failed! Icon was not applied." -ForegroundColor Red
    exit 1
}

# 4. Restart server if it was running previously
if ($wasRunning) {
    $ServerExe = Join-Path $ProjectPath "release\Moissonneuse-Serveur.exe"
    if (Test-Path $ServerExe) {
        Write-Host "   -> Restarting Moissonneuse-Serveur in the background..." -ForegroundColor Green
        Start-Process -FilePath $ServerExe -WorkingDirectory (Join-Path $ProjectPath "release")
    }
}

Write-Host "=============================================" -ForegroundColor Green
Write-Host "🎉 SUCCESS: Icon $IconNumber is now fully applied and running!" -ForegroundColor Green
Write-Host "=============================================" -ForegroundColor Green
