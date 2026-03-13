#!/usr/bin/env python3
"""Run the automated proof bundle for the relay registration/reconnect gate."""

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


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run the automated proof bundle for the relay registration/reconnect/drain release gate."
    )
    parser.add_argument("--pulse-dir", default=str(default_pulse_dir()))
    parser.add_argument("--pulse-mobile-dir", default=str(default_pulse_mobile_dir()))
    parser.add_argument("--frontend-dir", help="Optional override for the pulse frontend-modern directory")
    parser.add_argument("--json", action="store_true", help="Emit JSON instead of human-readable output")
    return parser.parse_args(argv)


def frontend_dir_from_args(args: argparse.Namespace) -> Path:
    if args.frontend_dir:
        return Path(args.frontend_dir).resolve()
    return Path(args.pulse_dir).resolve() / "frontend-modern"


def build_command_specs(args: argparse.Namespace) -> list[CommandSpec]:
    pulse_dir = Path(args.pulse_dir).resolve()
    pulse_mobile_dir = Path(args.pulse_mobile_dir).resolve()
    frontend_dir = frontend_dir_from_args(args)
    return [
        CommandSpec(
            name="relay-backend-api-guards",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./internal/api",
                "-run",
                "TestRelayEndpointsRequireLicenseFeature|TestRelayOnboardingEndpointsRequireLicenseFeature|TestRelayLicenseGatingResponseFormat|TestOnboardingQRPayloadStructure|TestOnboardingValidateSuccessAndFailure|TestOnboardingDeepLinkFormat",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="relay-backend-runtime",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./internal/relay",
                "-run",
                "TestClient_E2E_MultiMobileClientRelay|TestClient_AbruptDisconnectCancelsInFlightHandlers|TestClient_AbruptDisconnectMultipleChannelCleanup|TestClient_DrainDuringInFlightData|TestClient_DrainWithMultipleInFlightChannels|TestClientRegister_SessionResumeRejectionClearsCachedSession|TestRunLoop_SessionResumeRejectionFallsBackToFreshRegister",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="relay-frontend-runtime",
            cwd=str(frontend_dir),
            command=[
                "npx",
                "vitest",
                "run",
                "src/components/Dashboard/__tests__/RelayOnboardingCard.test.tsx",
                "src/components/Settings/__tests__/RelaySettingsPanel.runtime.test.tsx",
                "src/components/Settings/__tests__/settingsReadOnlyPanels.test.tsx",
            ],
        ),
        CommandSpec(
            name="relay-managed-runtime",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./internal/relay",
                "-run",
                "TestManagedRuntimeRelayRegistrationReconnectDrain",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="relay-mobile-client",
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
