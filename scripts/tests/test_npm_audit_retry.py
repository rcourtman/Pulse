#!/usr/bin/env python3

from __future__ import annotations

import json
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
    def run_check(
        self, mode: str, *arguments: str, require: str = "true", report=None
    ):
        with tempfile.TemporaryDirectory() as directory:
            fake_bin = Path(directory)
            count = fake_bin / "count"
            calls = fake_bin / "calls"
            report_path = fake_bin / "report.json"
            report_path.write_text(json.dumps(report), encoding="utf-8")
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
                      captured-vulnerability)
                        if [ "$count" -eq 1 ]; then
                          cat "$FAKE_NPM_REPORT"
                        else
                          # A diagnostic re-query could return a different verdict.
                          printf '%s\n' "$clean"
                        fi
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
                    "FAKE_NPM_REPORT": str(report_path),
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
            ["audit --json --package-lock-only --omit=dev"],
        )

    def test_retries_an_unavailable_audit_endpoint(self) -> None:
        result, calls = self.run_check("transient-success", "all")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            calls,
            ["audit --json"] * 2,
        )
        self.assertIn("retrying", result.stdout)

    def test_does_not_retry_a_vulnerability_report(self) -> None:
        result, calls = self.run_check("vulnerability", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(
            calls,
            ["audit --json"],
        )
        self.assertIn("vulnerabilities present", result.stdout)
        self.assertNotIn("retrying", result.stdout)

    def test_vulnerability_verdict_precedes_a_transport_error(self) -> None:
        result, calls = self.run_check("vulnerability-with-error", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --json"])
        self.assertIn("vulnerabilities present", result.stdout)
        self.assertNotIn("retrying", result.stdout)

    def test_reports_the_original_advisory_without_an_unbounded_requery(self) -> None:
        nodes = [
            "node_modules/@eslint/config-array/node_modules/brace-expansion",
            "node_modules/@eslint/eslintrc/node_modules/brace-expansion",
            "node_modules/brace-expansion",
            "node_modules/eslint/node_modules/brace-expansion",
        ]
        report = {
            "metadata": {"vulnerabilities": {"high": 1, "total": 1}},
            "vulnerabilities": {
                "brace-expansion": {
                    "name": "brace-expansion",
                    "severity": "high",
                    "isDirect": False,
                    "range": "<=1.1.20 || 4.0.0 - 5.0.11",
                    "nodes": nodes,
                    "fixAvailable": True,
                    "via": [
                        {
                            "source": 123456,
                            "name": "brace-expansion",
                            "title": "Quadratic-time expansion causes CPU denial of service",
                            "url": "https://github.com/advisories/GHSA-q2hr-2g5m-vwhr",
                            "severity": "high",
                            "range": "<=1.1.20",
                            "unrecognised": "must not be printed",
                        },
                        "transitive-dependency",
                    ],
                }
            },
            "error": {"detail": "must not be printed"},
        }
        result, calls = self.run_check(
            "captured-vulnerability", "all", require="false", report=report
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --json"])
        finding_lines = [
            line.removeprefix("audit finding ")
            for line in result.stdout.splitlines()
            if line.startswith("audit finding ")
        ]
        self.assertEqual(len(finding_lines), 1, result.stdout)
        finding = json.loads(finding_lines[0])
        self.assertEqual(finding["nodes"], nodes)
        self.assertIs(finding["fixAvailable"], True)
        self.assertEqual(finding["name"], "brace-expansion")
        self.assertEqual(finding["range"], "<=1.1.20 || 4.0.0 - 5.0.11")
        expected_advisory = report["vulnerabilities"]["brace-expansion"]["via"][0]
        self.assertEqual(
            finding["via"][0],
            {key: value for key, value in expected_advisory.items() if key != "unrecognised"},
        )
        self.assertEqual(finding["via"][1], "transitive-dependency")
        self.assertNotIn("must not be printed", result.stdout)
        self.assertNotIn("no vulnerabilities", result.stdout)

    def test_missing_advisory_detail_cannot_trigger_a_requery_or_clear_the_failure(self) -> None:
        result, calls = self.run_check("vulnerability", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --json"])
        self.assertIn("package-level detail unavailable", result.stdout)

    def test_escapes_advisory_text_instead_of_emitting_workflow_commands(self) -> None:
        title = "unsafe title\n::error::injected annotation\x1b[31m"
        report = {
            "metadata": {"vulnerabilities": {"high": 1, "total": 1}},
            "vulnerabilities": {
                "dependency": {"name": "dependency", "via": [{"title": title}]}
            },
        }
        result, calls = self.run_check(
            "captured-vulnerability", "production", "--package-lock-only", report=report
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --json --package-lock-only --omit=dev"])
        finding_line = next(
            (line for line in result.stdout.splitlines() if line.startswith("audit finding ")),
            "audit finding {}",
        )
        finding = json.loads(finding_line.removeprefix("audit finding "))
        self.assertEqual(finding.get("via"), [{"title": title}], result.stdout)
        self.assertNotIn("\n::error::injected", result.stdout)
        self.assertNotIn("\x1b", result.stdout)

    def test_persistent_outage_fails_when_a_result_is_required(self) -> None:
        result, calls = self.run_check("transient-failure", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --json"] * 3)
        self.assertIn("could not reach", result.stdout)

    def test_persistent_outage_warns_for_an_unchanged_dependency_graph(self) -> None:
        result, calls = self.run_check(
            "transient-failure", "all", require="false"
        )
        self.assertEqual(result.returncode, 0)
        self.assertEqual(calls, ["audit --json"] * 3)
        self.assertIn("::warning::", result.stdout)

    def test_unparseable_output_never_passes_as_clean(self) -> None:
        result, calls = self.run_check("garbage", "all")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(calls, ["audit --json"] * 3)

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
                ],
                "require_env": True,
                "verdict_env": {
                    "COMPLETE_AUDIT_RESULT": "${{ steps.audit-complete.outcome }}",
                },
            },
            ".github/workflows/security-scan.yml": {
                "job": "npm-audit",
                "runs": [
                    'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" all --package-lock-only',
                    'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" production --package-lock-only',
                ],
                "require_env": False,
                "verdict_env": {
                    "COMPLETE_AUDIT_RESULT": "${{ steps.audit-complete.outcome }}",
                    "PRODUCTION_AUDIT_RESULT": "${{ steps.audit-production.outcome }}",
                },
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
                    contract["verdict_env"],
                )


if __name__ == "__main__":
    unittest.main()
