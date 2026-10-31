#!/usr/bin/env python3
"""Check reporting guidance, not live revocation or GitHub data erasure."""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### If you already posted private information\n"
ANCHOR = "docs/TROUBLESHOOTING.md#if-you-already-posted-private-information"


def containment(text):
    """Restrict checks to the instructions a reader follows after exposure."""
    return " ".join(text.split(HEADING, 1)[1].split("\n### ", 1)[0].split())


def security_reporting(text):
    """Check the private route where a vulnerability reporter is directed."""
    heading = re.search(r"^## Reporting a vulnerability[ \t]*\n", text, re.MULTILINE)
    if heading is None:
        raise AssertionError("Security guide needs its private reporting section")
    remainder = text[heading.end():]
    end = re.search(r"^(?:## |---[ \t]*$)", remainder, re.MULTILINE)
    return " ".join(remainder[:end.start() if end else len(remainder)].split())


class PublicReportContainmentDocsTest(unittest.TestCase):
    def assert_private_disclosure_route(self, guide):
        section = security_reporting(guide)
        self.assertIn("<security@pulserelay.pro>", section)
        self.assertIn("Do not open a public issue or discussion for these reports.",
                      section)

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
        self.assert_private_disclosure_route((ROOT / "SECURITY.md").read_text())
        self.assertIn(ANCHOR, (ROOT / "CONTRIBUTING.md").read_text())

    def test_reporting_route_accepts_a_reworded_introduction(self):
        guide = (ROOT / "SECURITY.md").read_text()
        self.assert_private_disclosure_route(guide.replace("Email <", "Contact <", 1))

    def test_reporting_route_requires_its_section(self):
        guide = (ROOT / "SECURITY.md").read_text()
        with self.assertRaisesRegex(AssertionError, "private reporting section"):
            self.assert_private_disclosure_route(
                guide.replace("## Reporting a vulnerability", "## Other guidance", 1))

    def test_reporting_route_rejects_wrong_or_out_of_section_contact(self):
        address = "<security@pulserelay.pro>"
        guide = (ROOT / "SECURITY.md").read_text()
        for replacement in ("", "<other@example.com>"):
            for boundary in ("## Other guidance", "---"):
                with self.subTest(replacement=replacement, boundary=boundary):
                    wrong = guide.replace(address, replacement, 1)
                    wrong += f"\n{boundary}\n**Private disclosures:** {address}\n"
                    with self.assertRaises(AssertionError):
                        self.assert_private_disclosure_route(wrong)

    def test_reporting_route_keeps_the_public_posting_warning_with_the_contact(self):
        warning = "Do not open a public issue or discussion for these reports."
        guide = (ROOT / "SECURITY.md").read_text()
        misplaced = guide.replace(warning, "Use another route.", 1)
        misplaced += f"\n## Other guidance\n{warning}\n"
        with self.assertRaises(AssertionError):
            self.assert_private_disclosure_route(misplaced)

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
