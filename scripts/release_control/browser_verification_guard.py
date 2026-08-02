#!/usr/bin/env python3
"""Require fresh browser proof for staged user-visible frontend changes."""

from __future__ import annotations

import argparse
from datetime import datetime
import hashlib
import json
from pathlib import Path, PurePosixPath
import subprocess
import sys
from typing import Iterable, Sequence


REPO_ROOT = Path(__file__).resolve().parents[2]
RECEIPT_PATH = "frontend-modern/browser-verification.json"
FRONTEND_SOURCE_PREFIX = "frontend-modern/src/"
FRONTEND_ENTRY_FILES = {"frontend-modern/index.html"}
FRONTEND_SOURCE_SUFFIXES = {".css", ".scss", ".ts", ".tsx"}


def run_git(args: Sequence[str], *, repo_root: Path = REPO_ROOT) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=repo_root,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def git_blob_bytes(object_name: str, *, repo_root: Path = REPO_ROOT) -> bytes | None:
    result = subprocess.run(
        ["git", "show", object_name],
        cwd=repo_root,
        check=False,
        capture_output=True,
    )
    return result.stdout if result.returncode == 0 else None


def staged_files(*, repo_root: Path = REPO_ROOT) -> list[str]:
    output = run_git(
        ["diff", "--cached", "--name-only", "--diff-filter=ACMRD"],
        repo_root=repo_root,
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


def stdin_files(lines: Iterable[str]) -> list[str]:
    return [line.strip() for line in lines if line.strip()]


def is_frontend_test_or_fixture(path: str) -> bool:
    pure_path = PurePosixPath(path)
    name = pure_path.name
    return (
        "__tests__" in pure_path.parts
        or "__fixtures__" in pure_path.parts
        or ".test." in name
        or ".spec." in name
        or ".stories." in name
    )


def is_user_visible_frontend_source(path: str) -> bool:
    if path in FRONTEND_ENTRY_FILES:
        return True
    if not path.startswith(FRONTEND_SOURCE_PREFIX):
        return False
    if is_frontend_test_or_fixture(path):
        return False
    return PurePosixPath(path).suffix in FRONTEND_SOURCE_SUFFIXES


def frontend_runtime_paths(paths: Iterable[str]) -> list[str]:
    return sorted({path for path in paths if is_user_visible_frontend_source(path)})


def load_receipt_text(
    *,
    commit: str | None,
    repo_root: Path = REPO_ROOT,
) -> str:
    object_name = f"{commit}:{RECEIPT_PATH}" if commit else f":{RECEIPT_PATH}"
    return run_git(["show", object_name], repo_root=repo_root)


def expected_base_sha(*, commit: str | None, repo_root: Path = REPO_ROOT) -> str:
    revision = f"{commit}^" if commit else "HEAD"
    return run_git(["rev-parse", revision], repo_root=repo_root)


def content_sha256(
    paths: Sequence[str],
    *,
    commit: str | None,
    repo_root: Path = REPO_ROOT,
) -> dict[str, str]:
    digests: dict[str, str] = {}
    for path in paths:
        object_name = f"{commit}:{path}" if commit else f":{path}"
        content = git_blob_bytes(object_name, repo_root=repo_root)
        digests[path] = "deleted" if content is None else hashlib.sha256(content).hexdigest()
    return digests


def validate_receipt(
    payload: object,
    *,
    changed_paths: Sequence[str],
    expected_base: str,
    expected_content_sha256: dict[str, str],
) -> list[str]:
    errors: list[str] = []
    if not isinstance(payload, dict):
        return ["receipt root must be a JSON object"]

    if payload.get("version") != 1:
        errors.append("version must be 1")
    if payload.get("result") != "passed":
        errors.append('result must be "passed"')
    if payload.get("base_sha") != expected_base:
        errors.append(f"base_sha must match the verified parent {expected_base}")

    receipt_paths = payload.get("changed_paths")
    if not isinstance(receipt_paths, list) or not all(
        isinstance(path, str) and path for path in receipt_paths
    ):
        errors.append("changed_paths must be a non-empty string array")
    elif sorted(set(receipt_paths)) != sorted(set(changed_paths)):
        errors.append("changed_paths must exactly match staged user-visible frontend source files")

    if payload.get("content_sha256") != expected_content_sha256:
        errors.append("content_sha256 must exactly match the final staged frontend source content")

    routes = payload.get("routes")
    if not isinstance(routes, list) or not routes or not all(
        isinstance(route, str) and route.strip() for route in routes
    ):
        errors.append("routes must be a non-empty string array")

    states = payload.get("states")
    if not isinstance(states, list) or not states or not all(
        isinstance(state, str) and state.strip() for state in states
    ):
        errors.append("states must be a non-empty string array")

    interactions = payload.get("interactions")
    if not isinstance(interactions, list) or not interactions or not all(
        isinstance(interaction, str) and interaction.strip() for interaction in interactions
    ):
        errors.append("interactions must be a non-empty string array")

    viewports = payload.get("viewports")
    valid_viewports: list[dict] = []
    if isinstance(viewports, list):
        valid_viewports = [
            viewport
            for viewport in viewports
            if isinstance(viewport, dict)
            and isinstance(viewport.get("width"), int)
            and viewport["width"] > 0
            and isinstance(viewport.get("height"), int)
            and viewport["height"] > 0
        ]
    if len(valid_viewports) != len(viewports or []) or not valid_viewports:
        errors.append("viewports must contain valid positive integer width/height objects")
    else:
        if not any(viewport["width"] >= 1024 for viewport in valid_viewports):
            errors.append("viewports must include a desktop width of at least 1024 pixels")
        if not any(viewport["width"] <= 768 for viewport in valid_viewports):
            errors.append("viewports must include a narrow width of at most 768 pixels")

    verified_at = payload.get("verified_at")
    if not isinstance(verified_at, str) or not verified_at.endswith("Z"):
        errors.append("verified_at must be an ISO-8601 UTC timestamp ending in Z")
    else:
        try:
            datetime.fromisoformat(verified_at.removesuffix("Z") + "+00:00")
        except ValueError:
            errors.append("verified_at must be a valid ISO-8601 UTC timestamp")

    return errors


def build_template(paths: Sequence[str], *, repo_root: Path = REPO_ROOT) -> dict:
    changed_paths = frontend_runtime_paths(paths)
    return {
        "version": 1,
        "base_sha": expected_base_sha(commit=None, repo_root=repo_root),
        "verified_at": "YYYY-MM-DDTHH:MM:SSZ",
        "result": "replace-with-passed-after-verification",
        "changed_paths": changed_paths,
        "content_sha256": content_sha256(changed_paths, commit=None, repo_root=repo_root),
        "routes": ["/replace-with-verified-route"],
        "viewports": [
            {"width": 1280, "height": 800},
            {"width": 390, "height": 844},
        ],
        "states": ["replace with every state inspected"],
        "interactions": ["replace with every interaction exercised"],
    }


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--files-from-stdin",
        action="store_true",
        help="Read the changed path list from stdin instead of the staged index.",
    )
    parser.add_argument(
        "--commit",
        help="Validate the receipt stored in this commit against that commit's parent.",
    )
    parser.add_argument(
        "--print-template",
        action="store_true",
        help="Print a non-passing receipt template for the current staged frontend paths.",
    )
    return parser.parse_args(argv)


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(argv)
    paths = stdin_files(sys.stdin) if args.files_from_stdin else staged_files()

    if args.print_template:
        print(json.dumps(build_template(paths), indent=2))
        return 0

    changed_frontend_paths = frontend_runtime_paths(paths)
    if not changed_frontend_paths:
        print("Browser verification guard skipped (no user-visible frontend source changes).")
        return 0

    if RECEIPT_PATH not in paths:
        print(
            f"BLOCKED: {RECEIPT_PATH} must be updated and staged for user-visible frontend changes.",
            file=sys.stderr,
        )
        print(
            "Run the current build in a browser, exercise the complete interaction matrix, "
            "then record routes, desktop and narrow viewports, states, and interactions.",
            file=sys.stderr,
        )
        return 1

    try:
        payload = json.loads(load_receipt_text(commit=args.commit))
        expected_base = expected_base_sha(commit=args.commit)
    except (json.JSONDecodeError, subprocess.CalledProcessError) as exc:
        print(f"BLOCKED: unable to load browser verification receipt: {exc}", file=sys.stderr)
        return 1

    errors = validate_receipt(
        payload,
        changed_paths=changed_frontend_paths,
        expected_base=expected_base,
        expected_content_sha256=content_sha256(
            changed_frontend_paths,
            commit=args.commit,
        ),
    )
    if errors:
        print("BLOCKED: browser verification receipt is invalid:", file=sys.stderr)
        for error in errors:
            print(f"  - {error}", file=sys.stderr)
        return 1

    print(
        "Browser verification guard passed "
        f"({len(changed_frontend_paths)} frontend source file(s), "
        f"{len(payload['routes'])} route(s), {len(payload['viewports'])} viewport(s))."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
