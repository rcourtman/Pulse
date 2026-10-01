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

import format_staged_frontend


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


def prettier_format(
    prettier: Path,
    path: str,
    content: bytes,
    *,
    repo_root: Path = REPO_ROOT,
) -> bytes | None:
    # --stdin-filepath drives parser selection and config resolution, so this
    # matches what `prettier --write <path>` would produce.
    try:
        result = subprocess.run(
            [str(prettier), "--stdin-filepath", str(repo_root / path)],
            cwd=repo_root / "frontend-modern",
            check=True,
            capture_output=True,
            input=content,
        )
    except (subprocess.CalledProcessError, OSError):
        return None
    return result.stdout


def formatting_only_paths(
    paths: Sequence[str],
    *,
    commit: str | None,
    base: str | None = None,
    repo_root: Path = REPO_ROOT,
) -> set[str]:
    """Paths whose new content is exactly prettier's output for the old content.

    A `make format` sweep re-lays-out already-committed files without changing
    a single token, so it cannot change what renders. Demanding fresh browser
    proof for that would mean recording a receipt describing an interaction
    matrix nobody exercised, for a diff with no visual delta.

    This fails closed: an added or deleted file, an unreadable blob, a prettier
    that will not run, or any output that is not byte-identical all fall
    through and still require the receipt. `base` compares against an
    explicit range base instead of the commit's parent.
    """
    prettier = format_staged_frontend.prettier_bin()
    if prettier is None:
        return set()

    base_revision = base or (f"{commit}^" if commit else "HEAD")
    formatting_only: set[str] = set()
    for path in paths:
        new_object = f"{commit}:{path}" if commit else f":{path}"
        new_content = git_blob_bytes(new_object, repo_root=repo_root)
        old_content = git_blob_bytes(f"{base_revision}:{path}", repo_root=repo_root)
        if new_content is None or old_content is None or new_content == old_content:
            continue
        if prettier_format(prettier, path, old_content, repo_root=repo_root) == new_content:
            formatting_only.add(path)
    return formatting_only


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


