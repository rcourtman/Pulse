#!/usr/bin/env python3
"""Find the acknowledged child of this release run, not a similar build."""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys


JOB_NAME = "stage_private_pro_runtime"
DISPATCH_STEP = "Dispatch and verify private Pro runtime staging"
RUN_ID = r"[1-9][0-9]{0,18}"
# GitHub's raw job log prefixes stdout lines with a UTC timestamp. Match
# only the fixed acknowledgement emitted by the waiter, not script echoes,
# links, status messages or child logs. The parent job lives in a gated,
# immutable source workflow; only its own completed job log is read here.
ACK = re.compile(
    rf"^\d{{4}}-\d{{2}}-\d{{2}}T\d{{2}}:\d{{2}}:\d{{2}}(?:\.\d+)?Z "
    rf"Watching exact private Pro build run ({RUN_ID}) in rcourtman/pulse-enterprise\.\r?$",
    re.MULTILINE,
)


class EvidenceError(ValueError):
    pass


def read_api(endpoint: str) -> str:
    try:
        response = subprocess.run(["gh", "api", endpoint], capture_output=True, timeout=30, check=True)
    except (OSError, subprocess.SubprocessError):
        raise EvidenceError("could not read the exact prior staging job evidence") from None
    if len(response.stdout) > 2 * 1024 * 1024:
        raise EvidenceError("prior staging job evidence exceeded the read limit")
    return response.stdout.decode("utf-8")


def previous_child(repository: str, run_id: str, attempt: str, source: str) -> str:
    if not (
        re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository)
        and re.fullmatch(RUN_ID, run_id)
        and re.fullmatch(r"[1-9][0-9]?", attempt)
        and re.fullmatch(r"[0-9a-f]{40}", source)
    ):
        raise EvidenceError("invalid release run identity")

    root = f"repos/{repository}/actions"
    for earlier in range(int(attempt) - 1, 0, -1):
        metadata = json.loads(read_api(f"{root}/runs/{run_id}/attempts/{earlier}/jobs?per_page=100"))
        jobs = metadata["jobs"]
        if not isinstance(jobs, list) or metadata["total_count"] != len(jobs):
            raise EvidenceError(f"incomplete attempt {earlier} job evidence")
        stage_jobs = [job for job in jobs if job["name"] == JOB_NAME]
        if len(stage_jobs) != 1:
            raise EvidenceError(f"attempt {earlier} does not identify one staging job")
        job = stage_jobs[0]
        job_id = str(job["id"])
        if not (
            re.fullmatch(RUN_ID, job_id)
            and job["run_id"] == int(run_id)
            and job["run_attempt"] == earlier
            and job["head_sha"] == source
            and job["status"] == "completed"
        ):
            raise EvidenceError(f"attempt {earlier} staging job identity or result did not match")
        if job["conclusion"] == "skipped":
            continue
        dispatch = [step for step in job["steps"] if step["name"] == DISPATCH_STEP]
        if len(dispatch) != 1 or dispatch[0]["status"] != "completed" or not dispatch[0]["conclusion"]:
            raise EvidenceError(f"attempt {earlier} has no definite staging step result")
        if dispatch[0]["conclusion"] == "skipped":
            continue

        children = ACK.findall(read_api(f"{root}/jobs/{job_id}/logs"))
        if len(children) != 1:
            raise EvidenceError(f"attempt {earlier} has no unique child acknowledgement; dispatch may have been accepted")
        return children[0]

    # Original execution or every earlier staging step definitely skipped.
    return ""


def main() -> int:
    try:
        child = previous_child(*(os.environ[name] for name in (
            "GITHUB_REPOSITORY", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT", "GITHUB_SHA",
        )))
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
            output.write(f"child_id={child}\n")
    except EvidenceError as exc:
        print(f"::error::Private staging retry stopped: {exc}.", file=sys.stderr)
        return 1
    except (KeyError, TypeError, ValueError, OSError):
        print("::error::Private staging retry stopped: invalid or unavailable evidence.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
