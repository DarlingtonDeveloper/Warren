#!/usr/bin/env bash
set -euo pipefail

BINARY="${1:-./warren}"
CONFIG_DIR="/etc/warren"
INSTALL_BIN="/usr/local/bin/warren"
SERVICE_FILE="/etc/systemd/system/warren.service"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==> Installing Warren"

# Copy binary.
if [ ! -f "$BINARY" ]; then
    echo "Error: binary not found at $BINARY"
    echo "Usage: $0 [path-to-binary]"
    exit 1
fi
cp "$BINARY" "$INSTALL_BIN"
chmod 755 "$INSTALL_BIN"
echo "    Binary installed to $INSTALL_BIN"

# Create config directory.
mkdir -p "$CONFIG_DIR"
echo "    Config directory: $CONFIG_DIR"

# Create warren user/group if they don't exist.
if ! getent group warren >/dev/null 2>&1; then
    groupadd --system warren
    echo "    Created group: warren"
fi
if ! getent passwd warren >/dev/null 2>&1; then
    useradd --system --gid warren --no-create-home --shell /usr/sbin/nologin warren
    echo "    Created user: warren"
fi

# Ensure warren user can access Docker socket.
if getent group docker >/dev/null 2>&1; then
    usermod -aG docker warren
    echo "    Added warren to docker group"
fi

# Install systemd unit.
cp "$SCRIPT_DIR/warren.service" "$SERVICE_FILE"
chmod 644 "$SERVICE_FILE"
systemctl daemon-reload
echo "    Systemd unit installed"

# Enable and start.
systemctl enable warren
systemctl start warren
echo "    Service enabled and started"

echo "==> Done. Check status with: systemctl status warren"
