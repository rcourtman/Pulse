#!/usr/bin/env python3
"""Require fresh browser proof for staged user-visible frontend changes.

Every commit that changes user-visible frontend source records its browser
receipt in a file of its own under frontend-modern/browser-verification/, named
by the fingerprint of the content it verified. Until 2026-10-03 every such
commit rewrote one shared file, so any two open frontend pull requests
conflicted the moment either merged, and each conflict cost a rebuild and a
fresh browser pass. Receipts that are only ever added cannot collide, and git
merges them without help.
"""

from __future__ import annotations

import argparse
from datetime import datetime
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
from typing import Iterable, Sequence

import format_staged_frontend


REPO_ROOT = Path(__file__).resolve().parents[2]
RECEIPT_DIR = "frontend-modern/browser-verification"
# The single shared receipt every frontend commit rewrote before per-commit
# receipts. It is retired: nothing may write it again. Range mode still reads
# it from commits that modified it, because those commits were verified
# honestly against a tree that carried it and are still merging.
LEGACY_RECEIPT_PATH = "frontend-modern/browser-verification.json"
RECEIPT_FINGERPRINT_LENGTH = 16
DEFAULT_LANDED_REF = "origin/main"
GUARD_COMMAND = "python3 scripts/release_control/browser_verification_guard.py"
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


def blocked(headline: str, details: Iterable[str] = (), advice: Iterable[str] = ()) -> int:
    print(f"BLOCKED: {headline}", file=sys.stderr)
    for detail in details:
        print(f"  - {detail}", file=sys.stderr)
    for line in advice:
        print(line, file=sys.stderr)
    return 1


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


def is_receipt_path(path: str) -> bool:
    pure_path = PurePosixPath(path)
    return str(pure_path.parent) == RECEIPT_DIR and pure_path.suffix == ".json"


def receipt_fingerprint(digests: dict[str, str]) -> str:
    canonical = json.dumps(digests, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()[:RECEIPT_FINGERPRINT_LENGTH]


def receipt_path_for(digests: dict[str, str]) -> str:
    """Where the receipt verifying exactly this content must be stored.

    The name is derived from the verified content rather than chosen, so two
    changes can only share a receipt file when they verify identical bytes,
    and the guard knows which file a commit owes without searching. It does
    not include the parent, so rebuilding a commit on a new parent edits
    base_sha in place instead of renaming the file.
    """
    return f"{RECEIPT_DIR}/{receipt_fingerprint(digests)}.json"


def load_receipt_text(
    path: str,
    *,
    commit: str | None,
    repo_root: Path = REPO_ROOT,
) -> str:
    object_name = f"{commit}:{path}" if commit else f":{path}"
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
    expected_base: str | None,
    expected_content_sha256: dict[str, str],
    receipt_path: str | None = None,
) -> list[str]:
    """Reasons the receipt is not evidence for exactly this content.

    expected_base None validates a receipt staged without frontend source
    changes, where the commit's parent is not knowable from the index alone
    (a follow-up proof commit names HEAD, an amended one names HEAD's parent);
    range mode binds those to their real parent.
    """
    errors: list[str] = []
    if not isinstance(payload, dict):
        return ["receipt root must be a JSON object"]

    if payload.get("version") != 1:
        errors.append("version must be 1")
    if payload.get("result") != "passed":
        errors.append('result must be "passed"')
    if expected_base is None:
        base_sha = payload.get("base_sha")
        if not isinstance(base_sha, str) or not re.fullmatch(r"[0-9a-f]{40}", base_sha):
            errors.append("base_sha must be the full commit id of the verified parent")
    elif payload.get("base_sha") != expected_base:
        errors.append(f"base_sha must match the verified parent {expected_base}")

    digests = payload.get("content_sha256")
    if (
        receipt_path is not None
        and is_receipt_path(receipt_path)
        and isinstance(digests, dict)
        and all(isinstance(key, str) and isinstance(value, str) for key, value in digests.items())
        and receipt_path != receipt_path_for(digests)
    ):
        errors.append(
            f"receipt must be stored at {receipt_path_for(digests)}, "
            "the fingerprint of its content_sha256"
        )

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


