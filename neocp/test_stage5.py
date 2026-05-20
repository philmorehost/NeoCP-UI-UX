import urllib.request
import json
import os
import sys
import time
import subprocess
import socket
import ssl

BASE_URL = "http://localhost:8080"
CLUSTER_PORT = 8444

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
    print("[TEST] Restarting NeoCP server daemon...")
    # Kill any existing running server
    if os.name == 'nt':
        os.system("taskkill /f /im neocp.exe >nul 2>&1")
    else:
        os.system("pkill -f neocp.exe > /dev/null 2>&1")
    time.sleep(1.0)

    # Start the server in the background
    exe_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "neocp.exe")
    try:
        subprocess.Popen(
            [exe_path],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            cwd=os.path.dirname(os.path.abspath(__file__))
        )
        print("[TEST] Server daemon spawned in the background.")
        time.sleep(2.0) # wait for binding
    except Exception as e:
        print(f"[ERROR] Failed to spawn neocp.exe: {e}")
        sys.exit(1)

def test_cluster_attachment(token):
    print("\n[TEST] === Starting Cluster Node Attachment & mTLS Handshake Verification ===")

    # 1. Attach a node via API
    attach_url = f"{BASE_URL}/api/cluster/attach"
    node_id = "test-worker-01"
    payload = {
        "node_id": node_id,
        "ip": "127.0.0.1",
        "role": "web"
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        attach_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )

    try:
        print(f"[TEST] Attaching worker node '{node_id}'...")
        with urllib.request.urlopen(req) as res:
            resp = json.loads(res.read().decode("utf-8"))
            assert resp["success"] is True
            ca_cert = resp["ca_cert"]
            client_cert = resp["client_cert"]
            client_key = resp["client_key"]
            print("[TEST] Received mTLS certificate payload from Master.")
    except Exception as e:
        print(f"[ERROR] Failed to attach cluster node: {e}")
        sys.exit(1)

    # 2. Simulate Worker gRPC connection over mTLS
    print("[TEST] Simulating worker gRPC mTLS stream to port 8444...")

    # Save certs to temp files for SSLContext
    with open("tmp_ca.crt", "w") as f: f.write(ca_cert)
    with open("tmp_client.crt", "w") as f: f.write(client_cert)
    with open("tmp_client.key", "w") as f: f.write(client_key)

    context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH, cafile="tmp_ca.crt")
    context.load_cert_chain(certfile="tmp_client.crt", keyfile="tmp_client.key")
    context.check_hostname = False
    context.verify_mode = ssl.CERT_REQUIRED

    try:
        with socket.create_connection(("localhost", CLUSTER_PORT)) as sock:
            with context.wrap_socket(sock, server_hostname="localhost") as ssock:
                print(f"[TEST] mTLS Handshake successful with Master. Protocol: {ssock.version()}")

                # Send a telemetry packet
                packet = {
                    "node_id": node_id,
                    "metrics": {
                        "cpu_load": 12.5,
                        "ram_load": 45.2
                    }
                }
                ssock.sendall((json.dumps(packet) + "\n").encode("utf-8"))

                # Read ACK
                response = ssock.recv(1024).decode("utf-8")
                print(f"[TEST] Master response: {response.strip()}")
                assert "ack" in response.lower()

                # 3. Verify metrics in Master API while STILL CONNECTED
                time.sleep(0.5)
                nodes_url = f"{BASE_URL}/api/cluster/nodes"
                req = urllib.request.Request(nodes_url, headers={"Authorization": f"Bearer {token}"})
                with urllib.request.urlopen(req) as res:
                    raw_body = res.read().decode("utf-8")
                    print(f"[DEBUG] Cluster nodes response (while connected): {raw_body}")
                    nodes = json.loads(raw_body)
                    target = next((n for n in nodes if n["node_id"] == node_id), None)
                    if target is None:
                        print(f"[ERROR] Node '{node_id}' not found in cluster list.")
                        sys.exit(1)

                    assert target["cpu_load"] == 12.5
                    assert target["ram_load"] == 45.2
                    assert target["is_active"] is True
                    print("[TEST] Master successfully persisted worker telemetry metrics and marked node as ACTIVE.")
    except Exception as e:
        print(f"[ERROR] Cluster mTLS verification failed: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
    finally:
        for f in ["tmp_ca.crt", "tmp_client.crt", "tmp_client.key"]:
            if os.path.exists(f): os.remove(f)

def test_staging_replication(token):
    print("\n[TEST] === Starting 1-Click Staging & Serialization-Aware Rewrite Verification ===")

    owner = "patel"
    prod_domain = "blog.digitalneo.net"
    stage_subdomain = "blog-stage.digitalneo.net"

    # 1. Create a dummy WordPress-like config file with serialized data
    # Ensure absolute path relative to where neocp.exe is running
    base_dir = os.path.dirname(os.path.abspath(__file__))
    sandbox_path = os.path.join(base_dir, "sandbox", owner, "public_html", prod_domain)
    # Ensure production domain exists in the DB first (it's seeded, but let's be sure)

    os.makedirs(sandbox_path, exist_ok=True)

    wp_config_content = f"""<?php
define('DB_NAME', 'patel_wpblog');
define('DOMAIN', '{prod_domain}');
// Serialized PHP array simulation
$settings = 'a:2:{{s:4:"site";s:{len(prod_domain)}:"{prod_domain}";s:2:"db";s:12:"patel_wpblog";}}';
"""
    with open(os.path.join(sandbox_path, "wp-config.php"), "w") as f:
        f.write(wp_config_content)

    # 2. Trigger Clone to Staging
    clone_url = f"{BASE_URL}/api/staging?action=clone"
    payload = {
        "production_domain": prod_domain,
        "staging_subdomain": stage_subdomain
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        clone_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )

    try:
        print(f"[TEST] Cloning '{prod_domain}' to '{stage_subdomain}'...")
        with urllib.request.urlopen(req) as res:
            resp = json.loads(res.read().decode("utf-8"))
            assert resp["success"] is True
    except Exception as e:
        print(f"[ERROR] Staging clone failed: {e}")
        sys.exit(1)

    # 3. Verify file replication and serialization rewrite
    stage_path = os.path.join(base_dir, "sandbox", owner, "public_html", stage_subdomain)
    assert os.path.exists(stage_path), f"Staging directory not created at {stage_path}!"

    with open(os.path.join(stage_path, "wp-config.php"), "r") as f:
        content = f.read()
        print("[TEST] Verifying serialization-aware rewrite in staging config...")
        assert stage_subdomain in content
        assert "patel_staging_wpblog" in content
        # Check string length update: s:27:"staging-test.digitalneo.net"
        expected_len = len(stage_subdomain)
        assert f's:{expected_len}:"{stage_subdomain}"' in content
        print(f"[TEST] Successfully verified serialized length recalculation (s:{expected_len}).")

    # 4. Push Staging to Production
    push_url = f"{BASE_URL}/api/staging?action=push"
    payload = {
        "staging_subdomain": stage_subdomain,
        "sync_mode": "both"
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        push_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )

    try:
        print("[TEST] Pushing staging updates back to production...")
        # Add a new file in staging to verify sync
        with open(os.path.join(stage_path, "new_feature.php"), "w") as f:
            f.write("<?php echo 'new feature'; ?>")

        with urllib.request.urlopen(req) as res:
            resp = json.loads(res.read().decode("utf-8"))
            assert resp["success"] is True

        # Verify production now has the new file and original domain names
        with open(os.path.join(sandbox_path, "new_feature.php"), "r") as f:
            assert f.read() == "<?php echo 'new feature'; ?>"

        with open(os.path.join(sandbox_path, "wp-config.php"), "r") as f:
            content = f.read()
            assert prod_domain in content
            assert "patel_wpblog" in content
            assert f's:{len(prod_domain)}:"{prod_domain}"' in content

        print("[TEST] Staging-to-Production synchronization verified successfully.")
    except Exception as e:
        print(f"[ERROR] Staging push failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    restart_server()

    admin_token = get_token("admin", "admin123")
    user_token = get_token("patel", "patel123")

    test_cluster_attachment(admin_token)
    test_staging_replication(user_token)

    # Tear down
    print("\n[TEST] Tearing down background server process...")
    os.system("taskkill /f /im neocp.exe >nul 2>&1")

    print("\n[SUCCESS] ALL STAGE 5 DISTRIBUTED CLUSTERING & STAGING TESTS PASSED FLAWLESSLY!")
