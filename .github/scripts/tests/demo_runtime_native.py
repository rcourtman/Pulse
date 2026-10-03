#!/usr/bin/env python3
"""Secret-free, fresh public CI only. Never run this fixture on a service host."""
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import pwd
import shutil
import socket
import subprocess
import time

SCRIPTS = Path(__file__).resolve().parents[1]
RESULT = Path.cwd() / "demo-native-result.json"
ROOT = Path("/var/lib/pulse-deploy/demo")
OWNED = [Path("/etc/pulse"), Path("/opt/pulse"), Path("/var/lib/pulse-deploy"),
         Path("/etc/systemd/system/pulse.service"), Path("/etc/systemd/system/pulse-relay.service"),
         Path("/usr/local/share/ca-certificates/pulse-demo-native.crt")]


def command(argv, timeout=120):
    result = subprocess.run(argv, capture_output=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError("fixture-command-failed")
    return result.stdout


def binary(version, fail_after=0, port=17655):
    return ('''#!/usr/bin/python3
import http.server,json,os,threading,time
VERSION = %r
def fail():
    time.sleep(%r)
    print('panic: synthetic disposable service fault',flush=True)
    os._exit(1)
if %r: threading.Thread(target=fail,daemon=True).start()
class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        expected = {'/healthz'} if %r == 17656 else {'/api/health','/api/version'}
        if self.path not in expected:
            self.send_response(404); self.end_headers(); return
        body=json.dumps({'version':VERSION} if self.path=='/api/version' else {'status':'healthy'}).encode()
        self.send_response(200); self.end_headers(); self.wfile.write(body)
    def log_message(self,*args): pass
http.server.HTTPServer(('127.0.0.1',%r),Handler).serve_forever()
''' % (version, fail_after, fail_after, port, port)).encode()


def unit(executable, user="pulse"):
    return f"[Unit]\nDescription=Disposable demo native fixture\n[Service]\nUser={user}\nGroup={user}\nExecStart={executable}\nRestart=no\n[Install]\nWantedBy=multi-user.target\n"


def installer(version, fail_after=0):
    source = binary(version, fail_after)
    return ("#!/usr/bin/python3\nfrom pathlib import Path\n"
            "import base64\n"
            f"p=Path('/opt/pulse/bin/pulse');p.write_bytes(base64.b64decode({base64.b64encode(source).decode()!r}));p.chmod(0o755)\n"
            "p=Path('/etc/systemd/system/pulse.service');p.write_text(p.read_text()+'# candidate fixture\\n')\n"
            f"Path('/etc/pulse/persistent-marker').write_text({'candidate data ' + version!r})\n").encode()


def fixture_installer(version, fail_after=0):
    # The real engine invokes bash like the published installer. This fixture
    # deliberately changes executable, unit and persistent data without download.
    script = installer(version, fail_after)
    return ("#!/bin/bash\nset -eu\npython3 - <<'PY'\n" + script.decode().split("\n", 1)[1] + "\nPY\n").encode()


def wait(predicate, limit):
    deadline = time.monotonic() + limit
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(1)
    raise RuntimeError("fixture-observation-timeout")


def main():
    result = {"schema_version": 1, "control_sha": os.environ.get("GITHUB_SHA"),
              "run_id": os.environ.get("GITHUB_RUN_ID"), "run_attempt": os.environ.get("GITHUB_RUN_ATTEMPT"),
              "cases": [], "cleanup_complete": False, "signed_published_installer_acceptance": False}
    processes = []
    admitted = False
    original_caddy = None
    caddyfile = Path("/etc/caddy/Caddyfile")
    try:
        result["stage"] = "disposable-runner-admission"
        # Environment fields are context, not permission: combine them with a
        # fresh empty estate, effective root, disposable hosted VM and exact job.
        if (os.geteuid() != 0 or os.environ.get("GITHUB_ACTIONS") != "true"
                or os.environ.get("RUNNER_ENVIRONMENT") != "github-hosted"
                or socket.gethostname() in {"pulse-relay", "pulse-license", "pulse-dev"}
                or any(path.exists() or path.is_symlink() for path in OWNED)):
            raise RuntimeError("not-an-empty-disposable-public-ci-runner")
        admitted = True
        result["stage"] = "prepare-native-fixtures"
        original_caddy = caddyfile.read_bytes()
        command(["systemctl", "show", "--property=Version"])
        if not subprocess.run(["id", "pulse"], capture_output=True).returncode == 0:
            command(["useradd", "--system", "--user-group", "--no-create-home", "pulse"])
        owner = pwd.getpwnam("pulse")
        Path("/opt/pulse/bin").mkdir(parents=True)
        Path("/etc/pulse").mkdir(mode=0o755)
        os.chown("/etc/pulse", owner.pw_uid, owner.pw_gid)
        Path("/etc/pulse/.env").write_text("DEMO_MODE=true\n")
        Path("/etc/pulse/billing.json").write_text('{"capabilities":[]}')
        Path("/etc/pulse/persistent-marker").write_text("original data")
        for path in Path("/etc/pulse").iterdir():
            os.chown(path, owner.pw_uid, owner.pw_gid)
        Path("/opt/pulse/bin/pulse").write_bytes(binary("1.0.0")); Path("/opt/pulse/bin/pulse").chmod(0o755)
        Path("/opt/pulse/bin/relay").write_bytes(binary("1.0.0", port=17656)); Path("/opt/pulse/bin/relay").chmod(0o755)
        for name, executable in (("pulse", "/opt/pulse/bin/pulse"), ("pulse-relay", "/opt/pulse/bin/relay")):
            Path(f"/etc/systemd/system/{name}.service").write_text(unit(executable))
        command(["systemctl", "daemon-reload"])
        command(["systemctl", "enable", "--now", "pulse", "pulse-relay"])
        cert = Path("/etc/caddy/demo-native.crt"); key = Path("/etc/caddy/demo-native.key")
        command(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                 "-subj", "/CN=localhost", "-addext", "subjectAltName=IP:127.0.0.1",
                 "-addext", "basicConstraints=critical,CA:TRUE",
                 "-keyout", str(key), "-out", str(cert)])
        key.chmod(0o640); shutil.chown(key, group="caddy")
        shutil.copyfile(cert, OWNED[-1]); command(["update-ca-certificates"])
        caddyfile.write_text("{\n admin off\n auto_https off\n}\n" +
                             f"https://127.0.0.1:18443 {{\n tls {cert} {key}\n reverse_proxy 127.0.0.1:17655\n}}\n" +
                             f"https://127.0.0.1:18444 {{\n tls {cert} {key}\n reverse_proxy 127.0.0.1:17656\n}}\n")
        command(["systemctl", "restart", "caddy"])
        spec = importlib.util.spec_from_file_location("dispatcher", SCRIPTS / "dispatch-demo-runtime.py")
        dispatcher = importlib.util.module_from_spec(spec); spec.loader.exec_module(dispatcher)
        original = (SCRIPTS / "demo-runtime-transaction.py").read_bytes()
        # Only the cohost endpoint is rebound. Transaction, paths, locking,
        # systemd, HTTP/TLS, estate, clocks and watches remain real and unchanged.
        endpoint = b'https://relay.pulserelay.pro/healthz'
        assert original.count(endpoint) == 1
        source = original.replace(endpoint, b'https://127.0.0.1:18444/healthz')
        result["original_engine_sha256"] = hashlib.sha256(original).hexdigest()
        result["fixture_engine_sha256"] = hashlib.sha256(source).hexdigest()
        result["bootstrap_sha256"] = hashlib.sha256(dispatcher.BOOTSTRAP.encode()).hexdigest()
        result["driver_sha256"] = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
        result["tools"] = {"systemd": command(["systemctl", "--version"]).decode().splitlines()[0],
                           "caddy": command(["caddy", "version"]).decode().strip()}
        profile = {name: "2" for name in ("nodes", "vms_per_node", "lxcs_per_node", "docker_hosts", "docker_containers", "generic_hosts", "k8s_clusters", "k8s_nodes", "k8s_pods", "k8s_deployments")}
        profile.update(seed_duration="2h", sample_interval="5m", update_interval="15s")

        def submit(version, fault=0):
            install = fixture_installer(version, fault)
            request = {"mode": "update", "hostname": socket.gethostname(), "local_url": "http://127.0.0.1:17655",
                       "public_url": "https://127.0.0.1:18443/api/health", "version": "v" + version,
                       "profile": profile, "control_sha": os.environ["GITHUB_SHA"],
                       "run_id": os.environ["GITHUB_RUN_ID"], "run_attempt": os.environ["GITHUB_RUN_ATTEMPT"],
                       "installer_sha256": hashlib.sha256(install).hexdigest()}
            payload = {"request": request, "source": base64.b64encode(source).decode(), "installer": base64.b64encode(install).decode()}
            identity = hashlib.sha256(json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
            proc = subprocess.Popen(["python3", "-c", dispatcher.BOOTSTRAP], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            processes.append(proc); proc.stdin.write(json.dumps(payload).encode()); proc.stdin.close()
            return proc, ROOT / "attempts" / identity, payload

        # Readiness is real, including authenticated TLS through actual Caddy.
        result["stage"] = "native-listener-readiness"
        wait(lambda: subprocess.run(["curl", "--disable", "--silent", "--fail", "https://127.0.0.1:18443/api/health"], capture_output=True).returncode == 0, 60)
        proc, attempt, payload = submit("1.0.1")
        result["stage"] = "changed-executable-forward"
        proc.wait(timeout=900)
        receipt = json.loads((attempt / "receipt.json").read_text()); result["cases"].append(receipt)
        assert proc.returncode == 0 and receipt["status"] == "committed" and receipt["forward"]["elapsed_seconds"] >= 300
        assert Path("/etc/pulse/persistent-marker").read_text() == "candidate data 1.0.1"
        # A later candidate fails at 55s; lose the observer while the owned
        # systemd child continues, then TERM during actual full recovery.
        proc, attempt, payload = submit("1.0.2", 55)
        result["stage"] = "delayed-fault-observer-loss"
        wait(lambda: (attempt / "receipt.json").exists() and json.loads((attempt / "receipt.json").read_text())["status"] == "applying", 60)
        proc.terminate(); proc.wait(timeout=30)
        command(["systemctl", "is-active", "pulse-demo-" + attempt.name[:32]])
        wait(lambda: json.loads((attempt / "receipt.json").read_text())["status"] == "recovering", 180)
        result["stage"] = "term-during-owned-recovery"
        command(["systemctl", "kill", "--kill-whom=main", "--signal=TERM", "pulse-demo-" + attempt.name[:32]])
        replay = subprocess.run(["python3", "-c", dispatcher.BOOTSTRAP], input=json.dumps(payload).encode(), capture_output=True, timeout=900)
        receipt = json.loads((attempt / "receipt.json").read_text()); result["cases"].append(receipt)
        assert replay.returncode == 1 and receipt["status"] == "rolled_back" and receipt["recovery"]["elapsed_seconds"] >= 300
        assert receipt["failure"] in {"new-service-crash", "sustained-health"}
        assert Path("/etc/pulse/persistent-marker").read_text() == "candidate data 1.0.1"
        version = json.loads(command(["curl", "--disable", "--silent", "--fail", "http://127.0.0.1:17655/api/version"]))["version"]
        assert version == "1.0.1"
        result["stage"] = "complete"
        result["passed"] = True
    except Exception as error:
        result["passed"] = False
        result["failure_type"] = type(error).__name__
        result["failure_code"] = str(error) if isinstance(error, RuntimeError) else "fixture-observation-or-assertion"
    finally:
        if admitted:
            try:
                for proc in processes:
                    if proc.poll() is None:
                        proc.terminate(); proc.wait(timeout=30)
                # Stop only retained children belonging to this fresh fixture.
                if (ROOT / "attempts").exists():
                    for attempt in (ROOT / "attempts").iterdir():
                        subprocess.run(["systemctl", "stop", "pulse-demo-" + attempt.name[:32]], capture_output=True, timeout=1300)
                command(["systemctl", "disable", "--now", "pulse", "pulse-relay"])
                command(["systemctl", "stop", "caddy"])
                if original_caddy is not None:
                    caddyfile.write_bytes(original_caddy)
                for path in OWNED:
                    if path.is_dir(): shutil.rmtree(path)
                    elif path.exists(): path.unlink()
                for path in (Path("/etc/caddy/demo-native.crt"), Path("/etc/caddy/demo-native.key")):
                    if path.exists(): path.unlink()
                command(["systemctl", "daemon-reload"]); command(["update-ca-certificates", "--fresh"])
                result["cleanup_complete"] = not any(path.exists() for path in OWNED)
            except Exception as cleanup:
                result["cleanup_failure_type"] = type(cleanup).__name__
                result["passed"] = False
        RESULT.write_text(json.dumps(result, indent=2) + "\n")
        RESULT.chmod(0o644)
    return 0 if result.get("passed") and result["cleanup_complete"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