def blob_ids(
    revision: str | None,
    paths: Sequence[str],
    *,
    repo_root: Path = REPO_ROOT,
) -> dict[str, str]:
    """Object ids under the given paths in a commit, or in the index when revision is None.

    A path that does not exist there is simply absent from the result.
    """
    ids: dict[str, str] = {}
    for start in range(0, len(paths), 200):
        chunk = list(paths[start : start + 200])
        if revision is None:
            command = ["ls-files", "--stage", "-z", "--", *chunk]
        else:
            command = ["ls-tree", "-r", "-z", revision, "--", *chunk]
        output = run_git(["--literal-pathspecs", *command], repo_root=repo_root)
        for entry in output.split("\0"):
            meta, separator, path = entry.partition("\t")
            if not separator:
                continue
            fields = meta.split()
            ids[path] = fields[1] if revision is None else fields[2]
    return ids


def merge_parents(*, commit: str | None, repo_root: Path = REPO_ROOT) -> list[str]:
    """Parents beyond the first: the merge being concluded, or a merge commit's own."""
    if commit:
        return run_git(
            ["rev-list", "--parents", "-n", "1", commit], repo_root=repo_root
        ).split()[2:]
    merge_head = Path(run_git(["rev-parse", "--git-path", "MERGE_HEAD"], repo_root=repo_root))
    if not merge_head.is_absolute():
        merge_head = repo_root / merge_head
    if not merge_head.exists():
        return []
    return merge_head.read_text(encoding="utf-8").split()


def merged_in_paths(
    paths: Sequence[str],
    *,
    commit: str | None,
    repo_root: Path = REPO_ROOT,
) -> set[str]:
    """Paths whose new state is exactly what a merged parent already carries.

    Concluding a merge stages everything the other parent brought, so a plain
    staged-file reading mistakes the other side's frontend work and receipts
    for this commit's own. Demanding a receipt for them asked the author to
    vouch for files they never ran, which made a merge unable to repair a
    branch that fell behind main. Content a parent already holds was verified
    in that parent's history and is not this commit's change.
    """
    parents = merge_parents(commit=commit, repo_root=repo_root)
    if not parents or not paths:
        return set()
    new = blob_ids(commit, paths, repo_root=repo_root)
    inherited: set[str] = set()
    for parent in parents:
        theirs = blob_ids(parent, paths, repo_root=repo_root)
        inherited.update(path for path in paths if new.get(path) == theirs.get(path))
    return inherited


def parse_receipt_changes(name_status: str) -> dict[str, str]:
    changes: dict[str, str] = {}
    for line in name_status.splitlines():
        status, _, path = line.partition("\t")
        if path == LEGACY_RECEIPT_PATH or is_receipt_path(path):
            changes[path] = status[:1]
    return changes


def receipt_changes(*, commit: str | None, repo_root: Path = REPO_ROOT) -> dict[str, str]:
    """Receipt files a commit, or the staged index, adds (A), modifies (M) or deletes (D).

    Read from git rather than from the caller's path list, and with rename
    detection off: git reports a deleted receipt and a similar new one as a
    single rename, which would hide the deletion.
    """
    if commit:
        command = ["diff", "--no-renames", "--name-status", f"{commit}^", commit]
    else:
        command = ["diff", "--cached", "--no-renames", "--name-status"]
    return parse_receipt_changes(
        run_git([*command, "--", RECEIPT_DIR, LEGACY_RECEIPT_PATH], repo_root=repo_root)
    )


