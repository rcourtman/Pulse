#!/usr/bin/env python3
"""Report only the two scheduled watches' own issues, never resolve a report.

An issue title is not ownership. An unchanged GitHub Actions bot, exact body
and complete conversation identify a watch report; any edit, label, assignment
or other participant leaves it to the maintainers. Legacy reports are not
adopted. There is no atomic GitHub "close if no new human reply" operation, so
even an untouched report gets a recovery observation, NOT automatic closure.
Revalidation narrows the comment race; an arriving reply can never be hidden
by a close, body edit or state mutation. No check, signing or release code runs.
"""

import argparse
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass


REPOSITORY = "rcourtman/Pulse"
# GraphQL's Bot login, not the REST `github-actions[bot]` presentation.
BOT = {"__typename": "Bot", "login": "github-actions", "databaseId": 41898282}
ACTOR = "author { __typename login ... on Bot { databaseId } }"
WATCHES = {
    "memory": (
        "memory-regression.yml", "CI watch: Pulse memory comparison failed",
        "The daily memory comparison failed.",
        "The run summary compares main with the latest stable release on the same "
        "small mock estate, split into the Go heap, memory outside Go and file "
        "pages. The memory-regression artifact holds both heap profiles and "
        "heap-growth.txt. Check internal/mock first: mock fixtures are not "
        "production memory. No samples means a build or startup failed, not "
        "evidence of a memory change.",
    ),
    "signing": (
        "release-signing-preflight.yml", "CI watch: Release signing preflight failed",
        "The weekly release signing preflight failed.",
        "The run's annotations identify the failing checks: Developer ID identity, "
        "certificate expiry, signing, or Apple notary access. A failed credential "
        "check needs correction before release signing can succeed.",
    ),
}


class EvidenceError(Exception):
    """Missing, ambiguous or changed evidence must not authorise a mutation."""


@dataclass(frozen=True, order=True)
class Run:
    number: int
    attempt: int
    sha: str

    @property
    def url(self):
        return f"https://github.com/{REPOSITORY}/actions/runs/{self.number}/attempts/{self.attempt}"


def marker(watch):
    return f"<!-- pulse-ci-watch:v1:{WATCHES[watch][0]} -->"


def observation(result, run):
    return (f"Scheduled check: **{result}**.\n\nRun: {run.url}\n"
            f"Source: `{run.sha}`\n\n"
            "This is a check observation, not resolution of reported symptoms. "
            "This issue stays open for maintainer disposition.")


def body(watch, result, run, *, opening=False):
    text = f"{marker(watch)}\n\n"
    if opening:
        text += f"{WATCHES[watch][2]}\n\n{WATCHES[watch][3]}\n\n"
    return text + observation(result, run)


def parse_body(text, watch, *, opening=False):
    if not isinstance(text, str):
        raise EvidenceError("missing report body")
    matches = re.findall(
        rf"Run: https://github\.com/{REPOSITORY}/actions/runs/([1-9][0-9]*)/attempts/([1-9][0-9]*)\n"
        r"Source: `([0-9a-f]{40})`", text)
    for number, attempt, sha in matches:
        run = Run(int(number), int(attempt), sha)
        for result in ("failure", "success"):
            if text == body(watch, result, run, opening=opening):
                if opening and result != "failure":
                    break
                return result, run
    raise EvidenceError("report body changed or not owned")


def api(query, variables):
    """One request, no fallback/retry and no provider text in reporter errors."""
    try:
        result = subprocess.run(
            ["gh", "api", "graphql", "--input", "-"],
            input=json.dumps({"query": query, "variables": variables}),
            text=True, capture_output=True, timeout=30, check=False,
        )
        if result.returncode or len(result.stdout.encode()) > 8 * 1024 * 1024:
            raise EvidenceError("GitHub response unavailable; no request replayed")
        response = json.loads(result.stdout)
        if not isinstance(response, dict) or response.get("errors") or not isinstance(response.get("data"), dict):
            raise EvidenceError("incomplete GitHub response")
        return response["data"]
    except (OSError, subprocess.TimeoutExpired, ValueError) as error:
        raise EvidenceError("GitHub response unavailable; mutation outcome may be unknown") from error


