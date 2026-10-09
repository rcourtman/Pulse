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
