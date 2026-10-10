#!/usr/bin/env python3
"""Keep existing login help distinct by credential source and failure path.

These are documentation/source-consistency checks, not authentication or
installed recovery tests. No credentials, network or service changes are used.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


def section(heading: str) -> str:
    text = (ROOT / "docs/TROUBLESHOOTING.md").read_text(encoding="utf-8")
    return " ".join(text.split(heading + "\n", 1)[1].split("\n#### ", 1)[0].split())


class LoginTroubleshootingDocsTest(unittest.TestCase):
    def test_local_credentials_use_actual_precedence_and_file_format(self):
        text = section('#### "Invalid username or password" after setup')
        for expected in (
            "stop repeated password guesses", "username and client IP",
            "Deployment-supplied", "take precedence", "running password unchanged",
            "60 characters", "Compose YAML interpolation", "Docker `--env-file`",
            "different quoting rules", "do not blindly replace every `$` with `$$`",
            "CONFIGURATION.md#private-docker-authentication-file",
            "keeping hashes and resolved deployment configuration private",
            "#i-forgot-my-password", "not setup or re-enrolment",
        ):
            with self.subTest(statement=expected):
                self.assertIn(expected, text)
        self.assertNotIn("Use `$$2a$$...`", text)

    def test_browser_sso_and_api_failures_are_not_treated_as_one_password_problem(self):
        text = section("#### Cannot login / 401 Unauthorized")
        for expected in (
            "Browser session", "missing or expired session, not a wrong password",
            "same public Pulse URL", "fresh browser session",
            "Preserve any working administrator session", "SSO or proxy login",
            "PROXY_AUTH.md", "missing trusted proxy headers", "API client",
            "missing or invalid API token", "even when browser login works",
            "insufficient permissions or failed CSRF protection",
            "do not disable those checks or reset the password",
        ):
            with self.subTest(statement=expected):
                self.assertIn(expected, text)
        self.assertNotIn("- Clear browser cookies.", text)

    def test_lockout_rate_limit_and_identifier_reset_are_distinct(self):
        text = section("#### Cannot login / 401 Unauthorized")
        for expected in (
            "Account locked", "remaining wait", "separately to username and client IP",
            "clearing cookies does not reset it", "Too many requests", "429",
            "rate limiting, not proof that the password is wrong",
            "Wait rather than retrying rapidly, restarting Pulse or disabling authentication",
            "authenticated administrator", "`settings:write`", "CSRF protection",
            "resets one username or IP identifier at a time",
            "not a password, SSO account or every lockout",
            "Do not assume resetting the username also clears a separately locked IP",
            "../SECURITY.md#manual-recovery-admin",
        ):
            with self.subTest(statement=expected):
                self.assertIn(expected, text)

    def test_reporting_preserves_state_and_does_not_request_secrets(self):
        text = section("#### Cannot login / 401 Unauthorized")
        for expected in (
            "private header-file examples", "../SECURITY.md#usage",
            "never a token or session cookie pasted into a command or URL",
            "request path (without query strings)", "passwords, hashes, cookies, tokens",
            "full headers", "Copy as cURL", "network exports private",
            "does not justify deleting configuration, recreating the data volume",
            "re-enrolling agents",
        ):
            with self.subTest(statement=expected):
                self.assertIn(expected, text)
        self.assertNotIn("```", text)  # Links to the established private recipe only.

    def test_guidance_still_matches_current_source_boundaries(self):
        router = (ROOT / "internal/api/router.go").read_text(encoding="utf-8")
        login = router.split("func (r *Router) handleLogin(", 1)[1].split(
            "// handleResetLockout", 1
        )[0]
        for marker in (
            "GetLockoutInfo(loginReq.Username)", "GetLockoutInfo(clientIP)",
            '"account_locked"', "http.StatusForbidden", '"remainingMinutes"',
            '"rate_limit"', "http.StatusTooManyRequests", '"invalid_credentials"',
            "http.StatusUnauthorized",
        ):
            self.assertIn(marker, login)
        reset = router.split("func (r *Router) handleResetLockout(", 1)[1].split(
            "// handleState", 1
        )[0]
        for marker in ("RequireAdmin", "ensureSettingsWriteScope", "ResetLockout(resetReq.Identifier)"):
            self.assertIn(marker, reset)
        security = (ROOT / "internal/api/security.go").read_text(encoding="utf-8")
        self.assertIn("lockoutDuration   = 15 * time.Minute", security)
        auth = (ROOT / "internal/api/auth.go").read_text(encoding="utf-8")
        self.assertIn('"Invalid API token", http.StatusUnauthorized', auth)
        self.assertIn("ValidateAndExtendSession(cookie.Value)", auth)

    def test_quick_recovery_links_and_served_copy_do_not_invent_a_reset_ui(self):
        text = (ROOT / "docs/TROUBLESHOOTING.md").read_text(encoding="utf-8")
        quick = " ".join(text.split("### I forgot my password\n", 1)[1].split(
            "#### Recover the existing local administrator login", 1
        )[0].split())
        self.assertIn("../SECURITY.md#manual-recovery-admin", quick)
        self.assertNotIn("use the lockout reset in Pulse", quick)
        self.assertEqual(
            (ROOT / "docs/TROUBLESHOOTING.md").read_bytes(),
            (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes(),
        )


if __name__ == "__main__":
    unittest.main()