def complete(connection):
    if not isinstance(connection, dict):
        raise EvidenceError("missing conversation")
    nodes = connection.get("nodes")
    info = connection.get("pageInfo", {})
    if (not isinstance(nodes, list) or type(connection.get("totalCount")) is not int
            or connection["totalCount"] != len(nodes) or info.get("hasNextPage") is not False
            or any(not isinstance(node, dict) for node in nodes)):
        raise EvidenceError("incomplete conversation")
    return nodes


def find_report(call, watch):
    # Enumerate the bounded open inventory without a createdBy alias filter.
    # REST and GraphQL spell the Actions login differently; an empty filtered
    # response cannot prove that no report exists. Exact node ownership is
    # still required by untouched() before any comment, even for copied markers.
    query = f"""query($after:String) {{ repository(owner:"rcourtman", name:"Pulse") {{
      issues(first:100, after:$after, states:OPEN,
        orderBy:{{field:CREATED_AT, direction:ASC}}) {{
        totalCount pageInfo {{ hasNextPage endCursor }}
        nodes {{ id number title body {ACTOR} }}
      }}
    }} }}"""
    after, count, nodes, cursors = None, None, {}, set()
    for _ in range(10):
        page = call(query, {"after": after})["repository"]["issues"]
        total = page["totalCount"]
        if type(total) is not int or not 0 <= total <= 1000 or (count is not None and total != count):
            raise EvidenceError("issue inventory changed or exceeded its bound")
        count = total
        if not isinstance(page["nodes"], list) or len(page["nodes"]) > 100:
            raise EvidenceError("incomplete issue inventory")
        for node in page["nodes"]:
            if not isinstance(node, dict) or not isinstance(node.get("id"), str) or node["id"] in nodes:
                raise EvidenceError("ambiguous issue inventory")
            nodes[node["id"]] = node
        info = page["pageInfo"]
        if info.get("hasNextPage") is False:
            if len(nodes) != count:
                raise EvidenceError("incomplete issue inventory")
            # Do not select by title, or silently adopt an old unmarked report.
            owned = [n for n in nodes.values() if isinstance(n.get("body"), str) and marker(watch) in n["body"]]
            if len(owned) > 1:
                raise EvidenceError("multiple marked reports; maintainer disposition needed")
            return owned[0] if owned else None
        after = info.get("endCursor")
        if info.get("hasNextPage") is not True or not page["nodes"] or not isinstance(after, str) or not after or after in cursors:
            raise EvidenceError("incomplete issue inventory")
        cursors.add(after)
    raise EvidenceError("issue inventory exceeded its bound")


def snapshot(call, issue_id):
    query = f"""query($id:ID!) {{ node(id:$id) {{ ... on Issue {{
      id number title body state updatedAt {ACTOR} editor {{ login }} lastEditedAt
      locked milestone {{ id }} labels(first:1) {{ totalCount }} assignees(first:1) {{ totalCount }}
      comments(first:100) {{ totalCount pageInfo {{ hasNextPage }}
        nodes {{ id body {ACTOR} editor {{ login }} lastEditedAt }} }}
      timelineItems(first:100) {{ totalCount pageInfo {{ hasNextPage }}
        nodes {{ __typename ... on IssueComment {{ id }} }} }}
    }} }} }}"""
    return call(query, {"id": issue_id})["node"]


def untouched(issue, watch):
    if (issue.get("author") != BOT or issue.get("title") != WATCHES[watch][1]
            or issue.get("state") != "OPEN" or issue.get("locked") is not False
            or type(issue.get("number")) is not int or issue["number"] <= 0
            or not isinstance(issue.get("updatedAt"), str) or not issue["updatedAt"]
            or "editor" not in issue or issue["editor"] is not None
            or "lastEditedAt" not in issue or issue["lastEditedAt"] is not None
            or "milestone" not in issue or issue["milestone"] is not None
            or type(issue["labels"].get("totalCount")) is not int or issue["labels"]["totalCount"] != 0
            or type(issue["assignees"].get("totalCount")) is not int or issue["assignees"]["totalCount"] != 0):
        raise EvidenceError("report was taken over or is not owned")
    previous_result, previous_run = parse_body(issue["body"], watch, opening=True)
    comments = complete(issue["comments"])
    timeline = complete(issue["timelineItems"])
    ids = [c["id"] for c in comments]
    if (len(set(ids)) != len(ids)
            or any(t.get("__typename") != "IssueComment" for t in timeline)
            or [t.get("id") for t in timeline] != ids):
        raise EvidenceError("report has activity outside the watch conversation")
    for comment in comments:
        if (comment.get("author") != BOT or "editor" not in comment or comment["editor"] is not None
                or "lastEditedAt" not in comment or comment["lastEditedAt"] is not None):
            raise EvidenceError("conversation was taken over")
        result, run = parse_body(comment["body"], watch)
        if (run.number, run.attempt) <= (previous_run.number, previous_run.attempt):
            raise EvidenceError("ambiguous report ordering")
        previous_result, previous_run = result, run
    return previous_result, previous_run


