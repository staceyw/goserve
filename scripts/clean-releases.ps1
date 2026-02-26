# Clean up old GitHub releases, keeping only the latest.
# Usage: .\scripts\clean-releases.ps1

$ErrorActionPreference = "Stop"

$releases = gh release list --json tagName,isLatest | ConvertFrom-Json
$old = $releases | Where-Object { -not $_.isLatest }

if ($old.Count -eq 0) {
    Write-Host "No old releases to clean up." -ForegroundColor Green
    return
}

Write-Host "Keeping: $($releases | Where-Object { $_.isLatest } | ForEach-Object { $_.tagName })" -ForegroundColor Cyan
Write-Host "Deleting $($old.Count) old release(s):`n"

foreach ($r in $old) {
    Write-Host "  $($r.tagName) ..." -NoNewline
    gh release delete $r.tagName --yes --cleanup-tag 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Host " failed" -ForegroundColor Red
    } else {
        Write-Host " deleted" -ForegroundColor Green
    }
}

Write-Host "`nDone." -ForegroundColor Green
