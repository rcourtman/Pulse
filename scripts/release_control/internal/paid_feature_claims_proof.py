#!/usr/bin/env python3
"""Run the automated proof bundle for self-hosted paid feature claims."""

from __future__ import annotations

import argparse
import json
import re
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


@dataclass(frozen=True)
class CopyAuditFileSpec:
    repo: str
    relative_path: str
    required_patterns: tuple[str, ...]


FORBIDDEN_PUBLIC_COPY_PATTERNS: tuple[tuple[str, str], ...] = (
    (
        "unlimited self-hosted monitoring",
        r"\bunlimited\s+(?:self-hosted\s+)?(?:core\s+)?monitoring\b",
    ),
    ("higher monitoring limits", r"\b(?:higher|larger)\s+(?:monitoring\s+)?limits\b"),
    ("monitoring caps", r"\bmonitoring\s+caps?\b"),
    ("monitored-system caps", r"\bmonitored[-\s]system\s+caps?\b"),
    ("start trial CTA", r"\bstart\s+(?:a\s+)?(?:14[-\s]day\s+)?(?:pro\s+)?trial\b"),
    ("free trial CTA", r"\bfree\s+trial\b"),
    ("hosted model credits", r"\b(?:ai|model)\s+credits?\b"),
    ("hosted Patrol quickstart", r"\bhosted\s+(?:patrol\s+)?quickstart\b"),
    ("bundled Patrol run allowance", r"\b25\s+runs,\s+no\s+api\s+key\b"),
    (
        "customer-specific Relay URL promise",
        r"\byourlab\.pulserelay\.pro\b|\ba\s+custom\s+url\b|\bcustom\s+url\s*\+|\+\s*custom\s+url\b|custom\s+url\s+should\b|custom\s+url\s*\([^)]*pulserelay",
    ),
)


PUBLIC_COPY_AUDIT_FILES: tuple[CopyAuditFileSpec, ...] = (
    CopyAuditFileSpec(
        repo="pulse",
        relative_path="docs/PULSE_PRO.md",
        required_patterns=(
            r"monitored[-\s]system volume is no longer the paid gate",
            r"Relay.*14[-\s]day history",
            r"Pro.*90[-\s]day history",
            r"Alert-triggered root-cause analysis",
            r"Safe remediation workflows",
        ),
    ),
    CopyAuditFileSpec(
        repo="pulse",
        relative_path="docs/AI.md",
        required_patterns=(
            r"Pulse Patrol is available to everyone on the Community plan with BYOK",
            r"Pro adds alert-triggered root-cause analysis and safe remediation workflows",
            r"Community and Relay installs can still run scheduled Patrol findings with BYOK",
        ),
    ),
    CopyAuditFileSpec(
        repo="pulse",
        relative_path="docs/UPGRADE_v6.md",
        required_patterns=(
            r"monitoring stays available across Community, Relay, and Pro",
            r"Relay raises history to 14 days",
            r"Pro raises it to 90 days",
            r"does not expose a general in-app trial",
        ),
    ),
    CopyAuditFileSpec(
        repo="pulse",
        relative_path="docs/architecture/v6-pricing-and-tiering.md",
        required_patterns=(
            r"monitored systems are not the paid\s+gate",
            r"Customer-specific Relay URL.*standard outbound relay service",
            r"Self-hosted trial acquisition.*No",
            r"Metrics history.*14 days",
            r"Metrics history.*90 days",
        ),
    ),
    CopyAuditFileSpec(
        repo="pulse",
        relative_path="frontend-modern/src/utils/selfHostedPlans.ts",
        required_patterns=(
            r"Self-hosted Pulse includes core monitoring for free",
            r"Remote web access, pairing, and push",
            r"14-day metric history",
            r"Root-cause analysis, safe remediation workflows, 90-day history",
            r"admin/reporting extras",
        ),
    ),
    CopyAuditFileSpec(
        repo="pulse-pro",
        relative_path="MONETIZATION.md",
        required_patterns=(
            r"Relay Tier.*14-day history",
            r"Pulse Relay.*secure remote access",
            r"No inbound firewall ports",
            r"Self-hosted Pro should be sold on root-cause analysis",
        ),
    ),
    CopyAuditFileSpec(
        repo="pulse-pro",
        relative_path="landing-page/index.html",
        required_patterns=(
            r"Community keeps core monitoring free",
            r"(?:Pulse Mobile|mobile app) pairing(?: for handoff)?, push notifications, and 14-day history",
            r"root-cause analysis, safe remediation workflows, and 90-day history",
            r"Do Relay or Pro charge by server\?",
            r"Existing Pulse Pro and legacy Pro\+ holders keep their paid runtime access",
        ),
    ),
    CopyAuditFileSpec(
        repo="pulse-pro",
        relative_path="license-server/public_pricing.go",
        required_patterns=(
            r"Community keeps core monitoring free",
            r"(?:Pulse Mobile|mobile app) pairing(?: for handoff)?, push notifications, and 14-day history",
            r"root-cause analysis, safe remediation workflows, team controls, and 90-day history",
            r"not as the self-hosted paid gate",
        ),
    ),
    CopyAuditFileSpec(
        repo="pulse-pro",
        relative_path="license-server/scripts/create-v6-stripe-prices.sh",
        required_patterns=(
            r"Pulse Relay",
            r"Pulse Mobile pairing",
            r"no inbound firewall ports",
            r"14-day history",
        ),
    ),
)


