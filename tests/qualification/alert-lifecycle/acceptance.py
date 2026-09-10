#!/usr/bin/env python3
"""External, synthetic-only installed alert probe. No product imports or DB access."""
import argparse
import datetime
import ipaddress
import json
import os
from pathlib import Path
import subprocess
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

SOURCE = "2dcf23b9d75742049f72f7eac21b0bf7266ebf6e"
DIGEST = "sha256:d7d24aec91da45b901e7b1c4094d508b5f6c708dca114c0f4dbbdfddd86a82b4"
TEMPLATE = ('{"id":"{{.ID}}","resource":"{{.ResourceName}}",'
            '"start":"{{.StartTime}}","event":"{{.Event}}"}')


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def check(condition, message):
    if not condition:
        raise AssertionError(message)


def loopback_url(url):
    parsed = urllib.parse.urlsplit(url)
    check(parsed.scheme == "http" and not parsed.username and not parsed.password
          and not parsed.query and not parsed.fragment, "plain loopback HTTP URL required")
    check(ipaddress.ip_address(parsed.hostname).is_loopback, "non-loopback target refused")
    return url.rstrip("/")


def webhook_start(alert):
    # Published prepareWebhookData uses time.RFC3339, not RFC3339Nano.
    # Keep the full API value for incident queries; only project recipient matching.
    start = datetime.datetime.fromisoformat(alert["startTime"].replace("Z", "+00:00"))
    check(start.tzinfo is not None, "occurrence start lacks timezone")
    return start.isoformat(timespec="seconds").replace("+00:00", "Z")


def distinct_webhook_occurrences(old, current):
    check(webhook_start(old) != webhook_start(current),
          "occurrences indistinguishable at webhook timestamp precision")


