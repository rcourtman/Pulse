#!/usr/bin/env python3

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest
from unittest.mock import patch

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
                      captured-report)
                        # Malformed reports must remain the same on retry.
                        cat "$FAKE_NPM_REPORT"
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
                    "NPM_AUDIT_ATTEMPT_TIMEOUT": "60",
                    "NPM_AUDIT_MAX_SECONDS": "240",
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

    def test_fake_audit_budget_is_not_inherited_from_the_caller(self) -> None:
        # These are fixture controls, not a real advisory query. A caller's
        # exhausted/short budget must not prevent the canned verdict from
        # being read or change the exact one-request assertion.
        with patch.dict(os.environ, {
            "NPM_AUDIT_ATTEMPTS": "0",
            "NPM_AUDIT_ATTEMPT_TIMEOUT": "0",
            "NPM_AUDIT_MAX_SECONDS": "0",
            "NPM_AUDIT_RETRY_DELAY": "10000",
            "NPM_AUDIT_CMD": "/must-not-be-executed",
            "NPM_AUDIT_REQUIRE_RESULT": "false",
        }):
            for require, status, annotation in [
                ("true", 1, "::error::"), ("false", 0, "::warning::")
            ]:
                with self.subTest(require=require):
                    result, calls = self.run_check("vulnerability", "all", require=require)
                    self.assertEqual(result.returncode, status, result.stdout)
                    self.assertEqual(calls, ["audit --json"])
                    self.assertIn(annotation, result.stdout)
                    self.assertIn("vulnerabilities present", result.stdout)
                    self.assertNotIn("retrying", result.stdout)

    def test_shell_fixture_owns_its_default_and_explicit_timing_controls(self) -> None:
        env = os.environ.copy()
        env.update(NPM_AUDIT_ATTEMPTS="0", NPM_AUDIT_ATTEMPT_TIMEOUT="0",
                   NPM_AUDIT_MAX_SECONDS="0", NPM_AUDIT_RETRY_DELAY="10000",
                   NPM_AUDIT_CMD="/must-not-be-executed")
        result = subprocess.run(
            ["bash", str(ROOT / "scripts/tests/test-npm-audit-retry.sh")],
            cwd=ROOT, env=env, text=True, capture_output=True, check=False,
            timeout=90,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("all npm-audit-retry tests passed", result.stdout)
        self.assertIn("a hung audit is stopped at the per-attempt limit", result.stdout)
        self.assertIn("the wall-clock budget ends the retry sequence", result.stdout)

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
        self.assertIn("::error::", result.stdout)
        self.assertNotIn("retrying", result.stdout)

    def test_inherited_vulnerability_warns_for_an_unchanged_dependency_graph(self) -> None:
        result, calls = self.run_check("vulnerability", "all", require="false")
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual(calls, ["audit --json"])
        self.assertIn("vulnerabilities present", result.stdout)
        self.assertIn("::warning::", result.stdout)
        self.assertIn("base commit already has", result.stdout)
        self.assertNotIn("::error::", result.stdout)
        self.assertNotIn("no vulnerabilities", result.stdout)
        self.assertNotIn("retrying", result.stdout)

    def test_only_an_explicit_false_relaxes_a_vulnerability(self) -> None:
        for require in ["", "False", "0", "no", "true"]:
            with self.subTest(require=require):
                result, calls = self.run_check("vulnerability", "all", require=require)
                self.assertEqual(result.returncode, 1, result.stdout)
                self.assertEqual(calls, ["audit --json"], result.stdout + result.stderr)
                self.assertIn("::error::", result.stdout)
                self.assertNotIn("::warning::", result.stdout)

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
        # An inherited finding only warns, but it must be named exactly as a
        # blocking one would be.
        result, calls = self.run_check(
            "captured-vulnerability", "all", require="false", report=report
        )
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn("::warning::", result.stdout)
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

    def test_package_findings_count_even_when_summary_is_missing_or_zero(self) -> None:
        for metadata in [None, {}, {"vulnerabilities": {"total": 0}}]:
            for require in ["true", "false"]:
                with self.subTest(metadata=metadata, require=require):
                    report = {
                        "vulnerabilities": {
                            "dependency": {"name": "dependency", "severity": "high"}
                        }
                    }
                    if metadata is not None:
                        report["metadata"] = metadata
                    result, calls = self.run_check(
                        "captured-vulnerability", "all", require=require, report=report
                    )
                    self.assertEqual(
                        result.returncode, 1 if require == "true" else 0, result.stdout
                    )
                    self.assertEqual(calls, ["audit --json"], result.stdout)
                    self.assertIn("vulnerabilities present", result.stdout)
                    self.assertIn("package_records=1", result.stdout)
                    self.assertNotIn("no vulnerabilities", result.stdout)

    def test_positive_severity_counts_count_without_a_consistent_total(self) -> None:
        for severity in ["info", "low", "moderate", "high", "critical"]:
            for total in [None, 0]:
                for require in ["true", "false"]:
                    with self.subTest(severity=severity, total=total, require=require):
                        counts = {severity: 1}
                        if total is not None:
                            counts["total"] = total
                        report = {
                            "metadata": {"vulnerabilities": counts},
                            "error": {"code": "ETIMEDOUT"},
                        }
                        result, calls = self.run_check(
                            "captured-vulnerability", "production",
                            require=require, report=report,
                        )
                        self.assertEqual(
                            result.returncode, 1 if require == "true" else 0, result.stdout
                        )
                        self.assertEqual(calls, ["audit --json --omit=dev"], result.stdout)
                        self.assertIn("vulnerabilities present", result.stdout)

    def test_malformed_zero_verdicts_cannot_pass_as_clean(self) -> None:
        zero_counts = dict.fromkeys(
            ["info", "low", "moderate", "high", "critical", "total"], 0
        )
        reports = [
            {"metadata": {"vulnerabilities": {**zero_counts, "total": value}}}
            for value in [None, False, -1, 0.0, "0", [], {}]
        ]
        reports.extend([
            {"metadata": {"vulnerabilities": {**zero_counts, "high": "0"}}},
            {"metadata": {"vulnerabilities": {**zero_counts, "high": None}}},
            {"metadata": {"vulnerabilities": {"total": 0}}},
            {"metadata": {"vulnerabilities": zero_counts}, "vulnerabilities": []},
            {"metadata": {"vulnerabilities": zero_counts}, "vulnerabilities": None},
            {
                "metadata": {"vulnerabilities": zero_counts},
                "error": {"code": "ENOAUDIT"},
            },
        ])
        for report in reports:
            for require in ["true", "false"]:
                with self.subTest(report=report, require=require):
                    result, calls = self.run_check(
                        "captured-report", "all", require=require, report=report
                    )
                    self.assertEqual(
                        result.returncode, 1 if require == "true" else 0, result.stdout
                    )
                    self.assertEqual(calls, ["audit --json"] * 3, result.stdout)
                    self.assertNotIn("no vulnerabilities", result.stdout)
                    self.assertIn("could not reach", result.stdout)
                    if require == "false":
                        self.assertIn("::warning::", result.stdout)

    def test_summary_cannot_emit_untrusted_metadata_as_workflow_commands(self) -> None:
        unsafe = "0\n::error::injected metadata\x1b[31m"
        report = {"metadata": {"vulnerabilities": {"total": 1, "high": unsafe}}}
        result, calls = self.run_check("captured-vulnerability", "all", report=report)
        self.assertEqual(result.returncode, 1, result.stdout)
        self.assertEqual(calls, ["audit --json"])
        self.assertIn("vulnerabilities present", result.stdout)
        self.assertIn("high=unknown", result.stdout)
        self.assertNotIn("injected metadata", result.stdout)
        self.assertNotIn("\x1b", result.stdout)

    def test_passes_a_complete_zero_summary_and_empty_package_map(self) -> None:
        report = {
            "metadata": {
                "vulnerabilities": dict.fromkeys(
                    ["info", "low", "moderate", "high", "critical", "total"], 0
                )
            },
            "vulnerabilities": {},
        }
        result, calls = self.run_check("captured-report", "all", report=report)
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual(calls, ["audit --json"])
        self.assertIn("no vulnerabilities", result.stdout)

    def test_persistent_outage_fails_when_a_result_is_required(self) -> None:
        for require in ["true", "", "False"]:
            with self.subTest(require=require):
                result, calls = self.run_check("transient-failure", "all", require=require)
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
