import sys
import time
import argparse
from playwright.sync_api import sync_playwright

def run(username, site="https://webauthn.io"):
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(record_video_dir="videos/")
        page = context.new_page()
        
        print(f"Navigating to {site}...")
        page.goto(site)
        
        # Registration
        print(f"Registering user: {username}")
        page.fill("#input-email", username)
        page.click("#register-button")
        
        print("Waiting for FIDO registration prompt...")
        # We wait for the success message or a timeout
        # In a real FIDO test, the OS prompt appears here.
        # Playwright cannot interact with the OS prompt, but it waits for the browser to receive the result.
        try:
            page.wait_for_selector("text=Registration Successful", timeout=60000)
            print("Registration successful!")
        except Exception as e:
            print(f"Registration failed or timed out: {e}")
            browser.close()
            return

        time.sleep(2)
        
        # Login
        print(f"Logging in user: {username}")
        page.click("#login-button")
        
        print("Waiting for FIDO login prompt...")
        try:
            page.wait_for_selector("text=You're logged in", timeout=60000)
            print("Login successful!")
        except Exception as e:
            print(f"Login failed or timed out: {e}")
        
        time.sleep(5)
        browser.close()

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--username", default=f"testuser_{int(time.time())}")
    args = parser.parse_args()
    run(args.username)
