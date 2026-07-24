import io
import os
import subprocess
import unittest
from contextlib import redirect_stderr
from pathlib import Path
from unittest.mock import patch

from canonical_completion_guard import (
    CONTRACT_NEUTRAL_OVERRIDE_ENV,
    REPO_ROOT,
    SUBSYSTEM_REGISTRY,
    build_verification_requirements,
    check_staged_contracts,
    contract_texts_have_substantive_change,
    infer_impacted_subsystems,
    is_ignored_runtime_file,
    is_test_or_fixture,
    load_subsystem_rules,
    path_policy_matches,
    parse_args,
    required_contract_updates,
    resolve_diff_base,
    staged_contract_has_substantive_change,
    staged_verification_files_for_requirement,
    stdin_files,
    subsystem_matches_path,
)

WORKSPACE_REPOS_ROOT = REPO_ROOT.parent

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


def split_workspace_path(path: str) -> tuple[Path, str]:
    if ":" not in path:
        return REPO_ROOT, path
    repo_id, rel = path.split(":", 1)
    return WORKSPACE_REPOS_ROOT / repo_id, rel


def qualify_workspace_path(repo_root: Path, rel: str) -> str:
    if repo_root == REPO_ROOT:
        return rel
    return f"{repo_root.name}:{rel}"


def owned_runtime_files(rule: dict) -> list[str]:
    owned: set[str] = set()
    search_roots: set[Path] = set()

    for prefix in rule.get("owned_prefixes", []):
        repo_root, rel_prefix = split_workspace_path(prefix.rstrip("/"))
        root = repo_root / rel_prefix
        if root.exists():
            search_roots.add(root if root.is_dir() else root.parent)
            continue

        parent = root.parent
        while parent != repo_root and not parent.exists():
            parent = parent.parent
        if parent.exists():
            search_roots.add(parent)

    for root in search_roots:
        repo_root = root
        while repo_root != WORKSPACE_REPOS_ROOT and repo_root.parent != WORKSPACE_REPOS_ROOT:
            repo_root = repo_root.parent
        if repo_root.parent != WORKSPACE_REPOS_ROOT:
            repo_root = REPO_ROOT
        for path in root.rglob("*"):
            if not path.is_file():
                continue
            rel = path.relative_to(repo_root).as_posix()
            candidate = qualify_workspace_path(repo_root, rel)
            if is_test_or_fixture(candidate) or is_ignored_runtime_file(candidate):
                continue
            if subsystem_matches_path(rule, candidate):
                owned.add(candidate)

    for rel in rule.get("owned_files", []):
        repo_root, repo_rel = split_workspace_path(rel)
        path = repo_root / repo_rel
        if not path.exists() or not path.is_file():
            continue
        candidate = qualify_workspace_path(repo_root, repo_rel)
        if is_test_or_fixture(candidate) or is_ignored_runtime_file(candidate):
            continue
        if subsystem_matches_path(rule, candidate):
            owned.add(candidate)

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


