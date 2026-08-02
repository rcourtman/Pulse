#!/usr/bin/env python3

from __future__ import annotations

from io import StringIO
import unittest
from unittest.mock import patch

from browser_verification_guard import (
    RECEIPT_PATH,
    frontend_runtime_paths,
    main,
    validate_receipt,
)


BASE_SHA = "a" * 40
CHANGED_PATH = "frontend-modern/src/components/Example.tsx"
CONTENT_SHA = "c" * 64


def valid_receipt() -> dict:
    return {
        "version": 1,
        "base_sha": BASE_SHA,
        "verified_at": "2026-08-02T20:15:00Z",
        "result": "passed",
        "changed_paths": [CHANGED_PATH],
        "content_sha256": {CHANGED_PATH: CONTENT_SHA},
        "routes": ["/proxmox/overview"],
        "viewports": [
            {"width": 1280, "height": 800},
            {"width": 390, "height": 844},
        ],
        "states": ["toolbar closed", "View menu open", "Columns disclosure open"],
        "interactions": ["Open View, open Columns, dismiss with Escape"],
    }


class BrowserVerificationGuardTest(unittest.TestCase):
    def test_blocks_frontend_change_when_receipt_is_not_in_commit(self) -> None:
        with (
            patch("sys.stdin", StringIO(CHANGED_PATH + "\n")),
            patch("sys.stderr", new=StringIO()),
        ):
            self.assertEqual(main(["--files-from-stdin"]), 1)

    def test_frontend_runtime_paths_exclude_tests_and_receipt(self) -> None:
        self.assertEqual(
            frontend_runtime_paths(
                [
                    CHANGED_PATH,
                    "frontend-modern/src/components/Example.test.tsx",
                    "frontend-modern/src/components/__tests__/Example.tsx",
                    RECEIPT_PATH,
                    "frontend-modern/index.html",
                    "internal/api/server.go",
                ]
            ),
            ["frontend-modern/index.html", CHANGED_PATH],
        )

    def test_accepts_receipt_bound_to_changed_paths_and_two_viewports(self) -> None:
        self.assertEqual(
            validate_receipt(
                valid_receipt(),
                changed_paths=[CHANGED_PATH],
                expected_base=BASE_SHA,
                expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
            ),
            [],
        )

    def test_rejects_stale_base_and_incomplete_changed_path_coverage(self) -> None:
        receipt = valid_receipt()
        receipt["base_sha"] = "b" * 40
        receipt["changed_paths"] = ["frontend-modern/src/components/Other.tsx"]

        errors = validate_receipt(
            receipt,
            changed_paths=[CHANGED_PATH],
            expected_base=BASE_SHA,
            expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
        )

        self.assertTrue(any("base_sha" in error for error in errors))
        self.assertTrue(any("changed_paths" in error for error in errors))

    def test_rejects_dom_only_proof_without_states_interactions_or_narrow_viewport(self) -> None:
        receipt = valid_receipt()
        receipt["viewports"] = [{"width": 1280, "height": 800}]
        receipt["states"] = []
        receipt["interactions"] = []

        errors = validate_receipt(
            receipt,
            changed_paths=[CHANGED_PATH],
            expected_base=BASE_SHA,
            expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
        )

        self.assertTrue(any("narrow width" in error for error in errors))
        self.assertTrue(any("states" in error for error in errors))
        self.assertTrue(any("interactions" in error for error in errors))

    def test_rejects_source_content_changed_after_browser_verification(self) -> None:
        receipt = valid_receipt()

        errors = validate_receipt(
            receipt,
            changed_paths=[CHANGED_PATH],
            expected_base=BASE_SHA,
            expected_content_sha256={CHANGED_PATH: "d" * 64},
        )

        self.assertTrue(any("content_sha256" in error for error in errors))


if __name__ == "__main__":
    unittest.main()
