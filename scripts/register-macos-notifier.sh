#!/bin/bash
# Registers a minimal macOS app bundle so GitHub Notifier can send notifications
# under its own identity via terminal-notifier's -sender flag.
#
# Usage: ./scripts/register-macos-notifier.sh [bundle-id]
# Default bundle ID: com.oak3.github-notifier

set -euo pipefail

BUNDLE_ID="${1:-dev.oak3.github-notifier}"
APP_NAME="GitHub Notifier"
INSTALL_DIR="$HOME/Applications"
APP_PATH="$INSTALL_DIR/GitHubNotifier.app"

echo "Registering macOS notification sender: $BUNDLE_ID"

# Create app bundle structure
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
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>CFBundleExecutable</key>
    <string>GitHubNotifier</string>
    <key>NSUserNotificationAlertStyle</key>
    <string>alert</string>
</dict>
</plist>
EOF

# Write stub executable
cat > "$APP_PATH/Contents/MacOS/GitHubNotifier" << 'EOF'
#!/bin/bash
# Stub executable — exists only to satisfy macOS app bundle requirements
EOF
chmod +x "$APP_PATH/Contents/MacOS/GitHubNotifier"

# Ad-hoc sign (no Apple Developer account required)
codesign --sign - --force "$APP_PATH"

# Launch once to register with Notification Center
open "$APP_PATH"

echo ""
echo "Done. Add the following to ~/.github-notifier.conf:"
echo ""
echo "  MACOS_NOTIFICATION_SENDER=$BUNDLE_ID"
echo ""
echo "Note: you may need to grant notification permissions in:"
echo "  System Settings > Notifications > $APP_NAME"
