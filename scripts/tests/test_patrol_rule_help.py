"""Exercise the exact documented browser workaround with synthetic state only."""

import json
from pathlib import Path
import re
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDE = ROOT / "docs" / "AI.md"
SHIPPED_GUIDE = ROOT / "frontend-modern" / "public" / "docs" / "AI.md"


def removal_snippet():
    section = GUIDE.read_text().split("#### Remove a rule created by mistake\n", 1)[1]
    return re.search(r"```javascript\n(.*?)\n```", section, re.S).group(1)


HARNESS = r"""
const vm = require('node:vm');
let input = '';
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', async () => {
  const options = JSON.parse(input);
  const calls = [], messages = [], tables = [], confirmations = [];
  let cookie = options.cookie ?? 'pulse_org_id=org-a; pulse_csrf=synthetic-csrf';
  const context = {
    document: { get cookie() { return cookie; } },
    prompt: () => options.id === undefined ? null : options.id,
    confirm: message => {
      confirmations.push(message);
      if (options.changedOrg) cookie = 'pulse_org_id=org-b; pulse_csrf=synthetic-csrf';
      return options.confirm ?? true;
    },
    fetch: async (path, init) => {
      calls.push({path, ...init});
      const result = options.responses[calls.length - 1];
      if (!result) throw new Error('Unexpected additional request');
      if (result.networkFailure) throw new Error('Synthetic network failure');
      return {ok: result.status >= 200 && result.status < 300,
              status: result.status, json: async () => result.body};
    },
    console: {
      table: rows => tables.push(rows),
      info: message => messages.push({kind: 'info', message}),
      error: message => messages.push({kind: 'error', message}),
    },
  };
  await vm.runInNewContext(options.snippet, context, {timeout: 1000});
  process.stdout.write(JSON.stringify({calls, messages, tables, confirmations}));
});
"""


RULE = {"id": "rule-one", "created_from": "manual", "resource_id": "vm-1",
        "resource_name": "Test VM", "category": "backup", "description": "Test reason"}
OTHER_RULE = {**RULE, "id": "rule-two", "resource_id": "vm-2"}
DISMISSAL = {**RULE, "id": "finding_one", "created_from": "dismissed"}


