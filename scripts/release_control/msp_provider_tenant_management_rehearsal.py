#!/usr/bin/env python3
"""Exercise the MSP provider tenant management gate on a live Pulse runtime."""

from __future__ import annotations

import argparse
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any
from urllib import error, request


@dataclass
class CheckResult:
    name: str
    ok: bool
    detail: str


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Run a live MSP provider-tenant rehearsal against a Pulse control-plane "
            "runtime and optionally write a markdown report."
        )
    )
    parser.add_argument("--base-url", required=True, help="Pulse control-plane base URL")
    parser.add_argument("--account-id", required=True, help="MSP account ID to exercise")
    parser.add_argument("--timeout", type=float, default=30.0, help="HTTP timeout in seconds")
    parser.add_argument("--api-token", help="Optional X-API-Token for authenticated checks")
    parser.add_argument("--bearer-token", help="Optional bearer token for authenticated checks")
    parser.add_argument("--cookie", help="Optional raw Cookie header for authenticated checks")
    parser.add_argument(
        "--workspace-name",
        action="append",
        default=[],
        help="Workspace name to create under the MSP account; may be passed multiple times",
    )
    parser.add_argument(
        "--expected-plan-version",
        default="msp_starter",
        help="Expected canonical MSP plan version on created or returned workspaces",
    )
    parser.add_argument(
        "--member-email",
        action="append",
        default=[],
        help="Member email to invite under the MSP account; may be passed multiple times",
    )
    parser.add_argument(
        "--member-role",
        action="append",
        default=[],
        help="Role for the matching --member-email entry; defaults to tech when omitted",
    )
    parser.add_argument(
        "--expected-account-kind",
        default="msp",
        help="Expected account.kind value on portal responses",
    )
    parser.add_argument(
        "--public-signup-email",
        help="Optional email to use for the public-cloud boundary signup check",
    )
    parser.add_argument(
        "--public-signup-org-name",
        help="Optional org_name to use for the public-cloud boundary signup check",
    )
    parser.add_argument(
        "--public-signup-tier",
        default="power",
        help="Tier value to submit to public signup for the MSP/public boundary check",
    )
    parser.add_argument(
        "--public-signup-expected-status",
        type=int,
        default=400,
        help="Expected HTTP status for the MSP/public signup boundary check",
    )
    parser.add_argument(
        "--public-signup-expected-code",
        default="tier_unavailable",
        help="Expected error code for the MSP/public signup boundary check",
    )
    parser.add_argument("--report-out", help="Optional markdown report destination")
    parser.add_argument(
        "--report-title",
        default="MSP Provider Tenant Management Rehearsal",
        help="Markdown title used when writing --report-out",
    )
    parser.add_argument("--json", action="store_true", help="Emit JSON instead of human-readable output")
    return parser.parse_args(argv)


def normalize_base_url(raw: str) -> str:
    return raw.rstrip("/")


def build_auth_headers(args: argparse.Namespace) -> dict[str, str]:
    headers: dict[str, str] = {}
    if args.api_token:
        headers["X-API-Token"] = args.api_token
    if args.bearer_token:
        headers["Authorization"] = f"Bearer {args.bearer_token}"
    if args.cookie:
        headers["Cookie"] = args.cookie
    return headers


def summarize_body(body: bytes) -> str:
    text = body.decode("utf-8", errors="replace").strip()
    if not text:
        return "<empty response>"
    first_line = text.splitlines()[0].strip()
    if len(first_line) > 240:
        return first_line[:237] + "..."
    return first_line


def fetch(
    method: str,
    url: str,
    *,
    timeout: float,
    json_body: dict[str, Any] | None = None,
    headers: dict[str, str] | None = None,
) -> tuple[int, bytes, dict[str, str]]:
    req_headers = dict(headers or {})
    body = None
    if json_body is not None:
        body = json.dumps(json_body).encode("utf-8")
        req_headers.setdefault("Content-Type", "application/json")
    req = request.Request(url, data=body, headers=req_headers, method=method)
    try:
        with request.urlopen(req, timeout=timeout) as resp:
            payload = resp.read()
            return resp.status, payload, {key.lower(): value for key, value in resp.headers.items()}
    except error.HTTPError as exc:
        payload = exc.read()
        return exc.code, payload, {key.lower(): value for key, value in exc.headers.items()}
    except error.URLError as exc:
        raise RuntimeError(f"{method} {url} failed: {exc.reason}") from exc


