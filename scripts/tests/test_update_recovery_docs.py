#!/usr/bin/env python3
"""Exercise update help without touching a service, volume or Docker socket.

The synthetic commands prove scope and failure handling, not installed recovery.
The remaining checks bind the shipped help to the existing UI labels and
protect version-qualified recovery advice, not a particular snapshot writer.
"""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


def guide(name):
    return (ROOT / "docs" / f"{name}.md").read_text(encoding="utf-8")


def section(name, heading, level="##"):
    return guide(name).split(f"{level} {heading}\n", 1)[1].split(f"\n{level} ", 1)[0]


READER = r'''#!/usr/bin/python3
import json, os, sys
from pathlib import Path
name = Path(sys.argv[0]).name
args = sys.argv[1:]
with Path(os.environ['CALLS']).open('a') as output:
    output.write(json.dumps({'name': name, 'args': args}) + '\n')
if name == 'systemctl':
    if args == ['is-active', 'pulse']:
        print(os.environ['SERVICE_STATE'])
        sys.exit(0 if os.environ['SERVICE_STATE'] == 'active' else 3)
    assert args == ['show', 'pulse', '--property=FragmentPath', '--property=DropInPaths']
    print('FragmentPath=/etc/systemd/system/pulse.service\nDropInPaths=/etc/systemd/system/pulse.service.d/local.conf')
elif name == 'docker':
    if args == ['ps', '-a', '--format', '{{.Names}}']:
        print('example-app\nexample-app_pulse_backup_123')
    elif args == ['inspect', '--type', 'container', '--format',
                  'State={{.State.Status}} ImageRef={{.Config.Image}} ImageID={{.Image}} Started={{.State.StartedAt}}',
                  'example-app']:
        if os.environ.get('INSPECT_EXIT'):
            print('No such container: example-app', file=sys.stderr)
            sys.exit(int(os.environ['INSPECT_EXIT']))
        print('State=' + os.environ['CONTAINER_STATE'] +
              ' ImageRef=private.example/app:latest ImageID=sha256:current Started=2026-10-06T12:00:00Z')
    elif args == ['inspect', 'pulse', '--format', '{{.Config.Image}} {{.Image}}']:
        print('private.example/pulse:old sha256:previous')
    elif args == ['compose', 'pull', 'pulse']:
        sys.exit(int(os.environ.get('PULL_EXIT', '0')))
    elif args == ['compose', 'up', '-d', '--no-deps', 'pulse']:
        sys.exit(int(os.environ.get('UP_EXIT', '0')))
    else:
        raise AssertionError('unexpected Docker mutation or unbounded inspection')
else:
    raise AssertionError('unexpected command')
'''


