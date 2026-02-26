# Build script for GoServe
# Always run from the repo root, regardless of where this script is invoked.
Push-Location (Split-Path $PSScriptRoot -Parent)
Write-Host "Building GoServe for multiple platforms..." -ForegroundColor Cyan

# Windows 64-bit
Write-Host "`nBuilding Windows (amd64)..." -ForegroundColor Yellow
go build -ldflags="-s -w" -o goserve-windows-amd64.exe main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "  goserve-windows-amd64.exe" -ForegroundColor Green
}

# Linux 64-bit
Write-Host "`nBuilding Linux (amd64)..." -ForegroundColor Yellow
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -ldflags="-s -w" -o goserve-linux-amd64 main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "  goserve-linux-amd64" -ForegroundColor Green
}

# Linux ARM64 (Raspberry Pi, etc.)
Write-Host "`nBuilding Linux (arm64)..." -ForegroundColor Yellow
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -ldflags="-s -w" -o goserve-linux-arm64 main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "  goserve-linux-arm64" -ForegroundColor Green
}

# macOS ARM64 (Apple Silicon)
Write-Host "`nBuilding macOS (arm64)..." -ForegroundColor Yellow
$env:GOOS="darwin"; $env:GOARCH="arm64"; go build -ldflags="-s -w" -o goserve-darwin-arm64 main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "  goserve-darwin-arm64" -ForegroundColor Green
}

# Reset to Windows
$env:GOOS="windows"; $env:GOARCH="amd64"

Pop-Location
Write-Host "`nBuild complete!" -ForegroundColor Green
