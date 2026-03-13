#!/usr/bin/env python3
"""Run the live commercial cancellation/reactivation rehearsal."""

from __future__ import annotations

import argparse
import json
import subprocess
from dataclasses import asdict, dataclass
from pathlib import Path


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


def default_integration_dir() -> Path:
    return default_pulse_dir() / "tests" / "integration"


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Run the commercial cancellation/reactivation rehearsal against a live "
            "Pulse + Stripe environment and optionally write a markdown report."
        )
    )
    parser.add_argument(
        "--pulse-dir",
        default=str(default_pulse_dir()),
        help="Path to the pulse repo root",
    )
    parser.add_argument(
        "--integration-dir",
        default=str(default_integration_dir()),
        help="Path to the Pulse integration test package",
    )
    parser.add_argument(
        "--project",
        default="chromium",
        help="Playwright project to run",
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
        default="Commercial Cancellation/Reactivation Rehearsal",
        help="Markdown title used when writing --report-out",
    )
    return parser.parse_args(argv)


def summarize_output(stdout: str, stderr: str) -> str:
    text = "\n".join(part.strip() for part in (stdout, stderr) if part.strip()).strip()
    if not text:
        return "pass"
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    summary = lines[-1]
    if len(summary) > 240:
        return summary[:237] + "..."
    return summary


def run_command(name: str, cwd: Path, command: list[str]) -> CommandResult:
    proc = subprocess.run(
        command,
        cwd=str(cwd),
        capture_output=True,
        text=True,
        check=False,
    )
    return CommandResult(
        name=name,
        cwd=str(cwd),
        command=command,
        ok=proc.returncode == 0,
        exit_code=proc.returncode,
        detail=summarize_output(proc.stdout, proc.stderr),
    )


def run_rehearsal(args: argparse.Namespace) -> list[CommandResult]:
    pulse_dir = Path(args.pulse_dir).resolve()
    integration_dir = Path(args.integration_dir).resolve()
    return [
        run_command(
            "commercial-cancellation-automated-proof-floor",
            pulse_dir,
            ["python3", "scripts/release_control/commercial_cancellation_reactivation_proof.py", "--json"],
        ),
        run_command(
            "commercial-cancellation-playwright-live-journey",
            integration_dir,
            [
                "npm",
                "test",
                "--",
                "tests/14-commercial-cancellation-reactivation.spec.ts",
                f"--project={args.project}",
            ],
        ),
    ]


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
            "## Environment Requirements",
            "",
            "- Stripe sandbox credentials and the commercial fixture env vars documented in",
            "  `tests/integration/tests/14-commercial-cancellation-reactivation.spec.ts`.",
            "- A live Pulse runtime whose authenticated settings surface reflects the migrated",
            "  recurring commercial state under test.",
            "- A real public checkout origin for `pulse-pro/license-server`.",
        ]
    )
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    results = run_rehearsal(args)
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
