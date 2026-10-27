#!/usr/bin/env python3
"""Run the real paid-boundary audit against isolated source fixtures."""

from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SUBJECT = "reporting_subject_alerts.go"


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
        for name, filename, content, rejection in variants:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                fixture = Path(temporary)
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
