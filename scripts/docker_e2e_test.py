#!/usr/bin/env python3
"""
Docker-based E2E test for virtual-fido on host machine.
Tests the UHID transport using Docker container and Playwright.
"""
import os
import sys
import time
import signal
import subprocess
import argparse
from pathlib import Path

class DockerFIDOTest:
    def __init__(self, seed_file="/tmp/seed", signal_file="/tmp/fido_approve", log_file="/tmp/virtual-fido-docker.log"):
        self.seed_file = seed_file
        self.signal_file = signal_file
        self.log_file = log_file
        self.container_name = "virtual-fido-test"
        self.docker_proc = None
        
    def cleanup(self):
        """Stop and remove any existing container."""
        print("Cleaning up existing containers...")
        subprocess.run(["docker", "stop", self.container_name], 
                      capture_output=True, check=False)
        subprocess.run(["docker", "rm", self.container_name], 
                      capture_output=True, check=False)
        
        # Clean up signal file
        if os.path.exists(self.signal_file):
            os.remove(self.signal_file)
    
    def generate_seed(self):
        """Generate a random seed if it doesn't exist."""
        if not os.path.exists(self.seed_file):
            print(f"Generating seed at {self.seed_file}...")
            result = subprocess.run(
                ["docker", "run", "--rm", "virtual-fido:latest", "gen-seed"],
                capture_output=True, text=True, check=True
            )
            seed = result.stdout.strip()
            with open(self.seed_file, 'w') as f:
                f.write(seed)
            print(f"Seed generated: {seed[:16]}...")
        else:
            print(f"Using existing seed from {self.seed_file}")
    
    def start_container(self, auto_approve=False, auto_select=False):
        """Start the Docker container with virtual-fido."""
        print("Starting virtual-fido Docker container...")
        
        # Build command
        cmd = [
            "docker", "run",
            "--name", self.container_name,
            "--privileged",  # Required for UHID access
            "--device=/dev/uhid",  # Mount UHID device
            "-v", f"{self.seed_file}:/tmp/seed:ro",  # Mount seed file
            "-v", f"{self.signal_file}:/tmp/fido_approve",  # Mount signal file
            "virtual-fido:latest",
            "run",
            "--seed-file", "/tmp/seed",
            "--transport", "uhid",
            "--signal-file", "/tmp/fido_approve"
        ]
        
        if auto_approve:
            cmd.append("--always-approve")
        if auto_select:
            cmd.append("--auto-select")
        
        # Start container in background
        print(f"Running: {' '.join(cmd)}")
        with open(self.log_file, 'w') as log:
            log.write(f"=== Docker FIDO Test Log - {time.strftime('%Y-%m-%d %H:%M:%S')} ===\n")
            self.docker_proc = subprocess.Popen(
                cmd,
                stdout=log,
                stderr=subprocess.STDOUT
            )
        
        # Wait for container to be ready
        print("Waiting for container to initialize...")
        time.sleep(3)
        
        # Check if container is running
        result = subprocess.run(
            ["docker", "ps", "--filter", f"name={self.container_name}", "--format", "{{.Names}}"],
            capture_output=True, text=True
        )
        
        if self.container_name not in result.stdout:
            print("ERROR: Container failed to start!")
            self.show_logs()
            return False
        
        print("Container started successfully!")
        return True
    
    def send_approval(self):
        """Send approval signal by touching the signal file."""
        print(f"Sending approval signal via {self.signal_file}...")
        Path(self.signal_file).touch()
    
    def show_logs(self):
        """Display container logs."""
        print("\n=== Container Logs ===")
        if os.path.exists(self.log_file):
            with open(self.log_file, 'r') as f:
                print(f.read())
        else:
            subprocess.run(["docker", "logs", self.container_name])
    
    def run_playwright_test(self, site="http://localhost:8000", username="test@example.com"):
        """Run Playwright E2E test."""
        print(f"\nRunning Playwright test against {site}...")
        
        # Ensure videos directory exists
        os.makedirs("videos", exist_ok=True)
        
        # Run the test
        result = subprocess.run(
            [sys.executable, "test/playwright_fido.py", 
             "--site", site, 
             "--username", username],
            check=False
        )
        
        return result.returncode == 0
    
    def stop_container(self):
        """Stop and remove the container."""
        print("\nStopping container...")
        subprocess.run(["docker", "stop", self.container_name], 
                      capture_output=True, check=False)
        subprocess.run(["docker", "rm", self.container_name], 
                      capture_output=True, check=False)
        
        if self.docker_proc:
            try:
                self.docker_proc.terminate()
                self.docker_proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.docker_proc.kill()

def main():
    parser = argparse.ArgumentParser(description="Docker-based E2E test for virtual-fido")
    parser.add_argument("--site", default="http://localhost:8000", 
                       help="WebAuthn test site URL")
    parser.add_argument("--username", default="test@example.com",
                       help="Username for test")
    parser.add_argument("--auto-approve", action="store_true",
                       help="Auto-approve all FIDO requests")
    parser.add_argument("--auto-select", action="store_true",
                       help="Auto-select first identity")
    parser.add_argument("--seed-file", default="/tmp/seed",
                       help="Path to seed file")
    parser.add_argument("--signal-file", default="/tmp/fido_approve",
                       help="Path to signal file")
    parser.add_argument("--log-file", default="/tmp/virtual-fido-docker.log",
                       help="Path to log file")
    
    args = parser.parse_args()
    
    test = DockerFIDOTest(
        seed_file=args.seed_file,
        signal_file=args.signal_file,
        log_file=args.log_file
    )
    
    # Setup signal handler for cleanup
    def signal_handler(sig, frame):
        print("\nInterrupted! Cleaning up...")
        test.stop_container()
        test.cleanup()
        sys.exit(1)
    
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    try:
        # Cleanup any previous runs
        test.cleanup()
        
        # Generate seed
        test.generate_seed()
        
        # Start container
        if not test.start_container(
            auto_approve=args.auto_approve,
            auto_select=args.auto_select
        ):
            print("Failed to start container!")
            sys.exit(1)
        
        # Run Playwright test
        success = test.run_playwright_test(
            site=args.site,
            username=args.username
        )
        
        # Show logs
        test.show_logs()
        
        if success:
            print("\n✅ E2E test PASSED!")
            sys.exit(0)
        else:
            print("\n❌ E2E test FAILED!")
            sys.exit(1)
            
    except Exception as e:
        print(f"\n❌ Test error: {e}")
        test.show_logs()
        sys.exit(1)
    finally:
        test.stop_container()
        test.cleanup()

if __name__ == "__main__":
    main()
