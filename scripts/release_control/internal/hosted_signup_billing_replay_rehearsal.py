#!/usr/bin/env python3
"""Exercise the hosted signup and billing replay gate on a live Pulse runtime."""

from __future__ import annotations

import argparse
import hashlib
import hmac
import json
import time
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
            "Run a live hosted-signup and billing-replay rehearsal against a hosted "
            "Pulse instance and optionally write a markdown report."
        )
    )
    parser.add_argument("--base-url", required=True, help="Hosted Pulse base URL")
    parser.add_argument(
        "--fail-closed-base-url",
        help="Optional second Pulse base URL expected to fail hosted signup when public URL is missing",
    )
    parser.add_argument("--signup-email", required=True, help="Email to use for public signup")
    parser.add_argument("--org-name", required=True, help="Organization name to use for public signup")
    parser.add_argument(
        "--timeout",
        type=float,
        default=30.0,
        help="HTTP timeout in seconds",
    )
    parser.add_argument("--api-token", help="Optional X-API-Token for authenticated admin checks")
    parser.add_argument("--bearer-token", help="Optional Authorization bearer token for admin checks")
    parser.add_argument("--cookie", help="Optional Cookie header for admin checks")
    parser.add_argument(
        "--expected-checkout-base",
        help="Optional prefix expected for trial-start redirect action_url",
    )
    parser.add_argument(
        "--billing-org-id",
        help="Optional org ID to use for billing-state checks instead of the signup org_id",
    )
    parser.add_argument(
        "--expected-trial-subscription-state",
        default="trial",
        help="Expected subscription_state immediately after hosted signup",
    )
    parser.add_argument(
        "--expected-trial-plan-version",
        default="cloud_trial",
        help="Expected plan_version immediately after hosted signup",
    )
    parser.add_argument(
        "--prelink-webhook-payload-file",
        help="Optional JSON file containing a Stripe event payload expected to fail closed before linkage",
    )
    parser.add_argument(
        "--prelink-webhook-secret",
        help="Optional Stripe webhook secret for the pre-link payload",
    )
    parser.add_argument(
        "--prelink-webhook-expected-status",
        type=int,
        default=500,
        help="Expected HTTP status for the pre-link webhook delivery",
    )
    parser.add_argument(
        "--postlink-webhook-payload-file",
        help="Optional JSON file containing a Stripe event payload expected to succeed after linkage",
    )
    parser.add_argument(
        "--postlink-webhook-secret",
        help="Optional Stripe webhook secret for the post-link payload",
    )
    parser.add_argument(
        "--postlink-webhook-expected-status",
        type=int,
        default=200,
        help="Expected HTTP status for the post-link webhook delivery",
    )
    parser.add_argument(
        "--expected-postlink-subscription-state",
        help="Optional expected subscription_state after the post-link webhook succeeds",
    )
    parser.add_argument(
        "--expected-postlink-plan-version",
        help="Optional expected plan_version after the post-link webhook succeeds",
    )
    parser.add_argument(
        "--report-out",
        help="Optional markdown report destination",
    )
    parser.add_argument(
        "--report-title",
        default="Hosted Signup Billing Replay Rehearsal",
        help="Markdown title used when writing --report-out",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit JSON instead of human-readable output",
    )
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
    body: bytes | None = None,
    headers: dict[str, str] | None = None,
) -> tuple[int, bytes, dict[str, str]]:
    req_headers = dict(headers or {})
    request_body = body
    if json_body is not None:
        request_body = json.dumps(json_body).encode("utf-8")
        req_headers.setdefault("Content-Type", "application/json")
    req = request.Request(url, data=request_body, headers=req_headers, method=method)
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
    body: bytes | None = None,
    headers: dict[str, str] | None = None,
) -> tuple[int, dict[str, Any]]:
    status, payload, _headers = fetch(
        method,
        url,
        timeout=timeout,
        json_body=json_body,
        body=body,
        headers=headers,
    )
    try:
        parsed = json.loads(payload.decode("utf-8")) if payload else {}
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"{method} {url} returned non-JSON body: {summarize_body(payload)}") from exc
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{method} {url} did not return a JSON object")
    return status, parsed