def default_pulse_dir() -> Path:
    return Path(__file__).resolve().parents[3]


def default_pulse_pro_license_server_dir() -> Path:
    return default_pulse_dir().parent / "pulse-pro" / "license-server"


def default_pulse_pro_relay_server_dir() -> Path:
    return default_pulse_dir().parent / "pulse-pro" / "relay-server"


def default_pulse_enterprise_dir() -> Path:
    return default_pulse_dir().parent / "pulse-enterprise"


def default_pulse_pro_dir(args: argparse.Namespace) -> Path:
    return Path(args.pulse_pro_license_server_dir).resolve().parent


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
    parser.add_argument(
        "--pulse-enterprise-dir",
        default=str(default_pulse_enterprise_dir()),
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
    pulse_enterprise_dir = Path(args.pulse_enterprise_dir).resolve()
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
            name="pulse-pro-admin-extras-api-contract",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./internal/api",
                "-run",
                "^(Test.*RBAC|Test.*Audit|Test.*Reporting|Test.*Report|Test.*SAML|Test.*SSO|Test.*AgentProfiles|Test.*ProfileSuggestions|Test.*ConfigProfiles|Test.*RequireLicenseFeature|TestRouter.*Reporting|TestRouter.*SSO|TestHandle.*Reporting|TestHandle.*Audit|TestHandle.*RBAC|TestHandle.*Profile|TestBind.*)$",
                "-count=1",
            ],
        ),
        CommandSpec(
            name="pulse-pro-admin-extras-core-packages",
            cwd=str(pulse_dir),
            command=[
                "go",
                "test",
                "./pkg/audit",
                "./pkg/auth",
                "./pkg/reporting",
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
                "src/utils/__tests__/cloudPlans.test.ts",
                "src/utils/__tests__/commercialBillingModel.test.ts",
                "src/utils/__tests__/licensePresentation.test.ts",
                "src/stores/__tests__/license.test.ts",
                "src/components/shared/__tests__/HistoryChart.test.tsx",
                "src/components/Settings/__tests__/ProLicensePanel.test.tsx",
                "src/components/Settings/__tests__/RelaySettingsPanel.runtime.test.tsx",
            ],
        ),
        CommandSpec(
            name="pulse-frontend-free-first-commercial-boundary",
            cwd=str(frontend_dir),
            command=[
                "npm",
                "test",
                "--",
                "src/stores/__tests__/sessionPresentationPolicy.test.ts",
                "src/__tests__/useAppRuntimeState.test.ts",
                "src/features/patrol/__tests__/patrolCommercialBoundary.test.ts",
                "src/pages/__tests__/AIIntelligence.test.tsx",
                "src/components/Alerts/__tests__/InvestigateAlertButton.test.tsx",
                "src/components/shared/__tests__/MonitoredSystemLimitWarningBanner.test.tsx",
                "src/components/SetupWizard/__tests__/SetupCompletionPanel.guardrails.test.ts",
                "src/pages/__tests__/PricingHandoff.test.tsx",
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
        CommandSpec(
            name="pulse-enterprise-pro-admin-extras-runtime",
            cwd=str(pulse_enterprise_dir),
            command=[
                "go",
                "test",
                "./internal/audit",
                "./internal/auditadmin",
                "./internal/rbac",
                "./internal/rbacadmin",
                "./internal/reportingadmin",
                "./internal/ssoadmin",
                "-count=1",
            ],
        ),
    ]


def copy_audit_file_path(args: argparse.Namespace, spec: CopyAuditFileSpec) -> Path:
    if spec.repo == "pulse":
        return Path(args.pulse_dir).resolve() / spec.relative_path
    if spec.repo == "pulse-pro":
        return default_pulse_pro_dir(args) / spec.relative_path
    raise ValueError(f"unknown copy audit repo {spec.repo!r}")


def first_matching_line(content: str, pattern: str) -> str:
    compiled = re.compile(pattern, flags=re.IGNORECASE)
    for line in content.splitlines():
        if compiled.search(line):
            return line.strip()
    return ""


def audit_public_paid_claim_copy(args: argparse.Namespace) -> list[str]:
    failures: list[str] = []
    for spec in PUBLIC_COPY_AUDIT_FILES:
        path = copy_audit_file_path(args, spec)
        label = f"{spec.repo}:{spec.relative_path}"
        if not path.exists():
            failures.append(f"{label} is missing")
            continue

        content = path.read_text(encoding="utf-8")
        for name, pattern in FORBIDDEN_PUBLIC_COPY_PATTERNS:
            matching_line = first_matching_line(content, pattern)
            if matching_line:
                failures.append(f"{label} contains forbidden {name!r}: {matching_line}")

        for pattern in spec.required_patterns:
            if not re.search(pattern, content, flags=re.IGNORECASE | re.DOTALL):
                failures.append(f"{label} is missing required paid-claim pattern: {pattern}")

    return failures


def run_public_paid_claim_copy_audit(args: argparse.Namespace) -> CommandResult:
    failures = audit_public_paid_claim_copy(args)
    if failures:
        detail = "; ".join(failures[:3])
        if len(failures) > 3:
            detail += f"; plus {len(failures) - 3} more"
        return CommandResult(
            name="pulse-public-paid-claim-copy-audit",
            cwd=str(Path(args.pulse_dir).resolve()),
            command=["static-copy-audit", "public-paid-claim-copy"],
            ok=False,
            exit_code=1,
            detail=detail,
        )
    return CommandResult(
        name="pulse-public-paid-claim-copy-audit",
        cwd=str(Path(args.pulse_dir).resolve()),
        command=["static-copy-audit", "public-paid-claim-copy"],
        ok=True,
        exit_code=0,
        detail=f"checked {len(PUBLIC_COPY_AUDIT_FILES)} public paid-claim files",
    )


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
    return [
        run_public_paid_claim_copy_audit(args),
        *[run_command(spec) for spec in build_command_specs(args)],
    ]


def render_markdown_report(title: str, results: list[CommandResult]) -> str:
    lines = [
        f"# {title}",
        "",
        "## Scope",
        "",
        "This proof checks that self-hosted paid claims map to concrete public copy, entitlement, API, UI, Pro admin-extra, license-server, and relay-server behavior, while ordinary self-hosted sessions stay free-first and non-promotional.",
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
