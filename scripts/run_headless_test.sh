#!/bin/bash
set -e

# Kill any existing processes
echo "Cleaning up..."
echo 123qwe | sudo -S killall virtual-fido-macos || true
killall USBDriverInstaller || true

# Unix-style install to /Applications
echo "Deploying to /Applications..."
rm -rf /Applications/USBDriverInstaller.app
cp -r USBDriverInstaller.app /Applications/

# Uninstall Dext (ignore failure if not installed)
# Uninstall Dext (ignore failure if not installed)
echo "Uninstalling Driver..."
osascript handle_auth.scpt &
AUTH_PID=$!
/Applications/USBDriverInstaller.app/Contents/MacOS/USBDriverInstaller --uninstall || true
kill $AUTH_PID || true

# Install Dext
echo "Installing Driver..."
osascript handle_auth.scpt &
AUTH_PID=$!
/Applications/USBDriverInstaller.app/Contents/MacOS/USBDriverInstaller --install
kill $AUTH_PID || true

# Run Test
echo "Running E2E Test..."
python3 -u e2e_orch.py
