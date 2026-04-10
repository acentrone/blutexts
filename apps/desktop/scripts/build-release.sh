#!/bin/bash
# Build, sign, and package BlueSend desktop app
set -e

SIGN_IDENTITY="Apple Development: Anthony Centrone (LK82794QKP)"
APP_PATH="build/bin/bluesend.app"

echo "Building universal binary..."
export PATH="$HOME/go/bin:$PATH"
wails build -platform darwin/universal

echo "Code signing with persistent identity..."
codesign --deep --force --verify --verbose \
  --sign "$SIGN_IDENTITY" \
  --options runtime \
  "$APP_PATH"

echo "Verifying signature..."
codesign -dv "$APP_PATH" 2>&1 | grep "Authority"

echo "Creating DMG..."
rm -f BlueSend.dmg
create-dmg \
  --volname "BlueSend" \
  --window-pos 200 120 \
  --window-size 600 400 \
  --icon-size 100 \
  --icon "bluesend.app" 150 185 \
  --app-drop-link 450 185 \
  --hide-extension "bluesend.app" \
  "BlueSend.dmg" \
  "$APP_PATH"

echo "Done! BlueSend.dmg is ready."
