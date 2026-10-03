#!/usr/bin/env python3
"""Demo-only host transaction. Called by the reviewed CI dispatcher, not a worker tool."""
from __future__ import annotations

import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import stat
import subprocess
import sys
import time
from urllib.parse import urlsplit

WINDOW = 300
INTERVAL = 5
READY_TIMEOUT = 60
SERVICES = ("pulse", "pulse-relay", "caddy")
ROOT = Path("/var/lib/pulse-deploy/demo")
LOCK = Path("/var/lib/pulse-deploy/relay/deploy.lock")
PATHS = {"binary": Path("/opt/pulse/bin/pulse"),
         "unit": Path("/etc/systemd/system/pulse.service"),
         "dropins": Path("/etc/systemd/system/pulse.service.d"),
         "data": Path("/etc/pulse")}
RELAY_HEALTH = "https://relay.pulserelay.pro/health"
COUNT_KEYS = ("nodes", "vms_per_node", "lxcs_per_node", "docker_hosts",
              "docker_containers", "generic_hosts", "k8s_clusters", "k8s_nodes",
              "k8s_pods", "k8s_deployments")
DURATION_KEYS = ("seed_duration", "sample_interval", "update_interval")
# Only generated demo operational memory is reset on the already-unhealthy
# recovery route. Its complete original estate remains in the private snapshot.
DEMO_HISTORY = ("ai_incidents.json", "ai_incidents.json.tmp",
                "alerts/alert-history.json", "alerts/alert-history.backup.json",
                "alerts/alert-history.json.imported", "alerts/alert-history.backup.json.imported",
                "alerts/events.db", "alerts/events.db-shm", "alerts/events.db-wal")
CRASH = re.compile(r"\bpanic\b|\bfatal error\b|segmentation fault|core dumped|watchdog timeout", re.I)
TERMINAL = {"committed", "healthy_noop", "rolled_back", "refused"}


class Failure(Exception):
    """Only fixed stage names, never command output or private response bodies."""


