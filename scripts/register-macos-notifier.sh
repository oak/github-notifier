#!/bin/bash
# Wraps the github-notifier binary in a proper .app bundle so that macOS
# UNUserNotificationCenter accepts it as a notification sender and
# click-to-open works correctly.
#
# Usage: ./scripts/register-macos-notifier.sh [bundle-id]
# Default bundle ID: dev.oak3.github-notifier

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUNDLE_ID="${1:-dev.oak3.github-notifier}"
APP_NAME="GitHub Notifier"
INSTALL_DIR="$HOME/Applications"
APP_PATH="$INSTALL_DIR/GitHubNotifier.app"
BINARY_SRC="$REPO_ROOT/github-notifier"
BINARY_DEST="$APP_PATH/Contents/MacOS/github-notifier"

# Build the binary if not present
if [ ! -f "$BINARY_SRC" ]; then
    echo "Building github-notifier..."
    make -C "$REPO_ROOT" build
fi

echo "Creating .app bundle at $APP_PATH..."
mkdir -p "$APP_PATH/Contents/MacOS"
mkdir -p "$APP_PATH/Contents/Resources"

# Write Info.plist
cat > "$APP_PATH/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key>
    <string>$BUNDLE_ID</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleDisplayName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>CFBundleExecutable</key>
    <string>github-notifier</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSUserNotificationAlertStyle</key>
    <string>alert</string>
</dict>
</plist>
EOF

# Copy the real binary into the bundle (remove any stale executables first)
rm -f "$APP_PATH/Contents/MacOS/"*
cp "$BINARY_SRC" "$BINARY_DEST"

# Sign the binary first, then the bundle
codesign --sign - --force "$BINARY_DEST"
codesign --sign - --force "$APP_PATH"

echo ""
echo "Done. Launch the app with:"
echo ""
echo "  open ~/Applications/GitHubNotifier.app"
echo ""
echo "Or add it to Login Items in System Settings > General > Login Items."
echo ""
echo "Note: you may need to grant notification permissions in:"
echo "  System Settings > Notifications > $APP_NAME"
