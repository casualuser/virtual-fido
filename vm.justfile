# Justfile for Guest VM (macOS)
set shell := ["bash", "-c"]

# Path validation
PROJECT_DIR := shell("pwd")
PYTHON := "python3"

# install python dependencies
deps:
    {{PYTHON}} -m pip install playwright
    {{PYTHON}} -m playwright install chromium

# --- DKIT FLOW (DriverKit) ---

# --- DKIT FLOW (DriverKit) ---

dkit_cleanup:
    echo "Cleaning up DKIT..."
    if [ -n "$SUDO_PASSWORD" ]; then python3 -c "import os; open('/tmp/ask.py', 'w').write('#!/usr/bin/env python3\nprint(' + repr(os.environ['SUDO_PASSWORD']) + ')\n')" && chmod +x /tmp/ask.py && SUDO_ASKPASS=/tmp/ask.py sudo -A pkill -9 -f "virtual-fido-dkit" || true; else sudo pkill -9 -f "virtual-fido-dkit" || true; fi

dkit_test: dkit_cleanup deps dkit_verify

dkit_verify:
    chmod +x {{PROJECT_DIR}}/virtual-fido-dkit
    echo "Running DKIT E2E Test..."
    cd {{PROJECT_DIR}} && {{PYTHON}} -u scripts/e2e_orch.py --binary ./virtual-fido-dkit --log /tmp/virtual-fido-dkit.log --signal /tmp/fido_approve_dkit

dkit_insert args="":
    @echo "Inserting Virtual FIDO Key (DKIT)..."
    if [ -n "$SUDO_PASSWORD" ]; then python3 -c "import os; open('/tmp/ask.py', 'w').write('#!/usr/bin/env python3\nprint(' + repr(os.environ['SUDO_PASSWORD']) + ')\n')" && chmod +x /tmp/ask.py && SUDO_ASKPASS=/tmp/ask.py sudo -A pkill -9 -f "virtual-fido-dkit" || true; else sudo pkill -9 -f "virtual-fido-dkit" || true; fi
    if [ -n "$SUDO_PASSWORD" ]; then SUDO_ASKPASS=/tmp/ask.py sudo -A rm -f /tmp/virtual-fido-dkit.log; else sudo rm -f /tmp/virtual-fido-dkit.log; fi
    echo "--- DKIT LOG START: $(date) ---" | (if [ -n "$SUDO_PASSWORD" ]; then SUDO_ASKPASS=/tmp/ask.py sudo -A tee /tmp/virtual-fido-dkit.log > /dev/null; else sudo tee /tmp/virtual-fido-dkit.log > /dev/null; fi)
    chmod +x {{PROJECT_DIR}}/virtual-fido-dkit
    [ -f /tmp/seed ] || (head -c 32 /dev/urandom | xxd -p | tr -d '\n' > /tmp/seed)
    cd {{PROJECT_DIR}} && (if [ -n "$SUDO_PASSWORD" ]; then SUDO_ASKPASS=/tmp/ask.py sudo -A bash -c "nohup ./virtual-fido-dkit run --seed-file /tmp/seed --transport darwin --signal-file /tmp/fido_approve_dkit {{args}} >> /tmp/virtual-fido-dkit.log 2>&1 &"; else sudo bash -c "nohup ./virtual-fido-dkit run --seed-file /tmp/seed --transport darwin --signal-file /tmp/fido_approve_dkit {{args}} >> /tmp/virtual-fido-dkit.log 2>&1 &"; fi)
    @echo "DKIT Key inserted. Logs at /tmp/virtual-fido-dkit.log"

dkit_auto:
    just -f vm.justfile dkit_insert "--always-approve --auto-select"

dkit_activate:
    if [ -n "$SUDO_PASSWORD" ]; then python3 -c "import os; open('/tmp/ask.py', 'w').write('#!/usr/bin/env python3\nprint(' + repr(os.environ['SUDO_PASSWORD']) + ')\n')" && chmod +x /tmp/ask.py && SUDO_ASKPASS=/tmp/ask.py sudo -A touch /tmp/fido_approve_dkit; else sudo touch /tmp/fido_approve_dkit; fi
    @echo "Sent DKIT activation signal."

dkit_remove:
    if [ -n "$SUDO_PASSWORD" ]; then python3 -c "import os; open('/tmp/ask.py', 'w').write('#!/usr/bin/env python3\nprint(' + repr(os.environ['SUDO_PASSWORD']) + ')\n')" && chmod +x /tmp/ask.py && SUDO_ASKPASS=/tmp/ask.py sudo -A pkill -9 -f "virtual-fido-dkit" || true; else sudo pkill -9 -f "virtual-fido-dkit" || true; fi

