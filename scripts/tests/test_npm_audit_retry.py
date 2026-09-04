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
    def run_check(self, mode: str, *arguments: str, require: str = "true"):
        with tempfile.TemporaryDirectory() as directory:
            fake_bin = Path(directory)
            count = fake_bin / "count"
            calls = fake_bin / "calls"
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
                    clean='{"metadata":{"vulnerabilities":{"info":0,"low":0,"moderate":0,"high":0,"critical":0,"total":0}}}'
                    unavailable='{"error":{"code":"ENOAUDIT","summary":"503 Service Unavailable"}}'
                    vulnerable='{"metadata":{"vulnerabilities":{"info":0,"low":0,"moderate":0,"high":1,"critical":0,"total":1}}}'
                    vulnerable_with_error='{"error":{"code":"ETIMEDOUT"},"metadata":{"vulnerabilities":{"info":0,"low":0,"moderate":0,"high":1,"critical":0,"total":1}}}'
                    case "$FAKE_NPM_MODE" in
                      success)
                        printf '%s\n' "$clean"
                        exit 0
                        ;;
                      transient-success)
                        if [ "$count" -eq 1 ]; then
                          printf '%s\n' "$unavailable"
                          exit 1
                        fi
                        printf '%s\n' "$clean"
                        exit 0
                        ;;
                      transient-failure)
                        printf '%s\n' "$unavailable"
                        exit 42
                        ;;
                      vulnerability)
                        printf '%s\n' "$vulnerable"
                        exit 1
                        ;;
                      vulnerability-with-error)
                        printf '%s\n' "$vulnerable_with_error"
                        exit 1
                        ;;
                      garbage)
                        echo 'not json'
                        exit 1
                        ;;
                    esac
                    exit 64
                    """
                ),
                encoding="utf-8",
            )
            fake_npm.chmod(0o755)
            env = os.environ.copy()
            env.update(
                {
                    "FAKE_NPM_CALLS": str(calls),
                    "FAKE_NPM_COUNT": str(count),
                    "FAKE_NPM_MODE": mode,
                    "NPM_AUDIT_ATTEMPTS": "3",
                    "NPM_AUDIT_CMD": str(fake_npm),
                    "NPM_AUDIT_REQUIRE_RESULT": require,
                    "NPM_AUDIT_RETRY_DELAY": "0",
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
            return result, recorded_calls

    def test_passes_a_clean_production_audit_and_forwards_arguments(self) -> None:
        result, calls = self.run_check(
            "success", "production", "--package-lock-only"
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            calls,
            ["audit --json --fetch-timeout=60000 --package-lock-only --omit=dev"],
        )

    def test_retries_an_unavailable_audit_endpoint(self) -> None:
        result, calls = self.run_check("transient-success", "all")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            calls,
            ["audit --json --fetch-timeout=60000"] * 2,
        )
        self.assertIn("retrying", result.stdout)

    def test_does_not_retry_a_vulnerability_report(self) -> None:
        result, calls = self.run_check("vulnerability", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(
            calls,
            [
                "audit --json --fetch-timeout=60000",
                "audit --fetch-timeout=60000",
            ],
        )
        self.assertIn("vulnerabilities present", result.stdout)
        self.assertNotIn("retrying", result.stdout)

    def test_vulnerability_verdict_precedes_a_transport_error(self) -> None:
        result, calls = self.run_check("vulnerability-with-error", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(calls), 2)
        self.assertIn("vulnerabilities present", result.stdout)
        self.assertNotIn("retrying", result.stdout)

    def test_persistent_outage_fails_when_a_result_is_required(self) -> None:
        result, calls = self.run_check("transient-failure", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --json --fetch-timeout=60000"] * 3)
        self.assertIn("could not reach", result.stdout)

    def test_persistent_outage_warns_for_an_unchanged_dependency_graph(self) -> None:
        result, calls = self.run_check(
            "transient-failure", "all", require="false"
        )
        self.assertEqual(result.returncode, 0)
        self.assertEqual(calls, ["audit --json --fetch-timeout=60000"] * 3)
        self.assertIn("::warning::", result.stdout)

    def test_unparseable_output_never_passes_as_clean(self) -> None:
        result, calls = self.run_check("garbage", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --json --fetch-timeout=60000"] * 3)

    def test_rejects_an_unknown_scope(self) -> None:
        result, calls = self.run_check("success", "unknown")
        self.assertEqual(result.returncode, 2)
        self.assertEqual(calls, [])

    def test_all_workflow_audits_use_the_retry_boundary(self) -> None:
        expected = {
            ".github/workflows/build-and-test.yml": {
                "job": "frontend",
                "runs": [
                    'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" all',
                    'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" production',
                ],
                "require_env": True,
            },
            ".github/workflows/security-scan.yml": {
                "job": "npm-audit",
                "runs": [
                    'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" all --package-lock-only',
                    'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" production --package-lock-only',
                ],
                "require_env": False,
            },
        }
        for relative, contract in expected.items():
            with self.subTest(workflow=relative):
                workflow = yaml.safe_load(
                    (ROOT / relative).read_text(encoding="utf-8")
                )
                steps = workflow["jobs"][contract["job"]]["steps"]
                audits = [
                    step
                    for step in steps
                    if step.get("id") in {"audit-complete", "audit-production"}
                ]
                self.assertEqual([step["run"] for step in audits], contract["runs"])
                self.assertTrue(
                    all(step.get("continue-on-error") is True for step in audits)
                )
                for step in audits:
                    env = step.get("env", {})
                    self.assertEqual(
                        "NPM_AUDIT_REQUIRE_RESULT" in env,
                        contract["require_env"],
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
