#!/usr/bin/env python3
"""Run the automated proof floor for the commercial cancellation/reactivation gate."""

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


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Run the automated proof bundle for the commercial cancellation/"
            "reactivation release gate."
        )
    )
    parser.add_argument(
        "--pulse-dir",
        default=str(default_pulse_dir()),
        help="Path to the pulse repo root",
    )
    parser.add_argument(
        "--pulse-pro-license-server-dir",
        default=str(default_pulse_pro_license_server_dir()),
        help="Path to the pulse-pro/license-server directory",
    )
    parser.add_argument(
        "--frontend-dir",
        help="Optional override for the pulse frontend-modern directory",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit JSON instead of human-readable output",
    )
    parser.add_argument(
        "--report-out",
        help="Optional path to write a markdown report",
    )
    parser.add_argument(
        "--report-title",
        default="Commercial Cancellation/Reactivation Automated Proof",
        help="Markdown title used when writing --report-out",
    )
    return parser.parse_args(argv)


def default_pulse_dir() -> Path:
    return Path(__file__).resolve().parents[2]


def default_pulse_pro_license_server_dir() -> Path:
    return default_pulse_dir().parent / "pulse-pro" / "license-server"


def frontend_dir_from_args(args: argparse.Namespace) -> Path:
    if args.frontend_dir:
        return Path(args.frontend_dir).resolve()
    return Path(args.pulse_dir).resolve() / "frontend-modern"


def build_command_specs(args: argparse.Namespace) -> list[CommandSpec]:
    pulse_dir = Path(args.pulse_dir).resolve()
    pulse_pro_license_server_dir = Path(args.pulse_pro_license_server_dir).resolve()
    frontend_dir = frontend_dir_from_args(args)
    return [
        CommandSpec(
            name="pulse-api-cancellation-boundary",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./internal/api",
                "-run",
                "TestStripeWebhook_SubscriptionDeleted_RevokesCapabilities",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="pulse-v5-recurring-upgrade-migration",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./tests/migration",
                "-run",
                "TestV5FullUpgradeScenario/PersistedV5RecurringLicenseAutoExchanges",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="frontend-grandfathered-license-presentation",
            cwd=str(frontend_dir),
            command=[
                "npm",
                "test",
                "--",
                "src/utils/__tests__/licensePresentation.test.ts",
                "src/components/Settings/__tests__/ProLicensePanel.test.tsx",
            ],
        ),
        CommandSpec(
            name="pulse-pro-public-checkout-reentry-guard",
            cwd=str(pulse_pro_license_server_dir),
            command=[
                "go",
                "test",
                ".",
                "-run",
                "TestHandleCheckoutSessionCreate(_RejectsGrandfatheredPlanKey)?$",
                "-count=1",
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
    proc = subprocess.run(
        spec.command,
        cwd=spec.cwd,
        capture_output=True,
        text=True,
        check=False,
    )
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
    lines = [f"# {title}", "", "## Results", ""]
    for result in results:
        status = "PASS" if result.ok else "FAIL"
        lines.append(f"- `{status}` `{result.name}`")
        lines.append(f"  - cwd: `{result.cwd}`")
        lines.append(f"  - command: `{' '.join(result.command)}`")
        lines.append(f"  - detail: {result.detail}")
    lines.extend(
        [
            "",
            "## Manual Follow-up",
            "",
            "- If all commands passed, continue with the manual scenarios in",
            "  `docs/release-control/v6/COMMERCIAL_CANCELLATION_REACTIVATION_E2E_TEST_PLAN.md`.",
            "- Save the executed manual record from",
            "  `docs/release-control/v6/COMMERCIAL_CANCELLATION_REACTIVATION_RECORD_TEMPLATE.md`.",
        ]
    )
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    results = run_proof(args)
    if args.report_out:
        report_path = Path(args.report_out)
        report_path.write_text(
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
