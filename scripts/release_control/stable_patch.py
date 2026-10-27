#!/usr/bin/env python3
"""Reconstruct a mechanical stable patch. No network, provider or publication operations.

The train supplies GitHub's closing-PR selection. Admission reconstructs that
selection again and admits only release metadata on top, never other main work.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import re
import subprocess
import tempfile
import sys
import time

sys.dont_write_bytecode = True

SHA = re.compile(r"[0-9a-f]{40}")
VERSION = re.compile(r"(\d+)\.(\d+)\.([1-9]\d*)")
CONTROLS = (".github/workflows", "scripts/release_control")
SCHEMA = 1
# Founder direction, 14/26 October: weekly minor, 24-hour candidate soak.
# This publication fence has no caller-selected cadence or clock override.
MINOR_CYCLE_SECONDS = 7 * 86400
MINOR_SOAK_SECONDS = 24 * 3600
PATCH_MARGIN_SECONDS = 48 * 3600
REPOSITORY = "rcourtman/Pulse"


def git(root: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess:
    return subprocess.run(["git", "-c", "user.name=Pulse release",
                           "-c", "user.email=release@invalid", "-C", str(root), *args],
                          text=True, capture_output=True, check=check)


def value(root: Path, *args: str) -> str:
    return git(root, *args).stdout.strip()


def manifest_path(version: str) -> str:
    if not VERSION.fullmatch(version):
        raise ValueError("a stable patch needs X.Y.Z with a positive patch number")
    return f"docs/releases/PATCH_v{version}.json"


def validate_selection(root: Path, selection: dict) -> None:
    if set(selection) != {"schema_version", "version", "stable_tag", "stable_sha", "control_sha", "fixes"}:
        raise ValueError("invalid patch selection fields")
    if selection["schema_version"] != SCHEMA:
        raise ValueError("unsupported patch selection schema")
    manifest_path(selection["version"])
    major, minor, patch = map(int, selection["version"].split("."))
    if selection["stable_tag"] != f"v{major}.{minor}.{patch - 1}":
        raise ValueError("a patch must immediately follow its stable base")
    for key in ("stable_sha", "control_sha"):
        if not isinstance(selection[key], str) or not SHA.fullmatch(selection[key]):
            raise ValueError(f"invalid {key}")
    if value(root, "rev-parse", selection["stable_tag"] + "^{commit}") != selection["stable_sha"]:
        raise ValueError("stable tag does not identify the selected stable source")
    git(root, "merge-base", selection["stable_sha"], selection["control_sha"])
    fixes = selection["fixes"]
    if not isinstance(fixes, list) or not fixes:
        raise ValueError("a patch needs at least one closing pull request")
    # First-parent order is the order merge results reached main, including squashes.
    order = value(root, "rev-list", "--first-parent", "--reverse",
                  f"{selection['stable_sha']}..{selection['control_sha']}").splitlines()
    indices = []
    seen = set()
    for fix in fixes:
        if (not isinstance(fix, dict) or set(fix) != {"number", "merge_sha", "issues"}
                or type(fix["number"]) is not int or fix["number"] <= 0
                or fix["number"] in seen or not SHA.fullmatch(fix.get("merge_sha", ""))
                or not isinstance(fix["issues"], list) or not fix["issues"]
                or any(type(n) is not int or n <= 0 for n in fix["issues"])):
            raise ValueError("invalid or duplicate closing-PR identity")
        seen.add(fix["number"])
        if fix["merge_sha"] not in order:
            raise ValueError("fix is not an unshipped first-parent merge result on selected main")
        indices.append(order.index(fix["merge_sha"]))
    if indices != sorted(set(indices)):
        raise ValueError("fixes are not in merge order")


def commit(root: Path, message: str) -> None:
    git(root, "commit", "--quiet", "--allow-empty", "-m", message)


def assemble(root: Path, selection: dict) -> dict:
    """Apply whole merge results, skip only actual conflicts, then copy exact controls."""
    validate_selection(root, selection)
    if value(root, "status", "--porcelain"):
        raise ValueError("patch assembly requires a clean disposable checkout")
    git(root, "checkout", "--quiet", "--detach", selection["stable_sha"])
    applied, skipped = [], []
    for fix in selection["fixes"]:
        before = value(root, "rev-parse", "HEAD")
        parents = value(root, "rev-list", "--parents", "-n", "1", fix["merge_sha"]).split()[1:]
        args = ["cherry-pick", "--no-commit"]
        if len(parents) > 1:
            args += ["-m", "1"]
        result = git(root, *args, fix["merge_sha"], check=False)
        if result.returncode:
            conflicts = value(root, "diff", "--name-only", "--diff-filter=U").splitlines()
            # No empty/error/authentication failure is reclassified as a conflict.
            if not conflicts:
                raise ValueError(f"merge result #{fix['number']} failed without a content conflict")
            git(root, "reset", "--hard", before)
            git(root, "cherry-pick", "--quit", check=False)
            skipped.append({**fix, "conflicts": conflicts})
            continue
        commit(root, f"Apply the merged fix from #{fix['number']}\n\nMerge-result: {fix['merge_sha']}")
        applied.append(fix)
    git(root, "restore", "--source=" + selection["control_sha"], "--staged", "--worktree", "--", *CONTROLS)
    commit(root, "Use the reviewed release controls at " + selection["control_sha"])
    receipt = {**selection, "applied": applied, "skipped": skipped,
               "assembled_tree": value(root, "rev-parse", "HEAD^{tree}")}
    target = root / manifest_path(selection["version"])
    # Do not write through an archive's symlink or a parent directory symlink.
    if not target.resolve().is_relative_to(root.resolve()) or target.is_symlink():
        raise ValueError("patch manifest path escapes its disposable checkout")
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    git(root, "add", "--", manifest_path(selection["version"]))
    commit(root, "Record the mechanical stable patch source selection")
    return receipt


def validate(root: Path, version: str, source_branch: str) -> dict:
    """Rebuild the source tree and reject every extra byte outside release metadata."""
    core = version.removeprefix("v").removesuffix("-rc.1")
    if version.removeprefix("v") not in (core, core + "-rc.1"):
        raise ValueError("a stable patch uses only rc.1 and its unchanged stable promotion")
    if source_branch != "release/v" + core:
        raise ValueError("stable patch is not on its version-bound release branch")
    receipt = json.loads((root / manifest_path(core)).read_text())
    selection = {key: receipt[key] for key in
                 ("schema_version", "version", "stable_tag", "stable_sha", "control_sha", "fixes")}
    if selection["version"] != core:
        raise ValueError("patch manifest belongs to another version")
    validate_selection(root, selection)
    # Source/workflow controls must have been protected on main, not a foreign branch.
    git(root, "merge-base", "--is-ancestor", selection["control_sha"], "refs/remotes/origin/main")
    head = value(root, "rev-parse", "HEAD")
    with tempfile.TemporaryDirectory(prefix="pulse-patch-admission-") as raw:
        disposable = Path(raw) / "pulse"
        subprocess.run(["git", "clone", "--quiet", "--shared", "--no-checkout", str(root), str(disposable)],
                       check=True, capture_output=True)
        git(disposable, "checkout", "--quiet", "--detach", selection["control_sha"])
        rebuilt = assemble(disposable, selection)
        if rebuilt != receipt:
            raise ValueError("patch source receipt does not match reconstructed merge results")
        rebuilt_head = value(disposable, "rev-parse", "HEAD")
        # Inspect the candidate from the reconstructed checkout without executing its source.
        from resolve_release_promotion import RELEASE_METADATA_PATH_RE, VERSION_PIN_LINE_RES
        paths = value(disposable, "diff", "--name-only", rebuilt_head, head).splitlines()
        for path in paths:
            if RELEASE_METADATA_PATH_RE.fullmatch(path):
                continue
            pin = VERSION_PIN_LINE_RES.get(path)
            if pin:
                old = value(disposable, "show", f"{rebuilt_head}:{path}").splitlines()
                new = value(disposable, "show", f"{head}:{path}").splitlines()
                if (len([x for x in old if pin.fullmatch(x)]) == 1
                        and len([x for x in new if pin.fullmatch(x)]) == 1
                        and [x for x in old if not pin.fullmatch(x)] == [x for x in new if not pin.fullmatch(x)]):
                    continue
            raise ValueError(f"stable patch contains unselected source: {path}")
    return receipt


def api(path: str) -> object:
    """Read fixed project evidence only; a failed read is never retried or defaulted."""
    result = subprocess.run(["gh", "api", f"repos/{REPOSITORY}/{path}"],
                            check=True, text=True, capture_output=True)
    return json.loads(result.stdout)


def published_at(row: dict, tag: str) -> float:
    if (row.get("tag_name") != tag or row.get("draft") is not False
            or row.get("prerelease") is not False or not row.get("published_at")):
        raise ValueError("patch base/anchor must be a published stable release")
    stamp = datetime.fromisoformat(row["published_at"].replace("Z", "+00:00"))
    if stamp.tzinfo is None:
        raise ValueError("stable publication time has no timezone")
    return stamp.timestamp()


def activation_check(root: Path, version: str, source_sha: str) -> dict:
    """Re-read current stable and the clock immediately before public activation.

    Source reconstruction earlier in the pipeline cannot establish that its
    stable base is still current after a long build or activation-only recovery.
    Minor, beta and historical-line publication rules are deliberately unchanged.
    """
    match = re.fullmatch(r"v?(\d+)\.(\d+)\.(\d+)(?:-([a-z]+)\.(\d+))?", version)
    if not match:
        raise ValueError("invalid release version")
    major, minor, patch = map(int, match.groups()[:3])
    if not patch or (major, minor) < (6, 5):
        return {"applicable": False}
    if match.group(4) and match.groups()[3:] != ("rc", "1"):
        raise ValueError("mechanical patches permit only rc.1 and stable")
    if not SHA.fullmatch(source_sha):
        raise ValueError("activation source must be an exact SHA")
    core = f"{major}.{minor}.{patch}"
    receipt = json.loads(value(root, "show", f"{source_sha}:{manifest_path(core)}"))
    if receipt["version"] != core or receipt["stable_tag"] != f"v{major}.{minor}.{patch - 1}":
        raise ValueError("activation manifest does not identify the preceding stable")
    if not SHA.fullmatch(receipt["stable_sha"]):
        raise ValueError("activation manifest has no exact stable source")
    # Pagination is bounded and complete. A short/truncated first page cannot
    # hide a newer stable behind a run of previews or draft releases.
    rows = []
    for page in range(1, 101):
        batch = api(f"releases?per_page=100&page={page}")
        if not isinstance(batch, list):
            raise ValueError("stable release inventory unavailable")
        for row in batch:
            if not isinstance(row, dict) or not isinstance(row.get("tag_name"), str):
                raise ValueError("malformed release inventory")
            if re.fullmatch(r"v\d+\.\d+\.\d+", row["tag_name"]):
                if type(row.get("draft")) is not bool or type(row.get("prerelease")) is not bool:
                    raise ValueError("stable release inventory has unknown publication state")
        rows.extend(batch)
        if len(batch) < 100:
            break
    else:
        raise ValueError("stable release inventory exceeds bounded pagination")
    stable = [row for row in rows if row.get("draft") is False
              and row.get("prerelease") is False
              and re.fullmatch(r"v\d+\.\d+\.\d+", row.get("tag_name", ""))]
    if not stable:
        raise ValueError("no current published stable")
    current = max(stable, key=lambda row: tuple(map(int, row["tag_name"][1:].split("."))))
    if current["tag_name"] != receipt["stable_tag"]:
        raise ValueError("a newer stable superseded the patch base before activation")
    published_at(current, receipt["stable_tag"])
    commit = api(f"commits/{receipt['stable_tag']}")
    if not isinstance(commit, dict) or commit.get("sha") != receipt["stable_sha"]:
        raise ValueError("published patch base source changed before activation")
    if match.group(4) is None:
        # The 48-hour rule forbids a NEW patch cut. It must not become an
        # additional hold on promotion of its already-cut, soaked rc.1.
        return {"applicable": True, "source_sha": source_sha, "stable_tag": current["tag_name"],
                "stable_sha": receipt["stable_sha"], "observed_at": time.time(), "new_cut": False}
    anchor_tag = f"v{major}.{minor}.0"
    anchor = current if current["tag_name"] == anchor_tag else api(f"releases/tags/{anchor_tag}")
    due = published_at(anchor, anchor_tag) + MINOR_CYCLE_SECONDS - MINOR_SOAK_SECONDS
    # Read the time after provider evidence, not before potentially slow reads.
    observed = time.time()
    if observed >= due - PATCH_MARGIN_SECONDS:
        raise ValueError("patch activation is within 48 hours of minor candidate eligibility (or overdue)")
    return {"applicable": True, "new_cut": True, "source_sha": source_sha, "stable_tag": current["tag_name"],
            "stable_sha": receipt["stable_sha"], "observed_at": observed,
            "not_after": datetime.fromtimestamp(due - PATCH_MARGIN_SECONDS, timezone.utc).isoformat()}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("assemble", "validate", "activation-check"))
    parser.add_argument("--selection", type=Path)
    parser.add_argument("--version")
    parser.add_argument("--source-branch")
    parser.add_argument("--source-sha")
    args = parser.parse_args()
    root = Path.cwd().resolve()
    if args.operation == "assemble":
        result = assemble(root, json.loads(args.selection.read_text()))
    elif args.operation == "validate":
        result = validate(root, args.version, args.source_branch)
    else:
        result = activation_check(root, args.version, args.source_sha)
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
