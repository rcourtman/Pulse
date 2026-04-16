#!/usr/bin/env python3
"""Regression tests for publish-safe release body rendering."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

import render_release_body


class RenderReleaseBodyTest(unittest.TestCase):
    def test_sanitize_release_notes_strips_draft_markers_duplicate_sections_and_draft_links(self) -> None:
        raw = """# Pulse v6.0.0-rc.2 Draft Release Notes

_Draft only. Do not treat this as published until the governed `v6.0.0-rc.2`
tag and GitHub prerelease exist._

Intro paragraph.

## Operator References

- `docs/releases/V6_RC2_OPERATOR_SUPPORT_PACK_DRAFT.md`
- `docs/UPGRADE_v6.md`

## Installation

Old install section.

## Promotion Metadata

Old metadata section.
"""
        sanitized = render_release_body.sanitize_release_notes(raw, "6.0.0-rc.2")
        self.assertIn("# Pulse v6.0.0-rc.2 Release Notes", sanitized)
        self.assertNotIn("Draft Release Notes", sanitized)
        self.assertNotIn("Draft only. Do not treat this as published", sanitized)
        self.assertNotIn("_DRAFT.md", sanitized)
        self.assertIn("- `docs/UPGRADE_v6.md`", sanitized)
        self.assertNotIn("## Installation", sanitized)
        self.assertNotIn("## Promotion Metadata", sanitized)

    def test_main_renders_single_installation_and_promotion_metadata_sections(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            notes_file = Path(tmp) / "notes.md"
            output_file = Path(tmp) / "body.md"
            notes_file.write_text(
                "# Pulse v6.0.0-rc.2 Draft Release Notes\n\n"
                "_Draft only. Do not treat this as published until the governed `v6.0.0-rc.2` tag and GitHub prerelease exist._\n\n"
                "Body.\n",
                encoding="utf-8",
            )

            args = render_release_body.parse_args.__wrapped__ if hasattr(render_release_body.parse_args, "__wrapped__") else None
            del args  # satisfy linters if wrapped implementation changes later

            namespace = type(
                "Args",
                (),
                {
                    "version": "6.0.0-rc.2",
                    "release_notes_file": str(notes_file),
                    "output": str(output_file),
                    "promotion_channel": "rc",
                    "candidate_tag": "v6.0.0-rc.2",
                    "promoted_prerelease_tag": "",
                    "rollback_target": "v5.1.28",
                    "rollback_command": "./scripts/install.sh --version v5.1.28",
                    "planned_ga_date": "",
                    "planned_v5_eos_date": "",
                    "hotfix_exception": "false",
                    "hotfix_reason": "",
                },
            )()

            raw_text = Path(namespace.release_notes_file).read_text(encoding="utf-8")
            sanitized = render_release_body.sanitize_release_notes(raw_text, namespace.version).rstrip("\n")
            sections = [
                sanitized,
                render_release_body.build_installation_section(namespace.version),
                render_release_body.build_promotion_metadata_section(namespace),
            ]
            Path(namespace.output).write_text("\n\n".join(sections) + "\n", encoding="utf-8")

            body = output_file.read_text(encoding="utf-8")
            self.assertEqual(body.count("## Installation"), 1)
            self.assertEqual(body.count("## Promotion Metadata"), 1)
            self.assertIn("docker pull rcourtman/pulse:6.0.0-rc.2", body)
            self.assertIn("- Rollback target: v5.1.28", body)


if __name__ == "__main__":
    unittest.main()
