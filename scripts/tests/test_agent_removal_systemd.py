#!/usr/bin/env python3
"""Portable lab/recipe controls. Manager doubles are NOT native acceptance."""

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
SPEC = importlib.util.spec_from_file_location("agent_removal_native", ROOT / "scripts/installtests/agent_removal_systemd.py")
native = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(native)
STEP = "Check agent removal on native systemd"


def recipe(source):
    job = re.search(r"(?ms)^  native-agent:\n(.*?)(?=^  [a-z][a-z0-9-]*:|\Z)", source)[1]
    steps = dict(re.findall(r"(?ms)^      - name: ([^\n]+)\n(.*?)(?=^      - name:|\Z)", job))
    step = steps[STEP]
    assert "if: runner.os == 'Linux'" in step
    assert "timeout-minutes: 5" in step
    assert "EXPECTED_SOURCE_SHA: ${{ github.sha }}" in step
    assert not re.search(r"^        (continue-on-error|working-directory):", step, re.M)
    assert list(steps).index(STEP) < list(steps).index("Set up Go")
    assert "ubuntu-24.04\n" in job and "ubuntu-24.04-arm\n" in job
    assert "persist-credentials: false" in steps["Checkout repository"]
    shell = re.search(r"(?ms)^        run: \|\n((?:          [^\n]*\n)+)", step)[1]
    shell = "\n".join(line[10:] for line in shell.splitlines()) + "\n"
    for required in ('set -euo pipefail', 'test "$(git rev-parse HEAD)" = "$EXPECTED_SOURCE_SHA"',
                     "git diff --exit-code HEAD", "python3 -m unittest discover -s scripts/tests -p test_agent_removal_systemd.py -v",
                     "sudo env -i", "GITHUB_ACTIONS=true RUNNER_ENVIRONMENT=github-hosted GITHUB_REPOSITORY=rcourtman/Pulse",
                     'EXPECTED_SOURCE_SHA="$EXPECTED_SOURCE_SHA" RUNNER_TEMP="$RUNNER_TEMP"',
                     "python3 scripts/installtests/agent_removal_systemd.py", '--receipt "$RUNNER_TEMP/pulse-agent-removal.json"'):
        assert required in shell, required
    assert shell.index("git diff") < shell.index("sudo env -i")
    assert "secrets." not in step and "GH_TOKEN" not in shell
    retained = steps["Retain bounded native agent removal receipt"]
    assert "if: always() && runner.os == 'Linux'" in retained
    assert "path: ${{ runner.temp }}/pulse-agent-removal.json\n" in retained
    assert "retention-days: 7" in retained
    assert len(re.findall(r"(?m)^          path:", retained)) == 1 and "*.log" not in retained
    for event in ("push", "pull_request"):
        section = re.search(r"(?ms)^  " + event + r":\n(.*?)(?=^  [a-z_]+:|\Z)", source)[1]
        assert "'scripts/tests/test_agent_removal_systemd.py'" in section
        assert "'scripts/installtests/**'" in section and "'scripts/install.sh'" in section
    return shell


# An explicit file-backed manager model for the lab's verdict and cleanup
# controls only. The native CI command uses /usr/bin/systemctl, not this model.
MANAGER = r'''#!/usr/bin/python3
import json, os, pathlib, sys
units = pathlib.Path(os.environ['MODEL_UNITS'])
database = pathlib.Path(os.environ['MODEL_STATES'])
values = json.loads(database.read_text()) if database.exists() else {}
args = sys.argv[1:]
if args[0] == '--user':
    print('closed fixture: no user manager', file=sys.stderr)
    sys.exit(1)
verb = args.pop(0)
unit = args[-1] if args else ''
def observed(name):
    file = units / name
    value = values.get(name, {'ActiveState':'inactive', 'UnitFileState':'disabled', 'MainPID':'0'})
    return dict(value, LoadState='loaded' if file.exists() else 'not-found',
                UnitFileState=value['UnitFileState'] if file.exists() else '')
if verb == 'show':
    unit, properties = args
    if os.environ.get('MODEL_EMPTY_READ') == 'true': sys.exit(0)
    for key in reversed(properties.split('=', 1)[1].split(',')):
        if key != 'MainPID' or unit.endswith('.service'): print(key + '=' + observed(unit)[key])
elif verb == 'enable' or verb == 'start':
    v = observed(unit)
    text = (units / unit).read_text()
    v.update(ActiveState='active', MainPID=str(int(v.get('MainPID', '0')) + 100))
    if os.environ.get('MODEL_START_INACTIVE') == 'true': v.update(ActiveState='inactive', MainPID='0')
    if verb == 'enable': v['UnitFileState'] = 'enabled' if '[Install]' in text else 'static'
    values[unit] = v
elif verb == 'stop':
    file = units / unit
    text = file.read_text() if file.exists() else ''
    if 'RefuseManualStop=yes' in text: sys.exit(1)
    if os.environ.get('MODEL_IGNORE_STOP') != unit:
        target = 'pulse-removal-native-runner.service'
        if 'ExecStop=/usr/bin/systemctl start ' + target in text:
            values[target] = dict(observed(target), ActiveState='active', MainPID='200')
        values[unit] = dict(observed(unit), ActiveState='inactive', MainPID='0')
elif verb == 'disable':
    v = observed(unit)
    if v['UnitFileState'] != 'static': v['UnitFileState'] = 'disabled' if (units / unit).exists() else ''
    values[unit] = v
elif verb not in ('daemon-reload', 'reset-failed'): sys.exit(97)
database.write_text(json.dumps(values))
'''


