import unittest
from pathlib import Path

from canonical_completion_guard import REPO_ROOT
from subsystem_lookup import lookup_paths, parse_args, render_pretty


RECOVERY_PRODUCT_SURFACE_EXACT_FILES = [
    "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
    "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
    "tests/integration/tests/17-recovery-layout.spec.ts",
]

PATROL_PAGE_AND_STATE_EXACT_FILES = [
    "frontend-modern/src/components/Brand/__tests__/PulsePatrolLogo.test.tsx",
    "frontend-modern/src/features/patrol/__tests__/PatrolIntelligenceHeader.test.ts",
    "frontend-modern/src/features/patrol/__tests__/patrolControlPresentation.test.ts",
    "frontend-modern/src/features/patrol/__tests__/patrolInvestigationContextModel.test.ts",
    "frontend-modern/src/pages/__tests__/AIIntelligence.test.tsx",
    "frontend-modern/src/stores/__tests__/aiIntelligence.test.ts",
    "frontend-modern/src/stores/__tests__/aiIntelligenceSummaryModel.test.ts",
    "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
    "frontend-modern/src/utils/__tests__/patrolPagePresentation.test.ts",
    "tests/integration/tests/18-patrol-runtime-state.spec.ts",
    "tests/integration/tests/73-patrol-assistant-operator-briefing.spec.ts",
    "tests/integration/tests/78-monitor-first-patrol-workbench.spec.ts",
]

PLATFORM_CONNECTIONS_WORKSPACE_EXACT_FILES = [
    "frontend-modern/src/components/Settings/ConnectionEditor/__tests__/ConnectionEditor.test.tsx",
    "frontend-modern/src/components/Settings/__tests__/InfrastructureWorkspace.test.tsx",
    "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
    "frontend-modern/src/components/Settings/__tests__/useTrueNASSettingsPanelState.test.tsx",
    "frontend-modern/src/components/Settings/__tests__/useVMwareSettingsPanelState.test.tsx",
    "frontend-modern/src/utils/__tests__/clusterEndpointPresentation.test.ts",
    "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
    "frontend-modern/src/utils/__tests__/proxmoxSettingsPresentation.test.ts",
    "tests/integration/tests/21-truenas-settings-platform-connections.spec.ts",
    "tests/integration/tests/22-vmware-settings-platform-connections.spec.ts",
]

CONNECTIONS_LEDGER_WORKSPACE_EXACT_FILES = [
    "frontend-modern/src/components/Settings/__tests__/InfrastructureWorkspace.test.tsx",
    "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
]


def _contract_reference(contract_path: str, needle: str, runtime_path: str) -> dict:
    lines = (REPO_ROOT / contract_path).read_text(encoding="utf-8").splitlines()
    current_heading = None
    current_heading_line = None

    for line_number, line in enumerate(lines, start=1):
        if line.startswith("## "):
            current_heading = line
            current_heading_line = line_number
        if needle in line:
            if current_heading is None or current_heading_line is None:
                raise AssertionError(
                    f"reference {needle!r} in {contract_path} has no enclosing heading"
                )
            return {
                "heading": current_heading,
                "path": runtime_path,
                "line": line_number,
                "heading_line": current_heading_line,
            }

    raise AssertionError(f"reference {needle!r} not found in {contract_path}")