def make_stripe_signature(payload: bytes, secret: str, timestamp: int | None = None) -> str:
    ts = int(time.time() if timestamp is None else timestamp)
    signed_payload = f"{ts}.".encode("utf-8") + payload
    digest = hmac.new(secret.encode("utf-8"), signed_payload, hashlib.sha256).hexdigest()
    return f"t={ts},v1={digest}"


def safe_check(name: str, fn) -> CheckResult:
    try:
        return fn()
    except Exception as exc:  # pragma: no cover - exercised via callers/tests
        return CheckResult(name=name, ok=False, detail=str(exc))


def check_fail_closed(args: argparse.Namespace) -> CheckResult:
    status, payload = fetch_json(
        "POST",
        f"{normalize_base_url(args.fail_closed_base_url)}/api/public/signup",
        timeout=args.timeout,
        json_body={"email": args.signup_email, "org_name": args.org_name},
    )
    if status != 503:
        return CheckResult(
            name="fail-closed-signup-without-public-url",
            ok=False,
            detail=f"status={status}, expected 503",
        )
    if str(payload.get("code", "")).strip() != "public_url_missing":
        return CheckResult(
            name="fail-closed-signup-without-public-url",
            ok=False,
            detail=f"code={payload.get('code')!r}, expected 'public_url_missing'",
        )
    return CheckResult(
        name="fail-closed-signup-without-public-url",
        ok=True,
        detail="signup failed closed with code=public_url_missing",
    )


def check_trial_start_redirect(args: argparse.Namespace, auth_headers: dict[str, str]) -> CheckResult:
    if not auth_headers:
        return CheckResult(
            name="self-hosted-trial-redirect-to-hosted",
            ok=False,
            detail="authenticated admin credentials are required for this check",
        )
    status, payload = fetch_json(
        "POST",
        f"{normalize_base_url(args.base_url)}/api/license/trial/start",
        timeout=args.timeout,
        headers=auth_headers,
        json_body={},
    )
    if status != 409:
        return CheckResult(
            name="self-hosted-trial-redirect-to-hosted",
            ok=False,
            detail=f"status={status}, expected 409",
        )
    code = str(payload.get("code", "")).strip()
    if code != "trial_signup_required":
        return CheckResult(
            name="self-hosted-trial-redirect-to-hosted",
            ok=False,
            detail=f"code={code!r}, expected 'trial_signup_required'",
        )
    details = payload.get("details")
    if not isinstance(details, dict):
        return CheckResult(
            name="self-hosted-trial-redirect-to-hosted",
            ok=False,
            detail="missing details.action_url in response",
        )
    action_url = str(details.get("action_url", "")).strip()
    if not action_url:
        return CheckResult(
            name="self-hosted-trial-redirect-to-hosted",
            ok=False,
            detail="details.action_url was empty",
        )
    if args.expected_checkout_base and not action_url.startswith(args.expected_checkout_base):
        return CheckResult(
            name="self-hosted-trial-redirect-to-hosted",
            ok=False,
            detail=f"action_url={action_url!r} did not start with {args.expected_checkout_base!r}",
        )
    return CheckResult(
        name="self-hosted-trial-redirect-to-hosted",
        ok=True,
        detail=f"returned action_url={action_url}",
    )


def check_public_signup(args: argparse.Namespace) -> tuple[CheckResult, str]:
    status, payload = fetch_json(
        "POST",
        f"{normalize_base_url(args.base_url)}/api/public/signup",
        timeout=args.timeout,
        json_body={"email": args.signup_email, "org_name": args.org_name},
    )
    if status != 201:
        return (
            CheckResult(name="public-hosted-signup", ok=False, detail=f"status={status}, payload={payload!r}"),
            "",
        )
    org_id = str(payload.get("org_id", "")).strip()
    message = str(payload.get("message", "")).strip()
    if not org_id:
        return (
            CheckResult(name="public-hosted-signup", ok=False, detail="response missing org_id"),
            "",
        )
    if message != "Check your email for a magic link to finish signing in.":
        return (
            CheckResult(
                name="public-hosted-signup",
                ok=False,
                detail=f"unexpected signup message {message!r}",
            ),
            org_id,
        )
    return (
        CheckResult(name="public-hosted-signup", ok=True, detail=f"created org_id={org_id}"),
        org_id,
    )


