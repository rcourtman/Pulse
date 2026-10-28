#!/usr/bin/env python3
"""Execute the release queue with controlled children, not a second scheduler."""

import csv
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
MODULE = "github.com/rcourtman/pulse-go-rewrite/"
PACKAGES = [MODULE + name for name in (
    "scripts/installtests", "internal/monitoring", "pkg/new-package",
    "cmd/example", "tests/example", "internal/example")]

# All subprocess work is synthetic. The actual shell runner owns discovery,
# API partitioning, CPU leases, command flags, data paths and failure cleanup.
CHILD = r'''#!/usr/bin/env python3
import fcntl, json, os, re, sys, time
from pathlib import Path

root = Path(os.environ["QUEUE_FIXTURE"])
args = sys.argv[1:]
names = ["TestPrefix%02d" % n for n in range(20)] + [
    "TestWebSocketOriginAllowsTrustedForwardedHostedOriginIPv6Loopback",
    "TestServerInfoEndpointMethodNotAllowed", "TestTail"]
if args[:1] == ["list"]:
    if "-f" in args:
        print("github.com/rcourtman/pulse-go-rewrite/internal/api")
    else:
        for name in json.loads((root / "packages.json").read_text()): print(name)
        sys.exit(int(os.environ.get("QUEUE_LIST_EXIT", "0")))
    sys.exit(0)
if args[:2] == ["test", "-c"]:
    target = Path(args[args.index("-o") + 1])
    target.write_bytes(Path(__file__).read_bytes()); target.chmod(0o755)
    sys.exit(0)
if args[:1] == ["build"]: sys.exit(0)
if args[:1] == ["-test.list"]:
    print("\n".join(names)); sys.exit(0)
if args[:1] == ["-test.run"]:
    assert args[2:] == ["-test.timeout", "45m"], args
    selected = [n for n in names if re.fullmatch(args[1], n)]
    assert selected
    name = "api:" + selected[0]
    delay = .6 if selected[0].startswith("TestPrefix") else .15
else:
    assert args[:7] == ["test", "-race", "-p", "1", "-timeout", "30m", args[-1]], args
    assert len(args) == 7 and os.environ["GOMAXPROCS"] == "1"
    selected = []
    name = args[-1]
    # The first ordinary package remains busy after a short API shard ends.
    delay = .7 if name.endswith("scripts/installtests") else .25
assert os.environ["CI"] == os.environ["GITHUB_ACTIONS"] == "true"
def record(event):
    with (root / "children.jsonl").open("a") as f:
        fcntl.flock(f, fcntl.LOCK_EX)
        f.write(json.dumps(dict(event=event, name=name, pid=os.getpid(),
            width=int(os.environ["GOMAXPROCS"]), selected=selected,
            data=os.environ["PULSE_DATA_DIR"], time=time.monotonic())) + "\n")
record("start")
time.sleep(delay)
record("finish")
if os.environ.get("QUEUE_FAIL") in (name, "api" if selected else "other"):
    sys.exit(19)
'''


