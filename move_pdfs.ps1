param(
    [string]$CodesFile = "M:\dev\cpt\PDFXport\codes.txt",
    [string]$PdfDir = "M:\dev\cpt\PDFXport\release\output",
    [string]$TargetDir = "M:\dev\cpt\PDFXport\in"
)

# Ensure target directory exists
New-Item -ItemType Directory -LiteralPath $TargetDir -Force | Out-Null

# Read codes, find entries with Oui=Y, move matching PDFs
$lines = Get-Content -LiteralPath $CodesFile
$moved = 0
$errors = 0

for ($i = 1; $i -lt $lines.Count; $i++) {
    $cols = $lines[$i] -split "`t"
    $oui = $cols[0].Trim()
    $order = $cols[1].Trim()

    if ($oui -ne "Y") { continue }

    $src = Join-Path $PdfDir "$order.pdf"
    $dst = Join-Path $TargetDir "$order.pdf"

    if (Test-Path -LiteralPath $src) {
        Move-Item -LiteralPath $src -Destination $dst -Force
        Write-Host "✅ Déplacé : $order.pdf"
        $moved++
    }
    else {
        Write-Host "⚠️ Introuvable : $order.pdf"
        $errors++
    }
}

Write-Host "✅ Fait : $moved déplacé(s), $errors introuvable(s)"
