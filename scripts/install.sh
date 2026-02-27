#!/bin/sh
# goserve installer for Linux and macOS.
# Usage:  curl -fsSL https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.sh | bash
set -e

REPO="staceyw/goserve"
INSTALL_DIR="$(pwd)"
BASE_URL="https://github.com/$REPO/releases/latest/download"

# --- Detect OS and architecture -------------------------------------------

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux)  ;;
  darwin) ;;
  *)      echo "Error: Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)               echo "Error: Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY="goserve-${OS}-${ARCH}"

# --- Pre-flight checks ----------------------------------------------------

echo ""
echo "GoServe Installer"
echo "=================="
echo ""
echo "  OS/Arch:    ${OS}/${ARCH}"
echo "  Binary:     ${BINARY}"
echo "  Install to: ${INSTALL_DIR}/"
echo ""
echo "This will install into ${INSTALL_DIR}/:"
echo "  - goserve  (binary)"
echo "  - README.txt"
echo ""

# Check for curl or wget
FETCH=""
if command -v curl >/dev/null 2>&1; then
  FETCH="curl"
elif command -v wget >/dev/null 2>&1; then
  FETCH="wget"
else
  echo "Error: Neither curl nor wget found. Install one and retry."
  exit 1
fi

# Prompt for confirmation (works even when piped via curl | bash)
printf "Continue? [Y/n] "
if [ -t 0 ]; then
  read -r answer
else
  read -r answer < /dev/tty
fi
case "$answer" in
  [nN]*) echo "Aborted."; exit 0 ;;
esac

# --- Download --------------------------------------------------------------

download() {
  url="$1"
  dest="$2"
  echo "  Downloading $(basename "$dest") ..."
  if [ "$FETCH" = "curl" ]; then
    curl -fsSL -o "$dest" "$url"
  else
    wget -q -O "$dest" "$url"
  fi
}

echo ""
download "${BASE_URL}/${BINARY}" "${INSTALL_DIR}/goserve"

chmod +x "${INSTALL_DIR}/goserve"

# --- Generate README.txt --------------------------------------------------

cat > "${INSTALL_DIR}/README.txt" <<'README'
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
README
echo "  README.txt"

# --- Done ------------------------------------------------------------------

echo ""
echo "Installed to ${INSTALL_DIR}/"
echo ""
echo "Quick start:"
echo "  1) ./goserve"
echo "  2) Open http://localhost:8080 in your browser."
echo ""
echo "Enable uploads and file management:"
echo "  ./goserve -permlevel all"
echo ""
