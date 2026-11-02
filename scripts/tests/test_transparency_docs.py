#!/usr/bin/env python3
"""Keep the public triage disclosure honest about delivery and private support.

These checks cover documentation claims, not operation of the release train.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOCUMENTS = (
    ROOT / "docs/AI_TRANSPARENCY.md",
    ROOT / "frontend-modern/public/docs/AI_TRANSPARENCY.md",
)


class TransparencyDocsTest(unittest.TestCase):
    def documents(self):
        for path in DOCUMENTS:
            yield path, " ".join(path.read_text(encoding="utf-8").split())

    def test_disclosure_summarises_the_train_and_links_the_full_rules(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("Releases follow a fixed weekly train", text)
                self.assertIn("soaks for 24 hours and is promoted unchanged to stable", text)
                self.assertIn("unless a `release-blocker` issue stops it", text)
                self.assertIn("Stable patches are not yet available", text)
                self.assertIn("[limited stable patches](RELEASE_PROCESS.md#limited-stable-patches)", text)
                self.assertIn("[Releases and update channels](RELEASE_PROCESS.md)", text)
                self.assertNotIn("Patch releases are not scheduled", text)
                self.assertNotIn("takes only backports", text)
                self.assertNotIn("Every 14 days", text)
                self.assertNotIn("An early cut can bring", text)

    def test_linked_release_guide_does_not_offer_an_early_cut_or_soak_replacement(self):
        for directory in ("docs", "frontend-modern/public/docs"):
            path = ROOT / directory / "RELEASE_PROCESS.md"
            text = " ".join(path.read_text(encoding="utf-8").split()).replace("**", "")
            with self.subTest(document=path):
                self.assertIn("seven days after the last minor stable release", text)
                self.assertIn("one day earlier", text)
                self.assertIn("Nothing brings that cut forward", text)
                self.assertIn("an important fix ships in the next daily beta", text)
                self.assertIn("Only a `release-blocker` requires a fresh candidate", text)
                self.assertIn("does not replace a soaking candidate or bring stable publication forward", text)
                self.assertIn("same required checks and full 24-hour soak", text)
                self.assertNotIn("can bring", text)

    def test_patch_policy_is_not_claimed_as_available_delivery(self):
        for directory in ("docs", "frontend-modern/public/docs"):
            path = ROOT / directory / "RELEASE_PROCESS.md"
            text = " ".join(path.read_text(encoding="utf-8").split()).replace("**", "")
            with self.subTest(document=path):
                self.assertIn("As of 27 October 2026, stable patches are not yet available", text)
                self.assertIn("stable-regression", text)
                self.assertIn("eligible repairs already merged to `main`", text)
                self.assertIn("release metadata and publication support", text)
                self.assertIn("containing minor release", text)
                self.assertNotIn("Patch releases are not scheduled", text)

    def test_patch_guide_distinguishes_eligibility_cadence_and_safe_reporting(self):
        for directory in ("docs", "frontend-modern/public/docs"):
            path = ROOT / directory / "RELEASE_PROCESS.md"
            text = " ".join(path.read_text(encoding="utf-8").split()).replace("**", "")
            with self.subTest(document=path):
                for claim in (
                    "current stable broke behaviour working in the previous stable",
                    "security vulnerability or loses data",
                    "not a fast track for other bugs or feature requests",
                    "patch delivery is not guaranteed for every labelled issue",
                    "promoted unchanged after its required soak",
                    "never delays or replaces the weekly minor candidate",
                    "does not move the next minor's date",
                    "Only the current stable can be patched",
                    "You do not need to install a preview to report a stable regression",
                    "Do not downgrade, repeat an unsafe operation or change update channels",
                ):
                    self.assertIn(claim, text)


    def test_merged_fix_is_not_a_published_fix(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("A fix merged to `main` is not yet a published fix", text)
                self.assertIn("only after checking its released source", text)

    def test_private_support_is_not_automated_or_public_diagnostics(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("I handle private support email myself", text)
                self.assertIn("does not read a support mailbox or send email", text)
                self.assertIn("Keep credentials and private diagnostics out of public threads", text)
                self.assertNotIn("Automated support replies are sent", text)
                self.assertIn("[triage contract](ISSUE_TRIAGE.md)", text)

    def test_resolution_needs_every_reported_symptom_not_a_component_pass(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("reporter confirms the fix", text)
                self.assertIn("no reported symptom left unresolved", text)
                self.assertIn("A merged fix or a workaround alone is not enough", text)

    def test_timeout_starts_with_containing_stable_release_contact(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("Once the fix ships in a stable release", text)
                self.assertIn("After 14 days with no reply", text)
                self.assertIn("nobody else reporting the same problem", text)

    def test_timeout_is_not_confirmed_resolution_and_has_explicit_exclusions(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("`closed-needs-retest-timeout` label", text)
                self.assertIn("**not confirmed resolution**", text)
                self.assertIn("it can be reopened", text)
                self.assertIn("Feature requests, `needs-human` issues and unexplained symptoms are never closed this way", text)

    def test_contesting_a_close_preserves_the_report_and_safe_existing_evidence(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("original issue rather than filing it again", text)
                self.assertIn("which symptom remains", text)
                self.assertIn("Don't repeat an outage, update or backup just to retest", text)
                self.assertIn("TROUBLESHOOTING.md#collect-diagnostics-safely", text)

    def test_standing_authority_preserves_review_and_mechanical_releases(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("Maintainer changes receive independent review", text)
                self.assertIn("must pass the repository's required checks before merging", text)
                self.assertIn("without per-release approval", text)
                self.assertIn("no model chooses what goes in a release or when", text)
                self.assertNotIn("maintainer chooses release scope, maturity and timing", text)
                self.assertNotIn("b64709e7b7ad174e9c94ad2a0d3d841678690935", text)

    def test_shipped_disclosure_matches_the_triage_link_target(self):
        self.assertEqual(DOCUMENTS[0].read_bytes(), DOCUMENTS[1].read_bytes())


if __name__ == "__main__":
    unittest.main()
