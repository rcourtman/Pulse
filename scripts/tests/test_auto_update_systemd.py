#!/usr/bin/env python3
"""Discriminating controls for the native CI recipe; no host manager/root use."""
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("native_updater", ROOT / "scripts/installtests/auto_update_systemd.py")
native = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(native)
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
STEP = "Check current-source unattended recovery on native systemd"


def recipe(source):
    job = re.search(r"(?ms)^  frontend:\n(.*?)(?=^  [a-z][a-z0-9-]*:|\Z)", source)[1]
    steps = dict(re.findall(r"(?ms)^      - name: ([^\n]+)\n(.*?)(?=^      - name:|\Z)", job))
    order = list(steps)
    assert order.index("Verify backend with real frontend embed") < order.index(STEP) < order.index("Require frontend dependency audit")
    step = steps[STEP]
    assert not re.search(r"^        (if|continue-on-error|working-directory):", step, re.M)
    assert "runs-on: ubuntu-24.04" in job and "timeout-minutes: 30" in job
    assert "persist-credentials: false" in steps["Checkout repository"]
    assert "EXPECTED_SOURCE_SHA: ${{ github.sha }}" in step
    shell = re.search(r"(?ms)^        run: \|\n((?:          [^\n]*\n)+)", step)[1]
    shell = "\n".join(line[10:] for line in shell.splitlines()) + "\n"
    for command in ('test "$(git rev-parse HEAD)" = "$EXPECTED_SOURCE_SHA"',
                    "diff -qr --no-dereference frontend-modern/dist internal/api/frontend-modern/dist",
                    "-ldflags='-X main.Version=v0.0.1'", "-ldflags='-X main.Version=v0.0.2'",
                    "go build -mod=readonly -p=2", "sudo env -i",
                    "GITHUB_ACTIONS=true RUNNER_ENVIRONMENT=github-hosted GITHUB_REPOSITORY=rcourtman/Pulse",
                    "python3 scripts/installtests/auto_update_systemd.py",
                    '--receipt "$RUNNER_TEMP/pulse-native-updater.json"'):
        assert command in shell, command
    assert "GH_TOKEN" not in shell and "secrets." not in step
    retention = steps["Retain bounded native updater receipt"]
    assert "if: always()" in retention and "retention-days: 7" in retention
    assert "path: ${{ runner.temp }}/pulse-native-updater.json" in retention
    assert "*.log" not in retention and "pulse-native-before" not in retention
    return shell


