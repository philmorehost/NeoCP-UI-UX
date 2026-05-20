import urllib.request
import json
import os
import sys
import time
import subprocess

BASE_URL = "http://localhost:8080"

def get_admin_token():
    print("[TEST] Authenticating as administrator 'admin'...")
    url = f"{BASE_URL}/api/login"
    data = json.dumps({"username": "admin", "password": "password"}).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            token = response.get("token")
            print(f"[TEST] Admin Auth successful. Token received: {token[:20]}...")
            return token
    except Exception as e:
        print(f"[ERROR] Failed to authenticate admin: {e}")
        sys.exit(1)

def restart_server():
    print("[TEST] Restarting NeoCP server daemon to refresh database state...")
    # Kill any existing running server on Windows
    os.system("taskkill /f /im neocp.exe >nul 2>&1")
    time.sleep(1.0)
    
    # Start the server in the background
    try:
        subprocess.Popen(
            [os.path.abspath("neocp.exe")], 
            stdout=subprocess.DEVNULL, 
            stderr=subprocess.DEVNULL,
            cwd=os.path.abspath(".")
        )
        print("[TEST] Server daemon spawned in the background.")
        time.sleep(2.0) # wait for binding
    except Exception as e:
        print(f"[ERROR] Failed to spawn neocp.exe: {e}")
        sys.exit(1)

