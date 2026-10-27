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

    def test_all_main_minor_cycle_not_selective_backports(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("Minor stable publication is targeted seven days after the last minor stable release", text)
                self.assertIn("next minor release candidate becomes eligible one day earlier", text)
                self.assertIn("reserving its full 24-hour soak", text)
                self.assertIn("preparation and qualification can move publication later", text)
                self.assertIn("cut from the head of `main`", text)
                self.assertIn("everything on `main` at that cut", text)
                self.assertIn("Repairs go on `main`, not onto an older release line", text)
                self.assertIn("An important fix ships in the next beta, not through an early candidate or stable release", text)
                self.assertIn("no beta is cut within a day of the candidate being due", text)
                self.assertIn("A stable patch does not restart the weekly minor clock", text)
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
            for name in ("AI_TRANSPARENCY", "RELEASE_PROCESS"):
                path = ROOT / directory / f"{name}.md"
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

    def test_blockers_need_a_fresh_candidate_not_a_changed_soak(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("soaks for 24 hours", text)
                self.assertIn("promoted unchanged to stable", text)
                self.assertIn("open issue labelled `release-blocker`", text)
                self.assertIn("candidate regression from the previous stable release", text)
                self.assertIn("train cuts a fresh candidate that carries the repair", text)
                self.assertIn("not inserted into the frozen candidate", text)

    def test_source_preview_and_stable_availability_are_separate(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("A fix merged to `main` is not yet a published fix", text)
                self.assertIn("not proof that stable users have it", text)
                self.assertIn("checking its actual released source", text)
                self.assertIn("https://github.com/rcourtman/Pulse/releases", text)
                self.assertIn("[Releases and update channels](RELEASE_PROCESS.md)", text)
                self.assertIn("passing test alone does not establish availability", text)

    def test_private_support_is_not_automated_or_public_diagnostics(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("Private support email is handled by Richard", text)
                self.assertIn("does not read a support mailbox or send email", text)
                self.assertIn("Keep credentials and private diagnostics out of public threads", text)
                self.assertNotIn("Automated support replies are sent", text)
                self.assertIn("[topic-integrity triage contract](ISSUE_TRIAGE.md)", text)

    def test_resolution_needs_every_reported_symptom_not_a_component_pass(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("reporter confirms recovery", text)
                self.assertIn("no reported symptom left unresolved", text)
                self.assertIn("one recovered screen or a merged fix", text)
                self.assertIn("duplicate, need their own explanation", text)
                self.assertIn("demand-ledger entry is not an implemented feature", text)

    def test_timeout_starts_with_containing_stable_release_contact(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("fix for every reported symptom ships in a stable release", text)
                self.assertIn("closed after 14 days without a reply and can be reopened", text)
                self.assertIn("start with that stable-release retest reply", text)
                self.assertIn("earlier retest request named a beta or release candidate carrying the same fix", text)
                self.assertIn("limit on repeated unanswered contact prevents another reply", text)
                self.assertIn("later of that request and the stable release's publication", text)
                self.assertIn("A beta or release candidate alone never starts the timeout, nor does a fix commit", text)
                self.assertIn("at least 14 days without any reply from the reporter", text)
                self.assertIn("no other person reporting the same problem since that retest request", text)

    def test_timeout_is_not_confirmed_resolution_and_has_explicit_exclusions(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("`closed-needs-retest-timeout` label", text)
                self.assertIn("**not confirmed resolution**", text)
                self.assertIn("does not apply to feature requests", text)
                self.assertIn("issues labelled `needs-human`", text)
                self.assertIn("an unfixed or unexplained symptom", text)

    def test_contesting_a_close_preserves_the_report_and_safe_existing_evidence(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("original issue rather than filing it again", text)
                self.assertIn("which symptom remains", text)
                self.assertIn("version already observed", text)
                self.assertIn("unknown details are valid and credentials stay private", text)
                self.assertIn("do not repeat an outage, update or backup", text)
                self.assertIn("grounds to reopen the issue", text)
                self.assertIn("TROUBLESHOOTING.md#collect-diagnostics-safely", text)

    def test_standing_authority_preserves_review_and_mechanical_releases(self):
        for path, text in self.documents():
            with self.subTest(document=path):
                self.assertIn("Maintainer changes receive independent review", text)
                self.assertIn("must pass the repository's required checks before merging", text)
                self.assertIn("without per-release approval", text)
                self.assertIn("rather than having a model choose release scope", text)
                self.assertNotIn("maintainer chooses release scope, maturity and timing", text)
                self.assertNotIn("b64709e7b7ad174e9c94ad2a0d3d841678690935", text)

    def test_shipped_disclosure_matches_the_triage_link_target(self):
        self.assertEqual(DOCUMENTS[0].read_bytes(), DOCUMENTS[1].read_bytes())


if __name__ == "__main__":
    unittest.main()
