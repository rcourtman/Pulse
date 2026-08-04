#!/usr/bin/env python3
"""Collect and verify machine-readable proof from a running Pulse release."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import ssl
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


SCHEMA_VERSION = 1
PROOF_TYPE = "pulse-live-runtime"
ASSERTION = "successful-restore-points-have-evaluated-posture"
DEFAULT_TIMEOUT_SECONDS = 20.0
MAX_RESPONSE_BYTES = 16 * 1024 * 1024
ENV_NAME_PATTERN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


class ProofError(RuntimeError):
    """Raised when live proof cannot be collected or verified."""


def _canonical_json(value: object) -> bytes:
    return json.dumps(
        value,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    ).encode("utf-8")


def _sha256(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def _receipt_digest(receipt: dict[str, Any]) -> str:
    unsigned = dict(receipt)
    unsigned.pop("receiptSha256", None)
    return _sha256(_canonical_json(unsigned))


def seal_receipt(receipt: dict[str, Any]) -> dict[str, Any]:
    sealed = dict(receipt)
    sealed["receiptSha256"] = _receipt_digest(sealed)
    return sealed


def _normalize_version(value: str) -> str:
    normalized = value.strip()
    if normalized.lower().startswith("v"):
        normalized = normalized[1:]
    return normalized


def normalize_origin(value: str) -> str:
    parsed = urllib.parse.urlsplit(value.strip())
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        raise ProofError("base URL must be an absolute http:// or https:// URL")
    if parsed.username or parsed.password:
        raise ProofError("base URL must not contain credentials")
    if parsed.query or parsed.fragment:
        raise ProofError("base URL must not contain a query or fragment")
    if parsed.path not in {"", "/"}:
        raise ProofError("base URL must contain only the target origin")
    return urllib.parse.urlunsplit((parsed.scheme, parsed.netloc, "", "", ""))


def _secret_header(env_name: str, header_name: str) -> dict[str, str]:
    if not env_name:
        return {}
    if not ENV_NAME_PATTERN.fullmatch(env_name):
        raise ProofError(f"{header_name} environment variable name is invalid")
    value = os.environ.get(env_name, "").strip()
    if not value:
        raise ProofError(f"{header_name} environment variable {env_name!r} is empty")
    return {header_name: value}


def build_headers(authorization_env: str, cookie_env: str) -> dict[str, str]:
    headers = {"Accept": "application/json"}
    headers.update(_secret_header(authorization_env, "Authorization"))
    headers.update(_secret_header(cookie_env, "Cookie"))
    return headers


def _read_json_response(
    url: str,
    *,
    headers: dict[str, str],
    timeout_seconds: float,
    tls_verified: bool,
) -> tuple[dict[str, Any], str]:
    request = urllib.request.Request(url, headers=headers, method="GET")
    context = None
    if urllib.parse.urlsplit(url).scheme == "https" and not tls_verified:
        context = ssl._create_unverified_context()  # noqa: SLF001 - explicit operator option
    try:
        with urllib.request.urlopen(request, timeout=timeout_seconds, context=context) as response:
            body = response.read(MAX_RESPONSE_BYTES + 1)
            if len(body) > MAX_RESPONSE_BYTES:
                raise ProofError(f"response exceeded {MAX_RESPONSE_BYTES} bytes")
            if response.status != 200:
                raise ProofError(f"GET {urllib.parse.urlsplit(url).path} returned HTTP {response.status}")
    except urllib.error.HTTPError as exc:
        raise ProofError(
            f"GET {urllib.parse.urlsplit(url).path} returned HTTP {exc.code}"
        ) from exc
    except urllib.error.URLError as exc:
        reason = str(exc.reason) if exc.reason else exc.__class__.__name__
        raise ProofError(f"GET {urllib.parse.urlsplit(url).path} failed: {reason}") from exc
    except TimeoutError as exc:
        raise ProofError(f"GET {urllib.parse.urlsplit(url).path} timed out") from exc

    try:
        payload = json.loads(body)
    except json.JSONDecodeError as exc:
        raise ProofError(f"GET {urllib.parse.urlsplit(url).path} returned invalid JSON") from exc
    if not isinstance(payload, dict):
        raise ProofError(f"GET {urllib.parse.urlsplit(url).path} did not return a JSON object")
    return payload, _sha256(body)


def _collect_postures(
    origin: str,
    *,
    headers: dict[str, str],
    timeout_seconds: float,
    tls_verified: bool,
) -> tuple[list[dict[str, Any]], list[str], int]:
    postures: list[dict[str, Any]] = []
    response_hashes: list[str] = []
    page = 1
    expected_total: int | None = None

    while True:
        url = f"{origin}/api/recovery/postures?page={page}&limit=200"
        payload, response_hash = _read_json_response(
            url,
            headers=headers,
            timeout_seconds=timeout_seconds,
            tls_verified=tls_verified,
        )
        data = payload.get("data")
        meta = payload.get("meta")
        if not isinstance(data, list) or not isinstance(meta, dict):
            raise ProofError("posture response must contain data[] and meta{}")
        if not all(isinstance(item, dict) for item in data):
            raise ProofError("posture response data must contain JSON objects")

        total = meta.get("total")
        total_pages = meta.get("totalPages")
        response_page = meta.get("page")
        if not isinstance(total, int) or total < 0:
            raise ProofError("posture response meta.total must be a non-negative integer")
        if not isinstance(total_pages, int) or total_pages < 0:
            raise ProofError("posture response meta.totalPages must be a non-negative integer")
        if response_page != page:
            raise ProofError(f"posture response returned page {response_page!r}, expected {page}")
        if expected_total is None:
            expected_total = total
        elif total != expected_total:
            raise ProofError("posture total changed while proof was being collected; retry")

        postures.extend(data)
        response_hashes.append(response_hash)
        if page >= total_pages:
            break
        page += 1
        if page > 1000:
            raise ProofError("posture pagination exceeded 1000 pages")

    if expected_total is None:
        expected_total = 0
    if len(postures) != expected_total:
        raise ProofError(
            f"posture pagination returned {len(postures)} rows, expected {expected_total}"
        )
    return postures, response_hashes, expected_total


def evaluate_live_runtime(
    *,
    expected_version: str,
    observed_version: str,
    postures: list[dict[str, Any]],
    minimum_postures: int,
    minimum_successful_postures: int,
) -> tuple[dict[str, Any], list[str]]:
    failures: list[str] = []
    expected_normalized = _normalize_version(expected_version)
    observed_normalized = _normalize_version(observed_version)
    if not observed_normalized:
        failures.append("running version response was empty")
    elif observed_normalized != expected_normalized:
        failures.append(
            f"running version {observed_version!r} does not match expected {expected_version!r}"
        )

    state_counts: Counter[str] = Counter()
    successful_count = 0
    unknown_with_success: list[str] = []
    malformed_subjects: list[str] = []
    for index, posture in enumerate(postures):
        subject_id = posture.get("subjectResourceId")
        state = posture.get("state")
        if not isinstance(subject_id, str) or not subject_id.strip() or not isinstance(state, str):
            malformed_subjects.append(str(index))
            continue
        state_counts[state] += 1
        if posture.get("lastSuccessfulPointAt"):
            successful_count += 1
            if state == "unknown":
                unknown_with_success.append(subject_id)

    if malformed_subjects:
        failures.append(
            f"{len(malformed_subjects)} posture rows lacked a valid subjectResourceId or state"
        )
    if len(postures) < minimum_postures:
        failures.append(
            f"posture count {len(postures)} is below required minimum {minimum_postures}"
        )
    if successful_count < minimum_successful_postures:
        failures.append(
            "successful-posture count "
            f"{successful_count} is below required minimum {minimum_successful_postures}"
        )
    if unknown_with_success:
        failures.append(
            f"{len(unknown_with_success)} workloads with successful restore points remain unknown"
        )

    observed = {
        "version": observed_version,
        "postureTotal": len(postures),
        "stateCounts": dict(sorted(state_counts.items())),
        "successfulPostureCount": successful_count,
        "unknownWithSuccessfulPointCount": len(unknown_with_success),
        "unknownWithSuccessfulPointResourceIds": sorted(unknown_with_success),
    }
    return observed, failures


def collect_receipt(args: argparse.Namespace) -> dict[str, Any]:
    origin = normalize_origin(args.base_url)
    headers = build_headers(args.authorization_env, args.cookie_env)
    tls_verified = urllib.parse.urlsplit(origin).scheme == "https" and not args.insecure
    version_payload, version_hash = _read_json_response(
        f"{origin}/api/version",
        headers=headers,
        timeout_seconds=args.timeout_seconds,
        tls_verified=tls_verified,
    )
    observed_version = version_payload.get("version")
    if not isinstance(observed_version, str):
        raise ProofError("version response must contain a string version")
    postures, posture_hashes, posture_total = _collect_postures(
        origin,
        headers=headers,
        timeout_seconds=args.timeout_seconds,
        tls_verified=tls_verified,
    )
    observed, failures = evaluate_live_runtime(
        expected_version=args.expected_version,
        observed_version=observed_version,
        postures=postures,
        minimum_postures=args.minimum_postures,
        minimum_successful_postures=args.minimum_successful_postures,
    )
    is_source_build = version_payload.get("isSourceBuild") is True
    is_development = version_payload.get("isDevelopment") is True
    if is_source_build:
        failures.append("running target reports a source build, not a release artifact")
    if is_development:
        failures.append("running target reports a development build, not a release artifact")
    observed.update(
        {
            "build": version_payload.get("build", ""),
            "channel": version_payload.get("channel", ""),
            "deploymentType": version_payload.get("deploymentType", ""),
            "isSourceBuild": is_source_build,
            "isDevelopment": is_development,
            "versionResponseSha256": version_hash,
            "postureResponseSha256": posture_hashes,
        }
    )
    if posture_total != observed["postureTotal"]:
        raise ProofError("posture response total changed during evaluation")

    receipt = {
        "schemaVersion": SCHEMA_VERSION,
        "proofType": PROOF_TYPE,
        "assertion": ASSERTION,
        "result": "passed" if not failures else "failed",
        "collectedAt": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "target": {
            "origin": origin,
            "tlsVerified": tls_verified,
        },
        "expected": {
            "version": args.expected_version,
            "minimumPostures": args.minimum_postures,
            "minimumSuccessfulPostures": args.minimum_successful_postures,
        },
        "observed": observed,
        "failures": failures,
    }
    return seal_receipt(receipt)


def verify_receipt(
    receipt: dict[str, Any],
    *,
    expected_version: str,
    expected_origin: str,
    max_age_seconds: int,
    now: datetime | None = None,
) -> list[str]:
    errors: list[str] = []
    if receipt.get("schemaVersion") != SCHEMA_VERSION:
        errors.append(f"schemaVersion must be {SCHEMA_VERSION}")
    if receipt.get("proofType") != PROOF_TYPE:
        errors.append(f"proofType must be {PROOF_TYPE!r}")
    if receipt.get("assertion") != ASSERTION:
        errors.append(f"assertion must be {ASSERTION!r}")
    if receipt.get("receiptSha256") != _receipt_digest(receipt):
        errors.append("receiptSha256 does not match receipt content")
    if receipt.get("result") != "passed":
        errors.append("receipt result is not passed")
    if receipt.get("failures") != []:
        errors.append("receipt contains assertion failures")

    expected = receipt.get("expected")
    observed = receipt.get("observed")
    target = receipt.get("target")
    if not isinstance(expected, dict) or not isinstance(observed, dict):
        errors.append("receipt expected and observed fields must be objects")
    else:
        receipt_expected = expected.get("version")
        observed_version = observed.get("version")
        if _normalize_version(str(receipt_expected or "")) != _normalize_version(expected_version):
            errors.append("receipt expected version does not match verifier expectation")
        if _normalize_version(str(observed_version or "")) != _normalize_version(expected_version):
            errors.append("receipt observed version does not match verifier expectation")
        if observed.get("unknownWithSuccessfulPointCount") != 0:
            errors.append("receipt reports unknown postures with successful restore points")
        if observed.get("isSourceBuild") is not False:
            errors.append("receipt does not prove a packaged release artifact")
        if observed.get("isDevelopment") is not False:
            errors.append("receipt reports a development runtime")
        minimum_postures = expected.get("minimumPostures")
        minimum_successful = expected.get("minimumSuccessfulPostures")
        posture_total = observed.get("postureTotal")
        successful_total = observed.get("successfulPostureCount")
        state_counts = observed.get("stateCounts")
        unknown_ids = observed.get("unknownWithSuccessfulPointResourceIds")
        if not isinstance(minimum_postures, int) or minimum_postures < 1:
            errors.append("receipt minimumPostures must be a positive integer")
        elif not isinstance(posture_total, int) or posture_total < minimum_postures:
            errors.append("receipt posture total is below its required minimum")
        if not isinstance(minimum_successful, int) or minimum_successful < 1:
            errors.append("receipt minimumSuccessfulPostures must be a positive integer")
        elif not isinstance(successful_total, int) or successful_total < minimum_successful:
            errors.append("receipt successful-posture total is below its required minimum")
        if not isinstance(state_counts, dict) or not all(
            isinstance(state, str) and isinstance(count, int) and count >= 0
            for state, count in state_counts.items()
        ):
            errors.append("receipt stateCounts must contain non-negative integer counts")
        elif isinstance(posture_total, int) and sum(state_counts.values()) != posture_total:
            errors.append("receipt stateCounts do not sum to postureTotal")
        if unknown_ids != []:
            errors.append("receipt contains unknown successful-point resource IDs")
        version_hash = observed.get("versionResponseSha256")
        posture_hashes = observed.get("postureResponseSha256")
        sha256_pattern = re.compile(r"^[0-9a-f]{64}$")
        if not isinstance(version_hash, str) or not sha256_pattern.fullmatch(version_hash):
            errors.append("receipt version response SHA-256 is invalid")
        if not isinstance(posture_hashes, list) or not posture_hashes or not all(
            isinstance(value, str) and sha256_pattern.fullmatch(value)
            for value in posture_hashes
        ):
            errors.append("receipt posture response SHA-256 list is invalid")

    try:
        normalized_expected_origin = normalize_origin(expected_origin)
    except ProofError as exc:
        errors.append(str(exc))
        normalized_expected_origin = ""
    if not isinstance(target, dict):
        errors.append("receipt target must be an object")
    else:
        if target.get("origin") != normalized_expected_origin:
            errors.append("receipt target origin does not match verifier expectation")
        if target.get("tlsVerified") is not True:
            errors.append("receipt did not verify target TLS")

    collected_at = receipt.get("collectedAt")
    try:
        collected = datetime.fromisoformat(str(collected_at).replace("Z", "+00:00"))
        if collected.tzinfo is None:
            raise ValueError("timestamp has no timezone")
        reference_now = now or datetime.now(timezone.utc)
        age_seconds = (reference_now - collected).total_seconds()
        if age_seconds < -300:
            errors.append("receipt timestamp is more than five minutes in the future")
        if age_seconds > max_age_seconds:
            errors.append(
                f"receipt age {int(age_seconds)}s exceeds maximum {max_age_seconds}s"
            )
    except (TypeError, ValueError):
        errors.append("receipt collectedAt is not a valid timezone-aware timestamp")
    return errors


def _write_json_atomic(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="w",
        encoding="utf-8",
        dir=path.parent,
        prefix=f".{path.name}.",
        suffix=".tmp",
        delete=False,
    ) as handle:
        temporary = Path(handle.name)
        json.dump(payload, handle, indent=2, sort_keys=True)
        handle.write("\n")
    os.replace(temporary, path)


def _load_receipt(path: Path) -> dict[str, Any]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise ProofError(f"could not read receipt: {exc}") from exc
    except json.JSONDecodeError as exc:
        raise ProofError("receipt is not valid JSON") from exc
    if not isinstance(payload, dict):
        raise ProofError("receipt must be a JSON object")
    return payload


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Collect or verify live hardware proof. A published or installed release is not "
            "live-verified until this command produces a passing receipt."
        )
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    collect = subparsers.add_parser("collect", help="query a running Pulse target")
    collect.add_argument("--base-url", required=True, help="target origin, without a path")
    collect.add_argument("--expected-version", required=True)
    collect.add_argument("--output", required=True, type=Path)
    collect.add_argument("--authorization-env", default="")
    collect.add_argument("--cookie-env", default="")
    collect.add_argument("--minimum-postures", type=int, default=1)
    collect.add_argument("--minimum-successful-postures", type=int, default=1)
    collect.add_argument("--timeout-seconds", type=float, default=DEFAULT_TIMEOUT_SECONDS)
    collect.add_argument(
        "--insecure",
        action="store_true",
        help="disable TLS verification; receipts collected this way cannot be verified",
    )

    verify = subparsers.add_parser("verify", help="validate a saved proof receipt")
    verify.add_argument("--receipt", required=True, type=Path)
    verify.add_argument("--expected-version", required=True)
    verify.add_argument("--expected-origin", required=True)
    verify.add_argument("--max-age-seconds", required=True, type=int)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        if args.command == "collect":
            if args.minimum_postures < 1 or args.minimum_successful_postures < 1:
                raise ProofError("minimum posture counts must be positive")
            if args.timeout_seconds <= 0:
                raise ProofError("timeout must be positive")
            receipt = collect_receipt(args)
            _write_json_atomic(args.output, receipt)
            print(f"[{receipt['result'].upper()}] Live runtime proof: {args.output}")
            for failure in receipt["failures"]:
                print(f"ERROR: {failure}", file=sys.stderr)
            return 0 if receipt["result"] == "passed" else 1

        if args.max_age_seconds <= 0:
            raise ProofError("max age must be positive")
        receipt = _load_receipt(args.receipt)
        errors = verify_receipt(
            receipt,
            expected_version=args.expected_version,
            expected_origin=args.expected_origin,
            max_age_seconds=args.max_age_seconds,
        )
        if errors:
            for error in errors:
                print(f"ERROR: {error}", file=sys.stderr)
            return 1
        print(f"[PASSED] Verified live runtime proof: {args.receipt}")
        return 0
    except ProofError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
