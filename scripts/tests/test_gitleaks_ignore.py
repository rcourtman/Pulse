#!/usr/bin/env python3
"""Keep historical gitleaks suppressions exact and auditable."""

from __future__ import annotations

import json
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
    def test_portal_receipt_exceptions_are_public_auth_source_blob_ids(self) -> None:
        receipt_path = "internal/cloudcp/portal/browser-verification/faa976065fe7bd60.json"
        source_commit = "3b4ef64ed6a0c2e2356d775478ad4d64418a2222"
        receipt_text = (REPO_ROOT / receipt_path).read_text(encoding="utf-8")
        receipt = json.loads(receipt_text)
        source_lines = {
            9: "internal/cloudcp/portal/frontend/src/auth_controller.test.ts",
            10: "internal/cloudcp/portal/frontend/src/auth_controller.ts",
        }
        expected_entries = {
            f"{receipt_path}:generic-api-key:{line}" for line in source_lines
        }
        self.assertEqual(
            {entry for entry in ignore_entries() if receipt_path in entry},
            expected_entries,
        )
        for line, source_path in source_lines.items():
            with self.subTest(source_path=source_path):
                blob_id = subprocess.check_output(
                    ["git", "rev-parse", f"{source_commit}:{source_path}"],
                    cwd=REPO_ROOT, text=True,
                ).strip()
                self.assertRegex(blob_id, r"^[0-9a-f]{40}$")
                self.assertEqual(receipt["verified_blobs"][source_path], blob_id)
                self.assertEqual(
                    json.loads("{" + receipt_text.splitlines()[line - 1].rstrip(",") + "}"),
                    {source_path: blob_id},
                )

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
