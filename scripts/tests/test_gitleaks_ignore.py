#!/usr/bin/env python3
"""Keep historical gitleaks suppressions exact and auditable."""

from __future__ import annotations

import re
import subprocess
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
IGNORE_PATH = REPO_ROOT / ".gitleaksignore"
HISTORICAL_FINGERPRINT = re.compile(
    r"^(?P<commit>[0-9a-f]{40}):(?P<path>.+):(?P<rule>[a-z0-9-]+):(?P<line>[1-9][0-9]*)$"
)
CURRENT_FINGERPRINT = re.compile(
    r"^(?P<path>.+):(?P<rule>[a-z0-9-]+):(?P<line>[1-9][0-9]*)$"
)


def ignore_entries() -> list[str]:
    return [
        line.strip()
        for line in IGNORE_PATH.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]


class GitleaksIgnoreTest(unittest.TestCase):
    def test_entries_are_exact_fingerprints(self) -> None:
        invalid = [
            entry
            for entry in ignore_entries()
            if not HISTORICAL_FINGERPRINT.fullmatch(entry)
            and not CURRENT_FINGERPRINT.fullmatch(entry)
        ]
        self.assertEqual(invalid, [], msg=f"non-exact gitleaks ignore entries: {invalid}")

    def test_historical_fingerprints_resolve_to_the_named_blob(self) -> None:
        failures: list[str] = []

        for entry in ignore_entries():
            match = HISTORICAL_FINGERPRINT.fullmatch(entry)
            if match is None:
                continue

            commit = match.group("commit")
            path = match.group("path")
            result = subprocess.run(
                ["git", "cat-file", "-e", f"{commit}:{path}"],
                cwd=REPO_ROOT,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                check=False,
            )
            if result.returncode != 0:
                failures.append(entry)

        self.assertEqual(
            failures,
            [],
            msg="historical gitleaks fingerprints do not resolve:\n- "
            + "\n- ".join(failures),
        )


if __name__ == "__main__":
    unittest.main()