class SubsystemLookupTest(unittest.TestCase):
    def test_parse_args_accepts_lean_flag(self) -> None:
        args = parse_args(["internal/api/ai_handler.go", "--pretty", "--lean"])
        self.assertEqual(args.paths, ["internal/api/ai_handler.go"])
        self.assertTrue(args.pretty)
        self.assertTrue(args.lean)

    def test_lookup_paths_reports_multiple_subsystems_for_shared_runtime_file(self) -> None:
        result = lookup_paths(["internal/api/resources.go"])
        impacted = {entry["subsystem"] for entry in result["impacted_subsystems"]}
        self.assertEqual(impacted, {"api-contracts", "unified-resources"})
        self.assertEqual(result["status_audit_errors"], [])
        self.assertIn(
            result["control_plane"]["active_target"]["id"],
            {"v6-rc-cut", "v6-rc-stabilization", "v6-ga-promotion", "v6-product-lane-expansion"},
        )
        self.assertEqual(result["scope"]["control_plane_repo"], "pulse")
        self.assertEqual(result["status_summary"]["lane_count"], 23)

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
        result = lookup_paths(["tests/example.spec.ts"])
        self.assertEqual(result["files"][0]["classification"], "test-or-fixture")
        self.assertEqual(result["files"][0]["matches"], [])

    def test_lookup_paths_reports_unowned_runtime_files(self) -> None:
        result = lookup_paths(["README.md"])
        self.assertEqual(result["unowned_runtime_files"], ["README.md"])

    def test_lookup_paths_assigns_action_planning_cli_to_api_contracts(self) -> None:
        result = lookup_paths(["pkg/pulsecli/actions.go", "pkg/pulsecli/root.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts"},
        )

        for file_entry in result["files"]:
            self.assertEqual(file_entry["classification"], "runtime")
            self.assertEqual(
                {match["subsystem"] for match in file_entry["matches"]},
                {"api-contracts"},
            )
            match = file_entry["matches"][0]
            self.assertEqual(match["lane_context"]["lane_id"], "L6")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "pulse-cli-action-planning-contract",
            )

    def test_lookup_paths_normalizes_cross_repo_absolute_runtime_paths(self) -> None:
        result = lookup_paths([str(REPO_ROOT.parent / "pulse-mobile" / "src/relay/client.ts")])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"relay-runtime"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["path"], "pulse-mobile:src/relay/client.ts")
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"relay-runtime"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["lane_context"]["lane_id"], "L7")
        self.assertEqual(match["verification_requirement"]["id"], "mobile-relay-runtime")

    def test_lookup_paths_normalizes_workspace_relative_cross_repo_runtime_paths(self) -> None:
        result = lookup_paths(["repos/pulse-pro/scripts/grandfathered_recurring_cutover_preview.py"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"cloud-paid"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["path"], "pulse-pro:scripts/grandfathered_recurring_cutover_preview.py")
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"cloud-paid"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["lane_context"]["lane_id"], "L3")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "legacy-grandfathering-cutover",
        )

    def test_lookup_paths_assigns_shared_tag_badges_to_frontend_primitives(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/shared/TagBadges.tsx"])
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
            "shared-component-guardrails",
        )

    def test_lookup_paths_assigns_resource_detail_drawer_service_model_to_unified_resources(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Infrastructure/resourceDetailDrawerServiceModel.ts"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"unified-resources"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"unified-resources"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "resource-consumers",
        )
        self.assertIn(
            "frontend-modern/src/components/Infrastructure/__tests__/ResourceDetailDrawer.history.test.tsx",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_resource_detail_drawer_operational_model_to_unified_resources(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Infrastructure/resourceDetailDrawerOperationalModel.ts"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"unified-resources"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"unified-resources"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "resource-consumers",
        )
        self.assertIn(
            "frontend-modern/src/components/Infrastructure/__tests__/ResourceDetailDrawer.history.test.tsx",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_resource_detail_drawer_identity_model_to_unified_resources(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Infrastructure/resourceDetailDrawerIdentityModel.ts"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"unified-resources"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"unified-resources"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "resource-consumers",
        )
        self.assertIn(
            "frontend-modern/src/components/Infrastructure/__tests__/resourceDetailDrawerIdentityModel.test.ts",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_unified_resource_table_state_model_to_shared_infrastructure_hot_path(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability", "unified-resources"},
        )

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"performance-and-scalability", "unified-resources"},
        )
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["performance-and-scalability", "unified-resources"],
        )

        matches_by_subsystem = {
            match["subsystem"]: match for match in file_entry["matches"]
        }
        performance_match = matches_by_subsystem["performance-and-scalability"]
        self.assertEqual(
            performance_match["contract"],
            "docs/release-control/v6/internal/subsystems/performance-and-scalability.md",
        )
        self.assertEqual(performance_match["lane_context"]["lane_id"], "L10")
        self.assertEqual(
            performance_match["verification_requirement"]["id"],
            "workloads-hot-path",
        )

        unified_resources_match = matches_by_subsystem["unified-resources"]
        self.assertEqual(
            unified_resources_match["contract"],
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
        )
        self.assertEqual(unified_resources_match["lane_context"]["lane_id"], "L13")
        self.assertEqual(
            unified_resources_match["verification_requirement"]["id"],
            "resource-consumers",
        )
        self.assertIn(
            "frontend-modern/src/components/Infrastructure/__tests__/unifiedResourceTableStateModel.test.ts",
            unified_resources_match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_resource_detail_drawer_discovery_model_to_unified_resources(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Infrastructure/resourceDetailDiscoveryModel.ts"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"unified-resources"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"unified-resources"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "resource-consumers",
        )
        self.assertIn(
            "frontend-modern/src/components/Infrastructure/__tests__/ResourceDetailDrawer.discovery.test.ts",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_bulk_edit_dialog_to_alerts(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Alerts/BulkEditDialog.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"alerts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual({match["subsystem"] for match in file_entry["matches"]}, {"alerts"})
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/alerts.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
        self.assertEqual(match["verification_requirement"]["id"], "alerts-frontend-surface")
        self.assertIn(
            "frontend-modern/src/components/Alerts/__tests__/BulkEditDialog.test.tsx",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_thresholds_table_state_hook_to_alerts(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsTableState.ts"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"alerts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual({match["subsystem"] for match in file_entry["matches"]}, {"alerts"})
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/alerts.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
        self.assertEqual(match["verification_requirement"]["id"], "alerts-frontend-surface")
        self.assertIn(
            "frontend-modern/src/features/alerts/thresholds/hooks/__tests__/useThresholdsTableState.test.tsx",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_alert_overview_presentation_to_alerts(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/alertOverviewPresentation.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"alerts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual({match["subsystem"] for match in file_entry["matches"]}, {"alerts"})
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/alerts.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
        self.assertEqual(match["verification_requirement"]["id"], "alerts-frontend-surface")
        self.assertIn(
            "frontend-modern/src/utils/__tests__/alertOverviewPresentation.test.ts",
            match["verification_requirement"]["exact_files"],
        )

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

    def test_lookup_paths_assigns_organization_billing_state_owner_to_cloud_paid(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useOrganizationBillingPanelState.ts"]
        )
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

    def test_lookup_paths_assigns_pro_license_state_owner_to_cloud_paid(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/useProLicensePanelState.ts"])
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

    def test_lookup_paths_assigns_pricing_handoff_to_cloud_paid(self) -> None:
        result = lookup_paths(["frontend-modern/src/pages/PricingHandoff.tsx"])
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
        self.assertEqual(match["verification_requirement"]["id"], "pricing-pages")

    def test_lookup_paths_assigns_self_hosted_plan_model_to_cloud_paid(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/selfHostedPlans.ts"])
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
        self.assertEqual(match["verification_requirement"]["id"], "commercial-plan-models")

    def test_lookup_paths_assigns_relay_settings_state_owner_to_cloud_paid(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/useRelaySettingsPanelState.ts"])
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
            "relay-frontend-surfaces",
        )

    def test_lookup_paths_reports_retired_dashboard_page_as_unowned(self) -> None:
        path = "frontend-modern/src/pages/Dashboard.tsx"
        result = lookup_paths([path])
        self.assertEqual(result["unowned_runtime_files"], [path])
        self.assertEqual(result["impacted_subsystems"], [])
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(file_entry["matches"], [])

    def test_lookup_paths_reports_retired_dashboard_widgets_as_unowned(self) -> None:
        path = "frontend-modern/src/features/dashboardOverview/dashboardWidgets.ts"
        result = lookup_paths([path])
        self.assertEqual(result["unowned_runtime_files"], [path])
        self.assertEqual(result["impacted_subsystems"], [])
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(file_entry["matches"], [])

    def test_lookup_paths_reports_retired_dashboard_overview_as_unowned(self) -> None:
        path = "frontend-modern/src/features/dashboardOverview/ProblemResourcesTable.tsx"
        result = lookup_paths([path])
        self.assertEqual(result["unowned_runtime_files"], [path])
        self.assertEqual(result["impacted_subsystems"], [])
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(file_entry["matches"], [])

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

    def test_lookup_paths_reports_retired_dashboard_recovery_hook_as_unowned(self) -> None:
        path = "frontend-modern/src/hooks/useDashboardRecovery.ts"
        result = lookup_paths([path])
        self.assertEqual(result["unowned_runtime_files"], [path])
        self.assertEqual(result["impacted_subsystems"], [])
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(file_entry["matches"], [])

    def test_lookup_paths_reports_retired_dashboard_recovery_presentation_as_unowned(self) -> None:
        path = "frontend-modern/src/utils/dashboardRecoveryPresentation.ts"
        result = lookup_paths([path])
        self.assertEqual(result["unowned_runtime_files"], [path])
        self.assertEqual(result["impacted_subsystems"], [])
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(file_entry["matches"], [])

    def test_lookup_paths_reports_retired_dashboard_storage_presentation_as_unowned(self) -> None:
        path = "frontend-modern/src/utils/dashboardStoragePresentation.ts"
        result = lookup_paths([path])
        self.assertEqual(result["unowned_runtime_files"], [path])
        self.assertEqual(result["impacted_subsystems"], [])
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(file_entry["matches"], [])

    def test_lookup_paths_assigns_storage_surface_to_storage_recovery(self) -> None:
        # The legacy /storage route shell (pages/Storage.tsx) was retired
        # with the platform-first migration; storage is now exercised via
        # the embedded surface at components/Storage/Storage.tsx.
        result = lookup_paths(["frontend-modern/src/components/Storage/Storage.tsx"])
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
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L15")
        self.assertEqual(match["verification_requirement"]["id"], "storage-product-surface")

    def test_lookup_paths_reports_retired_recovery_summary_presentation_as_unowned(self) -> None:
        path = "frontend-modern/src/utils/recoverySummaryPresentation.ts"
        result = lookup_paths([path])
        self.assertEqual(result["unowned_runtime_files"], [path])
        self.assertEqual(result["impacted_subsystems"], [])
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(file_entry["matches"], [])

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
            [
                "frontend-modern/src/components/Settings/__tests__/APITokenManager.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
            ],
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

    def test_lookup_paths_reports_assistant_chat_frontend_client_as_shared_boundary(self) -> None:
        result = lookup_paths(["frontend-modern/src/api/aiChat.ts"])
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
                "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
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
                "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
                "frontend-modern/src/utils/__tests__/agentProfilesPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_connections_table_model_to_agent_lifecycle(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/connectionsTableModel.ts"]
        )
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
            "systems-ledger-workspace-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            CONNECTIONS_LEDGER_WORKSPACE_EXACT_FILES,
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

    def test_lookup_paths_assigns_internal_ai_config_to_ai_runtime(self) -> None:
        result = lookup_paths(["internal/config/ai.go"])
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
        self.assertEqual(match["verification_requirement"]["id"], "ai-runtime-config")
        self.assertEqual(match["verification_requirement"]["exact_files"], ["internal/config/ai_config_test.go"])

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

    def test_lookup_paths_reports_pulse_mcp_adapter_as_shared_boundary(self) -> None:
        result = lookup_paths(["cmd/pulse-mcp", "cmd/pulse-mcp/session_policy.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"ai-runtime", "api-contracts"},
        )

        directory_entry = result["files"][0]
        self.assertEqual(directory_entry["classification"], "runtime")
        self.assertIsNone(directory_entry["shared_ownership"])

        expected = {
            "ai-runtime": ("L6", "pulse-mcp-adapter"),
            "api-contracts": ("L6", "pulse-mcp-manifest-adapter"),
        }
        for file_entry in result["files"]:
            self.assertEqual(file_entry["classification"], "runtime")
            self.assertEqual(
                {match["subsystem"] for match in file_entry["matches"]},
                {"ai-runtime", "api-contracts"},
            )
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
            [
                "internal/relay/client_test.go",
                "internal/relay/config_env_test.go",
                "internal/relay/encryption_test.go",
                "internal/relay/protocol_test.go",
                "internal/relay/push_test.go",
            ],
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

    def test_lookup_paths_assigns_workloads_guest_row_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/GuestRow.tsx",
                "frontend-modern/src/components/Workloads/GuestRowCells.tsx",
                "frontend-modern/src/components/Workloads/guestRowModel.tsx",
                "frontend-modern/src/components/Workloads/useGuestRowState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workloads_guest_metadata_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/useWorkloadGuestMetadataState.ts"]
        )
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workload_route_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/workloadFilterConfigModel.ts",
                "frontend-modern/src/components/Workloads/workloadRouteModel.ts",
                "frontend-modern/src/components/Workloads/workloadRouteStateModel.ts",
                "frontend-modern/src/components/Workloads/workloadUrlSyncModel.ts",
                "frontend-modern/src/components/Workloads/useWorkloadFilterOptions.ts",
                "frontend-modern/src/components/Workloads/useWorkloadRouteState.ts",
                "frontend-modern/src/components/Workloads/useWorkloadUrlSync.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workloads_selection_model_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/workloadSelectionModel.ts",
                "frontend-modern/src/components/Workloads/useWorkloadSelectionState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workload_table_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/WorkloadsTable.tsx"]
        )
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workload_panel_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(["frontend-modern/src/components/Workloads/WorkloadPanel.tsx"])
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workload_header_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/WorkloadTableHeader.tsx"]
        )
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workloads_selection_state_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/useWorkloadSelectionState.ts"]
        )
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workload_url_sync_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/useWorkloadUrlSync.ts"]
        )
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workload_derived_state_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/useWorkloadsDerivedState.ts"]
        )
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workload_viewport_sync_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/useWorkloadViewportSync.ts"]
        )
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workloads_controls_state_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/useWorkloadsControlsState.ts"]
        )
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
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workloads_grouped_windowing_runtime_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Workloads/useGroupedTableWindowing.ts"]
        )
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
        self.assertEqual(match["lane_context"]["lane_id"], "L10")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "workloads-hot-path",
        )
        self.assertIn(
            "frontend-modern/src/components/Workloads/__tests__/useGroupedTableWindowing.test.ts",
            match["verification_requirement"]["exact_files"],
        )

    def test_lookup_paths_assigns_workloads_guest_drawer_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/GuestDrawer.tsx",
                "frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx",
                "frontend-modern/src/components/Workloads/guestDrawerModel.ts",
                "frontend-modern/src/components/Workloads/useGuestDrawerState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")

    def test_lookup_paths_assigns_workload_topology_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/workloadTopology.ts",
                "frontend-modern/src/components/Workloads/useGuestDrawerState.ts",
                "frontend-modern/src/components/Workloads/useGuestRowState.ts",
                "frontend-modern/src/components/Workloads/useWorkloadRouteState.ts",
                "frontend-modern/src/components/Workloads/useWorkloadsDerivedState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workloads_disk_list_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/DiskList.tsx",
                "frontend-modern/src/components/Workloads/diskListModel.ts",
                "frontend-modern/src/components/Workloads/useDiskListState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workloads_filter_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/WorkloadsFilter.tsx",
                "frontend-modern/src/components/Workloads/workloadsFilterModel.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workloads_threshold_slider_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/ThresholdSlider.tsx",
                "frontend-modern/src/components/Workloads/thresholdSliderModel.ts",
                "frontend-modern/src/components/Workloads/useThresholdSliderState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_threshold_slider_presentation_to_performance_and_scalability(
        self,
    ) -> None:
        result = lookup_paths(["frontend-modern/src/utils/thresholdSliderPresentation.ts"])
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
        self.assertEqual(match["lane_context"]["lane_id"], "L10")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "workloads-hot-path",
        )

    def test_lookup_paths_assigns_workloads_stacked_disk_bar_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/StackedDiskBar.tsx",
                "frontend-modern/src/components/Workloads/stackedDiskBarModel.ts",
                "frontend-modern/src/components/Workloads/useStackedDiskBarState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workloads_stacked_memory_bar_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/StackedMemoryBar.tsx",
                "frontend-modern/src/components/Workloads/stackedMemoryBarModel.ts",
                "frontend-modern/src/components/Workloads/useStackedMemoryBarState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workloads_metric_bar_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/MetricBar.tsx",
                "frontend-modern/src/components/Workloads/metricBarModel.ts",
                "frontend-modern/src/components/Workloads/useMetricBarState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
            )

    def test_lookup_paths_assigns_workloads_enhanced_cpu_bar_runtime_to_performance_and_scalability(self) -> None:
        result = lookup_paths(
            [
                "frontend-modern/src/components/Workloads/EnhancedCPUBar.tsx",
                "frontend-modern/src/components/Workloads/enhancedCpuBarModel.ts",
                "frontend-modern/src/components/Workloads/useEnhancedCPUBarState.ts",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"performance-and-scalability"},
        )
        for file_entry in result["files"]:
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
            self.assertEqual(match["lane_context"]["lane_id"], "L10")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "workloads-hot-path",
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
            [
                "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
            ],
        )

    def test_lookup_paths_assigns_settings_diagnostics_boundary_audit_to_frontend_primitives(
        self,
    ) -> None:
        result = lookup_paths(["frontend-modern/scripts/settings-diagnostics-boundary-audit.mjs"])
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
            "settings-diagnostics-boundary-audit",
        )

    def test_lookup_paths_assigns_settings_dialogs_to_frontend_primitives(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/SettingsDialogs.tsx"])
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
            [
                "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
            ],
        )

    def test_lookup_paths_assigns_settings_shell_state_to_frontend_primitives(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/useSettingsShellState.ts"])
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
            [
                "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
            ],
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
            [
                "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
            ],
        )

    def test_lookup_paths_assigns_network_settings_model_to_frontend_primitives(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/networkSettingsModel.ts"])
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
            [
                "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
            ],
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
            [
                "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
            ],
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

    def _assert_environment_lock_lookup(
        self, path: str, requirement_id: str, exact_files: list[str]
    ) -> None:
        result = lookup_paths([path])
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
        self.assertEqual(match["verification_requirement"]["id"], requirement_id)
        self.assertEqual(match["verification_requirement"]["exact_files"], exact_files)

    def test_lookup_paths_assigns_docker_runtime_settings_card_to_frontend_primitives(
        self,
    ) -> None:
        self._assert_environment_lock_lookup(
            "frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx",
            "settings-shell-and-framing",
            [
                "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
            ],
        )

    def test_lookup_paths_assigns_environment_lock_badge_to_frontend_primitives(self) -> None:
        self._assert_environment_lock_lookup(
            "frontend-modern/src/components/shared/EnvironmentLockBadge.tsx",
            "environment-lock-primitives",
            [
                "frontend-modern/src/utils/__tests__/environmentLockPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_environment_lock_presentation_to_frontend_primitives(
        self,
    ) -> None:
        self._assert_environment_lock_lookup(
            "frontend-modern/src/utils/environmentLockPresentation.ts",
            "environment-lock-primitives",
            [
                "frontend-modern/src/utils/__tests__/environmentLockPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_lookup_paths_assigns_remaining_settings_shell_panels_to_frontend_primitives(self) -> None:
        runtime_files = [
            "frontend-modern/src/components/Settings/APIAccessPanel.tsx",
            "frontend-modern/src/components/Settings/AIChatMaintenanceSection.tsx",
            "frontend-modern/src/components/Settings/AIModelSelectionSection.tsx",
            "frontend-modern/src/components/Settings/AIRuntimeControlsSection.tsx",
            "frontend-modern/src/components/Settings/AISettings.tsx",
            "frontend-modern/src/components/Settings/AISettingsStatusAndActions.tsx",
            "frontend-modern/src/components/Settings/AuditLogPanel.tsx",
            "frontend-modern/src/components/Settings/AuditWebhookPanel.tsx",
            "frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx",
            "frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx",
            "frontend-modern/src/components/Settings/SecurityOverviewPanel.tsx",
            "frontend-modern/src/components/Settings/SSOProvidersPanel.tsx",
            "frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx",
            "frontend-modern/src/components/Settings/settingsNavigationModel.ts",
            "frontend-modern/src/components/Settings/settingsPanelRegistryContext.tsx",
            "frontend-modern/src/components/Settings/settingsPanelRegistryLoaders.ts",
            "frontend-modern/src/components/Settings/settingsNavCatalog.ts",
            "frontend-modern/src/components/Settings/settingsNavVisibility.ts",
            "frontend-modern/src/components/Settings/settingsTabSaveBehavior.ts",
            "frontend-modern/src/components/Settings/useSettingsNavigation.ts",
            "frontend-modern/src/components/Settings/useDiscoverySettingsState.ts",
            "frontend-modern/src/components/Settings/useSettingsInfrastructurePanelProps.ts",
            "frontend-modern/src/components/Settings/useSettingsPanelRegistry.tsx",
            "frontend-modern/src/components/Settings/useSettingsSystemPanels.tsx",
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
                "frontend-modern/src/components/Settings/APIAccessPanel.tsx",
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
                [
                    "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                    "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                ],
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
                "frontend-modern/src/components/Settings/__tests__/OrganizationAccessPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/OrganizationOverviewPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/RBACPaywallPanels.test.tsx",
                "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                "frontend-modern/src/utils/__tests__/orgUtils.test.ts",
                "frontend-modern/src/utils/__tests__/organizationRolePresentation.test.ts",
                "frontend-modern/src/utils/__tests__/organizationSettingsPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_organization_sharing_state_to_organization_settings(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useOrganizationSharingPanelState.ts"]
        )
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

    def test_lookup_paths_assigns_organization_access_state_to_organization_settings(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useOrganizationAccessPanelState.ts"]
        )
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

    def test_lookup_paths_assigns_organization_access_members_section_to_organization_settings(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/OrganizationAccessMembersSection.tsx"]
        )
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

    def test_lookup_paths_assigns_organization_overview_state_to_organization_settings(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useOrganizationOverviewPanelState.ts"]
        )
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

    def test_lookup_paths_assigns_organization_overview_members_section_to_organization_settings(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/OrganizationOverviewMembersSection.tsx"]
        )
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

    def test_lookup_paths_assigns_organization_outgoing_shares_section_to_organization_settings(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/OrganizationOutgoingSharesSection.tsx"]
        )
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
                "frontend-modern/src/components/Settings/__tests__/OrganizationAccessPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/OrganizationOverviewPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/RBACPaywallPanels.test.tsx",
                "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
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
                "frontend-modern/src/components/Settings/__tests__/OrganizationAccessPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/OrganizationOverviewPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/RBACPaywallPanels.test.tsx",
                "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                "frontend-modern/src/utils/__tests__/orgUtils.test.ts",
                "frontend-modern/src/utils/__tests__/organizationRolePresentation.test.ts",
                "frontend-modern/src/utils/__tests__/organizationSettingsPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_roles_panel_state_to_organization_settings(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/useRolesPanelState.ts"])
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

    def test_lookup_paths_assigns_user_assignments_dialog_to_organization_settings(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/UserAssignmentsDialog.tsx"])
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

    def test_lookup_paths_assigns_orgs_api_to_shared_organization_settings_and_api_contracts(
        self,
    ) -> None:
        result = lookup_paths(["frontend-modern/src/api/orgs.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "organization-settings"},
        )

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "organization-settings"],
        )

        by_subsystem = {match["subsystem"]: match for match in file_entry["matches"]}
        self.assertEqual(set(by_subsystem), {"api-contracts", "organization-settings"})

        api_match = by_subsystem["api-contracts"]
        self.assertEqual(
            api_match["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_match["verification_requirement"]["id"],
            "frontend-api-clients",
        )
        self.assertEqual(
            api_match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/types/api.ts"],
        )

        organization_match = by_subsystem["organization-settings"]
        self.assertEqual(
            organization_match["contract"],
            "docs/release-control/v6/internal/subsystems/organization-settings.md",
        )
        self.assertEqual(organization_match["lane_context"]["lane_id"], "L14")
        self.assertEqual(
            organization_match["verification_requirement"]["id"],
            "organization-api-clients",
        )
        self.assertEqual(
            organization_match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/orgs.test.ts",
                "frontend-modern/src/api/__tests__/rbac.test.ts",
            ],
        )
        self.assertEqual(
            organization_match["matched_contract_references"],
            [
                {
                    "heading": "## Canonical Files",
                    "path": "frontend-modern/src/api/orgs.ts",
                    "line": 24,
                    "heading_line": 22,
                },
                {
                    "heading": "## Shared Boundaries",
                    "path": "frontend-modern/src/api/orgs.ts",
                    "line": 63,
                    "heading_line": 61,
                },
                {
                    "heading": "## Extension Points",
                    "path": "frontend-modern/src/api/orgs.ts",
                    "line": 85,
                    "heading_line": 70,
                },
            ],
        )

    def test_lookup_paths_assigns_rbac_backend_to_shared_organization_settings_and_api_contracts(
        self,
    ) -> None:
        result = lookup_paths(["internal/api/access_control_handlers.go"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "organization-settings"},
        )

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "organization-settings"],
        )

        by_subsystem = {match["subsystem"]: match for match in file_entry["matches"]}
        self.assertEqual(set(by_subsystem), {"api-contracts", "organization-settings"})

        api_match = by_subsystem["api-contracts"]
        self.assertEqual(
            api_match["verification_requirement"]["id"],
            "backend-payload-contracts",
        )
        self.assertEqual(
            api_match["matched_contract_references"],
            [
				{
					"heading": "## Shared Boundaries",
					"path": "internal/api/access_control_handlers.go",
					"line": 1204,
					"heading_line": 142,
				}
            ],
        )

        organization_match = by_subsystem["organization-settings"]
        self.assertEqual(
            organization_match["verification_requirement"]["id"],
            "organization-rbac-transport",
        )
        self.assertEqual(
            organization_match["verification_requirement"]["exact_files"],
            [
                "internal/api/enterprise_extension_rbac_admin_test.go",
                "internal/api/rbac_admin_handlers_test.go",
                "internal/api/rbac_handlers_additional_test.go",
                "internal/api/rbac_handlers_more_test.go",
                "internal/api/rbac_handlers_test.go",
            ],
        )
        self.assertEqual(
            organization_match["matched_contract_references"],
            [
                {
                    "heading": "## Canonical Files",
                    "path": "internal/api/access_control_handlers.go",
                    "line": 55,
                    "heading_line": 22,
                },
                {
                    "heading": "## Shared Boundaries",
                    "path": "internal/api/access_control_handlers.go",
                    "line": 65,
                    "heading_line": 61,
                },
                {
                    "heading": "## Extension Points",
                    "path": "internal/api/access_control_handlers.go",
                    "line": 99,
                    "heading_line": 70,
                },
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
            PATROL_PAGE_AND_STATE_EXACT_FILES,
        )

    def test_lookup_paths_assigns_patrol_surface_to_patrol_intelligence(self) -> None:
        result = lookup_paths(["frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx"])
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
            PATROL_PAGE_AND_STATE_EXACT_FILES,
        )

    def test_lookup_paths_assigns_patrol_control_presentation_to_patrol_intelligence(
        self,
    ) -> None:
        result = lookup_paths(["frontend-modern/src/features/patrol/patrolControlPresentation.ts"])
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
            PATROL_PAGE_AND_STATE_EXACT_FILES,
        )

    def test_lookup_paths_assigns_patrol_state_hook_to_patrol_intelligence(self) -> None:
        result = lookup_paths(["frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts"])
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
            PATROL_PAGE_AND_STATE_EXACT_FILES,
        )

    def test_lookup_paths_assigns_patrol_investigation_context_model_to_patrol_intelligence(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts"]
        )
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
            match["verification_requirement"]["id"],
            "patrol-page-and-state",
        )

    def test_lookup_paths_assigns_ai_intelligence_summary_model_to_patrol_intelligence(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/stores/aiIntelligenceSummaryModel.ts"]
        )
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
                "frontend-modern/src/utils/__tests__/approvalState.test.ts",
                "frontend-modern/src/utils/__tests__/findingAlertIdentity.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                "frontend-modern/src/utils/__tests__/remediationPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_remediation_presentation_to_patrol_intelligence(self) -> None:
        result = lookup_paths(["frontend-modern/src/utils/remediationPresentation.ts"])
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
                "frontend-modern/src/utils/__tests__/approvalState.test.ts",
                "frontend-modern/src/utils/__tests__/findingAlertIdentity.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                "frontend-modern/src/utils/__tests__/remediationPresentation.test.ts",
            ],
        )

    def test_lookup_paths_assigns_infrastructure_install_state_to_shared_agent_lifecycle_and_api_contracts(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx"]
        )
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
            api_match["verification_requirement"]["id"],
            "unified-agent-settings-surface",
        )
        self.assertEqual(
            api_match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                "frontend-modern/src/api/__tests__/monitoring.test.ts",
                "frontend-modern/src/api/__tests__/security.test.ts",
                "frontend-modern/src/components/Settings/__tests__/InfrastructureOperationsModel.test.tsx",
            ],
        )
        self.assertEqual(
            lifecycle_match["verification_requirement"]["id"],
            "unified-agent-settings-surface",
        )
        self.assertEqual(
            lifecycle_match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                "frontend-modern/src/api/__tests__/monitoring.test.ts",
                "frontend-modern/src/components/Settings/__tests__/InfrastructureOperationsModel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
            ],
        )

    def test_lookup_paths_assigns_infrastructure_configured_nodes_state_to_shared_agent_lifecycle_and_api_contracts(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts"]
        )
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
            api_match["verification_requirement"]["id"],
            "unified-agent-settings-surface",
        )
        self.assertEqual(
            lifecycle_match["verification_requirement"]["id"],
            "platform-connections-workspace-surface",
        )

    def test_lookup_paths_assigns_infrastructure_discovery_runtime_state_to_shared_agent_lifecycle_and_api_contracts(
        self,
    ) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts"]
        )
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
            api_match["verification_requirement"]["id"],
            "unified-agent-settings-surface",
        )
        self.assertEqual(
            lifecycle_match["verification_requirement"]["id"],
            "platform-connections-workspace-surface",
        )

    def test_lookup_paths_assigns_infrastructure_installer_section_to_agent_lifecycle(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"agent-lifecycle"},
        )

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertIsNone(file_entry["shared_ownership"])
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "agent-lifecycle")
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L16")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "unified-agent-settings-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                "frontend-modern/src/api/__tests__/monitoring.test.ts",
                "frontend-modern/src/components/Settings/__tests__/InfrastructureOperationsModel.test.tsx",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
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
                "scripts/installtests/install_sh_test.go",
                "scripts/installtests/pulse_auto_update_test.go",
                "scripts/installtests/root_install_sh_test.go",
            ],
        )

    def test_lookup_paths_assigns_mcp_installers_to_deployment_installability(self) -> None:
        result = lookup_paths(["scripts/install-mcp.sh", "scripts/install-mcp.ps1"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
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
                "pulse-mcp-installer-release-runtime",
            )
            self.assertEqual(
                match["verification_requirement"]["exact_files"],
                ["scripts/installtests/build_release_assets_test.go"],
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
                "scripts/installtests/install_sh_test.go",
                "scripts/installtests/pulse_auto_update_test.go",
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
                "scripts/installtests/install_sh_test.go",
                "scripts/installtests/pulse_auto_update_test.go",
                "scripts/installtests/root_install_sh_test.go",
            ],
        )

    def test_lookup_paths_assigns_control_plane_rollout_command_to_deployment_installability(self) -> None:
        result = lookup_paths(["cmd/pulse-control-plane/main.go"])
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
            "hosted-runtime-rollout-control",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "cmd/pulse-control-plane/provider_msp_backup_test.go",
                "cmd/pulse-control-plane/provider_msp_install_proof_test.go",
                "cmd/pulse-control-plane/provider_msp_preflight_test.go",
                "cmd/pulse-control-plane/provider_msp_proof_test.go",
                "cmd/pulse-control-plane/provider_msp_recover_test.go",
                "cmd/pulse-control-plane/provider_msp_status_test.go",
                "internal/cloudcp/docker/manager_test.go",
                "internal/cloudcp/provider_msp_backup_test.go",
                "internal/cloudcp/provider_msp_recovery_test.go",
                "internal/cloudcp/tenant_runtime_rollout_test.go",
                "scripts/installtests/provider_msp_deploy_test.go",
            ],
        )

    def test_lookup_paths_assigns_release_notes_index_to_deployment_installability(self) -> None:
        result = lookup_paths(["docs/RELEASE_NOTES.md"])
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
            "release-promotion-metadata-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/build_release_assets_test.go",
                "scripts/release_control/internal/record_rc_to_ga_rehearsal_test.py",
                "scripts/release_control/mobile_release_gate_test.py",
                "scripts/release_control/release_promotion_policy_support_test.py",
                "scripts/release_control/release_promotion_policy_test.py",
                "scripts/release_control/render_release_body_test.py",
                "scripts/release_control/resolve_release_promotion_test.py",
                "scripts/release_control/validate_artifact_release_line_test.py",
            ],
        )

    def test_lookup_paths_assigns_rc_release_packet_docs_to_deployment_installability(self) -> None:
        result = lookup_paths(["docs/releases/V6_CHANGELOG_RC2_DRAFT.md"])
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
            "release-promotion-metadata-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/build_release_assets_test.go",
                "scripts/release_control/internal/record_rc_to_ga_rehearsal_test.py",
                "scripts/release_control/mobile_release_gate_test.py",
                "scripts/release_control/release_promotion_policy_support_test.py",
                "scripts/release_control/release_promotion_policy_test.py",
                "scripts/release_control/render_release_body_test.py",
                "scripts/release_control/resolve_release_promotion_test.py",
                "scripts/release_control/validate_artifact_release_line_test.py",
            ],
        )

    def test_lookup_paths_assigns_stable_and_historical_release_packet_docs_to_deployment_installability(self) -> None:
        result = lookup_paths(
            [
                "docs/releases/RELEASE_NOTES_v6.md",
                "docs/releases/RELEASE_NOTES_v6_RC1.md",
                "docs/releases/V6_CHANGELOG.md",
                "docs/releases/V6_CHANGELOG_RC1.md",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
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
            self.assertEqual(
                match["verification_requirement"]["exact_files"],
                [
                    "scripts/installtests/build_release_assets_test.go",
                    "scripts/release_control/internal/record_rc_to_ga_rehearsal_test.py",
                    "scripts/release_control/mobile_release_gate_test.py",
                    "scripts/release_control/release_promotion_policy_support_test.py",
                    "scripts/release_control/release_promotion_policy_test.py",
                    "scripts/release_control/render_release_body_test.py",
                    "scripts/release_control/resolve_release_promotion_test.py",
                    "scripts/release_control/validate_artifact_release_line_test.py",
                ],
            )

    def test_lookup_paths_assigns_upgrade_guide_to_deployment_installability(self) -> None:
        result = lookup_paths(["docs/UPGRADE_v6.md"])
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
            "release-promotion-metadata-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/build_release_assets_test.go",
                "scripts/release_control/internal/record_rc_to_ga_rehearsal_test.py",
                "scripts/release_control/mobile_release_gate_test.py",
                "scripts/release_control/release_promotion_policy_support_test.py",
                "scripts/release_control/release_promotion_policy_test.py",
                "scripts/release_control/render_release_body_test.py",
                "scripts/release_control/resolve_release_promotion_test.py",
                "scripts/release_control/validate_artifact_release_line_test.py",
            ],
        )

    def test_lookup_paths_assigns_v6_feedback_template_to_deployment_installability(self) -> None:
        result = lookup_paths([".github/ISSUE_TEMPLATE/v6_rc_feedback.yml"])
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
            "release-promotion-metadata-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/build_release_assets_test.go",
                "scripts/release_control/internal/record_rc_to_ga_rehearsal_test.py",
                "scripts/release_control/mobile_release_gate_test.py",
                "scripts/release_control/release_promotion_policy_support_test.py",
                "scripts/release_control/release_promotion_policy_test.py",
                "scripts/release_control/render_release_body_test.py",
                "scripts/release_control/resolve_release_promotion_test.py",
                "scripts/release_control/validate_artifact_release_line_test.py",
            ],
        )

    def test_lookup_paths_assigns_version_file_to_deployment_installability(self) -> None:
        result = lookup_paths(["VERSION"])
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
            "release-promotion-metadata-runtime",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "scripts/installtests/build_release_assets_test.go",
                "scripts/release_control/internal/record_rc_to_ga_rehearsal_test.py",
                "scripts/release_control/mobile_release_gate_test.py",
                "scripts/release_control/release_promotion_policy_support_test.py",
                "scripts/release_control/release_promotion_policy_test.py",
                "scripts/release_control/render_release_body_test.py",
                "scripts/release_control/resolve_release_promotion_test.py",
                "scripts/release_control/validate_artifact_release_line_test.py",
            ],
        )

    def test_lookup_paths_assigns_helm_chart_to_deployment_installability(self) -> None:
        result = lookup_paths(
            [
                "deploy/helm/pulse/Chart.yaml",
                "deploy/helm/pulse/README.md",
                "deploy/helm/pulse/templates/deployment.yaml",
                "deploy/helm/pulse/values.yaml",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
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
                "helm-chart-release-runtime",
            )
            self.assertEqual(
                match["verification_requirement"]["exact_files"],
                [
                    "scripts/installtests/build_release_assets_test.go",
                    "scripts/release_control/subsystem_lookup_test.py",
                ],
            )

    def test_lookup_paths_assigns_secure_storage_hardening_to_security_privacy(self) -> None:
        result = lookup_paths(
            [
                "internal/cloudcp/auth/magiclink.go",
                "internal/cloudcp/auth/magiclink_store.go",
                "internal/crypto/crypto.go",
                "internal/securityutil/secure_storage_dir.go",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"cloud-paid", "security-privacy"},
        )
        expected_subsystems_by_path = {
            "internal/cloudcp/auth/magiclink.go": {"cloud-paid", "security-privacy"},
            "internal/cloudcp/auth/magiclink_store.go": {"cloud-paid", "security-privacy"},
            "internal/crypto/crypto.go": {"security-privacy"},
            "internal/securityutil/secure_storage_dir.go": {"security-privacy"},
        }
        for file_entry in result["files"]:
            self.assertEqual(file_entry["classification"], "runtime")
            self.assertEqual(
                {match["subsystem"] for match in file_entry["matches"]},
                expected_subsystems_by_path[file_entry["path"]],
            )
            match = next(
                match
                for match in file_entry["matches"]
                if match["subsystem"] == "security-privacy"
            )
            self.assertEqual(
                match["contract"],
                "docs/release-control/v6/internal/subsystems/security-privacy.md",
            )
            self.assertEqual(match["lane_context"]["lane_id"], "L14")
            self.assertEqual(
                match["verification_requirement"]["id"],
                "storage-directory-security",
            )
            self.assertEqual(
                match["verification_requirement"]["exact_files"],
                [
                    "internal/cloudcp/auth/magiclink_test.go",
                    "internal/crypto/crypto_test.go",
                    "internal/securityutil/secure_storage_dir_test.go",
                ],
            )

    def test_lookup_paths_assigns_mock_runtime_fixture_authority_to_monitoring(self) -> None:
        result = lookup_paths(
            [
                "internal/mock/demo_scenarios.go",
                "internal/mock/fixture_graph.go",
                "internal/mock/platform_fixtures.go",
            ]
        )
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"monitoring"},
        )
        for file_entry in result["files"]:
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
                "mock-runtime-fixtures",
            )
            self.assertEqual(
                match["verification_requirement"]["exact_files"],
                [
                    "internal/mock/canonical_api_guardrails_test.go",
                    "internal/mock/demo_scenarios_test.go",
                    "internal/mock/generator_test.go",
                    "internal/mock/platform_fixtures_test.go",
                    "tests/integration/tests/43-platform-mock-runtime.spec.ts",
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
        self.assertEqual({decision["id"] for decision in lane_context["open_decisions"]}, set())
        self.assertEqual(
            {gate["id"] for gate in lane_context["release_gates"]},
            {
                "cloud-hosted-tier-runtime-readiness",
                "commercial-cancellation-reactivation",
                "hosted-signup-billing-replay",
                "msp-provider-tenant-management",
                "paid-feature-entitlement-gating",
                "paid-runtime-build-attribution-alerting",
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
                "hosted-tenant-runtime-hibernation",
                "legacy-grandfathering-eligibility-cutoff",
                "msp-buying-motion-lock",
                "msp-design-partner-growth-offer",
                "msp-provider-hosted-first-launch",
                "msp-provider-tier-limit-lock",
                "pre-ga-public-checkout-posture",
                "self-hosted-plans-surface-entitlement-first",
                "self-hosted-paid-surface-classification",
                "self-hosted-core-monitoring-free",
                "self-hosted-paid-extras-packaging",
                "stable-release-promotion-model",
                "stripe-mapping-contract-lock",
                "trial-authority-saas-controlled",
                "v5-pro-price-grandfathering",
            },
        )

    def test_lookup_paths_assigns_billing_admin_state_owner_to_cloud_paid(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useBillingAdminPanelState.ts"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "cloud-paid")
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/cloud-paid.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L3")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "hosted-billing-admin-surface",
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
            {
                "agent-ready-operations-api-cli-contract",
                "canonical-timeline-source-precedence",
                "host-type-migration-boundary-audit",
                "mixed-private-infrastructure-operations-direction",
                "platform-admission-first-lab-ready-stage",
                "platform-page-membership-overlap-v1",
                "platform-support-model-v1",
                "vmware-vsphere-vcenter-first-admission-model",
                "vmware-vsphere-vcenter-support-claim-ratchet",
            },
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

    def test_lookup_paths_assigns_host_agent_ingest_runtime_to_monitoring(self) -> None:
        result = lookup_paths(["internal/monitoring/monitor_agents.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "monitoring")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/monitoring.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(match["verification_requirement"]["id"], "host-agent-ingest-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/monitoring/monitor_host_agents_test.go",
                "internal/monitoring/monitor_package_updates_test.go",
                "internal/unifiedresources/code_standards_test.go",
            ],
        )

    def test_lookup_paths_assigns_docker_swarm_runtime_to_monitoring(self) -> None:
        result = lookup_paths(["internal/dockeragent/swarm.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "monitoring")
        self.assertEqual(match["contract"], "docs/release-control/v6/internal/subsystems/monitoring.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(match["verification_requirement"]["id"], "docker-swarm-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/dockeragent/swarm_coverage_test.go",
                "internal/dockeragent/swarm_test.go",
            ],
        )

    def test_lookup_paths_assigns_system_logs_runtime_owner_to_frontend_primitives(self) -> None:
        result = lookup_paths(
            ["frontend-modern/src/components/Settings/useSystemLogsPanelState.ts"]
        )
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "frontend-primitives")
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/frontend-primitives.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L8")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "route-shell-and-operations",
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
                "internal/unifiedresources/resource_operator_state_policy_test.go",
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
                "internal/unifiedresources/resource_operator_state_policy_test.go",
                "internal/unifiedresources/snapshot_source_filter_test.go",
                "internal/unifiedresources/store_test.go",
            ],
        )

    def test_lookup_paths_assigns_discovery_tab_state_owner_to_unified_resources(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Discovery/useDiscoveryTabState.ts"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "unified-resources")
        self.assertEqual(
            match["contract"],
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L13")
        self.assertEqual(match["verification_requirement"]["id"], "resource-consumers")

    def test_lookup_paths_reports_dependent_contract_updates_for_shared_canonical_file(self) -> None:
        result = lookup_paths(["internal/unifiedresources/views.go"])
        file_entry = result["files"][0]
        self.assertEqual(
            {contract["subsystem"] for contract in file_entry["dependent_contract_updates"]},
            {"monitoring"},
        )
        monitoring_contract = file_entry["dependent_contract_updates"][0]
        self.assertEqual(
            monitoring_contract["matched_reference_details"],
            [
                _contract_reference(
                    "docs/release-control/v6/internal/subsystems/monitoring.md",
                    "12. `internal/unifiedresources/views.go`",
                    "internal/unifiedresources/views.go",
                ),
                _contract_reference(
                    "docs/release-control/v6/internal/subsystems/monitoring.md",
                    "3. Add typed read access through `internal/unifiedresources/views.go`",
                    "internal/unifiedresources/views.go",
                ),
            ],
        )
        self.assertEqual(
            {contract["subsystem"] for contract in result["required_contract_updates"]},
            {"monitoring", "unified-resources"},
        )

    def test_lookup_paths_reports_contract_focus_for_shared_api_runtime_file(self) -> None:
        result = lookup_paths(["internal/api/ai_handler.go"])
        file_entry = result["files"][0]
        by_subsystem = {match["subsystem"]: match for match in file_entry["matches"]}
        ai_runtime_expected = [
            _contract_reference(
                "docs/release-control/v6/internal/subsystems/ai-runtime.md",
                "3. `internal/api/ai_handler.go`",
                "internal/api/ai_handler.go",
            ),
            _contract_reference(
                "docs/release-control/v6/internal/subsystems/ai-runtime.md",
                "36. `internal/api/ai_handler.go` shared with `api-contracts`",
                "internal/api/ai_handler.go",
            ),
            _contract_reference(
                "docs/release-control/v6/internal/subsystems/ai-runtime.md",
                "3. Add or change Pulse Assistant request flow through `internal/api/ai_handler.go`",
                "internal/api/ai_handler.go",
            ),
        ]
        api_contracts_expected = [
            _contract_reference(
                "docs/release-control/v6/internal/subsystems/api-contracts.md",
                "64. `internal/api/ai_handler.go` shared with `ai-runtime`",
                "internal/api/ai_handler.go",
            ),
            _contract_reference(
                "docs/release-control/v6/internal/subsystems/api-contracts.md",
                "23. Keep hosted AI settings bootstrap on the shared API contract",
                "internal/api/ai_handler.go",
            ),
            _contract_reference(
                "docs/release-control/v6/internal/subsystems/api-contracts.md",
                "24. Keep post-boot AI enablement contract-backed on the shared AI/mobile approval surface",
                "internal/api/ai_handler.go",
            ),
        ]

        self.assertEqual(
            by_subsystem["ai-runtime"]["matched_contract_references"],
            ai_runtime_expected,
        )
        self.assertEqual(
            by_subsystem["api-contracts"]["matched_contract_references"],
            api_contracts_expected,
        )
        self.assertEqual(
            by_subsystem["api-contracts"]["ownership_basis"],
            {"type": "owned-prefix", "value": "internal/api/"},
        )

    def test_lookup_paths_lean_mode_trims_status_payload_but_keeps_contract_focus(self) -> None:
        result = lookup_paths(["internal/api/ai_handler.go"], lean=True)
        self.assertNotIn("scope", result)
        self.assertNotIn("status_summary", result)

        file_entry = result["files"][0]
        by_subsystem = {match["subsystem"]: match for match in file_entry["matches"]}
        api_match = by_subsystem["api-contracts"]
        self.assertEqual(api_match["lane_context"]["lane_id"], "L6")
        self.assertEqual(api_match["lane_context"]["open_decision_ids"], [])
        self.assertIn("paid-feature-entitlement-gating", api_match["lane_context"]["release_gate_ids"])
        self.assertEqual(
            [reference["line"] for reference in api_match["matched_contract_references"]],
            [
                _contract_reference(
                    "docs/release-control/v6/internal/subsystems/api-contracts.md",
                    "64. `internal/api/ai_handler.go` shared with `ai-runtime`",
                    "internal/api/ai_handler.go",
                )["line"],
                _contract_reference(
                    "docs/release-control/v6/internal/subsystems/api-contracts.md",
                    "23. Keep hosted AI settings bootstrap on the shared API contract",
                    "internal/api/ai_handler.go",
                )["line"],
                _contract_reference(
                    "docs/release-control/v6/internal/subsystems/api-contracts.md",
                    "24. Keep post-boot AI enablement contract-backed on the shared AI/mobile approval surface",
                    "internal/api/ai_handler.go",
                )["line"],
            ],
        )

    def test_render_pretty_shows_contract_focus_for_lean_lookup(self) -> None:
        rendered = render_pretty(lookup_paths(["internal/api/ai_handler.go"], lean=True))
        self.assertIn(
            "contract focus: "
            f"{_contract_reference('docs/release-control/v6/internal/subsystems/api-contracts.md', '64. `internal/api/ai_handler.go` shared with `ai-runtime`', 'internal/api/ai_handler.go')['heading']} "
            f"@L{_contract_reference('docs/release-control/v6/internal/subsystems/api-contracts.md', '64. `internal/api/ai_handler.go` shared with `ai-runtime`', 'internal/api/ai_handler.go')['line']}: "
            "internal/api/ai_handler.go",
            rendered,
        )
        self.assertIn(
            "contract focus: "
            f"{_contract_reference('docs/release-control/v6/internal/subsystems/ai-runtime.md', '3. `internal/api/ai_handler.go`', 'internal/api/ai_handler.go')['heading']} "
            f"@L{_contract_reference('docs/release-control/v6/internal/subsystems/ai-runtime.md', '3. `internal/api/ai_handler.go`', 'internal/api/ai_handler.go')['line']}: "
            "internal/api/ai_handler.go",
            rendered,
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
                "internal/hostagent/command_client_test.go",
                "internal/hostagent/commands_deploy_test.go",
                "internal/hostagent/commands_host_update_test.go",
                "internal/hostagent/commands_storage_cleanup_test.go",
                "internal/hostagent/package_updates_test.go",
                "internal/hostagent/send_report_test.go",
                "internal/hostagent/storage_cleanup_test.go",
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
