param(
    [string]$CodesFile = "M:\dev\cpt\PDFXport\codes.txt",
    [string]$PdfDir = "M:\dev\cpt\PDFXport\pdfxport-orchestrator\output"
)

# Collect PDF base names (without extension) into a HashSet for fast lookup
$pdfSet = New-Object 'System.Collections.Generic.HashSet[string]'
Get-ChildItem -LiteralPath $PdfDir -Filter *.pdf | ForEach-Object { $pdfSet.Add($_.BaseName) } | Out-Null

# Read the tab-delimited file, update the "Oui" column
$lines = Get-Content -LiteralPath $CodesFile
$header = $lines[0]
$updated = @($header)

for ($i = 1; $i -lt $lines.Count; $i++) {
    $cols = $lines[$i] -split "`t"
    $order = $cols[1].Trim()
    $oui = if ($pdfSet.Contains($order)) { "Y" } else { "" }
    $updated += "$oui`t$order"
}

$updated -join "`r`n" | Set-Content -LiteralPath $CodesFile -Encoding Default
Write-Host "✅ codes.txt mis à jour : $($updated.Count - 1) codes vérifiés"
