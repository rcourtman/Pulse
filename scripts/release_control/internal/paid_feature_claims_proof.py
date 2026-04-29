#!/usr/bin/env python3
"""Run the automated proof bundle for self-hosted paid feature claims."""

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
    return Path(__file__).resolve().parents[3]


def default_pulse_pro_license_server_dir() -> Path:
    return default_pulse_dir().parent / "pulse-pro" / "license-server"


def default_pulse_pro_relay_server_dir() -> Path:
    return default_pulse_dir().parent / "pulse-pro" / "relay-server"


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run the automated proof bundle for v6 paid feature claims."
    )
    parser.add_argument("--pulse-dir", default=str(default_pulse_dir()))
    parser.add_argument("--frontend-dir", help="Optional override for the pulse frontend-modern directory")
    parser.add_argument(
        "--pulse-pro-license-server-dir",
        default=str(default_pulse_pro_license_server_dir()),
    )
    parser.add_argument(
        "--pulse-pro-relay-server-dir",
        default=str(default_pulse_pro_relay_server_dir()),
    )
    parser.add_argument("--json", action="store_true", help="Emit JSON instead of human-readable output")
    parser.add_argument("--report-out", help="Optional path to write a markdown report")
    parser.add_argument(
        "--report-title",
        default="Paid Feature Claim Automated Proof",
        help="Markdown title used when writing --report-out",
    )
    return parser.parse_args(argv)


def frontend_dir_from_args(args: argparse.Namespace) -> Path:
    if args.frontend_dir:
        return Path(args.frontend_dir).resolve()
    return Path(args.pulse_dir).resolve() / "frontend-modern"


def build_command_specs(args: argparse.Namespace) -> list[CommandSpec]:
    pulse_dir = Path(args.pulse_dir).resolve()
    frontend_dir = frontend_dir_from_args(args)
    pulse_pro_license_server_dir = Path(args.pulse_pro_license_server_dir).resolve()
    pulse_pro_relay_server_dir = Path(args.pulse_pro_relay_server_dir).resolve()
    return [
        CommandSpec(
            name="pulse-licensing-paid-feature-contract",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./pkg/licensing",
                "-run",
                "^(TestSelfHostedPaidFeatureClaimMatrix|TestTierMonitoredSystemLimits|TestTierHistoryDays|TestTierFeatureInheritance|TestBuildEntitlementPayload_MaxHistoryDays)$",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="pulse-api-history-entitlement-enforcement",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./internal/api",
                "-run",
                "^TestHandleMetricsHistory_TierAwareHistoryRanges$",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="pulse-frontend-paid-claim-contract",
            cwd=str(frontend_dir),
            command=[
                "npm",
                "test",
                "--",
                "src/utils/__tests__/selfHostedPlans.test.ts",
                "src/utils/__tests__/commercialBillingModel.test.ts",
                "src/utils/__tests__/licensePresentation.test.ts",
                "src/stores/__tests__/license.test.ts",
                "src/components/shared/__tests__/HistoryChart.test.tsx",
                "src/components/Settings/__tests__/ProLicensePanel.test.tsx",
                "src/components/Settings/__tests__/RelaySettingsPanel.runtime.test.tsx",
            ],
        ),
        CommandSpec(
            name="pulse-pro-public-pricing-and-checkout-contract",
            cwd=str(pulse_pro_license_server_dir),
            command=[
                "go",
                "test",
                ".",
                "-run",
                "^(TestHandlePublicPricingModelV6|TestCanonicalTierEntitlementContracts_DoNotMonetizeMonitoringVolume|TestV6PublicProPricingPreservesPaidCapabilityFloor|TestHandleCheckoutSessionCreate|TestResolvePublicCheckoutPlan_LockedSelfHostedTiers|TestHandlePulseProDownloadRequiresAuth|TestHandlePulseProDownloadActivationKeyReturnsSignedURLs|TestHandlePulseProDownloadRejectsFreeLicense)$",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="pulse-pro-relay-entitlement-runtime",
            cwd=str(pulse_pro_relay_server_dir),
            command=[
                "go",
                "test",
                ".",
                "-run",
                "^(TestValidateLicense_AllProTiers|TestValidateLicense_FreeTierRejected|TestHostedEntitlementValidator_RejectsMissingRelayCapability|TestBridge_V6GrantRegister|TestBridge_V6GrantRejected_CancelledState|TestBridge_V6Grant_FullDataRouting|TestBridge_PushNotificationHandled)$",
                "-count=1",
            ],
        ),
    ]


def summarize_output(stdout: str, stderr: str) -> str:
    stdout_lines = [line.strip() for line in stdout.splitlines() if line.strip()]
    stderr_lines = [line.strip() for line in stderr.splitlines() if line.strip()]
    for line in reversed(stdout_lines):
        if line.startswith("ok ") or line.startswith("PASS ") or " passed " in line:
            return line[:237] + "..." if len(line) > 240 else line
    lines = stdout_lines or stderr_lines
    if not lines:
        return "pass"
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


def render_markdown_report(title: str, results: list[CommandResult]) -> str:
    lines = [
        f"# {title}",
        "",
        "## Scope",
        "",
        "This proof checks that self-hosted paid claims map to concrete entitlement, API, UI, license-server, and relay-server behavior.",
        "",
        "## Results",
        "",
    ]
    for result in results:
        status = "PASS" if result.ok else "FAIL"
        lines.append(f"- `{status}` `{result.name}`")
        lines.append(f"  - cwd: `{result.cwd}`")
        lines.append(f"  - command: `{' '.join(result.command)}`")
        lines.append(f"  - detail: {result.detail}")
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    results = run_proof(args)
    if args.report_out:
        Path(args.report_out).write_text(
            render_markdown_report(args.report_title, results),
            encoding="utf-8",
        )
    if args.json:
        print(json.dumps([asdict(result) for result in results], indent=2))
    else:
        for result in results:
            status = "PASS" if result.ok else "FAIL"
            print(f"{status} {result.name}: {result.detail}")
    return 0 if all(result.ok for result in results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
