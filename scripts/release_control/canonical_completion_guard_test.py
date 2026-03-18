import unittest
from pathlib import Path

from canonical_completion_guard import (
    REPO_ROOT,
    SUBSYSTEM_REGISTRY,
    build_verification_requirements,
    contract_patch_has_substantive_change,
    infer_impacted_subsystems,
    is_ignored_runtime_file,
    is_test_or_fixture,
    load_subsystem_rules,
    path_policy_matches,
    parse_args,
    required_contract_updates,
    staged_verification_files_for_requirement,
    stdin_files,
    subsystem_matches_path,
)


def owned_runtime_files(rule: dict) -> list[str]:
    owned: set[str] = set()
    search_roots: set[Path] = set()

    for prefix in rule.get("owned_prefixes", []):
        root = REPO_ROOT / prefix
        if root.exists():
            search_roots.add(root if root.is_dir() else root.parent)
            continue

        parent = root.parent
        while parent != REPO_ROOT and not parent.exists():
            parent = parent.parent
        if parent.exists():
            search_roots.add(parent)

    for root in search_roots:
        for path in root.rglob("*"):
            if not path.is_file():
                continue
            rel = path.relative_to(REPO_ROOT).as_posix()
            if is_test_or_fixture(rel) or is_ignored_runtime_file(rel):
                continue
            if subsystem_matches_path(rule, rel):
                owned.add(rel)

    for rel in rule.get("owned_files", []):
        path = REPO_ROOT / rel
        if not path.exists() or not path.is_file():
            continue
        if is_test_or_fixture(rel) or is_ignored_runtime_file(rel):
            continue
        if subsystem_matches_path(rule, rel):
            owned.add(rel)

    return sorted(owned)


def unmatched_owned_runtime_files(rule: dict) -> list[str]:
    policies = list(rule.get("verification", {}).get("path_policies", []))
    return [
        rel
        for rel in owned_runtime_files(rule)
        if not any(path_policy_matches(policy, rel) for policy in policies)
    ]


def first_matching_policy_id(rule: dict, rel: str) -> str:
    for policy in rule.get("verification", {}).get("path_policies", []):
        if path_policy_matches(policy, rel):
            return str(policy["id"])
    return "DEFAULT"