def receipt_commits_in_range(
    base: str,
    head: str,
    *,
    repo_root: Path = REPO_ROOT,
) -> list[str]:
    """Non-merge commits in base..head that touch a receipt.

    Merge commits are excluded: a merge resolution keeps or combines other
    commits' records without a browser run of its own, so it can never be
    evidence. --full-history keeps receipt commits on merged side branches
    that default history simplification would prune.
    """
    output = run_git(
        [
            "rev-list",
            "--reverse",
            "--no-merges",
            "--full-history",
            f"{base}..{head}",
            "--",
            RECEIPT_DIR,
            LEGACY_RECEIPT_PATH,
        ],
        repo_root=repo_root,
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


PRUNE_SEPARATELY = (
    "Git reads a deleted receipt beside a newly added one as a rename, and that "
    "rename conflicts with any other change that deletes the same receipt. Prune "
    f"landed receipts in a change that records none: {GUARD_COMMAND} --prune"
)


RETIRED_RECEIPT = (
    f"{LEGACY_RECEIPT_PATH} is retired and must not exist. Remove it with git rm; a "
    "receipt it held stays valid in the commit that recorded it. Each commit now "
    f"records its receipt in its own file under {RECEIPT_DIR}/, so concurrent "
    "frontend changes no longer conflict."
)


def range_receipt_coverage(
    base: str,
    head: str,
    *,
    repo_root: Path = REPO_ROOT,
) -> tuple[dict[str, dict[str, list[str]]], list[str], list[str]]:
    """Content digests verified by valid receipts authored inside base..head.

    Each receipt is validated exactly as the per-commit guard validates it at
    the commit that recorded it: bound to that commit's own parent, and its
    changed_paths and content_sha256 matching that commit's tree. A receipt
    that fails contributes no coverage and is reported as a diagnostic.

    Violations are returned separately because they block the range even when
    coverage is complete: a tip that still carries the retired shared receipt
    (recreated, or kept while resolving a merge with the main that deleted
    it), or a commit that both records and deletes receipts.
    """
    # Keep provenance beside each admitted digest so a failed composition
    # names what actually passed, without rereading or trusting invalid proof.
    covered: dict[str, dict[str, list[str]]] = {}
    diagnostics: list[str] = []
    violations: list[str] = []
    if LEGACY_RECEIPT_PATH in blob_ids(head, [LEGACY_RECEIPT_PATH], repo_root=repo_root):
        violations.append(RETIRED_RECEIPT)
    for commit in receipt_commits_in_range(base, head, repo_root=repo_root):
        try:
            changes = receipt_changes(commit=commit, repo_root=repo_root)
            commit_base = expected_base_sha(commit=commit, repo_root=repo_root)
        except subprocess.CalledProcessError as exc:
            diagnostics.append(f"{commit[:12]}: unable to read receipt changes: {exc}")
            continue
        recorded = sorted(
            path for path, status in changes.items()
            if status != "D" and path != LEGACY_RECEIPT_PATH
        )
        legacy_status = changes.get(LEGACY_RECEIPT_PATH)
        if legacy_status == "M":
            # The commit's own parent still carried the shared receipt, so it
            # was verified before per-commit receipts and is honest evidence.
            recorded.append(LEGACY_RECEIPT_PATH)
        elif legacy_status == "A":
            diagnostics.append(
                f"{commit[:12]}: recreates the retired {LEGACY_RECEIPT_PATH}, which is not "
                f"evidence; record the receipt under {RECEIPT_DIR}/ instead"
            )
        if recorded and any(
            status == "D" and path != LEGACY_RECEIPT_PATH for path, status in changes.items()
        ):
            violations.append(
                f"{commit[:12]}: records a receipt and deletes another. " + PRUNE_SEPARATELY
            )
        for receipt_path in recorded:
            label = f"{commit[:12]} {PurePosixPath(receipt_path).name}"
            try:
                payload = json.loads(
                    load_receipt_text(receipt_path, commit=commit, repo_root=repo_root)
                )
            except (json.JSONDecodeError, subprocess.CalledProcessError) as exc:
                diagnostics.append(f"{label}: unable to load receipt: {exc}")
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
                receipt_path=receipt_path,
            )
            if errors:
                diagnostics.append(f"{label}: receipt is invalid: " + "; ".join(errors))
                continue
            for path, digest in payload["content_sha256"].items():
                origins = covered.setdefault(path, {}).setdefault(digest, [])
                origins.append(f"{commit}:{receipt_path} (parent {commit_base})")
    return covered, diagnostics, violations


def range_coverage_errors(
    changed_paths: Sequence[str],
    *,
    base: str,
    head: str,
    repo_root: Path = REPO_ROOT,
) -> tuple[list[str], list[str]]:
    """Changed frontend paths whose final content no receipt in range verified.

    Returned beside the range's receipt-rule violations, which block on their
    own.

    Integration merges reviewed commits onto a main that keeps moving, so the
    range tip is usually a merge and the range carries several independently
    verified changes. Binding one tip receipt to the tip's parent and to the
    whole cumulative delta is invalid by construction there. What must hold is
    that every user-visible frontend file ships with content a real browser
    pass verified: its digest at head must equal the digest recorded by a valid
    receipt from a non-merge commit in the range.
    """
    covered, diagnostics, violations = range_receipt_coverage(base, head, repo_root=repo_root)
    final = content_sha256(changed_paths, commit=head, repo_root=repo_root)
    uncovered = [path for path in changed_paths if final[path] not in covered.get(path, {})]
    if not uncovered:
        return [], violations
    errors: list[str] = []
    for path in uncovered:
        errors.append(
            f"{path} final content {final[path]} is not verified by any valid receipt in the range"
        )
        prior = sorted(
            (digest, origin)
            for digest, origins in covered.get(path, {}).items()
            for origin in origins
        )
        if not prior:
            errors.append(f"{path}: no valid in-range receipt names this path")
            continue
        # A long-lived branch can contain many earlier browser passes. Bound
        # hints, not admission: every valid digest still contributes coverage.
        for digest, origin in prior[:3]:
            errors.append(f"{path}: different verified content {digest} at {origin}")
        if len(prior) > 3:
            errors.append(f"{path}: {len(prior) - 3} further valid receipt(s) omitted from hints")
    errors.extend(diagnostics)
    return errors, violations


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


