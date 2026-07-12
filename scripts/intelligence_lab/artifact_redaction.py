#!/usr/bin/env python3
"""Bounded redaction for ignored local intelligence-lab artifacts."""

from __future__ import annotations

import re
import hashlib
from pathlib import Path
import stat

MAX_ARTIFACT_TEXT = 64 * 1024

ALLOWED_ARTIFACT_BASENAMES = frozenset({
    "SHA256SUMS",
    "actions-history-current-build.png",
    "browser-trace.zip",
    "canonical-journey.json",
    "environment.json",
    "run-report.json",
})

_PATTERNS = (
    re.compile(r"(?i)(authorization\s*:\s*bearer\s+)[^\s\"']+"),
    re.compile(r"(?i)((?:api[_-]?token|password|secret)\s*[=:]\s*)[^\s\"']+"),
    re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----", re.S),
)


def redact_text(value: str) -> str:
    value = value[:MAX_ARTIFACT_TEXT]
    for pattern in _PATTERNS:
        value = pattern.sub(r"\1[REDACTED]" if pattern.groups else "[REDACTED PRIVATE KEY]", value)
    return value


def contains_forbidden_secret_shape(value: str) -> bool:
    lowered = value.lower()
    return any(marker in lowered for marker in ("authorization: bearer ", "-----begin private key-----"))


def assert_allowed_artifact_tree(root: Path) -> list[Path]:
    root = root.resolve()
    files: list[Path] = []
    for path in sorted(root.rglob("*")):
        relative = path.relative_to(root)
        if path.is_symlink():
            raise ValueError(f"artifact symlink is forbidden: {relative}")
        mode = path.stat().st_mode
        if stat.S_ISSOCK(mode):
            raise ValueError(f"artifact socket is forbidden: {relative}")
        if path.is_dir():
            raise ValueError(f"artifact subdirectory is forbidden: {relative}")
        lowered = path.name.lower()
        forbidden_name = (
            lowered.endswith((".db", ".sqlite", ".sqlite3", ".wal", ".shm", ".log", ".env", ".sock"))
            or ".db-" in lowered
            or any(marker in lowered for marker in ("token", "cookie", "authorization", "auth-header", "secret", "credential"))
        )
        if forbidden_name:
            raise ValueError(f"forbidden artifact basename: {relative}")
        if path.name not in ALLOWED_ARTIFACT_BASENAMES:
            raise ValueError(f"unknown artifact outside allowlist: {relative}")
        files.append(path)
    return files


def write_checksums(root: Path) -> None:
    files = [path for path in assert_allowed_artifact_tree(root) if path.name != "SHA256SUMS"]
    lines = [f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}" for path in files]
    manifest = root.resolve() / "SHA256SUMS"
    manifest.write_text("\n".join(lines) + "\n", encoding="utf-8")
    manifest.chmod(0o600)
    verify_checksums(root)


def verify_checksums(root: Path) -> None:
    files = assert_allowed_artifact_tree(root)
    manifest = root.resolve() / "SHA256SUMS"
    if manifest not in files:
        raise ValueError("SHA256SUMS is required")
    expected: dict[str, str] = {}
    for line in manifest.read_text(encoding="utf-8").splitlines():
        digest, separator, name = line.partition("  ")
        if not separator or not re.fullmatch(r"[a-f0-9]{64}", digest) or name not in ALLOWED_ARTIFACT_BASENAMES or name == "SHA256SUMS":
            raise ValueError("invalid checksum manifest entry")
        expected[name] = digest
    actual = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in files if path.name != "SHA256SUMS"}
    if expected != actual:
        raise ValueError("checksum manifest does not match allowed artifact tree")
