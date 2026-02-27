# goserve installer for Windows (PowerShell).
# Usage:  irm https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.ps1 | iex
#   - or -
# Usage:  iex (irm https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.ps1)

$ErrorActionPreference = "Stop"

$Repo = "staceyw/goserve"
$InstallDir = $PWD.Path
$BaseURL = "https://github.com/$Repo/releases/latest/download"

# --- Detect architecture ---------------------------------------------------

$Arch = if ($env:PROCESSOR_IDENTIFIER -match "ARM") {
    "arm64"
} elseif ([Environment]::Is64BitOperatingSystem) {
    "amd64"
} else {
    Write-Host "Error: 32-bit Windows is not supported." -ForegroundColor Red
    return
}

$Binary = "goserve-windows-${Arch}.exe"

# --- Pre-flight checks -----------------------------------------------------

Write-Host ""
Write-Host "GoServe Installer" -ForegroundColor Cyan
Write-Host "==================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Arch:       windows/$Arch"
Write-Host "  Binary:     $Binary"
Write-Host "  Install to: $InstallDir\"
Write-Host ""
Write-Host "This will install into ${InstallDir}\:"
Write-Host "  - goserve.exe  (binary)"
Write-Host "  - readme.txt"
Write-Host ""

# Prompt for confirmation
$answer = Read-Host "Continue? [Y/n]"
if ($answer -match "^[nN]") {
    Write-Host "Aborted."
    return
}

# --- Download ---------------------------------------------------------------

function Download-File($url, $dest) {
    $name = Split-Path $dest -Leaf
    Write-Host "  Downloading $name ..."
    try {
        Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
    } catch {
        Write-Host "Error downloading ${name}: $_" -ForegroundColor Red
        throw
    }
}

Write-Host ""
Download-File "$BaseURL/$Binary" (Join-Path $InstallDir "goserve.exe")

# --- Generate readme.txt ---------------------------------------------------

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
  -verbose     Log every HTTP request to the console

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

Linux Service (systemd)
----------------------
Install as a background service (e.g., Raspberry Pi):

  curl -fsSL https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install-service.sh | sudo bash

The script prompts for directory, port, and permissions, then sets up a
systemd service that starts on boot. After install, manage with:

  sudo systemctl status goserve       # check status
  sudo systemctl stop goserve         # stop
  sudo systemctl restart goserve      # restart
  journalctl -u goserve -f            # view logs

To change startup flags, edit /etc/systemd/system/goserve.service then:
  sudo systemctl daemon-reload && sudo systemctl restart goserve

More Info
---------
  https://github.com/staceyw/goserve
"@ | Set-Content -Path (Join-Path $InstallDir "readme.txt") -Encoding UTF8
Write-Host "  readme.txt"

# --- Done -------------------------------------------------------------------

Write-Host ""
Write-Host "Installed to $InstallDir\" -ForegroundColor Green
Write-Host ""
Write-Host "Quick start:"
Write-Host "  1) .\goserve.exe"
Write-Host "  2) Open http://localhost:8080 in your browser."
Write-Host ""
Write-Host "Enable uploads and file management:"
Write-Host "  .\goserve.exe -permlevel all"
Write-Host ""