def emit_template(paths: Sequence[str], *, write: bool, repo_root: Path = REPO_ROOT) -> int:
    template = build_template(paths, repo_root=repo_root)
    if not template["changed_paths"]:
        if write:
            return blocked(
                "no user-visible frontend source is staged, so there is no receipt to write."
            )
        print(json.dumps(template, indent=2))
        return 0
    receipt_path = receipt_path_for(template["content_sha256"])
    text = json.dumps(template, indent=2) + "\n"
    if not write:
        print(text, end="")
        print(f"Save the completed receipt as {receipt_path} and stage it.", file=sys.stderr)
        return 0
    target = repo_root / receipt_path
    if target.exists():
        return blocked(f"{receipt_path} already exists; edit it rather than overwriting a receipt.")
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")
    print(receipt_path)
    return 0


def prune_landed_receipts(landed_ref: str, *, repo_root: Path = REPO_ROOT) -> int:
    """Delete receipts whose commits are already on landed_ref.

    A receipt is read from the commit that recorded it, so once that commit is
    on main the file in the tree is dead weight. Only a branch that records no
    receipt of its own may prune (see PRUNE_SEPARATELY).
    """
    try:
        merge_base = run_git(["merge-base", landed_ref, "HEAD"], repo_root=repo_root)
        branch_changes = parse_receipt_changes(
            run_git(
                ["diff", "--no-renames", "--name-status", merge_base, "HEAD", "--", RECEIPT_DIR],
                repo_root=repo_root,
            )
        )
        staged_changes = receipt_changes(commit=None, repo_root=repo_root)
        recorded = sorted(
            path
            for changes in (branch_changes, staged_changes)
            for path, status in changes.items()
            if status != "D" and path != LEGACY_RECEIPT_PATH
        )
        if recorded:
            return blocked(
                f"this branch records {len(recorded)} receipt(s) of its own "
                f"({', '.join(PurePosixPath(path).name for path in recorded)}).",
                advice=[PRUNE_SEPARATELY],
            )
        landed = blob_ids(landed_ref, [RECEIPT_DIR], repo_root=repo_root)
        prunable = sorted(
            path
            for path, blob in blob_ids(None, [RECEIPT_DIR], repo_root=repo_root).items()
            if is_receipt_path(path) and landed.get(path) == blob
        )
        for start in range(0, len(prunable), 200):
            run_git(
                ["--literal-pathspecs", "rm", "--quiet", "--", *prunable[start : start + 200]],
                repo_root=repo_root,
            )
    except subprocess.CalledProcessError as exc:
        return blocked(f"unable to prune browser verification receipts: {exc}")
    if not prunable:
        print(f"No receipts on {landed_ref} to prune.")
        return 0
    print(
        f"Pruned {len(prunable)} receipt(s) already on {landed_ref}. "
        "Commit the staged deletions; they need no receipt."
    )
    return 0


