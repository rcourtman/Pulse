#!/usr/bin/env python3
"""Fail closed when GitHub Actions trust inputs become mutable or implicit."""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path


ACTION_SHA_RE = re.compile(r"^[0-9a-f]{40}$")
CONTAINER_DIGEST_RE = re.compile(r"^docker://.+@sha256:[0-9a-f]{64}$")
HOSTED_LATEST_RE = re.compile(r"\b(?:ubuntu|windows|macos)-latest\b")
USES_RE = re.compile(r"^\s*(?:-\s*)?uses:\s*([^\s#]+)")
RUN_RE = re.compile(r"^(\s*)(?:-\s*)?run:\s*(.*)$")
EXPRESSION_RE = re.compile(r"\$\{\{(.*?)\}\}")
# Workflow-call and dispatch inputs are data, not shell source. Secrets include
# github.token because Actions makes that credential available independently of
# an explicit secrets.GITHUB_TOKEN reference.
SHELL_DATA_CONTEXT_RE = re.compile(
    r"(?<![\w.])(?:inputs|secrets)\b|(?<![\w.])github\.token\b"
)
# GitHub documents these event fields as attacker-controlled strings. They may
# be passed through env, but interpolating them into a generated shell program
# lets quotes and shell metacharacters become code before the runner starts it.
UNTRUSTED_GITHUB_CONTEXT_RE = re.compile(
    r"(?<![\w.])github\.(?:"
    r"head_ref\b|ref\b|"
    r"event(?:\.[A-Za-z_][A-Za-z0-9_-]*)*\."
    r"(?:body|default_branch|email|head_branch|head_ref|label|message|name|page_name|ref|title)\b"
    r")"
)
SECRET_CONTEXT_RE = re.compile(
    r"(?<![\w.])secrets(?:"
    r"\.([A-Za-z_][A-Za-z0-9_]*)\b|"
    r"\[\s*(['\"])([A-Za-z_][A-Za-z0-9_]*)\2\s*\]"
    r")"
)
SECRET_CONTEXT_TOKEN_RE = re.compile(r"(?<![\w.])secrets\b")
# This value is intentionally public and only uses secret storage as a legacy
# configuration mechanism. Confidential credentials have no PR exception.
NON_CONFIDENTIAL_PULL_REQUEST_SECRETS = frozenset({"PULSE_LICENSE_PUBLIC_KEY"})
CHECKOUT_PREFIX = "actions/checkout@"
# v6.1.0 backports checkout's fail-closed fork-PR protection for privileged
# pull_request_target and workflow_run events. Keep this exact-pin allowlist
# reviewable: a dependency refresh must not silently discard that boundary.
PROTECTED_CHECKOUT_PINS = frozenset(
    {"d23441a48e516b6c34aea4fa41551a30e30af803"}
)
WRITE_CREDENTIAL_RATIONALE = "# required: authenticated git writes"
PERMISSIONS_RE = re.compile(r"^(\s*)permissions\s*:\s*(.*?)\s*$")
JOBS_RE = re.compile(r"^(\s*)jobs\s*:\s*$")
JOB_RE = re.compile(r"^(\s*)([A-Za-z0-9_-]+)\s*:\s*$")
RUNS_ON_RE = re.compile(r"^(\s*)runs-on\s*:")
TIMEOUT_RE = re.compile(r"^(\s*)timeout-minutes\s*:\s*(.*?)\s*$")


@dataclass(frozen=True)
class Finding:
    path: Path
    line: int
    message: str

    def render(self) -> str:
        return f"{self.path}:{self.line}: {self.message}"


def _indent(line: str) -> int:
    return len(line) - len(line.lstrip())


def _checkout_block(lines: list[str], uses_index: int) -> list[tuple[int, str]]:
    """Return lines belonging to the checkout step after its uses declaration."""
    uses_indent = _indent(lines[uses_index])
    block: list[tuple[int, str]] = []
    for index in range(uses_index + 1, len(lines)):
        line = lines[index]
        stripped = line.strip()
        if stripped and (
            _indent(line) < uses_indent
            or (_indent(line) == uses_indent and stripped.startswith("- "))
        ):
            break
        block.append((index, line))
    return block


def _run_script_lines(lines: list[str], run_index: int) -> list[tuple[int, str]]:
    """Return the source lines GitHub will materialize as a run script."""
    match = RUN_RE.match(lines[run_index])
    if not match:
        return []
    run_indent = len(match.group(1))
    value = match.group(2).strip()
    if value not in {"|", "|-", "|+", ">", ">-", ">+"}:
        return [(run_index, match.group(2))]

    script: list[tuple[int, str]] = []
    for index in range(run_index + 1, len(lines)):
        line = lines[index]
        if line.strip() and _indent(line) <= run_indent:
            break
        script.append((index, line))
    return script


