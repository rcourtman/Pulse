"""Release trains must receive the same CI triggers as main."""
import fnmatch
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]


class ReleaseTrainCIContractTest(unittest.TestCase):
    def test_build_and_e2e_cover_release_train_pushes_and_proposals(self):
        for workflow in ("build-and-test.yml", "test-e2e.yml"):
            source = (ROOT / ".github/workflows" / workflow).read_text()
            for event in ("push", "pull_request"):
                with self.subTest(workflow=workflow, event=event):
                    block = re.search(
                        rf"^  {event}:\n(.*?)(?=^  \w|^\S|\Z)",
                        source, re.MULTILINE | re.DOTALL,
                    )
                    self.assertIsNotNone(block)
                    branches = re.search(
                        r"^    branches:\n((?:      - .*\n)+)",
                        block.group(1), re.MULTILINE,
                    )
                    self.assertIsNotNone(branches)
                    patterns = [line.strip()[2:].strip('\"\'')
                                for line in branches.group(1).splitlines()]
                    for branch in ("main", "release/v6.4", "release/v7.0"):
                        self.assertTrue(
                            any(fnmatch.fnmatchcase(branch, p) for p in patterns),
                            f"{workflow} {event} excludes {branch}: {patterns}",
                        )


if __name__ == "__main__":
    unittest.main()