class ReleaseBackendQueueTest(unittest.TestCase):
    def run_queue(self, cpus=4, packages=None, **extra):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ("scripts", ".github/scripts", "internal/api", "bin"):
                (root / name).mkdir(parents=True)
            for name in ("scripts/run-release-backend-tests.sh", "scripts/shard_go_tests.py",
                         ".github/scripts/select-go-package-shard.py",
                         ".github/scripts/go-package-test-seconds.txt"):
                shutil.copyfile(ROOT / name, root / name)
            assets = root / "scripts/ensure_test_assets.sh"
            assets.write_text("#!/bin/bash\nexit 0\n"); assets.chmod(0o755)
            for name, text in {
                "go": CHILD,
                "git": '#!/bin/bash\nprintf "%s\\n" "$QUEUE_FIXTURE"\n',
                "getconf": f"#!/bin/bash\necho {cpus}\n",
            }.items():
                path = root / "bin" / name
                path.write_text(text); path.chmod(0o755)
            (root / "packages.json").write_text(json.dumps(
                packages if packages is not None else PACKAGES + [MODULE + "internal/api"]))
            result = subprocess.run(
                ["bash", str(root / "scripts/run-release-backend-tests.sh"),
                 "--data-root", str(root / "data"), "--api-shards", str(min(3, cpus))],
                cwd=root, capture_output=True, text=True, timeout=20,
                env={**os.environ, "PATH": str(root / "bin") + ":" + os.environ["PATH"],
                     "QUEUE_FIXTURE": str(root), "PULSE_BACKEND_TEST_SHARD_BOUNDARIES": "",
                     "PULSE_BACKEND_TEST_SHARD_WEIGHTS": "", **extra})
            events = root / "children.jsonl"
            events = [json.loads(line) for line in events.read_text().splitlines()] if events.exists() else []
            coverage = root / "data/release-backend-run/coverage"
            receipts = list(csv.DictReader((coverage / "execution.tsv").read_text().splitlines(), delimiter="\t")) if (coverage / "execution.tsv").exists() else []
            selected = (coverage / "non-api-packages.txt").read_text().splitlines() if (coverage / "non-api-packages.txt").exists() else []
            for event in events:
                if event["event"] == "start":
                    with self.assertRaises(ProcessLookupError, msg="runner left a child alive"):
                        os.kill(event["pid"], 0)
            return result, events, receipts, selected

    def test_complete_source_inventory_reuses_finished_api_width(self):
        for cpus in (1, 4, 8):
            with self.subTest(cpus=cpus):
                result, events, receipts, selected = self.run_queue(cpus)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(set(selected), set(PACKAGES))
                starts = [e for e in events if e["event"] == "start"]
                ordinary = [e for e in starts if not e["name"].startswith("api:")]
                self.assertEqual(sorted(e["name"] for e in ordinary), sorted(PACKAGES))
                self.assertEqual(len({e["data"] for e in ordinary}), len(PACKAGES))
                roots = [n for e in starts for n in e["selected"]]
                self.assertEqual(len(roots), len(set(roots)))
                self.assertEqual(len(roots), 23)
                # Validate the parent's actual lease journal as well as the
                # controlled children's independently recorded live widths.
                used = peak = 0
                for row in receipts:
                    used += int(row["width"]) * (1 if row["event"] == "start" else -1)
                    peak = max(peak, used)
                    self.assertLessEqual(used, cpus)
                    self.assertGreaterEqual(used, 0)
                    if row["event"] == "finish": self.assertEqual(row["exit"], "0")
                self.assertEqual(used, 0)
                self.assertEqual(peak, cpus)
                used = 0
                for event in sorted(events, key=lambda e: e["time"]):
                    used += event["width"] * (1 if event["event"] == "start" else -1)
                    self.assertLessEqual(used, cpus)
                self.assertEqual(used, 0)
                if cpus == 4:
                    prefix_end = next(e["time"] for e in events if e["event"] == "finish" and e["name"].startswith("api:TestPrefix"))
                    # Old scheduling leaves just the first ordinary package
                    # running here; released API slots must admit the tail.
                    self.assertGreater(len([e for e in ordinary if e["time"] < prefix_end]), 1)
                self.assertIn("Race-enabled backend release tests passed", result.stdout)

    def test_failed_partial_or_duplicate_discovery_starts_no_test_child(self):
        for packages, env in [(PACKAGES, {"QUEUE_LIST_EXIT": "23"}),
                              (PACKAGES + [PACKAGES[0]], {}), ([], {})]:
            with self.subTest(packages=packages, env=env):
                result, events, _, _ = self.run_queue(packages=packages, **env)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(events, [])
                self.assertNotIn("Race-enabled backend release tests passed", result.stdout)

    def test_failure_of_either_branch_fails_and_cleans_up(self):
        for branch in ("api", "other"):
            with self.subTest(branch=branch):
                result, _, receipts, _ = self.run_queue(QUEUE_FAIL=branch)
                self.assertEqual(result.returncode, 19, result.stdout + result.stderr)
                self.assertTrue(any(row["exit"] == "19" for row in receipts))
                self.assertNotIn("Race-enabled backend release tests passed", result.stdout)


if __name__ == "__main__":
    unittest.main()