class CanonicalCompletionGuardTest(unittest.TestCase):
    def test_registry_exists_and_contains_required_subsystems(self):
        self.assertTrue(SUBSYSTEM_REGISTRY.exists())
        rules = load_subsystem_rules()
        ids = {rule["id"] for rule in rules}
        self.assertEqual(
            ids,
            {
                "agent-lifecycle",
                "ai-runtime",
                "alerts",
                "api-contracts",
                "cloud-paid",
                "deployment-installability",
                "frontend-primitives",
                "monitoring",
                "notifications",
                "organization-settings",
                "patrol-intelligence",
                "performance-and-scalability",
                "relay-runtime",
                "security-privacy",
                "storage-recovery",
                "unified-resources",
            },
        )
        for rule in rules:
            self.assertIn("verification", rule)
            self.assertIn("allow_same_subsystem_tests", rule["verification"])
            self.assertIn("test_prefixes", rule["verification"])
            self.assertIn("exact_files", rule["verification"])
            self.assertIn("require_explicit_path_policy_coverage", rule["verification"])
            self.assertIn("path_policies", rule["verification"])

    def test_monitoring_runtime_change_requires_monitoring_contract(self):
        required = infer_impacted_subsystems(["internal/monitoring/monitor.go"])
        self.assertEqual(set(required), {"monitoring"})

        monitoring = required["monitoring"]
        self.assertEqual(monitoring["id"], "monitoring")
        self.assertEqual(
            monitoring["contract"],
            "docs/release-control/v6/internal/subsystems/monitoring.md",
        )
        self.assertEqual(
            monitoring["touched_runtime_files"],
            ["internal/monitoring/monitor.go"],
        )
        self.assertTrue(
            monitoring["verification"]["require_explicit_path_policy_coverage"]
        )
        self.assertEqual(
            monitoring["verification"]["exact_files"],
            ["internal/unifiedresources/code_standards_test.go"],
        )
        policy_ids = [policy["id"] for policy in monitoring["verification"]["path_policies"]]
        self.assertEqual(
            policy_ids,
            [
                "discovery-provider-runtime",
                "metrics-hot-path",
                "metrics-history-runtime",
                "memory-source-runtime",
                "container-entrypoint-runtime",
                "monitoring-runtime",
            ],
        )
        self.assertEqual(
            monitoring["verification_requirements"],
            [
                {
                    "id": "monitoring-runtime",
                    "label": "monitoring runtime proof",
                    "touched_runtime_files": ["internal/monitoring/monitor.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/monitoring/canonical_guardrails_test.go",
                        "internal/unifiedresources/code_standards_test.go",
                    ],
                }
            ],
        )

    def test_install_script_change_uses_unified_agent_installer_policy(self):
        required = infer_impacted_subsystems(["scripts/install.sh"])
        self.assertEqual(set(required), {"agent-lifecycle", "deployment-installability"})

        monitoring = required["agent-lifecycle"]
        self.assertEqual(
            monitoring["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            monitoring["touched_runtime_files"],
            ["scripts/install.sh"],
        )
        self.assertEqual(
            monitoring["verification_requirements"],
            [
                {
                    "id": "unified-agent-installer-runtime",
                    "label": "unified agent installer runtime proof",
                    "touched_runtime_files": ["scripts/install.sh"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "scripts/installtests/install_sh_test.go",
                    ],
                }
            ],
        )
        installability = required["deployment-installability"]
        self.assertEqual(
            installability["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(
            installability["touched_runtime_files"],
            ["scripts/install.sh"],
        )
        self.assertEqual(
            installability["verification_requirements"],
            [
                {
                    "id": "shell-installer-runtime",
                    "label": "shell installer runtime proof",
                    "touched_runtime_files": ["scripts/install.sh"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "scripts/installtests/install_sh_test.go",
                    ],
                }
            ],
        )

    def test_windows_install_script_change_uses_shared_installer_policies(self):
        required = infer_impacted_subsystems(["scripts/install.ps1"])
        self.assertEqual(set(required), {"agent-lifecycle", "deployment-installability"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["scripts/install.ps1"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "windows-agent-installer-runtime",
                    "label": "windows agent installer lifecycle proof",
                    "touched_runtime_files": ["scripts/install.ps1"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "scripts/installtests/install_ps1_test.go",
                    ],
                }
            ],
        )

        installability = required["deployment-installability"]
        self.assertEqual(
            installability["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(
            installability["touched_runtime_files"],
            ["scripts/install.ps1"],
        )
        self.assertEqual(
            installability["verification_requirements"],
            [
                {
                    "id": "deployment-script-runtime",
                    "label": "deployment script runtime proof",
                    "touched_runtime_files": ["scripts/install.ps1"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "scripts/installtests/install_docker_sh_test.go",
                        "scripts/installtests/install_ps1_test.go",
                        "scripts/installtests/root_install_sh_test.go",
                    ],
                }
            ],
        )

    def test_unified_agent_runtime_change_requires_agent_lifecycle_contract(self):
        required = infer_impacted_subsystems(["internal/hostagent/agent.go"])
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["internal/hostagent/agent.go"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "unified-agent-runtime",
                    "label": "unified agent runtime proof",
                    "touched_runtime_files": ["internal/hostagent/agent.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/agentupdate/coverage_test.go",
                        "internal/hostagent/agent_metrics_test.go",
                        "internal/hostagent/agent_new_test.go",
                        "internal/hostagent/send_report_test.go",
                    ],
                }
            ],
        )

    def test_proxmox_setup_runtime_change_requires_agent_lifecycle_contract(self):
        required = infer_impacted_subsystems(["internal/hostagent/proxmox_setup.go"])
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["internal/hostagent/proxmox_setup.go"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "proxmox-unified-agent-setup-runtime",
                    "label": "proxmox unified agent setup runtime proof",
                    "touched_runtime_files": ["internal/hostagent/proxmox_setup.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/hostagent/proxmox_setup_network_coverage_test.go",
                        "internal/hostagent/proxmox_setup_test.go",
                    ],
                }
            ],
        )

    def test_agentupdate_runtime_change_requires_agent_lifecycle_contract(self):
        required = infer_impacted_subsystems(["internal/agentupdate/update.go"])
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["internal/agentupdate/update.go"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "agent-update-runtime",
                    "label": "agent update runtime proof",
                    "touched_runtime_files": ["internal/agentupdate/update.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/agentupdate/coverage_test.go",
                        "internal/agentupdate/update_hostagent_integration_test.go",
                        "internal/agentupdate/update_http_test.go",
                    ],
                }
            ],
        )

    def test_container_install_script_change_uses_deployment_script_policy(self):
        required = infer_impacted_subsystems(["scripts/install-container-agent.sh"])
        self.assertEqual(set(required), {"deployment-installability"})

        installability = required["deployment-installability"]
        self.assertEqual(
            installability["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(
            installability["touched_runtime_files"],
            ["scripts/install-container-agent.sh"],
        )
        self.assertEqual(
            installability["verification_requirements"],
            [
                {
                    "id": "deployment-script-runtime",
                    "label": "deployment script runtime proof",
                    "touched_runtime_files": ["scripts/install-container-agent.sh"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "scripts/installtests/install_docker_sh_test.go",
                        "scripts/installtests/install_ps1_test.go",
                        "scripts/installtests/root_install_sh_test.go",
                    ],
                }
            ],
        )

    def test_auto_update_script_change_uses_deployment_script_policy(self):
        required = infer_impacted_subsystems(["scripts/pulse-auto-update.sh"])
        self.assertEqual(set(required), {"deployment-installability"})

        installability = required["deployment-installability"]
        self.assertEqual(
            installability["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(
            installability["touched_runtime_files"],
            ["scripts/pulse-auto-update.sh"],
        )
        self.assertEqual(
            installability["verification_requirements"],
            [
                {
                    "id": "deployment-script-runtime",
                    "label": "deployment script runtime proof",
                    "touched_runtime_files": ["scripts/pulse-auto-update.sh"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "scripts/installtests/install_docker_sh_test.go",
                        "scripts/installtests/install_ps1_test.go",
                        "scripts/installtests/root_install_sh_test.go",
                    ],
                }
            ],
        )

    def test_docker_entrypoint_change_uses_container_entrypoint_policy(self):
        required = infer_impacted_subsystems(["docker-entrypoint.sh"])
        self.assertEqual(set(required), {"monitoring"})

        monitoring = required["monitoring"]
        self.assertEqual(
            monitoring["contract"],
            "docs/release-control/v6/internal/subsystems/monitoring.md",
        )
        self.assertEqual(
            monitoring["touched_runtime_files"],
            ["docker-entrypoint.sh"],
        )
        self.assertEqual(
            monitoring["verification_requirements"],
            [
                {
                    "id": "container-entrypoint-runtime",
                    "label": "container entrypoint runtime proof",
                    "touched_runtime_files": ["docker-entrypoint.sh"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "scripts/installtests/docker_entrypoint_test.go",
                    ],
                }
            ],
        )

    def test_unified_resource_api_change_requires_two_contracts(self):
        required = infer_impacted_subsystems(["internal/api/resources.go"])
        self.assertEqual(set(required), {"api-contracts", "unified-resources"})

        api_contracts = required["api-contracts"]
        self.assertEqual(api_contracts["contract"], "docs/release-control/v6/internal/subsystems/api-contracts.md")
        self.assertEqual(api_contracts["touched_runtime_files"], ["internal/api/resources.go"])
        self.assertTrue(
            api_contracts["verification"]["require_explicit_path_policy_coverage"]
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "backend-payload-contracts",
                    "label": "backend API payload proof",
                    "touched_runtime_files": ["internal/api/resources.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": ["frontend-modern/src/api/__tests__/"],
                    "exact_files": [
                        "frontend-modern/src/types/api.ts",
                        "internal/api/contract_test.go",
                    ],
                }
            ],
        )

        unified_resources = required["unified-resources"]
        self.assertEqual(
            unified_resources["contract"],
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
        )
        self.assertEqual(
            unified_resources["touched_runtime_files"],
            ["internal/api/resources.go"],
        )
        self.assertTrue(
            unified_resources["verification"]["require_explicit_path_policy_coverage"]
        )
        self.assertEqual(
            unified_resources["verification_requirements"],
            [
                {
                    "id": "resource-consumers",
                    "label": "unified resource consumer proof",
                    "touched_runtime_files": ["internal/api/resources.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.performance.contract.test.tsx",
                        "frontend-modern/src/components/Infrastructure/__tests__/UnifiedResourceTable.workloads-link.test.tsx",
                        "frontend-modern/src/components/Infrastructure/__tests__/infrastructureSelectors.test.ts",
                        "frontend-modern/src/components/Infrastructure/__tests__/resourceDetailMappers.test.ts",
                        "frontend-modern/src/hooks/__tests__/useDashboardTrends.test.ts",
                        "frontend-modern/src/hooks/__tests__/useUnifiedResources.test.ts",
                        "frontend-modern/src/pages/__tests__/Infrastructure.empty-state.test.tsx",
                        "frontend-modern/src/pages/__tests__/Infrastructure.pbs-pmg.test.tsx",
                        "frontend-modern/src/routing/__tests__/resourceLinks.test.ts",
                        "frontend-modern/src/stores/__tests__/websocket-unified.test.ts",
                        "frontend-modern/src/types/__tests__/resource.test.ts",
                        "internal/unifiedresources/code_standards_test.go",
                    ],
                }
            ],
        )

    def test_nodes_client_change_requires_lifecycle_and_api_contracts(self):
        required = infer_impacted_subsystems(["frontend-modern/src/api/nodes.ts"])
        self.assertEqual(set(required), {"agent-lifecycle", "api-contracts"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/api/nodes.ts"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "proxmox-lifecycle-client-surface",
                    "label": "proxmox lifecycle API client proof",
                    "touched_runtime_files": ["frontend-modern/src/api/nodes.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/nodes.test.ts",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/api/nodes.ts"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "proxmox-node-client-contract",
                    "label": "proxmox node client API contract proof",
                    "touched_runtime_files": ["frontend-modern/src/api/nodes.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/nodes.test.ts",
                        "frontend-modern/src/types/api.ts",
                    ],
                }
            ],
        )

    def test_updates_client_change_requires_installability_and_api_contracts(self):
        required = infer_impacted_subsystems(["frontend-modern/src/api/updates.ts"])
        self.assertEqual(set(required), {"deployment-installability", "api-contracts"})

        installability = required["deployment-installability"]
        self.assertEqual(
            installability["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(
            installability["touched_runtime_files"],
            ["frontend-modern/src/api/updates.ts"],
        )
        self.assertEqual(
            installability["verification_requirements"],
            [
                {
                    "id": "updates-api-surface",
                    "label": "update transport API proof",
                    "touched_runtime_files": ["frontend-modern/src/api/updates.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/updates.test.ts",
                        "internal/api/updates_test.go",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/api/updates.ts"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "update-transport-contract",
                    "label": "update transport API contract proof",
                    "touched_runtime_files": ["frontend-modern/src/api/updates.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/updates.test.ts",
                        "frontend-modern/src/types/api.ts",
                        "internal/api/updates_test.go",
                    ],
                }
            ],
        )

    def test_updates_handler_change_requires_installability_and_api_contracts(self):
        required = infer_impacted_subsystems(["internal/api/updates.go"])
        self.assertEqual(set(required), {"deployment-installability", "api-contracts"})

        installability = required["deployment-installability"]
        self.assertEqual(
            installability["contract"],
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        )
        self.assertEqual(
            installability["touched_runtime_files"],
            ["internal/api/updates.go"],
        )
        self.assertEqual(
            installability["verification_requirements"],
            [
                {
                    "id": "updates-api-surface",
                    "label": "update transport API proof",
                    "touched_runtime_files": ["internal/api/updates.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/updates.test.ts",
                        "internal/api/updates_test.go",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["internal/api/updates.go"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "update-transport-contract",
                    "label": "update transport API contract proof",
                    "touched_runtime_files": ["internal/api/updates.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/updates.test.ts",
                        "frontend-modern/src/types/api.ts",
                        "internal/api/updates_test.go",
                    ],
                }
            ],
        )

    def test_agent_profiles_client_change_requires_lifecycle_and_api_contracts(self):
        required = infer_impacted_subsystems(["frontend-modern/src/api/agentProfiles.ts"])
        self.assertEqual(set(required), {"agent-lifecycle", "api-contracts"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/api/agentProfiles.ts"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "agent-profiles-surface",
                    "label": "agent profile management proof",
                    "touched_runtime_files": ["frontend-modern/src/api/agentProfiles.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/AgentProfilesPanel.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                        "frontend-modern/src/utils/__tests__/agentProfilesPresentation.test.ts",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/api/agentProfiles.ts"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "agent-profiles-client-contract",
                    "label": "agent profiles client API contract proof",
                    "touched_runtime_files": ["frontend-modern/src/api/agentProfiles.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/types/api.ts",
                    ],
                }
            ],
        )

    def test_recovery_route_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(["frontend-modern/src/pages/RecoveryRoute.tsx"])
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/pages/RecoveryRoute.tsx"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": ["frontend-modern/src/pages/RecoveryRoute.tsx"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_dashboard_page_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(["frontend-modern/src/pages/Dashboard.tsx"])
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/pages/Dashboard.tsx"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "dashboard-storage-recovery-surface",
                    "label": "dashboard storage and recovery surface proof",
                    "touched_runtime_files": ["frontend-modern/src/pages/Dashboard.tsx"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/pages/__tests__/DashboardPage.test.tsx",
                    ],
                }
            ],
        )

    def test_dashboard_widgets_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/pages/DashboardPanels/dashboardWidgets.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/pages/DashboardPanels/dashboardWidgets.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "dashboard-storage-recovery-surface",
                    "label": "dashboard storage and recovery surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/pages/DashboardPanels/dashboardWidgets.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/pages/__tests__/DashboardPage.test.tsx",
                    ],
                }
            ],
        )

    def test_recovery_points_hook_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(["frontend-modern/src/hooks/useRecoveryPoints.ts"])
        self.assertEqual(set(required), {"storage-recovery"})
        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["verification_requirements"][0]["id"],
            "recovery-product-surface",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/hooks/useRecoveryPoints.ts"],
        )

    def test_recovery_points_facets_hook_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/hooks/useRecoveryPointsFacets.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})
        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["verification_requirements"][0]["id"],
            "recovery-product-surface",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/hooks/useRecoveryPointsFacets.ts"],
        )

    def test_recovery_points_series_hook_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/hooks/useRecoveryPointsSeries.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})
        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["verification_requirements"][0]["id"],
            "recovery-product-surface",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/hooks/useRecoveryPointsSeries.ts"],
        )

    def test_recovery_rollups_hook_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(["frontend-modern/src/hooks/useRecoveryRollups.ts"])
        self.assertEqual(set(required), {"storage-recovery"})
        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["verification_requirements"][0]["id"],
            "recovery-product-surface",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/hooks/useRecoveryRollups.ts"],
        )

    def test_dashboard_recovery_hook_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(["frontend-modern/src/hooks/useDashboardRecovery.ts"])
        self.assertEqual(set(required), {"storage-recovery"})
        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["verification_requirements"][0]["id"],
            "recovery-product-surface",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/hooks/useDashboardRecovery.ts"],
        )

    def test_recovery_status_panel_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/pages/DashboardPanels/RecoveryStatusPanel.tsx"]
        )
        self.assertEqual(set(required), {"storage-recovery"})
        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["verification_requirements"][0]["id"],
            "recovery-product-surface",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/pages/DashboardPanels/RecoveryStatusPanel.tsx"],
        )

    def test_dashboard_recovery_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/dashboardRecoveryPresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})
        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["verification_requirements"][0]["id"],
            "recovery-product-surface",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/dashboardRecoveryPresentation.ts"],
        )

    def test_storage_panel_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/pages/DashboardPanels/StoragePanel.tsx"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/pages/DashboardPanels/StoragePanel.tsx"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "storage-product-surface",
                    "label": "storage product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/pages/DashboardPanels/StoragePanel.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
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
                }
            ],
        )

    def test_recovery_types_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(["frontend-modern/src/types/recovery.ts"])
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/types/recovery.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": ["frontend-modern/src/types/recovery.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_date_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryDatePresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryDatePresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryDatePresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_status_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryStatusPresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryStatusPresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryStatusPresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_summary_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoverySummaryPresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoverySummaryPresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoverySummaryPresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_record_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryRecordPresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryRecordPresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryRecordPresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_outcome_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryOutcomePresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryOutcomePresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryOutcomePresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_action_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryActionPresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryActionPresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryActionPresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_artifact_mode_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryArtifactModePresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryArtifactModePresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryArtifactModePresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_empty_state_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryEmptyStatePresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryEmptyStatePresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryEmptyStatePresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_filter_chip_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryFilterChipPresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryFilterChipPresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryFilterChipPresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_issue_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryIssuePresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryIssuePresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryIssuePresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_table_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryTablePresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryTablePresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryTablePresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_timeline_chart_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryTimelineChartPresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryTimelineChartPresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryTimelineChartPresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_recovery_timeline_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoveryTimelinePresentation.ts"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/utils/recoveryTimelinePresentation.ts"],
        )
        self.assertEqual(
            recovery["verification_requirements"],
            [
                {
                    "id": "recovery-product-surface",
                    "label": "recovery product surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/recoveryTimelinePresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Recovery/RecoverySummary.test.tsx",
                        "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
                        "frontend-modern/src/pages/__tests__/RecoveryRoute.test.tsx",
                        "frontend-modern/src/utils/__tests__/dashboardRecoveryPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                    ],
                }
            ],
        )

    def test_frontend_install_helper_change_requires_lifecycle_and_api_contracts(self):
        required = infer_impacted_subsystems(["frontend-modern/src/utils/agentInstallCommand.ts"])
        self.assertEqual(set(required), {"agent-lifecycle", "api-contracts"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/utils/agentInstallCommand.ts"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "frontend-install-command-helper",
                    "label": "frontend install command helper proof",
                    "touched_runtime_files": ["frontend-modern/src/utils/agentInstallCommand.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/utils/__tests__/agentInstallCommand.test.ts",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/utils/agentInstallCommand.ts"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "frontend-install-command-helper",
                    "label": "frontend install transport contract proof",
                    "touched_runtime_files": ["frontend-modern/src/utils/agentInstallCommand.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/utils/__tests__/agentInstallCommand.test.ts",
                    ],
                }
            ],
        )

    def test_node_modal_change_uses_node_setup_settings_policy(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/Settings/NodeModal.tsx"]
        )
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/NodeModal.tsx"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "node-setup-settings-surface",
                    "label": "node setup/install command proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/NodeModal.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/NodeModal.guardrails.test.ts"
                    ],
                }
            ],
        )

    def test_setup_completion_panel_change_uses_setup_completion_install_policy(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx"]
        )
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "setup-completion-install-surface",
                    "label": "setup completion install handoff proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/SetupWizard/__tests__/SetupCompletionPanel.guardrails.test.ts"
                    ],
                }
            ],
        )

    def test_results_step_change_uses_deploy_fallback_install_policy(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/Infrastructure/deploy/ResultsStep.tsx"]
        )
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/components/Infrastructure/deploy/ResultsStep.tsx"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "deploy-fallback-install-surface",
                    "label": "deploy fallback install command proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Infrastructure/deploy/ResultsStep.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Infrastructure/__tests__/DeployStepComponents.test.tsx"
                    ],
                }
            ],
        )

    def test_agent_profiles_panel_change_uses_agent_profiles_surface_policy(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/Settings/AgentProfilesPanel.tsx"]
        )
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/AgentProfilesPanel.tsx"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "agent-profiles-surface",
                    "label": "agent profile management proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/AgentProfilesPanel.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/AgentProfilesPanel.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                        "frontend-modern/src/utils/__tests__/agentProfilesPresentation.test.ts",
                    ],
                }
            ],
        )

    def test_agent_profiles_presentation_change_uses_agent_profiles_surface_policy(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/agentProfilesPresentation.ts"]
        )
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/utils/agentProfilesPresentation.ts"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "agent-profiles-surface",
                    "label": "agent profile management proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/agentProfilesPresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/AgentProfilesPanel.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/monitoredSystemModelGuardrails.test.ts",
                        "frontend-modern/src/utils/__tests__/agentProfilesPresentation.test.ts",
                    ],
                }
            ],
        )

    def test_unified_agent_settings_change_requires_lifecycle_and_api_contracts(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/Settings/UnifiedAgents.tsx"]
        )
        self.assertEqual(set(required), {"agent-lifecycle", "api-contracts"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/UnifiedAgents.tsx"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "unified-agent-settings-surface",
                    "label": "unified agent settings lifecycle proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/UnifiedAgents.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/api/__tests__/monitoring.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/UnifiedAgents.test.tsx",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/UnifiedAgents.tsx"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "unified-agent-settings-surface",
                    "label": "infrastructure operations API proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/UnifiedAgents.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/api/__tests__/monitoring.test.ts",
                        "frontend-modern/src/api/__tests__/security.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/UnifiedAgents.test.tsx",
                    ],
                }
            ],
        )

    def test_agent_install_backend_change_requires_lifecycle_and_api_contracts(self):
        required = infer_impacted_subsystems(["internal/api/agent_install_command_shared.go"])
        self.assertEqual(set(required), {"agent-lifecycle", "api-contracts"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["internal/api/agent_install_command_shared.go"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "agent-install-api-surface",
                    "label": "agent install and registration API proof",
                    "touched_runtime_files": ["internal/api/agent_install_command_shared.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/agent_install_command_shared_test.go",
                        "internal/api/config_handlers_auto_register_test.go",
                        "internal/api/config_handlers_canonical_auto_register_test.go",
                        "internal/api/contract_test.go",
                        "internal/api/hosted_agent_install_command_test.go",
                        "internal/api/unified_agent_more_test.go",
                        "internal/api/unified_agent_test.go",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["internal/api/agent_install_command_shared.go"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "agent-install-backend-contract",
                    "label": "agent install and registration backend API contract proof",
                    "touched_runtime_files": ["internal/api/agent_install_command_shared.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/agent_install_command_shared_test.go",
                        "internal/api/config_handlers_auto_register_test.go",
                        "internal/api/config_handlers_canonical_auto_register_test.go",
                        "internal/api/contract_test.go",
                        "internal/api/hosted_agent_install_command_test.go",
                        "internal/api/unified_agent_more_test.go",
                        "internal/api/unified_agent_test.go",
                    ],
                }
            ],
        )

    def test_unified_agent_backend_change_requires_lifecycle_and_api_contracts(self):
        required = infer_impacted_subsystems(["internal/api/unified_agent.go"])
        self.assertEqual(set(required), {"agent-lifecycle", "api-contracts"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["internal/api/unified_agent.go"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "agent-install-api-surface",
                    "label": "agent install and registration API proof",
                    "touched_runtime_files": ["internal/api/unified_agent.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/agent_install_command_shared_test.go",
                        "internal/api/config_handlers_auto_register_test.go",
                        "internal/api/config_handlers_canonical_auto_register_test.go",
                        "internal/api/contract_test.go",
                        "internal/api/hosted_agent_install_command_test.go",
                        "internal/api/unified_agent_more_test.go",
                        "internal/api/unified_agent_test.go",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["internal/api/unified_agent.go"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "agent-install-backend-contract",
                    "label": "agent install and registration backend API contract proof",
                    "touched_runtime_files": ["internal/api/unified_agent.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/agent_install_command_shared_test.go",
                        "internal/api/config_handlers_auto_register_test.go",
                        "internal/api/config_handlers_canonical_auto_register_test.go",
                        "internal/api/contract_test.go",
                        "internal/api/hosted_agent_install_command_test.go",
                        "internal/api/unified_agent_more_test.go",
                        "internal/api/unified_agent_test.go",
                    ],
                }
            ],
        )

    def test_config_setup_backend_change_requires_lifecycle_and_api_contracts(self):
        required = infer_impacted_subsystems(["internal/api/config_setup_handlers.go"])
        self.assertEqual(set(required), {"agent-lifecycle", "api-contracts"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["internal/api/config_setup_handlers.go"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "agent-install-api-surface",
                    "label": "agent install and registration API proof",
                    "touched_runtime_files": ["internal/api/config_setup_handlers.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/agent_install_command_shared_test.go",
                        "internal/api/config_handlers_auto_register_test.go",
                        "internal/api/config_handlers_canonical_auto_register_test.go",
                        "internal/api/contract_test.go",
                        "internal/api/hosted_agent_install_command_test.go",
                        "internal/api/unified_agent_more_test.go",
                        "internal/api/unified_agent_test.go",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["internal/api/config_setup_handlers.go"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "agent-install-backend-contract",
                    "label": "agent install and registration backend API contract proof",
                    "touched_runtime_files": ["internal/api/config_setup_handlers.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/agent_install_command_shared_test.go",
                        "internal/api/config_handlers_auto_register_test.go",
                        "internal/api/config_handlers_canonical_auto_register_test.go",
                        "internal/api/contract_test.go",
                        "internal/api/hosted_agent_install_command_test.go",
                        "internal/api/unified_agent_more_test.go",
                        "internal/api/unified_agent_test.go",
                    ],
                }
            ],
        )

    def test_session_store_change_uses_session_migration_proof_policy(self):
        required = infer_impacted_subsystems(["internal/api/session_store.go"])
        self.assertEqual(set(required), {"api-contracts"})

        api_contracts = required["api-contracts"]
        self.assertEqual(api_contracts["contract"], "docs/release-control/v6/internal/subsystems/api-contracts.md")
        self.assertEqual(api_contracts["touched_runtime_files"], ["internal/api/session_store.go"])
        self.assertTrue(
            api_contracts["verification"]["require_explicit_path_policy_coverage"]
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "auth-state-persistence-compatibility",
                    "label": "session and CSRF persistence migration proof",
                    "touched_runtime_files": ["internal/api/session_store.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/csrf_store_test.go",
                        "internal/api/session_store_test.go",
                        "tests/migration/v5_session_db_test.go",
                    ],
                }
            ],
        )

    def test_csrf_store_change_uses_auth_state_migration_proof_policy(self):
        required = infer_impacted_subsystems(["internal/api/csrf_store.go"])
        self.assertEqual(set(required), {"api-contracts"})

        api_contracts = required["api-contracts"]
        self.assertEqual(api_contracts["contract"], "docs/release-control/v6/internal/subsystems/api-contracts.md")
        self.assertEqual(api_contracts["touched_runtime_files"], ["internal/api/csrf_store.go"])
        self.assertTrue(
            api_contracts["verification"]["require_explicit_path_policy_coverage"]
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "auth-state-persistence-compatibility",
                    "label": "session and CSRF persistence migration proof",
                    "touched_runtime_files": ["internal/api/csrf_store.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/csrf_store_test.go",
                        "internal/api/session_store_test.go",
                        "tests/migration/v5_session_db_test.go",
                    ],
                }
            ],
        )

    def test_api_token_manager_change_requires_security_and_api_contracts(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/Settings/APITokenManager.tsx"]
        )
        self.assertEqual(set(required), {"security-privacy", "api-contracts"})

        security = required["security-privacy"]
        self.assertEqual(
            security["contract"],
            "docs/release-control/v6/internal/subsystems/security-privacy.md",
        )
        self.assertEqual(
            security["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/APITokenManager.tsx"],
        )
        self.assertEqual(
            security["verification_requirements"],
            [
                {
                    "id": "security-settings-surfaces",
                    "label": "security settings surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/APITokenManager.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/APITokenManager.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/SecurityPostureSummary.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                        "frontend-modern/src/stores/__tests__/systemSettings.test.ts",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/APITokenManager.tsx"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "api-token-management-surface",
                    "label": "API token management surface proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/APITokenManager.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/APITokenManager.test.tsx",
                    ],
                }
            ],
        )

    def test_security_client_change_requires_security_and_api_contracts(self):
        required = infer_impacted_subsystems(["frontend-modern/src/api/security.ts"])
        self.assertEqual(set(required), {"security-privacy", "api-contracts"})

        security = required["security-privacy"]
        self.assertEqual(
            security["contract"],
            "docs/release-control/v6/internal/subsystems/security-privacy.md",
        )
        self.assertEqual(
            security["touched_runtime_files"],
            ["frontend-modern/src/api/security.ts"],
        )
        self.assertEqual(
            security["verification_requirements"],
            [
                {
                    "id": "security-api-surface",
                    "label": "security API surface proof",
                    "touched_runtime_files": ["frontend-modern/src/api/security.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/security.test.ts",
                        "internal/api/security_regression_test.go",
                        "internal/api/security_status_additional_test.go",
                        "internal/api/security_tokens_lifecycle_test.go",
                        "internal/api/security_tokens_owner_binding_test.go",
                        "internal/api/security_tokens_test.go",
                        "internal/api/system_settings_telemetry_test.go",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/api/security.ts"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "security-transport-contract",
                    "label": "security transport API contract proof",
                    "touched_runtime_files": ["frontend-modern/src/api/security.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/security.test.ts",
                        "frontend-modern/src/types/api.ts",
                        "internal/api/security_regression_test.go",
                        "internal/api/security_status_additional_test.go",
                        "internal/api/security_tokens_lifecycle_test.go",
                        "internal/api/security_tokens_owner_binding_test.go",
                        "internal/api/security_tokens_test.go",
                        "internal/api/system_settings_telemetry_test.go",
                    ],
                }
            ],
        )

    def test_security_handler_change_requires_security_and_api_contracts(self):
        required = infer_impacted_subsystems(["internal/api/security.go"])
        self.assertEqual(set(required), {"security-privacy", "api-contracts"})

        security = required["security-privacy"]
        self.assertEqual(
            security["verification_requirements"][0]["id"],
            "security-api-surface",
        )
        self.assertEqual(
            security["touched_runtime_files"],
            ["internal/api/security.go"],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["verification_requirements"][0]["id"],
            "security-transport-contract",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["internal/api/security.go"],
        )

    def test_security_tokens_handler_change_requires_security_and_api_contracts(self):
        required = infer_impacted_subsystems(["internal/api/security_tokens.go"])
        self.assertEqual(set(required), {"security-privacy", "api-contracts"})

        security = required["security-privacy"]
        self.assertEqual(
            security["verification_requirements"][0]["id"],
            "security-api-surface",
        )
        self.assertEqual(
            security["touched_runtime_files"],
            ["internal/api/security_tokens.go"],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["verification_requirements"][0]["id"],
            "security-transport-contract",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["internal/api/security_tokens.go"],
        )

    def test_system_settings_handler_change_requires_security_and_api_contracts(self):
        required = infer_impacted_subsystems(["internal/api/system_settings.go"])
        self.assertEqual(set(required), {"security-privacy", "api-contracts"})

        security = required["security-privacy"]
        self.assertEqual(
            security["verification_requirements"][0]["id"],
            "security-api-surface",
        )
        self.assertEqual(
            security["touched_runtime_files"],
            ["internal/api/system_settings.go"],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["verification_requirements"][0]["id"],
            "security-transport-contract",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["internal/api/system_settings.go"],
        )

    def test_shared_canonical_file_requires_dependent_contract_update(self):
        required = required_contract_updates(["internal/unifiedresources/views.go"])
        self.assertEqual(
            set(required),
            {
                "docs/release-control/v6/internal/subsystems/monitoring.md",
                "docs/release-control/v6/internal/subsystems/unified-resources.md",
            },
        )
        self.assertEqual(
            required["docs/release-control/v6/internal/subsystems/unified-resources.md"]["reason"],
            "owner",
        )
        self.assertEqual(
            required["docs/release-control/v6/internal/subsystems/monitoring.md"]["reason"],
            "dependent-reference",
        )
        self.assertEqual(
            required["docs/release-control/v6/internal/subsystems/monitoring.md"]["matched_references"],
            [
                "## Canonical Files: internal/unifiedresources/views.go",
                "## Extension Points: internal/unifiedresources/views.go",
            ],
        )

    def test_monitoring_owned_runtime_does_not_require_unified_resources_contract(self):
        required = required_contract_updates(["internal/monitoring/monitor.go"])
        self.assertEqual(
            required,
            {
                "docs/release-control/v6/internal/subsystems/monitoring.md": {
                    "subsystem": "monitoring",
                    "contract": "docs/release-control/v6/internal/subsystems/monitoring.md",
                    "reason": "owner",
                    "touched_runtime_files": ["internal/monitoring/monitor.go"],
                    "matched_references": [],
                }
            },
        )

    def test_contract_patch_metadata_only_is_not_substantive(self):
        patch = """diff --git a/docs/release-control/v6/internal/subsystems/monitoring.md b/docs/release-control/v6/internal/subsystems/monitoring.md
index 1111111..2222222 100644
--- a/docs/release-control/v6/internal/subsystems/monitoring.md
+++ b/docs/release-control/v6/internal/subsystems/monitoring.md
@@ -1,12 +1,12 @@
 # Monitoring Contract

 ## Contract Metadata

 ```json
 {
-  "dependency_subsystem_ids": []
+  "dependency_subsystem_ids": ["unified-resources"]
 }
 ```

 ## Purpose
"""
        self.assertFalse(contract_patch_has_substantive_change(patch))

    def test_contract_patch_current_state_change_is_substantive(self):
        patch = """diff --git a/docs/release-control/v6/internal/subsystems/monitoring.md b/docs/release-control/v6/internal/subsystems/monitoring.md
index 1111111..2222222 100644
--- a/docs/release-control/v6/internal/subsystems/monitoring.md
+++ b/docs/release-control/v6/internal/subsystems/monitoring.md
@@ -20,8 +20,8 @@
 ## Current State

-Read-state migration remains partial.
+Read-state migration is now complete for storage-backed workload assembly.
"""
        self.assertTrue(contract_patch_has_substantive_change(patch))

    def test_contract_patch_shared_boundaries_change_is_substantive(self):
        patch = """diff --git a/docs/release-control/v6/internal/subsystems/api-contracts.md b/docs/release-control/v6/internal/subsystems/api-contracts.md
index 1111111..2222222 100644
--- a/docs/release-control/v6/internal/subsystems/api-contracts.md
+++ b/docs/release-control/v6/internal/subsystems/api-contracts.md
@@ -18,7 +18,7 @@
 ## Shared Boundaries

-1. `internal/api/resources.go` shared with `unified-resources`: old shared rationale.
+1. `internal/api/resources.go` shared with `unified-resources`: new shared rationale.
"""
        self.assertTrue(contract_patch_has_substantive_change(patch))

    def test_alerts_owned_runtime_has_no_default_fallback(self):
        alerts_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "alerts")
        self.assertTrue(alerts_rule["verification"]["require_explicit_path_policy_coverage"])
        self.assertEqual(unmatched_owned_runtime_files(alerts_rule), [])

    def test_unified_resources_owned_runtime_has_no_default_fallback(self):
        unified_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "unified-resources")
        self.assertTrue(unified_rule["verification"]["require_explicit_path_policy_coverage"])
        self.assertEqual(unmatched_owned_runtime_files(unified_rule), [])

    def test_alerts_frontend_page_uses_explicit_surface_guardrails(self):
        alerts_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "alerts")
        requirements = build_verification_requirements(
            alerts_rule,
            ["frontend-modern/src/pages/Alerts.tsx"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "alerts-frontend-surface",
                    "label": "alerts frontend surface proof",
                    "touched_runtime_files": ["frontend-modern/src/pages/Alerts.tsx"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Alerts/EmailProviderSelect.test.tsx",
                        "frontend-modern/src/components/Alerts/ResourceTable.test.tsx",
                        "frontend-modern/src/components/Alerts/Thresholds/hooks/__tests__/useCollapsedSections.test.ts",
                        "frontend-modern/src/components/Alerts/Thresholds/sections/__tests__/CollapsibleSection.test.tsx",
                        "frontend-modern/src/components/Alerts/WebhookConfig.test.tsx",
                        "frontend-modern/src/components/Alerts/__tests__/BulkEditDialog.test.tsx",
                        "frontend-modern/src/components/Alerts/__tests__/InvestigateAlertButton.test.tsx",
                        "frontend-modern/src/components/Alerts/__tests__/ThresholdsTable.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/OverviewTab.emptystate.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/OverviewTab.timelineerror.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/OverviewTab.total24h.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/helpers.test.ts",
                        "frontend-modern/src/features/alerts/identity.test.ts",
                        "frontend-modern/src/features/alerts/thresholds/__tests__/helpers.test.ts",
                        "frontend-modern/src/pages/__tests__/Alerts.helpers.test.ts",
                    ],
                }
            ],
        )

    def test_test_only_changes_do_not_require_contract_updates(self):
        required = infer_impacted_subsystems(
            [
                "internal/monitoring/monitor_extra_coverage_test.go",
                "frontend-modern/src/components/Alerts/__tests__/ThresholdsTable.test.tsx",
            ]
        )
        self.assertEqual(required, {})

    def test_deleted_runtime_path_still_requires_contract_updates(self):
        required = infer_impacted_subsystems(["internal/monitoring/monitor_metrics.go"])
        self.assertIn("monitoring", required)

    def test_exact_verification_file_counts_as_verification_artifact(self):
        monitoring_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "monitoring")
        requirement = build_verification_requirements(
            monitoring_rule,
            ["internal/monitoring/monitor.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            monitoring_rule,
            requirement,
            ["internal/unifiedresources/code_standards_test.go"],
        )
        self.assertEqual(matches, ["internal/unifiedresources/code_standards_test.go"])

    def test_monitoring_runtime_rejects_arbitrary_same_subsystem_test(self):
        monitoring_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "monitoring")
        requirement = build_verification_requirements(
            monitoring_rule,
            ["internal/monitoring/monitor.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            monitoring_rule,
            requirement,
            ["internal/monitoring/monitor_extra_coverage_test.go"],
        )
        self.assertEqual(matches, [])

    def test_api_contracts_reject_arbitrary_same_subsystem_test(self):
        api_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        requirement = build_verification_requirements(
            api_rule,
            ["internal/api/resources.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            api_rule,
            requirement,
            ["internal/api/stripe_webhook_handlers_test.go"],
        )
        self.assertEqual(matches, [])

    def test_api_contracts_accept_allowed_test_prefix(self):
        api_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        requirement = build_verification_requirements(
            api_rule,
            ["internal/api/resources.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            api_rule,
            requirement,
            ["frontend-modern/src/api/__tests__/alerts.test.ts"],
        )
        self.assertEqual(matches, ["frontend-modern/src/api/__tests__/alerts.test.ts"])

    def test_explicit_coverage_gap_uses_registry_path_policy_requirement(self):
        synthetic_rule = {
            "id": "synthetic",
            "verification": {
                "allow_same_subsystem_tests": False,
                "test_prefixes": [],
                "exact_files": ["synthetic/proof_test.go"],
                "require_explicit_path_policy_coverage": True,
                "path_policies": [
                    {
                        "id": "covered-path",
                        "label": "covered path proof",
                        "match_prefixes": ["synthetic/covered/"],
                        "match_files": [],
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": [],
                        "exact_files": ["synthetic/proof_test.go"],
                    }
                ],
            },
        }

        requirements = build_verification_requirements(
            synthetic_rule,
            ["synthetic/uncovered/runtime.go"],
        )

        self.assertEqual(
            requirements,
            [
                {
                    "id": "missing-path-policy-coverage",
                    "label": "registry path policy coverage",
                    "touched_runtime_files": ["synthetic/uncovered/runtime.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [],
                    "path_policy_gap": True,
                }
            ],
        )

    def test_api_contracts_owned_runtime_has_no_default_fallback(self):
        api_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        self.assertTrue(api_rule["verification"]["require_explicit_path_policy_coverage"])
        self.assertEqual(unmatched_owned_runtime_files(api_rule), [])

    def test_performance_rejects_non_performance_test(self):
        perf_rule = next(
            rule for rule in load_subsystem_rules() if rule["id"] == "performance-and-scalability"
        )
        requirement = build_verification_requirements(
            perf_rule,
            ["pkg/metrics/store.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            perf_rule,
            requirement,
            ["internal/api/router_test.go"],
        )
        self.assertEqual(matches, [])

    def test_performance_owned_runtime_has_no_default_fallback(self):
        perf_rule = next(
            rule for rule in load_subsystem_rules() if rule["id"] == "performance-and-scalability"
        )
        self.assertTrue(perf_rule["verification"]["require_explicit_path_policy_coverage"])
        self.assertEqual(unmatched_owned_runtime_files(perf_rule), [])

    def test_performance_api_slo_uses_explicit_guardrails(self):
        perf_rule = next(
            rule for rule in load_subsystem_rules() if rule["id"] == "performance-and-scalability"
        )
        requirements = build_verification_requirements(
            perf_rule,
            ["internal/api/slo.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "api-history-slo",
                    "label": "API history SLO proof",
                    "touched_runtime_files": ["internal/api/slo.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/slo_bench_test.go",
                    ],
                }
            ],
        )

    def test_monitoring_metrics_hot_path_uses_specific_proof_policy(self):
        monitoring_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "monitoring")
        requirements = build_verification_requirements(
            monitoring_rule,
            ["internal/monitoring/monitor_metrics.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "metrics-hot-path",
                    "label": "monitoring metrics hot-path proof",
                    "touched_runtime_files": ["internal/monitoring/monitor_metrics.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/monitoring/monitor_metrics_chart_batch_bench_test.go",
                        "internal/monitoring/monitor_metrics_slo_test.go",
                    ],
                }
            ],
        )

    def test_frontend_primitive_page_controls_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "frontend-primitives")
        requirements = build_verification_requirements(
            rule,
            ["frontend-modern/src/components/shared/PageControls.tsx"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "page-controls-and-filters",
                    "label": "page controls and filter proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/shared/PageControls.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                        "frontend-modern/src/components/shared/FilterToolbar.test.tsx",
                        "frontend-modern/src/components/shared/PageControls.guardrails.test.ts",
                        "frontend-modern/src/components/shared/PageControls.test.tsx",
                    ],
                }
            ],
        )

    def test_frontend_primitive_settings_shell_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "frontend-primitives")
        requirements = build_verification_requirements(
            rule,
            ["frontend-modern/src/components/Settings/SettingsPageShell.tsx"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "settings-shell-and-framing",
                    "label": "settings shell framing proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/SettingsPageShell.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                    ],
                }
            ],
        )

    def test_frontend_primitive_network_settings_shell_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "frontend-primitives")
        requirements = build_verification_requirements(
            rule,
            ["frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "settings-shell-and-framing",
                    "label": "settings shell framing proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/NetworkSettingsPanel.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                    ],
                }
            ],
        )

    def test_frontend_primitive_security_auth_shell_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "frontend-primitives")
        requirements = build_verification_requirements(
            rule,
            ["frontend-modern/src/components/Settings/SecurityAuthPanel.tsx"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "settings-shell-and-framing",
                    "label": "settings shell framing proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/SecurityAuthPanel.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                    ],
                }
            ],
        )

    def test_frontend_primitive_remaining_settings_shells_use_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "frontend-primitives")
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
        requirements = build_verification_requirements(rule, runtime_files)
        self.assertEqual(
            requirements,
            [
                {
                    "id": "settings-shell-and-framing",
                    "label": "settings shell framing proof",
                    "touched_runtime_files": runtime_files,
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                    ],
                }
            ],
        )

    def test_cloud_paid_entitlement_lease_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/entitlement_lease.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "entitlement-lease-boundary",
                    "label": "hosted entitlement lease proof",
                    "touched_runtime_files": ["pkg/licensing/entitlement_lease.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/cloud_paid_guardrails_test.go",
                        "pkg/licensing/database_source_test.go",
                        "pkg/licensing/entitlement_lease_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_entitlement_lease_rejects_arbitrary_same_subsystem_test(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirement = build_verification_requirements(
            rule,
            ["pkg/licensing/entitlement_lease.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            rule,
            requirement,
            ["pkg/licensing/service_activate_test.go"],
        )
        self.assertEqual(matches, [])

    def test_cloud_paid_hosted_entitlement_service_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["internal/cloudcp/entitlements/service.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "hosted-entitlement-issuer",
                    "label": "hosted entitlement issuer proof",
                    "touched_runtime_files": ["internal/cloudcp/entitlements/service.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/cloudcp/entitlements/service_test.go",
                        "pkg/licensing/entitlement_lease_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_registry_plan_canonicalization_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["internal/cloudcp/registry/registry.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "control-plane-registry-canonicalization",
                    "label": "control-plane registry plan proof",
                    "touched_runtime_files": ["internal/cloudcp/registry/registry.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/cloudcp/registry/registry_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_stripe_plan_resolution_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["internal/cloudcp/stripe/provisioner.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "stripe-plan-resolution",
                    "label": "stripe plan resolution proof",
                    "touched_runtime_files": ["internal/cloudcp/stripe/provisioner.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/cloudcp/stripe/cloud_lifecycle_integration_test.go",
                        "internal/cloudcp/stripe/helpers_test.go",
                        "internal/cloudcp/stripe/msp_lifecycle_integration_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_control_plane_paths_require_cloud_paid_contract(self):
        required = infer_impacted_subsystems(["internal/cloudcp/registry/registry.go"])
        self.assertIn("cloud-paid", required)
        self.assertEqual(
            required["cloud-paid"]["contract"],
            "docs/release-control/v6/internal/subsystems/cloud-paid.md",
        )

    def test_cloud_paid_jwt_claims_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/models.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "jwt-entitlement-claims",
                    "label": "JWT entitlement claim proof",
                    "touched_runtime_files": ["pkg/licensing/models.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/cloud_paid_guardrails_test.go",
                        "pkg/licensing/models_test.go",
                        "pkg/licensing/service_activate_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_activation_grant_bridge_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/activation_types.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "activation-grant-bridge",
                    "label": "activation grant bridge proof",
                    "touched_runtime_files": ["pkg/licensing/activation_types.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/activation_types_test.go",
                        "pkg/licensing/grant_claims_contract_test.go",
                        "pkg/licensing/service_activate_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_billing_state_canonicalization_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/billing_state_normalization.go", "pkg/licensing/database_source.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "billing-state-canonicalization",
                    "label": "billing state canonicalization proof",
                    "touched_runtime_files": [
                        "pkg/licensing/billing_state_normalization.go",
                        "pkg/licensing/database_source.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/billing_state_normalization_test.go",
                        "pkg/licensing/cloud_paid_guardrails_test.go",
                        "pkg/licensing/database_source_test.go",
                        "pkg/licensing/grant_claims_contract_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_runtime_entitlement_surface_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            [
                "pkg/licensing/evaluator.go",
                "pkg/licensing/token_source.go",
                "pkg/licensing/entitlement_payload.go",
                "pkg/licensing/hosted_subscription.go",
            ],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "runtime-entitlement-surface",
                    "label": "runtime entitlement surface proof",
                    "touched_runtime_files": [
                        "pkg/licensing/evaluator.go",
                        "pkg/licensing/token_source.go",
                        "pkg/licensing/entitlement_payload.go",
                        "pkg/licensing/hosted_subscription.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/cloud_paid_guardrails_test.go",
                        "pkg/licensing/entitlement_payload_test.go",
                        "pkg/licensing/evaluator_test.go",
                        "pkg/licensing/hosted_subscription_test.go",
                        "pkg/licensing/token_source_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_cloud_plan_contracts_use_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/features.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "cloud-plan-contracts",
                    "label": "cloud plan limit proof",
                    "touched_runtime_files": ["pkg/licensing/features.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/features_test.go",
                        "pkg/licensing/grant_claims_contract_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_stripe_plan_derivation_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/stripe_subscription.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "stripe-plan-derivation",
                    "label": "stripe plan derivation proof",
                    "touched_runtime_files": ["pkg/licensing/stripe_subscription.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/grant_claims_contract_test.go",
                        "pkg/licensing/stripe_subscription_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_activation_service_runtime_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            [
                "pkg/licensing/service.go",
                "pkg/licensing/grant_refresh.go",
                "pkg/licensing/revocation_poll.go",
            ],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "activation-service-runtime",
                    "label": "activation service runtime proof",
                    "touched_runtime_files": [
                        "pkg/licensing/service.go",
                        "pkg/licensing/grant_refresh.go",
                        "pkg/licensing/revocation_poll.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/cloud_paid_guardrails_test.go",
                        "pkg/licensing/grant_refresh_test.go",
                        "pkg/licensing/revocation_poll_test.go",
                        "pkg/licensing/service_activate_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_license_server_transport_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/license_server_client.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "license-server-transport",
                    "label": "license server transport proof",
                    "touched_runtime_files": ["pkg/licensing/license_server_client.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/license_server_client_test.go",
                        "pkg/licensing/revocation_poll_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_activation_state_persistence_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/persistence.go", "pkg/licensing/activation_store.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "activation-state-persistence",
                    "label": "activation state persistence proof",
                    "touched_runtime_files": [
                        "pkg/licensing/persistence.go",
                        "pkg/licensing/activation_store.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/activation_store_test.go",
                        "pkg/licensing/persistence_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_trial_activation_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/trial_activation.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "hosted-trial-activation",
                    "label": "hosted trial activation proof",
                    "touched_runtime_files": ["pkg/licensing/trial_activation.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/trial_activation_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_feature_and_limit_primitives_use_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/monitored_system_limit.go", "pkg/licensing/feature_map.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "feature-and-limit-primitives",
                    "label": "feature and limit primitive proof",
                    "touched_runtime_files": [
                        "pkg/licensing/monitored_system_limit.go",
                        "pkg/licensing/feature_map.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/capability_aliases_test.go",
                        "pkg/licensing/feature_map_test.go",
                        "pkg/licensing/monitored_system_limit_test.go",
                        "pkg/licensing/user_limit_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_billing_and_entitlement_types_use_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/billing_store.go", "pkg/licensing/subscription_transitions.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "billing-and-entitlement-types",
                    "label": "billing and entitlement type proof",
                    "touched_runtime_files": [
                        "pkg/licensing/billing_store.go",
                        "pkg/licensing/subscription_transitions.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/contract_test.go",
                        "pkg/licensing/entitlement_payload_test.go",
                        "pkg/licensing/grant_claims_contract_test.go",
                        "pkg/licensing/subscription_test.go",
                        "pkg/licensing/trial_start_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_commercial_migration_and_trial_flow_use_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/commercial_migration.go", "pkg/licensing/trial_start.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "commercial-migration-and-trial-flow",
                    "label": "commercial migration and trial flow proof",
                    "touched_runtime_files": [
                        "pkg/licensing/commercial_migration.go",
                        "pkg/licensing/trial_start.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/commercial_migration_test.go",
                        "pkg/licensing/http_test.go",
                        "pkg/licensing/quickstart_credits_test.go",
                        "pkg/licensing/trial_start_test.go",
                        "pkg/licensing/upgrade_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_conversion_pipeline_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/conversion_store.go", "pkg/licensing/metering/aggregator.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "conversion-telemetry-pipeline",
                    "label": "conversion telemetry pipeline proof",
                    "touched_runtime_files": [
                        "pkg/licensing/conversion_store.go",
                        "pkg/licensing/metering/aggregator.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/conversion_api_helpers_test.go",
                        "pkg/licensing/conversion_config_test.go",
                        "pkg/licensing/conversion_events_test.go",
                        "pkg/licensing/conversion_metrics_test.go",
                        "pkg/licensing/conversion_quality_test.go",
                        "pkg/licensing/conversion_recorder_test.go",
                        "pkg/licensing/conversion_store_queryplan_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_public_key_and_build_modes_use_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/public_key.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "public-key-and-build-modes",
                    "label": "public key and build mode proof",
                    "touched_runtime_files": ["pkg/licensing/public_key.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/license_server_client_test.go",
                        "pkg/licensing/public_key_test.go",
                        "pkg/licensing/service_activate_test.go",
                        "pkg/licensing/trial_activation_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_api_boundary_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["internal/api/licensing_handlers.go", "internal/api/payments_webhook_handlers.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "cloud-paid-api-boundary",
                    "label": "cloud paid API boundary proof",
                    "touched_runtime_files": [
                        "internal/api/licensing_handlers.go",
                        "internal/api/payments_webhook_handlers.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/hostedSignup.test.ts",
                        "frontend-modern/src/pages/__tests__/HostedSignup.test.tsx",
                        "internal/api/billing_state_handlers_test.go",
                        "internal/api/contract_test.go",
                        "internal/api/licensing_handlers_auto_migrate_test.go",
                        "internal/api/licensing_handlers_self_hosted_fallback_test.go",
                        "internal/api/stripe_webhook_handlers_additional_test.go",
                        "internal/api/stripe_webhook_handlers_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_has_no_pkg_licensing_catch_all_policy(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        for policy in rule["verification"]["path_policies"]:
            self.assertNotIn(
                "pkg/licensing/",
                policy.get("match_prefixes", []),
                msg="cloud-paid must not regain a package-wide pkg/licensing fallback policy",
            )

    def test_api_backend_runtime_can_use_types_file_as_proof(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        requirement = build_verification_requirements(
            rule,
            ["internal/api/security.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            rule,
            requirement,
            ["frontend-modern/src/types/api.ts"],
        )
        self.assertEqual(matches, ["frontend-modern/src/types/api.ts"])

    def test_stdin_files_strips_empty_lines(self):
        self.assertEqual(
            stdin_files(["internal/api/resources.go\n", "\n", "docs/release-control/v6/internal/SOURCE_OF_TRUTH.md"]),
            [
                "internal/api/resources.go",
                "docs/release-control/v6/internal/SOURCE_OF_TRUTH.md",
            ],
        )

    def test_parse_args_supports_files_from_stdin(self):
        args = parse_args(["--files-from-stdin"])
        self.assertTrue(args.files_from_stdin)

    def test_explicit_coverage_subsystems_have_no_unmatched_runtime_files(self):
        explicit_rules = {
            rule["id"]: rule
            for rule in load_subsystem_rules()
            if rule["verification"].get("require_explicit_path_policy_coverage")
        }
        self.assertEqual(
            set(explicit_rules),
            {
                "agent-lifecycle",
                "ai-runtime",
                "alerts",
                "api-contracts",
                "cloud-paid",
                "deployment-installability",
                "frontend-primitives",
                "monitoring",
                "notifications",
                "organization-settings",
                "patrol-intelligence",
                "performance-and-scalability",
                "relay-runtime",
                "security-privacy",
                "storage-recovery",
                "unified-resources",
            },
        )
        for subsystem_id, rule in explicit_rules.items():
            self.assertEqual(
                unmatched_owned_runtime_files(rule),
                [],
                msg=f"{subsystem_id} has runtime files that still rely on default verification fallback",
            )

    def test_cloud_paid_owned_runtime_files_do_not_resolve_to_pkg_licensing_fallback(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        fallback_matched = [
            rel
            for rel in owned_runtime_files(rule)
            if first_matching_policy_id(rule, rel) == "cloud-runtime-canonicalization"
        ]
        self.assertEqual(
            fallback_matched,
            [],
            msg="cloud-paid runtime files must not resolve to the removed pkg/licensing fallback policy",
        )


if __name__ == "__main__":
    unittest.main()
