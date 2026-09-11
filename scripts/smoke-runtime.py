#!/usr/bin/env python3
"""Exercise a real panel/core pair on loopback, with isolated disposable data.

Usage: python3 scripts/smoke-runtime.py PANEL_BINARY MIHOMO_BINARY
The core must be in an allowed installation directory (/opt/ or the managed
/usr/local/lib/3m-ui/). No existing service, database or public endpoint is used.
"""

import contextlib
import hashlib
import http.client
import http.server
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.request


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def stop_process(process):
    if process is not None and process.poll() is None:
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait(timeout=5)


def main():
    panel, core = map(lambda p: str(Path(p).resolve()), sys.argv[1:])
    processes = []
    with tempfile.TemporaryDirectory(prefix="3m-runtime-") as tmp:
        root = Path(tmp)
        data = root / "data"
        config = root / "config.yaml"
        port, listener_port, proxy_port, controller_port = [free_port() for _ in range(4)]
        env = os.environ.copy()
        env.update(THREE_M_UI_CONFIG=str(config), THREE_M_UI_DATA_DIR=str(data),
                   THREE_M_UI_MIHOMO_BINARY=core, THREE_M_UI_PORT=str(port),
                   THREE_M_UI_LISTEN="127.0.0.1", THREE_M_UI_ADMIN_USERNAME="admin",
                   THREE_M_UI_ADMIN_PASSWORD="",
                   THREE_M_UI_MIHOMO_CONTROLLER=f"127.0.0.1:{controller_port}")
        initialized = subprocess.run([panel, "init"], env=env, capture_output=True, text=True, check=True)
        password = re.search(r"^Initial password: (.+)$", initialized.stdout, re.M).group(1)
        fingerprint = hashlib.sha256(config.read_bytes()).hexdigest()
        again = subprocess.run([panel, "init"], env=env, capture_output=True, text=True, check=True)
        assert "Initial password:" not in again.stdout, "Repeated init reset credentials"
        assert hashlib.sha256(config.read_bytes()).hexdigest() == fingerprint, "Repeated init changed keys"
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        token = None

        def api(route, body=None):
            request = urllib.request.Request(f"http://127.0.0.1:{port}/api/v1/{route}",
                                             data=None if body is None else json.dumps(body).encode())
            request.add_header("Content-Type", "application/json")
            if token:
                request.add_header("Authorization", f"Bearer {token}")
            try:
                with opener.open(request, timeout=10) as response:
                    return json.load(response)
            except urllib.error.HTTPError as error:
                raise AssertionError(f"{route}: HTTP {error.code}: {error.read().decode()}") from None

        def start_panel(log):
            process = subprocess.Popen([panel], env=env, cwd=root, stdout=log, stderr=log, start_new_session=True)
            processes.append(process)
            for _ in range(60):
                if process.poll() is not None:
                    raise AssertionError("Panel exited before readiness")
                result = subprocess.run([panel, "healthcheck"], env=env, capture_output=True)
                if result.returncode == 0:
                    return process
                time.sleep(0.25)
            raise AssertionError("Panel readiness timed out")

        with contextlib.ExitStack() as stack:
            stack.callback(lambda: [stop_process(process) for process in reversed(processes)])
            def stop_core():
                with contextlib.suppress(Exception):
                    api("mihomo/stop", {})
            stack.callback(stop_core)
            log = stack.enter_context((root / "panel.log").open("w+"))
            running = start_panel(log)
            token = api("auth/login", {"username": "admin", "password": password})["token"]
            password = "smoke-" + secrets.token_urlsafe(20)
            original = re.search(r"^Initial password: (.+)$", initialized.stdout, re.M).group(1)
            api("auth/password", {"current_password": original, "new_password": password})
            token = api("auth/login", {"username": "admin", "password": password})["token"]
            ss_password = secrets.token_urlsafe(24)
            listener = api("listeners", {
                "name": "runtime-smoke", "protocol": "shadowsocks", "port": str(listener_port),
                "bind_address": "127.0.0.1", "enabled": True,
                "config": json.dumps({"cipher": "aes-128-gcm", "password": ss_password}),
            })
            # Core may finish start slightly after the listener response; poll briefly.
            for _ in range(40):
                if api("mihomo/status").get("running"):
                    break
                time.sleep(0.25)
            else:
                raise AssertionError("Bundled core did not start")
            # Persist real listener certificates as well as the Shadowsocks node.
            api("listeners", {"name": "certificate-smoke", "protocol": "trojan", "port": str(free_port()),
                              "bind_address": "127.0.0.1", "enabled": True, "config": "{}"})
            certificates = {str(p.relative_to(data)): hashlib.sha256(p.read_bytes()).hexdigest()
                            for p in (data / "listener-certs").rglob("*") if p.is_file()}
            assert certificates, "Listener certificates were not persisted"

            class Target(http.server.BaseHTTPRequestHandler):
                def do_GET(self):
                    body = b"3m-ui-real-proxy-ok"
                    self.send_response(200)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)

                def log_message(self, *_args):
                    pass

            target = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Target)
            threading.Thread(target=target.serve_forever, daemon=True).start()
            stack.callback(target.server_close)
            stack.callback(target.shutdown)
            client_config = root / "client.json"
            client_config.write_text(json.dumps({
                "mixed-port": proxy_port, "bind-address": "127.0.0.1", "allow-lan": False,
                "dns": {"enable": False}, "log-level": "silent",
                "proxies": [{"name": "server", "type": "ss", "server": "127.0.0.1", "port": listener_port,
                             "cipher": "aes-128-gcm", "password": ss_password}],
                "rules": ["MATCH,server"],
            }))
            client_log = stack.enter_context((root / "client.log").open("w+"))
            client = subprocess.Popen([core, "-d", str(root / "client"), "-f", str(client_config)],
                                      stdout=client_log, stderr=client_log, start_new_session=True)
            processes.append(client)

            def check_connection():
                for _ in range(40):
                    try:
                        connection = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=2)
                        connection.request("GET", f"http://127.0.0.1:{target.server_port}/")
                        response = connection.getresponse()
                        content = response.read()
                        connection.close()
                        if response.status == 200 and content == b"3m-ui-real-proxy-ok":
                            return
                    except OSError:
                        pass
                    time.sleep(0.25)
                raise AssertionError("Actual HTTP -> Shadowsocks -> target request failed")

            check_connection()
            # Use the existing authenticated stop API to stop the independently
            # supervised core before ending the panel process.
            api("mihomo/stop", {})
            stop_process(running)
            snapshot = root / "snapshot"
            shutil.copytree(data, snapshot)
            running = start_panel(log)
            token = api("auth/login", {"username": "admin", "password": password})["token"]
            assert any(item["id"] == listener["id"] for item in api("listeners")), "Restart lost node"
            check_connection()
            api("mihomo/stop", {})
            stop_process(running)
            shutil.rmtree(data)
            shutil.copytree(snapshot, data)
            running = start_panel(log)
            token = api("auth/login", {"username": "admin", "password": password})["token"]
            check_connection()
            assert hashlib.sha256(config.read_bytes()).hexdigest() == fingerprint, "Restart changed keys"
            for name, expected in certificates.items():
                assert hashlib.sha256((data / name).read_bytes()).hexdigest() == expected, "Restore changed TLS identity"
            api("mihomo/stop", {})
            stop_process(running)
            print("PASS: init, login/change-password, real Shadowsocks connection, restart, data restore and TLS identity")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    main()