def atomic_json(path, value):
    tmp = path.with_suffix(".tmp")
    with open(tmp, "w", encoding="utf-8") as stream:
        os.chmod(tmp, 0o600)
        json.dump(value, stream, sort_keys=True)
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(tmp, path)
    fd = os.open(path.parent, os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def validate(request):
    if set(request) != {"mode", "hostname", "local_url", "public_url", "version", "profile", "control_sha", "run_id", "run_attempt", "installer_sha256"}:
        raise Failure("request-shape")
    if request["mode"] not in {"update", "recover"}:
        raise Failure("request-mode")
    for field, pattern in (("version", r"v[0-9]+\.[0-9]+\.[0-9]+"),
                           ("hostname", r"[a-zA-Z0-9][a-zA-Z0-9.-]{0,252}"),
                           ("control_sha", r"[0-9a-f]{40}"),
                           ("run_id", r"[1-9][0-9]*"), ("run_attempt", r"[1-9][0-9]*")):
        if not isinstance(request[field], str) or not re.fullmatch(pattern, request[field]):
            raise Failure("request-identity")
    for field, public in (("local_url", False), ("public_url", True)):
        try:
            url = urlsplit(request[field])
            valid = (url.scheme == ("https" if public else "http")
                     and url.hostname and not url.username and not url.password
                     and not url.query and not url.fragment
                     and (url.path == "/api/health" if public else url.path in {"", "/"})
                     and (public or url.hostname in {"localhost", "127.0.0.1", "::1"})
                     and 0 < (url.port or (443 if public else 80)) < 65536)
        except (TypeError, ValueError):
            valid = False
        if not valid:
            raise Failure("request-url")
    profile = request["profile"]
    if not isinstance(profile, dict) or set(profile) != set(COUNT_KEYS + DURATION_KEYS):
        raise Failure("request-profile")
    for name, value in profile.items():
        pattern = r"[1-9][0-9]{0,3}" if name in COUNT_KEYS else r"[1-9][0-9]{0,3}[smhd]"
        if not isinstance(value, str) or not re.fullmatch(pattern, value):
            raise Failure("request-profile")
    digest = request["installer_sha256"]
    if (request["mode"] == "update" and not re.fullmatch(r"[0-9a-f]{64}", digest)) or (request["mode"] == "recover" and digest != ""):
        raise Failure("request-installer")


def copy_path(src, dst):
    """Copy without following symlinks, retaining ownership as well as modes."""
    info = src.lstat()
    if stat.S_ISLNK(info.st_mode):
        dst.symlink_to(os.readlink(src))
    elif stat.S_ISDIR(info.st_mode):
        dst.mkdir(mode=0o700)
        for child in src.iterdir():
            copy_path(child, dst / child.name)
        shutil.copystat(src, dst, follow_symlinks=False)
    elif stat.S_ISREG(info.st_mode):
        shutil.copy2(src, dst, follow_symlinks=False)
    else:
        raise Failure("snapshot-special-file")
    os.chown(dst, info.st_uid, info.st_gid, follow_symlinks=False)
    if not stat.S_ISLNK(info.st_mode):
        fd = os.open(dst, os.O_RDONLY | (os.O_DIRECTORY if stat.S_ISDIR(info.st_mode) else 0))
        try:
            os.fsync(fd)
        finally:
            os.close(fd)


def remove_path(path):
    if path.is_symlink() or path.is_file():
        path.unlink()
    elif path.is_dir():
        shutil.rmtree(path)


def estate_hash(paths):
    rows = []
    for label, root in sorted(paths.items()):
        exists = root.exists() or root.is_symlink()
        if not exists:
            rows.append((label, "absent"))
            continue
        for path in [root] + (sorted(root.rglob("*")) if root.is_dir() and not root.is_symlink() else []):
            info = path.lstat()
            kind = stat.S_IFMT(info.st_mode)
            content = os.readlink(path) if stat.S_ISLNK(info.st_mode) else ""
            if stat.S_ISREG(info.st_mode):
                digest = hashlib.sha256()
                with open(path, "rb") as stream:
                    for block in iter(lambda: stream.read(1024 * 1024), b""):
                        digest.update(block)
                content = digest.hexdigest()
            rows.append((label, str(path.relative_to(root)), kind, stat.S_IMODE(info.st_mode), info.st_uid, info.st_gid, content))
    return hashlib.sha256(json.dumps(rows, sort_keys=True).encode()).hexdigest()


class Host:
    now = staticmethod(time.monotonic)
    sleep = staticmethod(time.sleep)

    def command(self, argv, *, timeout=20, env=None):
        try:
            result = subprocess.run(argv, capture_output=True, timeout=timeout, env=env, check=False)
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise Failure("command-unavailable") from exc
        if result.returncode or len(result.stdout) > 1024 * 1024:
            raise Failure("command-failed")
        return result.stdout.decode("utf-8", errors="strict")

    def identity(self, hostname):
        if socket.gethostname() != hostname or os.geteuid() != 0:
            raise Failure("host-identity")
        unit = self.command(["systemctl", "show", "pulse", "--property=FragmentPath,User,Group,ExecStart,DropInPaths", "--no-pager"])
        fields = dict(line.split("=", 1) for line in unit.splitlines() if "=" in line)
        if (fields.get("FragmentPath") != str(PATHS["unit"])
                or fields.get("User") != "pulse" or fields.get("Group") != "pulse"
                or str(PATHS["binary"]) not in fields.get("ExecStart", "")):
            raise Failure("service-identity")
        for name in ("binary", "unit", "data"):
            if PATHS[name].is_symlink() or not PATHS[name].exists():
                raise Failure("estate-identity")
        if PATHS["dropins"].is_symlink():
            raise Failure("dropin-identity")
        for path in fields.get("DropInPaths", "").split():
            if Path(path).parent != PATHS["dropins"] or Path(path).is_symlink():
                raise Failure("unsupported-dropin")
        if self.command(["systemctl", "is-enabled", "pulse"]).strip() != "enabled":
            raise Failure("service-disabled")

    def state(self):
        states = {}
        for service in SERVICES:
            text = self.command(["systemctl", "show", service, "--property=ActiveState,SubState,MainPID,NRestarts", "--no-pager"])
            fields = dict(line.split("=", 1) for line in text.splitlines() if "=" in line)
            if (set(fields) != {"ActiveState", "SubState", "MainPID", "NRestarts"}
                    or not fields["MainPID"].isdigit() or not fields["NRestarts"].isdigit()):
                raise Failure("service-observation")
            states[service] = fields
        return states

    def status(self, url):
        # No redirects, response bodies, credential arguments or ambient curl config.
        try:
            result = self.command(["curl", "--disable", "--silent", "--connect-timeout", "3", "--max-time", "8", "--output", "/dev/null", "--write-out", "%{http_code}", "--", url], timeout=10)
        except Failure:
            return "000"
        return result if re.fullmatch(r"[1-5][0-9]{2}", result) else "000"

    def version(self, url):
        try:
            body = self.command(["curl", "--disable", "--silent", "--fail", "--connect-timeout", "3", "--max-time", "8", "--", url.rstrip("/") + "/api/version"], timeout=10)
            if len(body) > 8192:
                raise Failure("version-observation")
            value = json.loads(body)["version"]
            if not isinstance(value, str) or not re.fullmatch(r"v?[0-9]+\.[0-9]+\.[0-9]+", value):
                raise Failure("version-observation")
            return value.lstrip("v")
        except (ValueError, KeyError, TypeError) as exc:
            raise Failure("version-observation") from exc

    def cursors(self):
        cursors = {}
        for service in SERVICES:
            text = self.command(["journalctl", "-u", service, "-n", "1", "--output=json", "--show-cursor", "--no-pager"])
            matches = re.findall(r"^-- cursor: ([^\s]+)$", text, re.M)
            if len(matches) != 1:
                raise Failure("journal-cursor")
            cursors[service] = matches[0]
        return cursors

    def check_journal(self, cursors):
        for service, cursor in cursors.items():
            text = self.command(["journalctl", "-u", service, "--after-cursor", cursor, "--output=json", "--quiet", "--no-pager"])
            for line in text.splitlines():
                try:
                    entry = json.loads(line)
                    if not isinstance(entry.get("MESSAGE"), str) or not isinstance(entry.get("__CURSOR"), str):
                        raise Failure("journal-observation")
                except (ValueError, TypeError, AttributeError) as exc:
                    raise Failure("journal-observation") from exc
                if CRASH.search(entry["MESSAGE"]):
                    raise Failure("new-service-crash")
                cursors[service] = entry["__CURSOR"]

    def stop(self):
        self.command(["systemctl", "stop", "pulse"], timeout=90)

    def start(self):
        self.command(["systemctl", "daemon-reload"])
        self.command(["systemctl", "start", "pulse"], timeout=90)

    def install(self, attempt, request):
        installer = attempt / "installer.sh"
        if hashlib.sha256(installer.read_bytes()).hexdigest() != request["installer_sha256"]:
            raise Failure("installer-identity")
        self.command(["bash", str(installer), "--version", request["version"]], timeout=900,
                     env={"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "HOME": "/root", "PULSE_SERVICE_NAME": "pulse"})

    def profile_values(self, request):
        values = {"DEMO_MODE": "true", "PULSE_MOCK_MODE": "true", "PULSE_MOCK_RANDOM_METRICS": "true",
                  "PULSE_MOCK_STOPPED_PERCENT": "6", "PULSE_MOCK_SEED_METRICS_STORE": "false"}
        for key, value in request["profile"].items():
            key = {"seed_duration": "trends_seed_duration", "sample_interval": "trends_sample_interval"}.get(key, key)
            values["PULSE_MOCK_" + key.upper()] = value
        return values

    def profile_matches(self, request):
        text = (PATHS["data"] / ".env").read_text().splitlines()
        values = self.profile_values(request)
        return (all(text.count(key + "=" + value) == 1 for key, value in values.items())
                and "demo_fixtures" in json.loads((PATHS["data"] / "billing.json").read_text()).get("capabilities", []))

    def profile(self, request, unhealthy):
        env_file = PATHS["data"] / ".env"
        billing_file = PATHS["data"] / "billing.json"
        if env_file.is_symlink() or billing_file.is_symlink():
            raise Failure("configuration-symlink")
        values = self.profile_values(request)
        text = env_file.read_text()
        for key, value in values.items():
            pattern = re.compile(r"^[ \t]*" + re.escape(key) + r"=.*$", re.M)
            text = pattern.sub(key + "=" + value, text) if pattern.search(text) else text.rstrip("\n") + "\n" + key + "=" + value + "\n"
        env_file.write_text(text)
        data_owner = PATHS["data"].stat()
        os.chown(env_file, data_owner.st_uid, data_owner.st_gid)
        os.chmod(env_file, 0o600)
        billing = json.loads(billing_file.read_text())
        if not isinstance(billing, dict) or not isinstance(billing.get("capabilities", []), list):
            raise Failure("billing-shape")
        billing["capabilities"] = sorted(set(billing.get("capabilities", []) + ["demo_fixtures"]))
        billing.pop("integrity", None)
        billing_file.write_text(json.dumps(billing))
        os.chown(billing_file, data_owner.st_uid, data_owner.st_gid)
        os.chmod(billing_file, 0o600)
        if unhealthy:
            for relative in DEMO_HISTORY:
                path = PATHS["data"] / relative
                # Never follow a writable parent symlink to a path outside the estate.
                if any(p.is_symlink() for p in path.parents if p != PATHS["data"].parent):
                    raise Failure("history-symlink")
                if path.is_symlink():
                    raise Failure("history-symlink")
                if path.is_file():
                    path.unlink()


def active(state):
    return state["ActiveState"] == "active" and state["SubState"] == "running" and int(state["MainPID"]) > 0


class Transaction:
    def __init__(self, host, request, attempt):
        self.host, self.request, self.attempt = host, request, attempt
        self.receipt = {"schema_version": 2, "status": "intent", "mutated": False,
                        "control_sha": request["control_sha"], "run_id": request["run_id"],
                        "run_attempt": request["run_attempt"], "mode": request["mode"],
                        "expected_version": request["version"], "installer_sha256": request["installer_sha256"],
                        "forward": {}, "recovery": {}, "recovery_required": False}

    def save(self, status):
        self.receipt["status"] = status
        atomic_json(self.attempt / "receipt.json", self.receipt)

    def urls(self):
        return (self.request["local_url"].rstrip("/") + "/api/health", self.request["public_url"], RELAY_HEALTH)

    def healthy(self, states=None):
        return all(active(s) for s in (states or self.host.state()).values()) and all(self.host.status(url) == "200" for url in self.urls())

    def ready(self, cursors):
        deadline = self.host.now() + READY_TIMEOUT
        while True:
            self.host.check_journal(cursors)
            if self.healthy():
                return
            if self.host.now() >= deadline:
                raise Failure("listener-readiness")
            self.host.sleep(INTERVAL)

    def watch(self, phase, expected_version, cursors):
        baseline = self.host.state()
        started = self.host.now()
        evidence = self.receipt[phase]
        evidence.update({"required_seconds": WINDOW, "samples": 0, "elapsed_seconds": 0,
                         "initial_services": baseline})
        while True:
            self.host.check_journal(cursors)
            state = self.host.state()
            evidence["last_services"] = state
            if state != baseline or not self.healthy(state):
                raise Failure("sustained-health")
            evidence["observed_version"] = self.host.version(self.request["local_url"])
            if evidence["observed_version"] != expected_version:
                raise Failure("runtime-version")
            evidence["samples"] += 1
            elapsed = self.host.now() - started
            evidence["elapsed_seconds"] = elapsed
            if elapsed >= WINDOW:
                return
            self.host.sleep(min(INTERVAL, WINDOW - elapsed))

    def capture(self):
        snapshot = self.attempt / "snapshot"
        size = sum(path.lstat().st_size for root in PATHS.values() if root.exists()
                   for path in ([root] + (list(root.rglob("*")) if root.is_dir() else []))
                   if path.is_file() and not path.is_symlink())
        # Both our independent estate and the signed installer's own backup need
        # space. Refuse shortage; never delete backups or databases for headroom.
        if any(shutil.disk_usage(parent).free < 2 * size + 64 * 1024 * 1024
               for parent in (self.attempt, PATHS["data"].parent)):
            raise Failure("snapshot-headroom")
        snapshot.mkdir(mode=0o700)
        manifest = {}
        for label, path in PATHS.items():
            manifest[label] = path.exists() or path.is_symlink()
            if manifest[label]:
                copy_path(path, snapshot / label)
        atomic_json(snapshot / "paths.json", manifest)
        digest = estate_hash(PATHS)
        self.receipt["snapshot_sha256"] = digest
        self.receipt["snapshot_retained"] = True
        return digest

    def restore(self):
        snapshot = self.attempt / "snapshot"
        for label, exists in json.loads((snapshot / "paths.json").read_text()).items():
            remove_path(PATHS[label])
            if exists:
                copy_path(snapshot / label, PATHS[label])
        if estate_hash(PATHS) != self.receipt["snapshot_sha256"]:
            raise Failure("restored-estate-identity")

    def run(self):
        old_handlers = {}
        healthy_baseline = False
        stopped = False
        original_version = ""
        states = {}

        def cancel(_signum, _frame):
            raise Failure("cancelled-forward")

        try:
            validate(self.request)
            self.host.identity(self.request["hostname"])
            for entry in ROOT.glob("attempts/*"):
                if entry == self.attempt:
                    continue
                receipt = entry / "receipt.json"
                if not receipt.exists() or json.loads(receipt.read_text()).get("status") not in TERMINAL:
                    raise Failure("prior-recovery-required")
            states = self.host.state()
            if not all(active(states[name]) for name in ("pulse-relay", "caddy")) or self.host.status(RELAY_HEALTH) != "200":
                raise Failure("cohost-unhealthy")
            healthy_baseline = self.healthy(states)
            if healthy_baseline:
                original_version = self.host.version(self.request["local_url"])
                if self.request["mode"] == "recover" and original_version != self.request["version"].lstrip("v"):
                    raise Failure("recovery-source-mismatch")
            elif self.request["mode"] == "update":
                raise Failure("update-baseline-unhealthy")
            self.receipt["healthy_baseline"] = healthy_baseline
            self.receipt["baseline_services"] = states
            self.receipt["baseline_version"] = original_version
            old_handlers = {sig: signal.getsignal(sig) for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP)}
            for sig in old_handlers:
                signal.signal(sig, cancel)
            # The no-op is observed too; an immediate healthy sample is not success.
            if self.request["mode"] == "recover" and healthy_baseline and self.host.profile_matches(self.request):
                self.watch("forward", original_version, self.host.cursors())
                self.save("healthy_noop")
                return 0
            self.save("capturing")
            stopped = True
            self.host.stop()
            self.capture()
            self.save("applying")
            self.receipt["mutated"] = True
            cursors = self.host.cursors()
            if self.request["mode"] == "update":
                self.host.install(self.attempt, self.request)
                # The installer may start the unit; finish profile work quiescent.
                self.host.stop()
            self.host.profile(self.request, not healthy_baseline)
            self.host.start()
            self.ready(cursors)
            self.watch("forward", self.request["version"].lstrip("v"), cursors)
            # No unrelated Relay/Caddy restart is owned by this transaction.
            final = self.host.state()
            for service in ("pulse-relay", "caddy"):
                if final[service] != states[service]:
                    raise Failure("cohost-identity-changed")
            self.save("committed")
            return 0
        except Exception as exc:
            self.receipt["failure"] = str(exc) if isinstance(exc, Failure) else "transaction-observation"
            # Owned restoration, observation and the terminal receipt survive cancellation.
            for sig in old_handlers:
                signal.signal(sig, signal.SIG_IGN)
            if stopped:
                try:
                    self.save("recovering")
                    self.host.stop()
                    if "snapshot_sha256" in self.receipt:
                        self.restore()
                    if not healthy_baseline:
                        self.receipt["recovery_required"] = True
                        self.receipt["rollback"] = "unavailable-unhealthy-baseline"
                        self.save("recovery_required")
                    else:
                        cursors = self.host.cursors()
                        self.host.start()
                        self.ready(cursors)
                        self.watch("recovery", original_version, cursors)
                        final = self.host.state()
                        if any(final[name] != states[name] for name in ("pulse-relay", "caddy")):
                            raise Failure("cohost-identity-changed")
                        self.receipt["rollback"] = "verified"
                        self.save("rolled_back")
                except Exception as recovery:
                    self.receipt["rollback"] = "failed"
                    self.receipt["recovery_failure"] = str(recovery) if isinstance(recovery, Failure) else "recovery-observation"
                    self.receipt["recovery_required"] = True
                    self.save("rollback_failed")
            else:
                self.save("refused")
            return 1
        finally:
            for sig, handler in old_handlers.items():
                signal.signal(sig, handler)


def main():
    # Attempt paths are generated by the fixed CI bootstrap, not CLI overrides.
    if len(sys.argv) != 2:
        return 2
    attempt = Path(sys.argv[1])
    if attempt.parent != ROOT / "attempts" or not re.fullmatch(r"[0-9a-f]{64}", attempt.name) or attempt.is_symlink():
        return 2
    request = json.loads((attempt / "request.json").read_text())
    transaction = Transaction(Host(), request, attempt)
    fd = None
    try:
        LOCK.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        fd = os.open(LOCK, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        return transaction.run()
    except (OSError, ValueError):
        transaction.receipt["failure"] = "host-lock-unavailable"
        transaction.save("refused")
        return 1
    finally:
        if fd is not None:
            os.close(fd)


if __name__ == "__main__":
    sys.exit(main())
