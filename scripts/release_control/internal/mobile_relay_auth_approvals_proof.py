#!/usr/bin/env python3
"""Run the automated proof bundle for the mobile relay auth/approvals gate."""

from __future__ import annotations

import argparse
import json
import subprocess
from dataclasses import asdict, dataclass
from pathlib import Path


@dataclass
class CommandSpec:
    name: str
    cwd: str
    command: list[str]


@dataclass
class CommandResult:
    name: str
    cwd: str
    command: list[str]
    ok: bool
    exit_code: int
    detail: str


def default_pulse_dir() -> Path:
    return Path(__file__).resolve().parents[2]


def default_pulse_mobile_dir() -> Path:
    return default_pulse_dir().parent / "pulse-mobile"


def default_pulse_enterprise_dir() -> Path:
    return default_pulse_dir().parent / "pulse-enterprise"


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run the automated proof bundle for the mobile relay auth/approvals release gate."
    )
    parser.add_argument("--pulse-mobile-dir", default=str(default_pulse_mobile_dir()))
    parser.add_argument("--pulse-enterprise-dir", default=str(default_pulse_enterprise_dir()))
    parser.add_argument("--json", action="store_true", help="Emit JSON instead of human-readable output")
    return parser.parse_args(argv)


def build_command_specs(args: argparse.Namespace) -> list[CommandSpec]:
    pulse_mobile_dir = Path(args.pulse_mobile_dir).resolve()
    pulse_enterprise_dir = Path(args.pulse_enterprise_dir).resolve()
    return [
        CommandSpec(
            name="enterprise-approval-handlers",
            cwd=str(pulse_enterprise_dir),
            command=[
                "go",
                "test",
                "./internal/aiautofix",
                "-run",
                "TestHandleListApprovals|TestHandleApproveAndExecuteInvestigationFix|TestHandleApprove",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="mobile-api-client",
            cwd=str(pulse_mobile_dir),
            command=[
                "npm",
                "test",
                "--",
                "--runTestsByPath",
                "src/api/__tests__/client.test.ts",
            ],
        ),
        CommandSpec(
            name="mobile-relay-runtime",
            cwd=str(pulse_mobile_dir),
            command=[
                "npm",
                "test",
                "--",
                "--runTestsByPath",
                "src/hooks/__tests__/useRelay.test.ts",
                "src/hooks/__tests__/relayPushRefresh.test.ts",
                "src/notifications/__tests__/notificationRouting.test.ts",
                "src/stores/__tests__/mobileAccessState.test.ts",
            ],
        ),
        CommandSpec(
            name="mobile-secure-persistence-and-approvals",
            cwd=str(pulse_mobile_dir),
            command=[
                "npm",
                "test",
                "--",
                "--runTestsByPath",
                "src/__tests__/mobileRelayAuthApprovals.rehearsal.test.ts",
                "src/utils/__tests__/secureStorage.test.ts",
                "src/hooks/__tests__/useRelayLifecycle.test.ts",
                "src/hooks/__tests__/approvalActionPolicy.test.ts",
                "src/stores/__tests__/instanceStore.test.ts",
                "src/stores/__tests__/authStore.test.ts",
                "src/stores/__tests__/approvalStore.test.ts",
            ],
        ),
        CommandSpec(
            name="mobile-wire-protocol",
            cwd=str(pulse_mobile_dir),
            command=[
                "npm",
                "test",
                "--",
                "--runTestsByPath",
                "src/relay/__tests__/client.test.ts",
                "src/relay/__tests__/client-hardening.test.ts",
                "src/relay/__tests__/protocol-contract.test.ts",
            ],
        ),
    ]


def summarize_output(stdout: str, stderr: str) -> str:
    text = "\n".join(part.strip() for part in (stdout, stderr) if part.strip()).strip()
    if not text:
        return "pass"
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    summary = lines[-1]
    if len(summary) > 240:
        return summary[:237] + "..."
    return summary


def run_command(spec: CommandSpec) -> CommandResult:
    proc = subprocess.run(spec.command, cwd=spec.cwd, capture_output=True, text=True, check=False)
    return CommandResult(
        name=spec.name,
        cwd=spec.cwd,
        command=spec.command,
        ok=proc.returncode == 0,
        exit_code=proc.returncode,
        detail=summarize_output(proc.stdout, proc.stderr),
    )


def run_proof(args: argparse.Namespace) -> list[CommandResult]:
    return [run_command(spec) for spec in build_command_specs(args)]


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    results = run_proof(args)
    if args.json:
        print(json.dumps([asdict(result) for result in results], indent=2))
    else:
        for result in results:
            status = "PASS" if result.ok else "FAIL"
            print(f"{status} {result.name}: {result.detail}")
    return 0 if all(result.ok for result in results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