def _has_trigger(lines: list[str], event: str) -> bool:
    """Return whether the top-level Actions trigger includes *event*."""
    event_re = re.compile(rf"(?<![\w-]){re.escape(event)}(?![\w-])")
    for index, line in enumerate(lines):
        code = line.split("#", 1)[0]
        match = re.match(r"^on\s*:\s*(.*)$", code)
        if not match:
            continue
        inline = match.group(1).strip()
        if inline:
            return bool(event_re.search(inline))
        for trigger_line in lines[index + 1 :]:
            trigger_code = trigger_line.split("#", 1)[0]
            if not trigger_code.strip():
                continue
            if _indent(trigger_code) == 0:
                break
            if re.match(rf"^\s+{re.escape(event)}\s*:", trigger_code):
                return True
        return False
    return False


def _audit_runner_job_timeouts(path: Path, lines: list[str]) -> list[Finding]:
    """Require each locally executed job to declare one bounded time budget."""
    findings: list[Finding] = []
    jobs_index = next(
        (
            index
            for index, line in enumerate(lines)
            if JOBS_RE.match(line.split("#", 1)[0])
        ),
        None,
    )
    if jobs_index is None:
        return findings

    jobs_indent = _indent(lines[jobs_index])
    job_starts: list[tuple[int, int]] = []
    for index in range(jobs_index + 1, len(lines)):
        code = lines[index].split("#", 1)[0]
        if code.strip() and _indent(code) <= jobs_indent:
            break
        match = JOB_RE.match(code)
        if match and len(match.group(1)) == jobs_indent + 2:
            job_starts.append((index, len(match.group(1))))

    for position, (job_index, job_indent) in enumerate(job_starts):
        end_index = (
            job_starts[position + 1][0]
            if position + 1 < len(job_starts)
            else len(lines)
        )
        direct_indent = job_indent + 2
        runner_lines: list[int] = []
        timeout_declarations: list[tuple[int, str]] = []
        for index in range(job_index + 1, end_index):
            code = lines[index].split("#", 1)[0]
            run_match = RUNS_ON_RE.match(code)
            if run_match and len(run_match.group(1)) == direct_indent:
                runner_lines.append(index)
            timeout_match = TIMEOUT_RE.match(code)
            if timeout_match and len(timeout_match.group(1)) == direct_indent:
                timeout_declarations.append(
                    (index, timeout_match.group(2).strip())
                )

        # Reusable-workflow caller jobs have `uses` instead of `runs-on` and
        # cannot declare timeout-minutes. The called workflow owns its budgets.
        if not runner_lines:
            continue
        if len(timeout_declarations) != 1:
            findings.append(
                Finding(
                    path,
                    runner_lines[0] + 1,
                    "runner job must declare explicit timeout-minutes exactly once",
                )
            )
            continue

        timeout_index, timeout_value = timeout_declarations[0]
        if not timeout_value.isdigit() or not 1 <= int(timeout_value) <= 360:
            findings.append(
                Finding(
                    path,
                    timeout_index + 1,
                    "runner job timeout-minutes must be a literal integer from 1 through 360",
                )
            )

    return findings


