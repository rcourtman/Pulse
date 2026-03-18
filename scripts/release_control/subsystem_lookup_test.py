import unittest
from pathlib import Path

from canonical_completion_guard import REPO_ROOT
from subsystem_lookup import lookup_paths


class SubsystemLookupTest(unittest.TestCase):
    def test_lookup_paths_reports_multiple_subsystems_for_shared_runtime_file(self) -> None:
        result = lookup_paths(["internal/api/resources.go"])
        impacted = {entry["subsystem"] for entry in result["impacted_subsystems"]}
        self.assertEqual(impacted, {"api-contracts", "unified-resources"})
        self.assertEqual(result["status_audit_errors"], [])
        self.assertIn(
            result["control_plane"]["active_target"]["id"],
            {"v6-rc-stabilization", "v6-ga-promotion"},
        )
        self.assertEqual(result["scope"]["control_plane_repo"], "pulse")
        self.assertEqual(result["status_summary"]["lane_count"], 16)

        file_entry = result["files"][0]
        matches = {match["subsystem"] for match in file_entry["matches"]}
        self.assertEqual(matches, {"api-contracts", "unified-resources"})
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "unified-resources"],
        )
        expected_lanes = {
            "api-contracts": "L6",
            "unified-resources": "L13",
        }
        for match in file_entry["matches"]:
            self.assertEqual(match["lane_context"]["lane_id"], expected_lanes[match["subsystem"]])
            self.assertEqual(match["lane_context"]["lane"]["id"], expected_lanes[match["subsystem"]])

    def test_lookup_paths_classifies_tests_without_runtime_matches(self) -> None:
        result = lookup_paths(["internal/api/contract_test.go"])
        self.assertEqual(result["files"][0]["classification"], "test-or-fixture")
        self.assertEqual(result["files"][0]["matches"], [])

    def test_lookup_paths_reports_unowned_runtime_files(self) -> None:
        result = lookup_paths(["README.md"])
        self.assertEqual(result["unowned_runtime_files"], ["README.md"])

    def test_lookup_paths_assigns_organization_billing_panel_to_cloud_paid(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/OrganizationBillingPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"cloud-paid"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"cloud-paid"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/cloud-paid.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L3")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "organization-billing-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/components/Settings/__tests__/OrganizationBillingPanel.test.tsx"],
        )

    def test_lookup_paths_assigns_pro_license_panel_to_cloud_paid(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/ProLicensePanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"cloud-paid"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"cloud-paid"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/cloud-paid.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L3")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "pro-license-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/components/Settings/__tests__/ProLicensePanel.test.tsx"],
        )

    def test_lookup_paths_assigns_recovery_route_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/pages/RecoveryRoute.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_dashboard_page_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/pages/Dashboard.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "dashboard-storage-recovery-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/pages/__tests__/DashboardPage.test.tsx",
            ],
        )

    def test_lookup_paths_assigns_dashboard_widgets_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/pages/DashboardPanels/dashboardWidgets.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "dashboard-storage-recovery-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/pages/__tests__/DashboardPage.test.tsx",
            ],
        )

    def test_lookup_paths_assigns_recovery_points_hook_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/hooks/useRecoveryPoints.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")

    def test_lookup_paths_assigns_recovery_points_facets_hook_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/hooks/useRecoveryPointsFacets.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")

    def test_lookup_paths_assigns_recovery_points_series_hook_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/hooks/useRecoveryPointsSeries.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")

    def test_lookup_paths_assigns_recovery_rollups_hook_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/hooks/useRecoveryRollups.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")

    def test_lookup_paths_assigns_dashboard_recovery_hook_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/hooks/useDashboardRecovery.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")

    def test_lookup_paths_assigns_recovery_status_panel_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/pages/DashboardPanels/RecoveryStatusPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")

    def test_lookup_paths_assigns_dashboard_recovery_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/dashboardRecoveryPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")

    def test_lookup_paths_assigns_storage_panel_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/pages/DashboardPanels/StoragePanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "storage-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Storage/__tests__/DiskList.test.tsx",
                "frontend-modern/src/components/Storage/__tests__/Storage.test.tsx",
                "frontend-modern/src/components/Storage/__tests__/StorageControls.test.tsx",
                "frontend-modern/src/components/Storage/__tests__/StorageGroupRow.test.tsx",
                "frontend-modern/src/components/Storage/__tests__/StoragePoolDetail.test.tsx",
                "frontend-modern/src/components/Storage/code_standards.test.ts",
                "frontend-modern/src/features/storageBackups/__tests__/resourceStorageMapping.test.ts",
                "frontend-modern/src/features/storageBackups/__tests__/storageAdapters.test.ts",
                "frontend-modern/src/features/storageBackups/__tests__/storageAlertState.test.ts",
                "frontend-modern/src/features/storageBackups/__tests__/storageDomain.test.ts",
                "frontend-modern/src/features/storageBackups/__tests__/storageModelCore.test.ts",
                "frontend-modern/src/features/storageBackups/__tests__/storagePagePresentation.test.ts",
                "frontend-modern/src/features/storageBackups/__tests__/storagePoolsTablePresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_types_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/types/recovery.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_date_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryDatePresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_status_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryStatusPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_summary_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoverySummaryPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_record_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryRecordPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_outcome_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryOutcomePresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_action_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryActionPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_artifact_mode_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryArtifactModePresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_empty_state_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryEmptyStatePresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_filter_chip_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryFilterChipPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_issue_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryIssuePresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_table_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryTablePresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_timeline_chart_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryTimelineChartPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_recovery_timeline_presentation_to_storage_recovery(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/recoveryTimelinePresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"storage-recovery"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"storage-recovery"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/storage-recovery.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "recovery-product-surface")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_api_token_manager_to_api_contracts(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/APITokenManager.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "security-privacy"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"api-contracts", "security-privacy"},
        )
        api_match = next(match for match in file_entry["matches"] if match["subsystem"] == "api-contracts")
        self.assertEqual(api_match["contract"], "docs/release-control/v6/internal/subsystems/api-contracts.md")
        self.assertEqual(api_match["lane_context"]["lane_id"], "L6")
        self.assertEqual(
            api_match["verification_requirement"]["id"],
            "api-token-management-surface",
        )
        self.assertEqual(
            api_match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/components/Settings/__tests__/APITokenManager.test.tsx"],
        )
        security_match = next(
            match for match in file_entry["matches"] if match["subsystem"] == "security-privacy"
        )
        self.assertEqual(
            security_match["contract"],
            "docs/release-control/v6/internal/subsystems/security-privacy.md",
        )
        self.assertEqual(security_match["lane_context"]["lane_id"], "L14")
        self.assertEqual(
            security_match["verification_requirement"]["id"],
            "security-settings-surfaces",
        )

    def test_lookup_paths_reports_security_client_as_shared_boundary(self) -> None:
        result = lookup_paths(["frontend-modern/src/api/security.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "security-privacy"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"api-contracts", "security-privacy"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "security-privacy"],
        )
        expected = {
            "api-contracts": ("L6", "security-transport-contract"),
            "security-privacy": ("L14", "security-api-surface"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_security_handler_as_shared_boundary(self) -> None:
        result = lookup_paths(["internal/api/security.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "security-privacy"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"api-contracts", "security-privacy"},
        )
        expected = {
            "api-contracts": ("L6", "security-transport-contract"),
            "security-privacy": ("L14", "security-api-surface"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_security_tokens_handler_as_shared_boundary(self) -> None:
        result = lookup_paths(["internal/api/security_tokens.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "security-privacy"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"api-contracts", "security-privacy"},
        )
        expected = {
            "api-contracts": ("L6", "security-transport-contract"),
            "security-privacy": ("L14", "security-api-surface"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_system_settings_handler_as_shared_boundary(self) -> None:
        result = lookup_paths(["internal/api/system_settings.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "security-privacy"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"api-contracts", "security-privacy"},
        )
        expected = {
            "api-contracts": ("L6", "security-transport-contract"),
            "security-privacy": ("L14", "security-api-surface"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_ai_frontend_client_as_shared_boundary(self) -> None:
        result = lookup_paths(["frontend-modern/src/api/ai.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"ai-runtime", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"ai-runtime", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["ai-runtime", "api-contracts"],
        )
        expected = {
            "ai-runtime": ("L6", "ai-api-surface"),
            "api-contracts": ("L6", "frontend-api-clients"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_agent_profiles_client_as_shared_boundary(self) -> None:
        result = lookup_paths(["frontend-modern/src/api/agentProfiles.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["agent-lifecycle", "api-contracts"],
        )
        expected = {
            "agent-lifecycle": ("L16", "agent-profiles-surface"),
            "api-contracts": ("L6", "agent-profiles-client-contract"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_nodes_client_as_shared_boundary(self) -> None:
        result = lookup_paths(["frontend-modern/src/api/nodes.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["agent-lifecycle", "api-contracts"],
        )
        expected = {
            "agent-lifecycle": ("L16", "proxmox-lifecycle-client-surface"),
            "api-contracts": ("L6", "proxmox-node-client-contract"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_updates_client_as_shared_boundary(self) -> None:
        result = lookup_paths(["frontend-modern/src/api/updates.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"deployment-installability", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"deployment-installability", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "deployment-installability"],
        )
        expected = {
            "deployment-installability": ("L1", "updates-api-surface"),
            "api-contracts": ("L6", "update-transport-contract"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_updates_handler_as_shared_boundary(self) -> None:
        result = lookup_paths(["internal/api/updates.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"deployment-installability", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"deployment-installability", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "deployment-installability"],
        )
        expected = {
            "deployment-installability": ("L1", "updates-api-surface"),
            "api-contracts": ("L6", "update-transport-contract"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_frontend_install_helper_as_shared_boundary(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/agentInstallCommand.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["agent-lifecycle", "api-contracts"],
        )
        expected = {
            "agent-lifecycle": ("L16", "frontend-install-command-helper"),
            "api-contracts": ("L6", "frontend-install-command-helper"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_assigns_node_modal_to_agent_lifecycle(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/NodeModal.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "node-setup-settings-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Settings/__tests__/NodeModal.guardrails.test.ts"
            ],
        )

    def test_lookup_paths_assigns_setup_completion_panel_to_agent_lifecycle(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "setup-completion-install-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/SetupWizard/__tests__/SetupCompletionPanel.guardrails.test.ts"
            ],
        )

    def test_lookup_paths_assigns_results_step_to_agent_lifecycle(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Infrastructure/deploy/ResultsStep.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "deploy-fallback-install-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Infrastructure/__tests__/DeployStepComponents.test.tsx"
            ],
        )

    def test_lookup_paths_assigns_agent_profiles_panel_to_agent_lifecycle(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/AgentProfilesPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "agent-profiles-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                "frontend-modern/src/components/Settings/__tests__/AgentProfilesPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                "frontend-modern/src/utils/__tests__/agentProfilesPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_agent_profiles_presentation_to_agent_lifecycle(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/agentProfilesPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "agent-profiles-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                "frontend-modern/src/components/Settings/__tests__/AgentProfilesPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                "frontend-modern/src/utils/__tests__/agentProfilesPresentation.test.ts",
            ],
        )

    def test_lookup_paths_reports_agent_install_backend_as_shared_boundary(self) -> None:
        result = lookup_paths(["internal/api/agent_install_command_shared.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["agent-lifecycle", "api-contracts"],
        )
        expected = {
            "agent-lifecycle": ("L16", "agent-install-api-surface"),
            "api-contracts": ("L6", "agent-install-backend-contract"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_assigns_internal_ai_runtime_to_ai_runtime(self) -> None:
        result = lookup_paths(["internal/ai/intelligence.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"ai-runtime"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"ai-runtime"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/ai-runtime.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
        self.assertEqual(match["verification_requirement"]["id"], "ai-runtime-engine")
        self.assertEqual(match["verification_requirement"]["test_prefixes"], ["internal/ai/"])

    def test_lookup_paths_reports_notification_client_as_shared_boundary(self) -> None:
        result = lookup_paths(["frontend-modern/src/api/notifications.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "notifications"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"api-contracts", "notifications"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "notifications"],
        )
        expected = {
            "api-contracts": ("L6", "frontend-api-clients"),
            "notifications": ("L6", "notifications-api-surface"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_unified_agent_backend_as_shared_boundary(self) -> None:
        result = lookup_paths(["internal/api/unified_agent.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["agent-lifecycle", "api-contracts"],
        )
        expected = {
            "agent-lifecycle": ("L16", "agent-install-api-surface"),
            "api-contracts": ("L6", "agent-install-backend-contract"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_reports_config_setup_backend_as_shared_boundary(self) -> None:
        result = lookup_paths(["internal/api/config_setup_handlers.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle", "api-contracts"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["agent-lifecycle", "api-contracts"],
        )
        expected = {
            "agent-lifecycle": ("L16", "agent-install-api-surface"),
            "api-contracts": ("L6", "agent-install-backend-contract"),
        }
        for match in file_entry["matches"]:
            lane_id, requirement_id = expected[match["subsystem"]]
            self.assertEqual(match["lane_context"]["lane_id"], lane_id)
            self.assertEqual(match["verification_requirement"]["id"], requirement_id)

    def test_lookup_paths_assigns_install_script_to_shared_installer_subsystems(self) -> None:
        result = lookup_paths(["scripts/install.sh"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "deployment-installability"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"agent-lifecycle", "deployment-installability"},
        )
        by_subsystem = {match["subsystem"]: match for match in file_entry["matches"]}

        match = by_subsystem["agent-lifecycle"]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "unified-agent-installer-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["scripts/installtests/install_sh_test.go"],
        )

        match = by_subsystem["deployment-installability"]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L1")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "shell-installer-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["scripts/installtests/install_sh_test.go"],
        )

    def test_lookup_paths_assigns_docker_entrypoint_to_monitoring(self) -> None:
        result = lookup_paths(["docker-entrypoint.sh"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"monitoring"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"monitoring"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/monitoring.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "container-entrypoint-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["scripts/installtests/docker_entrypoint_test.go"],
        )

    def test_lookup_paths_assigns_relay_client_to_relay_runtime(self) -> None:
        result = lookup_paths(["internal/relay/client.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"relay-runtime"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"relay-runtime"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/relay-runtime.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L7")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "relay-client-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["internal/relay/client_test.go"],
        )

    def test_lookup_paths_assigns_relay_directory_root_to_relay_runtime(self) -> None:
        result = lookup_paths(["internal/relay"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"relay-runtime"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"relay-runtime"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/relay-runtime.md",
        )
        self.assertEqual(match["verification_requirement"]["id"], "relay-client-runtime")

    def test_lookup_paths_assigns_relay_config_persistence_to_relay_runtime(self) -> None:
        result = lookup_paths(["internal/config/persistence_relay.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"relay-runtime"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"relay-runtime"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/relay-runtime.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L7")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "relay-config-persistence",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["internal/config/persistence_relay_test.go"],
        )

    def test_lookup_paths_assigns_metrics_directory_root_to_performance_and_scalability(self) -> None:
        result = lookup_paths(["pkg/metrics"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"performance-and-scalability"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/performance-and-scalability.md",
        )
        self.assertEqual(
            match["verification_requirement"]["id"],
            "metrics-store-hot-paths",
        )
        self.assertIn(
            "pkg/metrics/store_additional_test.go",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_settings_page_shell_to_frontend_primitives(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/SettingsPageShell.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"frontend-primitives"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"frontend-primitives"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/frontend-primitives.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L8")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "settings-shell-and-framing",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts"],
        )

    def test_lookup_paths_assigns_network_settings_panel_to_frontend_primitives(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"frontend-primitives"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"frontend-primitives"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/frontend-primitives.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L8")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "settings-shell-and-framing",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts"],
        )

    def test_lookup_paths_assigns_security_auth_panel_to_frontend_primitives(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/SecurityAuthPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"frontend-primitives", "security-privacy"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"frontend-primitives", "security-privacy"},
        )
        match = next(match for match in file_entry["matches"] if match["subsystem"] == "frontend-primitives")
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/frontend-primitives.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L8")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "settings-shell-and-framing",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts"],
        )
        security_match = next(
            match for match in file_entry["matches"] if match["subsystem"] == "security-privacy"
        )
        self.assertEqual(
            security_match["contract"],
            "docs/release-control/v6/internal/subsystems/security-privacy.md",
        )
        self.assertEqual(security_match["lane_context"]["lane_id"], "L14")
        self.assertEqual(
            security_match["verification_requirement"]["id"],
            "security-settings-surfaces",
        )

    def test_lookup_paths_assigns_remaining_settings_shell_panels_to_frontend_primitives(self) -> None:
        runtime_files = [
            "frontend-modern/src/components/Settings/APIAccessPanel.tsx",
            "frontend-modern/src/components/Settings/AISettings.tsx",
            "frontend-modern/src/components/Settings/AuditLogPanel.tsx",
            "frontend-modern/src/components/Settings/AuditWebhookPanel.tsx",
            "frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx",
            "frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx",
            "frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx",
            "frontend-modern/src/components/Settings/SSOProvidersPanel.tsx",
            "frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx",
        ]
        result = lookup_paths(runtime_files)
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"frontend-primitives", "security-privacy"},
        )
        for file_entry in result["files"]:
            self.assertEqual(file_entry["classification"], "runtime")
            subsystems = {match["subsystem"] for match in file_entry["matches"]}
            if file_entry["path"] in {
                "frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx",
                "frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx",
            }:
                self.assertEqual(subsystems, {"frontend-primitives", "security-privacy"})
            else:
                self.assertEqual(subsystems, {"frontend-primitives"})
            match = next(
                match for match in file_entry["matches"] if match["subsystem"] == "frontend-primitives"
            )
            self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/frontend-primitives.md")
            self.assertEqual(match["lane_context"]["lane_id"], "L8")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "settings-shell-and-framing",
            )
            self.assertEqual(
                match["verification_requirement"]["exact_files"],
                ["frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts"],
            )

    def test_lookup_paths_assigns_organization_sharing_panel_to_organization_settings(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"organization-settings"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"organization-settings"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/organization-settings.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L14")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "organization-settings-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/orgs.test.ts",
                "frontend-modern/src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/RBACPaywallPanels.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                "frontend-modern/src/utils/__tests__/orgUtils.test.ts",
                "frontend-modern/src/utils/__tests__/organizationRolePresentation.test.ts",
                "frontend-modern/src/utils/__tests__/organizationSettingsPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_roles_panel_to_organization_settings(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/RolesPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"organization-settings"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"organization-settings"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/organization-settings.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L14")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "organization-settings-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/orgs.test.ts",
                "frontend-modern/src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/RBACPaywallPanels.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                "frontend-modern/src/utils/__tests__/orgUtils.test.ts",
                "frontend-modern/src/utils/__tests__/organizationRolePresentation.test.ts",
                "frontend-modern/src/utils/__tests__/organizationSettingsPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_user_assignments_panel_to_organization_settings(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/UserAssignmentsPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"organization-settings"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"organization-settings"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/organization-settings.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L14")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "organization-settings-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/orgs.test.ts",
                "frontend-modern/src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/RBACPaywallPanels.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                "frontend-modern/src/utils/__tests__/orgUtils.test.ts",
                "frontend-modern/src/utils/__tests__/organizationRolePresentation.test.ts",
                "frontend-modern/src/utils/__tests__/organizationSettingsPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_organization_model_to_organization_settings(self) -> None:
        result = lookup_paths(["internal/models/organization.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"organization-settings"},
        )
        match = result["files"][0]["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/organization-settings.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L14")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "organization-domain-model",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/api/org_handlers_test.go",
                "internal/api/org_lifecycle_handlers_test.go",
                "internal/api/org_validation_test.go",
                "internal/models/organization_additional_test.go",
            ],
        )

    def test_lookup_paths_assigns_ai_intelligence_page_to_patrol_intelligence(self) -> None:
        result = lookup_paths(["frontend-modern/src/pages/AIIntelligence.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"patrol-intelligence"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"patrol-intelligence"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/patrol-intelligence.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "patrol-page-and-state",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/Brand/__tests__/PulsePatrolLogo.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                "frontend-modern/src/pages/__tests__/AIIntelligence.test.tsx",
                "frontend-modern/src/stores/__tests__/aiIntelligence.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_findings_panel_to_patrol_intelligence(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/AI/FindingsPanel.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"patrol-intelligence"},
        )
        match = result["files"][0]["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/patrol-intelligence.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "patrol-findings-and-approvals",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/components/AI/__tests__/FindingsPanel.test.ts",
                "frontend-modern/src/components/patrol/__tests__/ApprovalSection.test.tsx",
                "frontend-modern/src/components/patrol/__tests__/InvestigationSection.test.tsx",
                "frontend-modern/src/utils/__tests__/approvalRiskPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/findingAlertIdentity.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_unified_agents_to_shared_agent_lifecycle_and_api_contracts(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/UnifiedAgents.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["agent-lifecycle", "api-contracts"],
        )
        matches = {match["subsystem"] for match in file_entry["matches"]}
        self.assertEqual(matches, {"agent-lifecycle", "api-contracts"})

        by_subsystem = {match["subsystem"]: match for match in file_entry["matches"]}
        api_match = by_subsystem["api-contracts"]
        lifecycle_match = by_subsystem["agent-lifecycle"]

        self.assertEqual(
            api_match["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(api_match["lane_context"]["lane_id"], "L6")
        self.assertEqual(
            api_match["verification_requirement"]["id"],
            "unified-agent-settings-surface",
        )
        self.assertEqual(
            api_match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                "frontend-modern/src/api/__tests__/monitoring.test.ts",
                "frontend-modern/src/api/__tests__/security.test.ts",
                "frontend-modern/src/components/Settings/__tests__/UnifiedAgents.test.tsx",
            ],
        )

        self.assertEqual(
            lifecycle_match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(lifecycle_match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            lifecycle_match["verification_requirement"]["id"],
            "unified-agent-settings-surface",
        )
        self.assertEqual(
            lifecycle_match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                "frontend-modern/src/api/__tests__/monitoring.test.ts",
                "frontend-modern/src/components/Settings/__tests__/UnifiedAgents.test.tsx",
            ],
        )

    def test_lookup_paths_reports_windows_installer_as_shared_boundary(self) -> None:
        result = lookup_paths(["scripts/install.ps1"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle", "deployment-installability"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["agent-lifecycle", "deployment-installability"],
        )
        matches = {match["subsystem"] for match in file_entry["matches"]}
        self.assertEqual(matches, {"agent-lifecycle", "deployment-installability"})

        by_subsystem = {match["subsystem"]: match for match in file_entry["matches"]}
        lifecycle_match = by_subsystem["agent-lifecycle"]
        installability_match = by_subsystem["deployment-installability"]

        self.assertEqual(
            lifecycle_match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(lifecycle_match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            lifecycle_match["verification_requirement"]["id"],
            "windows-agent-installer-runtime",
        )
        self.assertEqual(
            lifecycle_match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/install_ps1_test.go",
            ],
        )

        self.assertEqual(
            installability_match["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(installability_match["lane_context"]["lane_id"], "L1")
        self.assertEqual(
            installability_match["verification_requirement"]["id"],
            "deployment-script-runtime",
        )
        self.assertEqual(
            installability_match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/install_docker_sh_test.go",
                "scripts/installtests/install_ps1_test.go",
                "scripts/installtests/root_install_sh_test.go",
            ],
        )

    def test_lookup_paths_assigns_container_installer_to_deployment_installability(self) -> None:
        result = lookup_paths(["scripts/install-container-agent.sh"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"deployment-installability"},
        )
        file_entry = result["files"][0]
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
            "deployment-script-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/install_docker_sh_test.go",
                "scripts/installtests/install_ps1_test.go",
                "scripts/installtests/root_install_sh_test.go",
            ],
        )

    def test_lookup_paths_assigns_auto_update_script_to_deployment_installability(self) -> None:
        result = lookup_paths(["scripts/pulse-auto-update.sh"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"deployment-installability"},
        )
        file_entry = result["files"][0]
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
            "deployment-script-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/install_docker_sh_test.go",
                "scripts/installtests/install_ps1_test.go",
                "scripts/installtests/root_install_sh_test.go",
            ],
        )

    def test_lookup_paths_normalizes_absolute_repo_paths(self) -> None:
        absolute = str(Path(REPO_ROOT, "internal/api/resources.go"))
        result = lookup_paths([absolute])
        self.assertEqual(result["files"][0]["path"], "internal/api/resources.go")

    def test_lookup_paths_classifies_governance_files_as_ignored(self) -> None:
        result = lookup_paths(["docs/release-control/v6/internal/status.json"])
        self.assertEqual(result["files"][0]["classification"], "ignored")
        self.assertEqual(result["files"][0]["matches"], [])

    def test_lookup_paths_includes_relevant_open_decisions_for_lane(self) -> None:
        result = lookup_paths(["pkg/licensing/features.go"])
        match = next(
            item
            for item in result["files"][0]["matches"]
            if item["subsystem"] == "cloud-paid"
        )
        lane_context = match["lane_context"]
        self.assertEqual(lane_context["lane_id"], "L3")
        self.assertEqual(lane_context["open_decisions"], [])
        self.assertEqual(
            {gate["id"] for gate in lane_context["release_gates"]},
            {
                "cloud-hosted-tier-runtime-readiness",
                "commercial-cancellation-reactivation",
                "hosted-signup-billing-replay",
                "msp-provider-tenant-management",
                "paid-feature-entitlement-gating",
                "upgrade-state-and-entitlement-preservation",
            },
        )

    def test_lookup_paths_keeps_pricing_and_migration_resolved_decisions_for_cloud_paid_lane(self) -> None:
        result = lookup_paths(["pkg/licensing/features.go"])
        match = next(
            item
            for item in result["files"][0]["matches"]
            if item["subsystem"] == "cloud-paid"
        )
        lane_context = match["lane_context"]
        self.assertEqual(lane_context["lane_id"], "L3")
        self.assertEqual(
            {decision["id"] for decision in lane_context["resolved_decisions"]},
            {
                "cloud-msp-price-id-propagation",
                "cloud-msp-stripe-prices",
                "stable-release-promotion-model",
                "stripe-mapping-contract-lock",
                "trial-authority-saas-controlled",
                "v5-pro-price-grandfathering",
            },
        )

    def test_lookup_paths_keeps_cross_cutting_resolved_decisions_for_lane(self) -> None:
        result = lookup_paths(["internal/monitoring/monitor.go"])
        match = next(
            item
            for item in result["files"][0]["matches"]
            if item["subsystem"] == "monitoring"
        )
        lane_context = match["lane_context"]
        self.assertEqual(lane_context["lane_id"], "L13")
        self.assertEqual(
            {decision["id"] for decision in lane_context["resolved_decisions"]},
            {"host-type-migration-boundary-audit"},
        )

    def test_lookup_paths_assigns_monitoring_discovery_runtime(self) -> None:
        result = lookup_paths(["internal/monitoring/poll_providers.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "monitoring")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/monitoring.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(match["verification_requirement"]["id"], "discovery-provider-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/monitoring/kubernetes_agents_test.go",
                "internal/monitoring/monitor_host_agents_test.go",
                "internal/monitoring/monitor_pbs_coverage_test.go",
                "internal/monitoring/monitor_pmg_test.go",
                "internal/monitoring/monitor_polling_test.go",
                "internal/monitoring/truenas_poller_test.go",
            ],
        )

    def test_lookup_paths_assigns_monitoring_metrics_history_runtime(self) -> None:
        result = lookup_paths(["internal/monitoring/metrics_history.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "monitoring")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/monitoring.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(match["verification_requirement"]["id"], "metrics-history-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/monitoring/metrics_history_concurrency_test.go",
                "internal/monitoring/metrics_history_memory_regression_test.go",
                "internal/monitoring/metrics_history_test.go",
                "internal/monitoring/metrics_test.go",
                "internal/monitoring/mock_metrics_history_test.go",
            ],
        )

    def test_lookup_paths_assigns_unified_resource_metrics_targets_runtime(self) -> None:
        result = lookup_paths(["internal/unifiedresources/metrics_targets.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "unified-resources")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/unified-resources.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(match["verification_requirement"]["id"], "metrics-target-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/unifiedresources/metrics_targets_test.go",
                "internal/unifiedresources/metrics_test.go",
            ],
        )

    def test_lookup_paths_assigns_unified_resource_platform_registry_runtime(self) -> None:
        result = lookup_paths(["internal/unifiedresources/registry.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "unified-resources")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/unified-resources.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(match["verification_requirement"]["id"], "platform-registry-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/unifiedresources/kubernetes_registry_test.go",
                "internal/unifiedresources/pbs_pmg_registry_test.go",
                "internal/unifiedresources/registry_merge_policy_test.go",
                "internal/unifiedresources/registry_test.go",
                "internal/unifiedresources/resolve_test.go",
                "internal/unifiedresources/resolved_host_set_test.go",
                "internal/unifiedresources/snapshot_source_filter_test.go",
                "internal/unifiedresources/store_test.go",
            ],
        )

    def test_lookup_paths_assigns_unified_resource_kubernetes_capabilities_runtime(self) -> None:
        result = lookup_paths(["internal/unifiedresources/kubernetes_capabilities.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "unified-resources")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/unified-resources.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(match["verification_requirement"]["id"], "platform-registry-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/unifiedresources/kubernetes_registry_test.go",
                "internal/unifiedresources/pbs_pmg_registry_test.go",
                "internal/unifiedresources/registry_merge_policy_test.go",
                "internal/unifiedresources/registry_test.go",
                "internal/unifiedresources/resolve_test.go",
                "internal/unifiedresources/resolved_host_set_test.go",
                "internal/unifiedresources/snapshot_source_filter_test.go",
                "internal/unifiedresources/store_test.go",
            ],
        )

    def test_lookup_paths_reports_dependent_contract_updates_for_shared_canonical_file(self) -> None:
        result = lookup_paths(["internal/unifiedresources/views.go"])
        file_entry = result["files"][0]
        self.assertEqual(
            {contract["subsystem"] for contract in file_entry["dependent_contract_updates"]},
            {"monitoring"},
        )
        self.assertEqual(
            {contract["subsystem"] for contract in result["required_contract_updates"]},
            {"monitoring", "unified-resources"},
        )

    def test_lookup_paths_maps_unified_agent_runtime_to_agent_lifecycle(self) -> None:
        result = lookup_paths(["internal/hostagent/agent.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "agent-lifecycle")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/agent-lifecycle.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "unified-agent-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/agentupdate/coverage_test.go",
                "internal/hostagent/agent_metrics_test.go",
                "internal/hostagent/agent_new_test.go",
                "internal/hostagent/send_report_test.go",
            ],
        )

    def test_lookup_paths_maps_proxmox_setup_runtime_to_agent_lifecycle(self) -> None:
        result = lookup_paths(["internal/hostagent/proxmox_setup.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "agent-lifecycle")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/agent-lifecycle.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "proxmox-unified-agent-setup-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/hostagent/proxmox_setup_network_coverage_test.go",
                "internal/hostagent/proxmox_setup_test.go",
            ],
        )

    def test_lookup_paths_maps_agentupdate_runtime_to_agent_lifecycle(self) -> None:
        result = lookup_paths(["internal/agentupdate/update.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "agent-lifecycle")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/agent-lifecycle.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(match["verification_requirement"]["id"], "agent-update-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/agentupdate/coverage_test.go",
                "internal/agentupdate/update_hostagent_integration_test.go",
                "internal/agentupdate/update_http_test.go",
            ],
        )


if __name__ == "__main__":
    unittest.main()
