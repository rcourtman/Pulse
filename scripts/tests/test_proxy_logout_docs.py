#!/usr/bin/env python3
"""Keep proxy sign-out help consistent with the two shipped UI branches.

Documentation/source consistency, not execution of an IdP, logout or access
revocation. The browser fixture exercises the actual Docs consumer separately.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDE = ROOT / "docs/PROXY_AUTH.md"


def signout_section(text):
    return " ".join(text.split("### Sign-out and access removal", 1)[1]
                    .split("### Verify Headers", 1)[0].split())


class ProxyLogoutDocsTest(unittest.TestCase):
    def test_configuration_and_symptom_link_to_the_boundary(self):
        text = GUIDE.read_text()
        row = next(line for line in text.splitlines() if line.startswith("| `PROXY_AUTH_LOGOUT_URL`"))
        self.assertIn("not an access-revocation operation", row)
        self.assertIn("(#sign-out-and-access-removal)", row)
        symptom = next(line for line in text.splitlines() if line.startswith("| **Logout returns"))
        self.assertIn("(#sign-out-and-access-removal)", symptom)
        self.assertNotIn("URL to redirect to after logout", text)

    def test_help_distinguishes_redirect_from_pulse_and_provider_logout(self):
        text = signout_section(GUIDE.read_text())
        for phrase in ("instead of calling Pulse's `/api/logout`", "does not first clear its own session",
                       "Without that URL", "clears local UI auth state", "does **not** sign out",
                       "valid identity headers", "without showing Pulse's login form",
                       "not a Pulse home page", "installed configuration"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

    def test_two_logout_branches_still_match_the_shipped_source(self):
        source = (ROOT / "frontend-modern/src/useAppRuntimeState.ts").read_text()
        handler = source.split("const handleLogout = async () => {", 1)[1].split("const enhancedStore", 1)[0]
        redirect = re.search(r"if \(proxyAuth\?\.logoutURL\) \{([^}]+)\}", handler)
        self.assertIsNotNone(redirect)
        self.assertIn("window.location.href = proxyAuth.logoutURL;", redirect[1])
        self.assertIn("return;", redirect[1])
        self.assertNotIn("runtimeApiFetch", redirect[1])
        self.assertLess(redirect.end(), handler.index("runtimeApiFetch('/api/logout'"))
        self.assertIn("method: 'POST'", handler)
        self.assertIn("clearAuth();", handler)
        self.assertIn("sessionStorage.clear();", handler)
        server = (ROOT / "internal/api/router.go").read_text().split("func (r *Router) handleLogout", 1)[1]
        server = server.split("func (r *Router) establishSession", 1)[0]
        self.assertIn("req.Method != http.MethodPost", server)
        self.assertIn("r.clearSession(w, req)", server)
        proxy = (ROOT / "internal/api/auth.go").read_text().split("func CheckProxyAuth", 1)[1]
        proxy = proxy.split("\nfunc ", 1)[0]
        self.assertIn('r.Header.Get("X-Proxy-Secret")', proxy)
        self.assertIn("r.Header.Get(cfg.ProxyAuthUserHeader)", proxy)
        self.assertNotIn("GetSessionStore", proxy)

    def test_verification_uses_the_original_session_and_preserves_recovery(self):
        text = signout_section(GUIDE.read_text())
        for phrase in ("independent administrator recovery session", "same browser session",
                       "without signing in again", "new private window tests a different session",
                       "do not hand that browser or device", "device lock",
                       "do not repeatedly log out", "sign out all sessions",
                       "API tokens and agent credentials"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

    def test_revocation_and_help_keep_the_existing_access_boundaries(self):
        text = signout_section(GUIDE.read_text())
        for phrase in ("existing proxy/IdP assignment and session revocation controls",
                       "preserving unrelated users", "blocking only future IdP logins",
                       "existing proxy sessions", "(#-header-trust-boundary)",
                       "Do not disable Pulse authentication", "delete configuration",
                       "rotate the shared proxy secret", "out of commands and reports",
                       "redacted observation"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

    def test_shipped_help_matches_the_canonical_guide(self):
        self.assertEqual(GUIDE.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/PROXY_AUTH.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
