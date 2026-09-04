#!/usr/bin/env python3

from __future__ import annotations

import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "npm-audit-retry.sh"


class NpmAuditRetryTest(unittest.TestCase):
    def run_check(self, mode: str, *arguments: str):
        with tempfile.TemporaryDirectory() as directory:
            fake_bin = Path(directory)
            count = fake_bin / "count"
            calls = fake_bin / "calls"
            sleep_calls = fake_bin / "sleep-calls"
            count.write_text("0\n", encoding="utf-8")
            fake_npm = fake_bin / "npm"
            fake_npm.write_text(
                textwrap.dedent(
                    """\
                    #!/bin/sh
                    count=$(cat "$FAKE_NPM_COUNT")
                    count=$((count + 1))
                    printf '%s\n' "$count" > "$FAKE_NPM_COUNT"
                    printf '%s\n' "$*" >> "$FAKE_NPM_CALLS"
                    case "$FAKE_NPM_MODE" in
                      success)
                        echo 'found 0 vulnerabilities'
                        exit 0
                        ;;
                      transient-success)
                        if [ "$count" -eq 1 ]; then
                          echo 'npm warn audit network timeout at: https://registry.npmjs.org/-/npm/v1/security/advisories/bulk'
                          echo 'npm error audit endpoint returned an error'
                          exit 1
                        fi
                        echo 'found 0 vulnerabilities'
                        exit 0
                        ;;
                      transient-failure)
                        echo 'npm error code ETIMEDOUT'
                        echo 'npm error audit endpoint returned an error'
                        exit 42
                        ;;
                      vulnerability)
                        echo '# npm audit report'
                        echo 'example  <2.0.0'
                        echo '1 high severity vulnerability'
                        exit 1
                        ;;
                      vulnerability-and-transient)
                        echo '# npm audit report'
                        echo 'example  <2.0.0'
                        echo '1 high severity vulnerability'
                        echo 'npm error code ETIMEDOUT'
                        exit 1
                        ;;
                    esac
                    exit 64
                    """
                ),
                encoding="utf-8",
            )
            fake_npm.chmod(0o755)
            fake_sleep = fake_bin / "sleep"
            fake_sleep.write_text(
                "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$FAKE_SLEEP_CALLS\"\n",
                encoding="utf-8",
            )
            fake_sleep.chmod(0o755)
            env = os.environ.copy()
            env.update(
                {
                    "FAKE_NPM_CALLS": str(calls),
                    "FAKE_NPM_COUNT": str(count),
                    "FAKE_NPM_MODE": mode,
                    "FAKE_SLEEP_CALLS": str(sleep_calls),
                    "PATH": f"{directory}:{env['PATH']}",
                }
            )
            result = subprocess.run(
                [str(SCRIPT), *arguments],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            recorded_calls = (
                calls.read_text(encoding="utf-8").splitlines()
                if calls.exists()
                else []
            )
            recorded_sleeps = (
                sleep_calls.read_text(encoding="utf-8").splitlines()
                if sleep_calls.exists()
                else []
            )
            return result, recorded_calls, recorded_sleeps

    def test_passes_a_clean_audit_without_retry(self) -> None:
        result, calls, sleeps = self.run_check("success", "--omit=dev")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, ["audit --fetch-timeout=60000 --omit=dev"])
        self.assertEqual(sleeps, [])

    def test_retries_a_transient_audit_endpoint_failure(self) -> None:
        result, calls, sleeps = self.run_check(
            "transient-success", "--package-lock-only"
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            calls,
            [
                "audit --fetch-timeout=60000 --package-lock-only",
                "audit --fetch-timeout=60000 --package-lock-only",
            ],
        )
        self.assertEqual(sleeps, ["5"])
        self.assertIn("retrying", result.stderr)

    def test_does_not_retry_a_vulnerability_report(self) -> None:
        result, calls, sleeps = self.run_check("vulnerability")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --fetch-timeout=60000"])
        self.assertEqual(sleeps, [])
        self.assertIn("1 high severity vulnerability", result.stdout)
        self.assertNotIn("retrying", result.stderr)

    def test_vulnerability_report_takes_precedence_over_transport_marker(
        self,
    ) -> None:
        result, calls, sleeps = self.run_check("vulnerability-and-transient")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --fetch-timeout=60000"])
        self.assertEqual(sleeps, [])
        self.assertIn("1 high severity vulnerability", result.stdout)
        self.assertNotIn("retrying", result.stderr)

    def test_persistent_registry_failure_remains_fatal(self) -> None:
        result, calls, sleeps = self.run_check("transient-failure")
        self.assertEqual(result.returncode, 42)
        self.assertEqual(len(calls), 3)
        self.assertEqual(sleeps, ["5", "5"])
        self.assertIn("failed after 3 attempts", result.stderr)

    def test_all_workflow_audits_use_the_retry_boundary(self) -> None:
        for relative, job_name in (
            (".github/workflows/build-and-test.yml", "frontend"),
            (".github/workflows/security-scan.yml", "npm-audit"),
        ):
            with self.subTest(workflow=relative):
                workflow = yaml.safe_load(
                    (ROOT / relative).read_text(encoding="utf-8")
                )
                steps = workflow["jobs"][job_name]["steps"]
                if job_name == "frontend":
                    install = next(
                        step
                        for step in steps
                        if step.get("name") == "Install frontend dependencies"
                    )
                    self.assertEqual(install["run"], "npm ci --no-audit")
                audits = [
                    step
                    for step in steps
                    if step.get("id") in {"audit-complete", "audit-production"}
                ]
                self.assertEqual(len(audits), 2)
                self.assertTrue(
                    all(
                        step["run"].startswith(
                            '"${GITHUB_WORKSPACE}/scripts/npm-audit-retry.sh"'
                        )
                        and step.get("continue-on-error") is True
                        for step in audits
                    )
                )
                verdict = steps[-1]
                self.assertIn("Require", verdict["name"])
                self.assertEqual(verdict["if"], "${{ !cancelled() }}")
                self.assertEqual(
                    verdict["env"],
                    {
                        "COMPLETE_AUDIT_RESULT": "${{ steps.audit-complete.outcome }}",
                        "PRODUCTION_AUDIT_RESULT": "${{ steps.audit-production.outcome }}",
                    },
                )


if __name__ == "__main__":
    unittest.main()
