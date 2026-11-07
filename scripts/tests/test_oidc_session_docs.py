#!/usr/bin/env python3
"""Keep OIDC help honest about existing session and revocation boundaries.

Documentation/source consistency only: this does not exercise an IdP, browser
session, token exchange or installed access removal.
"""

from pathlib import Path
import re
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


def assert_login_restriction_setup(case, guide, callback):
    section = guide.split("### Access Control\n", 1)[1].split(
        "### Group-to-Role Mapping\n", 1)[0]
    setup = " ".join(re.sub(r"(?m)^> ?", "", section).split())
    for boundary in (
        "every non-empty list must match",
        "Entries within one list are alternatives",
        "does not bypass the other lists",
        "An empty group list skips only that check",
        "any domain and email restrictions still apply",
        "does not grant instance administrator privileges by itself",
        "removing the local administrator",
    ):
        case.assertIn(boundary, setup)
    case.assertNotIn("allows every IdP user to sign in", setup)
    # Keep the explanation next to setup, bound to the actual independent
    # callback gates. This is source consistency, not an IdP login proof.
    gates = callback.split("// Apply access restrictions from SSO provider config", 1)[1].split(
        "// RBAC:", 1)[0]
    for condition, failure in (
        ("if len(provider.AllowedEmails) > 0 && !matchesValue(email, provider.AllowedEmails)", "email_restricted"),
        ("if len(provider.AllowedDomains) > 0 && !matchesDomain(email, provider.AllowedDomains)", "domain_restricted"),
        ("if len(provider.AllowedGroups) > 0", "group_restricted"),
    ):
        case.assertIn(condition, gates)
        branch = gates.split(condition, 1)[1].split("\n\t}", 1)[0]
        case.assertIn(f'"{failure}"', branch)
        case.assertIn("return", branch)
    case.assertIn("if !intersects(groups, provider.AllowedGroups)", gates)
    case.assertNotIn("else if", gates)


def assert_discovery_test_scope(case, handlers):
    oidc = handlers.split("func (r *Router) testOIDCConnection(", 1)[1].split(
        "// SSO Metadata Preview", 1)[0]
    # Match the discovery verdict, not a punctuation-sensitive success slogan.
    # These source checks complement the executable OIDC issuer-parity tests.
    for condition, error in (
        ('if discovery.Issuer == "" {', '"missing_discovery_issuer"'),
        ('if discovery.Issuer != cfg.IssuerURL {', '"issuer_mismatch"'),
    ):
        case.assertIn(condition, oidc)
        branch = oidc.split(condition, 1)[1].split("\n\t}", 1)[0]
        case.assertIn("return SSOTestResponse{", branch)
        case.assertIn("Success: false", branch)
        case.assertIn(error, branch)
        case.assertLess(oidc.index(condition), oidc.index("details := &SSOTestDetails{"))
    success = oidc.rsplit("return SSOTestResponse{", 1)[1]
    case.assertIn("Success: true", success)
    case.assertIn("exact issuer match", success)
    case.assertIn("Client credentials and sign-in have not been tested.", success)


def assert_role_assignment_audit_scope(case, guide, callback):
    mapping = " ".join(guide.split("### Group-to-Role Mapping", 1)[1].split(
        "### Sessions and `offline_access`", 1)[0].split())
    case.assertIn("Successful non-empty assignments get an audit event.", mapping)
    case.assertIn("Clearing roles does not.", mapping)
    case.assertNotIn("Role changes are logged to the audit log for compliance tracking", mapping)
    # Source consistency only. A role clear is still applied by the callback,
    # but its successful empty result does not enter this audit branch.
    roles = callback.split("if authManager := internalauth.GetManager(); authManager != nil {", 1)[1].split(
        "// Stamp issuer/client", 1)[0]
    case.assertIn("applySSORoleAssignments(", roles)
    case.assertRegex(roles, re.compile(
        r'} else if len\(rolesToAssign\) > 0 \{\s*'
        r'LogAuditEventForTenant\(GetOrgID\(req.Context\(\)\), "sso_oidc_role_assignment"'
    ))


