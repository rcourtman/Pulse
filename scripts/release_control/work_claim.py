#!/usr/bin/env python3
"""Reserve and release governed work claims in the active status.json."""

from __future__ import annotations

import argparse
from datetime import datetime, timedelta, timezone
import json
from pathlib import Path
import re
import sys
from typing import Any

from control_plane import DEFAULT_CONTROL_PLANE
from status_audit import DEFAULT_STATUS_SCHEMA_CONTRACT, load_status_payload


STATUS_PATH = Path(DEFAULT_CONTROL_PLANE["status_path"])
STATUS_REL = str(DEFAULT_CONTROL_PLANE["status_rel"])
ACTIVE_PROFILE_ID = str(DEFAULT_CONTROL_PLANE["active_profile_id"])
ACTIVE_TARGET_ID = str(DEFAULT_CONTROL_PLANE["active_target_id"])
TARGETS_BY_ID = dict(DEFAULT_CONTROL_PLANE["targets_by_id"])
VALID_WORK_CLAIM_KINDS = set(DEFAULT_STATUS_SCHEMA_CONTRACT["valid_work_claim_kinds"])
WORK_ITEM_COLLECTIONS = {
    "lane": "lanes",
    "lane-followup": "lane_followups",
    "coverage-gap": "coverage_gaps",
    "candidate-lane": "candidate_lanes",
    "readiness-assertion": "readiness_assertions",
    "release-gate": "release_gates",
    "open-decision": "open_decisions",
}
NARROW_SAME_LANE_KINDS = {"lane-followup", "readiness-assertion", "release-gate", "open-decision"}
TOKEN_RE = re.compile(r"[^a-z0-9]+")


def utc_now() -> datetime:
    return datetime.now(timezone.utc).replace(microsecond=0)


def parse_timestamp(raw: str) -> datetime:
    normalized = raw.strip()
    if normalized.endswith("Z"):
        normalized = normalized[:-1] + "+00:00"
    try:
        parsed = datetime.fromisoformat(normalized)
    except ValueError as exc:
        raise ValueError(f"invalid timestamp {raw!r}; use RFC3339 such as 2026-03-19T12:00:00Z") from exc
    if parsed.tzinfo is None:
        raise ValueError(f"invalid timestamp {raw!r}; timezone is required")
    return parsed.astimezone(timezone.utc).replace(microsecond=0)


def format_timestamp(value: datetime) -> str:
    return value.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def claim_sort_key(claim: dict[str, Any]) -> tuple[str, str]:
    return (str(claim.get("claimed_at", "")), str(claim.get("id", "")).casefold())


def sanitize_token(value: str) -> str:
    token = TOKEN_RE.sub("-", value.strip().casefold()).strip("-")
    return token or "claim"


def claim_id_for(*, agent_id: str, work_kind: str, work_id: str) -> str:
    return f"{sanitize_token(agent_id)}-{sanitize_token(work_kind)}-{sanitize_token(work_id)}"


def require_work_claims(payload: dict[str, Any]) -> list[dict[str, Any]]:
    claims = payload.get("work_claims")
    if not isinstance(claims, list):
        raise ValueError("status.json missing work_claims list")
    if not all(isinstance(claim, dict) for claim in claims):
        raise ValueError("status.json work_claims must contain only objects")
    return [dict(claim) for claim in claims]


def lookup_work_item(payload: dict[str, Any], *, work_kind: str, work_id: str) -> dict[str, Any]:
    collection = WORK_ITEM_COLLECTIONS.get(work_kind)
    if collection is None:
        raise ValueError(f"unsupported work claim kind {work_kind!r}")
    entries = payload.get(collection)
    if not isinstance(entries, list):
        raise ValueError(f"status.json missing {collection} list")
    for entry in entries:
        if isinstance(entry, dict) and str(entry.get("id")) == work_id:
            return dict(entry)
    raise ValueError(f"status.json does not contain {work_kind}:{work_id}")