def fetch_json(
    method: str,
    url: str,
    *,
    timeout: float,
    json_body: dict[str, Any] | None = None,
    headers: dict[str, str] | None = None,
) -> tuple[int, Any]:
    status, payload, _headers = fetch(method, url, timeout=timeout, json_body=json_body, headers=headers)
    try:
        return status, json.loads(payload.decode("utf-8")) if payload else {}
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"{method} {url} returned non-JSON body: {summarize_body(payload)}") from exc


def safe_check(name: str, fn) -> CheckResult:
    try:
        return fn()
    except Exception as exc:  # pragma: no cover
        return CheckResult(name=name, ok=False, detail=str(exc))


def check_account_tenants(base_url: str, timeout: float, headers: dict[str, str], account_id: str) -> tuple[int, list[dict[str, Any]]]:
    status, payload = fetch_json(
        "GET",
        f"{normalize_base_url(base_url)}/api/accounts/{account_id}/tenants",
        timeout=timeout,
        headers=headers,
    )
    if not isinstance(payload, list):
        raise RuntimeError("tenant list payload was not a JSON array")
    return status, payload


def check_member_list(base_url: str, timeout: float, headers: dict[str, str], account_id: str) -> tuple[int, list[dict[str, Any]]]:
    status, payload = fetch_json(
        "GET",
        f"{normalize_base_url(base_url)}/api/accounts/{account_id}/members",
        timeout=timeout,
        headers=headers,
    )
    if not isinstance(payload, list):
        raise RuntimeError("member list payload was not a JSON array")
    return status, payload


def list_tenants_check(args: argparse.Namespace, headers: dict[str, str]) -> CheckResult:
    if not headers:
        return CheckResult(name="msp-tenant-list", ok=False, detail="authenticated credentials are required")
    status, tenants = check_account_tenants(args.base_url, args.timeout, headers, args.account_id)
    if status != 200:
        return CheckResult(name="msp-tenant-list", ok=False, detail=f"status={status}")
    return CheckResult(name="msp-tenant-list", ok=True, detail=f"tenant_count={len(tenants)}")


def create_workspace_check(args: argparse.Namespace, headers: dict[str, str], display_name: str) -> CheckResult:
    status, payload = fetch_json(
        "POST",
        f"{normalize_base_url(args.base_url)}/api/accounts/{args.account_id}/tenants",
        timeout=args.timeout,
        headers=headers,
        json_body={"display_name": display_name},
    )
    if status != 201:
        return CheckResult(
            name=f"msp-create-workspace:{display_name}",
            ok=False,
            detail=f"status={status}, payload={payload!r}",
        )
    if not isinstance(payload, dict):
        return CheckResult(name=f"msp-create-workspace:{display_name}", ok=False, detail="payload was not a JSON object")
    plan_version = str(payload.get("plan_version", "")).strip()
    tenant_id = str(payload.get("id", "")).strip()
    account_id = str(payload.get("account_id", "")).strip()
    if not tenant_id:
        return CheckResult(name=f"msp-create-workspace:{display_name}", ok=False, detail="response missing tenant id")
    if account_id != args.account_id:
        return CheckResult(
            name=f"msp-create-workspace:{display_name}",
            ok=False,
            detail=f"account_id={account_id!r}, expected {args.account_id!r}",
        )
    if args.expected_plan_version and plan_version != args.expected_plan_version:
        return CheckResult(
            name=f"msp-create-workspace:{display_name}",
            ok=False,
            detail=f"plan_version={plan_version!r}, expected {args.expected_plan_version!r}",
        )
    return CheckResult(
        name=f"msp-create-workspace:{display_name}",
        ok=True,
        detail=f"tenant_id={tenant_id} plan_version={plan_version!r}",
    )


def invite_member_check(args: argparse.Namespace, headers: dict[str, str], email: str, role: str) -> CheckResult:
    status, payload = fetch_json(
        "POST",
        f"{normalize_base_url(args.base_url)}/api/accounts/{args.account_id}/members",
        timeout=args.timeout,
        headers=headers,
        json_body={"email": email, "role": role},
    )
    if status != 201:
        return CheckResult(
            name=f"msp-invite-member:{email}",
            ok=False,
            detail=f"status={status}, payload={payload!r}",
        )
    return CheckResult(name=f"msp-invite-member:{email}", ok=True, detail=f"role={role}")


