# Docker E2E Testing for Virtual FIDO

This directory contains end-to-end tests for the Docker-based virtual-fido distribution.

## Prerequisites

1. **Docker** must be installed and running
2. **Python 3** with Playwright installed:
   ```bash
   pip install playwright
   playwright install chromium
   ```
3. **UHID kernel module** must be available (Linux only):
   ```bash
   # Check if UHID is available
   ls /dev/uhid
   
   # If not available, load the module
   sudo modprobe uhid
   ```

## Quick Start

### Run E2E test with local server:
```bash
# Terminal 1: Start the test server
just test_server

# Terminal 2: Run the E2E test
just docker_e2e
```

### Run E2E test with webauthn.io:
```bash
just docker_e2e_webauthn
```

### Run E2E test with custom site:
```bash
just docker_e2e_custom https://demo.yubico.com/webauthn-technical/registration
```

## Manual Testing

### 1. Build the Docker image:
```bash
just docker_build
```

### 2. Generate a seed:
```bash
docker run --rm virtual-fido:latest gen-seed > /tmp/seed
```

### 3. Run the container with UHID:
```bash
docker run -it --rm \
  --privileged \
  --device=/dev/uhid \
  -v /tmp/seed:/tmp/seed:ro \
  -v /tmp/fido_approve:/tmp/fido_approve \
  virtual-fido:latest \
  run \
  --seed-file /tmp/seed \
  --transport uhid \
  --signal-file /tmp/fido_approve \
  --always-approve \
  --auto-select
```

### 4. In another terminal, run Playwright test:
```bash
python3 test/playwright_fido.py \
  --site http://localhost:8000 \
  --username test@example.com
```

## Test Architecture

The Docker E2E test (`scripts/docker_e2e_test.py`) orchestrates:

1. **Container Management**: Starts/stops the virtual-fido Docker container
2. **UHID Transport**: Uses Linux UHID for HID device emulation
3. **Signal-based Approval**: Uses file-based signaling for user approval
4. **Playwright Integration**: Runs browser-based WebAuthn tests

## Troubleshooting

### Container fails to start
- Check Docker logs: `docker logs virtual-fido-test`
- Ensure `/dev/uhid` exists: `ls -l /dev/uhid`
- Try loading UHID module: `sudo modprobe uhid`

### UHID permission denied
- Run with `--privileged` flag
- Or add specific capability: `--cap-add=SYS_ADMIN`

### Test timeouts
- Check container logs: `cat /tmp/virtual-fido-docker.log`
- Verify the test site is accessible
- Ensure Playwright is properly installed

## Supported Test Sites

- **Local server**: `http://localhost:8000` (via `test/server.py`)
- **webauthn.io**: `https://webauthn.io`
- **Yubico Demo**: `https://demo.yubico.com/webauthn-technical/registration`

## Notes

- **Linux-Only for Host Integration**: Docker tests currently support **Linux hosts only** if you want the host browser to see the virtual key. This is because the `uhid` transport creates a virtual device in the Linux kernel. On macOS, Docker runs in a VM, meaning the virtual device stays inside the VM and is **not visible to the macOS host**.
- **macOS Testing**: For macOS testing (where you want the host browser to see the device), use the VM-based tests with the **native `vhid` or `dkit` transports**.
- **Privileged Mode**: The container runs in privileged mode to access `/dev/uhid` inside the Docker VM's Linux kernel.
