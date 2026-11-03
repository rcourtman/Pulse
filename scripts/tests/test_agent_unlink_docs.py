#!/usr/bin/env python3
"""Keep the non-destructive agent unlink recipe aligned with its producer."""

import json
from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


class AgentUnlinkDocsTest(unittest.TestCase):
    def test_example_matches_the_handler_and_client_request(self):
        doc = (ROOT / "docs/API.md").read_text()
        section = doc.split("`POST /api/agents/agent/unlink`", 1)[1].split(
            "`DELETE /api/agents/agent/{agent_id}`", 1
        )[0]
        payload = json.loads(re.search(r"```json\n(.*?)\n```", section, re.S)[1])
        handler = (ROOT / "internal/api/agent_ingest.go").read_text().split(
            "func (h *UnifiedAgentHandlers) HandleUnlink(", 1
        )[1].split("\nfunc ", 1)[0]
        schema = handler.split("var req struct {", 1)[1].split("\n\t}", 1)[0]
        self.assertEqual(set(payload), set(re.findall(r'json:"([^",]+)', schema)))
        self.assertEqual(payload, {"agentId": "canonical-agent-id"})
        self.assertIn("monitor.UnlinkHostAgent(agentID)", handler)
        client = (ROOT / "frontend-modern/src/api/monitoring.ts").read_text().split(
            "static async unlinkAgent(", 1
        )[1].split("\n  static ", 1)[0]
        self.assertIn("/agents/agent/unlink", client)
        self.assertRegex(client, r"JSON.stringify\(\{\s*agentId,")
        self.assertIn("(admin, `settings:write`)", section)
        self.assertNotIn("curl ", section)

    def test_separation_is_distinguished_from_removal_and_resource_unlink(self):
        doc = (ROOT / "docs/API.md").read_text()
        section = doc.split("`POST /api/agents/agent/unlink`", 1)[1].split(
            "`DELETE /api/agents/agent/{agent_id}`", 1
        )[0]
        self.assertIn("agent keeps reporting", section)
        self.assertIn("including after a server restart", section)
        self.assertIn("unknown provenance", section)
        self.assertIn("`targetId`", section)
        self.assertIn("CSRF", section)
        self.assertIn("Do not delete, reinstall or re-enrol", section)

    def test_help_keeps_ambiguous_identity_and_existing_state_safe(self):
        doc = (ROOT / "docs/TROUBLESHOOTING.md").read_text()
        section = doc.split("#### A NAS is shown as a Proxmox node", 1)[1].split(
            "\n#### ", 1
        )[0]
        self.assertIn("API.md#agent-management", section)
        self.assertIn("confirmed to be different machines", section)
        self.assertIn("alert coverage afterwards", section)
        self.assertIn("If the target or link is\nuncertain, retain it", section)
        self.assertIn("not full API responses or configuration files", section)

    def test_both_guides_match_the_shipped_documents(self):
        for name in ("API.md", "TROUBLESHOOTING.md"):
            with self.subTest(name=name):
                self.assertEqual(
                    (ROOT / "docs" / name).read_bytes(),
                    (ROOT / "frontend-modern/public/docs" / name).read_bytes(),
                )


if __name__ == "__main__":
    unittest.main()