def receipt_commits_in_range(
    base: str,
    head: str,
    *,
    repo_root: Path = REPO_ROOT,
) -> list[str]:
    """Non-merge commits in base..head that author a receipt version.

    Merge commits are excluded: a merge resolution of the shared receipt keeps
    or combines other commits' records without a browser run of its own, so it
    can never be evidence. --full-history keeps receipt commits on merged side
    branches that default history simplification would prune.
    """
    output = run_git(
        [
            "rev-list",
            "--reverse",
            "--no-merges",
            "--full-history",
            f"{base}..{head}",
            "--",
            RECEIPT_PATH,
        ],
        repo_root=repo_root,
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


def range_receipt_coverage(
    base: str,
    head: str,
    *,
    repo_root: Path = REPO_ROOT,
) -> tuple[dict[str, set[str]], list[str]]:
    """Content digests verified by valid receipts authored inside base..head.

    Each receipt is validated exactly as the per-commit guard validates it at
    the commit that recorded it: bound to that commit's own parent, and its
    changed_paths and content_sha256 matching that commit's tree. A receipt
    that fails contributes no coverage and is reported.
    """
    covered: dict[str, set[str]] = {}
    diagnostics: list[str] = []
    for commit in receipt_commits_in_range(base, head, repo_root=repo_root):
        try:
            payload = json.loads(load_receipt_text(commit=commit, repo_root=repo_root))
            commit_base = expected_base_sha(commit=commit, repo_root=repo_root)
        except (json.JSONDecodeError, subprocess.CalledProcessError) as exc:
            diagnostics.append(f"{commit[:12]}: unable to load receipt: {exc}")
            continue
        listed = payload.get("changed_paths") if isinstance(payload, dict) else None
        listed_paths = (
            [path for path in listed if isinstance(path, str) and path]
            if isinstance(listed, list)
            else []
        )
        errors = validate_receipt(
            payload,
            changed_paths=listed_paths,
            expected_base=commit_base,
            expected_content_sha256=content_sha256(
                listed_paths, commit=commit, repo_root=repo_root
            ),
        )
        if errors:
            diagnostics.append(f"{commit[:12]}: receipt is invalid: " + "; ".join(errors))
            continue
        for path, digest in payload["content_sha256"].items():
            covered.setdefault(path, set()).add(digest)
    return covered, diagnostics


def range_coverage_errors(
    changed_paths: Sequence[str],
    *,
    base: str,
    head: str,
    repo_root: Path = REPO_ROOT,
) -> list[str]:
    """Changed frontend paths whose final content no receipt in range verified.

    Integration merges reviewed commits onto a main that keeps moving, so the
    range tip is usually a merge and the range carries several independently
    verified changes. Binding one tip receipt to the tip's parent and to the
    whole cumulative delta is invalid by construction there. What must hold is
    that every user-visible frontend file ships with content a real browser
    pass verified: its digest at head must equal the digest recorded by a valid
    receipt from a non-merge commit in the range.
    """
    covered, diagnostics = range_receipt_coverage(base, head, repo_root=repo_root)
    final = content_sha256(changed_paths, commit=head, repo_root=repo_root)
    uncovered = [path for path in changed_paths if final[path] not in covered.get(path, set())]
    if not uncovered:
        return []
    errors = [
        f"{path} final content {final[path][:12]} is not verified by any valid receipt in the range"
        for path in uncovered
    ]
    errors.extend(diagnostics)
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
        "--base",
        help=(
            "Validate the range base..--commit: every changed user-visible frontend "
            "file's final content must match a valid receipt recorded by a non-merge "
            "commit in the range, bound to that commit's own parent."
        ),
    )
    parser.add_argument(
        "--print-template",
        action="store_true",
        help="Print a non-passing receipt template for the current staged frontend paths.",
    )
    args = parser.parse_args(argv)
    if args.base and not args.commit:
        parser.error("--base requires --commit")
    return args


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(argv)
    if args.files_from_stdin:
        paths = stdin_files(sys.stdin)
    elif args.base:
        paths = stdin_files(
            run_git(
                ["diff", "--name-only", "--diff-filter=ACMRD", args.base, args.commit],
                repo_root=REPO_ROOT,
            ).splitlines()
        )
    else:
        paths = staged_files()

    if args.print_template:
        print(json.dumps(build_template(paths), indent=2))
        return 0

    changed_frontend_paths = frontend_runtime_paths(paths)
    if changed_frontend_paths:
        reformatted = formatting_only_paths(
            changed_frontend_paths, commit=args.commit, base=args.base, repo_root=REPO_ROOT
        )
        if reformatted:
            print(
                f"Browser verification guard: {len(reformatted)} path(s) are prettier-only "
                "reformats of their committed content, with no visual delta to verify."
            )
            changed_frontend_paths = [
                path for path in changed_frontend_paths if path not in reformatted
            ]

    if not changed_frontend_paths:
        print("Browser verification guard skipped (no user-visible frontend source changes).")
        return 0

    if args.base:
        try:
            errors = range_coverage_errors(
                changed_frontend_paths,
                base=args.base,
                head=args.commit,
                repo_root=REPO_ROOT,
            )
        except subprocess.CalledProcessError as exc:
            print(f"BLOCKED: unable to evaluate browser verification range: {exc}", file=sys.stderr)
            return 1
        if errors:
            print(
                f"BLOCKED: browser verification does not cover {args.base[:12]}..{args.commit[:12]}:",
                file=sys.stderr,
            )
            for error in errors:
                print(f"  - {error}", file=sys.stderr)
            print(
                "Content changed after its browser pass (a correction, a conflict resolution, "
                "or two changes to one file) needs a fresh receipt for the final content, "
                "committed in its own non-merge commit and bound to that commit's parent.",
                file=sys.stderr,
            )
            return 1
        print(
            "Browser verification guard passed "
            f"({len(changed_frontend_paths)} frontend source file(s) across "
            f"{args.base[:12]}..{args.commit[:12]} covered by in-range receipts)."
        )
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