def assert_refresh_source(case, auth, store):
    # Follow the captured store through scheduling and conditional completion.
    # The old free-function/global-write spellings cannot establish these
    # claims after refresh ownership moved into SessionStore.
    case.assertIn("store := GetSessionStore()", auth)
    case.assertIn("session := store.GetSession(cookie.Value)", auth)
    case.assertIn("go store.refreshOIDCSessionTokens(cfg, cookie.Value, session)", auth)
    marker = "func (store *SessionStore) refreshOIDCSessionTokens("
    case.assertIn(marker, auth)
    refresh = auth.split(marker, 1)[1]
    for expected in (
        "attempt := store.beginOIDCRefresh(sessionToken, session)",
        "defer store.endOIDCRefresh(sessionToken, attempt)",
        "if oidcCfg == nil", "if err != nil",
        "if !store.completeOIDCRefresh(sessionToken, attempt, nil)",
        "if !store.completeOIDCRefresh(sessionToken, attempt, result)",
    ):
        case.assertIn(expected, refresh)
    for forbidden in (
        "GetSessionStore().InvalidateSession", "GetSessionStore().UpdateOIDCTokens",
        "resolveGroupRoles", "AllowedGroups",
    ):
        case.assertNotIn(forbidden, refresh)

    marker = "func (s *SessionStore) completeOIDCRefresh("
    case.assertIn(marker, store)
    completion = store.split(marker, 1)[1].split("// UpdateOIDCTokens", 1)[0]
    for expected in (
        "s.oidcRefreshes[key] != attempt", "session != attempt.session",
        "!sameOIDCRefreshSource(session, &attempt.snapshot)",
        "!now.Before(session.ExpiresAt)",
        "if result == nil {", "delete(s.sessions, key)",
        "session.OIDCRefreshToken = result.RefreshToken",
        "session.OIDCAccessTokenExp = result.Expiry",
        "session.ExpiresAt = now.Add(session.OriginalDuration)",
        "s.mu.Unlock()\n\ts.save()", "return true",
    ):
        case.assertIn(expected, completion)
    invalidation, success = completion.split("if result == nil {", 1)[1].split("} else {", 1)
    case.assertIn("delete(s.sessions, key)", invalidation)
    case.assertIn("session.OIDCRefreshToken = result.RefreshToken", success)
    case.assertNotIn("defer s.mu.Unlock()", completion)
    case.assertLess(completion.index("s.save()"), completion.index("return true"))


