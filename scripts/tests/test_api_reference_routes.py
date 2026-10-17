#!/usr/bin/env python3
"""Keep docs/API.md aligned with the routes and scopes the server registers.

Static checks only: no server, credentials or network. Each check reads the
reference and a router or handler source file, so drift in either direction
fails here instead of surfacing as a support question.
"""

from __future__ import annotations

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/API.md"
SCOPES = ROOT / "pkg/auth/scopes.go"

SCOPE_TOKEN = re.compile(r"`([a-z]+(?::[a-z]+)+)`")
ROUTE_LINE = re.compile(r"`(?:GET|POST|PUT|PATCH|DELETE)(?:/(?:GET|POST|PUT|PATCH|DELETE))* /api/")


def read(path: str | Path) -> str:
    return (ROOT / path).read_text(encoding="utf-8") if isinstance(path, str) else path.read_text(encoding="utf-8")


def known_scopes(text: str) -> set[str]:
    return set(re.findall(r'Scope\w+\s*=\s*"([^"]+)"', text))


def unknown_scopes_in_reference(doc: str, scopes: set[str]) -> list[str]:
    """Scope-looking tokens on route lines that no code constant defines.

    Only tokens whose first segment is a real scope family are judged, so a
    request value such as `provider:model` is not mistaken for a scope.
    """
    families = {scope.split(":")[0] for scope in scopes}
    unknown: list[str] = []
    for line in doc.splitlines():
        if not ROUTE_LINE.search(line):
            continue
        for token in SCOPE_TOKEN.findall(line):
            if token.split(":")[0] in families and token not in scopes:
                unknown.append(f"{token} in: {line.strip()[:100]}")
    return unknown


def missing_anchors(pairs: list[tuple[str, str, str]], doc: str, sources: dict[str, str]) -> list[str]:
    """Each pair is (text the reference must contain, source file, text the code must contain)."""
    failures: list[str] = []
    for doc_text, source, code_text in pairs:
        if doc_text not in doc:
            failures.append(f"reference lacks: {doc_text}")
        if code_text not in sources[source]:
            failures.append(f"{source} lacks: {code_text}")
    return failures


# (reference text, source file, code text). Routes this guide documents on the
# strength of the registration or handler named here.
ROUTE_ANCHORS = [
    ("`GET /api/ai/patrol/runs/{id}` (`ai:execute`)",
     "internal/api/router_routes_ai_relay.go",
     'r.mux.HandleFunc("/api/ai/patrol/runs/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute,'),
    ("`POST /api/availability-history` (`monitoring:read`)",
     "internal/api/router_routes_monitoring.go",
     'r.mux.HandleFunc("/api/availability-history", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead,'),
    ("`POST /api/agents/agent/{agent_id}/allow-reenroll` (admin, `settings:write`)",
     "internal/api/router_routes_registration.go",
     'strings.HasSuffix(req.URL.Path, "/allow-reenroll")'),
    ("`PUT /api/truenas/connections/{id}` (admin, `settings:write`)",
     "internal/api/router_routes_registration.go",
     "update:  r.trueNASHandlers.HandleUpdate,"),
    ("`POST /api/truenas/connections/{id}/test` (admin, `settings:write`)",
     "internal/api/router_routes_registration.go",
     "test:    r.trueNASHandlers.HandleTestSavedConnection,"),
    # Every profile route is wrapped once, for all methods, including GET.
    ("Token callers also need `settings:write` on every profile route, including GET requests",
     "internal/api/router_routes_registration.go",
     'r.mux.Handle("/api/admin/profiles/", RequireOrgAdmin(r.config, RequireScope(config.ScopeSettingsWrite,'),
    # Everything other than GET or HEAD on the /api/resources/ prefix needs the write scope.
    ("`POST /api/resources/{id}/link` (`monitoring:write`)",
     "internal/api/router_routes_monitoring.go",
     "if req.Method != http.MethodGet && req.Method != http.MethodHead {\n\t\t\tscope = config.ScopeMonitoringWrite"),
    ("`POST /api/resources/{id}/unlink` (`monitoring:write`)",
     "internal/api/router_routes_monitoring.go",
     "if req.Method != http.MethodGet && req.Method != http.MethodHead {\n\t\t\tscope = config.ScopeMonitoringWrite"),
    ("`POST /api/resources/{id}/report-merge` (`monitoring:write`)",
     "internal/api/router_routes_monitoring.go",
     "if req.Method != http.MethodGet && req.Method != http.MethodHead {\n\t\t\tscope = config.ScopeMonitoringWrite"),
    # Regeneration replaces the whole inventory, so a token caller must be instance-wide.
    ("An API token may call this only if it has full-access\nscope (`*`) and is not bound to a non-default organization",
     "internal/api/security_setup_fix.go",
     "!tokenHasInstanceWideAuthority(callerToken)"),
    ("`POST /api/agents/agent/uninstall` (`agent:report` token, or an admin session)",
     "internal/api/router_routes_registration.go",
     "RequireOrgAdmin(r.config, RequireScope(config.ScopeAgentReport, r.unifiedAgentHandlers.HandleUninstall))"),
    ("API-token callers also need the `ai:execute` scope for AI narration.",
     "internal/api/metrics_reporting_handlers.go",
     "if !reportCallerMayUseAI(ctx) {\n\t\treturn nil, nil, h.resolveExistingFindings(ctx)"),
    ("AI narration of the fleet summary follows the same rule as single reports",
     "internal/api/metrics_reporting_handlers.go",
     "_, fleetNarrator, findings := h.resolveNarrator(ctx)"),
    ("API-token callers manage only the tokens in their own organizations",
     "internal/api/security_tokens.go",
     "if !callerTokenMayManage(callerToken, &record) {"),
    ("`POST /api/security/tokens/<id>/rotate`",
     "internal/api/router_routes_auth_security.go",
     "r.handleRotateAPIToken(w, req)"),
    ("from an API token needs an instance-wide full-access token",
     "internal/api/security_setup_fix.go",
     "API token without instance-wide authority attempted quick security setup"),
    ("`PUT /api/notifications/email` (admin, `settings:write`)",
     "internal/api/alerting/notifications.go",
     'case path == "/email" && r.Method == http.MethodPut:\n\t\tif !requireAnyScope(config.ScopeSettingsWrite'),
]


class APIReferenceRoutesTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.doc = read(DOC)
        cls.scopes = known_scopes(read(SCOPES))
        cls.sources = {source: read(source) for _, source, _ in ROUTE_ANCHORS}

    def test_scope_catalogue_was_parsed(self) -> None:
        for scope in ("monitoring:read", "settings:write", "ai:execute", "audit:read"):
            self.assertIn(scope, self.scopes)

    def test_every_scope_named_on_a_route_line_exists_in_code(self) -> None:
        self.assertEqual(unknown_scopes_in_reference(self.doc, self.scopes), [])

    def test_documented_routes_match_their_registration(self) -> None:
        self.assertEqual(missing_anchors(ROUTE_ANCHORS, self.doc, self.sources), [])

    def test_retired_remediation_routes_say_so(self) -> None:
        for route in ("approve", "execute", "rollback"):
            line = next(l for l in self.doc.splitlines() if l.startswith(f"- `POST /api/ai/remediation/{route}`"))
            self.assertIn("410 Gone", line, route)
            self.assertIn("command_remediation_retired", line, route)

    def test_patrol_routes_documented_without_a_paid_label_have_no_licence_wrapper(self) -> None:
        # Patrol and managing its findings are free; only governed fixes are Pro.
        router = read("internal/api/router_routes_ai_relay.go")
        for name in ("stream", "run", "acknowledge", "snooze", "suppress", "suppressions", "dismissed"):
            route = f"/api/ai/patrol/{name}"
            bullets = [line for line in self.doc.splitlines()
                       if re.match(r"- `(?:GET|POST|DELETE) %s(?:/\{id\})?`" % re.escape(route), line)]
            self.assertTrue(bullets, route)
            for bullet in bullets:
                self.assertNotIn("Pro", bullet)
            start = router.index(f'HandleFunc("{route}"')
            following = router.find("r.mux.Handle", start + 10)
            registration = router[start:following if following != -1 else start + 900]
            self.assertNotIn("RequireLicenseFeature", registration, route)

    def test_reapprove_is_documented_as_retired(self) -> None:
        entry = self.doc.split("`POST /api/ai/findings/{id}/reapprove`", 1)[1].split("\n\n", 1)[0]
        for text in ("retired", "410 Gone", "command_fix_retired"):
            self.assertIn(text, entry)

    def test_validate_token_is_not_filed_under_the_licensed_audit_log(self) -> None:
        audit = self.doc.split("## 🧾 Audit Log (Pro)", 1)[1].split("\n## ", 1)[0]
        self.assertNotIn("/api/security/validate-token", audit)
        self.assertIn("/api/security/validate-token", self.doc)

    def test_negative_controls_fail_when_reference_or_code_drift(self) -> None:
        # An invented scope on a route line is reported.
        drifted = self.doc.replace("`POST /api/availability-history` (`monitoring:read`)",
                                   "`POST /api/availability-history` (`monitoring:peek`)")
        self.assertNotEqual(drifted, self.doc)
        self.assertTrue(unknown_scopes_in_reference(drifted, self.scopes))
        # A route dropped from the reference is reported.
        without_route = self.doc.replace("`GET /api/ai/patrol/runs/{id}` (`ai:execute`)", "")
        self.assertTrue(missing_anchors(ROUTE_ANCHORS, without_route, self.sources))
        # A route whose wrapper loses its scope in code is reported.
        weakened = dict(self.sources)
        key = "internal/api/router_routes_monitoring.go"
        weakened[key] = weakened[key].replace("RequireScope(config.ScopeMonitoringRead,", "RequireScope(config.ScopeSettingsRead,")
        self.assertTrue(missing_anchors(ROUTE_ANCHORS, self.doc, weakened))


if __name__ == "__main__":
    unittest.main()
