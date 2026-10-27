#!/usr/bin/env python3
"""Reconstruct a mechanical stable patch. No network, provider or publication operations.

The train supplies GitHub's closing-PR selection. Admission reconstructs that
selection again and admits only release metadata on top, never other main work.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import subprocess
import tempfile
import sys

sys.dont_write_bytecode = True

SHA = re.compile(r"[0-9a-f]{40}")
VERSION = re.compile(r"(\d+)\.(\d+)\.([1-9]\d*)")
CONTROLS = (".github/workflows", "scripts/release_control")
SCHEMA = 1


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


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("assemble", "validate"))
    parser.add_argument("--selection", type=Path)
    parser.add_argument("--version")
    parser.add_argument("--source-branch")
    args = parser.parse_args()
    root = Path.cwd().resolve()
    result = assemble(root, json.loads(args.selection.read_text())) if args.operation == "assemble" \
        else validate(root, args.version, args.source_branch)
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
