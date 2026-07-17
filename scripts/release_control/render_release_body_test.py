#!/usr/bin/env python3
"""Regression tests for publish-safe release body rendering."""

from __future__ import annotations

import re
import tempfile
import unittest
from pathlib import Path

import render_release_body

_REPO_ROOT = Path(__file__).resolve().parents[2]
_RC_DRAFT_PACKET_NAME_RE = re.compile(r"^RELEASE_NOTES_v6_RC(\d+)_DRAFT\.md$")


def _discover_rc_draft_packet_paths() -> tuple[str, ...]:
    """Return release_notes + changelog + support_pack relpaths for every in-repo RC draft packet."""
    paths: list[tuple[int, str]] = []
    for path in sorted((_REPO_ROOT / "docs" / "releases").glob("RELEASE_NOTES_v6_RC*_DRAFT.md")):
        match = _RC_DRAFT_PACKET_NAME_RE.match(path.name)
        if not match:
            continue
        n = int(match.group(1))
        paths.append((n, f"docs/releases/RELEASE_NOTES_v6_RC{n}_DRAFT.md"))
        paths.append((n, f"docs/releases/V6_CHANGELOG_RC{n}_DRAFT.md"))
        paths.append((n, f"docs/releases/V6_RC{n}_OPERATOR_SUPPORT_PACK_DRAFT.md"))
    return tuple(rel for _, rel in sorted(paths))


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
                "Body.\n\n"
                "## Fixed\n\n"
                "- Corrected a release issue.\n",
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
            self.assertIn(
                "public GitHub release assets and the public `rcourtman/pulse` Docker image are community builds",
                body,
            )
            self.assertIn("https://pulserelay.pro/download.html", body)
            self.assertIn("- Rollback target: v5.1.28", body)
            render_release_body.validate_release_body_shape(body, "6.0.0-rc.2")

    def test_flattened_release_notes_fail_closed(self) -> None:
        flattened = (
            "# Pulse v6.1.0-rc.2 Release Notes"
            "`v6.1.0-rc.2` is a release candidate."
            "## Highlights"
            "- Patrol findings stay governed."
            "## Upgrade Notes"
            "Use the RC channel."
        )

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "standalone release title",
        ):
            render_release_body.validate_release_notes_shape(
                flattened,
                "6.1.0-rc.2",
            )

    def test_stored_release_body_must_match_expected_rendered_markdown(self) -> None:
        expected = """# Pulse v6.1.0-rc.2 Release Notes

Intro.

## Highlights

- Patrol findings stay governed.

## Installation

Install details.

## Promotion Metadata

- Promotion channel: rc
"""
        validation_block = """<!-- VALIDATION_STATUS_START -->
## Release Asset Validation: PASSED

Assets passed.
<!-- VALIDATION_STATUS_END -->

"""
        stored = validation_block + expected

        clean = render_release_body.validate_release_body_shape(
            stored,
            "6.1.0-rc.2",
            expected_body=expected,
        )
        self.assertEqual(clean, expected)

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "does not exactly match",
        ):
            render_release_body.validate_release_body_shape(
                stored,
                "6.1.0-rc.2",
                expected_body=expected.replace("Patrol", "Assistant"),
            )

    def test_stored_release_body_rejects_inline_headings(self) -> None:
        flattened = """# Pulse v6.1.0-rc.2 Release Notes

Intro.## Highlights- Patrol findings stay governed.

## Installation

Install details.

## Promotion Metadata

- Promotion channel: rc
"""
        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "flattened Markdown",
        ):
            render_release_body.validate_release_body_shape(
                flattened,
                "6.1.0-rc.2",
            )

    def test_current_release_packets_use_pulse_mobile_handoff_copy(self) -> None:
        repo_root = _REPO_ROOT
        # Stable release notes are hardcoded; every in-repo RC draft packet is
        # discovered from the filesystem so adding a new RC doesn't require
        # editing this tuple.
        packet_paths = ("docs/releases/RELEASE_NOTES_v6.md",) + _discover_rc_draft_packet_paths()

        for relative_path in packet_paths:
            with self.subTest(relative_path=relative_path):
                text = (repo_root / relative_path).read_text(encoding="utf-8")
                self.assertIn("Pulse Mobile pairing for handoff", text)
                self.assertNotIn("mobile app pairing", text)
                self.assertNotIn("remote access/mobile/push", text)

    def test_rc3_packet_records_commit_coverage_and_release_artifact_hardening(self) -> None:
        repo_root = Path(__file__).resolve().parents[2]
        release_notes = (repo_root / "docs/releases/RELEASE_NOTES_v6_RC3_DRAFT.md").read_text(
            encoding="utf-8"
        )
        changelog = (repo_root / "docs/releases/V6_CHANGELOG_RC3_DRAFT.md").read_text(
            encoding="utf-8"
        )
        support_pack = (
            repo_root / "docs/releases/V6_RC3_OPERATOR_SUPPORT_PACK_DRAFT.md"
        ).read_text(encoding="utf-8")

        self.assertIn("158d65ccdb81077c35b9237a1652b2774ddb5d5c", release_notes)
        self.assertIn("commit count: `605`", changelog)
        self.assertIn("broad hardening RC with a corrective maintenance core", changelog)
        self.assertIn("Community-tier capabilities", release_notes)
        self.assertIn("stable-channel release resolution", release_notes)
        self.assertIn("Release asset uploads use bounded retries", release_notes)
        self.assertIn(
            "release artifact validation, draft metadata preservation, upload retries",
            support_pack,
        )

    def test_rc4_packet_records_commit_coverage_and_identity_hardening(self) -> None:
        repo_root = Path(__file__).resolve().parents[2]
        release_notes = (repo_root / "docs/releases/RELEASE_NOTES_v6_RC4_DRAFT.md").read_text(
            encoding="utf-8"
        )
        changelog = (repo_root / "docs/releases/V6_CHANGELOG_RC4_DRAFT.md").read_text(
            encoding="utf-8"
        )
        support_pack = (
            repo_root / "docs/releases/V6_RC4_OPERATOR_SUPPORT_PACK_DRAFT.md"
        ).read_text(encoding="utf-8")

        self.assertIn("7cebe788590d0485f65bf4e04830356204657e86", release_notes)
        self.assertIn("commit count: `57`", changelog)
        self.assertIn("stable identity principals", support_pack)
        self.assertIn("API-first action planning", changelog)
        self.assertIn("monitored-system and child-resource volume unmetered", release_notes)
        self.assertIn("Pulse Mobile pairing for handoff", support_pack)
        self.assertIn("pin Docker install defaults to `6.0.0-rc.4`", changelog)
        self.assertIn("Docker Compose and turnkey Docker installer defaults", release_notes)
        self.assertIn("release-validation\ncommits", changelog)
        self.assertIn("Tenant monitor state broadcasts", release_notes)
        self.assertIn("tenant\nmonitor broadcast guard", changelog)
        self.assertIn("live auth-env watcher teardown", release_notes)
        self.assertIn("join live config watcher goroutines", changelog)

    def test_rc5_packet_records_commit_coverage_and_agent_substrate(self) -> None:
        repo_root = Path(__file__).resolve().parents[2]
        release_notes = (repo_root / "docs/releases/RELEASE_NOTES_v6_RC5_DRAFT.md").read_text(
            encoding="utf-8"
        )
        changelog = (repo_root / "docs/releases/V6_CHANGELOG_RC5_DRAFT.md").read_text(
            encoding="utf-8"
        )
        support_pack = (
            repo_root / "docs/releases/V6_RC5_OPERATOR_SUPPORT_PACK_DRAFT.md"
        ).read_text(encoding="utf-8")

        self.assertIn("e36945741e1db5d763ab63eeeda18a58acda23c5", release_notes)
        self.assertIn("commit count: `428`", changelog)
        self.assertIn("agent-substrate HTTP contract", release_notes)
        self.assertIn("/api/agent/capabilities", changelog)
        self.assertIn("Pulse Intelligence", support_pack)
        self.assertIn("operator-state", changelog)
        self.assertIn("Pulse Mobile pairing for handoff", support_pack)

    def test_shipped_rc1_notes_document_current_agent_upgrade_surface(self) -> None:
        repo_root = Path(__file__).resolve().parents[2]
        release_notes = (repo_root / "docs/releases/RELEASE_NOTES_v6_RC1.md").read_text(
            encoding="utf-8"
        )

        self.assertIn("Settings -> Infrastructure -> Install on a host", release_notes)
        self.assertIn("first installs and in-place upgrades", release_notes)
        self.assertIn("after an upgraded agent authenticates", release_notes)
        self.assertNotIn("Settings -> Agents -> Installation commands", release_notes)
        self.assertNotIn("Settings → Agents → Installation commands", release_notes)

    def test_agent_paradigm_release_notes_blurb_documents_distribution_path(self) -> None:
        """The agent-paradigm source draft must keep its honest scope:
        an integrator reading the blurb sees a published distribution
        path (the install-mcp script + GitHub Release binaries) when
        the work lands, not the earlier "build from source" wording.

        Pin the blurb's stable touchstones so a future edit that
        accidentally regresses the install story (e.g. swaps the
        one-line installer for "clone the repo" again) fails this
        test instead of shipping silently into a release.
        """
        repo_root = Path(__file__).resolve().parents[2]
        blurb = (repo_root / "docs/releases/AGENT_PARADIGM.md").read_text(encoding="utf-8")

        self.assertIn("install-mcp.sh", blurb, "blurb must reference the published install script")
        self.assertIn("/api/agent/capabilities", blurb)
        self.assertIn("cmd/pulse-mcp", blurb)
        self.assertIn("cmd/agent-probe", blurb)
        self.assertIn("OpenCode, other MCP clients", blurb)
        self.assertIn("client-ready MCP config snippets", blurb)
        self.assertIn("OpenCode's native", blurb)
        self.assertIn("common `mcpServers` shape", blurb)
        self.assertIn("Drivable from MCP clients in one command", blurb)
        self.assertIn("Wire it into any MCP-speaking client", blurb)
        self.assertIn("manifest `requiredScopes`", blurb)
        self.assertIn("read-only subset", blurb)
        self.assertNotIn("common MCP config snippet", blurb)
        self.assertNotIn("clients that accept\n  `mcpServers`", blurb)
        self.assertNotIn("Claude Desktop / Claude Code", blurb)
        self.assertNotIn("Drivable from Claude in one command", blurb)
        self.assertNotIn("adapter for Claude Desktop and Claude Code", blurb)
        self.assertNotIn("`monitoring:read` (and", blurb)
        # The four-axis frame is the substrate's load-bearing claim;
        # if any axis name drifts in the blurb, agents reading
        # release notes will look for a different surface than what
        # ships.
        self.assertIn("Discovery", blurb)
        self.assertIn("Read", blurb)
        self.assertIn("Write", blurb)
        self.assertIn("Push", blurb)


if __name__ == "__main__":
    unittest.main()
