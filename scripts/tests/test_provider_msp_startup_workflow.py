#!/usr/bin/env python3
"""Keep the Provider MSP startup run on changes to its actual inputs."""

from pathlib import Path
import re
import unittest

import yaml


WORKFLOWS = Path(__file__).resolve().parents[2] / ".github/workflows"


def triggers(workflow):
    return yaml.load((WORKFLOWS / workflow).read_text(), Loader=yaml.BaseLoader)["on"]


def triggered(paths, patterns):
    """Apply the ordered GitHub paths rules for the literal/*/** subset here."""
    for path in paths:
        included = False
        for rule in patterns:
            exclude = rule.startswith("!")
            pattern = re.split(r"(\*\*|\*)", rule.removeprefix("!"))
            expression = "".join(
                ".*" if part == "**" else "[^/]*" if part == "*" else re.escape(part)
                for part in pattern
            )
            if re.fullmatch(expression, path):
                included = not exclude
        if included:
            return True
    return False


class ProviderMSPStartupWorkflowTest(unittest.TestCase):
    def test_help_markdown_keeps_docs_checks_without_a_compose_runner(self):
        provider = triggers("provider-msp-startup.yml")
        docs = triggers("public-docs.yml")
        for event in ("pull_request", "push"):
            for guide in ("PRODUCTION_SECURITY.md", "PBS.md", "i18n/fr/README.md"):
                paths = [f"docs/{guide}", f"frontend-modern/public/docs/{guide}"]
                with self.subTest(event=event, guide=guide):
                    self.assertFalse(triggered(paths, provider[event]["paths"]))
                    self.assertTrue(triggered(paths, docs[event]["paths"]))

    def test_mixed_diffs_still_prove_exact_source_startup(self):
        provider = triggers("provider-msp-startup.yml")
        for event in ("pull_request", "push"):
            self.assertEqual(provider[event]["branches"], ["main"])
            for path in (
                "internal/cloudcp/server.go",
                "cmd/pulse-control-plane/main.go",
                "pkg/licensing/manager.go",
                "deploy/provider-msp/compose.yml",
                "scripts/installtests/provider_msp_startup_test.go",
                "frontend-modern/src/App.tsx",
                "frontend-modern/public/docs/diagram.svg",
                "frontend-modern/public/help.md",
                "frontend-modern/package-lock.json",
                "go.mod",
                "go.sum",
                ".github/workflows/provider-msp-startup.yml",
            ):
                with self.subTest(event=event, path=path):
                    self.assertTrue(triggered(
                        ["frontend-modern/public/docs/PBS.md", path],
                        provider[event]["paths"],
                    ))


if __name__ == "__main__":
    unittest.main()
