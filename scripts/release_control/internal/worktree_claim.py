#!/usr/bin/env python3
"""Reserve a governed slice and create an isolated git worktree for it."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess
import sys
from typing import Any

from repo_file_io import REPO_ROOT
from work_claim import reserve_claim, write_status_payload, _slug


WORKSPACE_ROOT = REPO_ROOT.parents[1]
WORKTREES_ROOT = WORKSPACE_ROOT / "worktrees"
DEFAULT_BASE_BRANCH = "pulse/v6"


def build_branch_name(*, agent_id: str, work_kind: str, work_id: str) -> str:
    return f"pulse/{_slug(agent_id)}/{_slug(work_kind)}-{_slug(work_id)}"


def branch_path_slug(branch_name: str) -> str:
    if branch_name.startswith("pulse/"):
        branch_name = branch_name[len("pulse/") :]
    return branch_name.replace("/", "__")


def build_worktree_path(*, repo_root: Path, branch_name: str) -> Path:
    return WORKTREES_ROOT / repo_root.name / branch_path_slug(branch_name)


def parse_worktree_list(output: str) -> list[dict[str, str]]:
    entries: list[dict[str, str]] = []
    current: dict[str, str] = {}
    for line in output.splitlines():
        if not line.strip():
            if current:
                entries.append(current)
                current = {}
            continue
        key, _, value = line.partition(" ")
        current[key] = value
    if current:
        entries.append(current)
    return entries


def list_worktrees(*, repo_root: Path = REPO_ROOT) -> list[dict[str, str]]:
    result = subprocess.run(
        ["git", "worktree", "list", "--porcelain"],
        cwd=repo_root,
        check=True,
        capture_output=True,
        text=True,
    )
    return parse_worktree_list(result.stdout)


def validate_worktree_target(*, repo_root: Path, branch_name: str, path: Path) -> list[str]:
    errors: list[str] = []
    path = path.resolve()
    for entry in list_worktrees(repo_root=repo_root):
        existing_path = Path(entry.get("worktree", "")).resolve()
        branch_ref = entry.get("branch", "")
        if existing_path == path:
            errors.append(f"worktree path already exists in git worktree list: {path}")
        if branch_ref == f"refs/heads/{branch_name}":
            errors.append(f"branch already checked out in another worktree: {branch_name}")
    if path.exists():
        errors.append(f"worktree path already exists on disk: {path}")
    return errors


def create_worktree(*, repo_root: Path, path: Path, branch_name: str, base_branch: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(
        ["git", "worktree", "add", "-b", branch_name, str(path), base_branch],
        cwd=repo_root,
        check=True,
    )


def render_pretty(*, claim: dict[str, Any], branch_name: str, path: Path, errors: list[str], wrote: bool) -> str:
    lines = [
        "worktree_claim:",
        f"  claim_id={claim['id']}",
        f"  target={claim['target_id']}",
        f"  work={claim['work_item']['kind']}:{claim['work_item']['id']}",
        f"  agent={claim['agent_id']}",
        f"  branch={branch_name}",
        f"  path={path}",
        f"  wrote={'yes' if wrote else 'no'}",
    ]
    if errors:
        lines.append("errors:")
        for error in errors:
            lines.append(f"  - {error}")
    else:
        lines.append("status: ready")
    return "\n".join(lines)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Reserve a governed slice and create an isolated git worktree for it."
    )
    parser.add_argument("--kind", required=True, help="Work item kind, such as lane or release-gate.")
    parser.add_argument("--id", required=True, help="Work item id, such as L15.")
    parser.add_argument("--summary", required=True, help="Short human summary for the claim.")
    parser.add_argument("--agent-id", required=True, help="Stable agent identifier recorded in the claim.")
    parser.add_argument("--target-id", help="Override the target id; defaults to the active target.")
    parser.add_argument("--claim-id", help="Override the generated claim id.")
    parser.add_argument("--branch", help="Override the generated branch name.")
    parser.add_argument("--path", help="Override the generated worktree path.")
    parser.add_argument("--base-branch", default=DEFAULT_BASE_BRANCH, help="Base branch for the new worktree.")
    parser.add_argument(
        "--duration-hours",
        type=int,
        default=2,
        help="Claim duration before expiry. Defaults to 2 hours.",
    )
    parser.add_argument(
        "--replace-claim-id",
        action="append",
        default=[],
        help="Existing claim id to replace in the same write.",
    )
    parser.add_argument(
        "--write",
        action="store_true",
        help="Write the updated work_claims list and create the worktree when validation passes.",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Render a concise human summary instead of JSON.",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    from status_audit import load_status_payload

    payload = load_status_payload()
    claim, updated_payload, errors = reserve_claim(
        payload=payload,
        work_kind=args.kind,
        work_id=args.id,
        summary=args.summary,
        agent_id=args.agent_id,
        target_id=args.target_id,
        duration_hours=args.duration_hours,
        claim_id=args.claim_id,
        replace_claim_ids=list(args.replace_claim_id),
    )
    branch_name = args.branch or build_branch_name(
        agent_id=args.agent_id,
        work_kind=args.kind,
        work_id=args.id,
    )
    path = Path(args.path) if args.path else build_worktree_path(repo_root=REPO_ROOT, branch_name=branch_name)
    errors.extend(validate_worktree_target(repo_root=REPO_ROOT, branch_name=branch_name, path=path))

    wrote = False
    if not errors and args.write:
        write_status_payload(updated_payload)
        create_worktree(repo_root=REPO_ROOT, path=path, branch_name=branch_name, base_branch=args.base_branch)
        wrote = True

    if args.pretty:
        print(render_pretty(claim=claim, branch_name=branch_name, path=path, errors=errors, wrote=wrote))
    else:
        print(
            json.dumps(
                {
                    "claim": claim,
                    "branch": branch_name,
                    "path": str(path),
                    "errors": errors,
                    "wrote": wrote,
                },
                indent=2,
                sort_keys=True,
            )
        )
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