class OIDCSessionDocsTest(unittest.TestCase):
    def test_login_allowlists_are_not_described_as_bypasses(self):
        callback = (ROOT / "internal/api/oidc_handlers.go").read_text()
        assert_login_restriction_setup(self, GUIDE.read_text(), callback)

    def test_login_restriction_consistency_rejects_misleading_help_or_gate_drift(self):
        guide = GUIDE.read_text()
        callback = (ROOT / "internal/api/oidc_handlers.go").read_text()
        mutations = (
            ("guide", "every non-empty list must match", "any non-empty list can match"),
            ("guide", "An empty group list skips\n> only that check", "An empty group list allows every IdP user to sign in"),
            ("callback", "if len(provider.AllowedDomains) > 0 && !matchesDomain(email, provider.AllowedDomains)",
             "else if len(provider.AllowedDomains) > 0 && !matchesDomain(email, provider.AllowedDomains)"),
            ("callback", '"email_restricted"', '"another_error"'),
            ("callback", "if !intersects(groups, provider.AllowedGroups)", "if false"),
        )
        for target, before, after in mutations:
            with self.subTest(regression=target + ":" + before):
                original = guide if target == "guide" else callback
                self.assertIn(before, original)
                changed = original.replace(before, after, 1)
                with self.assertRaises(AssertionError):
                    assert_login_restriction_setup(
                        self, changed if target == "guide" else guide,
                        changed if target == "callback" else callback,
                    )

    def test_unconfirmed_changes_use_a_read_without_promising_recovery(self):
        guide = GUIDE.read_text()
        recovery = " ".join(guide.split("### A provider change could not be confirmed", 1)[1].split())
        for expected in (
            "Do not repeat the change", "independent administrator session open",
            "Check saved providers", "discards the open draft, including any typed secret",
            "without repeating the change or running a connection test",
            "versions without that button", "Do not bypass the warning through the API",
            "Callback / Redirect URL", "not a duplicate provider",
            "Open **Edit** without saving", "snapshot at that time",
            "earlier request cannot still finish", "unknown, not empty or unchanged",
            "unexpectedly missing, stop without adding or deleting anything",
            "configuration-load failure can look like an empty list",
            "does not confirm which client secret is stored",
            "does not prove working sign-in, intended permissions or revoked sessions",
            "[separate-session checks in Quick Start](#-quick-start)",
            "Keep secrets, cookies and full responses private",
        ):
            self.assertIn(expected, recovery)
        self.assertNotIn("```", recovery)
        # Keep existing troubleshooting visible in its established order.
        self.assertLess(guide.index("### Safe diagnosis"), guide.index("### A provider change could not be confirmed"))

    def test_provider_check_instructions_match_the_read_only_control(self):
        state = (ROOT / "frontend-modern/src/components/Settings/useSSOProvidersState.ts").read_text()
        panel = (ROOT / "frontend-modern/src/components/Settings/SSOProvidersPanel.tsx").read_text()
        self.assertIn("'Check saved providers'", panel)
        self.assertIn("onClick={() => void loadProviders()}", panel)
        self.assertIn("retry: false", state)
        inventory = state.split("const loadProviders = async () => {", 1)[1].split("const resetContext", 1)[0]
        self.assertIn("if (unconfirmedChange()) {\n      clearEditor();", inventory)
        self.assertIn("apiFetch('/api/security/sso/providers', requestOptions(controller))", inventory)
        self.assertIn("!Array.isArray(data.providers)", inventory)
        self.assertLess(inventory.index("!Array.isArray(data.providers)"), inventory.index("setUnconfirmedChange(null)"))
        for mutation in ("method:", "handleSave(", "handleDelete(", "testProvider("):
            self.assertNotIn(mutation, inventory)
        detail = state.split("const openEditModal = async", 1)[1].split("const beginMutation", 1)[0]
        self.assertIn("requestOptions(controller)", detail)
        self.assertNotIn("method:", detail)

    def test_role_assignment_audit_does_not_promise_events_for_cleared_roles(self):
        callback = (ROOT / "internal/api/oidc_handlers.go").read_text()
        assert_role_assignment_audit_scope(self, GUIDE.read_text(), callback)

    def test_role_assignment_audit_consistency_rejects_overclaims_and_source_drift(self):
        guide = GUIDE.read_text()
        callback = (ROOT / "internal/api/oidc_handlers.go").read_text()
        mutations = (
            ("guide", "Successful non-empty assignments get an audit event. Clearing roles does not.",
             "Role changes are logged to the audit log for compliance tracking."),
            ("callback", "} else if len(rolesToAssign) > 0 {", "} else if len(rolesToAssign) >= 0 {"),
            ("callback", '"sso_oidc_role_assignment"', '"another_event"'),
        )
        for target, before, after in mutations:
            with self.subTest(regression=target + ":" + before):
                original = guide if target == "guide" else callback
                self.assertIn(before, original)
                changed = original.replace(before, after, 1)
                with self.assertRaises(AssertionError):
                    assert_role_assignment_audit_scope(
                        self, changed if target == "guide" else guide,
                        changed if target == "callback" else callback,
                    )

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
        assert_discovery_test_scope(self, handlers)

    def test_discovery_consistency_rejects_false_green_regressions(self):
        handlers = (ROOT / "internal/api/identity_sso_handlers.go").read_text()
        mutations = (
            ('if discovery.Issuer == "" {', 'if false {'),
            ('if discovery.Issuer != cfg.IssuerURL {', 'if false {'),
            ('"missing_discovery_issuer"', '"ignored_missing_issuer"'),
            ('"issuer_mismatch"', '"ignored_issuer_mismatch"'),
            ("Client credentials and sign-in have not been tested.", "Sign-in is ready."),
        )
        for before, after in mutations:
            with self.subTest(regression=before):
                self.assertIn(before, handlers)
                with self.assertRaises(AssertionError):
                    assert_discovery_test_scope(self, handlers.replace(before, after, 1))

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
        assert_refresh_source(self, auth, store)

    def test_refresh_consistency_rejects_ownership_and_persistence_regressions(self):
        auth = (ROOT / "internal/api/auth.go").read_text()
        store = (ROOT / "internal/api/session_store.go").read_text()
        mutations = (
            ("auth", "go store.refreshOIDCSessionTokens(cfg, cookie.Value, session)",
             "go GetSessionStore().refreshOIDCSessionTokens(cfg, cookie.Value, session)"),
            ("auth", "if !store.completeOIDCRefresh(sessionToken, attempt, nil)", "if false"),
            ("auth", "if !store.completeOIDCRefresh(sessionToken, attempt, result)", "if false"),
            ("store", "session != attempt.session", "false"),
            ("store", "if result == nil {\n\t\tdelete(s.sessions, key)",
             "if result == nil {\n\t\t// Invalidation accidentally omitted"),
            ("store", "session.OIDCRefreshToken = result.RefreshToken",
             "// Rotated refresh token accidentally omitted"),
            ("store", "s.mu.Unlock()\n\ts.save()\n\treturn true\n}", "s.mu.Unlock()\n\treturn true\n}"),
            ("store", "s.mu.Unlock()\n\ts.save()\n\treturn true\n}", "s.save()\n\ts.mu.Unlock()\n\treturn true\n}"),
        )
        for source, before, after in mutations:
            with self.subTest(source=source, regression=before):
                original = auth if source == "auth" else store
                self.assertIn(before, original)
                changed = original.replace(before, after, 1)
                with self.assertRaises(AssertionError):
                    assert_refresh_source(self, changed if source == "auth" else auth,
                                          changed if source == "store" else store)

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
