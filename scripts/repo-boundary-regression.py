#!/usr/bin/env python3
"""Run the real paid-boundary audit against isolated source fixtures."""

from pathlib import Path
import os
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SUBJECT = "reporting_subject_alerts.go"
MODES = (
    "report", "--enforce", "--enforce-api-imports", "--enforce-api-root-imports",
    "--enforce-nonapi-imports", "--enforce-api-pkg-licensing-imports",
    "--enforce-paid-surface-allowlist",
)


def initialize_fixture(fixture):
    subprocess.run(["git", "init", "--quiet", str(fixture)], check=True,
                   stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10)


class RunnerDependencyTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.fixture = Path(self.temporary.name) / "source"
        self.fixture.mkdir()
        initialize_fixture(self.fixture)
        for filename in (
            "scripts/audit-private-boundary.sh",
            "scripts/repo-boundary-paid-surface.allowlist",
            "scripts/repo-boundary-private-implementation.baseline",
        ):
            target = self.fixture / filename
            target.parent.mkdir(parents=True, exist_ok=True)
            if filename.endswith(".sh"):
                shutil.copyfile(ROOT / filename, target)
            else:
                target.write_text("")
        self.write("internal/api/handler.go", "package api\n")
        self.write("internal/license/license.go", "package license\n")
        # Exercise the exact audit with only checkout-runner tools available.
        # An installed rg must not accidentally make these fixtures pass.
        tools = Path(self.temporary.name) / "bin"
        tools.mkdir()
        for name in ("bash", "git", "grep", "sed", "sort", "cat", "paste", "cut", "wc", "tr"):
            (tools / name).symlink_to(shutil.which(name))
        self.environment = dict(os.environ, PATH=str(tools))
        self.assertIsNone(shutil.which("rg", path=str(tools)))

    def write(self, path, content):
        target = self.fixture / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)

    def audit(self, mode, exitcode, message=None, cwd=None):
        result = subprocess.run(
            [shutil.which("bash"), str(self.fixture / "scripts/audit-private-boundary.sh"), mode],
            cwd=cwd or self.fixture, env=self.environment, stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT, text=True, timeout=30, check=False,
        )
        self.assertEqual(result.returncode, exitcode, result.stdout)
        if message:
            self.assertIn(message, result.stdout)
        return result.stdout

    def test_all_modes_work_without_ripgrep_or_network_installation(self):
        for mode in MODES:
            with self.subTest(mode=mode):
                self.audit(mode, 0)

    def test_each_enforcement_mode_still_rejects_its_violation(self):
        variants = (
            ("--enforce", "internal/api/reporting_extra.go", "package api\n",
             "new production paid-domain implementation files are not baselined"),
            ("--enforce-api-imports", "internal/api/handler.go",
             'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"\n',
             "non-test internal API files still import internal/license"),
            ("--enforce-api-root-imports", "internal/api/handler.go",
             'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n',
             "API root imports of internal/license remain"),
            ("--enforce-nonapi-imports", "cmd/example/nested/main.go",
             'package main\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n',
             "non-API runtime files import internal/license"),
            ("--enforce-api-pkg-licensing-imports", "internal/api/handler.go",
             'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"\n',
             "API files outside bridge allowlist still import pkg/licensing"),
            ("--enforce-paid-surface-allowlist", "scripts/repo-boundary-paid-surface.allowlist",
             "internal/api/missing.go\n", "paid-surface allowlist contains paths that no longer exist"),
        )
        for mode, path, content, message in variants:
            with self.subTest(mode=mode):
                target = self.fixture / path
                original = target.read_text() if target.exists() else None
                self.write(path, content)
                self.audit(mode, 1, message)
                if original is None:
                    target.unlink()
                else:
                    target.write_text(original)

    def test_report_mode_keeps_violation_visible_without_enforcing(self):
        self.write("internal/api/reporting_extra.go", "package api\n")
        self.audit("report", 0, "Production files in paid domains (new private implementation leakage): 1")

    def test_stale_baseline_is_still_rejected(self):
        self.write("scripts/repo-boundary-private-implementation.baseline", "internal/api/reporting_removed.go\n")
        self.audit("--enforce", 1, "shrink the private-implementation baseline")

    def test_exact_licensing_bridge_remains_allowed(self):
        self.write("internal/api/licensing_bridge.go", 'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"\n')
        self.audit("--enforce-api-pkg-licensing-imports", 0, "API files importing pkg/licensing outside bridge: 0")

    def test_semantic_routes_and_untracked_files_are_still_scanned(self):
        self.write("internal/api/handler.go", 'package api\nconst route = "/api/license"\n')
        self.audit("--enforce", 1, "internal/api/handler.go")

    def test_nonapi_scan_keeps_test_and_private_directory_exclusions(self):
        content = 'package example\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n'
        for path in ("cmd/example/main_test.go", "internal/api/handler.go", "internal/license/license.go"):
            self.write(path, content)
        self.audit("--enforce-nonapi-imports", 0, "Non-API runtime imports of internal/license: 0")

    def test_ignored_untracked_files_are_not_added_to_the_inventory(self):
        self.write(".gitignore", "internal/api/reporting_ignored.go\ncmd/ignored/\n")
        self.write("internal/api/reporting_ignored.go", 'package api\nconst route = "/api/license"\n')
        self.write("cmd/ignored/main.go", 'package main\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n')
        self.audit("--enforce", 0, "Production files in paid domains: 0")
        self.audit("--enforce-nonapi-imports", 0)

    def test_missing_repository_and_nested_directory_fail_closed(self):
        for cwd in (Path(self.temporary.name), self.fixture / "internal/api"):
            with self.subTest(cwd=cwd.name):
                self.audit("--enforce", 2, "Run the boundary audit from the Git repository root", cwd=cwd)

    def test_workflow_keeps_all_checks_without_package_bootstrap(self):
        workflow = (ROOT / ".github/workflows/repo-boundary-audit.yml").read_text()
        self.assertIn("run: python3 scripts/repo-boundary-regression.py", workflow)
        for mode in MODES:
            if mode != "report":
                self.assertIn(f"run: ./scripts/audit-private-boundary.sh {mode}\n", workflow)
        self.assertIn("./scripts/audit-private-boundary.sh | tee repo-boundary-audit.txt", workflow)
        for dependency in ("apt-get", "ripgrep", "curl ", "wget ", "continue-on-error"):
            self.assertNotIn(dependency, workflow)


