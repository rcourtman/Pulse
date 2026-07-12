#!/usr/bin/env python3
"""Run the RG-09 Debian and Ubuntu APT workflow certification in Colima."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

REPO_ROOT = Path(__file__).resolve().parents[4]
INTELLIGENCE_LAB_DIR = REPO_ROOT / "scripts" / "intelligence_lab"
if not INTELLIGENCE_LAB_DIR.is_dir():
    raise ImportError(f"canonical intelligence lab helpers missing: {INTELLIGENCE_LAB_DIR}")
sys.path.insert(0, str(INTELLIGENCE_LAB_DIR))

from artifact_redaction import (
    assert_allowed_artifact_tree,
    contains_forbidden_secret_shape,
    redact_text,
    write_checksums,
)

RUN_ID = re.compile(r"^[a-z0-9][a-z0-9-]{5,47}$")
SHA = re.compile(r"^[0-9a-f]{40}$")
LABEL_KEY = "com.pulse.intelligence-lab.run"
GATE_LABEL = "com.pulse.intelligence-lab.gate=rg-09"
IMAGES = ("debian:bookworm-slim", "ubuntu:24.04")


class LabError(RuntimeError):
    pass


def isolated_go_cache(run_id: str) -> Path:
    return Path("/Volumes/Development/.go-task-caches") / f"rg09-{run_id}"


def write_json(path: Path, value: object) -> None:
    payload = json.dumps(value, indent=2, sort_keys=True) + "\n"
    if contains_forbidden_secret_shape(payload):
        raise LabError(f"artifact failed secret-shape scan: {path.name}")
    path.write_text(redact_text(payload), encoding="utf-8")
    path.chmod(0o600)


def command(
    args: list[str],
    *,
    cwd: Path | None = None,
    env: dict[str, str] | None = None,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        args,
        text=True,
        capture_output=True,
        cwd=cwd,
        env=env,
        timeout=1800,
    )
    if check and result.returncode:
        detail = redact_text(result.stderr or result.stdout)
        raise LabError(
            f"command failed ({result.returncode}): {' '.join(args[:6])}: {detail}"
        )
    return result


def docker(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return command(["docker", "--context", "colima", *args], check=check)


def inventory() -> dict[str, list[str]]:
    return {
        "containers": sorted(filter(None, docker("ps", "-aq", "--no-trunc").stdout.splitlines())),
        "volumes": sorted(filter(None, docker("volume", "ls", "-q").stdout.splitlines())),
        "networks": sorted(filter(None, docker("network", "ls", "-q", "--no-trunc").stdout.splitlines())),
        "images": sorted(
            filter(
                None,
                docker("images", "--no-trunc", "--format", "{{.Repository}}:{{.Tag}} {{.ID}}").stdout.splitlines(),
            )
        ),
    }


def labelled_ids(kind: str, run_id: str) -> list[str]:
    label = f"{LABEL_KEY}={run_id}"
    if kind == "container":
        output = docker("ps", "-aq", "--filter", f"label={label}").stdout
    elif kind == "volume":
        output = docker("volume", "ls", "-q", "--filter", f"label={label}").stdout
    elif kind == "network":
        output = docker("network", "ls", "-q", "--filter", f"label={label}").stdout
    else:
        raise ValueError(kind)
    return sorted(filter(None, output.splitlines()))


def cleanup(run_id: str) -> dict[str, list[str]]:
    removed = {"containers": [], "volumes": [], "networks": []}
    for container_id in labelled_ids("container", run_id):
        docker("rm", "-f", container_id)
        removed["containers"].append(container_id)
    for volume in labelled_ids("volume", run_id):
        docker("volume", "rm", volume)
        removed["volumes"].append(volume)
    for network in labelled_ids("network", run_id):
        docker("network", "rm", network)
        removed["networks"].append(network)
    return removed


def linux_arch() -> str:
    arch = docker("version", "--format", "{{.Server.Arch}}").stdout.strip().lower()
    mapping = {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64", "amd64": "amd64"}
    if arch not in mapping:
        raise LabError(f"unsupported Colima server architecture: {arch}")
    return mapping[arch]


def build_pinned_agent(
    repo: Path, sha: str, scratch_dir: Path, go_cache: Path
) -> tuple[Path, Path, str]:
    archive = scratch_dir / "pulse-source.tar"
    source = scratch_dir / "pulse-source"
    binary = scratch_dir / "pulse-agent-linux"
    command(["git", "-C", str(repo), "cat-file", "-e", f"{sha}^{{commit}}"])
    command(["git", "-C", str(repo), "archive", "--format=tar", "--output", str(archive), sha])
    archive_digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    source.mkdir(mode=0o700)
    command(["tar", "-xf", str(archive), "-C", str(source)])
    binding = {"git_sha": sha, "archive_sha256": archive_digest}
    (source / ".pulse-rg09-source-binding.json").write_text(
        json.dumps(binding, sort_keys=True) + "\n", encoding="utf-8"
    )
    embed_dist = source / "internal" / "api" / "frontend-modern" / "dist"
    embed_dist.mkdir(parents=True, exist_ok=True)
    (embed_dist / "index.html").write_text(
        "<!doctype html><title>RG-09 proof build</title>\n", encoding="utf-8"
    )
    env = os.environ.copy()
    go_cache.mkdir(parents=True, mode=0o700)
    env.update(
        {
            "GOOS": "linux",
            "GOARCH": linux_arch(),
            "CGO_ENABLED": "0",
            "GOCACHE": str(go_cache),
        }
    )
    command(["go", "build", "-trimpath", "-o", str(binary), "./cmd/pulse-agent"], cwd=source, env=env)
    binary.chmod(0o700)
    return source, binary, archive_digest


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--sha", required=True)
    parser.add_argument("--repo", required=True, type=Path)
    parser.add_argument("--artifact-dir", required=True, type=Path)
    parser.add_argument("--scratch-dir", required=True, type=Path)
    args = parser.parse_args()
    if not RUN_ID.fullmatch(args.run_id):
        raise LabError("run id must be 6-48 lowercase letters, digits, or hyphens")
    if not SHA.fullmatch(args.sha):
        raise LabError("sha must be a full lowercase git SHA")
    repo = args.repo.resolve()
    artifact_dir = args.artifact_dir.resolve()
    scratch_dir = args.scratch_dir.resolve()
    go_cache = isolated_go_cache(args.run_id)
    if not repo.is_dir():
        raise LabError(f"repo does not exist: {repo}")
    artifact_dir.mkdir(parents=True, exist_ok=False)
    scratch_dir.mkdir(parents=True, exist_ok=False)
    report: dict[str, object] = {
        "run_id": args.run_id,
        "git_sha": args.sha,
        "gate_label": GATE_LABEL,
        "images": list(IMAGES),
        "preexisting_inventory": {},
        "cleanup": {},
    }
    failed: BaseException | None = None
    pre: dict[str, list[str]] = {}
    try:
        command(["colima", "start", "--cpu", "4", "--memory", "6", "--disk", "20"])
        if docker("context", "show").stdout.strip() != "colima":
            raise LabError("explicit Docker context is not colima")
        pre = inventory()
        report["preexisting_inventory"] = pre
        image_ids: dict[str, str] = {}
        for image in IMAGES:
            image_id = docker("image", "inspect", "--format", "{{.Id}}", image, check=False).stdout.strip()
            if not image_id:
                raise LabError(f"required pre-existing fixture image is unavailable: {image}; refusing to pull")
            image_ids[image] = image_id
        versions = {
            "git_sha": args.sha,
            "docker_client": docker("version", "--format", "{{.Client.Version}}").stdout.strip(),
            "docker_server": docker("version", "--format", "{{.Server.Version}}").stdout.strip(),
            "docker_context": docker("context", "show").stdout.strip(),
            "colima": command(["colima", "version"]).stdout.strip(),
            "fixture_images": image_ids,
        }
        write_json(artifact_dir / "environment.json", versions)
        source, binary, archive_digest = build_pinned_agent(
            repo, args.sha, scratch_dir, go_cache
        )
        env = os.environ.copy()
        env.update(
            {
                "DOCKER_CONTEXT": "colima",
                "PULSE_INTELLIGENCE_RG09_RUN_ID": args.run_id,
                "PULSE_INTELLIGENCE_RG09_AGENT_BINARY": str(binary),
                "PULSE_INTELLIGENCE_RG09_SOURCE_DIR": str(source),
                "PULSE_INTELLIGENCE_RG09_GIT_SHA": args.sha,
                "PULSE_INTELLIGENCE_RG09_ARTIFACT_DIR": str(artifact_dir),
                "PULSE_INTELLIGENCE_RG09_SCRATCH_DIR": str(scratch_dir),
                "PULSE_INTELLIGENCE_RG09_DEBIAN_IMAGE": IMAGES[0],
                "PULSE_INTELLIGENCE_RG09_UBUNTU_IMAGE": IMAGES[1],
            }
        )
        proof_command = "go test ./internal/api -run ^TestAPTWorkflowsColimaRealLabDebianAndUbuntu$ -count=1 -v"
        result = command(
            ["go", "test", "./internal/api", "-run", "^TestAPTWorkflowsColimaRealLabDebianAndUbuntu$", "-count=1", "-v"],
            cwd=source,
            env=env,
            check=False,
        )
        report["source_binding"] = {"git_sha": args.sha, "archive_sha256": archive_digest}
        report["proof"] = {
            "command": proof_command,
            "cwd": str(source),
            "source_sha": args.sha,
            "exit_code": result.returncode,
            "stdout": redact_text(result.stdout),
            "stderr": redact_text(result.stderr),
        }
        if result.returncode:
            raise LabError("RG-09 canonical Colima test failed")
    except BaseException as exc:
        failed = exc
        report["failure"] = redact_text(str(exc))
    finally:
        first = cleanup(args.run_id) if pre else {"containers": [], "volumes": [], "networks": []}
        second = cleanup(args.run_id) if pre else {"containers": [], "volumes": [], "networks": []}
        post = inventory() if pre else {}
        report["cleanup"] = {
            "first": first,
            "second": second,
            "post_inventory": post,
            "preexisting_unchanged": bool(pre) and post == pre,
        }
        shutil.rmtree(scratch_dir, ignore_errors=True)
        shutil.rmtree(go_cache, ignore_errors=True)
        if scratch_dir.exists():
            failed = failed or LabError("RG-09 scratch state was not deleted")
        if go_cache.exists():
            failed = failed or LabError("RG-09 isolated Go cache was not deleted")
        if second != {"containers": [], "volumes": [], "networks": []}:
            failed = failed or LabError("second label-scoped cleanup was not a no-op")
        if pre and post != pre:
            failed = failed or LabError("pre-existing Colima inventory changed")
        stopped = command(["colima", "stop"], check=False)
        report["colima_stopped"] = stopped.returncode == 0
        write_json(artifact_dir / "run-report.json", report)
        assert_allowed_artifact_tree(artifact_dir)
        write_checksums(artifact_dir)
    if failed:
        raise failed
    print(artifact_dir)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
