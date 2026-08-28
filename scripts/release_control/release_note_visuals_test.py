#!/usr/bin/env python3

import importlib.util
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("release_note_visuals.py")
REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("release_note_visuals", MODULE_PATH)
assert SPEC and SPEC.loader
visuals = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(visuals)

RENDERER_PATH = Path(__file__).with_name("render_release_body.py")
RENDERER_SPEC = importlib.util.spec_from_file_location("render_release_body", RENDERER_PATH)
assert RENDERER_SPEC and RENDERER_SPEC.loader
renderer = importlib.util.module_from_spec(RENDERER_SPEC)
RENDERER_SPEC.loader.exec_module(renderer)


def valid_plan():
    return {
        "schema_version": 1,
        "captures": [
            {
                "id": "responsive-settings",
                "title": "Settings fit smaller screens",
                "description": "Controls remain readable without horizontal scrolling.",
                "viewport": {"width": 390, "height": 844},
                "before": {
                    "route": "/settings/general",
                    "steps": [],
                    "ready": {
                        "kind": "role",
                        "role": "heading",
                        "name": "General",
                        "exact": True,
                        "nth": 0,
                    },
                },
                "after": {
                    "route": "/settings/general",
                    "steps": [
                        {
                            "action": "wait",
                            "locator": {
                                "kind": "text",
                                "value": "Appearance",
                                "exact": True,
                                "nth": 0,
                            },
                        }
                    ],
                    "ready": {
                        "kind": "text",
                        "value": "Appearance",
                        "exact": True,
                        "nth": 0,
                    },
                },
            }
        ],
    }


class ReleaseNoteVisualPlanTest(unittest.TestCase):
    def test_normalizes_a_safe_accessible_capture_plan(self):
        plan = visuals.validate_plan(valid_plan())
        self.assertEqual(plan["captures"][0]["viewport"], {"width": 390, "height": 844})
        self.assertEqual(
            visuals.asset_names(plan),
            [
                "release-note-responsive-settings-before.png",
                "release-note-responsive-settings-now.png",
            ],
        )

    def test_structured_output_schema_carries_public_and_capture_bounds(self):
        schema = visuals.json_schema()
        self.assertEqual(schema["properties"]["schema_version"]["type"], "integer")
        captures = schema["properties"]["captures"]
        capture = captures["items"]["properties"]
        self.assertEqual(captures["maxItems"], visuals.MAX_CAPTURES)
        self.assertEqual(capture["description"]["maxLength"], 240)
        self.assertEqual(
            capture["after"]["properties"]["steps"]["maxItems"],
            visuals.MAX_STEPS,
        )

    def test_current_only_capture_has_one_asset(self):
        raw = valid_plan()
        raw["captures"][0]["before"] = None
        plan = visuals.validate_plan(raw)
        self.assertEqual(
            sum(capture["before"] is not None for capture in plan["captures"]),
            0,
        )
        self.assertEqual(
            visuals.asset_names(plan),
            ["release-note-responsive-settings-now.png"],
        )

    def test_rejects_external_routes_and_arbitrary_selectors(self):
        raw = valid_plan()
        raw["captures"][0]["after"]["route"] = "https://example.com/"
        with self.assertRaisesRegex(visuals.PlanError, "same-origin"):
            visuals.validate_plan(raw)

        raw = valid_plan()
        raw["captures"][0]["after"]["route"] = "/\\\\example.com/"
        with self.assertRaisesRegex(visuals.PlanError, "same-origin"):
            visuals.validate_plan(raw)

        raw = valid_plan()
        raw["captures"][0]["after"]["steps"][0]["locator"]["kind"] = "css"
        with self.assertRaisesRegex(visuals.PlanError, "kind must be"):
            visuals.validate_plan(raw)

    def test_requires_visible_content_for_every_capture_state(self):
        raw = valid_plan()
        del raw["captures"][0]["after"]["ready"]
        with self.assertRaisesRegex(visuals.PlanError, "visible content"):
            visuals.validate_plan(raw)

    def test_rejects_public_punctuation_disallowed_by_release_notes(self):
        raw = valid_plan()
        raw["captures"][0]["description"] = "Before; now"
        with self.assertRaisesRegex(visuals.PlanError, "semicolons"):
            visuals.validate_plan(raw)

    def test_renders_release_asset_links_as_before_and_now(self):
        plan = visuals.validate_plan(valid_plan())
        markdown = visuals.render_markdown(plan, "rcourtman/Pulse", "v6.4.0")
        self.assertIn("## See the difference", markdown)
        self.assertIn("| Before | Now |", markdown)
        self.assertIn(
            "https://github.com/rcourtman/Pulse/releases/download/v6.4.0/"
            "release-note-responsive-settings-before.png",
            markdown,
        )
        self.assertNotIn(";", markdown)
        self.assertNotIn("\u2014", markdown)

    def test_release_body_accepts_visuals_between_notes_and_installation(self):
        notes = "\n".join(
            [
                "# Pulse v6.4.0 Release Notes",
                "",
                "Pulse is easier to use across the devices you already carry.",
                "",
                "## What's improved",
                "",
                "- **Responsive settings** - Controls now fit smaller screens cleanly.",
            ]
        )
        visual_markdown = visuals.render_markdown(
            visuals.validate_plan(valid_plan()), "rcourtman/Pulse", "v6.4.0"
        ).strip()
        rollback = renderer.build_rollback_section(
            type(
                "Args",
                (),
                {
                    "rollback_target": "v6.3.2",
                    "rollback_command": "./scripts/install.sh --version v6.3.2",
                },
            )()
        )
        body = "\n\n".join(
            [
                notes,
                visual_markdown,
                renderer.build_installation_section("6.4.0"),
                rollback,
            ]
        ) + "\n"
        self.assertEqual(renderer.validate_release_body_shape(body, "6.4.0"), body)

    def test_release_pipeline_captures_uploads_and_verifies_selected_visuals(self):
        workflow = (REPOSITORY_ROOT / ".github/workflows/create-release.yml").read_text()
        self.assertIn("release_screenshot_plan:", workflow)
        self.assertIn("release_note_visuals:", workflow)
        self.assertIn("scripts/capture-release-note-visuals.sh", workflow)
        self.assertIn('--release-visuals-file "$VISUAL_MARKDOWN_FILE"', workflow)
        self.assertIn('release_upload_with_retry "$TAG" "release-note-visuals/${asset_name}"', workflow)
        self.assertIn(r'"release-note-\(.id)-now.png"', workflow)

        for trigger_name in ("trigger-release.sh", "trigger-stable-patch.sh"):
            trigger = (REPOSITORY_ROOT / "scripts" / trigger_name).read_text()
            self.assertIn("--rawfile release_screenshot_plan", trigger)
            self.assertIn("release_screenshot_plan: $release_screenshot_plan", trigger)


if __name__ == "__main__":
    unittest.main()