class ReportingSubjectBoundaryTests(unittest.TestCase):
    def test_shared_exemption_is_exact_and_preserves_import_boundaries(self):
        variants = [
            ("shared pipeline", None, None, None),
            (
                "SSO configuration adapter exact path", "sso_config_preservation_extra.go",
                "package api\n", "new production paid-domain implementation files are not baselined",
            ),
            (
                "SSO configuration adapter private root import", "sso_config_preservation.go",
                'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n',
                "API root imports of internal/license remain",
            ),
            (
                "SSO configuration adapter private implementation import", "sso_config_preservation.go",
                'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"\n',
                "non-test internal API files still import internal/license",
            ),
            (
                "SSO configuration adapter licensing bridge required", "sso_config_preservation.go",
                'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"\n',
                "API files outside bridge allowlist still import pkg/licensing",
            ),
            (
                "activity observer exact path", "reporting_activity_extra.go",
                "package api\n", "new production paid-domain implementation files are not baselined",
            ),
            (
                "activity observer private root import", "reporting_activity.go",
                'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n',
                "API root imports of internal/license remain",
            ),
            (
                "activity observer licensing bridge required", "reporting_activity.go",
                'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"\n',
                "API files outside bridge allowlist still import pkg/licensing",
            ),
            (
                "backup adapter exact path", "reporting_backup_health_extra.go",
                "package api\n", "new production paid-domain implementation files are not baselined",
            ),
            (
                "backup adapter private root import", "reporting_backup_health.go",
                'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n',
                "API root imports of internal/license remain",
            ),
            (
                "new reporting implementation", "reporting_subject_alerts_extra.go",
                "package api\n", "new production paid-domain implementation files are not baselined",
            ),
            (
                "exact path not prefix", "reporting_subject_alerts.go.extra.go",
                "package api\n", "new production paid-domain implementation files are not baselined",
            ),
            (
                "private root import", SUBJECT,
                'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n',
                "API root imports of internal/license remain",
            ),
            (
                "licensing bridge required", SUBJECT,
                'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"\n',
                "API files outside bridge allowlist still import pkg/licensing",
            ),
        ]
        for adapter in ("sso_load_state", "sso_management_admission"):
            variants.extend([
                (
                    f"{adapter} exact path", f"{adapter}_extra.go",
                    "package api\n", "new production paid-domain implementation files are not baselined",
                ),
                (
                    f"{adapter} not path prefix", f"{adapter}.go.extra.go",
                    "package api\n", "new production paid-domain implementation files are not baselined",
                ),
                (
                    f"{adapter} private root import", f"{adapter}.go",
                    'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license"\n',
                    "API root imports of internal/license remain",
                ),
                (
                    f"{adapter} private implementation import", f"{adapter}.go",
                    'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"\n',
                    "non-test internal API files still import internal/license",
                ),
                (
                    f"{adapter} licensing bridge required", f"{adapter}.go",
                    'package api\nimport _ "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"\n',
                    "API files outside bridge allowlist still import pkg/licensing",
                ),
            ])
        for name, filename, content, rejection in variants:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                fixture = Path(temporary)
                initialize_fixture(fixture)
                for directory in ("internal/api", "internal/license"):
                    for source in (ROOT / directory).rglob("*.go"):
                        if source.name.endswith("_test.go"):
                            continue
                        target = fixture / source.relative_to(ROOT)
                        target.parent.mkdir(parents=True, exist_ok=True)
                        shutil.copyfile(source, target)
                for filename_to_copy in (
                    "scripts/audit-private-boundary.sh",
                    "scripts/repo-boundary-paid-surface.allowlist",
                    "scripts/repo-boundary-private-implementation.baseline",
                ):
                    target = fixture / filename_to_copy
                    target.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(ROOT / filename_to_copy, target)
                if filename:
                    (fixture / "internal/api" / filename).write_text(content)
                result = subprocess.run(
                    ["bash", "scripts/audit-private-boundary.sh", "--enforce"],
                    cwd=fixture, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                    text=True, timeout=30, check=False,
                )
                self.assertEqual(result.returncode, 1 if rejection else 0, result.stdout)
                if rejection:
                    self.assertIn(rejection, result.stdout)
                    self.assertIn(filename, result.stdout)


if __name__ == "__main__":
    unittest.main()
