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
echo "This will download into ${INSTALL_DIR}/:"
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
download "${BASE_URL}/README.txt" "${INSTALL_DIR}/README.txt"

chmod +x "${INSTALL_DIR}/goserve"

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
