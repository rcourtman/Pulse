#!/usr/bin/env python3
"""Execute launcher configuration admission with disposable sentinel data only.

The fixed /etc/pulse spelling is mapped to a fixture in copies of the launcher
and sync helper. All remaining production logic is unchanged. Build/process
tools are inert peers; the backend peer captures its actual environment and
exits before health monitoring, watchers or Vite can start. This is launcher
proof, not a running application, host-isolation or installed recovery test.
"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = Path(os.environ.get("HOT_DEV_CONFIG_TEST_SCRIPT", ROOT / "scripts/hot-dev.sh"))


class HotDevConfigTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.repo = self.base / "repo"
        self.installed = self.base / "installed"
        self.installed.mkdir()
        self.repo.mkdir()
        self.marker = self.base / "installed-env-read"
        self.capture = self.base / "backend.json"
        self.operations = self.base / "operations"
        self.seed = {
            ".env": f"touch '{self.marker}'\nPULSE_CONFIG_SENTINEL=installed-fixture\n",
            ".encryption.key": "synthetic-installed-key\n",
            "nodes.enc": "synthetic-installed-nodes\n",
            "email.enc": "synthetic-installed-mail\n",
            "webhooks.enc": "synthetic-installed-webhooks\n",
            "api_tokens.json": '[{"fixture": "not-a-real-token"}]\n',
            "system.json": "{}\n",
        }
        for name, content in self.seed.items():
            (self.installed / name).write_text(content)
        scripts = self.repo / "scripts"
        (scripts / "lib").mkdir(parents=True)
        for helper in (ROOT / "scripts/lib").glob("hot-dev-*.sh"):
            shutil.copyfile(helper, scripts / "lib" / helper.name)
        # Path mapping, not logic replacement; no real /etc content is accessed.
        for source, name in ((SCRIPT, "hot-dev.sh"), (ROOT / "scripts/sync-production-config.sh", "sync-production-config.sh")):
            target = scripts / name
            target.write_text(source.read_text().replace("/etc/pulse", str(self.installed)))
            target.chmod(0o700)
        self.bin = self.base / "bin"
        self.bin.mkdir()
        self.write_peer("go", '''
if [[ "$1" == env ]]; then exit 0; fi
if [[ "$1" != build ]]; then exit 81; fi
cp "$FIXTURE_BACKEND_PEER" ./pulse
chmod 700 ./pulse
''')
        self.write_peer("npm", 'echo unexpected-npm >> "$FIXTURE_OPERATIONS"; exit 82\n')
        for peer in ("sudo", "pkill"):
            self.write_peer(peer, f'echo {peer} >> "$FIXTURE_OPERATIONS"; exit 0\n')
        for peer in ("lsof", "pgrep"):
            self.write_peer(peer, "exit 1\n")
        self.write_peer("sleep", "exec /bin/sleep 0.15\n")
        backend = self.base / "backend-peer"
        backend.write_text('''#!/usr/bin/env python3
import json, os
from pathlib import Path
names = ("PULSE_DATA_DIR", "PULSE_CONFIG_SENTINEL", "PULSE_ENCRYPTION_KEY",
         "PULSE_DEV", "ALLOW_ADMIN_BYPASS", "BIND_ADDRESS", "FRONTEND_DEV_PORT")
Path(os.environ["FIXTURE_CAPTURE"]).write_text(json.dumps({n: os.environ.get(n) for n in names}))
''')
        backend.chmod(0o700)
        # Never inherit an installed environment, credentials or a dev session.
        self.env = {
            "PATH": f"{self.bin}:/usr/bin:/bin", "HOME": str(self.base), "LANG": "C.UTF-8",
            "HOT_DEV_USE_PRO": "false", "HOT_DEV_SKIP_NPM_CLEANUP": "true",
            "PULSE_BACKEND_LOG_FILE": str(self.base / "backend.log"),
            "HOT_DEV_LAB_MODE_FILE": str(self.base / "lab-mode"),
            "LAN_IP": "127.0.0.1", "ALL_IPS": "127.0.0.1",
            "FIXTURE_BACKEND_PEER": str(backend), "FIXTURE_CAPTURE": str(self.capture),
            "FIXTURE_OPERATIONS": str(self.operations),
        }

    def write_peer(self, name, body):
        target = self.bin / name
        target.write_text("#!/bin/bash\nset -eu\n" + body)
        target.chmod(0o700)

    def launch(self, **settings):
        result = subprocess.run(["bash", str(self.repo / "scripts/hot-dev.sh")],
                                env=self.env | settings, cwd=self.repo,
                                capture_output=True, text=True, timeout=15)
        self.output = result.stdout + result.stderr
        self.assertNotIn("unexpected-npm", self.operations.read_text() if self.operations.exists() else "")
        return result

    def captured_backend(self):
        self.assertTrue(self.capture.is_file(), self.output)
        self.assertIn("Backend failed to start!", self.output)  # intentional peer exit
        self.assertNotIn("Starting backend health monitor", self.output)
        return json.loads(self.capture.read_text())

    def assert_installed_unchanged(self):
        self.assertEqual({p.name: p.read_text() for p in self.installed.iterdir()}, self.seed)

    def assert_isolated(self, data):
        captured = self.captured_backend()
        self.assertEqual(captured["PULSE_DATA_DIR"], str(data))
        self.assertIsNone(captured["PULSE_CONFIG_SENTINEL"])
        self.assertNotEqual(captured["PULSE_ENCRYPTION_KEY"], self.seed[".encryption.key"].strip())
        self.assertEqual((captured["PULSE_DEV"], captured["ALLOW_ADMIN_BYPASS"]), ("true", "1"))
        self.assertFalse(self.marker.exists(), "installed .env was sourced")
        for name in ("nodes.enc", "email.enc", "webhooks.enc", "api_tokens.json"):
            self.assertFalse((data / name).exists(), f"silently imported {name}")
        self.assert_installed_unchanged()

    def test_fresh_default_does_not_import_env_key_targets_or_tokens(self):
        self.launch()
        self.assert_isolated(self.repo / "tmp/dev-config")

    def test_explicit_isolated_directory_does_not_load_installed_env(self):
        data = self.base / "isolated"
        self.launch(PULSE_DATA_DIR=str(data), HOT_DEV_USE_PROD_DATA="false")
        self.assert_isolated(data)

    def test_lab_network_selection_does_not_imply_config_import(self):
        self.launch(PULSE_DEV_LAB_AGENTS="true")
        self.assert_isolated(self.repo / "tmp/dev-config")
        self.assertEqual(json.loads(self.capture.read_text())["BIND_ADDRESS"], "0.0.0.0")

    def test_mock_mode_does_not_import_installed_config(self):
        self.launch(PULSE_MOCK_MODE="true")
        self.assert_isolated(self.repo / "tmp/mock-data")

    def test_explicit_copy_imports_existing_lab_files_but_not_installed_env(self):
        self.launch(HOT_DEV_IMPORT_PROD_CONFIG="true")
        captured = self.captured_backend()
        data = self.repo / "tmp/dev-config"
        self.assertEqual(captured["PULSE_DATA_DIR"], str(data))
        self.assertIsNone(captured["PULSE_CONFIG_SENTINEL"])
        self.assertFalse(self.marker.exists())
        for name in (".encryption.key", "nodes.enc", "email.enc", "webhooks.enc", "api_tokens.json"):
            self.assertEqual((data / name).read_text(), self.seed[name])
            self.assertEqual((data / name).stat().st_mode & 0o777, 0o600)
        self.assertIn("this is not isolated reproduction", self.output)
        self.assert_installed_unchanged()

    def test_explicit_direct_mode_loads_installed_env_and_keeps_existing_data(self):
        self.launch(HOT_DEV_USE_PROD_DATA="true")
        captured = self.captured_backend()
        self.assertEqual(captured["PULSE_DATA_DIR"], str(self.installed))
        self.assertEqual(captured["PULSE_CONFIG_SENTINEL"], "installed-fixture")
        self.assertTrue(self.marker.exists())
        self.assertEqual(captured["PULSE_ENCRYPTION_KEY"], self.seed[".encryption.key"].strip())
        self.assertIn("development authentication bypass", self.output)
        self.assert_installed_unchanged()

    def test_direct_mode_keeps_explicit_custom_data_override(self):
        data = self.base / "intentional-data"
        data.mkdir()
        self.launch(HOT_DEV_USE_PROD_DATA="true", PULSE_DATA_DIR=str(data))
        self.assertEqual(self.captured_backend()["PULSE_DATA_DIR"], str(data))
        self.assertTrue(self.marker.exists())
        self.assert_installed_unchanged()

    def test_nonempty_encrypted_files_and_backups_refuse_new_dev_key(self):
        data = self.repo / "tmp/dev-config"
        data.mkdir(parents=True)
        for name in ("nodes.enc", "email.enc", "webhooks.enc", "oidc.enc", "ai.enc", "nodes.enc.bak-fixture"):
            with self.subTest(name=name):
                artifact = data / name
                artifact.write_text("synthetic-encrypted-fixture")
                self.launch()
                self.assertFalse(self.capture.exists(), self.output)
                self.assertFalse((data / ".encryption.key").exists())
                self.assertIn("Encryption key is missing but encrypted data exists", self.output)
                self.assertIn(name, self.output)
                self.assertEqual(artifact.read_text(), "synthetic-encrypted-fixture")
                artifact.unlink()
        self.assert_installed_unchanged()

    def test_empty_encrypted_file_does_not_block_fresh_dev_key(self):
        data = self.repo / "tmp/dev-config"
        data.mkdir(parents=True)
        (data / "nodes.enc").touch()
        self.launch()
        self.captured_backend()
        self.assertTrue((data / ".encryption.key").read_text().strip())
        self.assertEqual((data / "nodes.enc").stat().st_size, 0)
        self.assert_installed_unchanged()

    def assert_early_refusal(self, **settings):
        result = self.launch(**settings)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.capture.exists(), self.output)
        self.assertFalse(self.operations.exists(), "invalid selection reached process cleanup")
        self.assertFalse(self.marker.exists())
        self.assert_installed_unchanged()

    def test_invalid_selectors_stop_before_cleanup_or_installed_reads(self):
        for flag in ("HOT_DEV_USE_PROD_DATA", "HOT_DEV_IMPORT_PROD_CONFIG"):
            with self.subTest(flag=flag):
                self.assert_early_refusal(**{flag: "tru"})
                self.assertIn("must be exactly true or false", self.output)

    def test_conflicting_selectors_stop_before_cleanup(self):
        self.assert_early_refusal(HOT_DEV_USE_PROD_DATA="true", HOT_DEV_IMPORT_PROD_CONFIG="true")

    def test_installed_data_path_needs_explicit_direct_opt_in(self):
        self.assert_early_refusal(PULSE_DATA_DIR=str(self.installed) + "/./")
        self.assertIn("requires HOT_DEV_USE_PROD_DATA=true", self.output)

    def test_installed_data_symlink_needs_explicit_direct_opt_in(self):
        alias = self.base / "data-alias"
        alias.symlink_to(self.installed, target_is_directory=True)
        self.assert_early_refusal(PULSE_DATA_DIR=str(alias))

    def test_default_data_symlink_cannot_write_managed_auth_into_installed_data(self):
        (self.repo / "tmp").mkdir()
        (self.repo / "tmp/dev-config").symlink_to(self.installed, target_is_directory=True)
        self.launch()
        self.assertFalse(self.capture.exists(), self.output)
        self.assertFalse(self.marker.exists())
        self.assert_installed_unchanged()

    def test_runtime_env_cannot_redirect_backend_to_installed_data(self):
        data = self.base / "isolated"
        data.mkdir()
        (data / ".env").write_text(f"PULSE_DATA_DIR='{self.installed}'\n")
        self.launch(PULSE_DATA_DIR=str(data))
        self.assertFalse(self.capture.exists(), self.output)
        self.assertFalse(self.marker.exists())
        self.assert_installed_unchanged()

    def test_runtime_env_cannot_change_selected_copy_mode(self):
        data = self.base / "isolated"
        data.mkdir()
        (data / ".env").write_text("HOT_DEV_IMPORT_PROD_CONFIG=true\n")
        self.launch(PULSE_DATA_DIR=str(data))
        self.assertFalse(self.capture.exists(), self.output)
        self.assertIn("readonly variable", self.output)
        self.assert_installed_unchanged()

    def test_copy_refuses_custom_or_mock_data_without_importing(self):
        for settings in ({"PULSE_DATA_DIR": str(self.base / "custom")}, {"PULSE_MOCK_MODE": "true"}):
            with self.subTest(settings=settings):
                self.launch(HOT_DEV_IMPORT_PROD_CONFIG="true", **settings)
                self.assertFalse(self.capture.exists(), self.output)
                self.assertIn("requires the default non-mock development directory", self.output)
                self.assertFalse(self.marker.exists())
                self.assert_installed_unchanged()

    def test_default_stays_isolated_without_an_installation(self):
        shutil.rmtree(self.installed)
        self.launch()
        self.assertIsNone(self.captured_backend()["PULSE_CONFIG_SENTINEL"])
        self.assertFalse(self.installed.exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)
