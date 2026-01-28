#!/usr/bin/env python3
"""
Manual E2E test script for debugging online tests in VM.
Run this directly in the VM to see what's happening with browser interactions.
"""
import sys
import time
import argparse
from playwright.sync_api import sync_playwright

def test_site(site, username, headless=False):
    """Test a specific site with optional headless mode."""
    print(f"\n{'='*60}")
    print(f"Testing: {site}")
    print(f"Username: {username}")
    print(f"Headless: {headless}")
    print(f"{'='*60}\n")
    
    with sync_playwright() as p:
        # Use system Chrome (likely has the Antigravity plugin or specific config)
        browser = p.chromium.launch(
            headless=headless,
            channel="chrome",  # Uses installed Google Chrome
            args=["--enable-logging", "--v=1"] # Enable verbose logging
        )
        context = browser.new_context(record_video_dir="videos/")
        page = context.new_page()
        
        # Capture console logs
        page.on("console", lambda msg: print(f"BROWSER_CONSOLE: {msg.text}"))
        
        is_yubico = "yubico.com" in site
        is_webauthn_io = "webauthn.io" in site
        is_local = "localhost" in site
        
        try:
            print(f"[1/8] Navigating to {site}...")
            page.goto(site, timeout=60000)
            page.screenshot(path=f"videos/01_navigation.png")
            print("✓ Navigation successful")
            
            time.sleep(2)  # Let page fully load
            page.screenshot(path=f"videos/02_page_loaded.png")
            
            if is_yubico:
                # Yubico technical page performs WebAuthn registration automatically when clicking NEXT
                print("[2/8] Clicking NEXT to start WebAuthn registration...")
                page.click('button:has-text("NEXT")', timeout=30000)
                page.screenshot(path=f"videos/03_next_clicked.png")
                print("✓ NEXT clicked, WebAuthn ceremony starting...")
                
                # No form to fill - registration happens automatically
                # Just need to handle the native prompt with hotkeys
                
            elif is_webauthn_io:
                print("[2/8] Filling WebAuthn.io form...")
                page.fill("input#input-email", username, timeout=30000)
                page.screenshot(path=f"videos/03_username_filled.png")
                print("✓ Form filled")
                
                print("[3/8] Clicking register button...")
                page.click("button#register-button", timeout=30000)
                page.screenshot(path=f"videos/04_register_clicked.png")
                
            elif is_local:
                print("[2/8] Filling local form...")
                page.fill("input#username", username, timeout=30000)
                page.screenshot(path=f"videos/03_username_filled.png")
                print("✓ Form filled")
                
                print("[3/8] Clicking register button...")
                page.click("button#register", timeout=30000)
                page.screenshot(path=f"videos/04_register_clicked.png")
            
            print("✓ Registration initiated")
            
            print("[4/8] Waiting for native prompt to appear...")
            time.sleep(2)
            page.screenshot(path=f"videos/05_before_hotkeys.png")
            
            print("[5/8] Sending hotkey hack for native prompt...")
            for i in range(3):
                time.sleep(1)
                page.keyboard.press("ArrowDown")
                time.sleep(0.5)
                page.keyboard.press("Enter")
            print("✓ Hotkeys sent")
            page.screenshot(path=f"videos/06_after_hotkeys.png")
            
            print("[6/8] Waiting for success message...")
            if is_yubico:
                page.wait_for_selector("text=Registration completed", timeout=120000)
                success_text = "Registration completed"
            else:
                page.wait_for_selector("text=Registration Successful", timeout=120000)
                success_text = "Registration Successful"
            
            page.screenshot(path=f"videos/07_registration_success.png")
            print(f"✅ SUCCESS: Found '{success_text}'")
            
            # Login flow
            print("[7/8] Starting login flow...")
            time.sleep(2)
            
            if is_local:
                page.click("button#login", timeout=30000)
                page.screenshot(path=f"videos/08_login_clicked.png")
                
                print("[8/8] Sending hotkeys for login...")
                for i in range(3):
                    time.sleep(1)
                    page.keyboard.press("ArrowDown")
                    time.sleep(0.5)
                    page.keyboard.press("Enter")
                
                page.wait_for_selector("text=Login Successful", timeout=120000)
                page.screenshot(path=f"videos/09_login_success.png")
                print("✅ Login successful!")
            
            return True
            
        except Exception as e:
            print(f"\n❌ FAILED: {e}")
            page.screenshot(path=f"videos/99_error.png")
            
            # Print page content for debugging
            print("\n--- Page Title ---")
            print(page.title())
            print("\n--- Page URL ---")
            print(page.url)
            
            return False
        finally:
            time.sleep(2)
            context.close()
            browser.close()

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Manual E2E test for debugging")
    parser.add_argument("--site", required=True, help="Site to test")
    parser.add_argument("--username", default=f"test_{int(time.time())}", help="Username to use")
    parser.add_argument("--headless", action="store_true", help="Run in headless mode")
    args = parser.parse_args()
    
    success = test_site(args.site, args.username, args.headless)
    sys.exit(0 if success else 1)
