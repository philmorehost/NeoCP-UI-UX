import urllib.request
import json
import os
import sys
import time
import subprocess

BASE_URL = "http://localhost:8080"

def get_token():
    print("[TEST] Authenticating as customer 'patel'...")
    url = f"{BASE_URL}/api/login"
    data = json.dumps({"username": "patel", "password": "password"}).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            token = response.get("token")
            print(f"[TEST] Authentication successful. Token received: {token[:20]}...")
            return token
    except Exception as e:
        print(f"[ERROR] Failed to authenticate: {e}")
        sys.exit(1)

def restart_server():
    print("[TEST] Restarting NeoCP server daemon to reload database state...")
    # Kill the server process on Windows
    os.system("taskkill /f /im neocp.exe >nul 2>&1")
    time.sleep(1.5)
    
    # Start the server in the background
    try:
        subprocess.Popen(
            [os.path.abspath("neocp.exe")], 
            stdout=subprocess.DEVNULL, 
            stderr=subprocess.DEVNULL,
            cwd=os.path.abspath(".")
        )
        print("[TEST] Server started in the background.")
        time.sleep(2.5) # Give the server ample time to bind port 8080 and 8443
    except Exception as e:
        print(f"[ERROR] Failed to spawn neocp.exe: {e}")
        sys.exit(1)

def test_ssl_order(token):
    print("\n[TEST] 1. Ordering SSL Certificate for blog.digitalneo.net...")
    url = f"{BASE_URL}/api/domains/ssl/order"
    data = json.dumps({"domain_name": "blog.digitalneo.net"}).encode("utf-8")
    req = urllib.request.Request(
        url, 
        data=data, 
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] SSL order response: {response}")
            if response.get("success") is True:
                print("[TEST] Let's Encrypt SSL successfully ordered and generated!")
            else:
                print(f"[ERROR] SSL ordering returned failure status: {response}")
                sys.exit(1)
    except Exception as e:
        print(f"[ERROR] Failed during Let's Encrypt SSL order: {e}")
        sys.exit(1)

    # Verify Nginx configuration file was regenerated and contains SSL configurations
    conf_path = os.path.join("nginx_vhosts", "blog.digitalneo.net.conf")
    if os.path.exists(conf_path):
        with open(conf_path, "r") as f:
            content = f.read()
            if "ssl_certificate" in content and "ssl_certificate_key" in content:
                print(f"[TEST] Verified regenerated Nginx vhost config at {conf_path} has SSL enabled!")
            else:
                print(f"[ERROR] Nginx config did not contain SSL parameters: {content}")
                sys.exit(1)
    else:
        print(f"[ERROR] Expected Nginx config file not found at {conf_path}")
        sys.exit(1)