class ModelLab(native.Lab):
    def helper(self, directory, refuse=False):
        # Model activation directly; never claim this is a native socket.
        self.service(native.HELPER + ".service")
        self.install_unit(native.HELPER + ".socket", "[Unit]\n" +
                          ("RefuseManualStop=yes\n" if refuse else "") + "[Install]\n")


class NativeRemovalControls(unittest.TestCase):
    installer = (ROOT / "scripts/install.sh").read_text()

    def test_definition_selection_preserves_each_actual_function_and_refuses_drift(self):
        selected = native.definitions(self.installer)
        for name in native.FUNCTIONS:
            marker = name + "() {"
            body = marker + self.installer.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n"
            self.assertIn(body, selected)
            for change in (self.installer.replace(marker, name + "_moved() {"), self.installer + body):
                with self.assertRaises(ValueError): native.definitions(change)

    def test_admission_excludes_worker_self_hosted_nonroot_existing_hosts_and_unbound_source(self):
        env = dict(GITHUB_ACTIONS="true", RUNNER_ENVIRONMENT="github-hosted", GITHUB_REPOSITORY="rcourtman/Pulse")
        with patch.object(Path, "exists", return_value=False):
            native.admission(env, 0, "a" * 40)
            for changed, uid, sha in ((env, 1000, "a" * 40), ({}, 0, "a" * 40),
                                      ({**env, "RUNNER_ENVIRONMENT": "self-hosted"}, 0, "a" * 40),
                                      ({**env, "GITHUB_REPOSITORY": "other/repo"}, 0, "a" * 40),
                                      (env, 0, "main")):
                with self.assertRaises(ValueError): native.admission(changed, uid, sha)
        with patch.object(Path, "exists", return_value=True):
            with self.assertRaises(ValueError): native.admission(env, 0, "a" * 40)

    def test_independent_observation_refuses_missing_duplicate_and_diagnostic_lines(self):
        complete = b"LoadState=loaded\nActiveState=active\nUnitFileState=enabled\nMainPID=123\n"
        self.assertEqual(native.state(complete)["MainPID"], "123")
        for changed in (b"", complete.replace(b"LoadState=loaded\n", b""), complete + b"\n",
                        complete + b"LoadState=loaded\n", complete + b"diagnostic\n"):
            with self.assertRaises(ValueError): native.state(changed)

    def test_recipe_owns_exact_clean_source_mandatory_linux_execution_and_bounded_retention(self):
        workflow = (ROOT / ".github/workflows/unified-agent-native.yml").read_text()
        recipe(workflow)
        for old, new in (("if: runner.os == 'Linux'\n        timeout-minutes: 5", "if: false\n        timeout-minutes: 5"),
                         ("sudo env -i", "sudo env"), ("git diff --exit-code HEAD", "true"),
                         ("EXPECTED_SOURCE_SHA: ${{ github.sha }}", "EXPECTED_SOURCE_SHA: main"),
                         ("'scripts/tests/test_agent_removal_systemd.py'", "'other.py'"),
                         ("path: ${{ runner.temp }}/pulse-agent-removal.json", "path: '**/*'")):
            self.assertIn(old, workflow)
            with self.assertRaises(AssertionError): recipe(workflow.replace(old, new))

    def model(self, callback):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            units, tools = root / "units", root / "tools"
            units.mkdir(); tools.mkdir()
            executable = tools / "systemctl"
            executable.write_text(MANAGER); executable.chmod(0o755)
            def model_pid(pid):
                database = root / "states.json"
                values = json.loads(database.read_text()) if database.exists() else {}
                return any(v["ActiveState"] == "active" and v["MainPID"] == pid for v in values.values())
            with patch.object(native, "LAB", root / "lab"), patch.object(native, "UNIT_DIR", units), patch.object(native, "pid_alive", side_effect=model_pid):
                lab = ModelLab("a" * 40, self.installer)
                lab.env.update(PATH=str(tools) + ":" + lab.env["PATH"], MODEL_UNITS=str(units), MODEL_STATES=str(root / "states.json"))
                try:
                    callback(lab, root)
                finally:
                    lab.cleanup()

    def test_all_nine_phase_verdicts_and_owned_cleanup_with_closed_manager_model(self):
        def run(lab, root):
            lab.prepare()
            for phase in native.PHASES:
                lab.exercise(phase)
                lab.cleanup_units()
            self.assertEqual([p["name"] for p in lab.receipt["phases"]], list(native.PHASES))
            self.assertTrue(all(p["passed"] for p in lab.receipt["phases"]))
            self.assertEqual(lab.receipt["credential_transport"], "closed-synthetic-revocation-only")
            lab.cleanup()
            self.assertTrue(lab.receipt["cleanup"]["complete"])
            self.assertFalse((root / "lab").exists())
            self.assertEqual(list((root / "units").iterdir()), [])
        self.model(run)

    def test_passing_stop_command_without_quiescence_is_not_a_native_pass(self):
        def run(lab, root):
            lab.prepare()
            lab.env["MODEL_IGNORE_STOP"] = native.AGENT + ".service"
            with self.assertRaisesRegex(RuntimeError, "verdict differs"): lab.exercise("active-agent")
            self.assertFalse(lab.receipt["phases"][0]["passed"])
            self.assertTrue(lab.receipt["phases"][0]["recovery_files_unchanged"])
            lab.env.pop("MODEL_IGNORE_STOP")
        self.model(run)

    def test_empty_successful_manager_read_is_not_an_absence_pass(self):
        def run(lab, root):
            lab.prepare()
            lab.env["MODEL_EMPTY_READ"] = "true"
            with self.assertRaisesRegex(ValueError, "observation missing"): lab.exercise("active-agent")
            self.assertEqual(lab.receipt["phases"], [])
            self.assertTrue((root / "units" / (native.AGENT + ".service")).exists())
            lab.env.pop("MODEL_EMPTY_READ")
        self.model(run)

    def test_successful_start_without_real_active_identity_cannot_satisfy_a_phase(self):
        def run(lab, root):
            lab.prepare()
            lab.env["MODEL_START_INACTIVE"] = "true"
            with self.assertRaisesRegex(RuntimeError, "intended active fixture"): lab.exercise("active-agent")
            self.assertEqual(lab.receipt["phases"], [])
        self.model(run)

    def test_lab_rejects_removal_that_discards_final_batch_readback(self):
        def run(lab, root):
            lab.prepare()
            last_read = '    for unit in "$@"; do\n        confirm_agent_unit_quiescent "$unit" || return 1\n    done\n'
            self.assertEqual(lab.definitions.count(last_read), 1)
            lab.definitions = lab.definitions.replace(last_read, "    :\n")
            with self.assertRaisesRegex(RuntimeError, "verdict differs"): lab.exercise("batch-reactivation")
            record = lab.receipt["phases"][0]
            self.assertEqual(record["after"][native.RUNNER + ".service"]["ActiveState"], "active")
            self.assertFalse(record["passed"])
        self.model(run)

    def test_preexisting_unit_is_never_stopped_or_removed(self):
        def run(lab, root):
            sentinel = root / "units" / (native.AGENT + ".service")
            sentinel.write_text("preexisting unrelated service\n")
            with self.assertRaisesRegex(ValueError, "already owned"): lab.prepare()
            lab.cleanup()
            self.assertEqual(sentinel.read_text(), "preexisting unrelated service\n")
            self.assertFalse(lab.owned_root)
        self.model(run)

    def test_failed_cleanup_stays_failed_and_keeps_ownership_for_reconciliation(self):
        lab = native.Lab("a" * 40, self.installer)
        lab.owned_units.add(native.AGENT + ".service")
        with patch.object(lab, "cleanup_units", side_effect=RuntimeError("stop refused")):
            with self.assertRaises(RuntimeError): lab.cleanup()
        self.assertFalse(lab.receipt["cleanup"]["complete"])
        self.assertEqual(lab.owned_units, {native.AGENT + ".service"})


if __name__ == "__main__":
    unittest.main()
