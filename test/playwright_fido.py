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
        page.screenshot(path="videos/01_navigation.png")
        
        # Registration
        print(f"Registering user: {username}")
        page.fill("#input-email", username)
        page.click("#register-button")
        page.screenshot(path="videos/02_registration_clicked.png")
        
        print("Waiting for FIDO registration prompt...")
        try:
            page.wait_for_selector("text=Registration Successful", timeout=30000)
            print("Registration successful!")
            page.screenshot(path="videos/03_registration_success.png")
        except Exception as e:
            print(f"Registration failed or timed out: {e}")
            page.screenshot(path="videos/03_registration_failed.png")
            context.close()
            browser.close()
            return

        time.sleep(2)
        
        # Login
        print(f"Logging in user: {username}")
        page.click("#login-button")
        page.screenshot(path="videos/04_login_clicked.png")
        
        print("Waiting for FIDO login prompt...")
        try:
            page.wait_for_selector("text=You're logged in", timeout=30000)
            print("Login successful!")
            page.screenshot(path="videos/05_login_success.png")
        except Exception as e:
            print(f"Login failed or timed out: {e}")
            page.screenshot(path="videos/05_login_failed.png")
        
        time.sleep(2)
        context.close()
        browser.close()

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--username", default=f"testuser_{int(time.time())}")
    args = parser.parse_args()
    run(args.username)
