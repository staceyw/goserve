# Dev build script for goserve
Push-Location (Split-Path $PSScriptRoot -Parent)
Write-Host "Building goserve..." -ForegroundColor Cyan
go build -ldflags="-s -w" -o goserve.exe main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "  goserve.exe" -ForegroundColor Green
} else {
    Write-Host "  Build failed!" -ForegroundColor Red
}
Pop-Location
