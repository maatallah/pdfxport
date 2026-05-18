# Master PDFXport Icon Changer & Builder Script
# Easily swap or bring your own custom system tray icon for Moissonneuse-Serveur.
#
# Usage:
#   .\change_icon.ps1 <1 | 2 | 3>
#   .\change_icon.ps1 "C:\path\to\my_custom_icon.png"

param (
    [Parameter(Position = 0)]
    [string]$Selection
)

$ProjectPath = Get-Item -Path . | Select-Object -ExpandProperty FullName
$OrchDir     = Join-Path $ProjectPath "pdfxport-orchestrator"
$IconGoFile  = Join-Path $OrchDir "internal/utils/icon.go"
$TempIco     = Join-Path $ProjectPath "icon_custom.ico"

# Helper function to generate high-contrast white circular backplated .ico from a PNG/JPG
function Generate-ContrastIco($imagePath, $icoPath) {
    Add-Type -AssemblyName System.Drawing
    
    $orig = [System.Drawing.Image]::FromFile($imagePath)
    $bmp = New-Object System.Drawing.Bitmap(16, 16)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    
    $g.Clear([System.Drawing.Color]::Transparent)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    
    # Draw premium solid white circular backplate
    $brush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::White)
    $g.FillEllipse($brush, 0, 0, 16, 16)
    
    # Scale and center the original image inside with a neat 1px border framing
    $g.DrawImage($orig, 1, 1, 14, 14)
    
    # Save as PNG to capture alpha-transparency
    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $pngBytes = $ms.ToArray()
    $ms.Close()
    
    # Construct standard ICO header (22 bytes wrapping the PNG)
    $icoHeader = New-Object byte[] 22
    $icoHeader[0] = 0x00; $icoHeader[1] = 0x00
    $icoHeader[2] = 0x01; $icoHeader[3] = 0x00
    $icoHeader[4] = 0x01; $icoHeader[5] = 0x00
    $icoHeader[6] = 16
    $icoHeader[7] = 16
    $icoHeader[8] = 0x00
    $icoHeader[9] = 0x00
    $icoHeader[10] = 0x01; $icoHeader[11] = 0x00
    $icoHeader[12] = 0x20; $icoHeader[13] = 0x00 # 32 BPP
    
    $pngSize = $pngBytes.Length
    $icoHeader[14] = [byte]($pngSize -band 0xFF)
    $icoHeader[15] = [byte](($pngSize -shr 8) -band 0xFF)
    $icoHeader[16] = [byte](($pngSize -shr 16) -band 0xFF)
    $icoHeader[17] = [byte](($pngSize -shr 24) -band 0xFF)
    $icoHeader[18] = 0x16; $icoHeader[19] = 0x00; $icoHeader[20] = 0x00; $icoHeader[21] = 0x00
    
    $fs = New-Object System.IO.FileStream($icoPath, [System.IO.FileMode]::Create)
    $fs.Write($icoHeader, 0, $icoHeader.Length)
    $fs.Write($pngBytes, 0, $pngBytes.Length)
    $fs.Close()
    
    $orig.Dispose()
    $bmp.Dispose()
    $g.Dispose()
    $brush.Dispose()
}

# If no selection is provided, show the interactive menu
if ($null -eq $Selection -or $Selection -eq "") {
    Write-Host "=============================================" -ForegroundColor Cyan
    Write-Host "      🌾 PDFXport - Change System Tray Icon 🌾" -ForegroundColor Cyan
    Write-Host "=============================================" -ForegroundColor Cyan
    Write-Host "Select an icon option to apply:" -ForegroundColor Gray
    Write-Host " [1] Icon 1 (High-contrast Green/Gold Leaf)" -ForegroundColor Yellow
    Write-Host " [2] Icon 2 (High-contrast Sleek Gear/Harvester)" -ForegroundColor Yellow
    Write-Host " [3] Icon 3 (High-contrast Retro Farm Tractor)" -ForegroundColor Yellow
    Write-Host " [4] Custom Image (Enter path to your own PNG or JPG)" -ForegroundColor Yellow
    Write-Host ""
    
    $choice = Read-Host "Enter icon choice (1, 2, 3, or 4)"
    if ($choice -match '^[1-3]$') {
        $Selection = $choice
    } elseif ($choice -eq "4") {
        $Selection = Read-Host "Enter absolute or relative path to your PNG/JPG image"
    } else {
        Write-Host "[ERROR] Invalid choice. Exiting." -ForegroundColor Red
        exit 1
    }
}

$FinalIco = ""

# Check if selection is a direct number (pre-built icons)
if ($Selection -match '^[1-3]$') {
    $FinalIco = Join-Path $ProjectPath "icon_$Selection.ico"
    if (-not (Test-Path $FinalIco)) {
        Write-Host "[ERROR] Pre-built Icon asset '$FinalIco' not found!" -ForegroundColor Red
        exit 1
    }
} else {
    # It's a custom path! Verify if it exists
    $ResolvedPath = Resolve-Path $Selection -ErrorAction SilentlyContinue
    if (-not $ResolvedPath) {
        # Try as relative path from current directory
        $ResolvedPath = Join-Path $ProjectPath $Selection
        if (-not (Test-Path $ResolvedPath)) {
            Write-Host "[ERROR] Custom image file '$Selection' not found!" -ForegroundColor Red
            exit 1
        }
    } else {
        $ResolvedPath = $ResolvedPath.Path
    }
    
    # Check extension
    $ext = [System.IO.Path]::GetExtension($ResolvedPath).ToLower()
    if ($ext -eq ".ico") {
        Write-Host "   -> Custom file is already a .ico. Using directly." -ForegroundColor Yellow
        $FinalIco = $ResolvedPath
    } elseif ($ext -eq ".png" -or $ext -eq ".jpg" -or $ext -eq ".jpeg") {
        Write-Host "   -> Custom image detected ($ext). Auto-converting to transparent backplated ICO..." -ForegroundColor Yellow
        Generate-ContrastIco $ResolvedPath $TempIco
        $FinalIco = $TempIco
    } else {
        Write-Host "[ERROR] Unsupported file format '$ext'. Only .png, .jpg, .jpeg, or .ico files are supported!" -ForegroundColor Red
        exit 1
    }
}

Write-Host "---------------------------------------------" -ForegroundColor Gray
Write-Host "Applying Icon..." -ForegroundColor Cyan

# 1. Read bytes and generate Go file
$bytes = [System.IO.File]::ReadAllBytes($FinalIco)
$hex = $bytes | ForEach-Object { "0x{0:x2}" -f $_ }
$goCode = "package utils`r`n`r`nvar IconData = []byte{ " + ($hex -join ", ") + " }`r`n"
[System.IO.File]::WriteAllText($IconGoFile, $goCode)
Write-Host "   -> Embedded chosen icon into icon.go successfully." -ForegroundColor Green

# Clean up temp file if created
if ($FinalIco -eq $TempIco -and (Test-Path $TempIco)) {
    Remove-Item $TempIco -Force | Out-Null
}

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
    Write-Host "[ERROR] Compilation failed! New icon was not applied." -ForegroundColor Red
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
Write-Host "🎉 SUCCESS: Your chosen icon is now fully applied and running!" -ForegroundColor Green
Write-Host "=============================================" -ForegroundColor Green
