import subprocess
import os
import signal
import sys
import time
import threading

# This script runs ENTIRELY on the Guest VM.
# It requires 'virtual-fido-macos' to be in the current directory.

def read_stream(stream, prefix):
    for line in iter(stream.readline, ''):
        print(f"{prefix}{line.strip()}")
        sys.stdout.flush()
    stream.close()

def main():
    print("Starting All-in-One E2E Test (VM Side) - Monolithic Mode...")

    # 0. Cleanup existing processes
    print("Cleanup: Killing existing virtual-fido-macos processes...")
    subprocess.run(["sudo", "killall", "virtual-fido-macos"], stderr=subprocess.DEVNULL)
    time.sleep(1)

    # 1. Start Virtual FIDO (Vault + Transport)
    seed_path = "/tmp/seed"
    if not os.path.exists(seed_path):
        with open(seed_path, "w") as f:
            f.write("00"*32)

    print("Subprocess: Starting Monolithic Virtual FIDO (Darwin)...")
    
    # We use 'run' command which links Vault <-> Transport directly
    cmd = [
        "sudo",
        "./virtual-fido-macos", "run",
        "--device-name", "HHhhh",
        "--seed-file", seed_path,
        "--auto-approve"
    ]

    
    # Make sure binary is executable
    subprocess.run(["chmod", "+x", "./virtual-fido-macos"])
    
    vf_proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT, # Merge stderr for debug logs
        text=True,
        bufsize=1
    )
    
    # Background Reader Thread for VF Output
    t_vf = threading.Thread(target=read_stream, args=(vf_proc.stdout, "[VF] "))
    t_vf.daemon = True
    t_vf.start()

    # Wait for driver initialization (heuristic)
    print("Waiting 10s for driver to initialize...")
    time.sleep(10)

    # Check if process is still alive
    if vf_proc.poll() is not None:
        print(f"ERROR: virtual-fido-macos exited immediately with code {vf_proc.returncode}")
        # Capture any immediate output
        return

    # --- 2. Run Playwright Test ---
    print("Starting Playwright Test...")
    pw_cmd = [
        "/Users/amo/.pyenv/versions/3.12.12/bin/python3.12", "tests/playwright_fido.py"
    ]
    
    # We assume playwright_fido.py is in the tests/ directory
    pw_proc = subprocess.run(pw_cmd, capture_output=True, text=True)

    print("Playwright Output:")
    print(pw_proc.stdout)
    
    if pw_proc.returncode != 0:
        print("Playwright Error:")
        print(pw_proc.stderr)
    
    print("Test Finished. Cleaning up...")
    
    # --- 3. Cleanup ---
    # Kill VF process
    subprocess.run(["sudo", "kill", str(vf_proc.pid)])
    try:
        vf_proc.terminate()
    except:
        pass

if __name__ == "__main__":
    main()
