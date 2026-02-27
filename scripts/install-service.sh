#!/bin/bash
# Install goserve as a systemd service on Linux (e.g., Raspberry Pi).
# Usage:  curl -fsSL https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install-service.sh | sudo bash
set -e

REPO="staceyw/goserve"
BASE_URL="https://github.com/$REPO/releases/latest/download"
BIN_PATH="/usr/local/bin/goserve"
SERVICE_FILE="/etc/systemd/system/goserve.service"

# --- Must be root ------------------------------------------------------------

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: This script must be run as root (use sudo)."
  exit 1
fi

# --- Detect architecture -----------------------------------------------------

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)             echo "Error: Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY="goserve-linux-${ARCH}"

# --- Helper for reading input (works with curl | sudo bash) ------------------

prompt() {
  printf "%s" "$1"
  if [ -t 0 ]; then
    read -r REPLY
  else
    read -r REPLY < /dev/tty
  fi
}

# --- Gather configuration ----------------------------------------------------

echo ""
echo "GoServe Service Installer"
echo "========================="
echo ""

# Serve directory — default to the invoking user's home/share
REAL_USER="${SUDO_USER:-$(logname 2>/dev/null || echo root)}"
DEFAULT_DIR="/home/${REAL_USER}/share"

prompt "Directory to serve [$DEFAULT_DIR]: "
SERVE_DIR="${REPLY:-$DEFAULT_DIR}"

# Validate or create directory
if [ -d "$SERVE_DIR" ]; then
  echo "  OK: $SERVE_DIR exists."
elif [ -e "$SERVE_DIR" ]; then
  echo "Error: $SERVE_DIR exists but is not a directory."
  exit 1
else
  prompt "  $SERVE_DIR does not exist. Create it? [Y/n] "
  case "$REPLY" in
    [nN]*) echo "Aborted."; exit 1 ;;
  esac
  mkdir -p "$SERVE_DIR"
  echo "  Created: $SERVE_DIR"
fi

# Check directory is readable
if [ ! -r "$SERVE_DIR" ]; then
  echo "Error: $SERVE_DIR is not readable. Check permissions."
  exit 1
fi

# Listen address
prompt "Listen address [:8080]: "
LISTEN="${REPLY:-:8080}"

# Permission level
prompt "Permission level (readonly/readwrite/all) [all]: "
PERMLEVEL="${REPLY:-all}"
case "$PERMLEVEL" in
  readonly|readwrite|all) ;;
  *) echo "Error: Invalid permission level: $PERMLEVEL"; exit 1 ;;
esac

# Check directory is writable if uploads enabled
if [ "$PERMLEVEL" != "readonly" ] && [ ! -w "$SERVE_DIR" ]; then
  echo "Error: $SERVE_DIR is not writable but permlevel=$PERMLEVEL requires write access."
  exit 1
fi

# Run-as user
DETECTED_USER=$(stat -c '%U' "$SERVE_DIR" 2>/dev/null || stat -f '%Su' "$SERVE_DIR" 2>/dev/null || echo "$REAL_USER")
prompt "Run service as user [$DETECTED_USER]: "
RUN_USER="${REPLY:-$DETECTED_USER}"

if ! id "$RUN_USER" >/dev/null 2>&1; then
  echo "Error: User $RUN_USER does not exist."
  exit 1
fi

# --- Confirm ------------------------------------------------------------------

echo ""
echo "Configuration:"
echo "  Binary:      $BIN_PATH"
echo "  Directory:   $SERVE_DIR"
echo "  Listen:      $LISTEN"
echo "  Permissions: $PERMLEVEL"
echo "  Run as:      $RUN_USER"
echo ""
prompt "Install and start service? [Y/n] "
case "$REPLY" in
  [nN]*) echo "Aborted."; exit 0 ;;
esac

# --- Download binary ----------------------------------------------------------

echo ""
echo "Downloading $BINARY ..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$BIN_PATH" "${BASE_URL}/${BINARY}"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$BIN_PATH" "${BASE_URL}/${BINARY}"
else
  echo "Error: Neither curl nor wget found."
  exit 1
fi
chmod +x "$BIN_PATH"
echo "  Installed: $BIN_PATH"

# --- Create systemd service ---------------------------------------------------

echo "Creating systemd service ..."
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=GoServe - HTTP File Server
After=network.target

[Service]
Type=simple
User=$RUN_USER
ExecStart=$BIN_PATH -dir "$SERVE_DIR" -listen "$LISTEN" -permlevel $PERMLEVEL
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo "  Created: $SERVICE_FILE"

# --- Enable and start ---------------------------------------------------------

systemctl daemon-reload
systemctl enable goserve
systemctl start goserve

echo ""
echo "Done! GoServe is running."
echo ""
echo "  Status:   sudo systemctl status goserve"
echo "  Logs:     journalctl -u goserve -f"
echo "  Stop:     sudo systemctl stop goserve"
echo "  Restart:  sudo systemctl restart goserve"
echo ""
echo "To change settings, edit $SERVICE_FILE then:"
echo "  sudo systemctl daemon-reload && sudo systemctl restart goserve"
echo ""
echo "  http://$(hostname):${LISTEN#*:}"
echo ""
