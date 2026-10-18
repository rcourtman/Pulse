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


def assert_callback_setup(case, guide):
    setup = " ".join(guide.split("## 🚀 Quick Start", 1)[1].split("## ⚙️ Configuration", 1)[0].split())
    markers = ("Keep a working, independent administrator login", "Prepare the IdP application",
               "Configure access before saving", "Save, then register the actual callback",
               "Verify without replacing the working login", "Only after verifying SSO")
    positions = []
    for marker in markers:
        case.assertIn(marker, setup)
        positions.append(setup.index(marker))
    case.assertEqual(positions, sorted(positions))
    for marker in ("Show access restrictions & role mapping", "New UI providers are enabled on save",
                   "do not register a guessed ID", "not the callback, user claims",
                   "an intended non-admin cannot use administrator controls",
                   "does not create or restore local credentials"):
        case.assertIn(marker, setup)


def assert_group_claim_advice(case, guide):
    case.assertNotIn("Requires the `groups` scope/claim", guide)
    case.assertNotIn("Ensure your IdP includes the `groups` scope", guide)
    for marker in ("a `groups` scope is not universally supported", "not a scope name",
                   "Entra ID has no `groups` scope", "alone neither supplies group membership"):
        case.assertIn(marker, " ".join(guide.split()))


def assert_provider_edit_entry(case, guide):
    chooser = guide.split("## ⚡ Quick Fixes", 1)[0]
    entry = chooser.split("- **SSO stopped working after editing a provider:**", 1)
    case.assertEqual(len(entry), 2, "the symptom chooser must name provider-save failures")
    entry = " ".join(entry[1].split("\n- ", 1)[0].split())
    case.assertIn("OIDC.md#sign-in-fails-after-editing-a-provider", entry)
    for boundary in ("saved without changing anything", "Keep the working administrator session",
                     "a local password reset does not restore", "Test does not prove sign-in"):
        case.assertIn(boundary, entry)


def assert_unauthorised_sso_routes(case, guide):
    section = guide.split("#### Cannot login / 401 Unauthorized", 1)[1].split("\n#### ", 1)[0]
    entry = " ".join(section.split("- **SSO or proxy login:**", 1)[1].split("\n- ", 1)[0].split())
    for target in ("OIDC.md#sign-in-fails-after-editing-a-provider",
                   "OIDC.md#safe-diagnosis", "PROXY_AUTH.md"):
        case.assertIn(target, entry)
    case.assertIn("unchanged save", entry)
    case.assertIn("without closing the working administrator session", entry)
    case.assertIn("A local lockout reset does not repair", entry)


class OIDCSessionDocsTest(unittest.TestCase):
    def test_login_symptom_chooser_reaches_provider_save_recovery(self):
        assert_provider_edit_entry(self, (ROOT / "docs/TROUBLESHOOTING.md").read_text())

    def test_unauthorised_help_separates_sso_recovery_from_proxy_auth(self):
        assert_unauthorised_sso_routes(self, (ROOT / "docs/TROUBLESHOOTING.md").read_text())
        for heading in ("### Sign-in fails after editing a provider", "### Safe diagnosis"):
            self.assertIn(heading, GUIDE.read_text())

    def test_setup_orders_callback_and_access_before_cutover(self):
        assert_callback_setup(self, GUIDE.read_text())

    def test_generic_group_advice_does_not_break_entra(self):
        assert_group_claim_advice(self, GUIDE.read_text())

    def test_provider_examples_copy_the_saved_callback(self):
        text = guide_section("## 📚 Provider Examples", "## 🔧 Troubleshooting")
        self.assertNotIn("Create the provider in Pulse first", text)
        self.assertNotIn("<provider-id>", text)
        self.assertIn("Prepare the Entra app registration and client credentials first", text)
        self.assertIn("Complete the redirect registration before signing in", text)
        self.assertIn("secret value from step 2", text)
        self.assertIn("Do **not** add `groups`", text)
        self.assertIn("compatibility path", text)

    def test_setup_matches_existing_form_and_discovery_test(self):
        panel = (ROOT / "frontend-modern/src/components/Settings/SSOProvidersPanel.tsx").read_text()
        self.assertIn("URL is generated when the provider is saved", panel)
        self.assertIn("access restrictions & role mapping", panel)
        self.assertIn('label="Group Role Mappings"', panel)
        model = (ROOT / "frontend-modern/src/components/Settings/ssoProvidersModel.ts").read_text()
        defaults = model.split("export const createEmptyProviderForm", 1)[1].split("});", 1)[0]
        self.assertIn("id: ''", defaults)
        self.assertIn("enabled: true", defaults)
        self.assertIn("oidcScopes: 'openid profile email'", defaults)
        handlers = (ROOT / "internal/api/identity_sso_handlers.go").read_text()
        create = handlers.split("func (r *Router) handleCreateSSOProvider", 1)[1].split(
            "func (r *Router) handleUpdateSSOProvider", 1)[0]
        self.assertIn("provider.ID = uuid.NewString()", create)
        self.assertIn('Message: "OIDC discovery successful"', handlers)

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
