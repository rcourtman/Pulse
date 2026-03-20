#!/usr/bin/env python3
"""Mechanical preflight for v6 agent sessions.

The goal is to make the canonical checks executable so future sessions do not
depend on chat memory:
- confirm the active branch
- confirm the active control plane target
- confirm a persisted work claim when mutating a governed slice
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from datetime import datetime, timezone
from typing import Any

from control_plane import active_control_plane
from repo_file_io import REPO_ROOT
from status_audit import load_status_payload
from work_claim import split_claims_by_state

DEFAULT_AGENT_ID = "codex"


def current_branch() -> str:
    result = subprocess.run(
        ["git", "-C", str(REPO_ROOT), "branch", "--show-current"],
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def audit_preflight(
    *,
    agent_id: str,
    require_active_claim: bool,
    staged: bool = False,
) -> dict[str, Any]:
    control_plane = active_control_plane(staged=staged)
    status_payload = load_status_payload(staged=staged)
    now = datetime.now(timezone.utc)
    active_claims, expired_claims = split_claims_by_state(
        status_payload.get("work_claims", []),
        now_utc=now,
    )
    agent_active_claims = [
        claim for claim in active_claims if str(claim.get("agent_id")) == agent_id
    ]

    errors: list[str] = []
    branch = current_branch()
    expected_branch = str(control_plane["prerelease_branch"])
    if branch != expected_branch:
        errors.append(
            f"current branch {branch!r} does not match active prerelease branch {expected_branch!r}"
        )
    if require_active_claim and len(agent_active_claims) != 1:
        errors.append(
            f"expected exactly one active work claim for agent {agent_id!r}, "
            f"found {len(agent_active_claims)}"
        )

    return {
        "branch": branch,
        "expected_branch": expected_branch,
        "active_profile_id": control_plane["active_profile_id"],
        "active_target_id": control_plane["active_target_id"],
        "agent_id": agent_id,
        "require_active_claim": require_active_claim,
        "active_claim_count": len(active_claims),
        "agent_active_claim_count": len(agent_active_claims),
        "expired_claim_count": len(expired_claims),
        "active_claim_ids": [str(claim.get("id")) for claim in active_claims],
        "agent_active_claim_ids": [str(claim.get("id")) for claim in agent_active_claims],
        "errors": errors,
    }


def render_pretty(report: dict[str, Any]) -> str:
    lines = [
        "agent_preflight:",
        f"  branch={report['branch']}",
        f"  expected_branch={report['expected_branch']}",
        f"  active_profile_id={report['active_profile_id']}",
        f"  active_target_id={report['active_target_id']}",
        f"  active_claim_count={report['active_claim_count']}",
        f"  expired_claim_count={report['expired_claim_count']}",
        f"  agent_id={report['agent_id']}",
        f"  agent_active_claim_count={report['agent_active_claim_count']}",
        f"  require_active_claim={report['require_active_claim']}",
    ]
    if report["active_claim_ids"]:
        lines.append(f"  active_claim_ids={', '.join(report['active_claim_ids'])}")
    if report["agent_active_claim_ids"]:
        lines.append(f"  agent_active_claim_ids={', '.join(report['agent_active_claim_ids'])}")
    if report["errors"]:
        lines.append("  errors:")
        lines.extend(f"    - {error}" for error in report["errors"])
    else:
        lines.append("  status=passed")
    return "\n".join(lines)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run the canonical v6 agent preflight.")
    parser.add_argument(
        "--agent-id",
        default=DEFAULT_AGENT_ID,
        help="Agent id to verify against active work claims (default: codex).",
    )
    parser.add_argument(
        "--require-active-claim",
        action="store_true",
        help="Fail unless the agent has exactly one active claim.",
    )
    parser.add_argument("--staged", action="store_true", help="Read staged repo JSON if available.")
    parser.add_argument("--pretty", action="store_true", help="Print a human-readable summary.")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    report = audit_preflight(
        agent_id=args.agent_id,
        require_active_claim=args.require_active_claim,
        staged=args.staged,
    )
    output = render_pretty(report) if args.pretty else json.dumps(report, indent=2, sort_keys=True)
    print(output)
    return 0 if not report["errors"] else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
