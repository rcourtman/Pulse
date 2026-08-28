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
    def test_rc13_packet_keeps_typed_truenas_smart_evidence_visible(self) -> None:
        notes = (
            _REPO_ROOT / "docs" / "releases" / "RELEASE_NOTES_v6.4.0-rc.13.md"
        ).read_text(encoding="utf-8")

        self.assertIn(
            "expose supported uncorrectable-error and spare-reserve values",
            notes,
        )
        render_release_body.validate_release_notes_shape(notes, "6.4.0-rc.13")

    def test_highlights_are_a_small_plain_language_overview(self) -> None:
        notes = """# Pulse v6.2.0 Release Notes

## Highlights

- Alerts now explain what went wrong and what to do next.
- Tables are easier to use on phones and small screens.
- Updates recover cleanly when an earlier installation was interrupted.

## Fixed

- Corrected a release issue.
"""

        render_release_body.validate_release_notes_shape(notes, "6.2.0")

    def test_generated_level_three_highlights_and_wrapped_bullets_are_supported(self) -> None:
        notes = """# Pulse v6.2.0 Release Notes

## v6.2.0

### Highlights

- Alerts now explain what went wrong and what to do
  next.
- Tables are easier to use on small screens.

### Bug Fixes

- Corrected a release issue.
"""

        render_release_body.validate_release_notes_shape(notes, "6.2.0")

    def test_highlights_reject_more_than_three_items(self) -> None:
        notes = """# Pulse v6.2.0 Release Notes

## Highlights

- One.
- Two.
- Three.
- Four.

## Fixed

- Corrected a release issue.
"""

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "at most 3 bullets",
        ):
            render_release_body.validate_release_notes_shape(notes, "6.2.0")

    def test_highlights_reject_long_or_formatted_items(self) -> None:
        long_item = "A" * 141
        long_notes = f"""# Pulse v6.2.0 Release Notes

## Highlights

- {long_item}

## Fixed

- Corrected a release issue.
"""
        formatted_notes = """# Pulse v6.2.0 Release Notes

## Highlights

- Read the [upgrade guide](https://example.com) for details.

## Fixed

- Corrected a release issue.
"""

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "140 characters or fewer",
        ):
            render_release_body.validate_release_notes_shape(long_notes, "6.2.0")
        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "must use plain text",
        ):
            render_release_body.validate_release_notes_shape(formatted_notes, "6.2.0")

    def test_highlights_reject_bare_and_parenthesized_issue_references(self) -> None:
        for reference in ("#123", "(#123)"):
            with self.subTest(reference=reference):
                notes = f"""# Pulse v6.2.0 Release Notes

## Highlights

- Alerts recover cleanly after an update {reference}.

## Fixed

- Corrected a release issue.
"""

                with self.assertRaisesRegex(
                    render_release_body.ReleaseBodyIntegrityError,
                    "issue references",
                ):
                    render_release_body.validate_release_notes_shape(notes, "6.2.0")

    def test_highlights_reject_common_github_issue_reference_variants(self) -> None:
        references = (
            "GH-123",
            "rcourtman/Pulse#123",
            "https://github.com/rcourtman/Pulse/issues/123",
            "https://github.com/rcourtman/Pulse/pull/123",
        )
        for reference in references:
            with self.subTest(reference=reference):
                notes = f"""# Pulse v6.2.0 Release Notes

## Highlights

- Alerts recover cleanly after an update {reference}.

## Fixed

- Corrected a release issue.
"""

                with self.assertRaisesRegex(
                    render_release_body.ReleaseBodyIntegrityError,
                    "issue references",
                ):
                    render_release_body.validate_release_notes_shape(notes, "6.2.0")

    def test_highlights_preserve_legitimate_plain_text_with_hash_characters(self) -> None:
        notes = """# Pulse v6.2.0 Release Notes

## Highlights

- C# service checks now recover cleanly after interrupted updates.
- F# applications keep their configured display names.

## Fixed

- Corrected a release issue.
"""

        render_release_body.validate_release_notes_shape(notes, "6.2.0")

    def test_release_notes_may_omit_highlights(self) -> None:
        notes = """# Pulse v6.2.1 Release Notes

## Fixed

- Corrected a maintenance issue.
"""

        render_release_body.validate_release_notes_shape(notes, "6.2.1")

    def test_future_release_notes_require_customer_facing_structure(self) -> None:
        notes = """# Pulse v6.4.0-rc.6 Release Notes

Pulse is faster and more predictable in larger environments.

## What's improved

- **Faster infrastructure views** — Tables stay responsive as estates grow.
- **Lighter realtime updates** — Pages do less work when resources change.
- **Safer API key handling** — Saved API keys are no longer returned to the browser.

## Before you upgrade

No manual migration is required.
"""

        render_release_body.validate_release_notes_shape(notes, "6.4.0-rc.6")

    def test_current_release_notes_reject_a_second_fixes_list(self) -> None:
        notes = """# Pulse v6.4.0-rc.6 Release Notes

Pulse is faster and more predictable in larger environments.

## What's improved

- **Faster infrastructure views** — Tables stay responsive as estates grow.

## Fixes

- Infrastructure tables no longer stall in larger estates.
"""

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "features and fixes once",
        ):
            render_release_body.validate_release_notes_shape(notes, "6.4.0-rc.6")

    def test_customer_facing_standard_exempts_only_the_already_cut_rc1(self) -> None:
        self.assertFalse(
            render_release_body._requires_customer_facing_standard("6.4.0-rc.1")
        )
        self.assertTrue(
            render_release_body._requires_customer_facing_standard("6.4.0-beta.1")
        )
        self.assertTrue(
            render_release_body._requires_customer_facing_standard("v6.4.0-rc.2")
        )
        self.assertTrue(
            render_release_body._requires_customer_facing_standard("6.4.0")
        )

    def test_v640_rc1_packet_records_current_customer_fixes(self) -> None:
        notes = (
            _REPO_ROOT / "docs" / "releases" / "RELEASE_NOTES_v6.4.0-rc.1.md"
        ).read_text(encoding="utf-8")

        render_release_body.validate_release_notes_shape(notes, "6.4.0-rc.1")
        normalized_notes = re.sub(r"\s+", " ", notes)
        self.assertIn(
            "Stopped Docker and Podman containers no longer re-fire health alerts "
            "from the stale health-check result retained by the container runtime.",
            normalized_notes,
        )
        self.assertIn(
            "Backup-location filters distinguish PBS servers and datastores so "
            "local and off-site restore points can be reviewed independently.",
            normalized_notes,
        )

    def test_future_release_notes_reject_internal_release_control_sections(self) -> None:
        notes = """# Pulse v6.4.0-rc.2 Release Notes

Pulse is faster and more predictable in larger environments.

## What's improved

- **Faster infrastructure views** — Tables stay responsive as estates grow.

## Release Qualification

- All readiness assertions and release gates passed.
"""

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "may only use",
        ):
            render_release_body.validate_release_notes_shape(notes, "6.4.0-rc.2")

    def test_future_release_notes_reject_internal_release_control_language(self) -> None:
        notes = """# Pulse v6.4.0 Release Notes

Pulse is faster and more predictable in larger environments.

## What's improved

- **Safer releases** - Every immutable candidate now crosses an exact-SHA gate.
"""

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "internal release-control language",
        ):
            render_release_body.validate_release_notes_shape(notes, "6.4.0")

    def test_future_release_notes_require_scannable_improvement_bullets(self) -> None:
        notes = """# Pulse v6.4.0 Release Notes

Pulse is faster and more predictable in larger environments.

## What's improved

- Tables stay responsive as estates grow.
"""

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "short bold outcome",
        ):
            render_release_body.validate_release_notes_shape(notes, "6.4.0")

    def test_customer_story_is_not_forced_into_a_fixed_item_count(self) -> None:
        bullets = "\n".join(
            f"- **Useful outcome {index}** - Users can understand change {index}."
            for index in range(1, 8)
        )
        notes = f"""# Pulse v6.4.0 Release Notes

Pulse has a concise set of improvements across the product.

## What's improved

{bullets}
"""

        render_release_body.validate_release_notes_shape(notes, "6.4.0")

    def test_new_release_notes_reject_semicolons(self) -> None:
        notes = """# Pulse v6.4.0 Release Notes

Pulse is faster; pages are steadier.

## What's improved

- **Faster pages** - Tables stay responsive as estates grow.
"""

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "semicolons or em dashes",
        ):
            render_release_body.validate_release_notes_shape(notes, "6.4.0")

    def test_new_release_notes_reject_em_dashes(self) -> None:
        notes = """# Pulse v6.4.0-rc.7 Release Notes

Pulse is faster and pages are steadier.

## What's improved

- **Faster pages** — Tables stay responsive as estates grow.
"""

        with self.assertRaisesRegex(
            render_release_body.ReleaseBodyIntegrityError,
            "semicolons or em dashes",
        ):
            render_release_body.validate_release_notes_shape(notes, "6.4.0-rc.7")

    def test_punctuation_rule_grandfathers_published_v640_rc_packets(self) -> None:
        self.assertFalse(
            render_release_body._requires_plain_release_punctuation("6.4.0-rc.6")
        )
        self.assertTrue(
            render_release_body._requires_plain_release_punctuation("6.4.0-rc.7")
        )
        self.assertTrue(
            render_release_body._requires_plain_release_punctuation("6.4.0")
        )
        self.assertTrue(
            render_release_body._requires_plain_release_punctuation("6.4.1")
        )

    def test_canonical_template_keeps_machine_process_out_of_customer_notes(self) -> None:
        template = (
            _REPO_ROOT / "docs/releases/RELEASE_NOTES_TEMPLATE.md"
        ).read_text(encoding="utf-8")

        self.assertIn("## What's improved", template)
        self.assertIn("## Before you upgrade", template)
        self.assertIn("For an RC, cover only changes since the immediately preceding RC", template)
        self.assertIn("For a stable GA release", template)
        self.assertIn("an unbounded factual investigation", template)
        self.assertIn("independent draft", template)
        self.assertIn("The models decide what to", template)
        self.assertIn("what matters, and how to tell", template)
        self.assertIn("the release story", template)
        self.assertIn("more than 260 characters", template)
        self.assertIn("must not contain semicolons or em", template)
        self.assertIn("dashes", template)
        self.assertNotIn(";", template)
        self.assertNotIn("—", template)
        self.assertIn("Do not add a separate `Fixes` section", template)
        self.assertNotIn("\n## Fixes\n", template)
        self.assertIn("pipeline appends the `Install` and `Roll back` sections", template)
        self.assertNotIn("## Release Qualification", template)
        self.assertNotIn("## Promotion Metadata", template)

    def test_v621_packet_documents_cached_update_verdict_age(self) -> None:
        release_notes = (
            _REPO_ROOT / "docs/releases/RELEASE_NOTES_v6.2.1.md"
        ).read_text(encoding="utf-8")
        changelog = (
            _REPO_ROOT / "docs/releases/V6_CHANGELOG_v6.2.1.md"
        ).read_text(encoding="utf-8")

        render_release_body.validate_release_notes_shape(release_notes, "6.2.1")
        self.assertIn('cached "Up to date" verdicts', release_notes)
        self.assertIn('cached "Up to date"', changelog)
        self.assertIn("#1601", release_notes)
        self.assertIn("#1601", changelog)

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

    def test_main_renders_concise_install_and_rollback_sections(self) -> None:
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
                    "require_windows_signing": "false",
                    "unsigned_windows_exception": "false",
                    "unsigned_windows_reason": "",
                },
            )()

            raw_text = Path(namespace.release_notes_file).read_text(encoding="utf-8")
            sanitized = render_release_body.sanitize_release_notes(raw_text, namespace.version).rstrip("\n")
            sections = [
                sanitized,
                render_release_body.build_installation_section(namespace.version),
                render_release_body.build_rollback_section(namespace),
            ]
            Path(namespace.output).write_text("\n\n".join(sections) + "\n", encoding="utf-8")

            body = output_file.read_text(encoding="utf-8")
            self.assertEqual(body.count("## Install"), 1)
            self.assertEqual(body.count("## Roll back"), 1)
            self.assertNotIn("## Promotion Metadata", body)
            self.assertIn("docker pull rcourtman/pulse:6.0.0-rc.2", body)
            self.assertIn("https://pulserelay.pro/download.html", body)
            self.assertIn("The rollback target is `v5.1.28`", body)
            self.assertIn("./scripts/install.sh --version v5.1.28", body)
            render_release_body.validate_release_body_shape(body, "6.0.0-rc.2")

    def test_release_body_accepts_visual_evidence_before_installation(self) -> None:
        notes = """# Pulse v6.4.0 Release Notes

Pulse is easier to use on the screens operators already carry.

## What's improved

- **Clearer small-screen controls** - Settings remain readable on phones.
"""
        visuals = """## See the difference

### Settings fit smaller screens

Controls remain readable without horizontal scrolling.

| Before | Now |
| --- | --- |
| ![Settings before](https://github.com/rcourtman/Pulse/releases/download/v6.4.0/release-note-settings-before.png) | ![Settings now](https://github.com/rcourtman/Pulse/releases/download/v6.4.0/release-note-settings-now.png) |
"""
        rollback_args = type(
            "Args",
            (),
            {
                "rollback_target": "v6.3.2",
                "rollback_command": "./scripts/install.sh --version v6.3.2",
            },
        )()
        body = "\n\n".join(
            [
                notes.strip(),
                visuals.strip(),
                render_release_body.build_installation_section("6.4.0"),
                render_release_body.build_rollback_section(rollback_args),
            ]
        ) + "\n"

        self.assertEqual(
            render_release_body.validate_release_body_shape(body, "6.4.0"),
            body,
        )

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

## Install

Install details.

## Roll back

Rollback details.
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

## Install

Install details.

## Roll back

Rollback details.
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
