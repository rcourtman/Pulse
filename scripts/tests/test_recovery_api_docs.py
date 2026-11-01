#!/usr/bin/env python3
"""Keep the API reference honest about the existing local recovery boundary.

These are documentation/source consistency checks, not execution of recovery,
credential handling, an IdP or a deployed administrator journey.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


def section(source, heading):
    matches = re.findall(rf"^{re.escape(heading)}\n(.*?)(?=^### |\Z)", source,
                         re.MULTILINE | re.DOTALL)
    if len(matches) != 1:
        raise AssertionError(f"Expected one {heading} section, found {len(matches)}")
    return matches[0]


class RecoveryAPIReferenceTest(unittest.TestCase):
    def setUp(self):
        self.guide = section((ROOT / "docs/API.md").read_text(),
                             "### Recovery sessions (direct loopback)")
        routes = (ROOT / "internal/api/router_routes_auth_security.go").read_text()
        self.handler = routes.split('r.mux.HandleFunc("/api/security/recovery",', 1)[1].split(
            "// Agent WebSocket", 1)[0]

    def test_reference_describes_each_existing_action_without_an_executable_bypass_recipe(self):
        rows = re.findall(r"^\| `(\w+)` \| (.+) \|$", self.guide, re.MULTILINE)
        self.assertEqual([action for action, _ in rows],
                         re.findall(r'case "(\w+)":', self.handler))
        self.assertEqual(len(rows), 3)
        effects = dict(rows)
        self.assertIn("does not create a browser session", effects["generate_token"])
        self.assertIn("does not remove configured authentication", effects["disable_auth"])
        self.assertIn("matching direct loopback requests", effects["disable_auth"])
        self.assertIn("does not reset a password", effects["enable_auth"])
        self.assertNotIn("```", self.guide, "Do not add a cookie/token-bearing copy-paste detour")
        for target in ("i-forgot-my-password", "recovery-mode"):
            self.assertIn(f"TROUBLESHOOTING.md#{target}", self.guide)

    def test_token_lifetime_and_consumption_match_admission_before_method_dispatch(self):
        token_check = self.handler.index("ValidateRecoveryTokenConstantTime(recoveryToken, clientIP)")
        self.assertLess(token_check, self.handler.index("if req.Method == http.MethodPost"))
        self.assertLess(token_check, self.handler.index("decodeSecurityRequestBody"))
        self.assertLess(token_check, self.handler.index("req.Method == http.MethodGet"))
        generate = self.handler.split('case "generate_token":', 1)[1].split('case "disable_auth":', 1)[0]
        for marker in ("if !isLoopback", "duration := 15", "recoveryRequest.Duration > 0",
                       "recoveryRequest.Duration <= 60"):
            self.assertIn(marker, generate)
        store = (ROOT / "internal/api/recovery_tokens.go").read_text().split(
            "func (r *RecoveryTokenStore) ValidateRecoveryTokenConstantTime(", 1)[1].split(
            "\nfunc ", 1)[0]
        self.assertIn("token.Used = true", store)
        self.assertIn("recoveryTokenBoundToIP(token, ip)", store)
        text = " ".join(self.guide.split())
        for marker in ("single-use", "bound to the IP", "15 minutes", "1 to 60",
                       "consumed at admission", "including by GET", "later fails validation",
                       "not a global authentication state", "uncertain POST"):
            self.assertIn(marker, text)

    def test_session_is_local_and_the_reference_does_not_offer_cookie_transfer(self):
        router = (ROOT / "internal/api/router.go").read_text()
        loopback = router.split("func isDirectLoopbackRequest(", 1)[1].split("\nfunc ", 1)[0]
        for marker in ("ip.IsLoopback()", '"X-Forwarded-For"', '"Forwarded"', '"X-Real-IP"'):
            self.assertIn(marker, loopback)
        auth = (ROOT / "internal/api/auth.go").read_text().split(
            "func requestMatchesRecoverySession(", 1)[1].split("\nfunc ", 1)[0]
        for marker in ("session.RecoveryBypass", "isDirectLoopbackRequest(r)",
                       "constantTimeStringEqual(expectedIP, actualIP)"):
            self.assertIn(marker, auth)
        self.assertIn("r.establishRecoverySession(w, req, recoveryUser)", self.handler)
        self.assertIn("r.clearSession(w, req)", self.handler)
        text = " ".join(self.guide.split())
        for marker in ("not password reset", "forwarded requests do not count",
                       "does **not** make", "usable remotely or through a reverse proxy",
                       "does not unlock a separate browser", "Do not transfer session cookies",
                       "command arguments, URLs or reports", "working administrator session"):
            self.assertIn(marker, text)

    def test_shipped_reference_matches_the_canonical_document(self):
        self.assertEqual((ROOT / "docs/API.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/API.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
