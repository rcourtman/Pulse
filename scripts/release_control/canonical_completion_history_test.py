import json
import tempfile
import unittest
from pathlib import Path

from canonical_completion_history import git, load_completions, validate_completion


class CanonicalCompletionHistoryTest(unittest.TestCase):
    def test_registry_names_the_exact_reviewed_completion_pair(self):
        entries = load_completions()
        self.assertEqual(
            entries["a2dbb27a68463adb02b614e9927174543f5b7799"]["completion_commit"],
            "5f99f5c13fcc547e5e34286dfe873415a4876cfa",
        )

    def test_registry_rejects_unknown_fields(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "history.json"
            path.write_text(
                json.dumps({"version": 1, "completions": [], "unexpected": True}),
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "only version and completions"):
                load_completions(path)

    def test_report_intake_completion_satisfies_the_real_guard(self):
        incomplete = "0baee0bccffca078eb8598b2ed4ff481e4227b1e"
        self.assertEqual(
            load_completions()[incomplete]["completion_commit"],
            "89b8151f19bdcc1a0ae653b26baf75c0f12d735c",
        )
        self.assertTrue(validate_completion(incomplete, git("rev-parse", "HEAD")))

    def test_unregistered_commit_uses_the_normal_guard(self):
        self.assertFalse(validate_completion("0" * 40, "1" * 40))


if __name__ == "__main__":
    unittest.main()
