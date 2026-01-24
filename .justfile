# Justfile for Virtual Fido E2E Testing
set shell := ["bash", "-c"]

# Variables
VM_USER := "amo"
VM_HOST := "192.168.64.6"
VM_DEST := "~/"
LOCAL_BUILD_DIR := "mac/output"
APP_NAME := "USBDriverInstaller.app"
VM_PROJECT := "~/virtual-fido"


# Setup environment variables for VM execution (ensure brew/pyenv paths)
export PATH := "/opt/homebrew/bin:" + env_var('PATH')

# Default target
default: test

# Sync files to VM using rsync
sync:
    ssh {{VM_USER}}@{{VM_HOST}} "mkdir -p {{VM_PROJECT}}"
    rsync -avz --delete \
        --exclude='.git' \
        --exclude='mac/USBDriver/build' \
        --exclude='mac/USBDriver/BuildProduct' \
        --exclude='mac/USBDriver/DerivedData' \
        --exclude='mac/USBDriver/build-installer' \
        --exclude='mac/USBDriver/build-lib' \
        . {{VM_USER}}@{{VM_HOST}}:{{VM_PROJECT}}/


# Run full test pipeline (Build -> Remote Test)
test: build remote-test

# Trigger the test on the VM
remote-test: sync
    @echo "Triggering remote test on VM..."
    ssh {{VM_USER}}@{{VM_HOST}} "export LANG=en_US.UTF-8; export LC_ALL=en_US.UTF-8; export PATH=/opt/homebrew/bin:/Users/amo/.pyenv/versions/3.12.12/bin:\$PATH; cd {{VM_PROJECT}} && just -f vm.justfile test"

reinstall-driver: sync
    @echo "Reinstalling driver on remote VM..."
    ssh {{VM_USER}}@{{VM_HOST}} "export PATH=/opt/homebrew/bin:\$PATH; cd {{VM_PROJECT}} && just -f vm.justfile reinstall"

insert: sync
    @echo "Inserting key on remote VM..."
    ssh {{VM_USER}}@{{VM_HOST}} "export PATH=/opt/homebrew/bin:\$PATH; cd {{VM_PROJECT}} && just -f vm.justfile insert"

remove: sync
    @echo "Removing key on remote VM..."
    ssh {{VM_USER}}@{{VM_HOST}} "export PATH=/opt/homebrew/bin:\$PATH; cd {{VM_PROJECT}} && just -f vm.justfile remove"

sync-just: sync
sync-binary: sync




# Build macOS components locally
build: build-driver build-client

build-driver:
    rm -rf mac/output
    mkdir -p mac/output
    cd mac/USBDriver && xcodebuild clean build -scheme USBDriverInstaller -configuration Debug -derivedDataPath build/installer CODE_SIGN_STYLE=Manual CODE_SIGN_IDENTITY="" CODE_SIGNING_REQUIRED=NO CODE_SIGNING_ALLOWED=NO PROVISIONING_PROFILE_SPECIFIER=""
    # Build USBDriverLib (static library for client)
    cd mac/USBDriver && xcodebuild clean build -scheme USBDriverLib -configuration Debug -derivedDataPath build/lib CODE_SIGN_STYLE=Manual CODE_SIGN_IDENTITY="" CODE_SIGNING_REQUIRED=NO CODE_SIGNING_ALLOWED=NO PROVISIONING_PROFILE_SPECIFIER=""

    # Manual Ad-Hoc Signing of the artifacts in the build directory
    codesign --force --deep --sign - --entitlements mac/USBDriver/USBDriver/USBDriver.entitlements mac/USBDriver/build/installer/Build/Products/Debug/USBDriverInstaller.app/Contents/Library/SystemExtensions/id.bulwark.VirtualUSBDriver.driver.dext
    codesign --force --deep --sign - --entitlements mac/USBDriver/USBDriverInstaller/USBDriverInstaller.entitlements mac/USBDriver/build/installer/Build/Products/Debug/USBDriverInstaller.app
    # Move artifact to expected location
    cp -r mac/USBDriver/build/installer/Build/Products/Debug/USBDriverInstaller.app mac/output/
    # Zip the app bundle for reliable transfer (VirtioFS listing bug workaround)
    cd mac/output && zip -r USBDriverInstaller.app.zip USBDriverInstaller.app
    # Copy headers for client build
    mkdir -p mac/output/include/USBDriverLib
    cp mac/USBDriver/USBDrverLib/USBDriverLib.h mac/output/include/USBDriverLib/
    cp mac/USBDriver/USBDrverLib/USBDriverShared.h mac/output/include/USBDriverLib/
    # Copy static library
    cp mac/USBDriver/build/lib/Build/Products/Debug/libUSBDriverLib.a mac/output/


build-client:
    # Clean go cache to ensure CGO changes are picked up
    go clean -cache
    go build -o virtual-fido-macos ./cmd/virtual-fido
    codesign --force --sign - -i id.bulwark.virtual-fido --entitlements mac/entitlements.plist virtual-fido-macos


