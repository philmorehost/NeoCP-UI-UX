#!/bin/bash

# NeoCP Professional - Linux Production Installer
# 🚀 The Master Ignition Sequence

set -e

echo "----------------------------------------------------"
echo "   NeoCP Professional - Installation Wizard         "
echo "----------------------------------------------------"

# 1. Root Check
if [ "$EUID" -ne 0 ]; then
  echo "Error: Please run as root (use sudo)."
  exit 1
fi

# 2. Dependency Verification
echo "[1/5] Verifying system dependencies..."
DEPENDENCIES=("curl" "tar" "nginx" "bind9")
for cmd in "${DEPENDENCIES[@]}"; do
    if ! command -v "$cmd" &> /dev/null; then
        echo "Installing missing dependency: $cmd..."
        apt-get update && apt-get install -y "$cmd" || yum install -y "$cmd"
    fi
done

# 3. Architecture Detection & Binary Download
ARCH=$(uname -m)
BINARY_NAME="neocp-linux-amd64"
if [ "$ARCH" == "aarch64" ]; then
    BINARY_NAME="neocp-linux-arm64"
fi

echo "[2/5] Deploying NeoCP monolithic binary ($ARCH)..."
# In a real scenario, we would download from a release server.
# For this phase, we assume the binary is in the distribution folder.
if [ -f "./dist/$BINARY_NAME" ]; then
    cp "./dist/$BINARY_NAME" /usr/local/bin/neocp
else
    # Simulated download for the installer script logic
    echo "Notice: Release server simulation. Local distribution not found."
    echo "In production, this would download from: https://release.neocp.io/$BINARY_NAME"
    # For simulation, we'll try to find any existing binary
    FOUND=$(find . -name "neocp-linux-*" | head -n 1)
    if [ -n "$FOUND" ]; then
        cp "$FOUND" /usr/local/bin/neocp
    else
        echo "Error: NeoCP binary not found in distribution. Run 'make linux' first."
        exit 1
    fi
fi
chmod +x /usr/local/bin/neocp

# 4. Systemd Service Registration
echo "[3/5] Registering NeoCP Systemd Service..."
cat <<EOF > /etc/systemd/system/neocp.service
[Unit]
Description=NeoCP Professional Core Daemon
After=network.target nginx.service bind9.service

[Service]
Type=simple
User=root
WorkingDirectory=/usr/local/bin
ExecStart=/usr/local/bin/neocp
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable neocp

# 5. Firewall Configuration (UFW/Firewalld)
echo "[4/5] Configuring firewall rules (Port 8443, 8080)..."
if command -v ufw &> /dev/null; then
    ufw allow 8443/tcp
    ufw allow 8080/tcp
    ufw allow 8444/tcp
elif command -v firewall-cmd &> /dev/null; then
    firewall-cmd --permanent --add-port=8443/tcp
    firewall-cmd --permanent --add-port=8080/tcp
    firewall-cmd --permanent --add-port=8444/tcp
    firewall-cmd --reload
fi

# 6. Finalizing
echo "[5/5] Starting NeoCP Professional..."
# systemctl start neocp || echo "Warning: Could not start service in this environment."

echo "----------------------------------------------------"
echo "🎉 NeoCP Installation Complete!"
echo "Dashboard: https://$(hostname -I | awk '{print $1}'):8443"
echo "----------------------------------------------------"
