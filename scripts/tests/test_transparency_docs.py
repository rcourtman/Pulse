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
                self.assertIn("Every 14 days after the last stable release", text)
                self.assertIn("next minor release candidate from the head of `main`", text)
                self.assertIn("everything on `main` at that cut", text)
                self.assertIn("Repairs go on `main`, not onto an older release line", text)
                self.assertIn("An early cut can bring the next minor release forward", text)
                self.assertIn("Patch releases are not scheduled", text)
                self.assertNotIn("takes only backports", text)

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
                self.assertIn("not the fix commit, a beta or a release candidate", text)
                self.assertIn("neither the reporter nor another person reports the same problem", text)

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
