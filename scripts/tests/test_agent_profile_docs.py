#!/usr/bin/env python3
"""Bind profile help to existing authority, application and API boundaries.

Static source/document checks only. No agent, credential, listener, provider,
service, profile mutation or installed containment is exercised.
"""

import json
from pathlib import Path
import re
import shlex
import unittest


ROOT = Path(__file__).resolve().parents[2]
NAMES = ("CENTRALIZED_MANAGEMENT.md", "UNIFIED_AGENT.md")


def plain(text):
    return " ".join(text.replace("**", "").split())


class AgentProfileDocsTest(unittest.TestCase):
    # Override these texts for actual-parent/adverse controls without editing
    # source files or manufacturing a mirror failure.
    texts = None

    def setUp(self):
        texts = self.texts or {name: (ROOT / "docs" / name).read_text() for name in NAMES}
        self.central = texts[NAMES[0]]
        self.guide = plain(self.central)
        self.agent = plain(texts[NAMES[1]].split(
            "## Remote Configuration (Agent Profiles, Pro/legacy Pro+/Cloud)", 1)[1].split(
                "\n## Uninstall", 1)[0])

    def test_shipped_mirrors_match(self):
        for name in NAMES:
            with self.subTest(name=name):
                self.assertEqual((ROOT / "docs" / name).read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / name).read_bytes())

    def test_supported_keys_match_the_existing_schema_and_startup_consumer(self):
        schema = (ROOT / "internal/models/profile_validation.go").read_text()
        keys = re.findall(r'Key:\s+"([^"]+)"', schema.split(
            "var ValidConfigKeys = []ConfigKeyDefinition{", 1)[1].split(
                "// ValidationError", 1)[0])
        table = self.central.split("## Supported Configuration Keys", 1)[1].split(
            "### Monitoring", 1)[0]
        rows = re.findall(r"^\| `([^`]+)` \|", table, re.MULTILINE)
        self.assertEqual(rows, keys)
        startup = (ROOT / "cmd/pulse-agent/main.go").read_text().split(
            "func applyRemoteSettings(", 1)[1]
        for key in keys:
            with self.subTest(key=key):
                self.assertIn(f'case "{key}":', startup)
        self.assertIn("JSON numbers are interpreted as seconds", self.guide)

    def test_host_metrics_do_not_grant_command_authority(self):
        row = next(line for line in self.central.splitlines() if line.startswith("| `enable_host`"))
        self.assertIn("metrics collection only; does not grant command execution", row)
        self.assertNotIn("metrics + command execution", self.central)
        for phrase in ("not a profile key", "installed local authority", "agent:exec",
                       "permitted agent binding", "monitoring-only runtime rejects remote",
                       "cannot promote that runtime", "Leave commands off for monitoring"):
            self.assertIn(phrase, self.guide)
        authority = (ROOT / "internal/hostagent/agent.go").read_text()
        self.assertIn("ResolveCommandAuthority(a.commandAuthorityProfile, commandsEnabled)", authority)
        self.assertIn("commandConfigAllowedForToken", (ROOT / "internal/api/agent_ingest.go").read_text())

    def test_live_refresh_is_not_a_blanket_restart_or_live_reload_claim(self):
        main = (ROOT / "cmd/pulse-agent/main.go").read_text()
        self.assertRegex(main, r"remoteConfigRefreshInterval\s*=\s*1 \* time.Minute")
        live = (ROOT / "internal/hostagent/agent.go").read_text().split(
            "func remoteConfigAppliedWithoutRestart(", 1)[1].split("\n}", 1)[0]
        self.assertIn('case "interval", "report_ip", "disable_ceph", availabilitySettingsKey:', live)
        for text in (self.guide, self.agent):
            self.assertIn("host module running", text)
            self.assertIn("about once a minute", text)
            for key in ("interval", "report_ip", "disable_ceph"):
                self.assertIn(f"`{key}`", text)
            self.assertIn("successful startup fetch", text)
            self.assertIn("not an all-module live reload", text)
            self.assertNotIn("Profile changes take effect on the next agent restart", text)
        self.assertIn("keeps its current settings if a refresh fails", self.guide)
        self.assertIn("do not disable signature or TLS verification", self.guide)

    def test_rollout_keeps_local_opt_outs_and_passive_acceptance(self):
        for phrase in ("hard opt-out", "cannot turn Docker/Podman collection back on",
                       "shared profile change affects every agent", "Validate",
                       "not host permissions", "non-production agent", "Read back both records",
                       "normal config fetches and fresh reports", "not proof that every setting",
                       "Do not restart, re-enrol or change a host identity",
                       "Monitoring is interrupted during restart", "independent coverage"):
            self.assertIn(phrase, self.guide)
        startup = (ROOT / "cmd/pulse-agent/main.go").read_text()
        self.assertIn("b && cfg.DockerExplicitlyDisabled", startup)

    def test_removal_and_rollback_are_not_emergency_revocation(self):
        for phrase in ("does not undo settings already applied", "revoke its token",
                       "revoke command authority", "Unassignment is not an emergency stop",
                       "new version", "every affected agent", "local service procedure"):
            self.assertIn(phrase, self.guide)
        self.assertIn("does not undo settings already applied", self.agent)
        rollback = (ROOT / "internal/api/config_profiles.go").read_text().split(
            "func (h *ConfigProfileHandler) RollbackProfile(", 1)[1]
        self.assertIn("profiles[i].Version = p.Version + 1", rollback)
        self.assertIn("selected organisation's Pulse config storage", self.guide)

    def test_api_access_uses_real_ids_scope_and_desired_not_applied_state(self):
        for phrase in ("administrator access, `settings:write` token scope",
                       "`agent_profiles` licence capability", "returned profile `id`",
                       "not `prod-servers`", "server's desired configuration",
                       "not an observation of what the process applied",
                       "restricted by token binding", "never give an agent an administrator token",
                       "before considering another write", "Do not blindly repeat POST"):
            self.assertIn(phrase, self.guide)
        routes = (ROOT / "internal/api/router_routes_registration.go").read_text()
        self.assertIn("RequireScope(config.ScopeSettingsWrite, RequireLicenseFeature", routes)
        handler = (ROOT / "internal/api/config_profiles.go").read_text()
        self.assertIn("input.ID = uuid.New().String()", handler)
        self.assertIn("if p.ID == input.ProfileID", handler)
        bodies = [json.loads(body) for body in re.findall(r"```json\n(.*?)```", self.central, re.DOTALL)]
        self.assertEqual(bodies[1]["profile_id"], "<returned-profile-uuid>")
        self.assertEqual(set(bodies[0]), {"name", "config"})

    def test_examples_keep_credentials_and_infrastructure_responses_private(self):
        commands = re.findall(r"```bash\n(.*?)```", self.central, re.DOTALL)
        self.assertEqual(len(commands), 1)
        for line in commands[0].splitlines():
            if line and not line.startswith("#"):
                words = shlex.split(line)
                self.assertEqual(words[:2], ["pulse_api", "GET"])
                self.assertTrue(words[2].startswith("/api/admin/profiles/"))
        self.assertNotIn("```http", self.central)
        for phrase in ("private header file", "same Bash session", "no redirects",
                       "automatic retries", "Never paste a token or session cookie",
                       "Copy as cURL", "not full responses", "manually redacted excerpt",
                       "may call the configured AI provider", "not a read-only diagnostic",
                       "not an empty profile list", "GET and POST only, not PUT or DELETE"):
            self.assertIn(phrase, self.guide)


if __name__ == "__main__":
    unittest.main()