def test_docker_containers(token):
    print("\n[TEST] === Starting Docker Registry & Containers Manager Verification ===")
    
    # 1. Fetch current list
    url = f"{BASE_URL}/api/docker/containers"
    req = urllib.request.Request(
        url,
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        with urllib.request.urlopen(req) as res:
            containers = json.loads(res.read().decode("utf-8")) or []
            print(f"[TEST] Current containers list retrieved: {containers}")
    except Exception as e:
        print(f"[ERROR] Failed to list containers: {e}")
        sys.exit(1)

    # 2. Deploy a new simulated container
    deploy_url = f"{BASE_URL}/api/docker/containers/deploy"
    payload = {
        "name": "redis-cache-stage3",
        "image": "redis:alpine",
        "ports": "6379:6379"
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        deploy_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        print("[TEST] Deploying simulated container 'redis-cache-stage3'...")
        with urllib.request.urlopen(req) as res:
            res_cont = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Container deployed: {res_cont}")
            assert res_cont["name"] == "redis-cache-stage3"
            assert res_cont["image"] == "redis:alpine"
            assert res_cont["status"] == "running"
    except Exception as e:
        print(f"[ERROR] Failed to deploy container: {e}")
        sys.exit(1)

    # 3. Toggle Container Status (stop it)
    toggle_url = f"{BASE_URL}/api/docker/containers/toggle"
    data = json.dumps({"name": "redis-cache-stage3"}).encode("utf-8")
    req = urllib.request.Request(
        toggle_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        print("[TEST] Toggling container state (should stop running)...")
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Toggle response: {response}")
            assert response.get("success") is True
    except Exception as e:
        print(f"[ERROR] Failed to toggle container: {e}")
        sys.exit(1)

    # Verify status changed in database
    req = urllib.request.Request(
        f"{BASE_URL}/api/docker/containers",
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        with urllib.request.urlopen(req) as res:
            containers = json.loads(res.read().decode("utf-8")) or []
            target = next((c for c in containers if c["name"] == "redis-cache-stage3"), None)
            assert target is not None, "Container not found in list"
            print(f"[TEST] Container status after toggle: {target['status']}")
            assert target["status"] == "stopped"
    except Exception as e:
        print(f"[ERROR] Verification failed: {e}")
        sys.exit(1)

    # 4. Delete container
    delete_url = f"{BASE_URL}/api/docker/containers?name=redis-cache-stage3"
    req = urllib.request.Request(
        delete_url,
        method="DELETE",
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        print("[TEST] Deleting container 'redis-cache-stage3'...")
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Delete response: {response}")
            assert response.get("success") is True
    except Exception as e:
        print(f"[ERROR] Failed to delete container: {e}")
        sys.exit(1)

    # Verify it is deleted from list
    req = urllib.request.Request(
        f"{BASE_URL}/api/docker/containers",
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        with urllib.request.urlopen(req) as res:
            containers = json.loads(res.read().decode("utf-8")) or []
            target = next((c for c in containers if c["name"] == "redis-cache-stage3"), None)
            assert target is None, "Container should have been deleted"
            print("[TEST] Verified container successfully purged from the database registry!")
    except Exception as e:
        print(f"[ERROR] Post-deletion list verification failed: {e}")
        sys.exit(1)


def test_firewall_and_cphulk(token):
    print("\n[TEST] === Starting Active WAF & OS Firewall Block Integration Verification ===")
    
    # 1. Fetch current blocks (should be clean or seeded)
    blocks_url = f"{BASE_URL}/api/security/firewall/blocks"
    req = urllib.request.Request(
        blocks_url,
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        with urllib.request.urlopen(req) as res:
            blocks = json.loads(res.read().decode("utf-8")) or []
            print(f"[TEST] Current active firewall block list: {blocks}")
    except Exception as e:
        print(f"[ERROR] Failed to fetch firewall blocks: {e}")
        sys.exit(1)

    # 2. Add an active IP block manually
    block_url = f"{BASE_URL}/api/security/firewall/block"
    payload = {
        "ip": "203.0.113.88",
        "reason": "Administrative manual lockout check"
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        block_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        print("[TEST] Injecting manual IP block for '203.0.113.88'...")
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] IP Block response: {response}")
            assert response.get("success") is True
    except Exception as e:
        print(f"[ERROR] Failed to add firewall block: {e}")
        sys.exit(1)

    # Verify IP listed in block grid
    req = urllib.request.Request(
        blocks_url,
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        with urllib.request.urlopen(req) as res:
            blocks = json.loads(res.read().decode("utf-8")) or []
            target = next((b for b in blocks if b["ip"] == "203.0.113.88"), None)
            assert target is not None, "Blocked IP not found in list"
            print(f"[TEST] Verified blocked IP exists. Reason: '{target['reason']}'")
    except Exception as e:
        print(f"[ERROR] Firewall block verification failed: {e}")
        sys.exit(1)

    # Remove the manual IP block
    unblock_url = f"{BASE_URL}/api/security/firewall/unblock"
    data = json.dumps({"ip": "203.0.113.88"}).encode("utf-8")
    req = urllib.request.Request(
        unblock_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        print("[TEST] Unblocking manual IP block for '203.0.113.88'...")
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] IP Unblock response: {response}")
            assert response.get("success") is True
    except Exception as e:
        print(f"[ERROR] Failed to unblock: {e}")
        sys.exit(1)

    # 3. Test cPHulk Brute-Force Gate
    intruder_ip = "198.51.100.99"
    print(f"\n[TEST] Simulating cPHulk intrusion brute force from Remote IP '{intruder_ip}'...")
    login_url = f"{BASE_URL}/api/login"
    
    # Send 5 failed login attempts
    for attempt in range(1, 6):
        data = json.dumps({"username": "admin", "password": "wrongpassword"}).encode("utf-8")
        req = urllib.request.Request(
            login_url,
            data=data,
            headers={
                "Content-Type": "application/json",
                "X-Forwarded-For": intruder_ip
            }
        )
        try:
            print(f"[TEST] Intrusion attempt {attempt}/5...")
            with urllib.request.urlopen(req) as res:
                print("[ERROR] Login succeeded with wrong password?!")
                sys.exit(1)
        except urllib.error.HTTPError as e:
            if e.code == 401:
                err_res = json.loads(e.read().decode("utf-8"))
                # expect normal invalid credentials error
                assert "invalid" in err_res.get("error").lower()
            else:
                print(f"[ERROR] Unexpected HTTP status code {e.code} during attempt {attempt}")
                sys.exit(1)
        except Exception as e:
            print(f"[ERROR] Failed to send login request: {e}")
            sys.exit(1)
            
    # The 6th attempt should be automatically BLOCKED with 403 Forbidden
    data = json.dumps({"username": "admin", "password": "password"}).encode("utf-8") # correct password, but should still block!
    req = urllib.request.Request(
        login_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "X-Forwarded-For": intruder_ip
        }
    )
    try:
        print("[TEST] Sending 6th attempt (using CORRECT password). Expected: 403 Forbidden / cPHulk lockout...")
        with urllib.request.urlopen(req) as res:
            print("[ERROR] 6th attempt succeeded despite intrusion lock!")
            sys.exit(1)
    except urllib.error.HTTPError as e:
        if e.code == 403:
            err_res = json.loads(e.read().decode("utf-8"))
            print(f"[TEST] Successfully caught expected 403 Forbidden! cPHulk response: '{err_res.get('error')}'")
            assert "cphulk" in err_res.get("error").lower()
        else:
            print(f"[ERROR] Unexpected HTTP status code {e.code} on 6th attempt")
            sys.exit(1)
    except Exception as e:
        print(f"[ERROR] Failed on 6th attempt: {e}")
        sys.exit(1)

    # Verify that intruder_ip is indeed in the firewall block list
    req = urllib.request.Request(
        blocks_url,
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        with urllib.request.urlopen(req) as res:
            blocks = json.loads(res.read().decode("utf-8")) or []
            target = next((b for b in blocks if b["ip"] == intruder_ip), None)
            assert target is not None, "Intruder IP not found in block list"
            print(f"[TEST] Verified intruder IP '{intruder_ip}' was programmatically added to Firewall Blocks! Reason: '{target['reason']}'")
    except Exception as e:
        print(f"[ERROR] Post-lockout firewall verification failed: {e}")
        sys.exit(1)

    # Clean up: Unblock intruder IP so tests are idempotent
    unblock_data = json.dumps({"ip": intruder_ip}).encode("utf-8")
    req = urllib.request.Request(
        unblock_url,
        data=unblock_data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            assert response.get("success") is True
            print("[TEST] Banned Intruder IP cleaned up successfully.")
    except Exception as e:
        print(f"[ERROR] Clean up of intruder IP failed: {e}")
        sys.exit(1)


if __name__ == "__main__":
    # Ensure server is running and clean
    restart_server()
    
    admin_token = get_admin_token()
    
    test_docker_containers(admin_token)
    test_firewall_and_cphulk(admin_token)
    
    # Tear down
    print("\n[TEST] Tearing down background server process...")
    os.system("taskkill /f /im neocp.exe >nul 2>&1")
    
    print("\n[SUCCESS] ALL STAGE 3 BACKEND INTEGRATION & INTEGRITY TESTS PASSED FLAWLESSLY!")
