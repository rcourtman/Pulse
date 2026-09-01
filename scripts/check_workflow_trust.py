#!/usr/bin/env python3
"""Fail closed when GitHub Actions trust inputs become mutable or implicit."""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path


def _yaml_key(name: str) -> str:
    """Return a pattern for equivalent plain, single-, or double-quoted keys."""
    escaped = re.escape(name)
    # YAML double-quoted scalars can spell any ASCII character through \x, \u,
    # or \U escapes. GitHub decodes those spellings before interpreting the
    # workflow, so the trust audit must do the same lexical matching. Without
    # this, e.g. "permi\x73sions" or "u\u0073es" bypasses the corresponding
    # permissions and dependency checks while remaining an ordinary Actions
    # key. The security-relevant key vocabulary is ASCII, so matching each
    # character's three exact escaped forms is sufficient and avoids a YAML
    # parser dependency in this early validation script.
    double_quoted = "".join(
        rf"(?:{re.escape(character)}|"
        rf"(?i:\\(?:x{ord(character):02x}|u{ord(character):04x}|"
        rf"U{ord(character):08x})))"
        for character in name
    )
    return rf'(?:{escaped}|"{double_quoted}"|\'{escaped}\')'


ACTION_SHA_RE = re.compile(r"^[0-9a-f]{40}$")
CONTAINER_DIGEST_RE = re.compile(r"^docker://.+@sha256:[0-9a-f]{64}$")
HOSTED_LATEST_RE = re.compile(r"\b(?:ubuntu|windows|macos)-latest\b")
USES_RE = re.compile(
    rf"^\s*(?:-\s*)?{_yaml_key('uses')}\s*:\s*([^\s#]+)"
)
RUN_RE = re.compile(rf"^(\s*)(?:-\s*)?{_yaml_key('run')}\s*:\s*(.*)$")
ENV_RE = re.compile(rf"^(\s*)(?:-\s*)?{_yaml_key('env')}\s*:\s*$")
ENV_ENTRY_RE = re.compile(r"^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*?)\s*$")
EXPRESSION_RE = re.compile(r"\$\{\{(.*?)\}\}")
GITHUB_COMMAND_FILE_RE = re.compile(r"\bGITHUB_(?:ENV|OUTPUT|PATH|STATE)\b")
SHELL_ASSIGNMENT_RE = re.compile(
    r"^\s*(?:(?:export|local|readonly)\s+|declare(?:\s+-[A-Za-z]+)?\s+)?"
    r"([A-Za-z_][A-Za-z0-9_]*)\s*="
)
POWERSHELL_ASSIGNMENT_RE = re.compile(
    r"^\s*(?:\[[^\]\r\n]+\]\s*)?\$([A-Za-z_][A-Za-z0-9_]*)\s*="
)
# A trusted reassignment only clears possible taint when it is guaranteed to
# execute. Assignments inside these Bash compound commands affect one branch or
# iteration, so a later command can still observe the original workflow value.
BASH_CONTROL_OPEN_RE = re.compile(
    r"^\s*(?:if|case|for|select|while|until)\b"
)
BASH_CONTROL_CLOSE_RE = re.compile(r"^\s*(?:fi|esac|done)\b")
# Workflow-call and dispatch inputs are data, not shell source. The legacy
# github.event.inputs alias is identical data, and repository_dispatch callers
# fully control client_payload. Step and job outputs are data too: they can
# carry event or input values across an otherwise-safe intermediate step.
# Whole github contexts are unsafe because they include event data (and the
# github context includes github.token). Secrets include github.token because
# Actions makes that credential available independently of an explicit
# secrets.GITHUB_TOKEN reference.
SHELL_DATA_CONTEXT_RE = re.compile(
    r"(?<![\w.])(?:inputs|secrets)\b|"
    r"(?<![\w.])github(?:\.token\b|\[\s*['\"]token['\"]\s*\])|"
    r"(?<![\w.])github\b(?:\.event\b|\[\s*['\"]event['\"]\s*\])"
    r"(?:\.(?:inputs|client_payload)\b|"
    r"\[\s*['\"](?:inputs|client_payload)['\"]\s*\])|"
    r"(?<![\w.])github\b(?:\.event\b|\[\s*['\"]event['\"]\s*\])?"
    r"(?!\s*(?:\.|\[))|"
    r"(?<![\w.])(?:steps|needs)\b"
    r"(?=[^}\n]*(?:\.outputs\b|\[\s*['\"]outputs['\"]\s*\]))"
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
# A workflow_run handler executes from the default branch with privileged
# context. It must not replace that trusted checkout with code identified by
# the triggering run, even when the upstream workflow name looks familiar.
WORKFLOW_RUN_CODE_REF_RE = re.compile(
    r"(?<![\w.])github\.event\.workflow_run\."
    r"(?:head_branch|head_repository|head_sha|pull_requests)\b"
)
# A canonical checkout is not enough if a later step reacquires code or an
# artifact from a less-trusted run. GitHub calls out artifact downloads and
# command-line PR fetches as equivalent privileged-workflow ingress paths.
WORKFLOW_RUN_ARTIFACT_ACTION_PREFIX = "actions/download-artifact@"
WORKFLOW_RUN_INGRESS_COMMAND_RE = re.compile(
    r"(?:"
    r"\bgh\s+(?:pr\s+checkout|run\s+download)\b|"
    r"\bgit\s+(?:clone|fetch|pull)\b|"
    r"/actions/artifacts/[^/\s'\"]+/zip(?:[?'\"\s]|$)|"
    r"refs/pull/"
    r")",
    re.IGNORECASE,
)
SECRET_CONTEXT_RE = re.compile(
    r"(?<![\w.])secrets(?:"
    r"\.([A-Za-z_][A-Za-z0-9_]*)\b|"
    r"\[\s*(['\"])([A-Za-z_][A-Za-z0-9_]*)\2\s*\]"
    r")"
)
SECRET_CONTEXT_TOKEN_RE = re.compile(r"(?<![\w.])secrets\b")
CACHE_ACTION_PREFIXES = ("actions/cache@", "actions/cache/")
SETUP_CACHE_ACTION_PREFIXES = (
    "actions/setup-go@",
    "actions/setup-node@",
    "actions/setup-python@",
    "gradle/actions/setup-gradle@",
    "ruby/setup-ruby@",
)
AUTO_CACHE_DISABLE_INPUTS = {
    "actions/setup-go@": "cache",
    "actions/setup-node@": "package-manager-cache",
}
CACHE_INPUT_RE = re.compile(
    rf"^\s*{_yaml_key('cache')}\s*:\s*(.*?)\s*$", re.IGNORECASE
)
EXTERNAL_CACHE_INPUT_RE = re.compile(
    rf"^\s*(?:{_yaml_key('cache-from')}|{_yaml_key('cache-to')})\s*:",
    re.IGNORECASE,
)
WRITE_PERMISSION_RE = re.compile(
    r'''^\s+(?:[A-Za-z-]+|"[A-Za-z-]+"|'[A-Za-z-]+')\s*:\s*'''
    r'''(?:write|"write"|'write')\s*$'''
)
# This value is intentionally public and only uses secret storage as a legacy
# configuration mechanism. Confidential credentials have no PR exception.
NON_CONFIDENTIAL_PULL_REQUEST_SECRETS = frozenset({"PULSE_LICENSE_PUBLIC_KEY"})
CHECKOUT_PREFIX = "actions/checkout@"
# v7.0.1 includes checkout's fail-closed fork-PR protection for privileged
# pull_request_target and workflow_run events. Keep this exact-pin allowlist
# reviewable: a dependency refresh must not silently discard that boundary.
PROTECTED_CHECKOUT_PINS = frozenset(
    {"3d3c42e5aac5ba805825da76410c181273ba90b1"}
)
WRITE_CREDENTIAL_RATIONALE = "# required: authenticated git writes"
PERMISSIONS_RE = re.compile(
    rf"^(\s*){_yaml_key('permissions')}\s*:\s*(.*?)\s*$"
)
JOBS_RE = re.compile(rf"^(\s*){_yaml_key('jobs')}\s*:\s*$")
JOB_RE = re.compile(
    r'''^(\s*)(?:[A-Za-z0-9_-]+|"[A-Za-z0-9_-]+"|'[A-Za-z0-9_-]+')\s*:\s*$'''
)
RUNS_ON_RE = re.compile(rf"^(\s*){_yaml_key('runs-on')}\s*:")
TIMEOUT_RE = re.compile(
    rf"^(\s*){_yaml_key('timeout-minutes')}\s*:\s*(.*?)\s*$"
)
EXPLICIT_MAPPING_KEY_RE = re.compile(r"^\s*(?:-\s*)?\?\s")
ESCAPED_MAPPING_KEY_RE = re.compile(
    r'''^\s*(?:-\s*)?"(?:[^"\\]|\\.)*\\(?:[^"\\]|\\.)*"\s*:'''
)
YAML_PROPERTY_PATTERN = (
    r"(?:[&*][^\s,\[\]{}]+|!(?:<[^>\r\n]+>|[^\s,\[\]{}]+))"
)
LEADING_YAML_PROPERTY_RE = re.compile(
    rf"^\s*(?:(?:---|-)[ \t]+)?{YAML_PROPERTY_PATTERN}(?:\s|$)"
)
YAML_VALUE_PROPERTY_RE = re.compile(
    r'''^\s*(?:-\s*)?(?:[A-Za-z0-9_.-]+|"[^"]+"|'[^']+')'''
    rf"\s*:\s*{YAML_PROPERTY_PATTERN}(?:\s|$)"
)
FLOW_YAML_PROPERTY_RE = re.compile(
    r"(?:[\[,]\s*|(?<![$\{])\{\s*|:\s*)"
    + YAML_PROPERTY_PATTERN
    + r"(?=\s|$|[,\]}])"
)
NONEMPTY_FLOW_MAPPING_RE = re.compile(
    r'''^\s*(?:-\s*)?(?:[A-Za-z0-9_.-]+|"[^"]+"|'[^']+')'''
    r"\s*:\s*\{(?!\s*\}\s*$)"
)
# OIDC-backed delivery identity is only trusted when GitHub owns the runner
# lifecycle. Keep the accepted image labels explicit and reviewable so a job
# cannot move to persistent or dynamically selected compute without changing
# this contract. These are the hosted images used by Pulse's attestation jobs.
TRUSTED_OIDC_RUNNER_LABELS = frozenset(
    {"ubuntu-24.04", "windows-2025", "macos-15"}
)


@dataclass(frozen=True)
class Finding:
    path: Path
    line: int
    message: str

    def render(self) -> str:
        return f"{self.path}:{self.line}: {self.message}"


def _indent(line: str) -> int:
    return len(line) - len(line.lstrip())


def _direct_mapping_indent(
    lines: list[str], parent_index: int, end_index: int | None = None
) -> int | None:
    """Return the indentation of a block mapping's direct children."""
    parent_indent = _indent(lines[parent_index])
    limit = len(lines) if end_index is None else end_index
    for line in lines[parent_index + 1 : limit]:
        code = line.split("#", 1)[0]
        if not code.strip():
            continue
        indent = _indent(code)
        if indent <= parent_indent:
            return None
        # YAML permits any consistent indentation width. The first content in
        # a block mapping establishes its direct-child indentation; assuming
        # two spaces here lets a valid, more deeply indented workflow evade
        # job-scoped trust checks.
        return indent
    return None


def _mapping_end_index(lines: list[str], parent_index: int) -> int:
    """Return the first line after a YAML block mapping."""
    parent_indent = _indent(lines[parent_index])
    for index in range(parent_index + 1, len(lines)):
        code = lines[index].split("#", 1)[0]
        if code.strip() and _indent(code) <= parent_indent:
            return index
    return len(lines)


def _permission_mapping_has_write(
    lines: list[str], permissions_index: int, end_index: int | None = None
) -> bool:
    """Return whether a permissions block grants any literal write scope."""
    match = PERMISSIONS_RE.match(lines[permissions_index].split("#", 1)[0])
    if not match:
        return False
    inline = match.group(2).strip().strip("'\"").lower()
    if inline:
        # Broad inline grants are rejected separately, but classify them as
        # privileged too so cache isolation remains fail-closed in one run.
        return inline == "write-all"

    child_indent = _direct_mapping_indent(lines, permissions_index, end_index)
    if child_indent is None:
        return False
    limit = len(lines) if end_index is None else end_index
    for line in lines[permissions_index + 1 : limit]:
        code = line.split("#", 1)[0]
        if not code.strip():
            continue
        indent = _indent(code)
        if indent <= _indent(lines[permissions_index]):
            break
        if indent == child_indent and WRITE_PERMISSION_RE.match(code):
            return True
    return False


def _permission_mapping_grants_scope_write(
    lines: list[str],
    permissions_index: int,
    scope: str,
    end_index: int | None = None,
) -> bool:
    """Return whether a permissions block grants literal write to *scope*."""
    match = PERMISSIONS_RE.match(lines[permissions_index].split("#", 1)[0])
    if not match:
        return False
    inline = match.group(2).strip().strip("'\"").lower()
    if inline:
        return inline == "write-all"

    child_indent = _direct_mapping_indent(lines, permissions_index, end_index)
    if child_indent is None:
        return False
    scope_write_re = re.compile(
        rf"^\s+{_yaml_key(scope)}\s*:\s*(?:write|\"write\"|'write')\s*$"
    )
    limit = len(lines) if end_index is None else end_index
    for line in lines[permissions_index + 1 : limit]:
        code = line.split("#", 1)[0]
        if not code.strip():
            continue
        indent = _indent(code)
        if indent <= _indent(lines[permissions_index]):
            break
        if indent == child_indent and scope_write_re.match(code):
            return True
    return False


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


def _runner_job_ranges(lines: list[str]) -> list[tuple[int, int, int]]:
    """Return (start, end, indent) for each locally executed workflow job."""
    jobs_index = next(
        (
            index
            for index, line in enumerate(lines)
            if JOBS_RE.match(line.split("#", 1)[0])
        ),
        None,
    )
    if jobs_index is None:
        return []

    jobs_end = _mapping_end_index(lines, jobs_index)

    job_indent = _direct_mapping_indent(lines, jobs_index, jobs_end)
    if job_indent is None:
        return []
    job_starts: list[tuple[int, int]] = []
    for index in range(jobs_index + 1, jobs_end):
        code = lines[index].split("#", 1)[0]
        match = JOB_RE.match(code)
        if match and len(match.group(1)) == job_indent:
            job_starts.append((index, len(match.group(1))))

    return [
        (
            job_index,
            job_starts[position + 1][0]
            if position + 1 < len(job_starts)
            else jobs_end,
            job_indent,
        )
        for position, (job_index, job_indent) in enumerate(job_starts)
    ]


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


def _audit_yaml_trust_shape(path: Path, lines: list[str]) -> list[Finding]:
    """Reject YAML forms whose expansion can hide workflow trust structure."""
    findings: list[Finding] = []
    script_indexes = {
        script_index
        for index, line in enumerate(lines)
        if RUN_RE.match(line.split("#", 1)[0])
        for script_index, _ in _run_script_lines(lines, index)
        if script_index != index
    }
    structural_block_re = re.compile(
        rf"^\s*(?:{_yaml_key('jobs')}|{_yaml_key('steps')})\s*:\s*(.*?)\s*$"
    )

    for index, line in enumerate(lines):
        if index in script_indexes:
            continue
        code = line.split("#", 1)[0]
        if not code.strip():
            continue
        if EXPLICIT_MAPPING_KEY_RE.match(code):
            findings.append(
                Finding(
                    path,
                    index + 1,
                    "explicit YAML mapping keys are prohibited because they can "
                    "hide workflow trust fields",
                )
            )
        if ESCAPED_MAPPING_KEY_RE.match(code):
            findings.append(
                Finding(
                    path,
                    index + 1,
                    "escaped YAML mapping keys are prohibited; use the canonical "
                    "literal key spelling",
                )
            )
        if (
            LEADING_YAML_PROPERTY_RE.match(code)
            or YAML_VALUE_PROPERTY_RE.search(code)
            or FLOW_YAML_PROPERTY_RE.search(code)
        ):
            findings.append(
                Finding(
                    path,
                    index + 1,
                    "YAML anchors, aliases, and tags are prohibited because expanded "
                    "nodes can bypass lexical workflow trust checks",
                )
            )
        structural_match = structural_block_re.match(code)
        if structural_match and structural_match.group(1):
            findings.append(
                Finding(
                    path,
                    index + 1,
                    "jobs and steps must use block mappings and sequences so every "
                    "trust-bearing field is directly auditable",
                )
            )
        if NONEMPTY_FLOW_MAPPING_RE.match(code) or re.match(r"^\s*-\s*\{", code):
            findings.append(
                Finding(
                    path,
                    index + 1,
                    "non-empty flow mappings are prohibited because they can hide "
                    "workflow trust fields on one line",
                )
            )

    jobs_index = next(
        (
            index
            for index, line in enumerate(lines)
            if JOBS_RE.match(line.split("#", 1)[0])
        ),
        None,
    )
    if jobs_index is not None:
        jobs_end = _mapping_end_index(lines, jobs_index)
        job_indent = _direct_mapping_indent(lines, jobs_index, jobs_end)
        if job_indent is not None:
            for index in range(jobs_index + 1, jobs_end):
                code = lines[index].split("#", 1)[0]
                if (
                    code.strip()
                    and _indent(code) == job_indent
                    and not JOB_RE.match(code)
                ):
                    findings.append(
                        Finding(
                            path,
                            index + 1,
                            "each job must use a canonical literal ID and block mapping",
                        )
                    )

    return findings


def _step_env_bindings(lines: list[str], run_index: int) -> dict[str, str]:
    """Return literal step env names and values for a run declaration."""
    run_match = RUN_RE.match(lines[run_index])
    if not run_match:
        return {}
    field_indent = len(run_match.group(1))

    step_start = run_index
    for index in range(run_index - 1, -1, -1):
        line = lines[index]
        if (
            line.strip()
            and _indent(line) == field_indent - 2
            and line.lstrip().startswith("- ")
        ):
            step_start = index
            break

    step_end = len(lines)
    for index in range(run_index + 1, len(lines)):
        line = lines[index]
        if (
            line.strip()
            and _indent(line) == field_indent - 2
            and line.lstrip().startswith("- ")
        ):
            step_end = index
            break

    bindings: dict[str, str] = {}

    # Job-level env is inherited by every run step. Locate the enclosing job
    # from normal Actions indentation before applying step-level overrides.
    job_key_indent = field_indent - 6
    job_start: int | None = None
    for index in range(step_start - 1, -1, -1):
        code = lines[index].split("#", 1)[0]
        if _indent(code) == job_key_indent and JOB_RE.match(code):
            job_start = index
            break
    if job_start is not None:
        job_end = len(lines)
        for index in range(job_start + 1, len(lines)):
            code = lines[index].split("#", 1)[0]
            if code.strip() and _indent(code) == job_key_indent and JOB_RE.match(code):
                job_end = index
                break
        job_field_indent = field_indent - 4
        for index in range(job_start + 1, job_end):
            code = lines[index].split("#", 1)[0]
            match = ENV_RE.match(code)
            if not match or len(match.group(1)) != job_field_indent:
                continue
            for env_line in lines[index + 1 : job_end]:
                env_code = env_line.split("#", 1)[0]
                if not env_code.strip():
                    continue
                if _indent(env_code) <= job_field_indent:
                    break
                entry = ENV_ENTRY_RE.match(env_code)
                if entry and _indent(env_code) == job_field_indent + 2:
                    bindings[entry.group(1)] = entry.group(2).strip("'\"")
            break

    for index in range(step_start, step_end):
        code = lines[index].split("#", 1)[0]
        match = ENV_RE.match(code)
        mapping_indent = len(match.group(1)) if match else -1
        inline_step_field = bool(match and code.lstrip().startswith("- "))
        if not match or not (
            mapping_indent == field_indent
            or (inline_step_field and mapping_indent + 2 == field_indent)
        ):
            continue
        for env_line in lines[index + 1 : step_end]:
            code = env_line.split("#", 1)[0]
            if not code.strip():
                continue
            if _indent(code) <= field_indent:
                break
            entry = ENV_ENTRY_RE.match(code)
            if entry and _indent(code) == field_indent + 2:
                bindings[entry.group(1)] = entry.group(2).strip("'\"")
        break
    return bindings


def _shell_variable_reference(line: str, name: str) -> bool:
    escaped = re.escape(name)
    return bool(
        re.search(
            rf"(?:\$\{{{escaped}(?=[^A-Za-z0-9_])|\${escaped}\b|"
            rf"\$env:{escaped}\b|\$\{{env:{escaped}\}})",
            line,
            re.IGNORECASE,
        )
    )


def _bash_assignment_persists(line: str, assignment: re.Match[str]) -> bool:
    """Return whether a Bash assignment changes the current shell."""
    declaration = line[: assignment.start(1)].strip()
    if declaration:
        return True

    quote = ""
    escaped = False
    parentheses = 0
    braces = 0
    value = line[assignment.end() :]
    for index, character in enumerate(value):
        if escaped:
            escaped = False
            continue
        if character == "\\" and quote != "'":
            escaped = True
            continue
        if quote:
            if character == quote:
                quote = ""
            continue
        if character in "'\"`":
            quote = character
            continue
        if character == "(":
            parentheses += 1
            continue
        if character == ")" and parentheses:
            parentheses -= 1
            continue
        if character == "{" and (braces or (index and value[index - 1] == "$")):
            braces += 1
            continue
        if character == "}" and braces:
            braces -= 1
            continue
        if not parentheses and not braces:
            if character == ";":
                return True
            if character in "|&":
                return False
            if character.isspace():
                remainder = value[index:].strip()
                return not remainder or remainder.startswith("#")

    # A bare assignment persists. On an incomplete quoted or nested value,
    # retain taint rather than treating malformed shell as validation.
    return not quote and not parentheses and not braces and not escaped


def _is_untrusted_expression(value: str) -> bool:
    return any(
        SHELL_DATA_CONTEXT_RE.search(expression)
        or UNTRUSTED_GITHUB_CONTEXT_RE.search(expression)
        for expression in EXPRESSION_RE.findall(value)
    )


def _has_confidential_secret_reference(lines: list[str]) -> bool:
    """Return whether workflow lines can resolve a confidential secret."""
    for line in lines:
        for expression in EXPRESSION_RE.findall(line.split("#", 1)[0]):
            static_references = list(SECRET_CONTEXT_RE.finditer(expression))
            secret_names = {
                match.group(1) or match.group(3) for match in static_references
            }
            if secret_names - NON_CONFIDENTIAL_PULL_REQUEST_SECRETS:
                return True
            if len(SECRET_CONTEXT_TOKEN_RE.findall(expression)) != len(
                static_references
            ):
                return True
    return False


def _audit_command_file_data(
    path: Path,
    lines: list[str],
    run_index: int,
) -> list[Finding]:
    """Keep raw workflow/event values out of GitHub runner command files."""
    findings: list[Finding] = []
    bindings = {
        name: value
        for name, value in _step_env_bindings(lines, run_index).items()
        if _is_untrusted_expression(value)
    }
    unsafe_names = set(bindings)
    bash_control_depth = 0

    for script_index, script_line in _run_script_lines(lines, run_index):
        if BASH_CONTROL_CLOSE_RE.match(script_line):
            bash_control_depth = max(0, bash_control_depth - 1)
        opens_bash_control = bool(BASH_CONTROL_OPEN_RE.match(script_line))
        assignment = SHELL_ASSIGNMENT_RE.match(script_line)
        powershell_assignment = False
        if assignment is None:
            assignment = POWERSHELL_ASSIGNMENT_RE.match(script_line)
            powershell_assignment = assignment is not None
        if assignment:
            assigned_name = assignment.group(1)
            assignment_value = script_line[assignment.end() :]
            if (
                "GITHUB_EVENT_PATH" in assignment_value
                or any(
                    _shell_variable_reference(assignment_value, name)
                    for name in unsafe_names
                )
            ):
                unsafe_names.add(assigned_name)
            else:
                # A later literal or trusted assignment replaces the prior
                # value. Keeping stale taint would hide real findings in noise
                # and encourage suppressions around the policy.
                if powershell_assignment:
                    unsafe_names = {
                        name
                        for name in unsafe_names
                        if name.casefold() != assigned_name.casefold()
                    }
                elif (
                    bash_control_depth == 0
                    and _bash_assignment_persists(script_line, assignment)
                ):
                    unsafe_names.discard(assigned_name)

        if opens_bash_control:
            bash_control_depth += 1
        if not GITHUB_COMMAND_FILE_RE.search(script_line):
            continue
        referenced_unsafe_names = sorted(
            name
            for name in unsafe_names
            if _shell_variable_reference(script_line, name)
        )
        if referenced_unsafe_names:
            findings.append(
                Finding(
                    path,
                    script_index + 1,
                    "untrusted workflow data must be validated or encoded "
                    "before writing to GitHub command files "
                    f"({', '.join(referenced_unsafe_names)})",
                )
            )
    return findings


def _has_trigger(lines: list[str], event: str) -> bool:
    """Return whether the top-level Actions trigger includes *event*."""
    event_re = re.compile(rf"(?<![\w-]){re.escape(event)}(?![\w-])")
    for index, line in enumerate(lines):
        code = line.split("#", 1)[0]
        match = re.match(rf"^{_yaml_key('on')}\s*:\s*(.*)$", code)
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
            if re.match(rf"^\s+{_yaml_key(event)}\s*:", trigger_code):
                return True
        return False
    return False


def _static_yaml_list(
    lines: list[str], key_index: int, inline_value: str
) -> list[str] | None:
    """Parse the small literal YAML string-list subset used by trust policy."""
    values: list[str] = []
    if inline_value:
        match = re.fullmatch(r"\[\s*(.*?)\s*\]", inline_value)
        if not match:
            return None
        raw_values = [] if not match.group(1) else match.group(1).split(",")
    else:
        key_indent = _indent(lines[key_index])
        raw_values = []
        for line in lines[key_index + 1 :]:
            code = line.split("#", 1)[0]
            if not code.strip():
                continue
            if _indent(code) <= key_indent:
                break
            match = re.match(r"^\s*-\s*(.*?)\s*$", code)
            if not match:
                return None
            raw_values.append(match.group(1))

    for raw_value in raw_values:
        value = raw_value.strip()
        if (
            len(value) >= 2
            and value[0] == value[-1]
            and value[0] in {"'", '"'}
        ):
            value = value[1:-1]
        if not re.fullmatch(r"[A-Za-z0-9._/-]+", value):
            return None
        values.append(value)
    return values


def _audit_workflow_run_trigger(path: Path, lines: list[str]) -> list[Finding]:
    """Bind privileged workflow_run handlers to canonical upstream code."""
    if not _has_trigger(lines, "workflow_run"):
        return []

    event_index: int | None = None
    event_indent = 0
    for index, line in enumerate(lines):
        code = line.split("#", 1)[0]
        if re.match(rf"^\s+{_yaml_key('workflow_run')}\s*:", code):
            event_index = index
            event_indent = _indent(code)
            break

    branch_declarations: list[tuple[int, list[str] | None]] = []
    if event_index is not None:
        for index in range(event_index + 1, len(lines)):
            code = lines[index].split("#", 1)[0]
            if not code.strip():
                continue
            if _indent(code) <= event_indent:
                break
            match = re.match(
                rf"^(\s*){_yaml_key('branches')}\s*:\s*(.*?)\s*$", code
            )
            if match and len(match.group(1)) == event_indent + 2:
                branch_declarations.append(
                    (index, _static_yaml_list(lines, index, match.group(2)))
                )

    if len(branch_declarations) != 1 or branch_declarations[0][1] != ["main"]:
        line_number = (
            branch_declarations[0][0] + 1
            if branch_declarations
            else (event_index + 1 if event_index is not None else 1)
        )
        return [
            Finding(
                path,
                line_number,
                "workflow_run must restrict the triggering workflow to the "
                "literal canonical branch list branches: [main]",
            )
        ]
    return []


def _audit_runner_job_timeouts(path: Path, lines: list[str]) -> list[Finding]:
    """Require each locally executed job to declare one bounded time budget."""
    findings: list[Finding] = []
    for job_index, end_index, _ in _runner_job_ranges(lines):
        direct_indent = _direct_mapping_indent(lines, job_index, end_index)
        if direct_indent is None:
            continue
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


def _audit_oidc_runner_trust(path: Path, lines: list[str]) -> list[Finding]:
    """Keep OIDC-backed delivery identity on reviewed GitHub-hosted images."""
    findings: list[Finding] = []
    jobs_index = next(
        (
            index
            for index, line in enumerate(lines)
            if JOBS_RE.match(line.split("#", 1)[0])
        ),
        len(lines),
    )
    top_level_oidc_write = any(
        not match.group(1)
        and _permission_mapping_grants_scope_write(lines, index, "id-token")
        for index, line in enumerate(lines[:jobs_index])
        if (match := PERMISSIONS_RE.match(line.split("#", 1)[0]))
    )

    for job_index, end_index, _ in _runner_job_ranges(lines):
        direct_indent = _direct_mapping_indent(lines, job_index, end_index)
        if direct_indent is None:
            continue
        permission_indexes = [
            index
            for index in range(job_index + 1, end_index)
            if (
                (match := PERMISSIONS_RE.match(lines[index].split("#", 1)[0]))
                and len(match.group(1)) == direct_indent
            )
        ]
        oidc_write = (
            any(
                _permission_mapping_grants_scope_write(
                    lines, index, "id-token", end_index
                )
                for index in permission_indexes
            )
            if permission_indexes
            else top_level_oidc_write
        )
        if not oidc_write:
            continue

        runner_declarations: list[tuple[int, str]] = []
        for index in range(job_index + 1, end_index):
            code = lines[index].split("#", 1)[0]
            match = re.match(
                rf"^(\s*){_yaml_key('runs-on')}\s*:\s*(.*?)\s*$", code
            )
            if match and len(match.group(1)) == direct_indent:
                runner_declarations.append(
                    (index, match.group(2).strip().strip("'\""))
                )

        # Reusable-workflow callers cannot choose a runner. The called
        # workflow's local jobs own and are independently audited for this
        # boundary.
        if not runner_declarations:
            continue
        if (
            len(runner_declarations) != 1
            or runner_declarations[0][1] not in TRUSTED_OIDC_RUNNER_LABELS
        ):
            finding_index = (
                runner_declarations[0][0] if runner_declarations else job_index
            )
            findings.append(
                Finding(
                    path,
                    finding_index + 1,
                    "id-token write jobs must use exactly one reviewed literal "
                    "GitHub-hosted runner label; dynamic or self-hosted runners "
                    "cannot mint trusted delivery identity",
                )
            )

    return findings


def _audit_privileged_job_caches(path: Path, lines: list[str]) -> list[Finding]:
    """Keep unsigned cache state out of credential- and write-capable jobs."""
    findings: list[Finding] = []
    jobs_index = next(
        (
            index
            for index, line in enumerate(lines)
            if JOBS_RE.match(line.split("#", 1)[0])
        ),
        len(lines),
    )
    jobs_end = (
        _mapping_end_index(lines, jobs_index)
        if jobs_index < len(lines)
        else len(lines)
    )
    top_level_write = any(
        not match.group(1)
        and _permission_mapping_has_write(lines, index)
        for index, line in enumerate(lines)
        if (match := PERMISSIONS_RE.match(line.split("#", 1)[0]))
    )
    top_level_confidential_secret = _has_confidential_secret_reference(
        lines[:jobs_index] + lines[jobs_end:]
    )

    for job_index, end_index, _ in _runner_job_ranges(lines):
        job_lines = lines[job_index:end_index]
        direct_indent = _direct_mapping_indent(lines, job_index, end_index)
        job_write = (
            any(
                len(match.group(1)) == direct_indent
                and _permission_mapping_has_write(lines, index, end_index)
                for index in range(job_index + 1, end_index)
                if (
                    match := PERMISSIONS_RE.match(
                        lines[index].split("#", 1)[0]
                    )
                )
            )
            if direct_indent is not None
            else False
        )
        privileged = (
            top_level_write
            or top_level_confidential_secret
            or _has_confidential_secret_reference(job_lines)
            or job_write
        )
        if not privileged:
            continue

        for relative_index, line in enumerate(job_lines):
            index = job_index + relative_index
            code = line.split("#", 1)[0]
            dependency_match = USES_RE.search(code)
            if dependency_match:
                dependency = dependency_match.group(1).strip("'\"").lower()
                if dependency.startswith(CACHE_ACTION_PREFIXES):
                    findings.append(
                        Finding(
                            path,
                            index + 1,
                            "credential- or write-capable jobs must not restore "
                            "or save unsigned caches",
                        )
                    )
                elif dependency.startswith(SETUP_CACHE_ACTION_PREFIXES):
                    action_block = _checkout_block(lines, index)
                    unsafe_cache_lines: list[int] = []
                    for cache_index, cache_line in action_block:
                        cache_match = CACHE_INPUT_RE.match(cache_line.split("#", 1)[0])
                        if not cache_match:
                            continue
                        value = cache_match.group(1).strip().strip("'\"").lower()
                        if value != "false":
                            unsafe_cache_lines.append(cache_index)

                    required_disable_input = next(
                        (
                            input_name
                            for prefix, input_name in AUTO_CACHE_DISABLE_INPUTS.items()
                            if dependency.startswith(prefix)
                        ),
                        None,
                    )
                    disable_declarations: list[tuple[int, str]] = []
                    if required_disable_input:
                        disable_re = re.compile(
                            rf"^\s*{_yaml_key(required_disable_input)}\s*:\s*(.*?)\s*$",
                            re.IGNORECASE,
                        )
                        for block_index, block_line in action_block:
                            disable_match = disable_re.match(
                                block_line.split("#", 1)[0]
                            )
                            if disable_match:
                                disable_declarations.append(
                                    (block_index, disable_match.group(1))
                                )
                    explicitly_disabled = (
                        required_disable_input is None
                        or (
                            len(disable_declarations) == 1
                            and disable_declarations[0][1]
                            .strip()
                            .strip("'\"")
                            .lower()
                            == "false"
                        )
                    )
                    if unsafe_cache_lines or not explicitly_disabled:
                        finding_index = (
                            unsafe_cache_lines[0]
                            if unsafe_cache_lines
                            else index
                        )
                        findings.append(
                            Finding(
                                path,
                                finding_index + 1,
                                "credential- or write-capable jobs must explicitly "
                                "disable setup-action caches",
                            )
                        )

            if EXTERNAL_CACHE_INPUT_RE.match(code):
                findings.append(
                    Finding(
                        path,
                        index + 1,
                        "credential- or write-capable jobs must not import or "
                        "export external build caches",
                    )
                )

    return findings


def audit_workflow(path: Path) -> list[Finding]:
    lines = path.read_text(encoding="utf-8").splitlines()
    findings = _audit_yaml_trust_shape(path, lines)
    findings.extend(_audit_runner_job_timeouts(path, lines))
    findings.extend(_audit_oidc_runner_trust(path, lines))
    findings.extend(_audit_privileged_job_caches(path, lines))
    findings.extend(_audit_workflow_run_trigger(path, lines))
    has_workflow_run_trigger = _has_trigger(lines, "workflow_run")

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
            if _has_confidential_secret_reference([line]):
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
            findings.extend(_audit_command_file_data(path, lines, index))
            for script_index, script_line in _run_script_lines(lines, index):
                if has_workflow_run_trigger and WORKFLOW_RUN_INGRESS_COMMAND_RE.search(
                    script_line
                ):
                    findings.append(
                        Finding(
                            path,
                            script_index + 1,
                            "workflow_run scripts must not acquire upstream "
                            "workflow artifacts or repository code",
                        )
                    )
                for expression in EXPRESSION_RE.findall(script_line):
                    if SHELL_DATA_CONTEXT_RE.search(expression):
                        findings.append(
                            Finding(
                                path,
                                script_index + 1,
                                "workflow inputs, dispatch payloads, secrets, "
                                "GitHub contexts, and step/job outputs must enter "
                                "run scripts through env",
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
        if has_workflow_run_trigger and dependency.lower().startswith(
            WORKFLOW_RUN_ARTIFACT_ACTION_PREFIX
        ):
            findings.append(
                Finding(
                    path,
                    line_number,
                    "workflow_run must not download upstream workflow artifacts",
                )
            )
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
        if has_workflow_run_trigger and any(
            WORKFLOW_RUN_CODE_REF_RE.search(block_line.split("#", 1)[0])
            for _, block_line in checkout_block
        ):
            findings.append(
                Finding(
                    path,
                    line_number,
                    "workflow_run checkout must not select code from triggering-run metadata",
                )
            )
        unsafe_pr_settings = [
            (block_index, block_line)
            for block_index, block_line in checkout_block
            if re.match(
                rf"^\s*{_yaml_key('allow-unsafe-pr-checkout')}\s*:",
                block_line,
            )
        ]
        for setting_index, setting in unsafe_pr_settings:
            if not re.match(
                rf"^\s*{_yaml_key('allow-unsafe-pr-checkout')}\s*:\s*"
                r"false(?:\s|$)",
                setting,
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
            if re.match(
                rf"^\s*{_yaml_key('persist-credentials')}\s*:", block_line
            )
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
            rf"^\s*{_yaml_key('persist-credentials')}\s*:\s*(true|false)\b",
            setting,
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
