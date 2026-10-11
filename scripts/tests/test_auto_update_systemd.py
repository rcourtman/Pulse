#!/usr/bin/env python3
"""Discriminating controls for the native CI recipe; no host manager/root use."""
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("native_updater", ROOT / "scripts/installtests/auto_update_systemd.py")
native = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(native)
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
STEP = "Check current-source unattended recovery on native systemd"
FIXTURE_COMMANDS = ("bash", "tar", "chmod", "mv", "timeout")
LINUX_CONTROL = "test_connected_signed_archive_and_recovery_controls_without_host_manager"
LINUX_ONLY_REASON = "Linux server ELF execution; portable controls remain required on macOS"


def selected_fixture_commands():
    # The containing native job has already supplied its Linux-only tools.
    # Capture only these executables, not the whole ambient PATH or environment.
    commands = {name: shutil.which(name) for name in FIXTURE_COMMANDS}
    for name, executable in commands.items():
        if executable is None:
            raise ValueError("missing installer fixture tool: " + name)
    return commands


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
                    "git diff --quiet HEAD",
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
    def test_fixture_command_selection_refuses_missing_tools_without_fallback(self):
        for missing in FIXTURE_COMMANDS:
            with self.subTest(missing=missing), patch.object(shutil, "which", side_effect=
                    lambda name: None if name == missing else "/supplied/" + name):
                with self.assertRaisesRegex(ValueError, "missing installer fixture tool: " + missing):
                    selected_fixture_commands()

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

    def test_signer_failure_retains_operation_and_metadata_not_private_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "signer").mkdir(mode=0o700)
            lab = native.NativeLab(root, ROOT, root / "a", root / "b", "a" * 40)
            lab.signer_command("generate-ephemeral-key", "-q", "-t", "ed25519", "-N", "", "-f",
                               str(root / "signer/key"))
            target = root / "installer"
            target.write_text("closed synthetic fixture, not a release\n")
            # A real OpenSSH refusal, not a simulated exit code or key read.
            (root / "signer/key").chmod(0o644)
            with self.assertRaisesRegex(RuntimeError, "native command failed"):
                lab.sign(target)
            failure = lab.receipt["command_failure"]
            self.assertEqual(failure["operation"], "sign-installer")
            self.assertEqual(failure["observed_exit"], 255)
            self.assertIn("private-key-permissions", failure["observed_diagnostics"])
            self.assertEqual(failure["signer_key_metadata"], {
                "present": True, "regular_file": True, "mode": "0644", "owned_by_executor": True,
            })
            self.assertFalse(Path(str(target) + ".sshsig").exists())
            retained = json.dumps(lab.receipt)
            self.assertNotIn("PRIVATE KEY", retained)
            self.assertNotIn(str(root), retained)
            self.assertNotIn(lab._last_output.decode(), retained)

    def test_prepare_owns_generated_signer_mode_before_first_real_signature(self):
        # Reproduce the exact observed mode, not its unknown generation cause.
        # Run real OpenSSH signing before stopping ahead of account/unit writes.
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            before, after = base / "a", base / "b"
            before.write_bytes(b"before\n"); after.write_bytes(b"after\n")
            lab = native.NativeLab(base / "lab", ROOT, before, after, "a" * 40)
            lab.unit = base / "absent.service"
            original_run = lab.run
            def confined_run(*args, **kwargs):
                if args[0] in (str(before), str(after)) and args[1:] == ("--version",):
                    tag = native.TAGS[0 if args[0] == str(before) else 1]
                    return subprocess.CompletedProcess(args, 0, ("Pulse " + tag + "\n").encode())
                if args[0] != "ssh-keygen":
                    raise AssertionError("unexpected service/account operation")
                result = original_run(*args, **kwargs)
                if args[1] == "-q":
                    (lab.root / "signer/key").chmod(0o644)
                return result
            def first_signature(archive):
                self.assertEqual((lab.root / "signer/key").stat().st_mode & 0o777, 0o600)
                native.NativeLab.sign(lab, archive)
                self.assertTrue(Path(str(archive) + ".sshsig").is_file())
                raise RuntimeError("stop before account/unit mutation")
            try:
                with patch.object(lab, "state", return_value={"LoadState": "not-found"}), \
                     patch.object(lab, "run", side_effect=confined_run), \
                     patch.object(lab, "sign", side_effect=first_signature):
                    with self.assertRaisesRegex(RuntimeError, "stop before account/unit mutation"):
                        lab.prepare()
                    self.assertFalse(lab.owned_unit)
                    self.assertFalse(lab.owned_user)
            finally:
                lab.cleanup()
            self.assertTrue(lab.receipt["cleanup"]["complete"])

    def test_signer_creation_refuses_adoption_wrong_type_owner_or_failed_mode(self):
        for mutation in ("existing", "symlink", "generated-symlink", "directory", "owner", "chmod", "mode-unchanged"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                root = Path(directory); (root / "signer").mkdir(mode=0o700)
                key = root / "signer/key"
                marker = root / "keep"; marker.write_text("not a key; not ours\n")
                lab = native.NativeLab(root, ROOT, root / "a", root / "b", "a" * 40)
                if mutation == "existing": key.write_text("not a key; not ours\n")
                if mutation == "symlink": key.symlink_to(marker)
                def generate(*args):
                    if mutation == "generated-symlink": key.symlink_to(marker)
                    elif mutation == "directory": key.mkdir()
                    else: key.write_text("mode/type fixture; not a key\n")
                with patch.object(lab, "signer_command", side_effect=generate) as command, \
                     patch.object(native.os, "geteuid", return_value=-1 if mutation == "owner" else os.getuid()), \
                     patch.object(Path, "chmod", side_effect=OSError("mode change unavailable") if mutation == "chmod" else None):
                    with self.assertRaises((ValueError, OSError)):
                        lab.create_signer()
                    if mutation in ("existing", "symlink"): command.assert_not_called()
                    else: command.assert_called_once()
                self.assertEqual(marker.read_text(), "not a key; not ours\n")

    def test_signer_timeout_does_not_attribute_stale_output_to_new_command(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lab = native.NativeLab(root, ROOT, root / "a", root / "b", "a" * 40)
            lab._last_output = b"UNPROTECTED PRIVATE KEY FILE from an earlier command"
            def timeout(*args):
                lab.receipt["command_failure"] = {"command": "ssh-keygen", "deadline_exceeded": True,
                                                  "observed_exit": -9}
                raise RuntimeError("native command deadline exceeded")
            with patch.object(lab, "run", side_effect=timeout), self.assertRaises(RuntimeError):
                lab.signer_command("generate-ephemeral-key", "-q")
            failure = lab.receipt["command_failure"]
            self.assertEqual(failure["operation"], "generate-ephemeral-key")
            self.assertEqual(failure["signer_key_metadata"], {"present": False})
            self.assertNotIn("observed_diagnostics", failure)

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

    def test_managed_process_identity_follows_served_readiness_and_rejects_residuals(self):
        for mutation in ("", "process-bytes", "stopped-after-readiness", "inactive-pid"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                lab = self.make_phase(directory)
                state = dict(LoadState="loaded", ActiveState="active", UnitFileState="disabled", MainPID="1")
                observed = {**state, "MainPID": "2"}
                if mutation == "stopped-after-readiness":
                    observed.update(ActiveState="inactive", MainPID="0")
                if mutation == "inactive-pid":
                    lab.state = unittest.mock.Mock(return_value={**state, "ActiveState": "inactive"})
                    with self.assertRaises(RuntimeError): lab.assert_runtime("v0.0.2", "inactive")
                    continue
                lab.state = unittest.mock.Mock(side_effect=[state, observed])
                order = []
                def response(url, **kwargs):
                    endpoint = url.rsplit("/", 1)[-1]
                    order.append(endpoint)
                    return io.BytesIO(json.dumps({"status": "healthy"} if endpoint == "health"
                                                 else {"version": "v0.0.2"}).encode())
                actual_digest = native.digest
                def process_digest(path):
                    if str(path).startswith("/proc/"):
                        order.append("process")
                        self.assertEqual(str(path), "/proc/2/exe")
                        return "wrong" if mutation == "process-bytes" else lab.receipt["binaries"]["v0.0.2"]
                    return actual_digest(path)
                with patch.object(native.urllib.request, "urlopen", side_effect=response), patch.object(native, "digest", side_effect=process_digest):
                    if mutation:
                        with self.assertRaises(RuntimeError): lab.assert_runtime("v0.0.2", "active")
                    else:
                        self.assertEqual(lab.assert_runtime("v0.0.2", "active"), observed)
                self.assertEqual(order[:2], ["health", "version"])
                self.assertEqual(order[2:], [] if mutation == "stopped-after-readiness" else ["process"])

    def test_cleanup_never_deletes_preexisting_unowned_scratch(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); marker = root / "prior"; marker.write_text("not ours\n")
            lab = native.NativeLab(root, ROOT, root / "a", root / "b", "a" * 40)
            with patch.object(lab, "run", side_effect=AssertionError("unowned manager operation")):
                lab.cleanup()
            self.assertTrue(marker.exists())
            self.assertFalse(lab.receipt["cleanup"]["owned_fixture_removed"])

    def test_prepare_allocates_scratch_before_child_home_initialisation(self):
        # Model a child runtime that eagerly initialises HOME. This catches the
        # macOS control's FileExistsError without pretending to run systemd.
        for load in ("not-found", "loaded"):
            with self.subTest(load=load), tempfile.TemporaryDirectory() as directory:
                base = Path(directory)
                before, after = base / "a", base / "b"
                before.write_bytes(b"before\n"); after.write_bytes(b"after\n")
                lab = native.NativeLab(base / "lab", ROOT, before, after, "a" * 40)
                lab.unit = base / "absent.service"
                def child_observation():
                    lab.root.mkdir(exist_ok=True)
                    (lab.root / "child-home-marker").write_text("runtime initialised HOME\n")
                    return {"LoadState": load}
                def bounded_command(*args, **kwargs):
                    if args[0] in (str(before), str(after)) and args[1:] == ("--version",):
                        tag = native.TAGS[0 if args[0] == str(before) else 1]
                        return subprocess.CompletedProcess(args, 0, ("Pulse " + tag + "\n").encode())
                    raise RuntimeError("stop before signer or service mutation")
                with patch.object(lab, "state", side_effect=child_observation) as observation, \
                     patch.object(lab, "run", side_effect=bounded_command):
                    expected = ValueError if load == "loaded" else RuntimeError
                    with self.assertRaises(expected):
                        lab.prepare()
                    observation.assert_called_once()
                    self.assertTrue(lab.owned_root)
                    self.assertTrue((lab.root / "child-home-marker").exists())
                    self.assertFalse(lab.owned_unit)
                    self.assertFalse(lab.owned_user)
                    lab.cleanup()
                self.assertFalse(lab.root.exists())
                self.assertTrue(lab.receipt["cleanup"]["complete"])

    def test_prepare_refuses_existing_directory_or_symlink_before_child(self):
        for symlink in (False, True):
            with self.subTest(symlink=symlink), tempfile.TemporaryDirectory() as directory:
                base = Path(directory); prior = base / "prior"; prior.mkdir()
                marker = prior / "keep"; marker.write_text("not ours\n")
                root = base / "lab" if symlink else prior
                if symlink:
                    root.symlink_to(prior, target_is_directory=True)
                lab = native.NativeLab(root, ROOT, base / "a", base / "b", "a" * 40)
                lab.unit = base / "absent.service"
                with patch.object(lab, "state", side_effect=AssertionError("unowned child")) as observation:
                    with self.assertRaises(FileExistsError):
                        lab.prepare()
                    observation.assert_not_called()
                    self.assertFalse(lab.owned_root)
                    lab.cleanup()
                self.assertEqual(marker.read_text(), "not ours\n")
                self.assertTrue(root.exists())
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
if name == "git":
 if args == ["rev-parse", "HEAD"]: print("b"*40 if os.environ["MISMATCH"] == "true" else "a"*40)
 elif args == ["diff", "--quiet", "HEAD"]:
  if os.environ["FAIL_BUILD"] == "source-modified": sys.exit(49)
 else: sys.exit(97)
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
        self.assertEqual([call[0] for call in calls], ["git", "git", "go", "go", "sudo"])
        self.assertTrue(receipt)
        self.assertEqual(calls[-1][1:3], ["env", "-i"])
        for failure, code, count in (("source-modified", 49, 2), ("before", 43, 3), ("after", 43, 4), ("native", 45, 5)):
            result, calls, receipt = self.run_recipe(failure)
            self.assertEqual(result.returncode, code)
            self.assertEqual(len(calls), count)
            self.assertFalse(receipt)
        result, calls, receipt = self.run_recipe(mismatch=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual([call[0] for call in calls], ["git"])
        self.assertFalse(receipt)

    def test_connected_linux_control_is_required_without_macho_or_validator_bypass(self):
        connected = getattr(NativeContracts, LINUX_CONTROL)
        self.assertEqual(getattr(connected, "__unittest_skip__", False), not sys.platform.startswith("linux"))
        # Import just the definitions as each platform would. No fixture is run
        # under a false platform identity, and no production function is changed.
        for platform in ("linux", "darwin"):
            spec = importlib.util.spec_from_file_location("platform_contracts", Path(__file__))
            module = importlib.util.module_from_spec(spec)
            with patch.object(sys, "platform", platform): spec.loader.exec_module(module)
            control = getattr(module.NativeContracts, LINUX_CONTROL)
            self.assertEqual(getattr(control, "__unittest_skip__", False), platform != "linux")
            for name in unittest.defaultTestLoader.getTestCaseNames(module.NativeContracts):
                if name != LINUX_CONTROL:
                    self.assertFalse(getattr(getattr(module.NativeContracts, name), "__unittest_skip__", False), name)

    @unittest.skipUnless(sys.platform.startswith("linux"), LINUX_ONLY_REASON)
    def test_connected_signed_archive_and_recovery_controls_without_host_manager(self):
        # Same orchestration and production shell functions, real signatures and
        # ELF staging. Only this test replaces the manager, account/chown and
        # unit output path. It is not native or real-Pulse runtime acceptance.
        manager = r'''#!/usr/bin/python3
import os, pathlib, sys
root = pathlib.Path(os.environ["PULSE_NATIVE_ROOT"])
# Tool startup must have an existing owned home, not create the lab before
# NativeLab has admitted and claimed its scratch root (as on macOS).
if not pathlib.Path(os.environ["HOME"]).is_dir(): sys.exit(96)
unit = root / "pulse-source-updater.service"
args = sys.argv[1:]
if any(".service" in a and a != "pulse-source-updater.service" for a in args): sys.exit(97)
load = "masked" if unit.is_symlink() else ("loaded" if unit.exists() else "not-found")
statefile = root / "manager-state"
active = statefile.read_text().strip() if statefile.exists() and load != "not-found" else "inactive"
values = dict(LoadState=load, ActiveState=active, UnitFileState="masked" if load == "masked" else "disabled", MainPID="123" if active == "active" else "0")
if args[0] == "show":
 properties = [p for a in args if a.startswith("--property=") for p in a.split("=", 1)[1].split(",")]
 if not properties or len(properties) != len(set(properties)) or any(p not in values for p in properties): sys.exit(97)
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
            def run(self, *args, **kwargs):
                if args[0] == "/bin/bash":
                    args = (str(tools / "bash"), *args[1:])
                return super().run(*args, **kwargs)

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
            selected = selected_fixture_commands()
            tool_trace = base / "selected-tools.jsonl"
            for name, executable in selected.items():
                # Record only tool identity, never shell arguments or key/log
                # bytes. Execute the actual supplied GNU utility, not a double.
                tool = tools / name
                tool.write_text(f'''#!{sys.executable}
import json, os, pathlib, sys
with pathlib.Path({str(tool_trace)!r}).open('a') as receipt:
    receipt.write(json.dumps({name!r}) + '\\n')
os.execv({executable!r}, [{executable!r}, *sys.argv[1:]])
''')
                tool.chmod(0o700)
            path = tools / "systemctl"; path.write_text(manager); path.chmod(0o755)
            account = r'''#!/usr/bin/python3
import os, pathlib, sys
marker = pathlib.Path(os.environ["PULSE_NATIVE_ROOT"]) / "fixture-user"
if pathlib.Path(sys.argv[0]).name == "id":
 if sys.argv[1:] != ["-u", "pulse"]: sys.exit(97)
 if not marker.exists(): sys.exit(1)
 print("1234")
elif pathlib.Path(sys.argv[0]).name == "useradd":
 if sys.argv[1:] != ["--system", "--home-dir", str(marker.parent / "install"), "--shell", "/bin/false", "pulse"]: sys.exit(97)
 marker.write_text("owned confined fixture account\n")
elif pathlib.Path(sys.argv[0]).name == "userdel":
 if sys.argv[1:] != ["pulse"]: sys.exit(97)
 marker.unlink()
elif pathlib.Path(sys.argv[0]).name == "chown":
 args = [arg for arg in sys.argv[1:] if arg != "-R"]
 if len(args) < 2 or args[0] != "pulse:pulse": sys.exit(97)
 if any(not pathlib.Path(p).is_relative_to(marker.parent) for p in args[1:]): sys.exit(97)
else: sys.exit(97)
'''
            for name in ("id", "useradd", "userdel", "chown"):
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
            lab.env["HOME"] = str(base)
            lab.env["PATH"] = str(lab.root / "tools") + ":" + str(tools) + ":/usr/sbin:/usr/bin:/sbin:/bin"
            original_write = Path.write_text
            def relocate(path, data, *args, **kwargs):
                if path.name == "installer-definitions":
                    before = "cat > /etc/systemd/system/$SERVICE_NAME.service << EOF"
                    self.assertEqual(data.count(before), 1)
                    append = "cat >> /etc/systemd/system/$SERVICE_NAME.service << EOF"
                    self.assertEqual(data.count(append), 2)
                    data = data.replace(before, 'cat > "$PULSE_NATIVE_ROOT/pulse-source-updater.service" << EOF')
                    data = data.replace(append, 'cat >> "$PULSE_NATIVE_ROOT/pulse-source-updater.service" << EOF')
                return original_write(path, data, *args, **kwargs)
            try:
                with patch.object(Path, "write_text", new=relocate), patch.object(native.time, "sleep"):
                    try:
                        lab.prepare()
                        self.assertEqual((lab.root / "data/.env").read_text(),
                                         "PULSE_TELEMETRY=false\nBIND_ADDRESS=127.0.0.1\n")
                        lab.exercise()
                    except RuntimeError as error:
                        self.fail(str(error) + "\nConfined fixture output:\n" + lab._last_output.decode())
                self.assertEqual([p["name"] for p in lab.receipt["phases"]],
                                 ["forward", "manual-reverse", "failed-active", "failed-inactive",
                                  "failed-masked", "bad-signature", "incomplete-restore"])
                self.assertTrue(lab.receipt["phases"][-1]["backup_retained"])
                used = [json.loads(line) for line in tool_trace.read_text().splitlines()]
                self.assertEqual(set(used), set(FIXTURE_COMMANDS),
                                 "nested shell lost a containing-job fixture tool")
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