def check_commit(
    paths: Sequence[str],
    *,
    commit: str | None,
    repo_root: Path = REPO_ROOT,
) -> int:
    """Validate one commit, or the staged index, against its own receipt."""
    try:
        merging = bool(merge_parents(commit=commit, repo_root=repo_root))
        changes = receipt_changes(commit=commit, repo_root=repo_root)
        frontend_paths = frontend_runtime_paths(paths)
        inherited = merged_in_paths(
            sorted({*changes, *frontend_paths}), commit=commit, repo_root=repo_root
        )
        # The index is held to the tree it will become: a merge that keeps the
        # shared receipt changes nothing against HEAD yet would restore it on
        # main. A named commit is held only to what it writes itself.
        retired_receipt_written = (
            changes.get(LEGACY_RECEIPT_PATH, "D") != "D"
            if commit
            else LEGACY_RECEIPT_PATH
            in blob_ids(None, [LEGACY_RECEIPT_PATH], repo_root=repo_root)
        )
    except subprocess.CalledProcessError as exc:
        return blocked(f"unable to inspect browser verification state: {exc}")

    if retired_receipt_written:
        return blocked(
            RETIRED_RECEIPT,
            advice=[f"For a new receipt, run: {GUARD_COMMAND} --write-template"],
        )

    changes = {path: status for path, status in changes.items() if path not in inherited}
    frontend_paths = [path for path in frontend_paths if path not in inherited]
    recorded = sorted(
        path for path, status in changes.items() if status != "D" and path != LEGACY_RECEIPT_PATH
    )
    pruned = sorted(
        path for path, status in changes.items() if status == "D" and path != LEGACY_RECEIPT_PATH
    )

    if frontend_paths:
        reformatted = formatting_only_paths(frontend_paths, commit=commit, repo_root=repo_root)
        if reformatted:
            print(
                f"Browser verification guard: {len(reformatted)} path(s) are prettier-only "
                "reformats of their committed content, with no visual delta to verify."
            )
            frontend_paths = [path for path in frontend_paths if path not in reformatted]

    if merging:
        if recorded:
            return blocked(
                "a merge commit cannot record browser evidence.",
                details=recorded,
                advice=[
                    "A merge resolution has no browser run behind it, so range validation "
                    "ignores receipts a merge commit writes.",
                    "Take the merged parent's version of each file (git checkout MERGE_HEAD -- "
                    "<path>) or unstage it, conclude the merge, then record fresh evidence in a "
                    "follow-up commit.",
                ],
            )
        if frontend_paths:
            print(
                f"Browser verification guard: {len(frontend_paths)} frontend source file(s) "
                "carry merged content that neither parent holds:",
                file=sys.stderr,
            )
            for path in frontend_paths:
                print(f"  - {path}", file=sys.stderr)
            print(
                "The merge may be concluded, but that content has had no browser pass. Verify "
                "it, then record a receipt in a follow-up commit, or range validation will "
                "block the change:\n"
                f"  printf '%s\\n' {' '.join(frontend_paths)} | {GUARD_COMMAND} "
                "--files-from-stdin --write-template",
                file=sys.stderr,
            )
            return 0
        print(
            "Browser verification guard skipped (the merge changes no user-visible frontend "
            "source beyond what its parents already carry)."
        )
        return 0

    if recorded and pruned:
        return blocked(
            "a commit that records a receipt must not delete another.",
            details=pruned,
            advice=[PRUNE_SEPARATELY],
        )

    if not frontend_paths:
        if not recorded:
            print("Browser verification guard skipped (no user-visible frontend source changes).")
            return 0
        # A proof commit that follows its source change. Everything except the
        # parent binding is checkable here; range mode binds the parent.
        failures: list[str] = []
        for receipt_path in recorded:
            try:
                payload = json.loads(
                    load_receipt_text(receipt_path, commit=commit, repo_root=repo_root)
                )
            except (json.JSONDecodeError, subprocess.CalledProcessError) as exc:
                failures.append(f"{receipt_path}: unable to load receipt: {exc}")
                continue
            listed = payload.get("changed_paths") if isinstance(payload, dict) else None
            listed_paths = (
                [path for path in listed if isinstance(path, str) and path]
                if isinstance(listed, list)
                else []
            )
            failures.extend(
                f"{receipt_path}: {error}"
                for error in validate_receipt(
                    payload,
                    changed_paths=listed_paths,
                    expected_base=None,
                    expected_content_sha256=content_sha256(
                        listed_paths, commit=commit, repo_root=repo_root
                    ),
                    receipt_path=receipt_path,
                )
            )
        if failures:
            return blocked("browser verification receipt is invalid:", details=failures)
        print(
            f"Browser verification guard passed ({len(recorded)} receipt(s) recorded without "
            "frontend source changes; range validation binds each to its parent)."
        )
        return 0

    try:
        digests = content_sha256(frontend_paths, commit=commit, repo_root=repo_root)
        expected_base = expected_base_sha(commit=commit, repo_root=repo_root)
    except subprocess.CalledProcessError as exc:
        return blocked(f"unable to load browser verification receipt: {exc}")
    receipt_path = receipt_path_for(digests)
    strays = [path for path in recorded if path != receipt_path]
    if receipt_path not in recorded:
        advice = [
            "Run the current build in a browser, exercise the complete interaction matrix, "
            "then record routes, desktop and narrow viewports, states, and interactions.",
            f"Start from a skeleton at that path with: {GUARD_COMMAND} --write-template",
        ]
        if strays:
            advice.append(
                "A receipt's file name is the fingerprint of the content it verified. "
                f"{', '.join(strays)} verifies different content: the staged source changed "
                "after it was written, or it was named by hand."
            )
        return blocked(
            f"{receipt_path} must be added and staged for user-visible frontend changes.",
            advice=advice,
        )
    if strays:
        return blocked(
            f"a commit records exactly one receipt, {receipt_path}.",
            details=strays,
            advice=["Unstage the others; one receipt covers every frontend file the commit changes."],
        )

    try:
        payload = json.loads(load_receipt_text(receipt_path, commit=commit, repo_root=repo_root))
    except (json.JSONDecodeError, subprocess.CalledProcessError) as exc:
        return blocked(f"unable to load browser verification receipt: {exc}")

    errors = validate_receipt(
        payload,
        changed_paths=frontend_paths,
        expected_base=expected_base,
        expected_content_sha256=digests,
        receipt_path=receipt_path,
    )
    if errors:
        return blocked("browser verification receipt is invalid:", details=errors)

    print(
        "Browser verification guard passed "
        f"({len(frontend_paths)} frontend source file(s), "
        f"{len(payload['routes'])} route(s), {len(payload['viewports'])} viewport(s))."
    )
    return 0


