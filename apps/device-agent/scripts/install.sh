#!/bin/bash
# BlueSend Device Agent Installer
# Run as: sudo bash install.sh
# Must be run on the Mac Mini that will host iMessage numbers.

set -e

AGENT_NAME="bluesend-agent"
INSTALL_DIR="/usr/local/bin"
LAUNCHD_PLIST="/Library/LaunchDaemons/io.bluesend.agent.plist"
CONFIG_DIR="/etc/bluesend"
LOG_DIR="/var/log/bluesend"

# ── Check prerequisites ───────────────────────────────────────
if [[ $(uname) != "Darwin" ]]; then
    echo "❌ This installer only runs on macOS"
    exit 1
fi

if [[ $EUID -ne 0 ]]; then
    echo "❌ Run as root: sudo bash install.sh"
    exit 1
fi

echo "📦 BlueSend Device Agent Installer"
echo "===================================="

# ── Collect configuration ────────────────────────────────────
if [[ -z "$DEVICE_TOKEN" ]]; then
    read -p "Device Token (from BlueSend admin panel): " DEVICE_TOKEN
fi
if [[ -z "$DEVICE_NAME" ]]; then
    read -p "Device Name (e.g. mac-mini-01): " DEVICE_NAME
fi
if [[ -z "$API_ENDPOINT" ]]; then
    read -p "API Endpoint [wss://devices.blutexts.com]: " API_ENDPOINT
    API_ENDPOINT="${API_ENDPOINT:-wss://devices.blutexts.com}"
fi

# ── Determine running user (non-root, logged in) ─────────────
AGENT_USER=$(who | grep console | awk '{print $1}' | head -1)
if [[ -z "$AGENT_USER" ]]; then
    read -p "macOS user to run agent as (e.g. admin): " AGENT_USER
fi
echo "Running agent as user: $AGENT_USER"

# ── Install binary ────────────────────────────────────────────
ARCH=$(uname -m)
BINARY="bluesend-agent-${ARCH}"
if [[ ! -f "$BINARY" ]]; then
    echo "⚠️  Binary $BINARY not found in current directory"
    echo "   Build with: make build-agent-mac"
    exit 1
fi

cp "$BINARY" "$INSTALL_DIR/$AGENT_NAME"
chmod +x "$INSTALL_DIR/$AGENT_NAME"
echo "✅ Binary installed to $INSTALL_DIR/$AGENT_NAME"

# ── Create config and log directories ────────────────────────
mkdir -p "$CONFIG_DIR" "$LOG_DIR"
chown "$AGENT_USER" "$LOG_DIR"

# ── Write environment config ──────────────────────────────────
cat > "$CONFIG_DIR/agent.env" << EOF
DEVICE_TOKEN=$DEVICE_TOKEN
DEVICE_NAME=$DEVICE_NAME
API_ENDPOINT=$API_ENDPOINT
LOG_LEVEL=info
EOF
chmod 600 "$CONFIG_DIR/agent.env"
echo "✅ Config written to $CONFIG_DIR/agent.env"

# ── Create launchd plist ──────────────────────────────────────
cat > "$LAUNCHD_PLIST" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.bluesend.agent</string>

    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/$AGENT_NAME</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
        <key>DEVICE_TOKEN</key>
        <string>$DEVICE_TOKEN</string>
        <key>DEVICE_NAME</key>
        <string>$DEVICE_NAME</string>
        <key>API_ENDPOINT</key>
        <string>$API_ENDPOINT</string>
    </dict>

    <key>UserName</key>
    <string>$AGENT_USER</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>ThrottleInterval</key>
    <integer>5</integer>

    <key>StandardOutPath</key>
    <string>$LOG_DIR/agent.log</string>

    <key>StandardErrorPath</key>
    <string>$LOG_DIR/agent.error.log</string>
</dict>
</plist>
EOF

chown root:wheel "$LAUNCHD_PLIST"
chmod 644 "$LAUNCHD_PLIST"
echo "✅ launchd plist created"

# ── Load the service ──────────────────────────────────────────
launchctl unload "$LAUNCHD_PLIST" 2>/dev/null || true
launchctl load -w "$LAUNCHD_PLIST"
echo "✅ Agent service loaded and started"

echo ""
echo "🚀 Installation complete!"
echo ""
echo "⚠️  IMPORTANT: Grant Full Disk Access to the agent binary:"
echo "   1. System Settings → Privacy & Security → Full Disk Access"
echo "   2. Click + and add: $INSTALL_DIR/$AGENT_NAME"
echo "   3. The agent will automatically reconnect after permission is granted"
echo ""
echo "📊 Check status: launchctl list io.bluesend.agent"
echo "📝 View logs:    tail -f $LOG_DIR/agent.log"
echo "🔄 Restart:      launchctl kickstart -k system/io.bluesend.agent"