def audit_workflow(path: Path) -> list[Finding]:
    lines = path.read_text(encoding="utf-8").splitlines()
    findings = _audit_runner_job_timeouts(path, lines)

    if _has_trigger(lines, "pull_request_target"):
        findings.append(
            Finding(
                path,
                1,
                "pull_request_target is prohibited; use pull_request or isolate "
                "privileged work from pull-request code",
            )
        )

    if _has_trigger(lines, "pull_request"):
        for index, line in enumerate(lines):
            code = line.split("#", 1)[0]
            expressions = EXPRESSION_RE.findall(code)
            secret_names: set[str] = set()
            has_unresolved_secret_reference = False
            for expression in expressions:
                static_references = list(SECRET_CONTEXT_RE.finditer(expression))
                secret_names.update(
                    match.group(1) or match.group(3) for match in static_references
                )
                if len(SECRET_CONTEXT_TOKEN_RE.findall(expression)) != len(
                    static_references
                ):
                    # Whole-context and dynamic references can expose any
                    # repository secret, so they cannot use the public-key
                    # exception reserved for a statically named value.
                    has_unresolved_secret_reference = True
            if (
                secret_names - NON_CONFIDENTIAL_PULL_REQUEST_SECRETS
                or has_unresolved_secret_reference
            ):
                findings.append(
                    Finding(
                        path,
                        index + 1,
                        "pull_request workflows must not reference confidential "
                        "repository secrets; isolate privileged work in a non-PR workflow",
                    )
                )

    permission_declarations = [
        (index, match)
        for index, line in enumerate(lines)
        if (match := PERMISSIONS_RE.match(line.split("#", 1)[0]))
    ]
    top_level_permissions = [
        (index, match)
        for index, match in permission_declarations
        if not match.group(1)
    ]
    if len(top_level_permissions) != 1:
        findings.append(
            Finding(
                path,
                1,
                "workflow must declare top-level permissions explicitly exactly once",
            )
        )
    for index, match in permission_declarations:
        # GitHub applies job-level permissions after the workflow default. Audit
        # every declaration so a job cannot reintroduce read-all, write-all, or
        # a dynamic grant beneath an otherwise least-privilege workflow.
        inline_value = match.group(2)
        if inline_value not in {"", "{}"}:
            scope = "workflow" if not match.group(1) else "job"
            findings.append(
                Finding(
                    path,
                    index + 1,
                    f"{scope} permissions must use a scope mapping or explicit empty mapping",
                )
            )

    for index, line in enumerate(lines):
        line_number = index + 1
        code = line.split("#", 1)[0]
        if HOSTED_LATEST_RE.search(code):
            findings.append(
                Finding(
                    path,
                    line_number,
                    "mutable hosted runner label; use an explicit dated image",
                )
            )

        if RUN_RE.match(code):
            for script_index, script_line in _run_script_lines(lines, index):
                for expression in EXPRESSION_RE.findall(script_line):
                    if SHELL_DATA_CONTEXT_RE.search(expression):
                        findings.append(
                            Finding(
                                path,
                                script_index + 1,
                                "workflow inputs and secrets must enter run scripts through env",
                            )
                        )
                    elif UNTRUSTED_GITHUB_CONTEXT_RE.search(expression):
                        findings.append(
                            Finding(
                                path,
                                script_index + 1,
                                "untrusted GitHub metadata must enter run scripts through env",
                            )
                        )

        match = USES_RE.search(code)
        if not match:
            continue
        dependency = match.group(1).strip("'\"")
        if dependency.startswith("./"):
            continue
        if dependency.startswith("docker://"):
            if not CONTAINER_DIGEST_RE.fullmatch(dependency):
                findings.append(
                    Finding(
                        path,
                        line_number,
                        "container action is not pinned to a sha256 digest",
                    )
                )
            continue

        owner_and_action, separator, ref = dependency.rpartition("@")
        if not separator or "/" not in owner_and_action or not ACTION_SHA_RE.fullmatch(ref):
            findings.append(
                Finding(
                    path,
                    line_number,
                    "remote action or reusable workflow is not pinned to a full commit SHA",
                )
            )
            continue

        # GitHub repository names are case-insensitive, so normalize before
        # applying checkout-specific credential controls.
        if not dependency.lower().startswith(CHECKOUT_PREFIX):
            continue
        if ref not in PROTECTED_CHECKOUT_PINS:
            findings.append(
                Finding(
                    path,
                    line_number,
                    "checkout pin is outside the reviewed privileged-event "
                    "protection baseline",
                )
            )
        checkout_block = _checkout_block(lines, index)
        unsafe_pr_settings = [
            (block_index, block_line)
            for block_index, block_line in checkout_block
            if re.match(r"^\s*allow-unsafe-pr-checkout\s*:", block_line)
        ]
        for setting_index, setting in unsafe_pr_settings:
            if not re.match(
                r"^\s*allow-unsafe-pr-checkout\s*:\s*false(?:\s|$)", setting
            ):
                findings.append(
                    Finding(
                        path,
                        setting_index + 1,
                        "checkout must not opt out of privileged-event PR protection",
                    )
                )
        credential_settings = [
            (block_index, block_line)
            for block_index, block_line in checkout_block
            if re.match(r"^\s*persist-credentials\s*:", block_line)
        ]
        if len(credential_settings) != 1:
            findings.append(
                Finding(
                    path,
                    line_number,
                    "checkout must set persist-credentials explicitly exactly once",
                )
            )
            continue

        setting_index, setting = credential_settings[0]
        value_match = re.match(
            r"^\s*persist-credentials\s*:\s*(true|false)\b", setting
        )
        if not value_match:
            findings.append(
                Finding(
                    path,
                    setting_index + 1,
                    "persist-credentials must be the literal true or false",
                )
            )
        elif value_match.group(1) == "true" and WRITE_CREDENTIAL_RATIONALE not in setting:
            findings.append(
                Finding(
                    path,
                    setting_index + 1,
                    f"persisted checkout credentials require {WRITE_CREDENTIAL_RATIONALE}",
                )
            )

    return findings


def audit_directory(workflow_directory: Path) -> list[Finding]:
    findings: list[Finding] = []
    paths = sorted(workflow_directory.glob("*.yml")) + sorted(
        workflow_directory.glob("*.yaml")
    )
    for path in paths:
        findings.extend(audit_workflow(path))
    return findings


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "workflow_directory",
        nargs="?",
        type=Path,
        default=Path(__file__).resolve().parents[1] / ".github" / "workflows",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not args.workflow_directory.is_dir():
        print(f"workflow directory not found: {args.workflow_directory}", file=sys.stderr)
        return 2
    findings = audit_directory(args.workflow_directory)
    if findings:
        for finding in findings:
            print(finding.render(), file=sys.stderr)
        print(f"GitHub Actions trust validation failed ({len(findings)} finding(s)).", file=sys.stderr)
        return 1
    print("GitHub Actions trust validation passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
