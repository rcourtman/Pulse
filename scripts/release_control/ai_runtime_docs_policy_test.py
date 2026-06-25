#!/usr/bin/env python3
"""Guard public Pulse Intelligence docs against retired AI shorthand."""

from __future__ import annotations

import unittest

from repo_file_io import read_repo_text

PUBLIC_AI_DOC_PATHS = (
    "docs/AI.md",
    "docs/AI_AUTONOMY.md",
    "docs/API.md",
    "docs/FAQ.md",
    "docs/PULSE_PRO.md",
    "docs/README.md",
)


class AIRuntimeDocsPolicyTest(unittest.TestCase):
    def test_public_ai_overview_uses_productized_context_language(self) -> None:
        content = read_repo_text("docs/AI.md")
        normalized_content = " ".join(content.split())

        self.assertIn("# Pulse Intelligence", content)
        self.assertIn(
            "Uses prior alert context, Patrol run history, and resource timelines",
            content,
        )
        self.assertIn("Investigation Context", content)
        self.assertIn("the configured LLM owns diagnosis", content)
        self.assertIn(
            "Pulse supplies context, capabilities, safety gates, approval state, and audit trails",
            content,
        )
        self.assertIn("shared **Pulse Intelligence Core**", content)
        self.assertIn("<!-- pulse-intelligence-overview:start -->", content)
        self.assertIn("<!-- pulse-intelligence-overview:end -->", content)
        self.assertIn(
            "Canonical context, governed actions, safety gates, approval state, action audit, and verification",
            normalized_content,
        )
        self.assertIn("Patrol as the primary built-in operator", normalized_content)
        self.assertIn(
            "Pulse Patrol**: Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened.",
            normalized_content,
        )
        self.assertIn(
            "Pulse Assistant**: The contextual explanation, approval, and handoff surface for Patrol findings, governed actions, verification, and operator questions. Affordances: tools and interactive questions.",
            normalized_content,
        )
        self.assertIn(
            "Pulse MCP**: The external-agent adapter that projects canonical Pulse Intelligence capabilities as MCP tools. Affordances: tools, resources, prompts, and capability metadata.",
            normalized_content,
        )
        self.assertIn("Pulse MCP", content)
        self.assertIn("### Tool Inventory", content)
        self.assertIn("The Assistant tool list is registry-owned at runtime", content)
        self.assertIn(
            "Each turn receives an available-tool manifest generated from Pulse's governed tool registry",
            normalized_content,
        )
        self.assertIn(
            "tool-result handling, approval boundaries, and Patrol-only tool filtering",
            normalized_content,
        )
        self.assertIn(
            "use the live `/api/agent/capabilities` manifest or `pulse-mcp` `tools/list`",
            normalized_content,
        )
        self.assertIn(
            "The same manifest also carries reusable `workflowPrompts` metadata",
            normalized_content,
        )
        self.assertIn(
            "Pulse Assistant-compatible starters and MCP `prompts/list` clients discover the same fleet triage, resource investigation, and Patrol finding review workflows",
            normalized_content,
        )
        self.assertIn(
            "Pulse Assistant and `pulse-mcp` are sibling surfaces over Pulse Intelligence",
            normalized_content,
        )
        self.assertIn("Patrol mode and governed Pulse Intelligence operations", normalized_content)
        self.assertIn("not competing implementations, and neither replaces the other", normalized_content)
        self.assertIn(
            "New operational capabilities should be added to the canonical API manifest first",
            normalized_content,
        )
        self.assertIn(
            "MCP-only actions and Assistant-only copies of the same business logic are drift",
            normalized_content,
        )
        self.assertNotIn("two supported operator-facing surfaces", normalized_content)
        self.assertNotIn("Pulse Assistant should be replaced by", normalized_content)
        self.assertNotIn("MCP replaces Pulse Assistant", normalized_content)
        self.assertNotIn("Assistant can be removed", normalized_content)
        self.assertIn("- **Anthropic** (API key)", content)
        self.assertIn("Anthropic OAuth is not a supported runtime authentication method", normalized_content)
        self.assertIn("does not make Anthropic configured", normalized_content)
        self.assertIn("go run ./cmd/eval -scenario resource-context", content)
        self.assertIn("EVAL_RESOURCE_CONTEXT_FORBIDDEN", content)
        self.assertIn("Pulse does not convert them into Pulse-authored findings", content)
        self.assertNotIn("Anthropic** (API key or OAuth)", content)
        self.assertNotIn("Anthropic API key or OAuth", content)
        self.assertNotIn("successful remediations (incident memory)", content)
        self.assertNotIn("**Incident memory**", content)
        self.assertNotIn("**Incident Memory**", content)
        self.assertNotIn("incident memory, investigation orchestration", content)
        self.assertNotIn("learns what's normal", content)
        self.assertNotIn("multi-layered intelligence platform", content)
        self.assertNotIn("capacity predictions", content)
        self.assertNotIn("Deterministic Signal Detection", content)
        self.assertNotIn("Pulse AI is built", content)
        self.assertNotIn("counts and feature flags only", normalized_content)
        self.assertNotIn("### Available Tools", content)
        self.assertNotIn("| Tool | Classification | Purpose |", content)
        self.assertNotIn("| `pulse_query`, `pulse_discovery` |", content)
        self.assertNotIn("| `patrol_report_finding` | Patrol |", content)
        self.assertNotIn("active_alert", content)
        self.assertNotIn("auto-recovery", content)
        self.assertNotIn('EVAL_RESOURCE_CONTEXT_FORBIDDEN="/mnt/pve/finance-db,/var/lib/homeassistant,secret"', content)
        self.assertNotRegex(content, r"(?i)understands resources before you ask")

    def test_public_ai_docs_use_current_surface_naming(self) -> None:
        for doc_path in PUBLIC_AI_DOC_PATHS:
            with self.subTest(doc_path=doc_path):
                content = read_repo_text(doc_path)
                self.assertNotIn("Pulse AI", content)
                self.assertNotIn("Settings → System → AI Assistant", content)
                self.assertNotIn("Settings > System > AI Assistant", content)
                self.assertNotIn("Settings > Pulse Assistant", content)

    def test_public_patrol_docs_use_mode_language(self) -> None:
        overview = read_repo_text("docs/AI.md")
        control_doc = read_repo_text("docs/AI_AUTONOMY.md")

        self.assertIn("Pro adds hands-on Patrol modes, issue investigation, governed fixes, verified outcomes, and 90-day history", overview)
        self.assertIn("## Patrol Modes", overview)
        self.assertIn("four modes", overview)
        self.assertIn("**Watch only**", overview)
        self.assertIn("**Ask before changes**", overview)
        self.assertIn("**Auto-fix safe issues**", overview)
        self.assertIn("**Policy autopilot**", overview)
        self.assertIn("When a finding is created in a Pro Patrol mode", overview)
        self.assertIn("**Patrol modes (Pro and above)**", overview)
        self.assertIn("**Governed fixes (Pro and above)**", overview)
        self.assertIn("**Issue investigation (Pro and above)**", overview)
        self.assertIn("Patrol mode and governed Pulse Intelligence operations", overview)
        self.assertNotIn("## Patrol Control Levels", overview)
        self.assertNotIn("four control levels", overview)
        self.assertNotIn("When a finding is created in a Pro control level", overview)
        self.assertNotIn("**Patrol control (Pro and above)**", overview)
        self.assertNotIn("**Alert investigation (Pro and above)**", overview)
        self.assertNotIn("Patrol control and governed Pulse Intelligence operations", overview)
        self.assertNotIn("## Autonomy Levels", overview)
        self.assertNotIn("autonomy modes", overview)

        self.assertIn("# Pulse Intelligence Modes and Safety Configuration", control_doc)
        self.assertIn("**Patrol Mode**", control_doc)
        self.assertIn("Patrol mode is configured on the **Patrol** page.", control_doc)
        self.assertIn("## Patrol Modes", control_doc)
        self.assertIn("Patrol mode sets how far Pulse can go", control_doc)
        self.assertIn("**UI:** Patrol → Patrol mode", control_doc)
        self.assertIn("The API keeps the autonomy_level field name for compatibility.", control_doc)
        self.assertNotIn("Patrol Control Level", control_doc)
        self.assertNotIn("## Patrol Control Levels", control_doc)
        self.assertNotIn("Patrol control is configured on the **Patrol** page.", control_doc)
        self.assertNotIn("Patrol control sets how far Pulse can go", control_doc)
        self.assertNotIn("**UI:** Patrol → Patrol control", control_doc)
        self.assertNotIn("Patrol Autonomy Levels", control_doc)
        self.assertNotIn("Patrol autonomy controls", control_doc)
        self.assertNotIn("Patrol Autonomy Level", control_doc)


if __name__ == "__main__":
    unittest.main()
