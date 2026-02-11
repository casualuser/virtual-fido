export RPATH := "@executable_path/transport/vhid/output"

vhid_suite:
    echo "Cleaning up VHID..."
    pkill -9 -f "virtual-fido-vhid" || true
    # install_name_tool to ensure dylib is found if not already set (safety measure)
    install_name_tool -add_rpath @executable_path/transport/vhid/output virtual-fido-vhid || true
    ./virtual-fido-vhid &
    sleep 2
    python3 scripts/e2e_suite.py
