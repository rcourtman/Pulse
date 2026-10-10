#!/usr/bin/env python3
"""Check reporting guidance, not live revocation or GitHub data erasure."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### If you already posted private information\n"
ANCHOR = "docs/TROUBLESHOOTING.md#if-you-already-posted-private-information"


def containment(text):
    """Restrict checks to the instructions a reader follows after exposure."""
    return " ".join(text.split(HEADING, 1)[1].split("\n### ", 1)[0].split())


class PublicReportContainmentDocsTest(unittest.TestCase):
    def test_revocation_is_issuer_specific_and_preserves_least_privilege(self):
        text = containment((ROOT / "docs/TROUBLESHOOTING.md").read_text())
        for statement in (
            "publicly posted credential as exposed",
            "trusted administrator session", "affected credential with its issuer",
            "Settings → API Access", "respective providers",
            "stopping an agent is not credential revocation",
            "clearing local browser cookies is not server-side revocation",
            "issuer's confirmation", "interrupt monitoring or notifications",
            "independent coverage", "replacement carrying no wider permissions",
            "not a reason to rotate unrelated credentials",
        ):
            with self.subTest(statement=statement):
                self.assertIn(statement, text)

    def test_removal_never_promises_erasure_or_replaces_revocation(self):
        text = containment((ROOT / "docs/TROUBLESHOOTING.md").read_text())
        for statement in (
            "Editing the text alone does not remove an attachment",
            "GitHub for help with content you cannot remove yourself",
            "do not wait for removal before revoking access",
            "notification emails and attachment links may survive",
            "not proof that the information is erased",
        ):
            with self.subTest(statement=statement):
                self.assertIn(statement, text)

    def test_follow_up_keeps_originals_private_and_uses_existing_disclosure_route(self):
        text = containment((ROOT / "docs/TROUBLESHOOTING.md").read_text())
        for statement in (
            "Do not paste it again", "test whether it still works",
            "Preserve the original locally, not in another public upload",
            "relevant redacted symptoms", "[private disclosure route](../SECURITY.md)",
            "not another issue or discussion", "Do not send the credential itself",
            "unreviewed replacement bundle",
        ):
            with self.subTest(statement=statement):
                self.assertIn(statement, text)
        self.assertIn("**Private disclosures:** <security@pulserelay.pro>",
                      (ROOT / "SECURITY.md").read_text())
        self.assertIn(ANCHOR, (ROOT / "CONTRIBUTING.md").read_text())

    def test_every_report_form_warns_before_the_first_public_input(self):
        for name in ("bug_report.yml", "v6_rc_feedback.yml", "feature_request.yml"):
            with self.subTest(form=name):
                text = (ROOT / ".github/ISSUE_TEMPLATE" / name).read_text()
                intro = text.split("  - type: ", 2)[1]
                for statement in (
                    "If a credential has already been posted", "revoke it with its issuer",
                    "remove the public copy", "editing a post is not revocation",
                    "Do not repost the original evidence",
                    "https://github.com/rcourtman/Pulse/security/policy",
                ):
                    self.assertIn(statement, intro)

    def test_shipped_help_and_contribution_entrypoint_match_their_sources(self):
        for source in ("docs/TROUBLESHOOTING.md", "CONTRIBUTING.md"):
            with self.subTest(source=source):
                self.assertEqual((ROOT / source).read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / Path(source).name).read_bytes())


if __name__ == "__main__":
    unittest.main()