def portal_dashboard_check(args: argparse.Namespace, headers: dict[str, str], expected_min_total: int) -> CheckResult:
    status, payload = fetch_json(
        "GET",
        f"{normalize_base_url(args.base_url)}/api/portal/dashboard?account_id={args.account_id}",
        timeout=args.timeout,
        headers=headers,
    )
    if status != 200:
        return CheckResult(name="msp-portal-dashboard", ok=False, detail=f"status={status}, payload={payload!r}")
    if not isinstance(payload, dict):
        return CheckResult(name="msp-portal-dashboard", ok=False, detail="payload was not a JSON object")
    account = payload.get("account")
    summary = payload.get("summary")
    if not isinstance(account, dict) or not isinstance(summary, dict):
        return CheckResult(name="msp-portal-dashboard", ok=False, detail="missing account or summary object")
    kind = str(account.get("kind", "")).strip()
    total = int(summary.get("total", 0))
    if args.expected_account_kind and kind != args.expected_account_kind:
        return CheckResult(
            name="msp-portal-dashboard",
            ok=False,
            detail=f"account.kind={kind!r}, expected {args.expected_account_kind!r}",
        )
    if total < expected_min_total:
        return CheckResult(
            name="msp-portal-dashboard",
            ok=False,
            detail=f"summary.total={total}, expected at least {expected_min_total}",
        )
    return CheckResult(name="msp-portal-dashboard", ok=True, detail=f"account.kind={kind!r} summary.total={total}")


def workspace_detail_check(args: argparse.Namespace, headers: dict[str, str], tenant_id: str, expected_name: str) -> CheckResult:
    status, payload = fetch_json(
        "GET",
        f"{normalize_base_url(args.base_url)}/api/portal/workspaces/{tenant_id}?account_id={args.account_id}",
        timeout=args.timeout,
        headers=headers,
    )
    if status != 200:
        return CheckResult(name=f"msp-workspace-detail:{tenant_id}", ok=False, detail=f"status={status}, payload={payload!r}")
    if not isinstance(payload, dict):
        return CheckResult(name=f"msp-workspace-detail:{tenant_id}", ok=False, detail="payload was not a JSON object")
    account = payload.get("account")
    workspace = payload.get("workspace")
    if not isinstance(account, dict) or not isinstance(workspace, dict):
        return CheckResult(name=f"msp-workspace-detail:{tenant_id}", ok=False, detail="missing account or workspace object")
    kind = str(account.get("kind", "")).strip()
    display_name = str(workspace.get("display_name", "")).strip()
    plan_version = str(workspace.get("plan_version", "")).strip()
    if args.expected_account_kind and kind != args.expected_account_kind:
        return CheckResult(
            name=f"msp-workspace-detail:{tenant_id}",
            ok=False,
            detail=f"account.kind={kind!r}, expected {args.expected_account_kind!r}",
        )
    if expected_name and display_name != expected_name:
        return CheckResult(
            name=f"msp-workspace-detail:{tenant_id}",
            ok=False,
            detail=f"display_name={display_name!r}, expected {expected_name!r}",
        )
    if args.expected_plan_version and plan_version != args.expected_plan_version:
        return CheckResult(
            name=f"msp-workspace-detail:{tenant_id}",
            ok=False,
            detail=f"plan_version={plan_version!r}, expected {args.expected_plan_version!r}",
        )
    return CheckResult(
        name=f"msp-workspace-detail:{tenant_id}",
        ok=True,
        detail=f"display_name={display_name!r} plan_version={plan_version!r}",
    )


def public_signup_boundary_check(args: argparse.Namespace) -> CheckResult:
    if not args.public_signup_email or not args.public_signup_org_name:
        return CheckResult(name="public-cloud-boundary", ok=True, detail="skipped (no public signup inputs provided)")
    status, payload = fetch_json(
        "POST",
        f"{normalize_base_url(args.base_url)}/api/public/signup",
        timeout=args.timeout,
        json_body={
            "email": args.public_signup_email,
            "org_name": args.public_signup_org_name,
            "tier": args.public_signup_tier,
        },
    )
    if status != args.public_signup_expected_status:
        return CheckResult(
            name="public-cloud-boundary",
            ok=False,
            detail=f"status={status}, expected {args.public_signup_expected_status}, payload={payload!r}",
        )
    if not isinstance(payload, dict):
        return CheckResult(name="public-cloud-boundary", ok=False, detail="payload was not a JSON object")
    code = str(payload.get("code", "")).strip()
    if args.public_signup_expected_code and code != args.public_signup_expected_code:
        return CheckResult(
            name="public-cloud-boundary",
            ok=False,
            detail=f"code={code!r}, expected {args.public_signup_expected_code!r}",
        )
    return CheckResult(name="public-cloud-boundary", ok=True, detail=f"status={status} code={code!r}")


