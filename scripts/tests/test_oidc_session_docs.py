#!/usr/bin/env python3
"""Keep OIDC help honest about existing session and revocation boundaries.

Documentation/source consistency only: this does not exercise an IdP, browser
session, token exchange or installed access removal.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDE = ROOT / "docs/OIDC.md"


def guide_section(start, end):
    text = GUIDE.read_text(encoding="utf-8")
    return " ".join(text.split(start, 1)[1].split(end, 1)[0].split())


class OIDCSessionDocsTest(unittest.TestCase):
    def test_cookie_server_and_idp_lifetimes_are_distinct(self):
        text = guide_section("### Sessions and `offline_access`", "#### Changing or removing access")
        for expected in (
            "different lifetimes", "does not guarantee a 30- or 90-day",
            "actually issue a refresh token", "SSO can work without",
            "24-hour lifetime", "sliding expiry", "not extend the browser cookie",
            "request-driven, not continuous polling", "before refresh completes",
        ):
            self.assertIn(expected, text)

    def test_token_exchange_failure_is_not_confused_with_missing_provider(self):
        text = guide_section("### Sessions and `offline_access`", "#### Changing or removing access")
        for expected in (
            "token exchange", "invalidates that Pulse session",
            "Failure to initialise the provider", "no matching enabled provider",
            "skips refresh", "changed issuer/client ID", "encrypted when persisted",
            "never publish session files",
        ):
            self.assertIn(expected, text)
        self.assertNotIn("automatically invalidated", text)

    def test_login_claims_and_provider_changes_do_not_promise_revocation(self):
        text = guide_section("#### Changing or removing access", "## 📚 Provider Examples")
        for expected in (
            "every non-empty restriction must pass", "applied at login",
            "does **not** re-evaluate", "existing Pulse session or role assignments",
            "Disabling or deleting a provider", "must not be treated as revoking",
            "independent administrator login", "Block future sign-ins",
            "Pro RBAC", "RBAC.md#removing-user-access", "provider-scoped identity",
            "active Pulse sessions", "later authorised SSO login can recreate",
            "not proof that an old session lost access", "targeted user removal is unavailable",
            "network or authenticated reverse-proxy access boundary",
            "blocking only the IdP login is not enough", "Do not disable Pulse authentication",
        ):
            self.assertIn(expected, text)

    def test_diagnosis_does_not_request_debug_restart_or_secrets(self):
        text = guide_section("### Safe diagnosis", "\0")
        for expected in (
            "existing login error", "claim names locally", "not paste a full IdP response",
            "Do not enable Debug or restart", "locally reviewed, redacted error",
            "client secrets", "authorization codes", "ID/access/refresh tokens",
            "full callback query strings", "HAR exports", "Copy as cURL",
            "Never disable TLS verification or access restrictions",
        ):
            self.assertIn(expected, text)
        self.assertNotIn("```", text)
        self.assertNotIn("LOG_LEVEL=debug", GUIDE.read_text())

    def test_session_and_refresh_claims_match_source(self):
        router = (ROOT / "internal/api/router.go").read_text()
        session = router.split("func (r *Router) establishOIDCSession(", 1)[1].split(
            "// handleLogin", 1
        )[0]
        self.assertIn("24*time.Hour", session)
        self.assertEqual(session.count("MaxAge:   86400") + session.count("MaxAge: 86400"), 2)
        store = (ROOT / "internal/api/session_store.go").read_text()
        sliding = store.split("func (s *SessionStore) ValidateAndExtendSession(", 1)[1]
        self.assertIn("now.After(session.ExpiresAt)", sliding)
        self.assertIn("session.ExpiresAt = now.Add(session.OriginalDuration)", sliding)
        auth = (ROOT / "internal/api/auth.go").read_text()
        self.assertIn("go refreshOIDCSessionTokens(cfg, cookie.Value, session)", auth)
        refresh = auth.split("func refreshOIDCSessionTokens(", 1)[1]
        self.assertIn("if oidcCfg == nil", refresh)
        self.assertIn("if err != nil", refresh)
        self.assertIn("GetSessionStore().InvalidateSession(sessionToken)", refresh)
        self.assertIn("GetSessionStore().UpdateOIDCTokens", refresh)
        self.assertNotIn("resolveGroupRoles", refresh)
        self.assertNotIn("AllowedGroups", refresh)

    def test_claim_gating_and_targeted_removal_match_source(self):
        callback = (ROOT / "internal/api/oidc_handlers.go").read_text()
        for marker in ("len(provider.AllowedEmails) > 0", "len(provider.AllowedDomains) > 0",
                       "len(provider.AllowedGroups) > 0", "applySSORoleAssignments"):
            self.assertIn(marker, callback)
        providers = (ROOT / "internal/api/identity_sso_handlers.go").read_text()
        mutations = providers.split("func (r *Router) handleUpdateSSOProvider(", 1)[1].split(
            "func (r *Router) saveSSOConfig(", 1
        )[0]
        self.assertNotIn("InvalidateUserSessions", mutations)
        self.assertNotIn("InvalidateSession", mutations)
        remove = (ROOT / "internal/api/access_control_handlers.go").read_text()
        self.assertIn("InvalidateUserSessions(username)", remove)
        self.assertIn('"Cannot remove your own user access"', remove)
        rbac = (ROOT / "docs/RBAC.md").read_text()
        self.assertIn("### Removing User Access", rbac)
        self.assertIn("Requires:", rbac)

    def test_served_guide_matches_source(self):
        self.assertEqual(GUIDE.read_bytes(), (ROOT / "frontend-modern/public/docs/OIDC.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
