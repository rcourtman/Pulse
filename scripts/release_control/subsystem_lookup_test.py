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
        self.assertEqual(result["status_summary"]["lane_count"], 12)

        file_entry = result["files"][0]
        matches = {match["subsystem"] for match in file_entry["matches"]}
        self.assertEqual(matches, {"api-contracts", "unified-resources"})
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "unified-resources"],
        )
        for match in file_entry["matches"]:
            self.assertEqual(match["lane_context"]["lane_id"], "L6")
            self.assertEqual(match["lane_context"]["lane"]["id"], "L6")

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
        self.assertEqual(match["contract"], "docs/release-control/v6/subsystems/cloud-paid.md")
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
        self.assertEqual(match["contract"], "docs/release-control/v6/subsystems/cloud-paid.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L3")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "pro-license-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/components/Settings/__tests__/ProLicensePanel.test.tsx"],
        )

    def test_lookup_paths_assigns_api_token_manager_to_api_contracts(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/APITokenManager.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            {match["subsystem"] for match in file_entry["matches"]},
            {"api-contracts"},
        )
        match = file_entry["matches"][0]
        self.assertEqual(match["contract"], "docs/release-control/v6/subsystems/api-contracts.md")
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "api-token-management-surface",
        )
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            ["frontend-modern/src/components/Settings/__tests__/APITokenManager.test.tsx"],
        )

    def test_lookup_paths_assigns_install_script_to_monitoring(self) -> None:
        result = lookup_paths(["scripts/install.sh"])
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
            "docs/release-control/v6/subsystems/monitoring.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
        self.assertEqual(
            match["verification_requirement"]["id"],
            "unified-agent-installer-runtime",
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
            "docs/release-control/v6/subsystems/monitoring.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
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
            "docs/release-control/v6/subsystems/relay-runtime.md",
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
            "docs/release-control/v6/subsystems/relay-runtime.md",
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
            "docs/release-control/v6/subsystems/frontend-primitives.md",
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
            "docs/release-control/v6/subsystems/frontend-primitives.md",
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
            "docs/release-control/v6/subsystems/frontend-primitives.md",
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

    def test_lookup_paths_assigns_remaining_settings_shell_panels_to_frontend_primitives(self) -> None:
        runtime_files = [
            "frontend-modern/src/components/Settings/APIAccessPanel.tsx",
            "frontend-modern/src/components/Settings/AuditLogPanel.tsx",
            "frontend-modern/src/components/Settings/AuditWebhookPanel.tsx",
            "frontend-modern/src/components/Settings/RecoverySettingsPanel.tsx",
            "frontend-modern/src/components/Settings/SSOProvidersPanel.tsx",
            "frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx",
        ]
        result = lookup_paths(runtime_files)
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"frontend-primitives"},
        )
        for file_entry in result["files"]:
            self.assertEqual(file_entry["classification"], "runtime")
            self.assertEqual(
                {match["subsystem"] for match in file_entry["matches"]},
                {"frontend-primitives"},
            )
            match = file_entry["matches"][0]
            self.assertEqual(
                match["contract"],
                "docs/release-control/v6/subsystems/frontend-primitives.md",
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
            "docs/release-control/v6/subsystems/organization-settings.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
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
                "frontend-modern/src/components/Settings/__tests__/agentModelGuardrails.test.ts",
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
            "docs/release-control/v6/subsystems/organization-settings.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
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
                "frontend-modern/src/components/Settings/__tests__/agentModelGuardrails.test.ts",
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
            "docs/release-control/v6/subsystems/organization-settings.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
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
                "frontend-modern/src/components/Settings/__tests__/agentModelGuardrails.test.ts",
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
            "docs/release-control/v6/subsystems/organization-settings.md",
        )
        self.assertEqual(match["lane_context"]["lane_id"], "L6")
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
            "docs/release-control/v6/subsystems/patrol-intelligence.md",
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
                "frontend-modern/src/components/Settings/__tests__/agentModelGuardrails.test.ts",
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
            "docs/release-control/v6/subsystems/patrol-intelligence.md",
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

    def test_lookup_paths_assigns_unified_agents_to_shared_monitoring_and_api_contracts(self) -> None:
        result = lookup_paths(["frontend-modern/src/components/Settings/UnifiedAgents.tsx"])
        self.assertEqual(result["unowned_runtime_files"], [])
        self.assertEqual(
            {item["subsystem"] for item in result["impacted_subsystems"]},
            {"api-contracts", "monitoring"},
        )
        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(
            file_entry["shared_ownership"]["subsystems"],
            ["api-contracts", "monitoring"],
        )
        matches = {match["subsystem"] for match in file_entry["matches"]}
        self.assertEqual(matches, {"api-contracts", "monitoring"})

        by_subsystem = {match["subsystem"]: match for match in file_entry["matches"]}
        api_match = by_subsystem["api-contracts"]
        monitoring_match = by_subsystem["monitoring"]

        self.assertEqual(
            api_match["contract"],
            "docs/release-control/v6/subsystems/api-contracts.md",
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
            monitoring_match["contract"],
            "docs/release-control/v6/subsystems/monitoring.md",
        )
        self.assertEqual(monitoring_match["lane_context"]["lane_id"], "L6")
        self.assertEqual(
            monitoring_match["verification_requirement"]["id"],
            "unified-agent-settings-surface",
        )
        self.assertEqual(
            monitoring_match["verification_requirement"]["exact_files"],
            [
                "frontend-modern/src/api/__tests__/monitoring.test.ts",
                "frontend-modern/src/components/Settings/__tests__/UnifiedAgents.test.tsx",
            ],
        )

    def test_lookup_paths_normalizes_absolute_repo_paths(self) -> None:
        absolute = str(Path(REPO_ROOT, "internal/api/resources.go"))
        result = lookup_paths([absolute])
        self.assertEqual(result["files"][0]["path"], "internal/api/resources.go")

    def test_lookup_paths_classifies_governance_files_as_ignored(self) -> None:
        result = lookup_paths(["docs/release-control/v6/status.json"])
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
        self.assertEqual(lane_context["lane_id"], "L6")
        self.assertEqual(
            {decision["id"] for decision in lane_context["resolved_decisions"]},
            {"host-type-migration-boundary-audit", "orchestrator-retired", "top-level-governance-split"},
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

    def test_lookup_paths_maps_hostagent_runtime_to_monitoring(self) -> None:
        result = lookup_paths(["internal/hostagent/agent.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "monitoring")
        self.assertEqual(match["contract"], "docs/release-control/v6/subsystems/monitoring.md")
        self.assertEqual(match["verification_requirement"]["id"], "host-agent-runtime")
        self.assertEqual(
            match["verification_requirement"]["exact_files"],
            [
                "internal/agentupdate/coverage_test.go",
                "internal/hostagent/agent_metrics_test.go",
                "internal/hostagent/agent_new_test.go",
                "internal/hostagent/send_report_test.go",
            ],
        )

    def test_lookup_paths_maps_agentupdate_runtime_to_monitoring(self) -> None:
        result = lookup_paths(["internal/agentupdate/update.go"])
        self.assertEqual(result["unowned_runtime_files"], [])

        file_entry = result["files"][0]
        self.assertEqual(file_entry["classification"], "runtime")
        self.assertEqual(len(file_entry["matches"]), 1)

        match = file_entry["matches"][0]
        self.assertEqual(match["subsystem"], "monitoring")
        self.assertEqual(match["contract"], "docs/release-control/v6/subsystems/monitoring.md")
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