def member_pairs(args: argparse.Namespace) -> list[tuple[str, str]]:
    roles = list(args.member_role)
    while len(roles) < len(args.member_email):
        roles.append("tech")
    return list(zip(args.member_email, roles))


def run_rehearsal(args: argparse.Namespace) -> list[CheckResult]:
    headers = build_auth_headers(args)
    results: list[CheckResult] = []
    created_workspace_ids: list[tuple[str, str]] = []

    results.append(safe_check("msp-tenant-list", lambda: list_tenants_check(args, headers)))

    for workspace_name in args.workspace_name:
        result = create_workspace_check(args, headers, workspace_name)
        results.append(result)
        if result.ok:
            detail_parts = result.detail.split()
            tenant_id = detail_parts[0].split("=", 1)[1]
            created_workspace_ids.append((tenant_id, workspace_name))

    if headers:
        status, tenants = check_account_tenants(args.base_url, args.timeout, headers, args.account_id)
        if status == 200:
            expected_total = len(tenants)
        else:
            expected_total = max(1, len(created_workspace_ids))
    else:
        expected_total = max(1, len(created_workspace_ids))

    for email, role in member_pairs(args):
        results.append(safe_check(f"msp-invite-member:{email}", lambda email=email, role=role: invite_member_check(args, headers, email, role)))

    results.append(safe_check("msp-member-list", lambda: _member_list_expectations(args, headers)))
    results.append(safe_check("msp-portal-dashboard", lambda: portal_dashboard_check(args, headers, expected_total)))

    for tenant_id, workspace_name in created_workspace_ids:
        results.append(
            safe_check(
                f"msp-workspace-detail:{tenant_id}",
                lambda tenant_id=tenant_id, workspace_name=workspace_name: workspace_detail_check(
                    args,
                    headers,
                    tenant_id,
                    workspace_name,
                ),
            )
        )

    results.append(safe_check("public-cloud-boundary", lambda: public_signup_boundary_check(args)))
    return results


def _member_list_expectations(args: argparse.Namespace, headers: dict[str, str]) -> CheckResult:
    if not headers:
        return CheckResult(name="msp-member-list", ok=False, detail="authenticated credentials are required")
    status, members = check_member_list(args.base_url, args.timeout, headers, args.account_id)
    if status != 200:
        return CheckResult(name="msp-member-list", ok=False, detail=f"status={status}")
    emails = {str(item.get('email', '')).strip() for item in members if isinstance(item, dict)}
    missing = [email for email, _role in member_pairs(args) if email not in emails]
    if missing:
        return CheckResult(name="msp-member-list", ok=False, detail=f"missing members: {', '.join(missing)}")
    return CheckResult(name="msp-member-list", ok=True, detail=f"member_count={len(members)}")


def render_markdown_report(
    *,
    title: str,
    base_url: str,
    account_id: str,
    results: list[CheckResult],
) -> str:
    lines = [
        f"# {title}",
        "",
        "## Inputs",
        "",
        f"- Base URL: `{base_url}`",
        f"- Account ID: `{account_id}`",
        "",
        "## Results",
        "",
    ]
    for result in results:
        status = "PASS" if result.ok else "FAIL"
        lines.append(f"- `{status}` `{result.name}`")
        lines.append(f"  - {result.detail}")
    lines.extend(
        [
            "",
            "## Manual Follow-up",
            "",
            "- If this was run against a local or staging-only control plane, rerun it against the real external MSP environment before closing the gate.",
            "- If account creation or billing bootstrap still required pre-seeding, record exactly which parts remained Stripe- or operations-driven outside the rehearsal surface.",
        ]
    )
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    results = run_rehearsal(args)
    if args.report_out:
        Path(args.report_out).write_text(
            render_markdown_report(
                title=args.report_title,
                base_url=args.base_url,
                account_id=args.account_id,
                results=results,
            ),
            encoding="utf-8",
        )
    if args.json:
        print(json.dumps([asdict(result) for result in results], indent=2))
    else:
        for result in results:
            status = "PASS" if result.ok else "FAIL"
            print(f"{status} {result.name}: {result.detail}")
    return 0 if results and all(result.ok for result in results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