def check_range(paths: Sequence[str], *, base: str, head: str, repo_root: Path = REPO_ROOT) -> int:
    """Validate base..head: violations block always, coverage when frontend source changed."""
    changed_frontend_paths = frontend_runtime_paths(paths)
    if changed_frontend_paths:
        reformatted = formatting_only_paths(
            changed_frontend_paths, commit=head, base=base, repo_root=repo_root
        )
        if reformatted:
            print(
                f"Browser verification guard: {len(reformatted)} path(s) are prettier-only "
                "reformats of their committed content, with no visual delta to verify."
            )
            changed_frontend_paths = [
                path for path in changed_frontend_paths if path not in reformatted
            ]
    try:
        errors, violations = range_coverage_errors(
            changed_frontend_paths, base=base, head=head, repo_root=repo_root
        )
    except subprocess.CalledProcessError as exc:
        return blocked(f"unable to evaluate browser verification range: {exc}")
    if errors:
        return blocked(
            f"browser verification does not cover {base[:12]}..{head[:12]}:",
            details=[*errors, *violations],
            advice=[
                "Content changed after its browser pass (a correction, a conflict resolution, "
                "or two changes to one file) needs a fresh receipt for the final content, "
                "committed in its own non-merge commit and bound to that commit's parent.",
                "Keep earlier receipts and reviewed history unchanged. Changing a receipt's "
                "hash or parent is not a browser pass for the final content.",
            ],
        )
    if violations:
        return blocked(
            f"browser verification receipts in {base[:12]}..{head[:12]} break the receipt rules:",
            details=violations,
        )
    if not changed_frontend_paths:
        print("Browser verification guard skipped (no user-visible frontend source changes).")
        return 0
    print(
        "Browser verification guard passed "
        f"({len(changed_frontend_paths)} frontend source file(s) across "
        f"{base[:12]}..{head[:12]} covered by in-range receipts)."
    )
    return 0


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--files-from-stdin",
        action="store_true",
        help="Read the changed path list from stdin instead of the staged index.",
    )
    parser.add_argument(
        "--commit",
        help="Validate the receipt recorded by this commit against that commit's parent.",
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
    parser.add_argument(
        "--write-template",
        action="store_true",
        help=(
            "Write that template to the receipt file the staged frontend content "
            "requires, and print its path."
        ),
    )
    parser.add_argument(
        "--prune",
        action="store_true",
        help=(
            "Delete and stage the removal of receipts whose commits are already on "
            "--landed-on. Run it on a branch that records no receipt of its own."
        ),
    )
    parser.add_argument(
        "--landed-on",
        default=DEFAULT_LANDED_REF,
        help=f"Ref that --prune treats as landed (default: {DEFAULT_LANDED_REF}).",
    )
    args = parser.parse_args(argv)
    if args.base and not args.commit:
        parser.error("--base requires --commit")
    return args


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(argv)
    if args.prune:
        return prune_landed_receipts(args.landed_on, repo_root=REPO_ROOT)

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
        paths = staged_files(repo_root=REPO_ROOT)

    if args.print_template or args.write_template:
        return emit_template(paths, write=args.write_template, repo_root=REPO_ROOT)

    if args.base:
        return check_range(paths, base=args.base, head=args.commit, repo_root=REPO_ROOT)
    return check_commit(paths, commit=args.commit, repo_root=REPO_ROOT)


if __name__ == "__main__":
    raise SystemExit(main())