def report(call, watch, result, run):
    if result not in ("failure", "success"):
        return "No observation: check did not finish with success or failure."
    issue = find_report(call, watch)
    if issue is None:
        if result == "success":
            return "No failing watch report to update; legacy/title-only issues left alone."
        # The workflow's existing concurrency group serialises its reporters.
        # Re-read the inventory too: a racing creation is not permission to add
        # another issue. Unknown POST outcomes are never blindly replayed.
        if find_report(call, watch) is not None:
            raise EvidenceError("report appeared during preparation")
        created = call("""mutation($title:String!, $body:String!, $repositoryId:ID!) {
          createIssue(input:{repositoryId:$repositoryId,title:$title,body:$body}) { issue { id } }
        }""", {"repositoryId": "R_kgDON5nJtA", "title": WATCHES[watch][1], "body": body(watch, result, run, opening=True)})
        if not isinstance(created["createIssue"]["issue"]["id"], str) or not created["createIssue"]["issue"]["id"]:
            raise EvidenceError("issue creation outcome unavailable; no retry")
        return "Created a separate, marked watch report; no existing issue changed."
    original = snapshot(call, issue["id"])
    if not isinstance(original, dict) or original.get("id") != issue["id"]:
        raise EvidenceError("issue identity unavailable")
    previous_result, previous_run = untouched(original, watch)
    if (run.number, run.attempt) <= (previous_run.number, previous_run.attempt) or result == previous_result:
        return "No new transition; report left unchanged."
    current = snapshot(call, issue["id"])
    untouched(current, watch)
    if original != current:
        raise EvidenceError("report changed during preparation")
    # Deliberately no close/update/delete API. A human can still arrive after
    # this read. The only possible raced mutation is a factual observation;
    # it neither removes that reply nor claims its symptoms are resolved.
    posted = call("""mutation($id:ID!, $body:String!) {
      addComment(input:{subjectId:$id,body:$body}) { commentEdge { node { id } } }
    }""", {"id": issue["id"], "body": body(watch, result, run)})
    if not isinstance(posted["addComment"]["commentEdge"]["node"]["id"], str) or not posted["addComment"]["commentEdge"]["node"]["id"]:
        raise EvidenceError("comment outcome unavailable; no retry")
    return "Recorded check transition; issue remains open for maintainer disposition."


def context(environment, watch):
    workflow = f"{REPOSITORY}/.github/workflows/{WATCHES[watch][0]}@refs/heads/main"
    if any(environment.get(key) != value for key, value in {
        "GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "schedule", "GITHUB_REPOSITORY": REPOSITORY,
        "GITHUB_SERVER_URL": "https://github.com", "GITHUB_REF": "refs/heads/main",
        "GITHUB_WORKFLOW_REF": workflow,
    }.items()):
        raise EvidenceError("not the scheduled main watch")
    number, attempt, sha = (environment.get(key, "") for key in ("GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT", "GITHUB_SHA"))
    if not re.fullmatch(r"[1-9][0-9]*", number) or not re.fullmatch(r"[1-9][0-9]*", attempt) or not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise EvidenceError("run identity unavailable")
    return Run(int(number), int(attempt), sha)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("watch", choices=WATCHES)
    parser.add_argument("result", choices=("success", "failure", "cancelled", "skipped"))
    args = parser.parse_args()
    try:
        message = report(api, args.watch, args.result, context(os.environ, args.watch))
    except (EvidenceError, KeyError, TypeError, AttributeError):
        # Never echo an issue body, customer reply, token or raw API error.
        print("CI watch reporter stopped: ownership/conversation or request outcome unavailable. No retry or closure.", file=sys.stderr)
        return 1
    print(message)
    return 0


if __name__ == "__main__":
    sys.exit(main())