def work_item_context(payload: dict[str, Any], *, work_kind: str, work_id: str) -> dict[str, Any]:
    item = lookup_work_item(payload, work_kind=work_kind, work_id=work_id)
    lane_ids: list[str] = []
    repo_ids: list[str] = []
    coverage_gap_ids: list[str] = []
    target_id: str | None = None

    if work_kind == "lane":
        lane_ids = [str(item["id"])]
        repo_ids = [str(repo_id) for repo_id in item.get("repo_ids", []) if isinstance(repo_id, str)]
    elif work_kind == "lane-followup":
        lane_ids = [str(lane_id) for lane_id in item.get("lane_ids", []) if isinstance(lane_id, str)]
        repo_ids = [str(repo_id) for repo_id in item.get("repo_ids", []) if isinstance(repo_id, str)]
    elif work_kind == "coverage-gap":
        lane_ids = [str(lane_id) for lane_id in item.get("lane_ids", []) if isinstance(lane_id, str)]
        repo_ids = [str(repo_id) for repo_id in item.get("repo_ids", []) if isinstance(repo_id, str)]
        coverage_gap_ids = [str(item["id"])]
    elif work_kind == "candidate-lane":
        lane_ids = [str(lane_id) for lane_id in item.get("current_lane_ids", []) if isinstance(lane_id, str)]
        repo_ids = [str(repo_id) for repo_id in item.get("repo_ids", []) if isinstance(repo_id, str)]
        coverage_gap_ids = [str(gap_id) for gap_id in item.get("coverage_gap_ids", []) if isinstance(gap_id, str)]
        target_id = str(item.get("target_id", "")).strip() or None
    elif work_kind in {"readiness-assertion", "release-gate", "open-decision"}:
        lane_ids = [str(lane_id) for lane_id in item.get("lane_ids", []) if isinstance(lane_id, str)]
        repo_ids = [str(repo_id) for repo_id in item.get("repo_ids", []) if isinstance(repo_id, str)]

    return {
        "item": item,
        "lane_ids": lane_ids,
        "repo_ids": repo_ids,
        "coverage_gap_ids": coverage_gap_ids,
        "target_id": target_id,
    }


def claim_state(claim: dict[str, Any], *, now_utc: datetime) -> str:
    expires_at = claim.get("expires_at")
    if not isinstance(expires_at, str):
        raise ValueError(f"work claim {claim.get('id')!r} is missing expires_at")
    return "active" if parse_timestamp(expires_at) > now_utc else "expired"


