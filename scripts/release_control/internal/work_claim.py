#!/usr/bin/env python3
"""Create or reserve governed work claims against the active Pulse profile."""

from __future__ import annotations

import argparse
from copy import deepcopy
from datetime import datetime, timedelta, timezone
import json
import re
import sys
from pathlib import Path
from typing import Any

from control_plane import DEFAULT_CONTROL_PLANE
from status_audit import audit_status_payload, load_status_payload, status_schema_contract


STATUS_PATH = Path(DEFAULT_CONTROL_PLANE["status_path"])


def _slug(value: str) -> str:
    compact = re.sub(r"[^a-z0-9]+", "-", value.strip().lower())
    compact = compact.strip("-")
    return compact or "work"


def _claim_sort_key(claim: dict[str, Any]) -> tuple[str, str]:
    return (str(claim["claimed_at"]), str(claim["id"]).casefold())


def build_work_claim(
    *,
    work_kind: str,
    work_id: str,
    summary: str,
    agent_id: str,
    target_id: str,
    now_utc: datetime,
    duration_hours: int,
    claim_id: str | None = None,
) -> dict[str, Any]:
    claimed_at = now_utc.replace(microsecond=0)
    expires_at = claimed_at + timedelta(hours=duration_hours)
    claim_id = claim_id or f"claim-{_slug(work_kind)}-{_slug(work_id)}-{claimed_at.strftime('%Y%m%d%H%M%S')}"
    timestamp = claimed_at.isoformat().replace("+00:00", "Z")
    return {
        "id": claim_id,
        "agent_id": agent_id,
        "summary": summary,
        "target_id": target_id,
        "claimed_at": timestamp,
        "heartbeat_at": timestamp,
        "expires_at": expires_at.replace(microsecond=0).isoformat().replace("+00:00", "Z"),
        "work_item": {
            "kind": work_kind,
            "id": work_id,
        },
    }


def apply_claim(
    payload: dict[str, Any],
    claim: dict[str, Any],
    *,
    replace_claim_ids: list[str],
) -> dict[str, Any]:
    updated = deepcopy(payload)
    raw_claims = list(updated.get("work_claims", []))
    raw_claims = [entry for entry in raw_claims if str(entry.get("id", "")) not in set(replace_claim_ids)]
    raw_claims.append(claim)
    raw_claims.sort(key=_claim_sort_key)
    updated["work_claims"] = raw_claims
    return updated


def reserve_claim(
    *,
    payload: dict[str, Any],
    work_kind: str,
    work_id: str,
    summary: str,
    agent_id: str,
    target_id: str | None,
    duration_hours: int,
    claim_id: str | None,
    replace_claim_ids: list[str],
    now_utc: datetime | None = None,
) -> tuple[dict[str, Any], dict[str, Any], list[str]]:
    baseline_report = audit_status_payload(payload, schema_contract=status_schema_contract())
    baseline_errors = set(str(error) for error in baseline_report.get("errors", []))
    now_utc = now_utc or datetime.now(timezone.utc)
    target_id = target_id or str(DEFAULT_CONTROL_PLANE["active_target_id"])
    claim = build_work_claim(
        work_kind=work_kind,
        work_id=work_id,
        summary=summary,
        agent_id=agent_id,
        target_id=target_id,
        now_utc=now_utc,
        duration_hours=duration_hours,
        claim_id=claim_id,
    )
    updated_payload = apply_claim(payload, claim, replace_claim_ids=replace_claim_ids)
    updated_report = audit_status_payload(updated_payload, schema_contract=status_schema_contract())
    new_errors = [str(error) for error in updated_report.get("errors", []) if str(error) not in baseline_errors]
    return claim, updated_payload, new_errors


def write_status_payload(payload: dict[str, Any], *, path: Path = STATUS_PATH) -> None:
    path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create or reserve a governed work claim.")
    parser.add_argument("--kind", required=True, help="Work item kind, such as lane or release-gate.")
    parser.add_argument("--id", required=True, help="Work item id, such as L14.")
    parser.add_argument("--summary", required=True, help="Short human summary for the claim.")
    parser.add_argument("--agent-id", required=True, help="Stable agent identifier recorded in the claim.")
    parser.add_argument("--target-id", help="Override the target id; defaults to the active target.")
    parser.add_argument("--claim-id", help="Override the generated claim id.")
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
        help="Write the updated work_claims list back to status.json when validation passes.",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Render a concise human summary instead of JSON.",
    )
    return parser.parse_args(argv)


def render_pretty(claim: dict[str, Any], errors: list[str], *, wrote: bool) -> str:
    lines = [
        "work_claim:",
        f"  id={claim['id']}",
        f"  target={claim['target_id']}",
        f"  work={claim['work_item']['kind']}:{claim['work_item']['id']}",
        f"  agent={claim['agent_id']}",
        f"  expires_at={claim['expires_at']}",
        f"  wrote={'yes' if wrote else 'no'}",
    ]
    if errors:
        lines.append("errors:")
        for error in errors:
            lines.append(f"  - {error}")
    else:
        lines.append("status: ready")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
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
    wrote = False
    if not errors and args.write:
        write_status_payload(updated_payload)
        wrote = True

    if args.pretty:
        print(render_pretty(claim, errors, wrote=wrote))
    else:
        print(
            json.dumps(
                {
                    "claim": claim,
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
