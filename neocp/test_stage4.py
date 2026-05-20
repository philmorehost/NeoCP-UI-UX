import urllib.request
import json
import os
import sys
import time
import subprocess

BASE_URL = "http://localhost:8080"

def get_token(username, password):
    print(f"[TEST] Authenticating as '{username}'...")
    url = f"{BASE_URL}/api/login"
    data = json.dumps({"username": username, "password": password}).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as res:
            response = json.loads(res.read().decode("utf-8"))
            token = response.get("token")
            print(f"[TEST] Auth successful for {username}. Token received: {token[:20]}...")
            return token
    except Exception as e:
        print(f"[ERROR] Failed to authenticate user '{username}': {e}")
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

def test_dns_records(token):
    print("\n[TEST] === Starting DNS CRUD & BIND9 Zone Generation Verification ===")
    
    domain = "blog.digitalneo.net"
    
    # 1. Fetch current records
    url = f"{BASE_URL}/api/domains/dns?domain={domain}"
    req = urllib.request.Request(
        url,
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        with urllib.request.urlopen(req) as res:
            records = json.loads(res.read().decode("utf-8")) or []
            print(f"[TEST] Current DNS records: {records}")
    except Exception as e:
        print(f"[ERROR] Failed to fetch DNS records: {e}")
        sys.exit(1)

    # 2. Add a new DNS record
    add_url = f"{BASE_URL}/api/domains/dns"
    payload = {
        "domain_name": domain,
        "record": {
            "type": "A",
            "name": "stage4test",
            "value": "192.0.2.4",
            "ttl": 3600
        }
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        add_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        print("[TEST] Publishing new DNS A record (stage4test -> 192.0.2.4)...")
        with urllib.request.urlopen(req) as res:
            record_res = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Record published: {record_res}")
            assert record_res["type"] == "A"
            assert record_res["name"] == "stage4test"
            assert record_res["value"] == "192.0.2.4"
            assert "dns_" in record_res["id"]
            record_id = record_res["id"]
    except Exception as e:
        print(f"[ERROR] Failed to publish DNS record: {e}")
        sys.exit(1)

    # 3. Check physical BIND9 zone file
    zone_file_path = os.path.join("dns_zones", f"{domain}.db")
    assert os.path.exists(zone_file_path), "BIND9 zone file was not written!"
    with open(zone_file_path, "r") as f:
        zone_content = f.read()
        print(f"[TEST] BIND9 zone file verified successfully. Length: {len(zone_content)}")
        assert "stage4test" in zone_content
        assert "192.0.2.4" in zone_content

    # 4. Delete the DNS record
    delete_payload = {
        "domain_name": domain,
        "record_id": record_id
    }
    data = json.dumps(delete_payload).encode("utf-8")
    req = urllib.request.Request(
        add_url,
        data=data,
        method="DELETE",
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        print(f"[TEST] Unpublishing DNS record with ID '{record_id}'...")
        with urllib.request.urlopen(req) as res:
            del_res = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Record deleted response: {del_res}")
            assert del_res.get("success") is True
    except Exception as e:
        print(f"[ERROR] Failed to delete DNS record: {e}")
        sys.exit(1)

    # 5. Check physical zone file after deletion
    with open(zone_file_path, "r") as f:
        zone_content = f.read()
        assert "stage4test" not in zone_content, "Deleted DNS record is still present in BIND9 zone file!"
        print("[TEST] BIND9 zone file successfully updated to exclude deleted record!")

def test_waf_policy_and_nginx(token):
    print("\n[TEST] === Starting OWASP WAF Policy & Nginx VHost Generation Verification ===")
    
    domain = "blog.digitalneo.net"
    waf_url = f"{BASE_URL}/api/domains/waf"
    
    # 1. Update WAF policy
    payload = {
        "domain_name": domain,
        "waf_policy": {
            "sqli_shield": True,
            "xss_block": True,
            "lfi_shield": True,
            "csrf_header": True
        }
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        waf_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        print("[TEST] Updating OWASP WAF policy to enable all shields...")
        with urllib.request.urlopen(req) as res:
            waf_res = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] WAF policy updated: {waf_res}")
            assert waf_res.get("success") is True
    except Exception as e:
        print(f"[ERROR] Failed to update WAF policy: {e}")
        sys.exit(1)

    # 2. Check Nginx configuration file
    nginx_conf_path = os.path.join("nginx_vhosts", f"{domain}.conf")
    assert os.path.exists(nginx_conf_path), "Nginx Virtual Host configuration file was not written!"
    with open(nginx_conf_path, "r") as f:
        conf_content = f.read()
        print(f"[TEST] Nginx config verified. Length: {len(conf_content)}")
        
        # Verify specific WAF headers and directives are injected
        assert "X-XSS-Protection" in conf_content, "XSS shield header not found in Nginx config"
        assert "Content-Security-Policy" in conf_content, "CSP header not found in Nginx config"
        assert "X-Frame-Options" in conf_content, "CSRF/Frame options header not found in Nginx config"
        assert "union\\s+select" in conf_content, "SQLi shielding regex not found in Nginx config"
        assert "\\.\\./" in conf_content, "LFI shielding regex not found in Nginx config"
        print("[TEST] Verified Nginx config contains all active WAF directives!")

def test_package_quota_gating(admin_token, user_token):
    print("\n[TEST] === Starting Reseller Package Limit Gating Verification ===")
    
    # 1. Create a Reseller Package restricting Standard Hosting Plan to current levels
    pkg_url = f"{BASE_URL}/api/packages"
    payload = {
        "name": "Standard Hosting Plan",
        "disk_limit": 5000,
        "bandwidth_limit": 100,
        "domains_limit": 2,
        "databases_limit": 1,
        "lve_cpu_pct": 50,
        "lve_ram_mb": 512
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        pkg_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {admin_token}"
        }
    )
    try:
        print("[TEST] Registering quota ceiling via Reseller Package...")
        with urllib.request.urlopen(req) as res:
            pkg_res = json.loads(res.read().decode("utf-8"))
            print(f"[TEST] Package configured: {pkg_res}")
    except Exception as e:
        print(f"[ERROR] Failed to register reseller package: {e}")
        sys.exit(1)

    # 2. Try to add a 3rd domain as Customer 'patel' (who currently has 2)
    domain_url = f"{BASE_URL}/api/domains"
    domain_payload = {
        "domain_name": "extra.digitalneo.net",
        "php_version": "8.2"
    }
    data = json.dumps(domain_payload).encode("utf-8")
    req = urllib.request.Request(
        domain_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {user_token}"
        }
    )
    try:
        print("[TEST] Registering 3rd domain (exceeding package limit of 2). Expected: 403 Forbidden...")
        with urllib.request.urlopen(req) as res:
            print("[ERROR] Adding 3rd domain succeeded despite package quota ceiling!")
            sys.exit(1)
    except urllib.error.HTTPError as e:
        if e.code == 403:
            err_res = json.loads(e.read().decode("utf-8"))
            print(f"[TEST] Correctly caught expected 403 Forbidden! Response: '{err_res.get('error')}'")
            assert "limit reached" in err_res.get("error").lower()
        else:
            print(f"[ERROR] Unexpected HTTP status code {e.code} during domain registration limit test")
            sys.exit(1)
    except Exception as e:
        print(f"[ERROR] Failed domain gating test: {e}")
        sys.exit(1)

    # 3. Try to add a 2nd database as Customer 'patel' (who currently has 1)
    db_url = f"{BASE_URL}/api/databases"
    db_payload = {
        "name": "patel_extradb",
        "db_user": "patel_extrauser",
        "password": "extradbpassword123",
        "remote_ips": "%"
    }
    data = json.dumps(db_payload).encode("utf-8")
    req = urllib.request.Request(
        db_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {user_token}"
        }
    )
    try:
        print("[TEST] Creating 2nd database (exceeding package limit of 1). Expected: 403 Forbidden...")
        with urllib.request.urlopen(req) as res:
            print("[ERROR] Creating 2nd database succeeded despite package quota ceiling!")
            sys.exit(1)
    except urllib.error.HTTPError as e:
        if e.code == 403:
            err_res = json.loads(e.read().decode("utf-8"))
            print(f"[TEST] Correctly caught expected 403 Forbidden! Response: '{err_res.get('error')}'")
            assert "limit reached" in err_res.get("error").lower()
        else:
            print(f"[ERROR] Unexpected HTTP status code {e.code} during database registration limit test")
            sys.exit(1)
    except Exception as e:
        print(f"[ERROR] Failed database gating test: {e}")
        sys.exit(1)

if __name__ == "__main__":
    # Ensure server is running and clean
    restart_server()
    
    admin_token = get_token("admin", "password")
    user_token = get_token("patel", "password")
    
    test_dns_records(user_token)
    test_waf_policy_and_nginx(user_token)
    test_package_quota_gating(admin_token, user_token)
    
    # Tear down
    print("\n[TEST] Tearing down background server process...")
    os.system("taskkill /f /im neocp.exe >nul 2>&1")
    
    print("\n[SUCCESS] ALL STAGE 4 BACKEND INTEGRATION & INTEGRITY TESTS PASSED FLAWLESSLY!")
