# GoServe installer for Windows (PowerShell).
# Usage:  irm https://raw.githubusercontent.com/staceyw/GoServe/main/scripts/install.ps1 | iex
#   - or -
# Usage:  iex (irm https://raw.githubusercontent.com/staceyw/GoServe/main/scripts/install.ps1)

$ErrorActionPreference = "Stop"

$Repo = "staceyw/GoServe"
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
Write-Host "This will download into ${InstallDir}\:"
Write-Host "  - goserve.exe  (binary)"
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

# --- Done -------------------------------------------------------------------

Write-Host ""
Write-Host "Installed to $InstallDir\" -ForegroundColor Green
Write-Host ""
Write-Host "Quick start:"
Write-Host "  1) .\goserve.exe"
Write-Host "  2) Open http://localhost:8080 in your browser."
Write-Host ""
Write-Host "Enable uploads and file management:"
Write-Host "  .\goserve.exe -upload -modify"
Write-Host ""