def report(name, cpu):
    return {"agent": {"id": name, "type": "unified", "intervalSeconds": 5,
                      "commandsEnabled": False},
            "host": {"hostname": name, "platform": "linux", "cpuCount": 1},
            "metrics": {"cpuUsagePercent": cpu,
                        "memory": {"totalBytes": 1073741824, "usedBytes": 104857600,
                                   "freeBytes": 968884224, "usage": 10}},
            "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat()}


class Evidence:
    def __init__(self, directory):
        self.directory = Path(directory)
        self.directory.mkdir(mode=0o700, parents=True, exist_ok=False)
        self.lock = threading.Lock()
        self.counter = 0

    def save(self, label, data):
        with self.lock:
            self.counter += 1
            path = self.directory / ("%04d-%s.json" % (self.counter, label))
            path.write_text(json.dumps(data, indent=2) + "\n")


class Recipient:
    def __init__(self, evidence):
        self.evidence = evidence
        self.lock = threading.Lock()
        self.failures = 0
        self.receipts = []

    def mode(self, failures):
        with self.lock:
            self.failures = failures  # -1 fails indefinitely

    def accept(self, payload):
        with self.lock:
            status = 503 if self.failures else 200
            if self.failures > 0:
                self.failures -= 1
            record = {"monotonic": time.monotonic(), "status": status, "payload": payload}
            self.receipts.append(record)
            self.evidence.save("recipient", record)
            return status

    def matching(self, name, status=200):
        with self.lock:
            return [r for r in self.receipts if r["status"] == status
                    and r["payload"].get("resource") == name
                    and r["payload"].get("event") == "alert"]


def serve(recipient, port):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            if self.path != "/receipt":
                self.send_error(404)
                return
            size = int(self.headers.get("Content-Length", "0"))
            if not 0 < size <= 65536:
                self.send_error(413)
                return
            try:
                payload = json.loads(self.rfile.read(size))
                check(isinstance(payload, dict), "object required")
            except (ValueError, AssertionError):
                self.send_error(400)
                return
            status = recipient.accept(payload)
            self.send_response(status)
            if status == 503:
                self.send_header("Retry-After", "0")
            self.end_headers()
            self.wfile.write(b"controlled recipient\n")

    server = ThreadingHTTPServer(("127.0.0.1", port), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server


class Driver:
    def __init__(self, args, evidence, recipient):
        self.args, self.evidence, self.recipient = args, evidence, recipient
        self.base = loopback_url(args.base)
        self.token = os.environ["PULSE_ACCEPTANCE_TOKEN"]
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        self.names = ["acceptance-" + args.run_id + "-" + x for x in ("retry", "terminal", "quiet")]
        self.results = []
        self.record_observations = False

    def api(self, path, body=None, method=None):
        data = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(self.base + path, data=data, method=method,
                                     headers={"X-API-Token": self.token,
                                              "Content-Type": "application/json"})
        try:
            with self.opener.open(req, timeout=10) as response:
                result = json.load(response)
        except urllib.error.HTTPError as error:
            # Do not retain error bodies that might echo authentication material.
            self.evidence.save("http-error", {"path": path, "status": error.code})
            raise RuntimeError("HTTP %s on %s" % (error.code, path)) from None
        if self.record_observations and path.startswith((
                "/api/alerts/active", "/api/alerts/history", "/api/alerts/incidents",
                "/api/notifications/delivery-log", "/api/notifications/health",
                "/api/notifications/queue/stats")):
            self.evidence.save("observation", {"path": path, "response": result})
        return result

    def snapshot(self, label, alert=None):
        data = {path: self.api("/api/" + path) for path in
                ("alerts/active", "alerts/history", "notifications/delivery-log?limit=200",
                 "notifications/health", "notifications/queue/stats")}
        if alert:
            query = urllib.parse.urlencode({"alertIdentifier": alert["id"],
                                            "started_at": alert["startTime"]})
            data["incident"] = self.api("/api/alerts/incidents?" + query)
        self.evidence.save(label, data)
        return data

    def wait(self, label, predicate, timeout=90, tick=None):
        deadline = time.monotonic() + timeout
        while True:
            if tick:
                tick()
            result = predicate()
            if result:
                self.results.append(label)
                self.evidence.save("assertion", {"check": label, "result": "pass"})
                return result
            if time.monotonic() >= deadline:
                raise TimeoutError(label)
            time.sleep(2)

    def ingest(self, name, cpu):
        response = self.api("/api/agents/agent/report", report(name, cpu))
        check(response.get("success") is True, "ingest rejected")
        check(response.get("serverVersion", "").lstrip("v") == "6.4.4-beta.2",
              "installed server version mismatch")

    def active(self, name):
        return next((a for a in (self.api("/api/alerts/active") or [])
                     if a["resourceName"] == name and a["type"] == "cpu"), None)

    def delivery(self, alert, outcome):
        entries = self.api("/api/notifications/delivery-log?limit=200")["entries"] or []
        return [e for e in entries if alert["id"] in (e["alertIds"] or [])
                and e["outcome"] == outcome]

    def received_occurrence(self, name, alert):
        return any(r["payload"].get("id") == alert["id"]
                   and r["payload"].get("start") == webhook_start(alert)
                   for r in self.recipient.matching(name))

    def retry_terminal(self, alert, terminal_ids):
        retry = self.api("/api/notifications/terminal-failures/retry", {})
        # Preserve the returned result even when the assertion fails.
        self.evidence.save("retry-response", retry)
        check(retry.get("affected", 0) >= 1, "no terminal retry admitted")
        self.wait("terminal-retry-sent", lambda: any(
            entry["notificationId"] in terminal_ids
            for entry in self.delivery(alert, "sent")))

    def run(self):
        # Operator supplies independent immutable image inspection, not an API version claim.
        identity = json.loads(Path(self.args.identity).read_text())
        check(identity["source"] == SOURCE and identity["image_digest"] == DIGEST,
              "wrong inspected artifact")
        self.evidence.save("identity-owner-attestation", identity)
        check(not self.api("/api/notifications/webhooks"), "fixture already has webhooks")
        check(not self.api("/api/alerts/active"), "fixture already has alerts")
        check(not self.api("/api/alerts/history"), "fixture already has alert history")
        check(not self.api("/api/notifications/delivery-log")["entries"], "fixture has deliveries")
        for channel in ("email", "apprise"):
            check(not self.api("/api/notifications/" + channel).get("enabled"),
                  "fixture has enabled external destination")
        self.record_observations = True
        config = self.api("/api/alerts/config")
        config.update(enabled=True, activationState="active", disableAllAgents=False,
                      disableAllAgentsOffline=True, flappingEnabled=False,
                      agentDefaults={"cpu": {"trigger": 80, "clear": 60}},
                      timeThresholds={"agent": 0},
                      metricTimeThresholds={"agent": {"cpu": 0}},
                      metricEvaluationWindows={"agent": {"cpu": 0}})
        config["schedule"] = {"initialNotify": "webhook", "cooldown": 0,
                              "maxAlertsHour": 60, "notifyOnResolve": False,
                              "grouping": {"enabled": False},
                              "quietHours": {"enabled": False},
                              "escalation": {"enabled": False}}
        self.api("/api/alerts/config", config, "PUT")
        self.api("/api/notifications/webhooks",
                 {"id": "acceptance-" + self.args.run_id, "name": "Synthetic acceptance",
                  "url": "http://127.0.0.1:%d/receipt" % self.args.port,
                  "enabled": True, "method": "POST", "service": "generic",
                  "template": TEMPLATE})
        first, terminal, quiet = self.names
        self.recipient.mode(1)
        alert = self.wait("firing", lambda: self.active(first),
                          tick=lambda: self.ingest(first, 95))
        self.wait("ordinary-recipient-after-transient", lambda: self.recipient.matching(first))
        check(self.recipient.matching(first, 503), "transient failure not observed")
        self.wait("sent-audit", lambda: self.delivery(alert, "sent"))
        self.snapshot("firing-transient", alert)

        self.recipient.mode(-1)
        old = self.wait("terminal-firing", lambda: self.active(terminal),
                        tick=lambda: self.ingest(terminal, 95))
        terminal_entries = self.wait("exhaustion", lambda: self.delivery(old, "dead_letter"),
                  timeout=600, tick=lambda: self.ingest(terminal, 95))
        terminal_ids = {entry["notificationId"] for entry in terminal_entries}
        before = self.snapshot("before-restart", old)
        # This hook belongs to Delivery and must restart only the disposable server.
        proc = subprocess.run([self.args.restart_hook], timeout=120, capture_output=True)
        check(proc.returncode == 0, "restart hook failed (retain owner logs separately)")
        receipt = json.loads(proc.stdout)
        check(receipt["before_process"] != receipt["after_process"]
              and receipt["same_volume"] is True and receipt["image_digest"] == DIGEST,
              "restart owner receipt does not establish changed process / preserved volume")
        self.evidence.save("restart-owner-attestation", receipt)
        # Hook must return only after authenticated readiness; an error is not retried away.
        after = self.snapshot("after-restart", old)
        check(after["incident"]["id"] == before["incident"]["id"], "incident changed at restart")
        check(self.delivery(old, "dead_letter"), "terminal audit lost at restart")
        self.results.append("restart-preserved-terminal-and-incident")

        # Retry while the occurrence is still firing. Resolution intentionally
        # cancels obsolete terminal firing rows; a later recurrence cannot revive them.
        self.recipient.mode(0)
        self.retry_terminal(old, terminal_ids)
        self.wait("terminal-retry-recipient", lambda: self.received_occurrence(terminal, old))
        self.snapshot("old-occurrence-after-retry", old)
        self.wait("resolved", lambda: not self.active(terminal),
                  tick=lambda: self.ingest(terminal, 10))
        current = self.wait("recurrence", lambda: self.active(terminal),
                            tick=lambda: self.ingest(terminal, 95))
        check(current["startTime"] != old["startTime"], "recurrence reused old start")
        distinct_webhook_occurrences(old, current)
        self.wait("recurrence-recipient", lambda: self.received_occurrence(terminal, current))
        self.snapshot("old-occurrence-after-recurrence", old)
        self.snapshot("current-occurrence-after-recurrence", current)
        # Occurrence-side-effect correctness requires review of both retained timelines.
        config["disableAllAgents"] = True
        self.api("/api/alerts/config", config, "PUT")
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            self.ingest(quiet, 95)
            check(not self.active(quiet), "disabled-agent alert fired")
            check(not self.recipient.matching(quiet), "disabled-agent notification delivered")
            time.sleep(2)
        self.results.append("disabled-agent-suppression-30s")
        config["disableAllAgents"] = False
        self.api("/api/alerts/config", config, "PUT")
        self.recipient.mode(-1)
        dismissed = self.wait("dismissal-firing", lambda: self.active(quiet),
                              tick=lambda: self.ingest(quiet, 95))
        self.wait("dismissal-exhaustion", lambda: self.delivery(dismissed, "dead_letter"),
                  timeout=600, tick=lambda: self.ingest(quiet, 95))
        saved = self.snapshot("before-dismiss", dismissed)
        dismissal = self.api("/api/notifications/terminal-failures/dismiss", {})
        check(dismissal.get("affected", 0) >= 1, "nothing dismissed")
        self.evidence.save("dismiss-response", dismissal)
        self.wait("dismissal-health", lambda: self.api("/api/notifications/health")["overall_healthy"])
        retained = self.snapshot("after-dismiss", dismissed)
        def audit_keys(snapshot):
            return {(e["notificationId"], e["timestamp"], e["outcome"])
                    for e in snapshot["notifications/delivery-log?limit=200"]["entries"]}
        check(audit_keys(saved) <= audit_keys(retained), "dismissal lost delivery history")
        self.results.append("dismissal-retains-audit")
        self.snapshot("final")
        return {"passed": self.results, "installed_acceptance_complete": False,
                "requires_review": ["occurrence-specific replay side effects",
                                    "system notification-warning retirement",
                                    "email/provider acceptance", "reporter acceptance"]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--identity", required=True)
    parser.add_argument("--restart-hook", required=True)
    parser.add_argument("--evidence", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--port", type=int, default=18765)
    parser.add_argument("--disposable-fixture", action="store_true", required=True)
    args = parser.parse_args()
    check(args.run_id.isalnum() and len(args.run_id) <= 24, "short alphanumeric run ID required")
    loopback_url(args.base)
    os.umask(0o077)
    evidence = Evidence(args.evidence)
    recipient = Recipient(evidence)
    server = serve(recipient, args.port)
    driver = None
    try:
        driver = Driver(args, evidence, recipient)
        result = driver.run()
        evidence.save("result", result)
    except Exception as error:
        evidence.save("result", {"installed_acceptance_complete": False,
                                 "passed": driver.results if driver else [],
                                 "failure_type": type(error).__name__})
        raise
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main()
