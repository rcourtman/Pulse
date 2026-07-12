#!/usr/bin/env python3
"""Run the released, label-scoped Task06 Docker restart journey in Colima."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import shutil

from artifact_redaction import assert_allowed_artifact_tree, contains_forbidden_secret_shape, redact_text, write_checksums

RUN_ID = re.compile(r"^[a-z0-9][a-z0-9-]{5,47}$")
LABEL_KEY = "com.pulse.intelligence-lab.run"


class LabError(RuntimeError):
    pass


def command(args: list[str], *, env: dict[str, str] | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(args, text=True, capture_output=True, env=env, timeout=600)
    if check and result.returncode:
        raise LabError(f"command failed ({result.returncode}): {' '.join(args[:4])}\n{redact_text(result.stderr)}")
    return result


def docker(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return command(["docker", "--context", "colima", *args], check=check)


def inventory() -> dict[str, list[str]]:
    return {
        "containers": sorted(filter(None, docker("ps", "-aq", "--no-trunc").stdout.splitlines())),
        "volumes": sorted(filter(None, docker("volume", "ls", "-q").stdout.splitlines())),
        "networks": sorted(filter(None, docker("network", "ls", "-q", "--no-trunc").stdout.splitlines())),
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


def write_json(path: Path, value: object) -> None:
    payload = json.dumps(value, indent=2, sort_keys=True) + "\n"
    if contains_forbidden_secret_shape(payload):
        raise LabError(f"artifact failed secret-shape scan: {path.name}")
    path.write_text(redact_text(payload), encoding="utf-8")
    path.chmod(0o600)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--repo", default=str(Path(__file__).resolve().parents[2]))
    args = parser.parse_args()
    if not RUN_ID.fullmatch(args.run_id):
        raise LabError("run id must be 6-48 lowercase letters, digits, or hyphens")
    repo = Path(args.repo).resolve()
    artifact_dir = repo / "tmp" / "intelligence-lab" / args.run_id
    scratch_dir = repo / "tmp" / "intelligence-lab-scratch" / args.run_id
    artifact_dir.mkdir(parents=True, exist_ok=False)
    scratch_dir.mkdir(parents=True, exist_ok=False)
    label = f"{LABEL_KEY}={args.run_id}"
    name = f"pulse-task06-{args.run_id}"
    pre = inventory()
    report: dict[str, object] = {"run_id": args.run_id, "label": label, "preexisting_inventory": pre, "cleanup": {}}
    failed: BaseException | None = None
    try:
        if docker("context", "show").stdout.strip() != "colima":
            raise LabError("explicit Docker context is not colima")
        versions = {
            "git_sha": command(["git", "-C", str(repo), "rev-parse", "HEAD"]).stdout.strip(),
            "docker_client": docker("version", "--format", "{{.Client.Version}}").stdout.strip(),
            "docker_server": docker("version", "--format", "{{.Server.Version}}").stdout.strip(),
            "docker_context": docker("context", "show").stdout.strip(),
            "colima": command(["colima", "version"]).stdout.strip(),
        }
        write_json(artifact_dir / "environment.json", versions)
        docker("network", "create", "--label", label, name)
        docker("volume", "create", "--label", label, name)
        fixture_script = 'n=$(cat /state/count 2>/dev/null || echo 0); n=$((n+1)); echo "$n" >/state/count; if [ "$n" -le 4 ]; then exit 1; fi; sleep 3600'
        container_id = docker(
            "run", "-d", "--name", name, "--label", label, "--network", name,
            "--mount", f"source={name},target=/state", "--restart", "on-failure:5",
            "alpine:3.20", "/bin/sh", "-c", fixture_script,
        ).stdout.strip()
        deadline = time.time() + 30
        restart_count = 0
        state = ""
        while time.time() < deadline:
            observed = docker("inspect", "--format", "{{.State.Status}} {{.RestartCount}}", container_id).stdout.split()
            if len(observed) == 2:
                state, restart_count = observed[0], int(observed[1])
            if state == "running" and restart_count >= 4:
                break
            time.sleep(0.25)
        if state != "running" or restart_count < 4:
            raise LabError(f"fixture did not reach bounded fresh restart-loop state: state={state} restarts={restart_count}")
        report["fixture"] = {"container_id": container_id, "state": state, "restart_count": restart_count}
        env = os.environ.copy()
        env.update({
            "DOCKER_CONTEXT": "colima",
            "PULSE_INTELLIGENCE_LAB_RUN_ID": args.run_id,
            "PULSE_INTELLIGENCE_LAB_CONTAINER_ID": container_id,
            "PULSE_INTELLIGENCE_LAB_ARTIFACT_DIR": str(artifact_dir),
            "PULSE_INTELLIGENCE_LAB_SCRATCH_DIR": str(scratch_dir),
        })
        proofs = [
            ["go", "test", "./internal/agentexec", "./internal/hostagent", "./internal/api", "-run", "DockerContainer|DockerLifecycle|DockerRestartColimaRealLab", "-count=1"],
            ["go", "test", "./internal/actionlifecycle", "./internal/agentexec", "./internal/hostagent", "./internal/api", "-run", "ConcurrentDuplicateExecuteAtClaimBoundary|RestartRecoveryReconcilesReceiptPendingWithoutResend|RestartRecoveryNotFoundRemainsReceiptPendingWithoutResend|PolicyBarrierRevocations|EmergencyStopBlocksHumanAndPolicyAdmission|NeverAutoRemediate|WrongAgentLateDuplicateAndInterruptedAreInert|DockerDuplicateReplay|StaleBeforeStateIsTripleZero|FailedInspectIsTripleZero|StaleReadbackIsInconclusive", "-count=1"],
            [sys.executable, "scripts/intelligence_lab/test_artifact_redaction.py"],
        ]
        proof_results = []
        for proof in proofs:
            result = command(proof, env=env)
            proof_results.append({"command": proof, "exit_code": result.returncode, "stdout": redact_text(result.stdout), "stderr": redact_text(result.stderr)})
        report["proofs"] = proof_results
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
            failed = failed or LabError("run-scoped scratch state was not deleted")
        write_json(artifact_dir / "run-report.json", report)
        assert_allowed_artifact_tree(artifact_dir)
        write_checksums(artifact_dir)
        if second != {"containers": [], "volumes": [], "networks": []}:
            failed = failed or LabError("second label-scoped cleanup was not a no-op")
        if post != pre:
            failed = failed or LabError("preexisting Colima inventory changed")
    if failed:
        raise failed
    print(artifact_dir)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