def check_magic_link_request(args: argparse.Namespace) -> CheckResult:
    status, payload = fetch_json(
        "POST",
        f"{normalize_base_url(args.base_url)}/api/public/magic-link/request",
        timeout=args.timeout,
        json_body={"email": args.signup_email},
    )
    if status != 200:
        return CheckResult(
            name="public-magic-link-request",
            ok=False,
            detail=f"status={status}, payload={payload!r}",
        )
    if payload.get("success") is not True:
        return CheckResult(
            name="public-magic-link-request",
            ok=False,
            detail=f"success={payload.get('success')!r}, expected true",
        )
    return CheckResult(
        name="public-magic-link-request",
        ok=True,
        detail="magic-link request succeeded",
    )


def check_hosted_org_list(base_url: str, timeout: float, auth_headers: dict[str, str], expected_org_id: str) -> CheckResult:
    if not auth_headers:
        return CheckResult(
            name="hosted-org-list",
            ok=False,
            detail="authenticated admin credentials are required for this check",
        )
    status, payload = fetch_json(
        "GET",
        f"{normalize_base_url(base_url)}/api/hosted/organizations",
        timeout=timeout,
        headers=auth_headers,
    )
    if status != 200:
        return CheckResult(name="hosted-org-list", ok=False, detail=f"status={status}, payload={payload!r}")
    orgs: object
    if isinstance(payload, list):
        orgs = payload
    else:
        orgs = payload.get("organizations", payload)
    if not isinstance(orgs, list):
        return CheckResult(name="hosted-org-list", ok=False, detail="payload did not contain an organization list")
    for entry in orgs:
        if isinstance(entry, dict) and str(entry.get("org_id", "")).strip() == expected_org_id:
            return CheckResult(name="hosted-org-list", ok=True, detail=f"org_id={expected_org_id} is visible")
    return CheckResult(name="hosted-org-list", ok=False, detail=f"org_id={expected_org_id} not found")


def check_billing_state(
    *,
    base_url: str,
    timeout: float,
    auth_headers: dict[str, str],
    org_id: str,
    expected_subscription_state: str | None,
    expected_plan_version: str | None,
    name: str,
) -> CheckResult:
    if not auth_headers:
        return CheckResult(name=name, ok=False, detail="authenticated admin credentials are required for this check")
    status, payload = fetch_json(
        "GET",
        f"{normalize_base_url(base_url)}/api/admin/orgs/{org_id}/billing-state",
        timeout=timeout,
        headers=auth_headers,
    )
    if status != 200:
        return CheckResult(name=name, ok=False, detail=f"status={status}, payload={payload!r}")
    got_state = str(payload.get("subscription_state", "")).strip()
    got_plan = str(payload.get("plan_version", "")).strip()
    if expected_subscription_state and got_state != expected_subscription_state:
        return CheckResult(
            name=name,
            ok=False,
            detail=f"subscription_state={got_state!r}, expected {expected_subscription_state!r}",
        )
    if expected_plan_version and got_plan != expected_plan_version:
        return CheckResult(
            name=name,
            ok=False,
            detail=f"plan_version={got_plan!r}, expected {expected_plan_version!r}",
        )
    return CheckResult(
        name=name,
        ok=True,
        detail=f"subscription_state={got_state!r} plan_version={got_plan!r}",
    )


def load_json_payload(path: str) -> bytes:
    raw = Path(path).read_bytes()
    json.loads(raw.decode("utf-8"))
    return raw


