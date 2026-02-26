# Cross-compile binaries and create a GitHub release.
# Usage:  .\scripts\release.ps1 v1.4.0
#         .\scripts\release.ps1 v1.4.0 -DryRun   # build only, no upload

param(
    [Parameter(Position=0)]
    [string]$Tag,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

if (-not $Tag) {
    $latest = (gh release list --limit 1 --json tagName 2>$null | ConvertFrom-Json)
    if ($latest -and $latest.Count -gt 0) {
        Write-Host "Latest release: $($latest[0].tagName)"
    }
    $Tag = Read-Host "Enter version tag (e.g. v1.4.0)"
    if (-not $Tag) { throw "Version tag is required" }
}
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$dist = Join-Path $root "dist"

if (Test-Path $dist) { Remove-Item $dist -Recurse -Force }
New-Item -ItemType Directory -Path $dist | Out-Null

$targets = @(
    @{ GOOS="darwin";  GOARCH="arm64"; Out="goserve-darwin-arm64" },
    @{ GOOS="linux";   GOARCH="amd64"; Out="goserve-linux-amd64" },
    @{ GOOS="linux";   GOARCH="arm64"; Out="goserve-linux-arm64" },
    @{ GOOS="windows"; GOARCH="amd64"; Out="goserve-windows-amd64.exe" },
    @{ GOOS="windows"; GOARCH="arm64"; Out="goserve-windows-arm64.exe" }
)

# Build binaries
Write-Host "Building $($targets.Count) targets ..."
Push-Location $root
try {
    foreach ($t in $targets) {
        $env:GOOS   = $t.GOOS
        $env:GOARCH = $t.GOARCH
        $out = Join-Path $dist $t.Out
        Write-Host "  $($t.GOOS)/$($t.GOARCH) -> $($t.Out)"
        go build -ldflags "-s -w" -o $out .
        if ($LASTEXITCODE -ne 0) { throw "Build failed for $($t.GOOS)/$($t.GOARCH)" }
    }
} finally {
    Remove-Item Env:\GOOS  -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
    Pop-Location
}

# Generate README.txt into dist
$readmePath = Join-Path $dist "README.txt"
@"
goserve - Lightweight HTTP File Server
======================================

Single binary, 12 themes, WebDAV support, no dependencies.

Quick Start
-----------
  goserve                        Serve current directory (read-only)
  goserve -dir /path/to/folder   Serve a specific directory
  goserve -permlevel readwrite   Enable uploads
  goserve -permlevel all         Full file management (upload/delete/rename/edit)
  goserve -listen :3000          Custom port

Then open http://localhost:8080 in your browser.

Flags
-----
  -listen      Address to listen on (default localhost:8080, repeatable)
  -dir         Directory to serve (default .)
  -permlevel   Permission level: readonly, readwrite, all (default readonly)
  -maxsize     Max upload size in MB (default 100)
  -logins      Path to authentication file
  -quiet       Suppress request logs

Permission Levels
-----------------
  readonly     Browse and view files
  readwrite    Browse, view, and upload files
  all          Browse, view, upload, delete, rename, and edit files

Authentication
--------------
  goserve -logins logins.txt

  Login file format (one user per line):
    username:password:permission

  Example:
    admin:secret:all
    user:pass123:readwrite
    guest:guest:readonly

WebDAV
------
Built-in WebDAV server at /webdav/

  Windows:  File Explorer > Map network drive > http://localhost:8080/webdav/
  macOS:    Finder > Go > Connect to Server > http://localhost:8080/webdav/
  Linux:    sudo mount -t davfs http://localhost:8080/webdav/ /mnt/goserve

More Info
---------
  https://github.com/staceyw/goserve
"@ | Set-Content -Path $readmePath -Encoding UTF8
Write-Host "  README.txt" -ForegroundColor Green

# Collect assets
$assets = @()
foreach ($t in $targets) {
    $assets += Join-Path $dist $t.Out
}
$assets += $readmePath

if ($DryRun) {
    Write-Host ""
    Write-Host "Dry run complete. Artifacts in: $dist"
    Write-Host "  Binaries: $($targets.Count)"
    Write-Host "Re-run without -DryRun to upload to GitHub."
    return
}

# Create release
Write-Host ""
Write-Host "Creating release $Tag ..."
$notes = @"
## Option 1: Install Script

Run one command to download the binary into the current directory.

**Linux / macOS:**
``````
curl -fsSL https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.sh | bash
``````

**Windows (PowerShell):**
``````
iex (irm https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.ps1)
``````

## Option 2: Manual Download

Pick your platform binary below.

| File | Description |
|------|-------------|
| ``goserve-darwin-arm64`` | macOS Apple Silicon |
| ``goserve-linux-amd64`` | Linux x64 |
| ``goserve-linux-arm64`` | Linux ARM64 / Raspberry Pi |
| ``goserve-windows-amd64.exe`` | Windows x64 |
| ``goserve-windows-arm64.exe`` | Windows ARM64 |
"@
gh release create $Tag --title "goserve $Tag" --generate-notes --notes $notes
if ($LASTEXITCODE -ne 0) { throw "gh release create failed" }

Write-Host ""
Write-Host "Uploading $($assets.Count) assets ..."
$i = 0
foreach ($a in $assets) {
    $i++
    $name = Split-Path $a -Leaf
    $size = [math]::Round((Get-Item $a).Length / 1MB, 1)
    Write-Host "  [$i/$($assets.Count)] $name (${size} MB) ..." -NoNewline
    gh release upload $Tag $a --clobber 2>$null
    if ($LASTEXITCODE -ne 0) { throw "Upload failed for $name" }
    Write-Host " done" -ForegroundColor Green
}

Write-Host ""
Write-Host "Done: https://github.com/staceyw/goserve/releases/tag/$Tag"
