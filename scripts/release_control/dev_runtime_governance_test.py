import unittest

from canonical_completion_guard import infer_impacted_subsystems, is_test_or_fixture
from subsystem_lookup import lookup_paths


class DevRuntimeGovernanceTest(unittest.TestCase):
    def test_shell_smoke_tests_are_classified_as_tests(self) -> None:
        self.assertTrue(is_test_or_fixture("scripts/tests/test-hot-dev-bg.sh"))
        self.assertEqual(infer_impacted_subsystems(["scripts/tests/test-hot-dev-bg.sh"]), {})

    def test_lookup_leaves_unmatched_tests_without_matches(self) -> None:
        result = lookup_paths(["tests/example.spec.ts"])
        self.assertEqual(result["files"][0]["classification"], "test-or-fixture")
        self.assertEqual(result["files"][0]["matches"], [])

    def test_lookup_maps_hot_dev_bg_proof_file_to_installability(self) -> None:
        result = lookup_paths(["scripts/tests/test-hot-dev-bg.sh"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(result["impacted_subsystems"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "test-or-fixture")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"deployment-installability"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L1")
        self.assertEqual(match["verification_requirement"]["id"], "dev-runtime-orchestration")
        self.assertIn(
            "scripts/tests/test-hot-dev-bg.sh",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_maps_integration_browser_default_surfaces_to_installability(self) -> None:
        result = lookup_paths(
            [
                "tests/integration/README.md",
                "tests/integration/QUICK_START.md",
                "tests/integration/playwright.config.ts",
                "tests/integration/tests/helpers.ts",
                "tests/integration/tests/runtime-defaults.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {entry["subsystem"] for entry in result["impacted_subsystems"]},
            {"deployment-installability"},
        )

        for file_entry in result["files"]:
            self.assertEqual(file_entry["classification"], "runtime")
            self.assertEqual(
                {match["subsystem"] for match in file_entry["matches"]},
                {"deployment-installability"},
            )
            match = file_entry["matches"][0]
            self.assertEqual(
                match["contract"],
                "docs/release-control/v6/internal/subsystems/deployment-installability.md",
            )
            self.assertEqual(match["lane_context"]["lane_id"], "L1")
            self.assertEqual(match["verification_requirement"]["id"], "dev-runtime-orchestration")
            self.assertIn(
                "scripts/tests/test-hot-dev-bg.sh",
                match["verification_requirement"]["exact_files"],
            )

    def test_lookup_maps_release_promotion_workflows_to_installability(self) -> None:
        result = lookup_paths(
            [
                ".github/workflows/publish-docker.yml",
                ".github/workflows/promote-floating-tags.yml",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {entry["subsystem"] for entry in result["impacted_subsystems"]},
            {"deployment-installability"},
        )

        for file_entry in result["files"]:
            self.assertEqual(file_entry["classification"], "runtime")
            self.assertEqual(
                {match["subsystem"] for match in file_entry["matches"]},
                {"deployment-installability"},
            )
            match = file_entry["matches"][0]
            self.assertEqual(
                match["contract"],
                "docs/release-control/v6/internal/subsystems/deployment-installability.md",
            )
            self.assertEqual(match["lane_context"]["lane_id"], "L1")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "release-promotion-metadata-runtime",
            )
            self.assertIn(
                "scripts/release_control/release_promotion_policy_test.py",
                match["verification_requirement"]["exact_files"],
            )

    def test_lookup_maps_release_artifact_follow_on_surfaces_to_installability(self) -> None:
        result = lookup_paths(
            [
                ".github/workflows/helm-pages.yml",
                ".github/workflows/publish-helm-chart.yml",
                ".github/workflows/update-demo-server.yml",
                "scripts/trigger-release-dry-run.sh",
                "scripts/trigger-release.sh",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {entry["subsystem"] for entry in result["impacted_subsystems"]},
            {"deployment-installability"},
        )

        for file_entry in result["files"]:
            self.assertEqual(file_entry["classification"], "runtime")
            self.assertEqual(
                {match["subsystem"] for match in file_entry["matches"]},
                {"deployment-installability"},
            )
            match = file_entry["matches"][0]
            self.assertEqual(
                match["contract"],
                "docs/release-control/v6/internal/subsystems/deployment-installability.md",
            )
            self.assertEqual(match["lane_context"]["lane_id"], "L1")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "release-promotion-metadata-runtime",
            )
            self.assertIn(
                "scripts/release_control/release_promotion_policy_test.py",
                match["verification_requirement"]["exact_files"],
            )


if __name__ == "__main__":
    unittest.main()