class UpdateRecoveryDocsTest(unittest.TestCase):
    def exercise(self, recipe, **settings):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in ("systemctl", "docker"):
                executable = root / name
                executable.write_text(READER, encoding="utf-8")
                executable.chmod(0o700)
            calls_path = root / "calls.jsonl"
            env = dict(os.environ, PATH=f"{root}:{os.environ['PATH']}",
                       CALLS=str(calls_path), SERVICE_STATE="active",
                       CONTAINER_STATE="running",
                       SYNTHETIC_CREDENTIAL="never-print-this-private-value")
            env.update(settings)
            result = subprocess.run(["bash", "-c", recipe], env=env,
                                    text=True, capture_output=True, timeout=10)
            calls = [json.loads(line) for line in calls_path.read_text().splitlines()]
            self.assertNotIn(env["SYNTHETIC_CREDENTIAL"], result.stdout + result.stderr)
            return result, calls

    def compose_recipe(self):
        recipes = re.findall(r"```bash\n(.*?)```", section("DOCKER", "🔄 Updates"), re.S)
        return next(recipe for recipe in recipes if "compose" in recipe)

    def test_mirrors_match_the_exercised_guides(self):
        for name in ("AUTO_UPDATE", "DOCKER", "INSTALL"):
            with self.subTest(guide=name):
                self.assertEqual(guide(name),
                                 (ROOT / "frontend-modern/public/docs" / f"{name}.md").read_text())

    def test_rollback_matches_the_existing_history_control(self):
        help_text = " ".join(section("AUTO_UPDATE", "Rollback").split())
        ui = (ROOT / "frontend-modern/src/components/Settings/UpdateHistorySection.tsx").read_text()
        for label in ("Update History", "Roll back"):
            self.assertIn(label, ui)
            self.assertIn(label, help_text)
        self.assertIn("entry.action === 'update' && entry.status === 'success' && Boolean(entry.backup_path)", ui)
        for phrase in ("successful in-app update", "retained backup", "version before that update",
                       "Older versions may not have this control", "actual backup contents"):
            self.assertIn(phrase, help_text)
        self.assertNotIn("There is no rollback UI", help_text)

    def test_snapshot_scope_is_not_claimed_to_be_a_full_data_backup(self):
        help_text = " ".join(section("AUTO_UPDATE", "What an update snapshot contains", "###").split())
        for phrase in ("running server binary", "`VERSION`", "`.env`", "`data/`", "`config/`",
                       "`PULSE_INSTALL_DIR`", "not necessarily a complete", "active `PULSE_DATA_DIR`",
                       "Copy errors can leave a partial", "`backup_path`", "lost on reboot"):
            self.assertIn(phrase, help_text)

    def test_online_rollback_never_promises_full_state_or_live_data_rewind(self):
        help_text = " ".join(section("AUTO_UPDATE", "Rollback").split())
        for phrase in ("Returning to an older binary is not a full-state recovery",
                       "Update History is not a full-state recovery tool",
                       "Do not assume later settings and alert changes will be reverted",
                       "omit active data stored elsewhere", "do not restore it while Pulse is running",
                       "[Manual Rollback](#manual-rollback)"):
            self.assertIn(phrase, help_text)
        self.assertNotIn("warns that later settings and alert changes will be reverted", help_text)

    def test_snapshot_and_install_help_qualify_scope_without_claiming_future_support(self):
        scope = " ".join(section("AUTO_UPDATE", "What an update snapshot contains", "###").split())
        for phrase in ("published **v6.4.5** and **v6.4.6-rc.1**", "limited installation snapshot",
                       "backup made by another updater version", "completeness, consistency",
                       "not permission to rewind live runtime stores"):
            self.assertIn(phrase, scope)
        install = " ".join(section("INSTALL", "Rollback", "###").split())
        for phrase in ("Neither is guaranteed to include all active data",
                       "Update History is not a full-state recovery tool",
                       "data migrated by a newer version", "version-specific snapshot scope",
                       "AUTO_UPDATE.md#manual-rollback", "matching data and keys together",
                       "preserve reversible copies", "Do not replace live runtime data"):
            self.assertIn(phrase, install)
        self.assertNotIn("backups are created automatically", install)
        self.assertNotIn("auto-restores on failure", install)

    def test_recovery_preserves_failed_state_keys_and_sidecars(self):
        help_text = " ".join(section("AUTO_UPDATE", "Rollback").split())
        for phrase in ("consistent private backup", "matching `.encryption.key`", "signing key together",
                       "SQLite sidecar files", "Keep these backups private", "missing, partial",
                       "isolated recovery instance", "no access to monitored systems or notification destinations",
                       "Do not start a second connected copy", "keep reversible copies",
                       "regenerate encryption or signing keys", "running version and edition",
                       "An image change alone is not a data rollback"):
            self.assertIn(phrase, help_text)
        # No executable deletion, overwrite, restore, start or stop recipe is
        # safe without knowing this deployment's effective paths and backup.
        blocks = re.findall(r"```bash\n(.*?)```", section("AUTO_UPDATE", "Rollback"), re.S)
        self.assertEqual(len(blocks), 1)
        self.assertNotRegex(blocks[0], r"\b(?:rm|cp|mv|install|tee|start|stop|restart)\b")

    def test_read_only_discovery_handles_active_and_inactive_services(self):
        recipe = re.findall(r"```bash\n(.*?)```", section("AUTO_UPDATE", "Manual Rollback", "###"), re.S)[0]
        for state in ("active", "inactive"):
            with self.subTest(state=state):
                result, calls = self.exercise(recipe, SERVICE_STATE=state)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls, [
                    {"name": "systemctl", "args": ["is-active", "pulse"]},
                    {"name": "systemctl", "args": ["show", "pulse", "--property=FragmentPath", "--property=DropInPaths"]},
                ])
                self.assertIn(state, result.stdout)
                self.assertIn("FragmentPath=", result.stdout)
                self.assertNotIn("Environment=", result.stdout)

    def test_compose_updates_only_pulse_after_a_successful_pull(self):
        result, calls = self.exercise(self.compose_recipe())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, [
            {"name": "docker", "args": ["compose", "pull", "pulse"]},
            {"name": "docker", "args": ["compose", "up", "-d", "--no-deps", "pulse"]},
        ])

    def test_failed_pull_never_recreates_and_failed_recreation_is_not_success(self):
        for settings, exit_code, count in (({"PULL_EXIT": "7"}, 7, 1), ({"UP_EXIT": "8"}, 8, 2)):
            with self.subTest(settings=settings):
                result, calls = self.exercise(self.compose_recipe(), **settings)
                self.assertEqual(result.returncode, exit_code, result.stderr)
                self.assertEqual(len(calls), count)

    def test_image_selection_is_durable_and_private_edition_is_preserved(self):
        help_text = " ".join(section("DOCKER", "🔄 Updates").split())
        for phrase in ("exact target", "persist `PULSE_IMAGE`", "has no effect on a hardcoded image line",
                       "paid Pro installs must keep the private image", "same data mount, ports and settings",
                       "running image again", "an image change alone does not undo data migrations"):
            self.assertIn(phrase, help_text)
        recipe = re.findall(r"```bash\n(.*?)```", section("DOCKER", "🔄 Updates"), re.S)[0]
        result, calls = self.exercise(recipe)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls[0]["args"], ["inspect", "pulse", "--format", "{{.Config.Image}} {{.Image}}"])
        self.assertIn("private.example/pulse:old sha256:previous", result.stdout)
        short = section("AUTO_UPDATE", "Docker", "###")
        self.assertIn("DOCKER.md#-updates", short)
        self.assertNotIn("docker pull", short)

    def workload_check_recipes(self):
        return re.findall(r"```bash\n(.*?)```", section(
            "DOCKER", "Check a failed or pending workload update", "###"), re.S)

    def test_workload_discovery_and_inspection_are_scoped_read_only(self):
        recipes = self.workload_check_recipes()
        self.assertEqual(len(recipes), 2)
        result, calls = self.exercise(recipes[0])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, [{"name": "docker", "args": ["ps", "-a", "--format", "{{.Names}}"]}])
        self.assertIn("example-app_pulse_backup_123", result.stdout)
        recipe = recipes[1].replace("container='affected-container'", "container='example-app'")
        for state in ("running", "exited", "restarting"):
            with self.subTest(state=state):
                result, calls = self.exercise(recipe, CONTAINER_STATE=state)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls, [{"name": "docker", "args": [
                    "inspect", "--type", "container", "--format",
                    "State={{.State.Status}} ImageRef={{.Config.Image}} ImageID={{.Image}} Started={{.State.StartedAt}}",
                    "example-app",
                ]}])
                self.assertIn(f"State={state}", result.stdout)
                self.assertIn("ImageID=sha256:current", result.stdout)
                self.assertNotIn("Environment", result.stdout)

    def test_missing_workload_preserves_the_error_without_recovery_mutation(self):
        recipe = self.workload_check_recipes()[1].replace(
            "container='affected-container'", "container='example-app'")
        result, calls = self.exercise(recipe, INSPECT_EXIT="1")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(calls), 1)
        self.assertIn("No such container", result.stderr)

    def test_workload_identity_and_pending_receipt_help_preserve_uncertainty(self):
        text = " ".join(section("DOCKER", "Check a failed or pending workload update", "###").split())
        for phrase in ("on the host running the affected container", "same Docker context",
                       "do not change socket permissions", "not `pulse` unless Pulse itself was the target",
                       "tag such as `latest` can stay unchanged", "not a registry manifest digest",
                       "cannot establish whether the image changed", "not proof of an image update",
                       "Do not start, rename, delete or update anything", "not proof that its application or data is healthy",
                       "does not send another container update", "not the full inspection"):
            self.assertIn(phrase, text)
        ui = (ROOT / "frontend-modern/src/features/actions/ActionReviewDialog.tsx").read_text()
        badge = (ROOT / "frontend-modern/src/components/shared/containerUpdateBadgeModel.ts").read_text()
        self.assertIn("Review action", badge)
        self.assertIn("Check for receipt", ui)
        self.assertIn("refreshReceipt()", ui)


if __name__ == "__main__":
    unittest.main()
