import subprocess
import os
import signal
import sys
import time
import threading
import argparse

# This script runs ENTIRELY on the Guest VM.
# It supports both dkit and vhid flows.

def read_stream(stream, prefix):
    for line in iter(stream.readline, ''):
        print(f"{prefix}{line.strip()}")
        sys.stdout.flush()
    stream.close()

def main():
    parser = argparse.ArgumentParser(description="E2E Orchestrator for Virtual FIDO")
    parser.add_argument("--binary", default="./virtual-fido-macos", help="Path to virtual-fido binary")
    parser.add_argument("--log", default="/tmp/virtual-fido.log", help="Path to log file")
    parser.add_argument("--signal", default="/tmp/fido_approve", help="Path to approval signal file")
    args = parser.parse_args()

    print(f"Starting E2E Test with binary: {args.binary}")
    
    sudo_pass = os.environ.get("SUDO_PASSWORD", "")
    if not sudo_pass:
        print("WARNING: SUDO_PASSWORD not set. Sudo commands may fail.")

    askpass_path = "/tmp/fido_askpass.py"
    with open(askpass_path, "w") as f:
        f.write("#!/usr/bin/env python3\n")
        f.write(f"print({repr(sudo_pass)})\n")
    os.chmod(askpass_path, 0o755)
    
    os.environ["SUDO_ASKPASS"] = askpass_path
    os.environ["DISPLAY"] = ":0"

    def run_sudo(cmd_args):
        full_cmd = ["sudo", "-A"] + cmd_args
        p = subprocess.run(full_cmd, capture_output=True, text=True)
        if p.returncode != 0:
            print(f"Sudo Error ({cmd_args}): {p.stderr}")
        return p.returncode

    # 0. Cleanup existing processes
    binary_name = os.path.basename(args.binary)
    print(f"Cleanup: Killing existing {binary_name} processes...")
    run_sudo(["killall", binary_name])
    time.sleep(1)

    # 1. Start Virtual FIDO
    seed_path = "/tmp/seed"
    if not os.path.exists(seed_path):
        with open(seed_path, "w") as f:
            f.write("00"*32)

    # We use 'run' command with natural flow flags for automated testing
    cmd = [
        "sudo", "-A",
        args.binary, "run",
        "--device-name", "HHhhh",
        "--seed-file", seed_path,
        "--always-approve",
        "--auto-select"
    ]

    # Make sure binary is executable
    subprocess.run(["chmod", "+x", args.binary])
    
    vf_proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT, 
        text=True,
        bufsize=1
    )
    
    # Background Reader Thread for VF Output
    t_vf = threading.Thread(target=read_stream, args=(vf_proc.stdout, f"[{binary_name}] "))
    t_vf.daemon = True
    t_vf.start()

    # Wait for driver initialization
    print("Waiting 10s for driver to initialize...")
    time.sleep(10)

    # Check if process is still alive
    if vf_proc.poll() is not None:
        print(f"ERROR: {binary_name} exited immediately with code {vf_proc.returncode}")
        return

    # --- 2. Run Playwright Test ---
    print("Starting Playwright Test...")
    pw_cmd = [
        "python3", "-u", "test/playwright_fido.py"
    ]
    
    pw_proc = subprocess.Popen(
        pw_cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1
    )

    for line in iter(pw_proc.stdout.readline, ''):
        print(f"PLAYWRIGHT: {line.strip()}")
        sys.stdout.flush()
    
    pw_proc.wait()
    print("Test Finished. Cleaning up...")
    
    # --- 3. Cleanup ---
    run_sudo(["kill", str(vf_proc.pid)])
    try:
        vf_proc.terminate()
    except:
        pass

if __name__ == "__main__":
    main()
