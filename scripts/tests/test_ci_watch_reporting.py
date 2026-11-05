#!/usr/bin/env python3
"""Offline lifecycle regressions; never run the memory/signing checks or gh.

The workflow entry commands also run end-to-end with an inert gh executable.
Fixtures include a human reply arriving at the final mutation boundary: a
passing watch can record its result, but cannot hide or resolve that reply.
Discovered by scripts/tests/run.sh and ordinary script-smoke CI.
"""

from copy import deepcopy
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / ".github/scripts/report-ci-watch.py"
spec = importlib.util.spec_from_file_location("watch_reporter", SCRIPT)
reporter = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = reporter
spec.loader.exec_module(reporter)
OLD = reporter.Run(100, 1, "a" * 40)
NOW = reporter.Run(200, 1, "b" * 40)
# Literal GraphQL Bot response observed on 2026-11-04 for Actions' public
# database ID. GraphQL uses `github-actions`, unlike REST's `[bot]` suffix.
# Do not derive API fixtures from the production constant being checked.
GRAPHQL_ACTIONS_BOT = {"__typename": "Bot", "login": "github-actions", "databaseId": 41898282}


def connection(nodes):
    return {"totalCount": len(nodes), "pageInfo": {"hasNextPage": False}, "nodes": nodes}


def issue(watch, number=10):
    return {"id": f"I_{number}", "number": number, "title": reporter.WATCHES[watch][1],
            "body": reporter.body(watch, "failure", OLD, opening=True), "author": deepcopy(GRAPHQL_ACTIONS_BOT),
            "state": "OPEN", "locked": False, "editor": None, "lastEditedAt": None,
            "milestone": None, "labels": {"totalCount": 0}, "assignees": {"totalCount": 0},
            "updatedAt": "2026-11-04T00:00:00Z", "comments": connection([]), "timelineItems": connection([])}


def reply(row, watch, result="success", run=NOW, author=None):
    comment = {"id": f"C_{len(row['comments']['nodes'])}", "body": reporter.body(watch, result, run),
               "author": deepcopy(author or GRAPHQL_ACTIONS_BOT), "editor": None, "lastEditedAt": None}
    row["comments"]["nodes"].append(comment)
    row["comments"]["totalCount"] += 1
    row["timelineItems"]["nodes"].append({"__typename": "IssueComment", "id": comment["id"]})
    row["timelineItems"]["totalCount"] += 1
    return comment


class GitHub:
    def __init__(self, rows=()):
        self.rows = deepcopy(list(rows))
        self.calls, self.writes = [], []
        self.list_count = self.snap_count = 0
        self.before_list = self.before_snapshot = self.before_write = None

    def __call__(self, query, variables):
        self.calls.append((query, deepcopy(variables)))
        if query.startswith("query($after:"):
            self.list_count += 1
            if self.before_list:
                self.before_list(self)
            # This double deliberately includes human/legacy rows too; they
            # cannot pass ownership, even with a copied marker or title.
            offset = int(variables["after"] or 0)
            page = deepcopy(self.rows[offset:offset + 100])
            return {"repository": {"issues": {
                "totalCount": len(self.rows), "nodes": page,
                "pageInfo": {"hasNextPage": offset + 100 < len(self.rows), "endCursor": str(offset + 100)},
            }}}
        if query.startswith("query($id:"):
            self.snap_count += 1
            if self.before_snapshot:
                self.before_snapshot(self)
            return {"node": deepcopy(next(r for r in self.rows if r["id"] == variables["id"]))}
        if self.before_write:
            self.before_write(self)
        self.writes.append((query, deepcopy(variables)))
        if "createIssue(" in query:
            row = issue("memory" if "memory-regression.yml" in variables["body"] else "signing", 11)
            row.update(title=variables["title"], body=variables["body"])
            self.rows.append(row)
            return {"createIssue": {"issue": {"id": row["id"]}}}
        if "addComment(" in query:
            row = next(r for r in self.rows if r["id"] == variables["id"])
            watch = "memory" if "memory-regression.yml" in variables["body"] else "signing"
            result, run = reporter.parse_body(variables["body"], watch)
            comment = reply(row, watch, result, run)
            return {"addComment": {"commentEdge": {"node": {"id": comment["id"]}}}}
        raise AssertionError("unexpected API, especially state/edit/close/delete")


