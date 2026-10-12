#!/usr/bin/env python3
"""Run the RG-06 limited-unattended-autonomy journey in disposable Colima fixtures."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time

from artifact_redaction import assert_allowed_artifact_tree, contains_forbidden_secret_shape, redact_text, write_checksums

RUN_ID = re.compile(r"^[a-z0-9][a-z0-9-]{5,47}$")
SHA = re.compile(r"^[0-9a-f]{40}$")
LABEL_KEY = "com.pulse.intelligence-lab.run"
GATE_LABEL = "com.pulse.intelligence-lab.gate=rg-06"
IMAGE = "debian:bookworm-slim"


class LabError(RuntimeError):
    pass


def write_json(path: Path, value: object) -> None:
    payload = json.dumps(value, indent=2, sort_keys=True) + "\n"
    if contains_forbidden_secret_shape(payload):
        raise LabError(f"artifact failed secret-shape scan: {path.name}")
    path.write_text(redact_text(payload), encoding="utf-8")
    path.chmod(0o600)


def command(args: list[str], *, cwd: Path | None = None, env: dict[str, str] | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(args, text=True, capture_output=True, cwd=cwd, env=env, timeout=1200)
    if check and result.returncode:
        detail = redact_text(result.stderr or result.stdout)
        raise LabError(f"command failed ({result.returncode}): {' '.join(args[:5])}: {detail}")
    return result


def docker(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return command(["docker", "--context", "colima", *args], check=check)


def inventory() -> dict[str, list[str]]:
    return {
        "containers": sorted(filter(None, docker("ps", "-aq", "--no-trunc").stdout.splitlines())),
        "volumes": sorted(filter(None, docker("volume", "ls", "-q").stdout.splitlines())),
        "networks": sorted(filter(None, docker("network", "ls", "-q", "--no-trunc").stdout.splitlines())),
        "images": sorted(filter(None, docker("images", "--no-trunc", "--format", "{{.Repository}}:{{.Tag}} {{.ID}}").stdout.splitlines())),
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


def build_pinned_agent(repo: Path, sha: str, scratch_dir: Path) -> tuple[Path, Path, str]:
    archive = scratch_dir / "pulse-source.tar"
    source = scratch_dir / "pulse-source"
    binary = scratch_dir / "pulse-agent-linux"
    command(["git", "-C", str(repo), "archive", "--format=tar", "--output", str(archive), sha])
    archive_digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    source.mkdir(mode=0o700)
    command(["tar", "-xf", str(archive), "-C", str(source)])
    (source / ".pulse-rg06-source-binding.json").write_text(json.dumps({"git_sha": sha, "archive_sha256": archive_digest}, sort_keys=True) + "\n", encoding="utf-8")
    # The repository intentionally does not track generated frontend assets,
    # while internal/api embeds frontend-modern/dist at compile time. RG-06
    # does not exercise the frontend, so materialize a deterministic
    # scratch-only stub rather than borrowing assets from the dirty checkout.
    embed_dist = source / "internal" / "api" / "frontend-modern" / "dist"
    embed_dist.mkdir(parents=True, exist_ok=True)
    (embed_dist / "index.html").write_text(
        "<!doctype html><title>RG-06 proof build</title>\n",
        encoding="utf-8",
    )
    env = os.environ.copy()
    env.update({"GOOS": "linux", "GOARCH": linux_arch(), "CGO_ENABLED": "0", "GOCACHE": str(scratch_dir / "go-cache")})
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
    if not repo.is_dir():
        raise LabError(f"repo does not exist: {repo}")
    artifact_dir.mkdir(parents=True, exist_ok=False)
    scratch_dir.mkdir(parents=True, exist_ok=False)
    label = f"{LABEL_KEY}={args.run_id}"
    name = f"pulse-rg06-{args.run_id}"
    pre = inventory()
    report: dict[str, object] = {
        "run_id": args.run_id,
        "git_sha": args.sha,
        "label": label,
        "gate_label": GATE_LABEL,
        "image": IMAGE,
        "preexisting_inventory": pre,
        "authorized_dispatches": 0,
        "cleanup": {},
    }
    failed: BaseException | None = None
    try:
        if docker("context", "show").stdout.strip() != "colima":
            raise LabError("explicit Docker context is not colima")
        image = docker("image", "inspect", "--format", "{{.Id}}", IMAGE).stdout.strip()
        if not image:
            raise LabError(f"required pre-existing fixture image is unavailable: {IMAGE}; refusing to pull")
        versions = {
            "git_sha": args.sha,
            "docker_client": docker("version", "--format", "{{.Client.Version}}").stdout.strip(),
            "docker_server": docker("version", "--format", "{{.Server.Version}}").stdout.strip(),
            "docker_context": docker("context", "show").stdout.strip(),
            "colima": command(["colima", "version"]).stdout.strip(),
            "fixture_image": IMAGE,
            "fixture_image_id": image,
        }
        write_json(artifact_dir / "environment.json", versions)
        source, binary, archive_digest = build_pinned_agent(repo, args.sha, scratch_dir)
        report["fixture"] = {"agent_image": IMAGE, "image_id": image, "agent_binary": str(binary), "agent_source": str(source), "agent_id": "rg06-agent-" + args.run_id, "cache_fixture": "/var/cache/apt/archives/rg06-fixture.deb", "archive_sha256": archive_digest}
        env = os.environ.copy()
        env.update({
            "DOCKER_CONTEXT": "colima",
            "PULSE_INTELLIGENCE_RG06_RUN_ID": args.run_id,
            "PULSE_INTELLIGENCE_RG06_AGENT_ID": "rg06-agent-" + args.run_id,
            "PULSE_INTELLIGENCE_RG06_AGENT_IMAGE": IMAGE,
            "PULSE_INTELLIGENCE_RG06_AGENT_BINARY": str(binary),
            "PULSE_INTELLIGENCE_RG06_SOURCE_DIR": str(source),
            "PULSE_INTELLIGENCE_RG06_GIT_SHA": args.sha,
            "PULSE_INTELLIGENCE_RG06_ARTIFACT_DIR": str(artifact_dir),
            "PULSE_INTELLIGENCE_RG06_SCRATCH_DIR": str(scratch_dir),
        })
        result = command(
            ["go", "test", "./internal/api", "-run", "^TestPatrolAutonomyColimaRealLabCanonicalJourney$", "-count=1"],
            cwd=source,
            env=env,
            check=False,
        )
        report["proof"] = {"command": "go test ./internal/api -run ^TestPatrolAutonomyColimaRealLabCanonicalJourney$ -count=1", "cwd": str(source), "source_sha": args.sha, "exit_code": result.returncode, "stdout": redact_text(result.stdout), "stderr": redact_text(result.stderr)}
        if result.returncode:
            raise LabError("RG-06 canonical Colima test failed")
        report["authorized_dispatches"] = 1
    except BaseException as exc:
        failed = exc
        report["failure"] = redact_text(str(exc))
    finally:
        first = cleanup(args.run_id)
        second = cleanup(args.run_id)
        post = inventory()
        report["cleanup"] = {"first": first, "second": second, "post_inventory": post, "preexisting_unchanged": post == pre}
        shutil.rmtree(scratch_dir, ignore_errors=True)
        if scratch_dir.exists():
            failed = failed or LabError("RG-06 scratch state was not deleted")
        if second != {"containers": [], "volumes": [], "networks": []}:
            failed = failed or LabError("second label-scoped cleanup was not a no-op")
        if post != pre:
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