dkit_logs:
    tail -f /tmp/virtual-fido-dkit.log

# --- VHID FLOW (Virtual HID) ---

vhid_cleanup:
    echo "Cleaning up VHID..."
    if [ -n "$SUDO_PASSWORD" ]; then python3 -c "import os; open('/tmp/ask.py', 'w').write('#!/usr/bin/env python3\nprint(' + repr(os.environ['SUDO_PASSWORD']) + ')\n')" && chmod +x /tmp/ask.py && SUDO_ASKPASS=/tmp/ask.py sudo -A pkill -9 -f "virtual-fido-vhid" || true; else sudo pkill -9 -f "virtual-fido-vhid" || true; fi

vhid_test: vhid_cleanup deps vhid_verify

vhid_verify:
    chmod +x {{PROJECT_DIR}}/virtual-fido-vhid
    echo "Running VHID E2E Test..."
    cd {{PROJECT_DIR}} && {{PYTHON}} -u scripts/e2e_orch.py --binary ./virtual-fido-vhid --log /tmp/virtual-fido-vhid.log --signal /tmp/fido_approve_vhid

vhid_insert args="":
    @echo "Inserting Virtual FIDO Key (VHID)..."
    if [ -n "$SUDO_PASSWORD" ]; then python3 -c "import os; open('/tmp/ask.py', 'w').write('#!/usr/bin/env python3\nprint(' + repr(os.environ['SUDO_PASSWORD']) + ')\n')" && chmod +x /tmp/ask.py && SUDO_ASKPASS=/tmp/ask.py sudo -A pkill -9 -f "virtual-fido-vhid" || true; else sudo pkill -9 -f "virtual-fido-vhid" || true; fi
    if [ -n "$SUDO_PASSWORD" ]; then SUDO_ASKPASS=/tmp/ask.py sudo -A rm -f /tmp/virtual-fido-vhid.log; else sudo rm -f /tmp/virtual-fido-vhid.log; fi
    echo "--- VHID LOG START: $(date) ---" | (if [ -n "$SUDO_PASSWORD" ]; then SUDO_ASKPASS=/tmp/ask.py sudo -A tee /tmp/virtual-fido-vhid.log > /dev/null; else sudo tee /tmp/virtual-fido-vhid.log > /dev/null; fi)
    chmod +x {{PROJECT_DIR}}/virtual-fido-vhid
    [ -f /tmp/seed ] || (head -c 32 /dev/urandom | xxd -p | tr -d '\n' > /tmp/seed)
    cd {{PROJECT_DIR}} && (if [ -n "$SUDO_PASSWORD" ]; then SUDO_ASKPASS=/tmp/ask.py sudo -A bash -c "nohup ./virtual-fido-vhid run --seed-file /tmp/seed --transport darwin --signal-file /tmp/fido_approve_vhid {{args}} >> /tmp/virtual-fido-vhid.log 2>&1 &"; else sudo bash -c "nohup ./virtual-fido-vhid run --seed-file /tmp/seed --transport darwin --signal-file /tmp/fido_approve_vhid {{args}} >> /tmp/virtual-fido-vhid.log 2>&1 &"; fi)
    @echo "VHID Key inserted. Logs at /tmp/virtual-fido-vhid.log"

vhid_auto:
    just -f vm.justfile vhid_insert "--always-approve --auto-select"

vhid_activate:
    if [ -n "$SUDO_PASSWORD" ]; then python3 -c "import os; open('/tmp/ask.py', 'w').write('#!/usr/bin/env python3\nprint(' + repr(os.environ['SUDO_PASSWORD']) + ')\n')" && chmod +x /tmp/ask.py && SUDO_ASKPASS=/tmp/ask.py sudo -A touch /tmp/fido_approve_vhid; else sudo touch /tmp/fido_approve_vhid; fi
    @echo "Sent VHID activation signal."

vhid_remove:
    if [ -n "$SUDO_PASSWORD" ]; then python3 -c "import os; open('/tmp/ask.py', 'w').write('#!/usr/bin/env python3\nprint(' + repr(os.environ['SUDO_PASSWORD']) + ')\n')" && chmod +x /tmp/ask.py && SUDO_ASKPASS=/tmp/ask.py sudo -A pkill -9 -f "virtual-fido-vhid" || true; else sudo pkill -9 -f "virtual-fido-vhid" || true; fi

vhid_logs:
    tail -f /tmp/virtual-fido-vhid.log

# Legacy/Default alias (default to dkit for now)
insert args="": (dkit_insert args)
remove: dkit_remove
auto: dkit_auto
cleanup: dkit_cleanup vhid_cleanup
