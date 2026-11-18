"""Check the dependency graph and audit verdicts used in frontend CI."""

from pathlib import Path
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"


class FrontendDependencyAuditTest(unittest.TestCase):
    def test_pull_request_audits_the_locked_frontend_graph(self):
        jobs = yaml.safe_load((WORKFLOWS / "build-and-test.yml").read_text())["jobs"]
        self.assertEqual(jobs["changes"]["outputs"]["frontend_deps"],
                         "${{ steps.filter.outputs.frontend_deps }}")
        producer = jobs["frontend-build"]
        self.assertEqual(producer["timeout-minutes"], 30)
        steps = {step["name"]: step for step in producer["steps"]}
        self.assertIn("Check Solid serialization dependency edges", steps)
        self.assertEqual(steps["Install frontend dependencies"]["run"], "npm ci --no-audit")
        audit = steps["Audit complete frontend dependency graph"]
        self.assertEqual(audit["id"], "audit-complete")
        self.assertIs(audit["continue-on-error"], True)
        self.assertEqual(audit["env"]["NPM_AUDIT_REQUIRE_RESULT"],
                         "${{ needs.changes.outputs.frontend_deps }}")
        self.assertEqual(audit["run"], 'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" all')
        verdict = steps["Require frontend dependency audit"]
        self.assertEqual(verdict["if"], "${{ !cancelled() }}")
        self.assertEqual(verdict["env"]["COMPLETE_AUDIT_RESULT"], "${{ steps.audit-complete.outcome }}")

    def test_scheduled_audit_requires_both_reports(self):
        jobs = yaml.safe_load((WORKFLOWS / "security-scan.yml").read_text())["jobs"]
        steps = {step["name"]: step for step in jobs["npm-audit"]["steps"]}
        production = steps["Audit production dependencies"]
        self.assertEqual(production["id"], "audit-production")
        self.assertEqual(production["run"],
                         'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" production --package-lock-only')
        self.assertEqual(steps["Require dependency audits"]["env"], {
            "COMPLETE_AUDIT_RESULT": "${{ steps.audit-complete.outcome }}",
            "PRODUCTION_AUDIT_RESULT": "${{ steps.audit-production.outcome }}",
        })

    def test_runner_keeps_strict_findings_and_wall_clock_bounds(self):
        runner = (ROOT / "scripts/npm-audit-retry.sh").read_text()
        self.assertNotIn("--audit-level", runner)
        for expression in (
            "NPM_AUDIT_REQUIRE_RESULT:-true",
            'AUDIT_ARGS=(audit --json "$@")',
            "AUDIT_ARGS+=(--omit=dev)",
            "if type(value) is int and value >= 0:",
            "has_findings = isinstance(findings, dict) and bool(findings)",
            "if has_findings or any(count > 0 for count in counts.values()):",
            'print("vulnerable")',
            'isinstance(report, dict) and not report.get("error")',
            "len(counts) == len(count_names) and all(count == 0 for count in counts.values())",
            'and ("vulnerabilities" not in report or isinstance(findings, dict))',
            'if [ "${REQUIRE_RESULT}" = "false" ]; then',
            "NPM_AUDIT_MAX_SECONDS", "NPM_AUDIT_ATTEMPT_TIMEOUT",
            "export npm_config_fetch_retries=0", "DEADLINE=",
        ):
            with self.subTest(expression=expression):
                self.assertIn(expression, runner)


if __name__ == "__main__":
    unittest.main()
