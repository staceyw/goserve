# Build script for GoServe
Write-Host "Building GoServe for multiple platforms..." -ForegroundColor Cyan

# Windows 64-bit
Write-Host "`nBuilding Windows (amd64)..." -ForegroundColor Yellow
go build -o goserve-windows-amd64.exe main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ go-server-windows-amd64.exe" -ForegroundColor Green
}

# Linux 64-bit
Write-Host "`nBuilding Linux (amd64)..." -ForegroundColor Yellow
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o goserve-linux-amd64 main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ go-server-linux-amd64" -ForegroundColor Green
}

# Linux ARM64 (Raspberry Pi, etc.)
Write-Host "`nBuilding Linux (arm64)..." -ForegroundColor Yellow
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o goserve-linux-arm64 main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ go-server-linux-arm64" -ForegroundColor Green
}

Write-Host "`n✓ Build complete!" -ForegroundColor Green