def test_quotas(token):
    print("\n[TEST] 2. Auditing and Enforcing Hard Disk Quota Limits...")
    
    # 2a. Modify neocp_data.json directly to set patel's DiskLimit to 1 MB for testing
    db_file = "neocp_data.json"
    if not os.path.exists(db_file):
        print(f"[ERROR] Database file {db_file} not found!")
        sys.exit(1)

    with open(db_file, "r") as f:
        db = json.load(f)
    
    original_limit = db["accounts"]["patel"]["disk_limit"]
    print(f"[TEST] Original disk limit for 'patel': {original_limit} MB")
    
    # Temporarily set limit to 1 MB
    db["accounts"]["patel"]["disk_limit"] = 1
    with open(db_file, "w") as f:
        json.dump(db, f, indent=2)
    
    print("[TEST] Injected temporary 1 MB disk quota limit into database file on disk.")
    
    # Restart server so the database singleton is reloaded
    restart_server()
    
    # Refresh authentication token after server restart
    token = get_token()
    
    url = f"{BASE_URL}/api/filemanager/write"
    large_content = "X" * (2 * 1024 * 1024) # 2 MB content
    data = json.dumps({
        "path": "/home/patel/public_html/large_test_file.txt",
        "content": large_content
    }).encode("utf-8")
    
    req = urllib.request.Request(
        url, 
        data=data, 
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    
    try:
        print("[TEST] Attempting to write a 2MB file. Expected: 403 Forbidden Quota Violation...")
        with urllib.request.urlopen(req) as res:
            response = res.read().decode("utf-8")
            print(f"[ERROR] Write succeeded, but was expected to fail! Response: {response}")
            sys.exit(1)
    except urllib.error.HTTPError as e:
        if e.code == 403:
            err_res = json.loads(e.read().decode("utf-8"))
            print(f"[TEST] Successfully caught expected 403 Forbidden! Quota violation message: {err_res.get('error')}")
        else:
            print(f"[ERROR] Write failed with unexpected HTTP error code {e.code}: {e.read()}")
            sys.exit(1)
    except Exception as e:
        print(f"[ERROR] Unexpected exception: {e}")
        sys.exit(1)

    # Restore database limit
    print("[TEST] Restoring database file settings back to normal...")
    with open(db_file, "r") as f:
        db = json.load(f)
    db["accounts"]["patel"]["disk_limit"] = original_limit
    with open(db_file, "w") as f:
        json.dump(db, f, indent=2)
    
    # Restart server again to reload normal state
    restart_server()
    print(f"[TEST] Restored disk limit for 'patel' back to original {original_limit} MB and server reloaded.")
    return get_token()

def test_backups(token):
    print("\n[TEST] 3. Verifying Zip Backup & Recovery Subsystem...")
    
    # 3a. Create Backup zip
    url = f"{BASE_URL}/api/backup/create"
    req = urllib.request.Request(
        url, 
        data=b"", 
        headers={"Authorization": f"Bearer {token}"}
    )
    backup_filename = None
    try:
        print("[TEST] Triggering user ZIP backup archive generation...")
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Backup response: {response}")
            if response.get("success") is True:
                backup_filename = response.get("filename")
                print(f"[TEST] Successfully generated ZIP archive: {backup_filename}")
            else:
                print(f"[ERROR] Backup creation returned failure status: {response}")
                sys.exit(1)
    except Exception as e:
        print(f"[ERROR] Failed to create backup: {e}")
        sys.exit(1)

    # 3b. List backups
    url = f"{BASE_URL}/api/backup/list"
    req = urllib.request.Request(
        url,
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        print("[TEST] Fetching active backups list...")
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Active backups: {response}")
            found = False
            for item in response:
                if item.get("filename") == backup_filename:
                    print(f"[TEST] Verified backup archive {backup_filename} is listed in metadata! Size: {item.get('size_mb'):.4f} MB")
                    found = True
                    break
            if not found:
                print(f"[ERROR] Generated backup archive {backup_filename} was not found in the list!")
                sys.exit(1)
    except Exception as e:
        print(f"[ERROR] Failed to list backups: {e}")
        sys.exit(1)

    # 3c. Write a temporary "secret.txt" file into patel's public_html
    print("[TEST] Creating a new file 'secret.txt' in the user's workspace...")
    write_url = f"{BASE_URL}/api/filemanager/write"
    write_data = json.dumps({
        "path": "/home/patel/public_html/secret.txt",
        "content": "NeoCP Backup Test Secret String"
    }).encode("utf-8")
    req = urllib.request.Request(
        write_url, 
        data=write_data, 
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        with urllib.request.urlopen(req) as res:
            print("[TEST] 'secret.txt' file written successfully!")
    except Exception as e:
        print(f"[ERROR] Failed to write test file: {e}")
        sys.exit(1)

    # 3d. Restore from the backup archive (this should purge active workspace and remove 'secret.txt')
    restore_url = f"{BASE_URL}/api/backup/restore"
    restore_data = json.dumps({"filename": backup_filename}).encode("utf-8")
    req = urllib.request.Request(
        restore_url, 
        data=restore_data, 
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        print(f"[TEST] Initiating restoration of backup {backup_filename}...")
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Restore response: {response}")
            if response.get("success") is True:
                print("[TEST] Restored files from ZIP package successfully!")
            else:
                print(f"[ERROR] Backup restoration returned failure status: {response}")
                sys.exit(1)
    except Exception as e:
        print(f"[ERROR] Failed to restore from backup: {e}")
        sys.exit(1)

    # 3e. Verify 'secret.txt' no longer exists (it was created *after* the backup zip, so the purge deleted it)
    secret_file_path = os.path.join("sandbox", "patel", "public_html", "secret.txt")
    if not os.path.exists(secret_file_path):
        print("[TEST] Verified 'secret.txt' has been successfully removed by purge and restoration! State reverted perfectly.")
    else:
        print(f"[ERROR] 'secret.txt' still exists at {secret_file_path}! Restoration did not purge the file.")
        sys.exit(1)

if __name__ == "__main__":
    # Wait for the Go server to fully initialize
    time.sleep(1)
    token = get_token()
    test_ssl_order(token)
    token = test_quotas(token)
    test_backups(token)
    print("\n[SUCCESS] ALL STAGE 2 ACTIVE SUBSYSTEM INTEGRATION TESTS PASSED FLAWLESSLY!")
