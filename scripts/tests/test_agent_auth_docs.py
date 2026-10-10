#!/usr/bin/env python3
"""Keep passive agent-authentication triage aligned with the real install flow.

These source/document controls do not authenticate an installed agent or
establish a reporter's cause. No credential, service or target is accessed.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/UNIFIED_AGENT.md"


def section():
    text = DOC.read_text(encoding="utf-8")
    marker = "### Agent lookup returns HTTP 401 or 403\n"
    if marker not in text:
        raise AssertionError("missing agent authentication triage")
    return re.split(r"\n#{2,3} ", text.split(marker, 1)[1], 1)[0]


class AgentAuthDocsTest(unittest.TestCase):
    def test_status_explanations_match_authentication_before_scoped_lookup(self):
        text = " ".join(section().split())
        routes = (ROOT / "internal/api/router_routes_registration.go").read_text()
        self.assertIn(
            '"/api/agents/agent/lookup", RequireAuth(r.config, '
            'RequireScope(config.ScopeAgentReport, r.unifiedAgentHandlers.HandleLookup)',
            routes,
        )
        auth = (ROOT / "internal/api/auth.go").read_text()
        self.assertIn('http.Error(w, "Invalid API token", http.StatusUnauthorized)', auth)
        scope = (ROOT / "internal/api/apihttp/scope.go").read_text()
        denial = scope.split("func RespondMissingScope(", 1)[1]
        self.assertIn("w.WriteHeader(http.StatusForbidden)", denial)
        self.assertIn('"error":', denial)
        self.assertIn('"missing_scope"', denial)
        for fact in ("authenticates the request **before**", "**HTTP 401** is an authentication rejection",
                     "**HTTP 403** with `missing_scope`", "`agent:report`", "**HTTP 404**",
                     "upstream authentication layer"):
            self.assertIn(fact, text)

    def test_normal_install_is_not_conflated_with_bootstrap_enrolment(self):
        text = " ".join(section().split())
        install = (ROOT / "frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx").read_text()
        args = install.split("const getInstallerExtraArgs = () => [", 1)[1].split("];", 1)[0]
        self.assertNotIn("--enroll", args)
        scopes = (ROOT / "internal/api/agenttokens/install.go").read_text()
        host = scopes.split("func HostScopes(", 1)[1].split("\n}", 1)[0]
        self.assertIn("config.ScopeAgentReport", host)
        self.assertIn("config.ScopeAgentConfigRead", host)
        self.assertNotIn("ScopeAgentEnroll", host)
        prompt = (ROOT / "frontend-modern/src/utils/agentInstallCommand.ts").read_text()
        self.assertIn('read -r -s -p', prompt)
        self.assertIn('--token-file "$token_file"', prompt)
        for fact in ("silent token prompt", "does not change reporting permissions",
                     "It does not use `--enroll`", "bootstrap token for a runtime token",
                     "Do not substitute a bootstrap token", "normal host installation"):
            self.assertIn(fact, text)

    def test_triage_preserves_unknowns_state_and_private_credentials(self):
        text = " ".join(section().split())
        host = (ROOT / "internal/hostagent/agent.go").read_text()
        self.assertIn("Pulse rejected this agent's API token", host)
        client = (ROOT / "internal/remoteconfig/client.go").read_text()
        self.assertIn('Str("component", "remote_config_client")', client)
        for fact in ("bounded agent log reader", "affected agent host", "retrieval and does not establish",
                     "ordinary host reports were accepted", "successful-looking installer output does not prove",
                     "No host-report error", "inconclusive", "Do not restart, re-enrol, delete identity",
                     "same Pulse instance", "do not prove that the running service is using that record",
                     "service environments, token files and connection files private",
                     "Do not grant administrator scope", "disable authentication/TLS",
                     "manually redacted error line", "saved credentials unchanged",
                     "not itself a reason to reinstall or rotate every token"):
            self.assertIn(fact, text)
        self.assertNotIn("```", section(), "Reuse the bounded reader; do not add live credential probes")

    def test_shipped_help_matches_the_canonical_guide(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/UNIFIED_AGENT.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
