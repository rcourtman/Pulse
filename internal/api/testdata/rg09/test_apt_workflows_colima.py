import unittest
from pathlib import Path

import apt_workflows_colima as lab


class APTWorkflowsColimaTest(unittest.TestCase):
    def test_images_and_gate_are_closed(self) -> None:
        self.assertEqual(lab.IMAGES, ("debian:bookworm-slim", "ubuntu:24.04"))
        self.assertEqual(lab.GATE_LABEL, "com.pulse.intelligence-lab.gate=rg-09")

    def test_redaction_helpers_are_bound_to_canonical_repo_path(self) -> None:
        repo_root = Path(__file__).resolve().parents[4]
        self.assertEqual(
            lab.INTELLIGENCE_LAB_DIR,
            repo_root / "scripts" / "intelligence_lab",
        )
        self.assertTrue((lab.INTELLIGENCE_LAB_DIR / "artifact_redaction.py").is_file())

    def test_run_and_sha_validation_are_bounded(self) -> None:
        self.assertIsNotNone(lab.RUN_ID.fullmatch("rg09-release-001"))
        self.assertIsNone(lab.RUN_ID.fullmatch("RG09/unsafe"))
        self.assertIsNotNone(lab.SHA.fullmatch("a" * 40))
        self.assertIsNone(lab.SHA.fullmatch("a" * 39))

    def test_isolated_go_cache_uses_development_ssd(self) -> None:
        self.assertEqual(
            lab.isolated_go_cache("rg09-release-001"),
            Path("/Volumes/Development/.go-task-caches/rg09-rg09-release-001"),
        )


if __name__ == "__main__":
    unittest.main()