def check_signed_webhook_delivery(
    *,
    base_url: str,
    timeout: float,
    payload_file: str,
    secret: str,
    expected_status: int,
    name: str,
) -> CheckResult:
    payload = load_json_payload(payload_file)
    signature = make_stripe_signature(payload, secret)
    status, body, _headers = fetch(
        "POST",
        f"{normalize_base_url(base_url)}/api/stripe/webhook",
        timeout=timeout,
        body=payload,
        headers={
            "Content-Type": "application/json",
            "Stripe-Signature": signature,
        },
    )
    if status != expected_status:
        return CheckResult(
            name=name,
            ok=False,
            detail=f"status={status}, expected {expected_status}, body={summarize_body(body)}",
        )
    return CheckResult(
        name=name,
        ok=True,
        detail=f"status={status}, body={summarize_body(body)}",
    )


def run_rehearsal(args: argparse.Namespace) -> list[CheckResult]:
    auth_headers = build_auth_headers(args)
    results: list[CheckResult] = []

    if args.fail_closed_base_url:
        results.append(safe_check("fail-closed-signup-without-public-url", lambda: check_fail_closed(args)))

    if auth_headers:
        results.append(
            safe_check(
                "self-hosted-trial-redirect-to-hosted",
                lambda: check_trial_start_redirect(args, auth_headers),
            )
        )

    signup_result, signup_org_id = check_public_signup(args)
    results.append(signup_result)
    if signup_org_id:
        results.append(safe_check("public-magic-link-request", lambda: check_magic_link_request(args)))

    billing_org_id = args.billing_org_id or signup_org_id
    if auth_headers and signup_org_id:
        results.append(
            safe_check(
                "hosted-org-list",
                lambda: check_hosted_org_list(args.base_url, args.timeout, auth_headers, signup_org_id),
            )
        )
    if auth_headers and billing_org_id:
        results.append(
            safe_check(
                "billing-state-after-signup",
                lambda: check_billing_state(
                    base_url=args.base_url,
                    timeout=args.timeout,
                    auth_headers=auth_headers,
                    org_id=billing_org_id,
                    expected_subscription_state=args.expected_trial_subscription_state,
                    expected_plan_version=args.expected_trial_plan_version,
                    name="billing-state-after-signup",
                ),
            )
        )

    if args.prelink_webhook_payload_file and args.prelink_webhook_secret:
        results.append(
            safe_check(
                "prelink-webhook-delivery",
                lambda: check_signed_webhook_delivery(
                    base_url=args.base_url,
                    timeout=args.timeout,
                    payload_file=args.prelink_webhook_payload_file,
                    secret=args.prelink_webhook_secret,
                    expected_status=args.prelink_webhook_expected_status,
                    name="prelink-webhook-delivery",
                ),
            )
        )
    if args.postlink_webhook_payload_file and args.postlink_webhook_secret:
        results.append(
            safe_check(
                "postlink-webhook-delivery",
                lambda: check_signed_webhook_delivery(
                    base_url=args.base_url,
                    timeout=args.timeout,
                    payload_file=args.postlink_webhook_payload_file,
                    secret=args.postlink_webhook_secret,
                    expected_status=args.postlink_webhook_expected_status,
                    name="postlink-webhook-delivery",
                ),
            )
        )
        if auth_headers and billing_org_id and (
            args.expected_postlink_subscription_state or args.expected_postlink_plan_version
        ):
            results.append(
                safe_check(
                    "billing-state-after-webhook",
                    lambda: check_billing_state(
                        base_url=args.base_url,
                        timeout=args.timeout,
                        auth_headers=auth_headers,
                        org_id=billing_org_id,
                        expected_subscription_state=args.expected_postlink_subscription_state,
                        expected_plan_version=args.expected_postlink_plan_version,
                        name="billing-state-after-webhook",
                    ),
                )
            )

    return results


def render_markdown_report(
    *,
    title: str,
    base_url: str,
    signup_email: str,
    org_name: str,
    results: list[CheckResult],
) -> str:
    lines = [
        f"# {title}",
        "",
        "## Inputs",
        "",
        f"- Base URL: `{base_url}`",
        f"- Signup email: `{signup_email}`",
        f"- Org name: `{org_name}`",
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
            "- If the webhook replay was only partially exercised, complete the missing replay/linkage path on the same hosted runtime and append the result.",
            "- If the live runtime differs from the final hosted environment, rerun the same command against the actual external hosted surface before closing the gate.",
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
                signup_email=args.signup_email,
                org_name=args.org_name,
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