class NativeContracts(unittest.TestCase):
    def test_definition_boundary_is_exact_and_rejects_drift(self):
        source = (ROOT / "install.sh").read_text()
        definitions = native.installer_definitions(source)
        self.assertEqual(definitions, source[:source.index(native.PARSE_MARKER)])
        for changed in (source.replace(native.PARSE_MARKER, "# moved\n"),
                        source.replace(native.PARSE_MARKER, native.PARSE_MARKER * 2),
                        source.replace(native.PARSE_MARKER, native.PARSE_MARKER + "echo different entry\n")):
            with self.assertRaises(ValueError): native.installer_definitions(changed)

    def test_host_admission_requires_fresh_public_vm_not_worker_or_container(self):
        env = dict(GITHUB_ACTIONS="true", RUNNER_ENVIRONMENT="github-hosted", GITHUB_REPOSITORY="rcourtman/Pulse")
        with patch.object(Path, "exists", return_value=False):
            native.hosted_admission(env, 0, "a" * 40)
            for changed, uid, sha in ((env, 1000, "a" * 40), ({}, 0, "a" * 40),
                                      ({**env, "RUNNER_ENVIRONMENT": "self-hosted"}, 0, "a" * 40),
                                      ({**env, "GITHUB_REPOSITORY": "other/repo"}, 0, "a" * 40),
                                      (env, 0, "main")):
                with self.assertRaises(ValueError): native.hosted_admission(changed, uid, sha)
        with patch.object(Path, "exists", return_value=True):
            with self.assertRaises(ValueError): native.hosted_admission(env, 0, "a" * 40)

    def test_transport_and_private_allocator_have_no_online_fallback(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); (root / "tmp").mkdir()
            script = root / "curl"; script.write_text(native.TRANSPORT); script.chmod(0o755)
            (root / "installer").write_text("signed fixture\n")
            (root / "installer.sshsig").write_text("signature sidecar\n")
            env = {**os.environ, "PULSE_NATIVE_ROOT": str(root)}
            for tag in ("latest/download", "download/v0.0.1", "download/v0.0.2"):
                for suffix in ("", ".sshsig"):
                    result = subprocess.run([str(script), "-fsSL", f"https://github.com/rcourtman/Pulse/releases/{tag}/install.sh{suffix}"], env=env, capture_output=True)
                    self.assertEqual(result.returncode, 0)
                    self.assertEqual(result.stdout, (root / ("installer" + suffix)).read_bytes())
            for url in ("https://example.com/install.sh", "https://github.com/rcourtman/Pulse/releases/download/v6.5.0/install.sh"):
                result = subprocess.run([str(script), "-fsSL", url, "-o", str(root / "unexpected")], env=env, capture_output=True)
                self.assertEqual(result.returncode, 97)
                self.assertFalse((root / "unexpected").exists())
            allocator = root / "mktemp"; allocator.write_text(native.TRANSPORT); allocator.chmod(0o755)
            result = subprocess.run([str(allocator), "-d", "/tmp/pulse-backup.XXXXXX"], env=env, capture_output=True, check=True)
            self.assertEqual(Path(result.stdout.decode().strip()).parent, root / "tmp")

    def test_real_verifier_accepts_fixture_and_refuses_tampering(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lab = native.NativeLab(root, ROOT, root / "a", root / "b", "a" * 40)
            (root / "signer").mkdir()
            lab.run("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(root / "signer/key"))
            installer = root / "installer"; installer.write_text(native.INSTALLER)
            lab.sign(installer)
            shell = ('source "$PULSE_NATIVE_UPDATER"; '
                     'PINNED_RELEASE_SSH_PUBLIC_KEY=$(cat "$PULSE_NATIVE_ROOT/signer/key.pub"); '
                     'verify_release_signature "$PULSE_NATIVE_ROOT/installer" "$PULSE_NATIVE_ROOT/installer.sshsig" fixture')
            self.assertEqual(lab.shell(shell).returncode, 0)
            installer.write_text(native.INSTALLER + "# tampered\n")
            result = subprocess.run(["/bin/bash", "-c", shell], env=lab.env, capture_output=True)
            self.assertEqual(result.returncode, 1)
            self.assertIn(b"Cryptographic signature verification failed", result.stdout)

    def make_phase(self, directory, mutation=""):
        root = Path(directory)
        for name in ("install/bin", "data", "tmp"):
            (root / name).mkdir(parents=True)
        before, after = root / "a", root / "b"
        before.write_bytes(b"baseline executable\n"); after.write_bytes(b"changed executable\n")
        (root / "install/bin/pulse").write_bytes(after.read_bytes())
        (root / "install/VERSION").write_text("v0.0.2\n")
        (root / "data/sentinel").write_text("original\n")
        lab = native.NativeLab(root, ROOT, before, after, "a" * 40)
        lab.baseline_data = {"sentinel": native.digest(root / "data/sentinel")}
        lab.baseline_mode = 0o644
        lab.receipt["binaries"] = {"v0.0.1": native.digest(before), "v0.0.2": native.digest(after)}
        states = [{"LoadState": "loaded", "ActiveState": "inactive", "UnitFileState": "disabled", "MainPID": "0"}] * 2
        if mutation == "mask": states[1] = {**states[1], "LoadState": "masked", "UnitFileState": "masked"}
        if mutation == "activity": states[1] = {**states[1], "ActiveState": "active", "MainPID": "123"}
        lab.state = unittest.mock.Mock(side_effect=states)
        def run(*args, **kwargs):
            if mutation == "persistent": (root / "data/sentinel").write_text("changed\n")
            if mutation == "binary": (root / "install/bin/pulse").write_text("different\n")
            if mutation == "version": (root / "install/VERSION").write_text("v0.0.1\n")
            if mutation == "backup": (root / "tmp/pulse-backup.extra").mkdir()
            if mutation == "staging": (root / "install/bin/pulse.rollback.leak").write_text("leaked\n")
            return subprocess.CompletedProcess(args, 0, b"fixture command, not native execution\n")
        lab.run = run
        return lab

    def test_phase_cannot_accept_wrong_bytes_state_mask_persistence_or_backup(self):
        for mutation in ("", "binary", "version", "mask", "activity", "persistent", "backup", "staging"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                lab = self.make_phase(directory, mutation)
                if mutation:
                    with self.assertRaises(RuntimeError): lab.phase("forward", "v0.0.2", active="inactive")
                    self.assertEqual(lab.receipt["phases"], [])
                else:
                    lab.phase("forward", "v0.0.2", active="inactive")
                    self.assertTrue(lab.receipt["phases"][0]["passed"])

    def test_cleanup_never_deletes_preexisting_unowned_scratch(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); marker = root / "prior"; marker.write_text("not ours\n")
            lab = native.NativeLab(root, ROOT, root / "a", root / "b", "a" * 40)
            with patch.object(lab, "run", side_effect=AssertionError("unowned manager operation")):
                lab.cleanup()
            self.assertTrue(marker.exists())
            self.assertFalse(lab.receipt["cleanup"]["owned_fixture_removed"])

    def run_recipe(self, failure="", mismatch=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); tools = root / "tools"; tools.mkdir()
            for name in ("frontend-modern/dist", "internal/api/frontend-modern/dist"):
                (root / name).mkdir(parents=True); (root / name / "index.html").write_text("real-embed fixture\n")
            trace = root / "trace"
            fake = '''#!/usr/bin/python3
import json, os, pathlib, sys
args = sys.argv[1:]; name = pathlib.Path(sys.argv[0]).name
with pathlib.Path(os.environ["TRACE"]).open("a") as out: out.write(json.dumps([name, *args])+"\\n")
if name == "git": print("b"*40 if os.environ["MISMATCH"] == "true" else "a"*40)
if name == "go":
 if args[args.index("-o")+1].endswith(os.environ["FAIL_BUILD"]) and os.environ["FAIL_BUILD"]: sys.exit(43)
 pathlib.Path(args[args.index("-o")+1]).write_text("compiler double, not a product build\\n")
if name == "sudo":
 if os.environ["FAIL_BUILD"] == "native": sys.exit(45)
 pathlib.Path(os.environ["RUNNER_TEMP"]+"/pulse-native-updater.json").write_text("{}\\n")
'''
            for name in ("git", "go", "sudo"):
                path = tools / name; path.write_text(fake); path.chmod(0o755)
            env = {**os.environ, "PATH": str(tools) + ":" + os.environ["PATH"], "TRACE": str(trace),
                   "RUNNER_TEMP": str(root), "EXPECTED_SOURCE_SHA": "a" * 40,
                   "FAIL_BUILD": failure, "MISMATCH": "true" if mismatch else "false"}
            result = subprocess.run(["/bin/bash", "-c", recipe(WORKFLOW.read_text())], env=env, cwd=root, capture_output=True)
            return result, [json.loads(line) for line in trace.read_text().splitlines()], (root / "pulse-native-updater.json").exists()

    def test_exact_workflow_recipe_is_scrubbed_and_propagates_all_failures(self):
        result, calls, receipt = self.run_recipe()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([call[0] for call in calls], ["git", "go", "go", "sudo"])
        self.assertTrue(receipt)
        self.assertEqual(calls[-1][1:3], ["env", "-i"])
        for failure, code, count in (("before", 43, 2), ("after", 43, 3), ("native", 45, 4)):
            result, calls, receipt = self.run_recipe(failure)
            self.assertEqual(result.returncode, code)
            self.assertEqual(len(calls), count)
            self.assertFalse(receipt)
        result, calls, receipt = self.run_recipe(mismatch=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual([call[0] for call in calls], ["git"])
        self.assertFalse(receipt)

    def test_connected_signed_archive_and_recovery_controls_without_host_manager(self):
        # Same orchestration and production shell functions, real signatures and
        # ELF staging. Only this test replaces the manager, account/chown and
        # unit output path. It is not native or real-Pulse runtime acceptance.
        manager = r'''#!/usr/bin/python3
import os, pathlib, sys
root = pathlib.Path(os.environ["PULSE_NATIVE_ROOT"])
unit = root / "pulse-source-updater.service"
args = sys.argv[1:]
if any(".service" in a and a != "pulse-source-updater.service" for a in args): sys.exit(97)
load = "masked" if unit.is_symlink() else ("loaded" if unit.exists() else "not-found")
statefile = root / "manager-state"
active = statefile.read_text().strip() if statefile.exists() and load != "not-found" else "inactive"
values = dict(LoadState=load, ActiveState=active, UnitFileState="masked" if load == "masked" else "disabled", MainPID="123" if active == "active" else "0")
if args[0] == "show":
 properties = next(a.split("=", 1)[1] for a in args if a.startswith("--property=")).split(",")
 for p in properties: print(values[p] if "--value" in args else p+"="+values[p])
elif args[0] in ("start", "stop"):
 if args[0] == "start" and load != "loaded": sys.exit(1)
 statefile.write_text("active" if args[0] == "start" else "inactive")
 if args[0] == "start" and not (root / "data/.encryption.key").exists():
  (root / "data/.encryption.key").write_text("synthetic persistence marker, not a key\n")
elif args[0] == "mask": unit.symlink_to("/dev/null")
elif args[0] == "unmask": unit.unlink()
elif args[0] not in ("daemon-reload", "reset-failed"): sys.exit(97)
'''
        class FixtureLab(native.NativeLab):
            def assert_runtime(self, tag, active):
                if native.digest(self.root / "install/bin/pulse") != self.receipt["binaries"][tag]:
                    raise RuntimeError("fixture executable mismatch")
                if (self.root / "install/VERSION").read_text() != tag + "\n":
                    raise RuntimeError("fixture version mismatch")
                state = self.state()
                if state["ActiveState"] != active: raise RuntimeError("fixture activity mismatch")
                return state
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory); tools = base / "tools"; tools.mkdir()
            path = tools / "systemctl"; path.write_text(manager); path.chmod(0o755)
            account = r'''#!/usr/bin/python3
import os, pathlib, sys
marker = pathlib.Path(os.environ["PULSE_NATIVE_ROOT"]) / "fixture-user"
if pathlib.Path(sys.argv[0]).name == "id":
 if sys.argv[1:] != ["-u", "pulse"]: sys.exit(97)
 if not marker.exists(): sys.exit(1)
 print("1234")
else:
 if sys.argv[1:] != ["pulse"]: sys.exit(97)
 marker.unlink()
'''
            for name in ("id", "userdel"):
                tool = tools / name; tool.write_text(account); tool.chmod(0o755)
            program = base / "fixture.go"
            program.write_text('package main\nimport "fmt"\nvar Version string\nfunc main(){fmt.Println("Pulse " + Version)}\n')
            binaries = []
            for tag in native.TAGS:
                binary = base / tag
                subprocess.run(["go", "build", "-o", str(binary), "-ldflags=-X main.Version=" + tag, str(program)],
                               check=True, capture_output=True, timeout=60)
                binaries.append(binary)
            lab = FixtureLab(base / "lab", ROOT, *binaries, "a" * 40)
            lab.unit = lab.root / "pulse-source-updater.service"
            lab.env["PATH"] = str(lab.root / "tools") + ":" + str(tools) + ":/usr/sbin:/usr/bin:/sbin:/bin"
            original_write = Path.write_text
            def relocate(path, data, *args, **kwargs):
                if path.name == "installer-definitions":
                    before = "cat > /etc/systemd/system/$SERVICE_NAME.service << EOF"
                    self.assertEqual(data.count(before), 1)
                    data = data.replace(before, 'cat > "$PULSE_NATIVE_ROOT/pulse-source-updater.service" << EOF')
                    data += '\ncreate_user() { touch "$PULSE_NATIVE_ROOT/fixture-user"; }\nchown() { :; }\n'
                return original_write(path, data, *args, **kwargs)
            try:
                with patch.object(Path, "write_text", new=relocate), patch.object(native.time, "sleep"):
                    try:
                        lab.prepare()
                        lab.exercise()
                    except RuntimeError as error:
                        self.fail(str(error) + "\nConfined fixture output:\n" + lab._last_output.decode())
                self.assertEqual([p["name"] for p in lab.receipt["phases"]],
                                 ["forward", "manual-reverse", "failed-active", "failed-inactive",
                                  "failed-masked", "bad-signature", "incomplete-restore"])
                self.assertTrue(lab.receipt["phases"][-1]["backup_retained"])
            finally:
                lab.cleanup()
            self.assertTrue(lab.receipt["cleanup"]["complete"])
            self.assertFalse(lab.root.exists())

    def test_workflow_cannot_hide_or_rebind_native_step(self):
        source = WORKFLOW.read_text()
        for changed in (source.replace("- name: " + STEP + "\n", "- name: " + STEP + "\n        continue-on-error: true\n"),
                        source.replace("sudo env -i", "sudo env"),
                        source.replace("main.Version=v0.0.2", "main.Version=v0.0.1"),
                        source.replace("EXPECTED_SOURCE_SHA: ${{ github.sha }}", "EXPECTED_SOURCE_SHA: main"),
                        source.replace("runs-on: ubuntu-24.04", "runs-on: self-hosted")):
            with self.assertRaises(AssertionError): recipe(changed)


if __name__ == "__main__":
    unittest.main()
