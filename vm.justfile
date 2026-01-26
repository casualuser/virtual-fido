# Justfile for Guest VM (macOS)
set shell := ["bash", "-c"]

# Path validation
PROJECT_DIR := shell("pwd")
SCRIPT_DIR := PROJECT_DIR + "/scripts"

# install python dependencies
deps:
    /Users/amo/.pyenv/versions/3.12.12/bin/python3.12 -m pip install playwright
    /Users/amo/.pyenv/versions/3.12.12/bin/python3.12 -m playwright install chromium

# cleanup environment
cleanup:
    echo "Cleaning up..."
    sudo pkill -9 -f "virtual-fido-macos" || true

# Full test flow on VM
test: cleanup deps verify

# run the e2e python test
verify:
    chmod +x {{PROJECT_DIR}}/virtual-fido-macos
    echo "Running Python E2E Test (HID Transport)..."
    # We run from project dir so scripts can find relative assets if needed
    cd {{PROJECT_DIR}} && /Users/amo/.pyenv/versions/3.12.12/bin/python3.12 -u scripts/e2e_orch.py --transport hid

# Manual Key management
insert:
    @echo "Inserting Virtual FIDO Key (HID)..."
    sudo pkill -9 -f "virtual-fido-macos" || true
    # Clear old logs and write start marker
    sudo rm -f /tmp/virtual-fido.log
    echo "--- LOG START: $(date) ---" | sudo tee /tmp/virtual-fido.log > /dev/null
    chmod +x {{PROJECT_DIR}}/virtual-fido-macos
    # Create a random seed if it doesn't exist (required for key persistence)
    [ -f /tmp/seed ] || (head -c 32 /dev/urandom | xxd -p | tr -d '\n' > /tmp/seed)
    # Run with HID transport flag and auto-approve for seamless testing
    cd {{PROJECT_DIR}} && sudo bash -c "nohup ./virtual-fido-macos run --seed-file /tmp/seed --transport darwin --auto-approve >> /tmp/virtual-fido.log 2>&1 &"
    @echo "Key inserted. Logs at /tmp/virtual-fido.log"

remove:
    @echo "Removing Virtual FIDO Key..."
    sudo pkill -9 -f "virtual-fido-macos" || true
    @echo "Key removed."