def split_claims_by_state(
    claims: list[dict[str, Any]],
    *,
    now_utc: datetime,
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    active: list[dict[str, Any]] = []
    expired: list[dict[str, Any]] = []
    for claim in claims:
        if claim_state(claim, now_utc=now_utc) == "active":
            active.append(dict(claim))
        else:
            expired.append(dict(claim))
    return active, expired


def resolve_target_id(
    *,
    work_kind: str,
    work_context: dict[str, Any],
    explicit_target_id: str | None,
) -> str:
    target_id = explicit_target_id or work_context.get("target_id") or ACTIVE_TARGET_ID
    target = TARGETS_BY_ID.get(target_id)
    if target is None:
        raise ValueError(f"unknown target_id {target_id!r}")
    if str(target.get("profile_id", "")).strip() != ACTIVE_PROFILE_ID:
        raise ValueError(f"target_id {target_id!r} does not belong to active profile {ACTIVE_PROFILE_ID!r}")
    if str(target.get("status", "")).strip() == "completed":
        raise ValueError(f"target_id {target_id!r} must not reference a completed target")
    candidate_target_id = work_context.get("target_id")
    if work_kind == "candidate-lane" and candidate_target_id and target_id != candidate_target_id:
        raise ValueError(
            f"candidate-lane target_id must stay {candidate_target_id!r}, got {target_id!r}"
        )
    return target_id


def claim_matches(
    claim: dict[str, Any],
    *,
    agent_id: str,
    target_id: str,
    work_kind: str,
    work_id: str,
) -> bool:
    work_item = claim.get("work_item")
    if not isinstance(work_item, dict):
        return False
    return (
        str(claim.get("agent_id")) == agent_id
        and str(claim.get("target_id")) == target_id
        and str(work_item.get("kind")) == work_kind
        and str(work_item.get("id")) == work_id
    )


def claim_overlap_messages(
    payload: dict[str, Any],
    *,
    new_work_kind: str,
    new_work_id: str,
    existing_claims: list[dict[str, Any]],
) -> list[str]:
    new_context = work_item_context(payload, work_kind=new_work_kind, work_id=new_work_id)
    new_lane_ids = set(new_context["lane_ids"])
    new_gap_ids = set(new_context["coverage_gap_ids"])
    messages: list[str] = []

    for claim in existing_claims:
        work_item = claim.get("work_item")
        if not isinstance(work_item, dict):
            continue
        existing_kind = str(work_item.get("kind", ""))
        existing_id = str(work_item.get("id", ""))
        if existing_kind not in VALID_WORK_CLAIM_KINDS or not existing_id:
            continue
        existing_context = work_item_context(payload, work_kind=existing_kind, work_id=existing_id)
        existing_lane_ids = set(existing_context["lane_ids"])
        existing_gap_ids = set(existing_context["coverage_gap_ids"])
        claim_id = str(claim.get("id", ""))

        if new_work_kind == existing_kind and new_work_id == existing_id:
            messages.append(
                f"existing active claim {claim_id!r} already reserves {existing_kind}:{existing_id}"
            )
            continue

        if new_work_kind == "lane" and existing_kind in NARROW_SAME_LANE_KINDS:
            for lane_id in sorted(new_lane_ids & existing_lane_ids):
                messages.append(
                    f"existing active claim {claim_id!r} already reserves narrower same-lane work on lane {lane_id}"
                )
        elif existing_kind == "lane" and new_work_kind in NARROW_SAME_LANE_KINDS:
            for lane_id in sorted(new_lane_ids & existing_lane_ids):
                messages.append(
                    f"existing active claim {claim_id!r} already holds the broad lane reservation for {lane_id}"
                )

        if new_work_kind == "candidate-lane" and existing_kind == "coverage-gap":
            for gap_id in sorted(new_gap_ids & existing_gap_ids):
                messages.append(
                    f"existing active claim {claim_id!r} already reserves coverage-gap {gap_id}"
                )
        elif new_work_kind == "coverage-gap" and existing_kind == "candidate-lane":
            for gap_id in sorted(new_gap_ids & existing_gap_ids):
                messages.append(
                    f"existing active claim {claim_id!r} already reserves candidate-lane promotion for coverage-gap {gap_id}"
                )

    return messages


def build_new_claim(
    *,
    agent_id: str,
    summary: str,
    target_id: str,
    work_kind: str,
    work_id: str,
    claimed_at: datetime,
    expires_at: datetime,
) -> dict[str, Any]:
    return {
        "id": claim_id_for(agent_id=agent_id, work_kind=work_kind, work_id=work_id),
        "agent_id": agent_id,
        "summary": summary,
        "target_id": target_id,
        "claimed_at": format_timestamp(claimed_at),
        "heartbeat_at": format_timestamp(claimed_at),
        "expires_at": format_timestamp(expires_at),
        "work_item": {
            "kind": work_kind,
            "id": work_id,
        },
    }


def claim_work(
    payload: dict[str, Any],
    *,
    work_kind: str,
    work_id: str,
    summary: str,
    agent_id: str,
    ttl_hours: float,
    target_id: str | None = None,
    now_utc: datetime | None = None,
) -> dict[str, Any]:
    if work_kind not in VALID_WORK_CLAIM_KINDS:
        raise ValueError(f"unsupported work claim kind {work_kind!r}")
    if ttl_hours <= 0:
        raise ValueError("ttl_hours must be greater than 0")
    if not summary.strip():
        raise ValueError("summary must be non-empty")
    if not agent_id.strip():
        raise ValueError("agent_id must be non-empty")

    now = now_utc or utc_now()
    status_payload = json.loads(json.dumps(payload))
    claims = require_work_claims(status_payload)
    active_claims, expired_claims = split_claims_by_state(claims, now_utc=now)
    work_context = work_item_context(status_payload, work_kind=work_kind, work_id=work_id)
    resolved_target_id = resolve_target_id(
        work_kind=work_kind,
        work_context=work_context,
        explicit_target_id=target_id.strip() if target_id else None,
    )

    same_agent_active = [claim for claim in active_claims if str(claim.get("agent_id")) == agent_id]
    exact_same_claim = [
        claim
        for claim in same_agent_active
        if claim_matches(
            claim,
            agent_id=agent_id,
            target_id=resolved_target_id,
            work_kind=work_kind,
            work_id=work_id,
        )
    ]

    replaced_claims: list[dict[str, Any]] = []
    if len(same_agent_active) == 1 and len(exact_same_claim) == 1:
        kept_active_claims = [claim for claim in active_claims if claim["id"] != exact_same_claim[0]["id"]]
        renewed_claim = dict(exact_same_claim[0])
        renewed_claim["summary"] = summary
        renewed_claim["heartbeat_at"] = format_timestamp(now)
        renewed_claim["expires_at"] = format_timestamp(now + timedelta(hours=ttl_hours))
        action = "renewed"
        next_claim = renewed_claim
    else:
        replaced_ids = {str(claim.get("id")) for claim in same_agent_active}
        replaced_claims = [dict(claim) for claim in same_agent_active]
        kept_active_claims = [claim for claim in active_claims if str(claim.get("id")) not in replaced_ids]
        action = "claimed"
        next_claim = build_new_claim(
            agent_id=agent_id,
            summary=summary,
            target_id=resolved_target_id,
            work_kind=work_kind,
            work_id=work_id,
            claimed_at=now,
            expires_at=now + timedelta(hours=ttl_hours),
        )

    conflicts = claim_overlap_messages(
        status_payload,
        new_work_kind=work_kind,
        new_work_id=work_id,
        existing_claims=kept_active_claims,
    )
    if conflicts:
        joined = "\n".join(f"- {message}" for message in conflicts)
        raise ValueError(f"cannot reserve {work_kind}:{work_id} because it overlaps active work:\n{joined}")

    final_claims = sorted(kept_active_claims + [next_claim], key=claim_sort_key)
    status_payload["work_claims"] = final_claims
    return {
        "action": action,
        "status_path": STATUS_REL,
        "active_target_id": ACTIVE_TARGET_ID,
        "claim": dict(next_claim),
        "replaced_claims": replaced_claims,
        "pruned_expired_claims": expired_claims,
        "payload": status_payload,
    }


def release_claims(
    payload: dict[str, Any],
    *,
    agent_id: str,
    work_kind: str | None = None,
    work_id: str | None = None,
    now_utc: datetime | None = None,
) -> dict[str, Any]:
    if not agent_id.strip():
        raise ValueError("agent_id must be non-empty")
    if (work_kind is None) != (work_id is None):
        raise ValueError("release filters must provide both work_kind and work_id together")
    if work_kind is not None and work_kind not in VALID_WORK_CLAIM_KINDS:
        raise ValueError(f"unsupported work claim kind {work_kind!r}")

    now = now_utc or utc_now()
    status_payload = json.loads(json.dumps(payload))
    claims = require_work_claims(status_payload)
    active_claims, expired_claims = split_claims_by_state(claims, now_utc=now)
    released_claims: list[dict[str, Any]] = []
    kept_active_claims: list[dict[str, Any]] = []

    for claim in active_claims:
        work_item = claim.get("work_item")
        claim_work_kind = str(work_item.get("kind", "")) if isinstance(work_item, dict) else ""
        claim_work_id = str(work_item.get("id", "")) if isinstance(work_item, dict) else ""
        matches_agent = str(claim.get("agent_id")) == agent_id
        matches_item = work_kind is None or (claim_work_kind == work_kind and claim_work_id == work_id)
        if matches_agent and matches_item:
            released_claims.append(dict(claim))
            continue
        kept_active_claims.append(dict(claim))

    if not released_claims:
        target_text = f"{work_kind}:{work_id}" if work_kind and work_id else "any active claim"
        raise ValueError(f"no active claim for agent {agent_id!r} matches {target_text}")

    status_payload["work_claims"] = sorted(kept_active_claims, key=claim_sort_key)
    return {
        "action": "released",
        "status_path": STATUS_REL,
        "active_target_id": ACTIVE_TARGET_ID,
        "released_claims": released_claims,
        "pruned_expired_claims": expired_claims,
        "payload": status_payload,
    }


def write_status_payload(payload: dict[str, Any]) -> None:
    STATUS_PATH.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Reserve or release a governed work claim.")
    parser.add_argument("--kind", help="Governed work item kind.")
    parser.add_argument("--id", dest="work_id", help="Governed work item id.")
    parser.add_argument("--summary", help="Short claim summary.")
    parser.add_argument("--agent-id", help="Stable agent id.")
    parser.add_argument("--target-id", help="Override claim target id; defaults to the active target.")
    parser.add_argument(
        "--ttl-hours",
        type=float,
        default=2.0,
        help="Lease duration in hours for new or renewed claims (default: 2).",
    )
    parser.add_argument(
        "--release",
        action="store_true",
        help="Release active claims for --agent-id instead of creating or renewing one.",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Print a concise human-readable summary instead of JSON.",
    )
    parser.add_argument(
        "--write",
        action="store_true",
        help="Persist the updated work_claims list back to status.json.",
    )
    parser.add_argument(
        "--now",
        help="Override the current UTC time for testing, in RFC3339 format.",
    )
    return parser.parse_args(argv)


def render_pretty(result: dict[str, Any]) -> str:
    lines = [
        f"work_claim: action={result['action']} write_applied={result['write_applied']} status_path={result['status_path']}",
        f"active_target={result['active_target_id']}",
    ]
    claim = result.get("claim")
    if isinstance(claim, dict):
        work_item = claim.get("work_item", {})
        lines.append(
            f"claim: id={claim.get('id')} agent={claim.get('agent_id')} target={claim.get('target_id')} "
            f"work={work_item.get('kind')}:{work_item.get('id')} "
            f"claimed_at={claim.get('claimed_at')} expires_at={claim.get('expires_at')}"
        )
        lines.append(f"  {claim.get('summary')}")
    if result.get("released_claims"):
        lines.append("released_claims:")
        for claim_item in result["released_claims"]:
            work_item = claim_item.get("work_item", {})
            lines.append(
                f"  - {claim_item.get('id')} agent={claim_item.get('agent_id')} "
                f"work={work_item.get('kind')}:{work_item.get('id')}"
            )
    if result.get("replaced_claims"):
        lines.append("replaced_claims:")
        for claim_item in result["replaced_claims"]:
            work_item = claim_item.get("work_item", {})
            lines.append(
                f"  - {claim_item.get('id')} target={claim_item.get('target_id')} "
                f"work={work_item.get('kind')}:{work_item.get('id')}"
            )
    if result.get("pruned_expired_claims"):
        lines.append("pruned_expired_claims:")
        for claim_item in result["pruned_expired_claims"]:
            work_item = claim_item.get("work_item", {})
            lines.append(
                f"  - {claim_item.get('id')} agent={claim_item.get('agent_id')} "
                f"work={work_item.get('kind')}:{work_item.get('id')} "
                f"expired_at={claim_item.get('expires_at')}"
            )
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    now = parse_timestamp(args.now) if args.now else utc_now()
    payload = load_status_payload()

    try:
        if args.release:
            result = release_claims(
                payload,
                agent_id=args.agent_id or "",
                work_kind=args.kind,
                work_id=args.work_id,
                now_utc=now,
            )
        else:
            result = claim_work(
                payload,
                work_kind=args.kind or "",
                work_id=args.work_id or "",
                summary=args.summary or "",
                agent_id=args.agent_id or "",
                ttl_hours=args.ttl_hours,
                target_id=args.target_id,
                now_utc=now,
            )
        if args.write:
            write_status_payload(result["payload"])
        result["write_applied"] = bool(args.write)
        result.pop("payload", None)
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc

    output = render_pretty(result) if args.pretty else json.dumps(result, indent=2, sort_keys=True)
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
