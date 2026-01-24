# Justfile for Guest VM (macOS)
set shell := ["bash", "-c"]

# Path validation
PROJECT_DIR := shell("pwd")
APP_ZIP := PROJECT_DIR + "/mac/output/USBDriverInstaller.app.zip"
SCRIPT_DIR := PROJECT_DIR + "/scripts"

# install python dependencies

# install python dependencies
deps:
    /Users/amo/.pyenv/versions/3.12.12/bin/python3.12 -m pip install playwright
    /Users/amo/.pyenv/versions/3.12.12/bin/python3.12 -m playwright install chromium

# cleanup environment
cleanup:
    echo "Cleaning up..."
    sudo killall virtual-fido-macos || true
    sudo killall USBDriverInstaller || true

# deploy app to /Applications (required for dext loading)
deploy:
    echo "Deploying to /Applications..."
    sudo rm -rf /Applications/USBDriverInstaller.app
    # Unzip directly into /Applications
    sudo unzip -q {{APP_ZIP}} -d /Applications/

# uninstall the driver
uninstall:
    echo "Uninstalling Driver..."
    sudo killall osascript || true
    nohup osascript {{SCRIPT_DIR}}/handle_auth.scpt > /dev/null 2>&1 &
    # Use perl to timeout after 20s if GUI hangs
    LC_ALL=C perl -e 'alarm 20; exec @ARGV' /Applications/USBDriverInstaller.app/Contents/MacOS/USBDriverInstaller --uninstall || true

# install the driver
install:
    echo "Installing Driver..."
    sudo killall osascript || true
    nohup osascript {{SCRIPT_DIR}}/handle_auth.scpt > /dev/null 2>&1 &
    LC_ALL=C perl -e 'alarm 20; exec @ARGV' /Applications/USBDriverInstaller.app/Contents/MacOS/USBDriverInstaller --install || true

# Full test flow on VM
test: cleanup deps deploy uninstall install-wait verify


install-wait: install
    @echo "Settling for 5s..."
    sleep 5

# run the e2e python test
verify:
    chmod +x {{PROJECT_DIR}}/virtual-fido-macos
    echo "Running Python E2E Test..."
    # We run from project dir so scripts can find relative assets if needed
    cd {{PROJECT_DIR}} && /Users/amo/.pyenv/versions/3.12.12/bin/python3.12 -u scripts/e2e_orch.py


# Reinstall driver
reinstall: deploy uninstall install-wait

# Manual Key management
insert:
    @echo "Inserting Virtual FIDO Key..."
    sudo killall virtual-fido-macos || true
    # Clear old logs and write start marker
    sudo rm -f /tmp/virtual-fido.log
    echo "--- LOG START: $(date) ---" | sudo tee /tmp/virtual-fido.log > /dev/null
    chmod +x {{PROJECT_DIR}}/virtual-fido-macos
    # Use seed file and auto-approve to skip prompts
    # We use sudo bash -c to ensure the redirection happens with root permissions
    cd {{PROJECT_DIR}} && sudo bash -c "nohup ./virtual-fido-macos run --seed-file test.seed --auto-approve >> /tmp/virtual-fido.log 2>&1 &"
    @echo "Key inserted. Logs at /tmp/virtual-fido.log"






remove:
    @echo "Removing Virtual FIDO Key..."
    sudo killall virtual-fido-macos || true
    @echo "Key removed."

# Run standalone connection test tool
test-conn:
    @echo "Building connection test tool..."
    clang -framework IOKit -framework CoreFoundation mac/USBDriver/USBDrverLib/test_conn.c -o /tmp/test_conn
    codesign --force --sign - --entitlements mac/entitlements.plist /tmp/test_conn
    @echo "Running connection test tool..."
    /tmp/test_conn