class PatrolRuleHelpTests(unittest.TestCase):
    def run_snippet(self, **options):
        options.setdefault("responses", [{"status": 200, "body": [RULE, OTHER_RULE, DISMISSAL]}])
        options["snippet"] = removal_snippet()
        result = subprocess.run(["node", "-e", HARNESS], input=json.dumps(options),
                                text=True, capture_output=True, check=True, timeout=5)
        return json.loads(result.stdout)

    def assert_no_delete(self, result):
        self.assertFalse(any(call.get("method") == "DELETE" for call in result["calls"]))
        self.assertFalse(any(message["kind"] == "info" for message in result["messages"]))

    def test_shipped_help_matches_source(self):
        self.assertEqual(GUIDE.read_bytes(), SHIPPED_GUIDE.read_bytes())

    def test_rule_controls_precede_the_older_console_workaround(self):
        guide = GUIDE.read_text()
        current, older = guide.split("##### Older builds without rule controls\n", 1)
        self.assertIn("##### Use the rule controls when available", current)
        self.assertIn("**Patrol → Activity**", current)
        self.assertNotIn("```javascript", current)
        self.assertIn("In v6.4.5", older)
        self.assertIn("without an upgrade or restart", older)
        self.assertIn("not a way around denied access", older)

    def test_documented_controls_match_the_existing_production_flow(self):
        guide = GUIDE.read_text().split("##### Older builds without rule controls\n", 1)[0]
        ui = (ROOT / "frontend-modern/src/features/patrol/PatrolSuppressionRules.tsx").read_text()
        for label in ["Suppression rules", "Remove rule", "Remove this rule", "Cancel",
                      "Reload rules", "Rule ID", "All resources", "All categories"]:
            with self.subTest(label=label):
                self.assertIn(f"**{label}**", guide)
                self.assertIn(label, ui)
        for path in ["components/AI/FindingsPanel.tsx",
                     "features/patrol/PatrolAttentionWorkbench.tsx"]:
            creator = (ROOT / "frontend-modern/src" / path).read_text()
            self.assertIn("Manage suppression rules", creator)
        self.assertIn("**Manage suppression rules**", guide)
        self.assertIn("/patrol/activity#patrol-suppression-rules", ui)

    def test_uncertain_ui_results_require_readback_not_console_deletion(self):
        guide = GUIDE.read_text().split("##### Older builds without rule controls\n", 1)[0]
        self.assertIn("selected row disappears", guide)
        self.assertIn("**Rule removed**", guide)
        self.assertIn("check that exact\n   ID before trying again", guide)
        self.assertIn("lost response does not prove deletion failed", guide)
        self.assertIn("do not bypass the failure with the console workaround", guide)
        self.assertIn("organisation or access change", guide)
        self.assertIn("Other rules may\nstill cover the same scope", guide)

    def test_help_distinguishes_manual_rules_from_individual_dismissals(self):
        guide = GUIDE.read_text().split("##### Older builds without rule controls\n", 1)[0]
        self.assertIn("not a separately created\nrule", guide)
        self.assertIn("automatically reopen previously dismissed", guide)
        self.assertIn("other rules and finding history intact", guide)
        self.assertIn("**Reopen finding**", guide)
        self.assertIn("**Finding options\nand history**", guide)

    def test_cancel_is_read_only_and_hides_finding_decisions(self):
        result = self.run_snippet()
        self.assert_no_delete(result)
        self.assertEqual(len(result["calls"]), 1)
        self.assertEqual([row["id"] for row in result["tables"][0]], ["rule-one", "rule-two"])

    def test_unknown_blank_and_finding_ids_cannot_delete(self):
        for rule_id in ["missing", " ", "finding_one"]:
            with self.subTest(rule_id=rule_id):
                self.assert_no_delete(self.run_snippet(id=rule_id))

    def test_duplicate_id_cannot_delete(self):
        self.assert_no_delete(self.run_snippet(id="rule-one", responses=[
            {"status": 200, "body": [RULE, RULE]}]))

    def test_confirmation_cancel_is_read_only(self):
        result = self.run_snippet(id="rule-one", confirm=False)
        self.assert_no_delete(result)
        self.assertIn("Test VM", result["confirmations"][0])
        self.assertIn("backup", result["confirmations"][0])
        self.assertIn("Test reason", result["confirmations"][0])

    def test_missing_csrf_and_changed_organisation_stop(self):
        for options in [{"cookie": "pulse_org_id=org-a"}, {"changedOrg": True}]:
            with self.subTest(options=options):
                self.assert_no_delete(self.run_snippet(id="rule-one", **options))

    def test_failed_or_malformed_list_cannot_delete(self):
        for response in [{"status": 401}, {"status": 403}, {"status": 200, "body": {}},
                         {"networkFailure": True}]:
            with self.subTest(response=response):
                self.assert_no_delete(self.run_snippet(id="rule-one", responses=[response]))

    def test_delete_failure_or_lost_response_is_not_retried_or_reported_as_success(self):
        for response in [{"status": 403}, {"status": 404}, {"status": 503},
                         {"networkFailure": True}]:
            with self.subTest(response=response):
                result = self.run_snippet(id="rule-one", responses=[
                    {"status": 200, "body": [RULE, OTHER_RULE]}, response])
                self.assertEqual(len(result["calls"]), 2)
                self.assertFalse(any(m["kind"] == "info" for m in result["messages"]))

    def test_readback_must_succeed_and_show_selected_rule_absent(self):
        for response in [{"status": 200, "body": [RULE, OTHER_RULE]}, {"status": 403},
                         {"networkFailure": True}]:
            with self.subTest(response=response):
                result = self.run_snippet(id="rule-one", responses=[
                    {"status": 200, "body": [RULE, OTHER_RULE]}, {"status": 200}, response])
                self.assertEqual(len(result["calls"]), 3)
                self.assertFalse(any(m["kind"] == "info" for m in result["messages"]))

    def test_only_selected_rule_is_deleted_using_bound_browser_context(self):
        rule = {**RULE, "id": "rule /?other"}
        result = self.run_snippet(id=rule["id"], responses=[
            {"status": 200, "body": [rule, OTHER_RULE, DISMISSAL]}, {"status": 200},
            {"status": 200, "body": [OTHER_RULE, DISMISSAL]}])
        self.assertEqual(len(result["calls"]), 3)
        delete = result["calls"][1]
        self.assertEqual(delete["path"], "/api/ai/patrol/suppressions/rule%20%2F%3Fother")
        self.assertEqual(delete["headers"]["X-CSRF-Token"], "synthetic-csrf")
        for call in result["calls"]:
            self.assertEqual(call["credentials"], "same-origin")
            self.assertEqual(call["redirect"], "error")
            self.assertEqual(call["headers"]["X-Pulse-Org-ID"], "org-a")
            self.assertNotIn("X-API-Token", call["headers"])
        self.assertEqual(result["messages"][0]["kind"], "info")
        self.assertNotIn("synthetic-csrf", json.dumps(result["tables"]))
        self.assertNotIn("synthetic-csrf", json.dumps(result["messages"]))

    def test_wildcard_scope_is_explicit_in_list_and_confirmation(self):
        rule = {**RULE, "resource_id": "", "resource_name": "", "category": ""}
        result = self.run_snippet(id=rule["id"], confirm=False,
                                  responses=[{"status": 200, "body": [rule]}])
        self.assert_no_delete(result)
        self.assertEqual(result["tables"][0][0]["resource"], "Any resource")
        self.assertEqual(result["tables"][0][0]["category"], "Any category")
        self.assertIn("Any resource", result["confirmations"][0])
        self.assertIn("Any category", result["confirmations"][0])


if __name__ == "__main__":
    unittest.main()