def environment(watch):
    return {"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "schedule", "GITHUB_REF": "refs/heads/main",
            "GITHUB_REPOSITORY": reporter.REPOSITORY, "GITHUB_SERVER_URL": "https://github.com",
            "GITHUB_WORKFLOW_REF": f"rcourtman/Pulse/.github/workflows/{reporter.WATCHES[watch][0]}@refs/heads/main",
            "GITHUB_RUN_ID": str(NOW.number), "GITHUB_RUN_ATTEMPT": str(NOW.attempt), "GITHUB_SHA": NOW.sha}


class WatchReporting(unittest.TestCase):
    def watches(self):
        return reporter.WATCHES

    def denied(self, api, watch):
        with self.assertRaises((reporter.EvidenceError, KeyError, TypeError, AttributeError)):
            reporter.report(api, watch, "success", NOW)
        self.assertEqual(api.writes, [])

    def test_normal_create_recover_and_recurrence_without_state_mutation(self):
        for watch in self.watches():
            with self.subTest(watch=watch):
                api = GitHub()
                reporter.report(api, watch, "failure", OLD)
                self.assertEqual(len(api.rows), 1)
                self.assertEqual(api.list_count, 2)
                self.assertIn("createIssue(", api.writes[0][0])
                reporter.report(api, watch, "success", NOW)
                self.assertEqual(len(api.writes), 2)
                self.assertIn("addComment(", api.writes[1][0])
                self.assertIn("not resolution of reported symptoms", api.writes[1][1]["body"])
                reporter.report(api, watch, "failure", reporter.Run(201, 1, "c" * 40))
                self.assertEqual(len(api.writes), 3)
                self.assertEqual(api.rows[0]["state"], "OPEN")
                self.assertEqual(api.rows[0]["body"], reporter.body(watch, "failure", OLD, opening=True))

    def test_literal_graphql_actions_author_is_required_for_issue_and_comments(self):
        for watch in self.watches():
            # Validate both the issue author and a previously posted comment.
            row = issue(watch)
            reply(row, watch)
            with self.subTest(watch=watch, identity="observed GraphQL Bot"):
                api = GitHub([row])
                reporter.report(api, watch, "failure", reporter.Run(300, 1, "c" * 40))
                self.assertEqual(len(api.writes), 1)
                self.assertIn("addComment(", api.writes[0][0])
                self.assertEqual(api.rows[0]["state"], "OPEN")
            for field in ("issue", "comment"):
                for actor in ({**GRAPHQL_ACTIONS_BOT, "login": "github-actions[bot]"},
                              {**GRAPHQL_ACTIONS_BOT, "databaseId": 1},
                              {**GRAPHQL_ACTIONS_BOT, "__typename": "User"},
                              {"login": "github-actions", "__typename": "Bot"}):
                    with self.subTest(watch=watch, field=field, actor=actor):
                        changed = deepcopy(row)
                        target = changed if field == "issue" else changed["comments"]["nodes"][0]
                        target["author"] = actor
                        self.denied(GitHub([changed]), watch)

    def test_human_title_collision_and_legacy_bot_are_never_adopted(self):
        for watch in self.watches():
            for actor in ({"login": "reporter", "__typename": "User"}, GRAPHQL_ACTIONS_BOT):
                with self.subTest(watch=watch, actor=actor):
                    row = issue(watch)
                    row.update(author=actor, body="Unresolved original symptoms; not a marked watch report.")
                    api = GitHub([row])
                    reporter.report(api, watch, "success", NOW)
                    self.assertEqual(api.writes, [])
                    reporter.report(api, watch, "failure", NOW)
                    self.assertEqual(len(api.writes), 1)
                    self.assertIn("createIssue(", api.writes[0][0])
                    self.assertEqual(api.rows[0], row)

    def test_copied_marker_cannot_grant_ownership_to_a_human_or_another_bot(self):
        for watch in self.watches():
            for actor in ({"login": "reporter", "__typename": "User"},
                          {**GRAPHQL_ACTIONS_BOT, "databaseId": 1}, {**GRAPHQL_ACTIONS_BOT, "login": "pulse-triage[bot]"}):
                row = issue(watch)
                row["author"] = actor
                self.denied(GitHub([row]), watch)

    def test_any_human_or_community_reply_preserves_the_report(self):
        for watch in self.watches():
            for actor in ({"login": "reporter", "__typename": "User"},
                          {"login": "pulse-triage[bot]", "__typename": "Bot", "databaseId": 1}):
                row = issue(watch)
                reply(row, watch, author=actor)
                self.denied(GitHub([row]), watch)

    def test_ownership_edits_labels_assignments_locks_milestones_and_state(self):
        for watch in self.watches():
            changes = {"title": "Taken over", "body": issue(watch)["body"] + "\nA human addition",
                       "locked": True, "editor": {"login": "maintainer"}, "lastEditedAt": "2026-11-04T01:00:00Z",
                       "milestone": {"id": "M_1"}, "labels": {"totalCount": 1},
                       "assignees": {"totalCount": 1}, "state": "CLOSED"}
            for key, value in changes.items():
                with self.subTest(watch=watch, field=key):
                    row = issue(watch)
                    row[key] = value
                    self.denied(GitHub([row]), watch)

    def test_removed_takeover_labels_and_crossreferences_remain_takeovers(self):
        for watch in self.watches():
            for kind in ("UnlabeledEvent", "AssignedEvent", "CrossReferencedEvent", "RenamedTitleEvent", "ReopenedEvent"):
                row = issue(watch)
                row["timelineItems"] = connection([{"__typename": kind}])
                self.denied(GitHub([row]), watch)

    def test_incomplete_or_mismatched_conversation_is_not_empty(self):
        for watch in self.watches():
            for field in ("comments", "timelineItems"):
                for change in ({"totalCount": 1}, {"pageInfo": {"hasNextPage": True}},
                               {"nodes": None}, {"pageInfo": {}}, {"totalCount": None}):
                    row = issue(watch)
                    row[field].update(change)
                    self.denied(GitHub([row]), watch)
            row = issue(watch)
            reply(row, watch)
            row["timelineItems"] = connection([])
            self.denied(GitHub([row]), watch)

    def test_edited_deleted_unknown_and_nonreporter_comments_fail_closed(self):
        for watch in self.watches():
            for change in ({"body": "Another unresolved symptom"}, {"lastEditedAt": "2026-11-04"},
                           {"editor": {"login": "maintainer"}}, {"author": None}):
                row = issue(watch)
                reply(row, watch).update(change)
                self.denied(GitHub([row]), watch)

    def test_unknown_required_fields_fail_closed(self):
        for watch in self.watches():
            for field in issue(watch):
                row = issue(watch)
                del row[field]
                api = GitHub([issue(watch)])
                def omit(api, row=row):
                    api.rows = [row]
                api.before_snapshot = omit
                with self.subTest(watch=watch, field=field):
                    self.denied(api, watch)

    def test_multiple_marked_reports_are_ambiguous_not_first_match(self):
        for watch in self.watches():
            self.denied(GitHub([issue(watch), issue(watch, 12)]), watch)

    def test_complete_inventory_paginates_without_an_actor_filter_or_title_search(self):
        for watch in self.watches():
            rows = []
            for n in range(100):
                row = issue(watch, n + 1000)
                row["body"] = "unrelated legacy report"
                row["author"] = {"login": "reporter", "__typename": "User"}
                rows.append(row)
            api = GitHub([*rows, issue(watch)])
            reporter.report(api, watch, "success", NOW)
            self.assertEqual(api.list_count, 2)
            self.assertEqual(len(api.writes), 1)
            # Enumerate all open issues rather than rely on REST/GraphQL login
            # aliases in createdBy. Ownership is checked on the returned node.
            self.assertIn("states:OPEN", api.calls[0][0])
            self.assertNotIn("filterBy", api.calls[0][0])
            self.assertNotIn("createdBy", api.calls[0][0])
            self.assertNotIn("search(", api.calls[0][0])

    def test_inventory_changes_duplicate_ids_missing_cursor_and_partial_counts(self):
        for watch in self.watches():
            good = GitHub([issue(watch)])
            def bad(response):
                def call(query, variables):
                    result = good(query, variables)
                    if query.startswith("query($after:"):
                        result["repository"]["issues"].update(response)
                    return result
                call.writes = good.writes
                return call
            for response in ({"totalCount": 2}, {"totalCount": None}, {"nodes": None},
                             {"nodes": [issue(watch), issue(watch)], "totalCount": 2},
                             {"pageInfo": {"hasNextPage": True, "endCursor": None}},
                             {"pageInfo": {"hasNextPage": True, "endCursor": "0"}},
                             {"totalCount": 1001}):
                self.denied(bad(response), watch)

    def test_report_creation_race_never_duplicates_or_mutates_the_new_report(self):
        for watch in self.watches():
            api = GitHub()
            def race(api):
                if api.list_count == 2:
                    api.rows.append(issue(watch))
            api.before_list = race
            with self.assertRaises(reporter.EvidenceError):
                reporter.report(api, watch, "failure", NOW)
            self.assertEqual(api.writes, [])

    def test_human_reply_or_takeover_between_reads_prevents_a_comment(self):
        for watch in self.watches():
            for mode in ("comment", "label", "body", "update"):
                api = GitHub([issue(watch)])
                def race(api):
                    if api.snap_count == 2:
                        row = api.rows[0]
                        if mode == "comment":
                            reply(row, watch, author={"login": "reporter"})
                        elif mode == "label":
                            row["labels"]["totalCount"] = 1
                        elif mode == "body":
                            row["body"] += "\nUnresolved"
                        else:
                            row["updatedAt"] = "2026-11-04T01:01:01Z"
                api.before_snapshot = race
                self.denied(api, watch)

    def test_human_arriving_after_final_read_cannot_be_hidden_or_resolved(self):
        for watch in self.watches():
            api = GitHub([issue(watch)])
            def race(api):
                reply(api.rows[0], watch, author={"login": "reporter", "__typename": "User"})["body"] = "Still broken"
            api.before_write = race
            reporter.report(api, watch, "success", NOW)
            self.assertEqual(len(api.writes), 1)
            self.assertIn("addComment(", api.writes[0][0])
            self.assertEqual(api.rows[0]["state"], "OPEN")
            self.assertEqual(api.rows[0]["comments"]["nodes"][0]["body"], "Still broken")
            self.assertNotIn("state", api.writes[0][1])

    def test_repeated_result_lost_response_and_older_run_are_not_reposted(self):
        for watch in self.watches():
            row = issue(watch)
            reply(row, watch)
            api = GitHub([row])
            reporter.report(api, watch, "success", NOW)
            reporter.report(api, watch, "failure", OLD)
            reporter.report(api, watch, "success", reporter.Run(300, 1, NOW.sha))
            self.assertEqual(api.writes, [])

    def test_skipped_cancelled_and_repeated_failure_do_not_mutate(self):
        for watch in self.watches():
            api = GitHub([issue(watch)])
            for result in ("skipped", "cancelled", "failure"):
                reporter.report(api, watch, result, NOW)
            self.assertEqual(api.writes, [])

    def test_ambiguous_comment_order_and_edited_markers_stop_updates(self):
        for watch in self.watches():
            for run in (OLD, reporter.Run(99, 10, NOW.sha)):
                row = issue(watch)
                reply(row, watch, run=run)
                self.denied(GitHub([row]), watch)

    def test_scheduled_main_workflow_context_is_bound_before_access(self):
        for watch in self.watches():
            self.assertEqual(reporter.context(environment(watch), watch), NOW)
            for key in environment(watch):
                with self.assertRaises(reporter.EvidenceError):
                    reporter.context({**environment(watch), key: "unexpected"}, watch)

    def test_read_failure_and_uncertain_write_are_not_retried(self):
        for watch in self.watches():
            api = GitHub([issue(watch)])
            def fail(api):
                raise reporter.EvidenceError("unknown result")
            api.before_write = fail
            with self.assertRaises(reporter.EvidenceError):
                reporter.report(api, watch, "success", NOW)
            self.assertEqual(sum(q.startswith("mutation") for q, _ in api.calls), 1)
            self.assertEqual(api.writes, [])
        for response in (subprocess.CompletedProcess([], 1, "", "private provider value"),
                         subprocess.CompletedProcess([], 0, "not json", ""),
                         subprocess.CompletedProcess([], 0, '{"data":{},"errors":[{"message":"private"}]}', "")):
            with patch.object(reporter.subprocess, "run", return_value=response) as run:
                with self.assertRaises(reporter.EvidenceError) as caught:
                    reporter.api("query", {})
                self.assertNotIn("private", str(caught.exception))
                self.assertEqual(run.call_count, 1)

    def test_api_command_is_one_graphql_request_with_json_on_stdin(self):
        response = subprocess.CompletedProcess([], 0, '{"data":{"node":null}}', "")
        with patch.object(reporter.subprocess, "run", return_value=response) as run:
            self.assertEqual(reporter.api("query($id:ID!){node(id:$id){id}}", {"id": "I_10"}), {"node": None})
        args, kwargs = run.call_args
        self.assertEqual(args[0], ["gh", "api", "graphql", "--input", "-"])
        self.assertEqual(json.loads(kwargs["input"])["variables"], {"id": "I_10"})
        self.assertEqual(kwargs["timeout"], 30)

    def test_both_exact_workflow_entry_commands_execute_the_reporter_offline(self):
        for watch in self.watches():
            source = (ROOT / ".github/workflows" / reporter.WATCHES[watch][0]).read_text()
            job = source.split("  report-scheduled-result:\n", 1)[1]
            command = re.search(r"^        run: (.+)$", job, re.M).group(1)
            self.assertEqual(shlex.split(command)[:3], ["python3", ".github/scripts/report-ci-watch.py", watch])
            self.assertIn("contents: read\n      issues: write", job)
            self.assertIn("ref: ${{ github.sha }}", job)
            self.assertIn("persist-credentials: false", job)
            self.assertNotIn("secrets.", job)
            self.assertNotRegex(job, r"issue (close|comment|create)|closeIssue|updateIssue")
            with tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                fake = root / "gh"
                fake.write_text(f'#!/bin/sh\nexec {shlex.quote(sys.executable)} {shlex.quote(str(Path(__file__).resolve()))} --fake-gh "$@"\n')
                fake.chmod(0o700)
                state = root / "state.json"
                state.write_text(json.dumps({"rows": [], "writes": []}))
                env = {**os.environ, **environment(watch), "PATH": f"{root}:{os.environ['PATH']}",
                       "CI_WATCH_FIXTURE": str(state), "RESULT": "failure"}
                result = subprocess.run(["bash", "-eu", "-c", command], cwd=ROOT, env=env, text=True, capture_output=True, timeout=15)
                self.assertEqual(result.returncode, 0, result.stderr)
                data = json.loads(state.read_text())
                self.assertEqual(len(data["writes"]), 1)
                self.assertEqual(len(data["rows"]), 1)
                env["RESULT"] = "success"
                result = subprocess.run(["bash", "-eu", "-c", command], cwd=ROOT, env=env, text=True, capture_output=True, timeout=15)
                self.assertEqual(result.returncode, 0, result.stderr)
                # Identical run cannot claim a new result; a later scheduled
                # run can record recovery without ever closing the issue.
                self.assertEqual(len(json.loads(state.read_text())["writes"]), 1)
                env["GITHUB_RUN_ID"] = "300"
                result = subprocess.run(["bash", "-eu", "-c", command], cwd=ROOT, env=env, text=True, capture_output=True, timeout=15)
                self.assertEqual(result.returncode, 0, result.stderr)
                data = json.loads(state.read_text())
                self.assertEqual(len(data["writes"]), 2)
                self.assertEqual(data["rows"][0]["state"], "OPEN")
                self.assertEqual(len(data["rows"][0]["comments"]["nodes"]), 1)
                # A subsequent failure must also recognise the literal bot
                # comment from recovery, not just a newly created issue.
                for run_id, outcome in (("400", "failure"), ("500", "success")):
                    env.update(GITHUB_RUN_ID=run_id, RESULT=outcome)
                    result = subprocess.run(["bash", "-eu", "-c", command], cwd=ROOT, env=env,
                                            text=True, capture_output=True, timeout=15)
                    self.assertEqual(result.returncode, 0, result.stderr)
                data = json.loads(state.read_text())
                self.assertEqual(len(data["writes"]), 4)
                self.assertEqual(data["rows"][0]["author"], GRAPHQL_ACTIONS_BOT)
                self.assertEqual(data["rows"][0]["state"], "OPEN")
                self.assertEqual(len(data["rows"][0]["comments"]["nodes"]), 3)
                self.assertTrue(all(c["author"] == GRAPHQL_ACTIONS_BOT
                                    for c in data["rows"][0]["comments"]["nodes"]))
                self.assertTrue(all("createIssue(" in q or "addComment(" in q for q, _ in data["writes"]))


def fake_gh():
    assert sys.argv[2:] == ["api", "graphql", "--input", "-"]
    path = Path(os.environ["CI_WATCH_FIXTURE"])
    state = json.loads(path.read_text())
    api = GitHub(state["rows"])
    request = json.load(sys.stdin)
    data = api(request["query"], request["variables"])
    path.write_text(json.dumps({"rows": api.rows, "writes": state["writes"] + api.writes}))
    print(json.dumps({"data": data}))


if __name__ == "__main__":
    if sys.argv[1:2] == ["--fake-gh"]:
        fake_gh()
    else:
        unittest.main(verbosity=2)