RECOVERY_PRODUCT_SURFACE_EXACT_FILES = [
    "frontend-modern/src/components/Recovery/__tests__/Recovery.test.tsx",
    "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
    "tests/integration/tests/17-recovery-layout.spec.ts",
]


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

    def test_monitoring_runtime_change_requires_monitoring_and_agent_lifecycle_contracts(
        self,
    ):
        required = infer_impacted_subsystems(["internal/monitoring/monitor.go"])
        self.assertEqual(set(required), {"agent-lifecycle", "monitoring"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["internal/monitoring/monitor.go"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "host-agent-server-lifecycle",
                    "label": "server-side host-agent deletion and re-enrollment lifecycle proof",
                    "touched_runtime_files": ["internal/monitoring/monitor.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/host_agent_removal_lifecycle_integration_test.go",
                        "internal/config/host_continuity_test.go",
                        "internal/models/metrics_types_test.go",
                        "internal/monitoring/monitor_host_agent_removal_lifecycle_test.go",
                        "internal/monitoring/monitor_host_agents_test.go",
                        "scripts/installtests/agent_state_dir_lifecycle_test.go",
                        "scripts/installtests/install_ps1_test.go",
                        "scripts/installtests/install_sh_test.go",
                    ],
                }
            ],
        )

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
                "host-agent-ingest-runtime",
                "truenas-runtime",
                "metrics-hot-path",
                "metrics-history-runtime",
                "storage-risk-runtime",
                "memory-source-runtime",
                "docker-collect-runtime",
                "docker-swarm-runtime",
                "kubernetes-native-agent-runtime",
                "runtime-report-model",
                "proxmox-guest-counter-runtime",
                "proxmox-zfs-runtime",
                "proxmox-cluster-client-runtime",
                "proxmox-ceph-runtime",
                "proxmox-backup-identity-monitoring",
                "container-entrypoint-runtime",
                "mock-runtime-fixtures",
                "pbs-protection-evidence-runtime",
                "diskinventory-collection-trust",
                "agent-fleet-diagnostics-runtime",
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
                        "internal/config/docker_metadata_test.go",
                        "internal/config/guest_metadata_test.go",
                        "internal/monitoring/availability_poller_test.go",
                        "internal/monitoring/availability_udp_test.go",
                        "internal/monitoring/canonical_guardrails_test.go",
                        "internal/monitoring/ceph_test.go",
                        "internal/monitoring/issue1485_unraid_lifecycle_test.go",
                        "internal/monitoring/issue1595_collection_trust_test.go",
                        "internal/monitoring/issue1613_contract_test.go",
                        "internal/monitoring/monitor_additional_test.go",
                        "internal/monitoring/monitor_alert_intent_test.go",
                        "internal/monitoring/monitor_alert_override_migration_test.go",
                        "internal/monitoring/monitor_backups_readstate_test.go",
                        "internal/monitoring/monitor_docker_test.go",
                        "internal/monitoring/monitor_host_agent_removal_lifecycle_test.go",
                        "internal/monitoring/monitor_host_agents_test.go",
                        "internal/monitoring/monitor_pve_cluster_refresh_test.go",
                        "internal/monitoring/monitor_pve_guest_lxc_test.go",
                        "internal/monitoring/ratetracker_test.go",
                        "internal/monitoring/truenas_poller_test.go",
                        "internal/unifiedresources/code_standards_test.go",
                    ],
                }
            ],
        )

    def test_docker_collect_runtime_change_requires_monitoring_contract(self):
        required = infer_impacted_subsystems(["internal/dockeragent/collect.go"])
        self.assertEqual(set(required), {"monitoring"})

        monitoring = required["monitoring"]
        self.assertEqual(
            monitoring["contract"],
            "docs/release-control/v6/internal/subsystems/monitoring.md",
        )
        self.assertEqual(
            monitoring["touched_runtime_files"],
            ["internal/dockeragent/collect.go"],
        )
        self.assertEqual(
            monitoring["verification_requirements"],
            [
                {
                    "id": "docker-collect-runtime",
                    "label": "docker and podman container collection proof",
                    "touched_runtime_files": ["internal/dockeragent/collect.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/dockeragent/agent_collect_test.go",
                        "internal/dockeragent/agent_cpu_test.go",
                        "internal/dockeragent/agent_internal_test.go",
                        "internal/dockeragent/swarm_coverage_test.go",
                    ],
                }
            ],
        )

    def test_proxmox_ceph_runtime_change_requires_monitoring_contract(self):
        required = infer_impacted_subsystems(["pkg/proxmox/ceph.go"])
        self.assertEqual(set(required), {"monitoring"})

        monitoring = required["monitoring"]
        self.assertEqual(
            monitoring["contract"],
            "docs/release-control/v6/internal/subsystems/monitoring.md",
        )
        self.assertEqual(
            monitoring["touched_runtime_files"],
            ["pkg/proxmox/ceph.go"],
        )
        self.assertEqual(
            monitoring["verification_requirements"],
            [
                {
                    "id": "proxmox-ceph-runtime",
                    "label": "proxmox ceph compatibility proof",
                    "touched_runtime_files": ["pkg/proxmox/ceph.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/proxmox/ceph_test.go",
                        "pkg/proxmox/cluster_client_api_test.go",
                    ],
                }
            ],
        )

    def test_host_agent_ingest_runtime_change_requires_monitoring_and_agent_lifecycle_contracts(
        self,
    ):
        required = infer_impacted_subsystems(["internal/monitoring/monitor_agents.go"])
        self.assertEqual(set(required), {"agent-lifecycle", "monitoring"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["internal/monitoring/monitor_agents.go"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "host-agent-server-lifecycle",
                    "label": "server-side host-agent deletion and re-enrollment lifecycle proof",
                    "touched_runtime_files": [
                        "internal/monitoring/monitor_agents.go"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/host_agent_removal_lifecycle_integration_test.go",
                        "internal/config/host_continuity_test.go",
                        "internal/models/metrics_types_test.go",
                        "internal/monitoring/monitor_host_agent_removal_lifecycle_test.go",
                        "internal/monitoring/monitor_host_agents_test.go",
                        "scripts/installtests/agent_state_dir_lifecycle_test.go",
                        "scripts/installtests/install_ps1_test.go",
                        "scripts/installtests/install_sh_test.go",
                    ],
                }
            ],
        )

        monitoring = required["monitoring"]
        self.assertEqual(
            monitoring["contract"],
            "docs/release-control/v6/internal/subsystems/monitoring.md",
        )
        self.assertEqual(
            monitoring["touched_runtime_files"],
            ["internal/monitoring/monitor_agents.go"],
        )
        self.assertEqual(
            monitoring["verification_requirements"],
            [
                {
                    "id": "host-agent-ingest-runtime",
                    "label": "monitoring host-agent ingest proof",
                    "touched_runtime_files": ["internal/monitoring/monitor_agents.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/config/host_continuity_test.go",
                        "internal/monitoring/issue1485_unraid_lifecycle_test.go",
                        "internal/monitoring/issue1595_collection_trust_test.go",
                        "internal/monitoring/monitor_docker_test.go",
                        "internal/monitoring/monitor_host_agent_removal_lifecycle_test.go",
                        "internal/monitoring/monitor_host_agents_test.go",
                        "internal/monitoring/monitor_package_updates_test.go",
                        "internal/unifiedresources/code_standards_test.go",
                    ],
                }
            ],
        )

    def test_storage_risk_runtime_change_requires_monitoring_contract(self):
        required = infer_impacted_subsystems(["internal/storagehealth/topology.go"])
        self.assertEqual(set(required), {"monitoring"})

        monitoring = required["monitoring"]
        self.assertEqual(
            monitoring["contract"],
            "docs/release-control/v6/internal/subsystems/monitoring.md",
        )
        self.assertEqual(
            monitoring["touched_runtime_files"],
            ["internal/storagehealth/topology.go"],
        )
        self.assertEqual(
            monitoring["verification_requirements"],
            [
                {
                    "id": "storage-risk-runtime",
                    "label": "monitoring storage-risk proof",
                    "touched_runtime_files": ["internal/storagehealth/topology.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/resources_test.go",
                        "internal/storagehealth/risk_test.go",
                        "internal/storagehealth/topology_test.go",
                        "internal/storagehealth/zfs_pool_health_contract_test.go",
                    ],
                }
            ],
        )

    def test_diskinventory_runtime_uses_collection_trust_policy(self):
        required = infer_impacted_subsystems(
            [
                "pkg/diskinventory/identity.go",
                "pkg/diskinventory/status.go",
            ]
        )
        self.assertEqual(set(required), {"monitoring"})

        monitoring = required["monitoring"]
        self.assertEqual(
            monitoring["contract"],
            "docs/release-control/v6/internal/subsystems/monitoring.md",
        )
        self.assertEqual(
            monitoring["verification_requirements"],
            [
                {
                    "id": "diskinventory-collection-trust",
                    "label": "physical-disk collection trust and identity proof",
                    "touched_runtime_files": [
                        "pkg/diskinventory/identity.go",
                        "pkg/diskinventory/status.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/hostagent/issue1595_sas_collection_test.go",
                        "internal/monitoring/issue1595_collection_trust_test.go",
                        "pkg/diskinventory/identity_test.go",
                        "pkg/diskinventory/status_test.go",
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
                        "scripts/installtests/agent_state_dir_lifecycle_test.go",
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
                        "scripts/installtests/agent_state_dir_lifecycle_test.go",
                        "scripts/installtests/install_sh_test.go",
                    ],
                }
            ],
        )

    def test_mobile_relay_runtime_change_requires_relay_contract(self):
        required = infer_impacted_subsystems(["pulse-mobile:src/relay/client.ts"])
        self.assertEqual(set(required), {"relay-runtime"})

        relay = required["relay-runtime"]
        self.assertEqual(
            relay["contract"],
            "docs/release-control/v6/internal/subsystems/relay-runtime.md",
        )
        self.assertEqual(
            relay["touched_runtime_files"],
            ["pulse-mobile:src/relay/client.ts"],
        )
        self.assertEqual(
            relay["verification_requirements"],
            [
                {
                    "id": "mobile-relay-runtime",
                    "label": "mobile relay runtime proof",
                    "touched_runtime_files": ["pulse-mobile:src/relay/client.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pulse-mobile:src/relay/__tests__/channel.test.ts",
                        "pulse-mobile:src/relay/__tests__/client-hardening.test.ts",
                        "pulse-mobile:src/relay/__tests__/client.test.ts",
                        "pulse-mobile:src/relay/__tests__/encryption.test.ts",
                        "pulse-mobile:src/relay/__tests__/identity.test.ts",
                        "pulse-mobile:src/relay/__tests__/protocol-contract.test.ts",
                        "pulse-mobile:src/relay/__tests__/protocol.test.ts",
                        "pulse-mobile:src/relay/__tests__/proxy.test.ts",
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
                        "scripts/installtests/install_sh_test.go",
                        "scripts/installtests/pulse_auto_update_test.go",
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
                        "internal/hostagent/agent_flushbuffer_test.go",
                        "internal/hostagent/agent_metrics_test.go",
                        "internal/hostagent/agent_new_test.go",
                        "internal/hostagent/command_client_test.go",
                        "internal/hostagent/commands_deploy_test.go",
                        "internal/hostagent/commands_host_update_test.go",
                        "internal/hostagent/commands_storage_cleanup_test.go",
                        "internal/hostagent/docker_lifecycle_test.go",
                        "internal/hostagent/issue1595_sas_collection_test.go",
                        "internal/hostagent/observer_delivery_test.go",
                        "internal/hostagent/package_updates_test.go",
                        "internal/hostagent/send_report_test.go",
                        "internal/hostagent/smartctl_standby_guard_test.go",
                        "internal/hostagent/storage_cleanup_test.go",
                        "internal/hostagent/unraid_test.go",
                        "pkg/agents/host/report_test.go",
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
                        "scripts/installtests/install_sh_test.go",
                        "scripts/installtests/pulse_auto_update_test.go",
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
                        "scripts/installtests/install_sh_test.go",
                        "scripts/installtests/pulse_auto_update_test.go",
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
                        "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
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

    def test_organization_client_change_requires_organization_settings_and_api_contracts(self):
        required = infer_impacted_subsystems(["frontend-modern/src/api/orgs.ts"])
        self.assertEqual(set(required), {"api-contracts", "organization-settings"})

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/api/orgs.ts"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "frontend-api-clients",
                    "label": "frontend API client proof",
                    "touched_runtime_files": ["frontend-modern/src/api/orgs.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": ["frontend-modern/src/api/__tests__/"],
                    "exact_files": ["frontend-modern/src/types/api.ts"],
                }
            ],
        )

        organization_settings = required["organization-settings"]
        self.assertEqual(
            organization_settings["contract"],
            "docs/release-control/v6/internal/subsystems/organization-settings.md",
        )
        self.assertEqual(
            organization_settings["touched_runtime_files"],
            ["frontend-modern/src/api/orgs.ts"],
        )
        self.assertEqual(
            organization_settings["verification_requirements"],
            [
                {
                    "id": "organization-api-clients",
                    "label": "organization and RBAC API client proof",
                    "touched_runtime_files": ["frontend-modern/src/api/orgs.ts"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/orgs.test.ts",
                        "frontend-modern/src/api/__tests__/rbac.test.ts",
                    ],
                }
            ],
        )

    def test_organization_rbac_backend_change_requires_organization_settings_and_api_contracts(self):
        required = infer_impacted_subsystems(["internal/api/access_control_handlers.go"])
        self.assertEqual(set(required), {"api-contracts", "organization-settings"})

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["contract"],
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
        )
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["internal/api/access_control_handlers.go"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "backend-payload-contracts",
                    "label": "backend API payload proof",
                    "touched_runtime_files": ["internal/api/access_control_handlers.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": ["frontend-modern/src/api/__tests__/"],
                    "exact_files": [
                        "frontend-modern/src/types/api.ts",
                        "internal/api/ai_handlers_more_test.go",
                        "internal/api/ai_handlers_patrol_actions_additional_test.go",
                        "internal/api/audit_handlers_test.go",
                        "internal/api/contract_test.go",
                        "internal/api/docker_agents_report_size_test.go",
                        "internal/api/host_agent_removal_lifecycle_integration_test.go",
                        "internal/api/metadata_handlers_test.go",
                        "internal/api/patrol_autopilot_test.go",
                        "pulse-enterprise:test/extensions_contract_test.go",
                    ],
                }
            ],
        )

        organization_settings = required["organization-settings"]
        self.assertEqual(
            organization_settings["contract"],
            "docs/release-control/v6/internal/subsystems/organization-settings.md",
        )
        self.assertEqual(
            organization_settings["touched_runtime_files"],
            ["internal/api/access_control_handlers.go"],
        )
        self.assertEqual(
            organization_settings["verification_requirements"],
            [
                {
                    "id": "organization-rbac-transport",
                    "label": "organization RBAC backend transport proof",
                    "touched_runtime_files": ["internal/api/access_control_handlers.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/api/enterprise_extension_rbac_admin_test.go",
                        "internal/api/rbac_admin_handlers_test.go",
                        "internal/api/rbac_handlers_additional_test.go",
                        "internal/api/rbac_handlers_more_test.go",
                        "internal/api/rbac_handlers_test.go",
                    ],
                }
            ],
        )

    def test_dashboard_page_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(["frontend-modern/src/pages/Dashboard.tsx"])
        self.assertEqual(required, {})

    def test_dashboard_widgets_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/features/dashboardOverview/dashboardWidgets.ts"]
        )
        self.assertEqual(required, {})

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

    def test_dashboard_recovery_hook_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(["frontend-modern/src/hooks/useDashboardRecovery.ts"])
        self.assertEqual(required, {})

    def test_dashboard_recovery_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/dashboardRecoveryPresentation.ts"]
        )
        self.assertEqual(required, {})

    def test_dashboard_storage_presentation_change_requires_storage_recovery_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/dashboardStoragePresentation.ts"]
        )
        self.assertEqual(required, {})

    def test_storage_surface_change_requires_storage_recovery_contract(self):
        # The legacy /storage route shell (pages/Storage.tsx) was retired
        # with the platform-first migration; the canonical Storage surface
        # now lives at components/Storage/Storage.tsx and is embedded inside
        # platform pages.
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/Storage/Storage.tsx"]
        )
        self.assertEqual(set(required), {"storage-recovery"})

        recovery = required["storage-recovery"]
        self.assertEqual(
            recovery["contract"],
            "docs/release-control/v6/internal/subsystems/storage-recovery.md",
        )
        self.assertEqual(
            recovery["touched_runtime_files"],
            ["frontend-modern/src/components/Storage/Storage.tsx"],
        )
        self.assertEqual(
            recovery["verification_requirements"][0]["id"],
            "storage-product-surface",
        )

    def test_retired_recovery_summary_presentation_stays_unowned(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/recoverySummaryPresentation.ts"]
        )
        self.assertEqual(required, {})

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
                        "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
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
                        "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
                        "frontend-modern/src/utils/__tests__/agentProfilesPresentation.test.ts",
                    ],
                }
            ],
        )

    def test_infrastructure_install_state_change_requires_lifecycle_and_api_contracts(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx"]
        )
        self.assertEqual(set(required), {"agent-lifecycle", "api-contracts"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "unified-agent-settings-surface",
                    "label": "unified agent settings lifecycle proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/api/__tests__/monitoring.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/InfrastructureOperationsModel.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                    ],
                }
            ],
        )

        api_contracts = required["api-contracts"]
        self.assertEqual(
            api_contracts["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx"],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "unified-agent-settings-surface",
                    "label": "infrastructure operations API proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/api/__tests__/monitoring.test.ts",
                        "frontend-modern/src/api/__tests__/security.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/InfrastructureOperationsModel.test.tsx",
                    ],
                }
            ],
        )

    def _assert_platform_connections_workspace_change_requires_agent_lifecycle(
        self, touched_path: str
    ) -> None:
        required = infer_impacted_subsystems([touched_path])
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(lifecycle["touched_runtime_files"], [touched_path])
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "platform-connections-workspace-surface",
                    "label": "platform connections workspace lifecycle proof",
                    "touched_runtime_files": [touched_path],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": PLATFORM_CONNECTIONS_WORKSPACE_EXACT_FILES,
                }
            ],
        )

    def _assert_systems_ledger_change_requires_agent_lifecycle(self, touched_path: str) -> None:
        required = infer_impacted_subsystems([touched_path])
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(lifecycle["touched_runtime_files"], [touched_path])
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "systems-ledger-workspace-surface",
                    "label": "systems ledger lifecycle proof",
                    "touched_runtime_files": [touched_path],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": CONNECTIONS_LEDGER_WORKSPACE_EXACT_FILES,
                }
            ],
        )

    def test_connections_table_model_change_requires_agent_lifecycle(self):
        self._assert_systems_ledger_change_requires_agent_lifecycle(
            "frontend-modern/src/components/Settings/connectionsTableModel.ts"
        )

    def test_infrastructure_installer_section_change_requires_agent_lifecycle(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx"]
        )
        self.assertEqual(set(required), {"agent-lifecycle"})

        lifecycle = required["agent-lifecycle"]
        self.assertEqual(
            lifecycle["contract"],
            "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
        )
        self.assertEqual(
            lifecycle["touched_runtime_files"],
            ["frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx"],
        )
        self.assertEqual(
            lifecycle["verification_requirements"],
            [
                {
                    "id": "unified-agent-settings-surface",
                    "label": "unified agent settings lifecycle proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/api/__tests__/agentProfiles.test.ts",
                        "frontend-modern/src/api/__tests__/monitoring.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/InfrastructureOperationsModel.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
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

    def _assert_api_token_manager_change_requires_security_and_api_contracts(
        self, touched_path: str
    ) -> None:
        required = infer_impacted_subsystems([touched_path])
        self.assertEqual(set(required), {"security-privacy", "api-contracts"})

        security = required["security-privacy"]
        self.assertEqual(
            security["contract"],
            "docs/release-control/v6/internal/subsystems/security-privacy.md",
        )
        self.assertEqual(
            security["touched_runtime_files"],
            [touched_path],
        )
        self.assertEqual(
            security["verification_requirements"],
            [
                {
                    "id": "security-settings-surfaces",
                    "label": "security settings surface proof",
                    "touched_runtime_files": [touched_path],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/APITokenManager.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/SecurityPostureSummary.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                        "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
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
            [touched_path],
        )
        self.assertEqual(
            api_contracts["verification_requirements"],
            [
                {
                    "id": "api-token-management-surface",
                    "label": "API token management surface proof",
                    "touched_runtime_files": [touched_path],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/Settings/__tests__/APITokenManager.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                        "frontend-modern/src/components/shared/SharedPrimitives.guardrails.test.ts",
                    ],
                }
            ],
        )

    def test_api_token_manager_change_requires_security_and_api_contracts(self):
        self._assert_api_token_manager_change_requires_security_and_api_contracts(
            "frontend-modern/src/components/Settings/APITokenManager.tsx"
        )

    def test_api_token_manager_model_change_requires_security_and_api_contracts(self):
        self._assert_api_token_manager_change_requires_security_and_api_contracts(
            "frontend-modern/src/components/Settings/apiTokenManagerModel.ts"
        )

    def test_api_token_manager_state_change_requires_security_and_api_contracts(self):
        self._assert_api_token_manager_change_requires_security_and_api_contracts(
            "frontend-modern/src/components/Settings/useAPITokenManagerState.ts"
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
                        "internal/api/websocket_origin_security_test.go",
                        "internal/websocket/hub_test.go",
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
        self.assertEqual(
            required["docs/release-control/v6/internal/subsystems/monitoring.md"]["matched_reference_details"],
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

    def test_monitoring_owned_runtime_requires_each_shared_owner_contract(self):
        required = required_contract_updates(["internal/monitoring/monitor.go"])
        self.assertEqual(
            required,
            {
                "docs/release-control/v6/internal/subsystems/agent-lifecycle.md": {
                    "subsystem": "agent-lifecycle",
                    "contract": "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
                    "reason": "owner",
                    "touched_runtime_files": ["internal/monitoring/monitor.go"],
                    "matched_references": [],
                },
                "docs/release-control/v6/internal/subsystems/monitoring.md": {
                    "subsystem": "monitoring",
                    "contract": "docs/release-control/v6/internal/subsystems/monitoring.md",
                    "reason": "owner",
                    "touched_runtime_files": ["internal/monitoring/monitor.go"],
                    "matched_references": [],
                }
            },
        )

    def test_contract_metadata_only_change_is_not_substantive(self):
        base = """# Monitoring Contract

## Contract Metadata

```json
{
  "dependency_subsystem_ids": []
}
```

## Purpose

Watch things.
"""
        staged = base.replace(
            '"dependency_subsystem_ids": []',
            '"dependency_subsystem_ids": ["unified-resources"]',
        )
        self.assertFalse(contract_texts_have_substantive_change(base, staged))

    def test_contract_current_state_change_is_substantive(self):
        base = """# Monitoring Contract

## Purpose

Watch things.

## Current State

Read-state migration remains partial.
"""
        staged = base.replace(
            "Read-state migration remains partial.",
            "Read-state migration is now complete for storage-backed workload assembly.",
        )
        self.assertTrue(contract_texts_have_substantive_change(base, staged))

    def test_contract_shared_boundaries_change_is_substantive(self):
        base = """# API Contracts Contract

## Shared Boundaries

1. `internal/api/resources.go` shared with `unified-resources`: old shared rationale.
"""
        staged = base.replace("old shared rationale.", "new shared rationale.")
        self.assertTrue(contract_texts_have_substantive_change(base, staged))

    def test_contract_current_state_edit_far_below_section_header_is_substantive(self):
        # Regression: the old implementation attributed sections from
        # `git diff --unified=1000` context, so an edit more than 1000 lines
        # below its `## ` header lost its section and was wrongly reported
        # as not substantive (unified-resources.md's Current State spans
        # thousands of lines).
        filler = "\n".join(f"- historical note {i} about the registry." for i in range(1500))
        base = f"""# Unified Resources Contract

## Contract Metadata

```json
{{
  "dependency_subsystem_ids": []
}}
```

## Current State

{filler}

Registry rebuilds still emit no-op relationship_change events.

## Extension Points

None yet.
"""
        staged = base.replace(
            "Registry rebuilds still emit no-op relationship_change events.",
            "Registry rebuilds now skip no-op relationship_change events.",
        )
        self.assertTrue(contract_texts_have_substantive_change(base, staged))

    def test_contract_metadata_edit_in_file_with_long_current_state_stays_non_substantive(self):
        # Counterpart to the deep-edit regression test: full-file attribution
        # must not over-report either — a metadata-only tweak in a file whose
        # Current State is huge is still not substantive.
        filler = "\n".join(f"- historical note {i} about the registry." for i in range(1500))
        base = f"""# Unified Resources Contract

## Contract Metadata

```json
{{
  "dependency_subsystem_ids": []
}}
```

## Current State

{filler}
"""
        staged = base.replace(
            '"dependency_subsystem_ids": []',
            '"dependency_subsystem_ids": ["monitoring"]',
        )
        self.assertFalse(contract_texts_have_substantive_change(base, staged))

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
                        "frontend-modern/src/features/alerts/AlertDeliveryHealthCard.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/AlertIntentPolicyPanel.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/OverviewTab.emptystate.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/OverviewTab.timelineerror.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/OverviewTab.total24h.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/ThresholdsTab.test.tsx",
                        "frontend-modern/src/features/alerts/__tests__/alertsConfigurationModel.test.ts",
                        "frontend-modern/src/features/alerts/__tests__/helpers.test.ts",
                        "frontend-modern/src/features/alerts/__tests__/useAlertDestinationsTabState.test.tsx",
                        "frontend-modern/src/features/alerts/identity.test.ts",
                        "frontend-modern/src/features/alerts/thresholds/__tests__/helpers.test.ts",
                        "frontend-modern/src/features/alerts/thresholds/hooks/__tests__/truenasThresholdPersistence.test.tsx",
                        "frontend-modern/src/features/alerts/thresholds/hooks/__tests__/useThresholdsTableState.test.tsx",
                        "frontend-modern/src/pages/__tests__/Alerts.helpers.test.ts",
                        "frontend-modern/src/utils/__tests__/alertOverviewPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/alertTargetTypes.test.ts",
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

    def test_patrol_autopilot_runtime_accepts_only_dedicated_verification(self):
        rules = {rule["id"]: rule for rule in load_subsystem_rules()}
        cases = [
            (
                "ai-runtime",
                "internal/api/ai_handlers.go",
                "ai-api-surface",
                "internal/api/patrol_autopilot_test.go",
            ),
            (
                "api-contracts",
                "internal/api/ai_handlers.go",
                "backend-payload-contracts",
                "internal/api/patrol_autopilot_test.go",
            ),
            (
                "ai-runtime",
                "internal/config/ai.go",
                "ai-runtime-config",
                "internal/config/patrol_autopilot_persistence_test.go",
            ),
            (
                "ai-runtime",
                "internal/config/patrol_autopilot_persistence.go",
                "ai-runtime-config",
                "internal/config/patrol_autopilot_persistence_test.go",
            ),
            (
                "unified-resources",
                "internal/unifiedresources/patrol_autopilot.go",
                "patrol-autopilot-runtime",
                "internal/unifiedresources/patrol_autopilot_test.go",
            ),
        ]

        for subsystem_id, runtime_path, requirement_id, proof_path in cases:
            with self.subTest(subsystem=subsystem_id, runtime=runtime_path):
                rule = rules[subsystem_id]
                requirement = next(
                    requirement
                    for requirement in build_verification_requirements(rule, [runtime_path])
                    if requirement["id"] == requirement_id
                )
                self.assertEqual(
                    staged_verification_files_for_requirement(
                        rule,
                        requirement,
                        [proof_path],
                    ),
                    [proof_path],
                )
                self.assertEqual(
                    staged_verification_files_for_requirement(rule, requirement, []),
                    [],
                )

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

    def test_remediation_presentation_change_requires_patrol_contract(self):
        required = infer_impacted_subsystems(
            ["frontend-modern/src/utils/remediationPresentation.ts"]
        )
        self.assertEqual(set(required), {"patrol-intelligence"})

        patrol = required["patrol-intelligence"]
        self.assertEqual(
            patrol["contract"],
            "docs/release-control/v6/internal/subsystems/patrol-intelligence.md",
        )
        self.assertEqual(
            patrol["touched_runtime_files"],
            ["frontend-modern/src/utils/remediationPresentation.ts"],
        )
        self.assertEqual(
            patrol["verification_requirements"],
            [
                {
                    "id": "patrol-findings-and-approvals",
                    "label": "patrol findings and approvals proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/utils/remediationPresentation.ts"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/AI/__tests__/FindingsPanel.test.ts",
                        "frontend-modern/src/components/patrol/__tests__/ApprovalSection.test.tsx",
                        "frontend-modern/src/components/patrol/__tests__/InvestigationSection.test.tsx",
                        "frontend-modern/src/utils/__tests__/approvalRiskPresentation.test.ts",
                        "frontend-modern/src/utils/__tests__/approvalState.test.ts",
                        "frontend-modern/src/utils/__tests__/findingAlertIdentity.test.ts",
                        "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
                        "frontend-modern/src/utils/__tests__/remediationPresentation.test.ts",
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
                        "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/useAuditLogPanelState.test.tsx",
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
                        "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/useAuditLogPanelState.test.tsx",
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
                        "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/useAuditLogPanelState.test.tsx",
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
                        "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/useAuditLogPanelState.test.tsx",
                    ],
                }
            ],
        )

    def test_frontend_primitive_settings_diagnostics_boundary_audit_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "frontend-primitives")
        requirements = build_verification_requirements(
            rule,
            ["frontend-modern/scripts/settings-diagnostics-boundary-audit.mjs"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "settings-diagnostics-boundary-audit",
                    "label": "settings diagnostics internal analytics boundary proof",
                    "touched_runtime_files": [
                        "frontend-modern/scripts/settings-diagnostics-boundary-audit.mjs"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/scripts/__tests__/settings-diagnostics-boundary-audit.test.mjs",
                        "frontend-modern/src/components/Settings/__tests__/DiagnosticsResultsPanel.test.tsx",
                        "frontend-modern/src/components/Settings/__tests__/diagnosticsModel.test.ts",
                        "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                    ],
                }
            ],
        )

    def _assert_environment_lock_boundary_requires_frontend_primitives(
        self, touched_path: str, requirement_id: str, exact_files: list[str]
    ) -> None:
        required = infer_impacted_subsystems([touched_path])
        self.assertEqual(set(required), {"frontend-primitives"})

        frontend = required["frontend-primitives"]
        self.assertEqual(
            frontend["contract"],
            "docs/release-control/v6/internal/subsystems/frontend-primitives.md",
        )
        self.assertEqual(frontend["touched_runtime_files"], [touched_path])
        self.assertEqual(
            frontend["verification_requirements"],
            [
                {
                    "id": requirement_id,
                    "label": (
                        "settings shell framing proof"
                        if requirement_id == "settings-shell-and-framing"
                        else "environment lock primitive proof"
                    ),
                    "touched_runtime_files": [touched_path],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": exact_files,
                }
            ],
        )

    def test_docker_runtime_settings_card_change_requires_frontend_primitives(self):
        self._assert_environment_lock_boundary_requires_frontend_primitives(
            "frontend-modern/src/components/Settings/DockerRuntimeSettingsCard.tsx",
            "settings-shell-and-framing",
            [
                "frontend-modern/src/components/Settings/__tests__/dataHandlingPanelModel.test.ts",
                "frontend-modern/src/components/Settings/__tests__/settingsArchitecture.test.ts",
                "frontend-modern/src/components/Settings/__tests__/useAuditLogPanelState.test.tsx",
            ],
        )

    def test_environment_lock_badge_change_requires_frontend_primitives(self):
        self._assert_environment_lock_boundary_requires_frontend_primitives(
            "frontend-modern/src/components/shared/EnvironmentLockBadge.tsx",
            "environment-lock-primitives",
            [
                "frontend-modern/src/utils/__tests__/environmentLockPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
            ],
        )

    def test_environment_lock_presentation_change_requires_frontend_primitives(self):
        self._assert_environment_lock_boundary_requires_frontend_primitives(
            "frontend-modern/src/utils/environmentLockPresentation.ts",
            "environment-lock-primitives",
            [
                "frontend-modern/src/utils/__tests__/environmentLockPresentation.test.ts",
                "frontend-modern/src/utils/__tests__/frontendResourceTypeBoundaries.test.ts",
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
                        "pkg/licensing/installation_status_poll_test.go",
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
                        "pkg/licensing/installation_status_poll_test.go",
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
                "pkg/licensing/installation_status_poll.go",
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
                        "pkg/licensing/installation_status_poll.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "pkg/licensing/cloud_paid_guardrails_test.go",
                        "pkg/licensing/grant_refresh_test.go",
                        "pkg/licensing/installation_status_poll_test.go",
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
                        "pkg/licensing/installation_status_poll_test.go",
                        "pkg/licensing/license_server_client_test.go",
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
                    "label": "hosted entitlement lease signing proof",
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
                        "pkg/licensing/commercial_migration_load_test.go",
                        "pkg/licensing/commercial_migration_test.go",
                        "pkg/licensing/http_test.go",
                        "pkg/licensing/quickstart_credits_test.go",
                        "pkg/licensing/trial_start_test.go",
                        "pkg/licensing/upgrade_test.go",
                    ],
                }
            ],
        )

    def test_cloud_paid_conversion_pipeline_stays_retired_without_policy_coverage(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "cloud-paid")
        requirements = build_verification_requirements(
            rule,
            ["pkg/licensing/conversion_store.go", "pkg/licensing/metering/aggregator.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "missing-path-policy-coverage",
                    "label": "registry path policy coverage",
                    "touched_runtime_files": [
                        "pkg/licensing/conversion_store.go",
                        "pkg/licensing/metering/aggregator.go",
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [],
                    "path_policy_gap": True,
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
        self.assertIsNone(args.diff_base)

    def test_parse_args_supports_diff_base_with_files_from_stdin(self):
        args = parse_args(["--files-from-stdin", "--diff-base", "0d787a4b1"])
        self.assertTrue(args.files_from_stdin)
        self.assertEqual(args.diff_base, "0d787a4b1")

    def test_parse_args_rejects_diff_base_without_files_from_stdin(self):
        stderr = io.StringIO()
        with redirect_stderr(stderr), self.assertRaises(SystemExit):
            parse_args(["--diff-base", "0d787a4b1"])
        self.assertIn("--diff-base requires --files-from-stdin", stderr.getvalue())

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


class ContractNeutralOverrideTest(unittest.TestCase):
    def test_check_staged_contracts_returns_zero_when_override_set(self):
        stderr = io.StringIO()
        with (
            patch.dict(
                os.environ,
                {CONTRACT_NEUTRAL_OVERRIDE_ENV: "contract-neutral fix"},
                clear=False,
            ),
            patch(
                "canonical_completion_guard.infer_impacted_subsystems",
                return_value={
                    "cloud-paid": {
                        "contract_path": "docs/subsystems/cloud-paid.md",
                        "verification_requirements": [],
                    }
                },
            ),
            patch(
                "canonical_completion_guard.required_contract_updates",
                return_value={"docs/subsystems/cloud-paid.md": {}},
            ),
            patch(
                "canonical_completion_guard.staged_contract_has_substantive_change",
                return_value=False,
            ),
            patch(
                "canonical_completion_guard.staged_verification_files_for_requirement",
                return_value=[],
            ),
            patch(
                "canonical_completion_guard.format_missing_requirements",
                return_value="BLOCKED: missing contract",
            ),
            redirect_stderr(stderr),
        ):
            self.assertEqual(check_staged_contracts(["pkg/cloud/runtime.go"]), 0)

        stderr_value = stderr.getvalue()
        self.assertIn("canonical-shape block bypassed by", stderr_value)
        self.assertIn("contract-neutral fix", stderr_value)
        self.assertIn("Suppressed requirements", stderr_value)
        self.assertIn("BLOCKED: missing contract", stderr_value)

    def test_check_staged_contracts_blocks_and_suggests_bypass_when_unset(self):
        stderr = io.StringIO()
        with (
            patch.dict(os.environ, {CONTRACT_NEUTRAL_OVERRIDE_ENV: ""}, clear=False),
            patch(
                "canonical_completion_guard.infer_impacted_subsystems",
                return_value={
                    "cloud-paid": {
                        "contract_path": "docs/subsystems/cloud-paid.md",
                        "verification_requirements": [],
                    }
                },
            ),
            patch(
                "canonical_completion_guard.required_contract_updates",
                return_value={"docs/subsystems/cloud-paid.md": {}},
            ),
            patch(
                "canonical_completion_guard.staged_contract_has_substantive_change",
                return_value=False,
            ),
            patch(
                "canonical_completion_guard.staged_verification_files_for_requirement",
                return_value=[],
            ),
            patch(
                "canonical_completion_guard.format_missing_requirements",
                return_value="BLOCKED: missing contract",
            ),
            redirect_stderr(stderr),
        ):
            self.assertEqual(check_staged_contracts(["pkg/cloud/runtime.go"]), 1)

        stderr_value = stderr.getvalue()
        self.assertIn("BLOCKED: missing contract", stderr_value)
        self.assertIn(CONTRACT_NEUTRAL_OVERRIDE_ENV, stderr_value)
        self.assertIn("contract-neutral", stderr_value)


class DiffBaseContractComparisonTest(unittest.TestCase):
    """The guard runs in two modes. Pre-commit compares HEAD against the
    index, where the pending contract update is staged. CI has nothing
    staged (index == HEAD), so it must compare the push-range base against
    HEAD instead; before --diff-base existed, CI misreported every staged
    contract as having no substantive change."""

    CONTRACT_PATH = "docs/release-control/v6/internal/subsystems/deployment-installability.md"
    BASE_TEXT = "## Current State\n\nExisting obligation.\n"
    HEAD_TEXT = "## Current State\n\nExisting obligation.\nNew scoped exception recorded.\n"

    def test_index_mode_compares_head_against_index(self):
        blobs = {
            f"HEAD:{self.CONTRACT_PATH}": self.BASE_TEXT,
            f":0:{self.CONTRACT_PATH}": self.HEAD_TEXT,
        }
        with patch("canonical_completion_guard.git_blob_text", side_effect=blobs.get):
            self.assertTrue(staged_contract_has_substantive_change(self.CONTRACT_PATH))

    def test_index_mode_reports_no_change_when_index_equals_head(self):
        # The CI failure mode: index == HEAD, so the index comparison is
        # empty even though the push range contains a substantive update.
        blobs = {
            f"HEAD:{self.CONTRACT_PATH}": self.HEAD_TEXT,
            f":0:{self.CONTRACT_PATH}": self.HEAD_TEXT,
        }
        with patch("canonical_completion_guard.git_blob_text", side_effect=blobs.get):
            self.assertFalse(staged_contract_has_substantive_change(self.CONTRACT_PATH))

    def test_diff_base_mode_compares_base_against_head(self):
        blobs = {
            f"0d787a4b1:{self.CONTRACT_PATH}": self.BASE_TEXT,
            f"HEAD:{self.CONTRACT_PATH}": self.HEAD_TEXT,
            f":0:{self.CONTRACT_PATH}": self.HEAD_TEXT,
        }
        with patch("canonical_completion_guard.git_blob_text", side_effect=blobs.get):
            self.assertTrue(
                staged_contract_has_substantive_change(self.CONTRACT_PATH, "0d787a4b1")
            )

    def test_diff_base_mode_reports_no_change_for_identical_texts(self):
        blobs = {
            f"0d787a4b1:{self.CONTRACT_PATH}": self.HEAD_TEXT,
            f"HEAD:{self.CONTRACT_PATH}": self.HEAD_TEXT,
        }
        with patch("canonical_completion_guard.git_blob_text", side_effect=blobs.get):
            self.assertFalse(
                staged_contract_has_substantive_change(self.CONTRACT_PATH, "0d787a4b1")
            )

    def test_check_staged_contracts_threads_diff_base_into_comparison(self):
        seen: list[tuple[str, str | None]] = []

        def record(path, diff_base=None):
            seen.append((path, diff_base))
            return True

        with (
            patch(
                "canonical_completion_guard.infer_impacted_subsystems",
                return_value={},
            ),
            patch(
                "canonical_completion_guard.required_contract_updates",
                return_value={self.CONTRACT_PATH: {"subsystem": "deployment-installability"}},
            ),
            patch(
                "canonical_completion_guard.staged_contract_has_substantive_change",
                side_effect=record,
            ),
        ):
            self.assertEqual(
                check_staged_contracts([self.CONTRACT_PATH], diff_base="0d787a4b1"),
                0,
            )
        self.assertEqual(seen, [(self.CONTRACT_PATH, "0d787a4b1")])

    def test_resolve_diff_base_returns_merge_base_sha(self):
        head = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=REPO_ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        self.assertEqual(resolve_diff_base("HEAD"), head)

    def test_resolve_diff_base_falls_back_to_raw_ref(self):
        zero_sha = "0" * 40
        self.assertEqual(resolve_diff_base(zero_sha), zero_sha)


class ReleaseCycleArtifactGuardTest(unittest.TestCase):
    """The release_cycle_artifact_globs field in a subsystem rule lets
    per-RC-cycle artifacts (packet drafts, version-pointer docs, regenerated
    record files) skip the contract-update + verification requirements
    without losing the subsystem-tracked status. The lookup tool still
    reports ownership; the shape guard just doesn't demand a contract delta.
    Contract-shape files (Dockerfile, workflow YAMLs, runbook) still
    trigger as normal."""

    def test_rc_draft_packets_register_as_cycle_artifacts(self):
        impacted = infer_impacted_subsystems(
            [
                "docs/releases/RELEASE_NOTES_v6_RC5_DRAFT.md",
                "docs/releases/V6_CHANGELOG_RC5_DRAFT.md",
                "docs/releases/V6_RC5_OPERATOR_SUPPORT_PACK_DRAFT.md",
            ]
        )
        self.assertIn("deployment-installability", impacted)
        entry = impacted["deployment-installability"]
        # All three files are tracked as touches AND as cycle artifacts.
        self.assertEqual(set(entry["touched_runtime_files"]), set(entry["cycle_artifact_files"]))
        # No contract-update requirement since every touch is a cycle artifact.
        self.assertEqual(required_contract_updates([
            "docs/releases/RELEASE_NOTES_v6_RC5_DRAFT.md",
            "docs/releases/V6_CHANGELOG_RC5_DRAFT.md",
            "docs/releases/V6_RC5_OPERATOR_SUPPORT_PACK_DRAFT.md",
        ]), {})

    def test_version_pointer_docs_register_as_cycle_artifacts(self):
        impacted = infer_impacted_subsystems(
            ["docs/RELEASE_NOTES.md", "docs/UPGRADE_v6.md"]
        )
        self.assertIn("deployment-installability", impacted)
        self.assertEqual(
            required_contract_updates(["docs/RELEASE_NOTES.md", "docs/UPGRADE_v6.md"]),
            {},
        )

    def test_contract_shape_files_still_require_contract_update(self):
        impacted = infer_impacted_subsystems(["docker-compose.yml"])
        self.assertIn("deployment-installability", impacted)
        self.assertEqual(
            impacted["deployment-installability"]["cycle_artifact_files"],
            [],
            "docker-compose.yml is a contract-shape file, not a cycle artifact",
        )
        required = required_contract_updates(["docker-compose.yml"])
        self.assertIn(
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
            required,
        )

    def test_mixed_artifact_and_shape_only_requires_contract_update_for_shape(self):
        paths = [
            "docs/releases/RELEASE_NOTES_v6_RC5_DRAFT.md",
            "scripts/install-docker.sh",
        ]
        impacted = infer_impacted_subsystems(paths)
        entry = impacted["deployment-installability"]
        self.assertEqual(
            set(entry["cycle_artifact_files"]),
            {"docs/releases/RELEASE_NOTES_v6_RC5_DRAFT.md"},
        )
        required = required_contract_updates(paths)
        contract_path = "docs/release-control/v6/internal/subsystems/deployment-installability.md"
        self.assertIn(contract_path, required)
        # The contract update requirement carries the non-artifact file only.
        self.assertEqual(
            required[contract_path]["touched_runtime_files"],
            ["scripts/install-docker.sh"],
        )


if __name__ == "__main__":
    unittest.main()
