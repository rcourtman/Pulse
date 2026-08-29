#!/usr/bin/env python3
"""Summarize Pulse pseudonymous telemetry for operator-facing adoption reads.

This script intentionally normalizes version strings before aggregation so
manual builds, dev builds, and accidental `v` prefixes do not pollute
published-release reporting.
"""

from __future__ import annotations

import argparse
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
import gzip
import json
import re
import sqlite3
import subprocess
import sys
from typing import Any, Iterable
from urllib.request import Request, urlopen


DEFAULT_DB_PATH = "/var/lib/pulse-license/licenses.sqlite"
DEFAULT_GITHUB_REPO = "rcourtman/Pulse"
DEFAULT_LATEST_INSTALL_WINDOWS = (
    ("24h", timedelta(hours=24)),
    ("72h", timedelta(hours=72)),
    ("7d", timedelta(days=7)),
)
ADOPTION_COUNT_FIELDS = (
    ("pve_nodes", "PVE nodes"),
    ("pbs_instances", "PBS instances"),
    ("pmg_instances", "PMG instances"),
    ("vms", "VMs"),
    ("containers", "LXC containers"),
    ("agent_hosts", "Agent hosts"),
    ("docker_hosts", "Docker hosts"),
    ("docker_containers", "Docker containers"),
    ("kubernetes_clusters", "Kubernetes clusters"),
    ("kubernetes_nodes", "Kubernetes nodes"),
    ("kubernetes_pods", "Kubernetes pods"),
    ("kubernetes_deployments", "Kubernetes deployments"),
    ("storage_pools", "Storage pools"),
    ("physical_disks", "Physical disks"),
    ("ceph_clusters", "Ceph clusters"),
    ("network_shares", "Network shares"),
    ("truenas_systems", "TrueNAS systems"),
    ("truenas_vms", "TrueNAS VMs"),
    ("truenas_apps", "TrueNAS apps"),
    ("vmware_hosts", "VMware hosts"),
    ("vmware_vms", "VMware VMs"),
    ("vmware_datastores", "VMware datastores"),
    ("availability_targets", "Availability targets"),
    ("availability_probe_targets", "Availability probe targets"),
    ("availability_probe_agents", "Availability probe agents"),
    ("active_alerts", "Active alerts"),
    ("rbac_custom_roles", "Custom RBAC roles"),
    ("rbac_user_assignments", "RBAC user assignments"),
    ("audit_reads_30d", "Audit reads (30d)"),
    ("report_schedules", "Report schedules"),
    ("report_schedules_enabled", "Enabled report schedules"),
    ("report_schedules_run_30d", "Report schedule runs (30d)"),
    ("agent_profiles", "Agent profiles"),
    ("update_attempts_30d", "Update attempts (30d)"),
    ("update_successes_30d", "Update successes (30d)"),
    ("update_failures_30d", "Update failures (30d)"),
)
FEATURE_BOOL_FIELDS = (
    ("ai_enabled", "AI enabled"),
    ("patrol_enabled", "Patrol enabled"),
    ("discovery_enabled", "Discovery enabled"),
    ("notifications_enabled", "Notifications enabled"),
    ("ai_actions_enabled", "AI actions enabled"),
    ("alert_ai_enabled", "Alert AI enabled"),
    ("relay_enabled", "Relay enabled"),
    ("sso_enabled", "SSO enabled"),
    ("multi_tenant", "Multi-tenant"),
    ("paid_license", "Paid license"),
    ("has_api_tokens", "Has API tokens"),
)
USER_BASE_CATEGORY_FIELDS = (
    (
        "deployment_method",
        "Deployment method signal (best effort; upgraded installs often report container_other or binary_other)",
    ),
    (
        "known_install_age_bucket",
        "Time since first schema-v2 lifecycle observation",
    ),
    ("activation_stage", "Highest observed activation stage"),
    ("time_to_first_monitored_resource_bucket", "Time to first monitored resource"),
    ("estate_size_bucket", "Estate size"),
    ("update_last_failure_category", "Last update failure category"),
)
USER_BASE_BOOL_FIELDS = (
    ("auth_configured", "Authentication configured"),
    ("monitoring_active", "Monitoring currently active"),
    ("outcome_observed_30d", "Operational outcome observed"),
)
USER_BASE_COUNT_FIELDS = (
    ("configured_connections", "Configured connections"),
    ("alerts_fired_30d", "Alerts fired (30d)"),
    ("alerts_acknowledged_30d", "Alerts acknowledged (30d)"),
    ("alerts_resolved_30d", "Alerts resolved (30d)"),
    ("notification_attempts_7d", "Notification attempts, including retries (7d)"),
    ("notification_deliveries_7d", "Notification deliveries (7d)"),
    ("notification_failures_authentication_7d", "Notification authentication failures (7d, schema v5+)"),
    ("notification_failures_rate_limited_7d", "Notification rate-limit failures (7d, schema v5+)"),
    ("notification_failures_connectivity_7d", "Notification connectivity failures (7d, schema v5+)"),
    ("notification_failures_tls_7d", "Notification TLS failures (7d, schema v5+)"),
    ("notification_failures_configuration_7d", "Notification configuration failures (7d, schema v5+)"),
    ("notification_failures_rejected_7d", "Notification destination rejections (7d; new schema v15+ classifications are HTTP 4xx)"),
    ("notification_failures_server_error_7d", "Notification destination server errors (7d, schema v15+)"),
    ("notification_failures_unknown_7d", "Notification unknown failures (7d, schema v5+)"),
)
SERVICE_HEALTH_ROW_FIELDS = (
    "service_health_observed",
    "service_health_healthy",
    "service_health_failure_category",
    "service_health_cohort",
    "service_health_previous_version",
    "service_health_previous_observed",
    "service_health_previous_healthy",
)
NOTIFICATION_FAILURE_COUNT_SIGNALS = (
    (
        "notification_attempt_failures_7d_schema_v2",
        "Notification failed attempts (7d, legacy schema v2)",
    ),
    (
        "notification_terminal_failures_7d_schema_v3",
        "Notification terminal failures (7d, schema v3+)",
    ),
)
PULSE_INTELLIGENCE_ASSISTANT_LOOP_BOOL_FIELDS = (
    "pulse_intelligence_assistant_operations_loop_30d",
    "pulse_intelligence_assistant_approved_execution_loop_30d",
    "pulse_intelligence_assistant_approved_action_success_loop_30d",
    "pulse_intelligence_assistant_resolved_operations_loop_30d",
)
PULSE_INTELLIGENCE_EXTERNAL_AGENT_LOOP_BOOL_FIELDS = (
    "pulse_intelligence_external_agent_operations_loop_30d",
    "pulse_intelligence_external_agent_approved_execution_loop_30d",
    "pulse_intelligence_external_agent_approved_action_success_loop_30d",
    "pulse_intelligence_external_agent_resolved_operations_loop_30d",
)
PULSE_INTELLIGENCE_MCP_ADAPTER_LOOP_BOOL_FIELDS = (
    "pulse_intelligence_mcp_adapter_operations_loop_30d",
    "pulse_intelligence_mcp_adapter_approved_execution_loop_30d",
    "pulse_intelligence_mcp_adapter_approved_action_success_loop_30d",
    "pulse_intelligence_mcp_adapter_resolved_operations_loop_30d",
)
PULSE_INTELLIGENCE_PATROL_CONTROL_COMPLETED_BOOL_FIELDS = (
    "pulse_intelligence_patrol_control_completed_operations_loop_30d",
    "pulse_intelligence_pro_activation_completed_operations_loop_30d",
)
PULSE_INTELLIGENCE_PATROL_CONTROL_RESOLVED_BOOL_FIELDS = (
    "pulse_intelligence_patrol_control_resolved_operations_loop_30d",
    "pulse_intelligence_pro_activation_resolved_operations_loop_30d",
)
PULSE_INTELLIGENCE_PATROL_CONTROL_PAID_COMPLETED_BOOL_FIELDS = (
    "pulse_intelligence_patrol_control_paid_completed_operations_loop_30d",
    "pulse_intelligence_pro_activation_paid_completed_operations_loop_30d",
)
PULSE_INTELLIGENCE_PATROL_CONTROL_PAID_RESOLVED_BOOL_FIELDS = (
    "pulse_intelligence_patrol_control_paid_resolved_operations_loop_30d",
    "pulse_intelligence_pro_activation_paid_resolved_operations_loop_30d",
)
PULSE_INTELLIGENCE_PATROL_CONTROL_STARTER_COUNT_FIELDS = (
    "pulse_intelligence_patrol_control_operations_loop_starter_requests_30d",
    "pulse_intelligence_patrol_operations_loop_starter_requests_30d",
    "pulse_intelligence_pro_activation_operations_loop_starter_requests_30d",
)
PULSE_INTELLIGENCE_BOOL_FIELDS = (
    ("pulse_intelligence_loop_configured", "Loop configured"),
    ("pulse_intelligence_loop_active_30d", "Loop active 30d"),
    ("pulse_intelligence_complete_operations_loop_30d", "Complete operations loop 30d"),
    ("pulse_intelligence_approved_execution_loop_30d", "Approved execution loop 30d"),
    ("pulse_intelligence_resolved_operations_loop_30d", "Resolved operations loop 30d"),
    (
        "pulse_intelligence_patrol_control_completed_operations_loop_30d",
        "Patrol control completed operations loop 30d",
    ),
    (
        "pulse_intelligence_patrol_control_resolved_operations_loop_30d",
        "Patrol control resolved operations loop 30d",
    ),
    (
        "pulse_intelligence_patrol_control_paid_completed_operations_loop_30d",
        "Patrol control paid completed operations loop 30d",
    ),
    (
        "pulse_intelligence_patrol_control_paid_resolved_operations_loop_30d",
        "Patrol control paid resolved operations loop 30d",
    ),
    (
        "pulse_intelligence_pro_activation_completed_operations_loop_30d",
        "Legacy Pro activation completed operations loop 30d",
    ),
    (
        "pulse_intelligence_pro_activation_resolved_operations_loop_30d",
        "Legacy Pro activation resolved operations loop 30d",
    ),
    (
        "pulse_intelligence_pro_activation_paid_completed_operations_loop_30d",
        "Legacy Pro activation paid completed operations loop 30d",
    ),
    (
        "pulse_intelligence_pro_activation_paid_resolved_operations_loop_30d",
        "Legacy Pro activation paid resolved operations loop 30d",
    ),
    ("pulse_intelligence_governed_action_active_30d", "Governed action active 30d"),
    ("pulse_intelligence_assistant_operations_loop_30d", "Assistant operations loop 30d"),
    (
        "pulse_intelligence_assistant_approved_execution_loop_30d",
        "Assistant approved execution loop 30d",
    ),
    (
        "pulse_intelligence_assistant_approved_action_success_loop_30d",
        "Assistant approved action success loop 30d",
    ),
    (
        "pulse_intelligence_assistant_resolved_operations_loop_30d",
        "Assistant resolved operations loop 30d",
    ),
    ("pulse_intelligence_external_agent_operations_loop_30d", "Token-authenticated capability API operations loop 30d"),
    (
        "pulse_intelligence_external_agent_approved_execution_loop_30d",
        "Token-authenticated capability API approved execution loop 30d",
    ),
    (
        "pulse_intelligence_external_agent_approved_action_success_loop_30d",
        "Token-authenticated capability API approved action success loop 30d",
    ),
    (
        "pulse_intelligence_external_agent_resolved_operations_loop_30d",
        "Token-authenticated capability API resolved operations loop 30d",
    ),
    ("pulse_intelligence_mcp_adapter_operations_loop_30d", "Pulse MCP adapter operations loop 30d"),
    (
        "pulse_intelligence_mcp_adapter_approved_execution_loop_30d",
        "Pulse MCP adapter approved execution loop 30d",
    ),
    (
        "pulse_intelligence_mcp_adapter_approved_action_success_loop_30d",
        "Pulse MCP adapter approved action success loop 30d",
    ),
    (
        "pulse_intelligence_mcp_adapter_resolved_operations_loop_30d",
        "Pulse MCP adapter resolved operations loop 30d",
    ),
    ("pulse_intelligence_external_agent_enabled", "Capability API operations-loop token configured"),
    ("pulse_intelligence_external_agent_used_30d", "Token-authenticated capability API used 30d"),
    ("pulse_intelligence_mcp_adapter_used_30d", "Pulse MCP adapter used 30d"),
)
PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELDS = (
    (
        "pulse_intelligence_external_agent_context_requests_30d",
        "Token-authenticated capability API context requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_event_stream_requests_30d",
        "Token-authenticated capability API event-stream requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_provisioning_requests_30d",
        "Token-authenticated capability API provisioning requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_operator_state_requests_30d",
        "Token-authenticated capability API operator-state requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_finding_requests_30d",
        "Token-authenticated capability API finding requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_action_requests_30d",
        "Token-authenticated capability API action requests 30d",
    ),
)
PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELD_NAMES = tuple(
    field for field, _ in PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELDS
)
PULSE_INTELLIGENCE_EXTERNAL_AGENT_ACTIVITY_BOOL_FIELD_NAMES = (
    "pulse_intelligence_external_agent_used_30d",
    "pulse_intelligence_mcp_adapter_used_30d",
)
PULSE_INTELLIGENCE_COUNT_FIELDS = (
    (
        "pulse_intelligence_operations_loop_starter_requests_30d",
        "Operations-loop starter requests 30d",
    ),
    (
        "pulse_intelligence_assistant_operations_loop_starter_requests_30d",
        "Assistant operations-loop starter requests 30d",
    ),
    (
        "pulse_intelligence_patrol_operations_loop_starter_requests_30d",
        "Patrol operations-loop starter requests 30d",
    ),
    (
        "pulse_intelligence_patrol_control_operations_loop_starter_requests_30d",
        "Patrol control operations-loop starter requests 30d",
    ),
    (
        "pulse_intelligence_pro_activation_operations_loop_starter_requests_30d",
        "Legacy Pro activation operations-loop starter requests 30d",
    ),
    (
        "pulse_intelligence_mcp_operations_loop_starter_requests_30d",
        "Pulse MCP operations-loop starter requests 30d",
    ),
    ("pulse_intelligence_assistant_ai_calls_30d", "Assistant AI calls 30d"),
    ("pulse_intelligence_assistant_context_ai_calls_30d", "Assistant governed-context AI calls 30d"),
    ("pulse_intelligence_assistant_tool_calls_30d", "Assistant governed-tool calls 30d"),
    ("pulse_intelligence_patrol_ai_calls_30d", "Patrol AI calls 30d"),
    ("pulse_intelligence_patrol_runs_30d", "Patrol runs 30d"),
    ("pulse_intelligence_patrol_new_findings_30d", "Patrol new findings 30d"),
    ("pulse_intelligence_patrol_investigations_30d", "Patrol investigations 30d"),
    ("pulse_intelligence_patrol_resolved_findings_30d", "Patrol resolved findings 30d"),
    ("pulse_intelligence_patrol_autofixes_30d", "Patrol autofixes 30d"),
    *PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELDS,
    ("pulse_intelligence_action_plans_30d", "Action plans 30d"),
    ("pulse_intelligence_approval_requests_30d", "Approval requests 30d"),
    ("pulse_intelligence_rejected_action_decisions_30d", "Rejected action decisions 30d"),
    ("pulse_intelligence_approved_action_decisions_30d", "Approved action decisions 30d"),
    ("pulse_intelligence_approved_action_attempts_30d", "Approved action attempts 30d"),
    ("pulse_intelligence_approved_action_successes_30d", "Approved action successes 30d"),
    ("pulse_intelligence_patrol_action_plans_30d", "Patrol-origin action plans 30d"),
    ("pulse_intelligence_patrol_approval_requests_30d", "Patrol-origin approval requests 30d"),
    ("pulse_intelligence_patrol_rejected_action_decisions_30d", "Patrol-origin rejected action decisions 30d"),
    ("pulse_intelligence_patrol_approved_action_decisions_30d", "Patrol-origin approved action decisions 30d"),
    ("pulse_intelligence_patrol_approved_action_attempts_30d", "Patrol-origin approved action attempts 30d"),
    ("pulse_intelligence_patrol_approved_action_successes_30d", "Patrol-origin approved action successes 30d"),
    (
        "pulse_intelligence_approved_action_failures_pre_dispatch_30d",
        "Approved action failures (pre-dispatch refusal) 30d",
    ),
    (
        "pulse_intelligence_approved_action_failures_execution_30d",
        "Approved action failures (execution) 30d",
    ),
    (
        "pulse_intelligence_approved_action_failures_unverified_30d",
        "Approved action failures (completed unverified) 30d",
    ),
    (
        "pulse_intelligence_approved_action_stuck_executing_30d",
        "Approved action attempts stuck executing 30d",
    ),
    (
        "pulse_intelligence_approved_action_in_flight_30d",
        "Approved action attempts still in flight 30d",
    ),
    (
        "pulse_intelligence_approved_action_unclassified_30d",
        "Approved action attempts unclassified 30d",
    ),
    (
        "pulse_intelligence_approved_action_refusals_plan_stale_30d",
        "Pre-dispatch refusals: stale or expired plan 30d",
    ),
    (
        "pulse_intelligence_approved_action_refusals_policy_30d",
        "Pre-dispatch refusals: policy or operator safety control 30d",
    ),
    (
        "pulse_intelligence_approved_action_refusals_capability_30d",
        "Pre-dispatch refusals: capability unavailable 30d",
    ),
    (
        "pulse_intelligence_approved_action_refusals_other_30d",
        "Pre-dispatch refusals: other 30d",
    ),
    (
        "pulse_intelligence_verified_finding_resolutions_30d",
        "Independently verified action-to-finding resolutions 30d",
    ),
)
PULSE_INTELLIGENCE_OUTCOME_COHORTS = (
    (
        "loop_configured",
        "Loop configured",
        ("pulse_intelligence_loop_configured",),
        (),
    ),
    (
        "loop_active_30d",
        "Loop active 30d",
        ("pulse_intelligence_loop_active_30d",),
        tuple(field for field, _ in PULSE_INTELLIGENCE_COUNT_FIELDS),
    ),
    (
        "complete_operations_loop_30d",
        "Complete operations loop 30d",
        ("pulse_intelligence_complete_operations_loop_30d",),
        (),
    ),
    (
        "approved_execution_loop_30d",
        "Approved execution loop 30d",
        ("pulse_intelligence_approved_execution_loop_30d",),
        (),
    ),
    (
        "resolved_operations_loop_30d",
        "Resolved operations loop 30d",
        ("pulse_intelligence_resolved_operations_loop_30d",),
        (),
    ),
    (
        "patrol_control_completed_operations_loop_30d",
        "Patrol control completed operations loop 30d",
        PULSE_INTELLIGENCE_PATROL_CONTROL_COMPLETED_BOOL_FIELDS,
        (),
    ),
    (
        "patrol_control_resolved_operations_loop_30d",
        "Patrol control resolved operations loop 30d",
        PULSE_INTELLIGENCE_PATROL_CONTROL_RESOLVED_BOOL_FIELDS,
        (),
    ),
    (
        "patrol_control_paid_completed_operations_loop_30d",
        "Paid Patrol control completed operations loop 30d",
        PULSE_INTELLIGENCE_PATROL_CONTROL_PAID_COMPLETED_BOOL_FIELDS,
        (),
    ),
    (
        "patrol_control_paid_resolved_operations_loop_30d",
        "Paid Patrol control resolved operations loop 30d",
        PULSE_INTELLIGENCE_PATROL_CONTROL_PAID_RESOLVED_BOOL_FIELDS,
        (),
    ),
    (
        "assistant_operations_loop_30d",
        "Assistant operations loop 30d",
        ("pulse_intelligence_assistant_operations_loop_30d",),
        (),
    ),
    (
        "assistant_approved_execution_loop_30d",
        "Assistant approved execution loop 30d",
        ("pulse_intelligence_assistant_approved_execution_loop_30d",),
        (),
    ),
    (
        "assistant_approved_action_success_loop_30d",
        "Assistant approved action success loop 30d",
        ("pulse_intelligence_assistant_approved_action_success_loop_30d",),
        (),
    ),
    (
        "assistant_resolved_operations_loop_30d",
        "Assistant resolved operations loop 30d",
        ("pulse_intelligence_assistant_resolved_operations_loop_30d",),
        (),
    ),
    (
        "external_agent_operations_loop_30d",
        "Capability API/MCP adapter operations loop 30d",
        (
            "pulse_intelligence_external_agent_operations_loop_30d",
            "pulse_intelligence_mcp_adapter_operations_loop_30d",
        ),
        (),
    ),
    (
        "external_agent_approved_execution_loop_30d",
        "Capability API/MCP adapter approved execution loop 30d",
        (
            "pulse_intelligence_external_agent_approved_execution_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_execution_loop_30d",
        ),
        (),
    ),
    (
        "external_agent_approved_action_success_loop_30d",
        "Capability API/MCP adapter approved action success loop 30d",
        (
            "pulse_intelligence_external_agent_approved_action_success_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_action_success_loop_30d",
        ),
        (),
    ),
    (
        "external_agent_resolved_operations_loop_30d",
        "Capability API/MCP adapter resolved operations loop 30d",
        (
            "pulse_intelligence_external_agent_resolved_operations_loop_30d",
            "pulse_intelligence_mcp_adapter_resolved_operations_loop_30d",
        ),
        (),
    ),
    (
        "mcp_adapter_operations_loop_30d",
        "Pulse MCP adapter operations loop 30d",
        ("pulse_intelligence_mcp_adapter_operations_loop_30d",),
        (),
    ),
    (
        "mcp_adapter_approved_execution_loop_30d",
        "Pulse MCP adapter approved execution loop 30d",
        ("pulse_intelligence_mcp_adapter_approved_execution_loop_30d",),
        (),
    ),
    (
        "mcp_adapter_approved_action_success_loop_30d",
        "Pulse MCP adapter approved action success loop 30d",
        ("pulse_intelligence_mcp_adapter_approved_action_success_loop_30d",),
        (),
    ),
    (
        "mcp_adapter_resolved_operations_loop_30d",
        "Pulse MCP adapter resolved operations loop 30d",
        ("pulse_intelligence_mcp_adapter_resolved_operations_loop_30d",),
        (),
    ),
    (
        "operations_loop_starter_requests",
        "Operations-loop starter requests",
        (),
        ("pulse_intelligence_operations_loop_starter_requests_30d",),
    ),
    (
        "assistant_operations_loop_starter_requests",
        "Assistant operations-loop starter requests",
        (),
        ("pulse_intelligence_assistant_operations_loop_starter_requests_30d",),
    ),
    (
        "patrol_operations_loop_starter_requests",
        "Patrol operations-loop starter requests",
        (),
        ("pulse_intelligence_patrol_operations_loop_starter_requests_30d",),
    ),
    (
        "patrol_control_operations_loop_starter_requests",
        "Patrol control operations-loop starter requests",
        (),
        PULSE_INTELLIGENCE_PATROL_CONTROL_STARTER_COUNT_FIELDS,
    ),
    (
        "pro_activation_operations_loop_starter_requests",
        "Legacy Pro activation operations-loop starter requests",
        (),
        ("pulse_intelligence_pro_activation_operations_loop_starter_requests_30d",),
    ),
    (
        "mcp_operations_loop_starter_requests",
        "Pulse MCP operations-loop starter requests",
        (),
        ("pulse_intelligence_mcp_operations_loop_starter_requests_30d",),
    ),
    (
        "assistant_activity",
        "Assistant activity",
        (),
        ("pulse_intelligence_assistant_ai_calls_30d",),
    ),
    (
        "assistant_context_activity",
        "Assistant governed-context activity",
        (),
        ("pulse_intelligence_assistant_context_ai_calls_30d",),
    ),
    (
        "assistant_tool_activity",
        "Assistant governed-tool activity",
        (),
        ("pulse_intelligence_assistant_tool_calls_30d",),
    ),
    (
        "patrol_activity",
        "Patrol activity",
        (),
        (
            "pulse_intelligence_patrol_ai_calls_30d",
            "pulse_intelligence_patrol_runs_30d",
            "pulse_intelligence_patrol_new_findings_30d",
            "pulse_intelligence_patrol_investigations_30d",
            "pulse_intelligence_patrol_resolved_findings_30d",
            "pulse_intelligence_patrol_autofixes_30d",
        ),
    ),
    (
        "patrol_resolution_30d",
        "Patrol resolution 30d",
        (),
        ("pulse_intelligence_patrol_resolved_findings_30d",),
    ),
    (
        "external_agent_used_30d",
        "Capability API/MCP adapter used 30d",
        PULSE_INTELLIGENCE_EXTERNAL_AGENT_ACTIVITY_BOOL_FIELD_NAMES,
        PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELD_NAMES,
    ),
    (
        "mcp_adapter_used_30d",
        "Pulse MCP adapter used 30d",
        ("pulse_intelligence_mcp_adapter_used_30d",),
        (),
    ),
    *(
        (
            field.removeprefix("pulse_intelligence_").removesuffix("_30d"),
            label,
            (),
            (field,),
        )
        for field, label in PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELDS
    ),
    (
        "governed_action_active_30d",
        "Governed action active 30d",
        ("pulse_intelligence_governed_action_active_30d",),
        (
            "pulse_intelligence_action_plans_30d",
            "pulse_intelligence_approval_requests_30d",
            "pulse_intelligence_rejected_action_decisions_30d",
            "pulse_intelligence_approved_action_decisions_30d",
            "pulse_intelligence_approved_action_attempts_30d",
            "pulse_intelligence_approved_action_successes_30d",
        ),
    ),
    (
        "approved_action_decision_30d",
        "Approved action decision 30d",
        (),
        ("pulse_intelligence_approved_action_decisions_30d",),
    ),
    (
        "approved_action_execution_30d",
        "Approved action execution 30d",
        (),
        ("pulse_intelligence_approved_action_attempts_30d",),
    ),
    (
        "approved_action_success_30d",
        "Approved action success 30d",
        (),
        ("pulse_intelligence_approved_action_successes_30d",),
    ),
    (
        "patrol_action_plan_30d",
        "Patrol-origin action plan 30d",
        (),
        ("pulse_intelligence_patrol_action_plans_30d",),
    ),
    (
        "patrol_approval_request_30d",
        "Patrol-origin approval request 30d",
        (),
        ("pulse_intelligence_patrol_approval_requests_30d",),
    ),
    (
        "patrol_action_decision_30d",
        "Patrol-origin action decision 30d",
        (),
        (
            "pulse_intelligence_patrol_rejected_action_decisions_30d",
            "pulse_intelligence_patrol_approved_action_decisions_30d",
        ),
    ),
    (
        "patrol_action_attempt_30d",
        "Patrol-origin approved action attempt 30d",
        (),
        ("pulse_intelligence_patrol_approved_action_attempts_30d",),
    ),
    (
        "patrol_action_success_30d",
        "Patrol-origin approved action success 30d",
        (),
        ("pulse_intelligence_patrol_approved_action_successes_30d",),
    ),
)
PULSE_INTELLIGENCE_OPERATION_SIGNAL_GROUPS = {
    "configured": (
        ("pulse_intelligence_loop_configured",),
        (),
    ),
    "patrol": (
        (),
        (
            "pulse_intelligence_patrol_ai_calls_30d",
            "pulse_intelligence_patrol_runs_30d",
            "pulse_intelligence_patrol_new_findings_30d",
            "pulse_intelligence_patrol_investigations_30d",
            "pulse_intelligence_patrol_resolved_findings_30d",
            "pulse_intelligence_patrol_autofixes_30d",
        ),
    ),
    "patrol_resolution": (
        (),
        ("pulse_intelligence_patrol_resolved_findings_30d",),
    ),
    "patrol_issue": (
        (),
        (
            "pulse_intelligence_patrol_new_findings_30d",
            "pulse_intelligence_patrol_investigations_30d",
            "pulse_intelligence_patrol_resolved_findings_30d",
            "pulse_intelligence_patrol_autofixes_30d",
        ),
    ),
    "collaboration": (
        (
            *PULSE_INTELLIGENCE_ASSISTANT_LOOP_BOOL_FIELDS,
            *PULSE_INTELLIGENCE_EXTERNAL_AGENT_LOOP_BOOL_FIELDS,
            *PULSE_INTELLIGENCE_MCP_ADAPTER_LOOP_BOOL_FIELDS,
            *PULSE_INTELLIGENCE_EXTERNAL_AGENT_ACTIVITY_BOOL_FIELD_NAMES,
        ),
        (
            "pulse_intelligence_assistant_context_ai_calls_30d",
            "pulse_intelligence_assistant_tool_calls_30d",
            *PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELD_NAMES,
        ),
    ),
    "assistant_collaboration": (
        PULSE_INTELLIGENCE_ASSISTANT_LOOP_BOOL_FIELDS,
        (
            "pulse_intelligence_assistant_context_ai_calls_30d",
            "pulse_intelligence_assistant_tool_calls_30d",
        ),
    ),
    "external_agent_collaboration": (
        (
            *PULSE_INTELLIGENCE_EXTERNAL_AGENT_LOOP_BOOL_FIELDS,
            *PULSE_INTELLIGENCE_EXTERNAL_AGENT_ACTIVITY_BOOL_FIELD_NAMES,
        ),
        PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELD_NAMES,
    ),
    "mcp_adapter": (
        (
            *PULSE_INTELLIGENCE_MCP_ADAPTER_LOOP_BOOL_FIELDS,
            "pulse_intelligence_mcp_adapter_used_30d",
        ),
        (),
    ),
    "governed_action": (
        ("pulse_intelligence_governed_action_active_30d",),
        (
            "pulse_intelligence_action_plans_30d",
            "pulse_intelligence_approval_requests_30d",
            "pulse_intelligence_rejected_action_decisions_30d",
            "pulse_intelligence_approved_action_decisions_30d",
            "pulse_intelligence_approved_action_attempts_30d",
            "pulse_intelligence_approved_action_successes_30d",
        ),
    ),
    "governed_decision": (
        (),
        (
            "pulse_intelligence_rejected_action_decisions_30d",
            "pulse_intelligence_approved_action_decisions_30d",
            "pulse_intelligence_approved_action_attempts_30d",
        ),
    ),
    "patrol_action": (
        (),
        (
            "pulse_intelligence_patrol_action_plans_30d",
            "pulse_intelligence_patrol_approval_requests_30d",
            "pulse_intelligence_patrol_rejected_action_decisions_30d",
            "pulse_intelligence_patrol_approved_action_decisions_30d",
            "pulse_intelligence_patrol_approved_action_attempts_30d",
            "pulse_intelligence_patrol_approved_action_successes_30d",
        ),
    ),
    "approved_execution": (
        (
            "pulse_intelligence_approved_execution_loop_30d",
            "pulse_intelligence_assistant_approved_execution_loop_30d",
            "pulse_intelligence_external_agent_approved_execution_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_execution_loop_30d",
        ),
        ("pulse_intelligence_approved_action_attempts_30d",),
    ),
    "approved_success": (
        (
            "pulse_intelligence_assistant_approved_action_success_loop_30d",
            "pulse_intelligence_external_agent_approved_action_success_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_action_success_loop_30d",
        ),
        ("pulse_intelligence_approved_action_successes_30d",),
    ),
    "complete_operations_loop": (
        (
            "pulse_intelligence_complete_operations_loop_30d",
            *PULSE_INTELLIGENCE_PATROL_CONTROL_COMPLETED_BOOL_FIELDS,
            "pulse_intelligence_assistant_operations_loop_30d",
            "pulse_intelligence_external_agent_operations_loop_30d",
            "pulse_intelligence_mcp_adapter_operations_loop_30d",
        ),
        (),
    ),
    "approved_execution_loop": (
        (
            "pulse_intelligence_approved_execution_loop_30d",
            "pulse_intelligence_assistant_approved_execution_loop_30d",
            "pulse_intelligence_external_agent_approved_execution_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_execution_loop_30d",
        ),
        (),
    ),
    "resolved_operations_loop": (
        (
            "pulse_intelligence_resolved_operations_loop_30d",
            *PULSE_INTELLIGENCE_PATROL_CONTROL_RESOLVED_BOOL_FIELDS,
            "pulse_intelligence_assistant_resolved_operations_loop_30d",
            "pulse_intelligence_external_agent_resolved_operations_loop_30d",
            "pulse_intelligence_mcp_adapter_resolved_operations_loop_30d",
        ),
        (),
    ),
    "patrol_control_completed_operations_loop": (
        PULSE_INTELLIGENCE_PATROL_CONTROL_COMPLETED_BOOL_FIELDS,
        (),
    ),
    "patrol_control_resolved_operations_loop": (
        PULSE_INTELLIGENCE_PATROL_CONTROL_RESOLVED_BOOL_FIELDS,
        (),
    ),
    "assistant_operations_loop": (
        ("pulse_intelligence_assistant_operations_loop_30d",),
        (),
    ),
    "assistant_approved_execution_loop": (
        ("pulse_intelligence_assistant_approved_execution_loop_30d",),
        (),
    ),
    "assistant_approved_success_loop": (
        ("pulse_intelligence_assistant_approved_action_success_loop_30d",),
        (),
    ),
    "assistant_resolved_operations_loop": (
        ("pulse_intelligence_assistant_resolved_operations_loop_30d",),
        (),
    ),
    "external_agent_operations_loop": (
        (
            "pulse_intelligence_external_agent_operations_loop_30d",
            "pulse_intelligence_mcp_adapter_operations_loop_30d",
        ),
        (),
    ),
    "external_agent_approved_execution_loop": (
        (
            "pulse_intelligence_external_agent_approved_execution_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_execution_loop_30d",
        ),
        (),
    ),
    "external_agent_approved_success_loop": (
        (
            "pulse_intelligence_external_agent_approved_action_success_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_action_success_loop_30d",
        ),
        (),
    ),
    "external_agent_resolved_operations_loop": (
        (
            "pulse_intelligence_external_agent_resolved_operations_loop_30d",
            "pulse_intelligence_mcp_adapter_resolved_operations_loop_30d",
        ),
        (),
    ),
    "mcp_adapter_operations_loop": (
        ("pulse_intelligence_mcp_adapter_operations_loop_30d",),
        (),
    ),
    "mcp_adapter_approved_execution_loop": (
        ("pulse_intelligence_mcp_adapter_approved_execution_loop_30d",),
        (),
    ),
    "mcp_adapter_approved_success_loop": (
        ("pulse_intelligence_mcp_adapter_approved_action_success_loop_30d",),
        (),
    ),
    "mcp_adapter_resolved_operations_loop": (
        ("pulse_intelligence_mcp_adapter_resolved_operations_loop_30d",),
        (),
    ),
}
PULSE_INTELLIGENCE_OPERATIONS_FUNNEL_STAGES = (
    ("configured", "Configured", ("configured",)),
    ("patrol_activity", "Patrol detection/investigation", ("patrol",)),
    ("patrol_issue_evidence", "Patrol issue evidence", ("patrol_issue",)),
    ("assistant_mcp_collaboration", "Assistant/MCP collaboration", ("collaboration",)),
    ("governed_action", "Governed action activity", ("governed_action",)),
    ("governed_decision", "Approve/reject decision", ("governed_decision",)),
    ("approved_action_execution", "Approved action execution", ("approved_execution",)),
    ("approved_action_success", "Approved action success", ("approved_success",)),
    ("patrol_resolution", "Patrol resolution", ("patrol_resolution",)),
    (
        "complete_operations_loop",
        "Complete operations loop",
        ("complete_operations_loop",),
    ),
    (
        "approved_execution_loop",
        "Approved execution loop",
        ("approved_execution_loop",),
    ),
    (
        "resolved_operations_loop",
        "Resolved operations loop",
        ("resolved_operations_loop",),
    ),
    (
        "patrol_control_completed_operations_loop",
        "Patrol control completed operations loop",
        ("patrol_control_completed_operations_loop",),
    ),
    (
        "patrol_control_resolved_operations_loop",
        "Patrol control resolved operations loop",
        ("patrol_control_resolved_operations_loop",),
    ),
    (
        "assistant_operations_loop",
        "Assistant operations loop",
        ("assistant_operations_loop",),
    ),
    (
        "assistant_approved_execution_loop",
        "Assistant approved execution loop",
        ("assistant_approved_execution_loop",),
    ),
    (
        "assistant_approved_success_loop",
        "Assistant approved action success loop",
        ("assistant_approved_success_loop",),
    ),
    (
        "assistant_resolved_operations_loop",
        "Assistant resolved operations loop",
        ("assistant_resolved_operations_loop",),
    ),
    (
        "external_agent_operations_loop",
        "Capability API/MCP adapter operations loop",
        ("external_agent_operations_loop",),
    ),
    (
        "external_agent_approved_execution_loop",
        "Capability API/MCP adapter approved execution loop",
        ("external_agent_approved_execution_loop",),
    ),
    (
        "external_agent_approved_success_loop",
        "Capability API/MCP adapter approved action success loop",
        ("external_agent_approved_success_loop",),
    ),
    (
        "external_agent_resolved_operations_loop",
        "Capability API/MCP adapter resolved operations loop",
        ("external_agent_resolved_operations_loop",),
    ),
    (
        "mcp_adapter_operations_loop",
        "Pulse MCP adapter operations loop",
        ("mcp_adapter_operations_loop",),
    ),
    (
        "mcp_adapter_approved_execution_loop",
        "Pulse MCP adapter approved execution loop",
        ("mcp_adapter_approved_execution_loop",),
    ),
    (
        "mcp_adapter_approved_success_loop",
        "Pulse MCP adapter approved success loop",
        ("mcp_adapter_approved_success_loop",),
    ),
    (
        "mcp_adapter_resolved_operations_loop",
        "Pulse MCP adapter resolved operations loop",
        ("mcp_adapter_resolved_operations_loop",),
    ),
)
DEEP_SIGNAL_FIELDS = (
    ("agent_hosts", "Agent hosts", "count"),
    ("docker_containers", "Docker containers", "count"),
    ("kubernetes_nodes", "Kubernetes nodes", "count"),
    ("kubernetes_pods", "Kubernetes pods", "count"),
    ("kubernetes_deployments", "Kubernetes deployments", "count"),
    ("storage_pools", "Storage pools", "count"),
    ("physical_disks", "Physical disks", "count"),
    ("ceph_clusters", "Ceph clusters", "count"),
    ("network_shares", "Network shares", "count"),
    ("truenas_systems", "TrueNAS systems", "count"),
    ("truenas_vms", "TrueNAS VMs", "count"),
    ("truenas_apps", "TrueNAS apps", "count"),
    ("vmware_hosts", "VMware hosts", "count"),
    ("vmware_vms", "VMware VMs", "count"),
    ("vmware_datastores", "VMware datastores", "count"),
    ("availability_targets", "Availability targets", "count"),
    ("availability_probe_targets", "Availability probe targets", "count"),
    ("availability_probe_agents", "Availability probe agents", "count"),
    ("rbac_custom_roles", "Custom RBAC roles", "count"),
    ("rbac_user_assignments", "RBAC user assignments", "count"),
    ("audit_reads_30d", "Audit reads (30d)", "count"),
    ("report_schedules", "Report schedules", "count"),
    ("report_schedules_enabled", "Enabled report schedules", "count"),
    ("report_schedules_run_30d", "Report schedule runs (30d)", "count"),
    ("agent_profiles", "Agent profiles", "count"),
    ("update_attempts_30d", "Update attempts (30d)", "count"),
    ("update_successes_30d", "Update successes (30d)", "count"),
    ("update_failures_30d", "Update failures (30d)", "count"),
    ("patrol_enabled", "Patrol enabled", "bool"),
    ("discovery_enabled", "Discovery enabled", "bool"),
    ("notifications_enabled", "Notifications enabled", "bool"),
    ("ai_actions_enabled", "AI actions enabled", "bool"),
    ("alert_ai_enabled", "Alert AI enabled", "bool"),
    *(
        (field, label, "bool")
        for field, label in PULSE_INTELLIGENCE_BOOL_FIELDS
    ),
    *(
        (field, label, "count")
        for field, label in PULSE_INTELLIGENCE_COUNT_FIELDS
    ),
)
ALERT_QUALITY_SCHEMA_VERSION = 14
ALERT_QUALITY_COUNT_FIELDS = (
    ("active_alerts_info", "Active alerts: info"),
    ("active_alerts_warning", "Active alerts: warning"),
    ("active_alerts_critical", "Active alerts: critical"),
    ("active_alerts_age_under_1h", "Active alerts under 1h"),
    ("active_alerts_age_1h_24h", "Active alerts 1h-24h"),
    ("active_alerts_age_1d_7d", "Active alerts 1d-7d"),
    ("active_alerts_age_7d_plus", "Active alerts 7d+"),
    ("alerts_fired_info_30d", "Alerts fired: info (30d)"),
    ("alerts_fired_warning_30d", "Alerts fired: warning (30d)"),
    ("alerts_fired_critical_30d", "Alerts fired: critical (30d)"),
    ("alerts_resolved_info_30d", "Alerts resolved: info (30d)"),
    ("alerts_resolved_warning_30d", "Alerts resolved: warning (30d)"),
    ("alerts_resolved_critical_30d", "Alerts resolved: critical (30d)"),
    ("alerts_resolution_under_15m_30d", "Resolution under 15m (30d)"),
    ("alerts_resolution_15m_1h_30d", "Resolution 15m-1h (30d)"),
    ("alerts_resolution_1h_24h_30d", "Resolution 1h-24h (30d)"),
    ("alerts_resolution_1d_7d_30d", "Resolution 1d-7d (30d)"),
    ("alerts_resolution_7d_plus_30d", "Resolution 7d+ (30d)"),
    ("alerts_repeat_occurrences_30d", "Repeat alert occurrences (30d)"),
    ("alerts_snoozed_occurrences_30d", "Snoozed alert occurrences (30d)"),
    ("alerts_resolved_while_snoozed_30d", "Alerts resolved while snoozed (30d)"),
    ("alert_manager_tenants", "Alert manager tenants"),
    ("alert_delivery_active_tenants", "Alert delivery active tenants"),
    ("alert_flapping_enabled_tenants", "Flapping detection enabled tenants"),
    ("alert_intent_policy_configured_tenants", "Alert intent policy tenants"),
    ("alert_event_history_authoritative_tenants", "Authoritative event-history tenants"),
    ("alert_active_state_authoritative_tenants", "Authoritative active-state tenants"),
    ("alert_active_state_persistence_degraded_tenants", "Degraded active-state tenants"),
)
REPORT_ROW_COLUMNS = tuple(
    dict.fromkeys(
        (
            "received_at",
            "install_id",
            "version",
            "version_raw",
            "schema_version",
            "version_channel",
            "version_build",
            "version_is_development",
            "version_is_published_release",
            "platform",
            "notification_failures_7d",
            *(key for key, _ in ALERT_QUALITY_COUNT_FIELDS),
            *(key for key, _ in ADOPTION_COUNT_FIELDS),
            *(key for key, _ in FEATURE_BOOL_FIELDS),
            *(key for key, _ in USER_BASE_CATEGORY_FIELDS),
            *(key for key, _ in USER_BASE_BOOL_FIELDS),
            *(key for key, _ in USER_BASE_COUNT_FIELDS),
            *SERVICE_HEALTH_ROW_FIELDS,
            *(key for key, _ in PULSE_INTELLIGENCE_BOOL_FIELDS),
            *(key for key, _ in PULSE_INTELLIGENCE_COUNT_FIELDS),
        )
    )
)
REPORT_ROW_PROJECTION = ", ".join(REPORT_ROW_COLUMNS)
REPORT_HISTORY_SIGNAL_COLUMNS = tuple(
    dict.fromkeys(
        (
            *(key for key, _ in PULSE_INTELLIGENCE_BOOL_FIELDS),
            *(key for key, _ in PULSE_INTELLIGENCE_COUNT_FIELDS),
        )
    )
)
TARGET_RELEASE_ACTIVITY_COUNT_FIELDS = tuple(
    dict.fromkeys(
        (
            "alerts_fired_30d",
            "alerts_acknowledged_30d",
            "alerts_resolved_30d",
            *(field for field, _ in ALERT_QUALITY_COUNT_FIELDS),
            "notification_attempts_7d",
            "notification_deliveries_7d",
            "notification_failures_7d",
            "notification_failures_authentication_7d",
            "notification_failures_rate_limited_7d",
            "notification_failures_connectivity_7d",
            "notification_failures_tls_7d",
            "notification_failures_configuration_7d",
            "notification_failures_rejected_7d",
            "notification_failures_server_error_7d",
            "notification_failures_unknown_7d",
            "audit_reads_30d",
            "report_schedules_run_30d",
            "update_attempts_30d",
            "update_successes_30d",
            "update_failures_30d",
            *(key for key, _ in PULSE_INTELLIGENCE_COUNT_FIELDS),
        )
    )
)
TARGET_RELEASE_ACTIVITY_LABELS = {
    **{key: label for key, label in USER_BASE_COUNT_FIELDS},
    **{key: label for key, label in ADOPTION_COUNT_FIELDS},
    **{key: label for key, label in PULSE_INTELLIGENCE_COUNT_FIELDS},
    "notification_failures_7d": "Notification failures (7d; schema-dependent semantics)",
    **{key: label for key, label in ALERT_QUALITY_COUNT_FIELDS},
}
GIT_DESCRIBE_RE = re.compile(
    r"^(?P<base>\d+\.\d+\.\d+(?:-[0-9A-Za-z\.-]+)?)-(?P<count>\d+)-g(?P<sha>[0-9a-fA-F]+)(?P<dirty>-dirty)?$"
)
SEMVER_RE = re.compile(
    r"^(?P<major>\d+)\.(?P<minor>\d+)\.(?P<patch>\d+)(?:-(?P<prerelease>[^+]+))?(?:\+(?P<build>.+))?$"
)
TOKEN_RE = re.compile(r"[^0-9A-Za-z.-]+")


@dataclass(frozen=True)
class ClassifiedVersion:
    raw_version: str
    version: str
    channel: str
    build: str
    is_development: bool
    is_published_release: bool


@dataclass(frozen=True)
class PulseIntelligenceInstallAnalysis:
    latest_received_at: datetime
    first_paid_at: datetime | None
    observed_free_start: bool
    observed_free_to_paid: bool
    cohort_keys: frozenset[str]
    free_cohort_keys: frozenset[str]
    signal_groups: frozenset[str]
    free_signal_groups: frozenset[str]


@dataclass(frozen=True)
class TargetReleaseInstallAnalysis:
    first_received_at: datetime
    latest_received_at: datetime
    heartbeat_count: int
    same_version_pair_count: int
    first_counts: tuple[int, ...]
    increase_totals: tuple[int, ...]
    decrease_totals: tuple[int, ...]
    latest_schema_version: int
    latest_known_install_age_bucket: str


@dataclass
class _TargetReleaseInstallAccumulator:
    first_received_at: datetime | None = None
    latest_received_at: datetime | None = None
    heartbeat_count: int = 0
    same_version_pair_count: int = 0
    first_counts: tuple[int, ...] = ()
    increase_totals: list[int] = field(
        default_factory=lambda: [0] * len(TARGET_RELEASE_ACTIVITY_COUNT_FIELDS)
    )
    decrease_totals: list[int] = field(
        default_factory=lambda: [0] * len(TARGET_RELEASE_ACTIVITY_COUNT_FIELDS)
    )
    adjacent_received_at: datetime | None = None
    adjacent_is_target: bool = False
    adjacent_counts: tuple[int, ...] = ()
    latest_schema_version: int = 0
    latest_known_install_age_bucket: str = "unknown"

    def observe(self, row: dict[str, Any], received_at: datetime, is_target: bool) -> None:
        counts = tuple(
            parse_optional_nonnegative_int(row.get(field))
            for field in TARGET_RELEASE_ACTIVITY_COUNT_FIELDS
        )
        if is_target:
            self.heartbeat_count += 1
            if self.first_received_at is None or received_at < self.first_received_at:
                self.first_received_at = received_at
                self.first_counts = counts
            if self.latest_received_at is None or received_at > self.latest_received_at:
                self.latest_received_at = received_at
                self.latest_schema_version = parse_optional_nonnegative_int(
                    row.get("schema_version")
                )
                self.latest_known_install_age_bucket = (
                    str(row.get("known_install_age_bucket") or "unknown").strip()
                    or "unknown"
                )

        if (
            is_target
            and self.adjacent_is_target
            and self.adjacent_received_at is not None
            and received_at != self.adjacent_received_at
        ):
            self.same_version_pair_count += 1
            if received_at > self.adjacent_received_at:
                earlier_counts, later_counts = self.adjacent_counts, counts
            else:
                earlier_counts, later_counts = counts, self.adjacent_counts
            for index, (earlier, later) in enumerate(zip(earlier_counts, later_counts)):
                change = later - earlier
                if change > 0:
                    self.increase_totals[index] += change
                elif change < 0:
                    self.decrease_totals[index] += -change

        self.adjacent_received_at = received_at
        self.adjacent_is_target = is_target
        self.adjacent_counts = counts

    def finalize(self) -> TargetReleaseInstallAnalysis:
        if self.first_received_at is None or self.latest_received_at is None:
            raise ValueError("target release analysis requires at least one heartbeat")
        return TargetReleaseInstallAnalysis(
            first_received_at=self.first_received_at,
            latest_received_at=self.latest_received_at,
            heartbeat_count=self.heartbeat_count,
            same_version_pair_count=self.same_version_pair_count,
            first_counts=self.first_counts,
            increase_totals=tuple(self.increase_totals),
            decrease_totals=tuple(self.decrease_totals),
            latest_schema_version=self.latest_schema_version,
            latest_known_install_age_bucket=self.latest_known_install_age_bucket,
        )


@dataclass
class _PulseIntelligenceInstallAccumulator:
    latest_received_at: datetime | None = None
    first_free_at: datetime | None = None
    first_paid_at: datetime | None = None
    cohort_keys: set[str] = field(default_factory=set)
    signal_groups: set[str] = field(default_factory=set)
    earliest_free_cohort_at: dict[str, datetime] = field(default_factory=dict)
    earliest_free_signal_group_at: dict[str, datetime] = field(default_factory=dict)

    def observe(self, row: dict[str, Any]) -> None:
        received_at = parse_received_at(str(row["received_at"]))
        posture = parse_optional_bool(row.get("paid_license"))
        if self.latest_received_at is None or received_at > self.latest_received_at:
            self.latest_received_at = received_at
        if posture is False and (
            self.first_free_at is None or received_at < self.first_free_at
        ):
            self.first_free_at = received_at
        if posture is True and (
            self.first_paid_at is None or received_at < self.first_paid_at
        ):
            self.first_paid_at = received_at

        row_cohort_keys, row_signal_groups = pulse_intelligence_row_analysis_keys(row)
        self.cohort_keys.update(row_cohort_keys)
        self.signal_groups.update(row_signal_groups)
        if posture is not False:
            return
        for key in row_cohort_keys:
            observed_at = self.earliest_free_cohort_at.get(key)
            if observed_at is None or received_at < observed_at:
                self.earliest_free_cohort_at[key] = received_at
        for key in row_signal_groups:
            observed_at = self.earliest_free_signal_group_at.get(key)
            if observed_at is None or received_at < observed_at:
                self.earliest_free_signal_group_at[key] = received_at

    def finalize(self) -> PulseIntelligenceInstallAnalysis:
        if self.latest_received_at is None:
            raise ValueError("Pulse Intelligence install analysis requires at least one row")

        first_paid_at = self.first_paid_at
        free_cohort_keys = {
            key
            for key, observed_at in self.earliest_free_cohort_at.items()
            if first_paid_at is None or observed_at < first_paid_at
        }
        free_signal_groups = {
            key
            for key, observed_at in self.earliest_free_signal_group_at.items()
            if first_paid_at is None or observed_at < first_paid_at
        }
        signal_groups = set(self.signal_groups)
        pulse_intelligence_derive_signal_groups(signal_groups)
        pulse_intelligence_derive_signal_groups(free_signal_groups)
        observed_free_start = self.first_free_at is not None and (
            first_paid_at is None or self.first_free_at < first_paid_at
        )
        return PulseIntelligenceInstallAnalysis(
            latest_received_at=self.latest_received_at,
            first_paid_at=first_paid_at,
            observed_free_start=observed_free_start,
            observed_free_to_paid=observed_free_start and first_paid_at is not None,
            cohort_keys=frozenset(self.cohort_keys),
            free_cohort_keys=frozenset(free_cohort_keys),
            signal_groups=frozenset(signal_groups),
            free_signal_groups=frozenset(free_signal_groups),
        )


PULSE_INTELLIGENCE_ANALYSIS_BOOL_FIELDS = tuple(
    sorted(
        {
            field
            for _, _, bool_fields, _ in PULSE_INTELLIGENCE_OUTCOME_COHORTS
            for field in bool_fields
        }
        | {
            field
            for bool_fields, _ in PULSE_INTELLIGENCE_OPERATION_SIGNAL_GROUPS.values()
            for field in bool_fields
        }
    )
)
PULSE_INTELLIGENCE_ANALYSIS_COUNT_FIELDS = tuple(
    sorted(
        {
            field
            for _, _, _, count_fields in PULSE_INTELLIGENCE_OUTCOME_COHORTS
            for field in count_fields
        }
        | {
            field
            for _, count_fields in PULSE_INTELLIGENCE_OPERATION_SIGNAL_GROUPS.values()
            for field in count_fields
        }
    )
)
PULSE_INTELLIGENCE_COHORT_BOOL_KEYS_BY_FIELD = {
    field: frozenset(
        key
        for key, _, bool_fields, _ in PULSE_INTELLIGENCE_OUTCOME_COHORTS
        if field in bool_fields
    )
    for field in PULSE_INTELLIGENCE_ANALYSIS_BOOL_FIELDS
}
PULSE_INTELLIGENCE_COHORT_COUNT_KEYS_BY_FIELD = {
    field: frozenset(
        key
        for key, _, _, count_fields in PULSE_INTELLIGENCE_OUTCOME_COHORTS
        if field in count_fields
    )
    for field in PULSE_INTELLIGENCE_ANALYSIS_COUNT_FIELDS
}
PULSE_INTELLIGENCE_SIGNAL_GROUP_BOOL_KEYS_BY_FIELD = {
    field: frozenset(
        key
        for key, (bool_fields, _) in PULSE_INTELLIGENCE_OPERATION_SIGNAL_GROUPS.items()
        if field in bool_fields
    )
    for field in PULSE_INTELLIGENCE_ANALYSIS_BOOL_FIELDS
}
PULSE_INTELLIGENCE_SIGNAL_GROUP_COUNT_KEYS_BY_FIELD = {
    field: frozenset(
        key
        for key, (_, count_fields) in PULSE_INTELLIGENCE_OPERATION_SIGNAL_GROUPS.items()
        if field in count_fields
    )
    for field in PULSE_INTELLIGENCE_ANALYSIS_COUNT_FIELDS
}


def normalize_reported_version(raw: str) -> str:
    value = raw.strip()
    if value.startswith("v"):
        value = value[1:]
    if not value:
        return "0.0.0-dev"

    match = GIT_DESCRIBE_RE.match(value)
    if match:
        build = f"git.{match.group('count')}.g{match.group('sha').lower()}"
        if match.group("dirty"):
            build += ".dirty"
        return f"{match.group('base')}+{build}"

    if SEMVER_RE.match(value):
        return value

    sanitized = TOKEN_RE.sub("-", value).strip("-.").lower()
    if not sanitized:
        sanitized = "dev"
    return f"0.0.0-{sanitized}"


def parse_semver(version: str) -> dict[str, str] | None:
    match = SEMVER_RE.match(version)
    if not match:
        return None
    return {
        "prerelease": match.group("prerelease") or "",
        "build": match.group("build") or "",
    }


def version_channel(version: str) -> str:
    parsed = parse_semver(version)
    if parsed is None:
        return "unknown"
    prerelease = parsed["prerelease"].lower()
    build = parsed["build"].lower()
    if build:
        return "dev"
    if prerelease.startswith("rc."):
        return "rc"
    if prerelease == "dev" or prerelease.startswith("dev."):
        return "dev"
    if prerelease:
        return "prerelease"
    return "stable"


def classify_reported_version(raw: str, published_versions: set[str]) -> ClassifiedVersion:
    normalized = normalize_reported_version(raw)
    parsed = parse_semver(normalized) or {"build": ""}
    channel = version_channel(normalized)
    published_candidate = channel in {"stable", "rc"} and not parsed["build"]
    is_published_release = normalized in published_versions if published_versions else published_candidate
    return ClassifiedVersion(
        raw_version=raw.strip(),
        version=normalized,
        channel=channel,
        build=parsed["build"],
        is_development=channel == "dev",
        is_published_release=is_published_release,
    )


def parse_optional_bool(value: Any) -> bool | None:
    if value is None:
        return None
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value != 0
    normalized = str(value).strip().lower()
    if normalized == "":
        return None
    if normalized in {"1", "true", "t", "yes", "y"}:
        return True
    if normalized in {"0", "false", "f", "no", "n"}:
        return False
    return None


def parse_optional_nonnegative_int(value: Any) -> int:
    if value is None:
        return 0
    try:
        parsed = int(value)
    except (TypeError, ValueError):
        return 0
    return max(parsed, 0)


# Mock fixture fleet signature: the internal/mock defaults ship 120 Kubernetes
# pods (3 clusters × 40) alongside 7 VMware hosts (4 lab + 3 edge), and scaled
# fixture configs multiply both together. Versions before client-side mock
# suppression (6.1.0-rc.2 and earlier) sent telemetry from mock-mode boots
# (e2e, CI, qual runs, demo containers), so matching rows are excluded from
# adoption reads by default.
MOCK_FLEET_KUBERNETES_PODS_PER_SCALE = 120
MOCK_FLEET_VMWARE_HOSTS_PER_SCALE = 7


def is_mock_fleet_row(row: dict[str, Any]) -> bool:
    pods = parse_optional_nonnegative_int(row.get("kubernetes_pods"))
    if pods <= 0 or pods % MOCK_FLEET_KUBERNETES_PODS_PER_SCALE != 0:
        return False
    scale = pods // MOCK_FLEET_KUBERNETES_PODS_PER_SCALE
    vmware_hosts = parse_optional_nonnegative_int(row.get("vmware_hosts"))
    return vmware_hosts == scale * MOCK_FLEET_VMWARE_HOSTS_PER_SCALE


def classify_row_version(row: dict[str, Any], published_versions: set[str]) -> ClassifiedVersion:
    raw_version = str(row.get("version") or "")
    identity = classify_reported_version(raw_version, published_versions)

    stored_raw = str(row.get("version_raw") or "").strip()
    stored_channel = str(row.get("version_channel") or "").strip().lower()
    stored_build = str(row.get("version_build") or "").strip()
    stored_is_development = parse_optional_bool(row.get("version_is_development"))
    stored_is_published = parse_optional_bool(row.get("version_is_published_release"))

    if stored_raw:
        identity = ClassifiedVersion(
            raw_version=stored_raw,
            version=identity.version,
            channel=identity.channel,
            build=identity.build,
            is_development=identity.is_development,
            is_published_release=identity.is_published_release,
        )
    if stored_channel:
        identity = ClassifiedVersion(
            raw_version=identity.raw_version,
            version=identity.version,
            channel=stored_channel,
            build=identity.build,
            is_development=identity.is_development,
            is_published_release=identity.is_published_release,
        )
    if stored_build:
        identity = ClassifiedVersion(
            raw_version=identity.raw_version,
            version=identity.version,
            channel=identity.channel,
            build=stored_build,
            is_development=identity.is_development,
            is_published_release=identity.is_published_release,
        )
    if stored_is_development is not None:
        identity = ClassifiedVersion(
            raw_version=identity.raw_version,
            version=identity.version,
            channel=identity.channel,
            build=identity.build,
            is_development=stored_is_development,
            is_published_release=identity.is_published_release,
        )

    if published_versions:
        is_published_release = identity.version in published_versions
    elif stored_is_published is not None:
        is_published_release = stored_is_published
    else:
        is_published_release = identity.is_published_release

    return ClassifiedVersion(
        raw_version=identity.raw_version,
        version=identity.version,
        channel=identity.channel,
        build=identity.build,
        is_development=identity.is_development,
        is_published_release=is_published_release,
    )


def parse_received_at(raw: str) -> datetime:
    return datetime.strptime(raw, "%Y-%m-%d %H:%M:%S").replace(tzinfo=timezone.utc)


def normalize_release_tag(tag: str) -> str:
    version = tag.strip()
    if version.startswith("v"):
        version = version[1:]
    return version


def compare_semver_precedence(left: str, right: str) -> int | None:
    """Compare SemVer precedence while ignoring build metadata."""

    left_match = SEMVER_RE.match(normalize_release_tag(left))
    right_match = SEMVER_RE.match(normalize_release_tag(right))
    if left_match is None or right_match is None:
        return None

    left_core = tuple(int(left_match.group(name)) for name in ("major", "minor", "patch"))
    right_core = tuple(int(right_match.group(name)) for name in ("major", "minor", "patch"))
    if left_core != right_core:
        return -1 if left_core < right_core else 1

    left_prerelease = left_match.group("prerelease")
    right_prerelease = right_match.group("prerelease")
    if left_prerelease is None and right_prerelease is None:
        return 0
    if left_prerelease is None:
        return 1
    if right_prerelease is None:
        return -1

    left_identifiers = left_prerelease.split(".")
    right_identifiers = right_prerelease.split(".")
    for left_identifier, right_identifier in zip(left_identifiers, right_identifiers):
        if left_identifier == right_identifier:
            continue
        left_numeric = left_identifier.isdigit()
        right_numeric = right_identifier.isdigit()
        if left_numeric and right_numeric:
            return -1 if int(left_identifier) < int(right_identifier) else 1
        if left_numeric != right_numeric:
            return -1 if left_numeric else 1
        return -1 if left_identifier < right_identifier else 1
    if len(left_identifiers) == len(right_identifiers):
        return 0
    return -1 if len(left_identifiers) < len(right_identifiers) else 1


def fetch_published_releases(repo: str) -> list[dict[str, Any]]:
    releases: list[dict[str, Any]] = []
    page = 1
    while True:
        request = Request(
            f"https://api.github.com/repos/{repo}/releases?per_page=100&page={page}",
            headers={
                "Accept": "application/vnd.github+json",
                "User-Agent": "pulse-telemetry-adoption-report",
            },
        )
        with urlopen(request, timeout=15) as response:
            payload = json.loads(response.read().decode("utf-8"))
        if not payload:
            break
        for release in payload:
            if release.get("draft"):
                continue
            raw_tag = str(release.get("tag_name", "")).strip()
            version = normalize_release_tag(raw_tag)
            if version:
                releases.append(
                    {
                        "version": version,
                        "tag_name": raw_tag,
                        "is_prerelease": bool(release.get("prerelease")),
                        "published_at": str(release.get("published_at") or ""),
                    }
                )
        page += 1
    return releases


def fetch_published_versions(repo: str) -> set[str]:
    return {release["version"] for release in fetch_published_releases(repo)}


def latest_rc_version(releases: Iterable[dict[str, Any]]) -> str | None:
    rc_releases = [
        release
        for release in releases
        if release.get("is_prerelease") and version_channel(str(release.get("version") or "")) == "rc"
    ]
    if not rc_releases:
        return None
    latest = max(rc_releases, key=lambda release: str(release.get("published_at") or ""))
    return str(latest["version"])


def latest_target_release_version(releases: Iterable[dict[str, Any]]) -> str | None:
    release_list = list(releases)
    stable_releases = [
        release
        for release in release_list
        if not release.get("is_prerelease")
        and version_channel(str(release.get("version") or "")) == "stable"
    ]
    if stable_releases:
        latest = max(stable_releases, key=lambda release: str(release.get("published_at") or ""))
        return str(latest["version"])
    return latest_rc_version(release_list)


def fetch_rows_local(db_path: str, since_days: int) -> dict[str, Any]:
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    try:
        db_stats = dict(
            conn.execute(
                """
                SELECT
                  MAX(received_at) AS latest_ping,
                  COUNT(*) AS total_rows,
                  COUNT(DISTINCT install_id) AS total_distinct_installs
                FROM telemetry_pings
                """
            ).fetchone()
        )
        rows_sql = (
            f"SELECT {REPORT_ROW_PROJECTION} "
            "FROM telemetry_pings "
            "WHERE received_at >= datetime('now', ?) "
            "ORDER BY received_at DESC"
        )
        rows = [
            dict(row)
            for row in conn.execute(
                rows_sql,
                (f"-{since_days} days",),
            ).fetchall()
        ]
        return {"db_stats": db_stats, "rows": rows}
    finally:
        conn.close()


def fetch_rows_remote(
    ssh_host: str,
    db_path: str,
    since_days: int,
    target_version: str | None = None,
    baseline_version: str | None = None,
) -> dict[str, Any]:
    # Let SQLite aggregate the history into sufficient per-install evidence.
    # Only the latest row and compact per-install facts cross the network, not
    # the full heartbeat history.
    remote_script = """
import gzip
import json
import sqlite3
import sys

db_path = sys.argv[1]
since_days = int(sys.argv[2])
column_names = sys.argv[3].split(",")
intelligence_columns = sys.argv[4].split(",")
target_version = sys.argv[5]
target_activity_columns = sys.argv[6].split(",") if sys.argv[6] else []
baseline_version = sys.argv[7] if len(sys.argv) > 7 else ""
analysis_versions = [value for value in (target_version, baseline_version) if value]
if not column_names or any(
    not name or not name[0].isalpha() or not name.replace("_", "").isalnum()
    for name in column_names
):
    raise ValueError("invalid telemetry report column projection")
if not intelligence_columns or any(
    name not in column_names
    for name in intelligence_columns
):
    raise ValueError("invalid telemetry history signal projection")
if any(name not in column_names for name in target_activity_columns):
    raise ValueError("invalid target release activity projection")
conn = sqlite3.connect(db_path)
conn.row_factory = sqlite3.Row
available_columns = {
    row[1] for row in conn.execute("PRAGMA table_info(telemetry_pings)")
}
missing_columns = sorted(set(column_names) - available_columns)
row_projections = [
    name if name in available_columns else "0 AS " + name
    for name in column_names
]
db_stats_sql = (
    "SELECT MAX(received_at) AS latest_ping, "
    "COUNT(*) AS total_rows, "
    "COUNT(DISTINCT install_id) AS total_distinct_installs "
    "FROM telemetry_pings"
)
rows_sql = (
    "WITH ranked AS ("
    "SELECT " + ", ".join(row_projections) + ", "
    "ROW_NUMBER() OVER ("
    "PARTITION BY install_id ORDER BY received_at DESC, rowid DESC"
    ") AS latest_rank "
    "FROM telemetry_pings "
    "WHERE received_at >= datetime('now', ?)"
    ") SELECT " + ", ".join(column_names) + " "
    "FROM ranked WHERE latest_rank = 1"
)
analysis_selects = [
    "install_id",
    "MAX(received_at)",
    "MIN(CASE WHEN paid_license = 0 THEN received_at END)",
    "MIN(CASE WHEN paid_license = 1 THEN received_at END)",
]
for name in intelligence_columns:
    signal_expression = name if name in available_columns else "0"
    analysis_selects.append(
        "MAX(CASE WHEN " + signal_expression + " <> 0 THEN 1 ELSE 0 END)"
    )
    analysis_selects.append(
        "MIN(CASE WHEN paid_license = 0 AND " + signal_expression +
        " <> 0 THEN received_at END)"
    )
analysis_sql = (
    "SELECT " + ", ".join(analysis_selects) + " "
    "FROM telemetry_pings "
    "WHERE received_at >= datetime('now', ?) "
    "GROUP BY install_id"
)
target_analysis_sql = None
if analysis_versions:
    target_match = "(ordered_version = ? OR ordered_version = ?)"
    previous_target_match = "(previous_version = ? OR previous_version = ?)"
    pair_match = previous_target_match + " AND received_at <> previous_received_at"
    target_selects = [
        "install_id",
        "COUNT(*)",
        "SUM(CASE WHEN " + pair_match + " THEN 1 ELSE 0 END)",
        "MIN(received_at)",
        "MAX(received_at)",
        "MAX(CASE WHEN last_rank = 1 THEN COALESCE(schema_version, 0) END)",
        "MAX(CASE WHEN last_rank = 1 THEN COALESCE(known_install_age_bucket, 'unknown') END)",
    ]
    for name in target_activity_columns:
        target_selects.append(
            "MAX(CASE WHEN first_rank = 1 THEN COALESCE(" + name + ", 0) END)"
        )
        target_selects.append(
            "SUM(CASE WHEN " + pair_match + " AND " + name + " > previous_" + name +
            " THEN " + name + " - previous_" + name + " ELSE 0 END)"
        )
        target_selects.append(
            "SUM(CASE WHEN " + pair_match + " AND " + name + " < previous_" + name +
            " THEN previous_" + name + " - " + name + " ELSE 0 END)"
        )
    ordered_columns = [
        "install_id",
        "received_at",
        "rowid AS source_rowid",
        "TRIM(version) AS ordered_version",
        ("schema_version" if "schema_version" in available_columns else "0 AS schema_version"),
        ("known_install_age_bucket" if "known_install_age_bucket" in available_columns else "'unknown' AS known_install_age_bucket"),
        *(
            name if name in available_columns else "0 AS " + name
            for name in target_activity_columns
        ),
        "LAG(received_at) OVER install_order AS previous_received_at",
        "LAG(TRIM(version)) OVER install_order AS previous_version",
        *(
            "LAG(COALESCE(" + (name if name in available_columns else "0") +
            ", 0)) OVER install_order AS previous_" + name
            for name in target_activity_columns
        ),
    ]
    ordered_output_columns = [
        "install_id",
        "received_at",
        "source_rowid",
        "ordered_version",
        "schema_version",
        "known_install_age_bucket",
        *target_activity_columns,
        "previous_received_at",
        "previous_version",
        *("previous_" + name for name in target_activity_columns),
    ]
    target_analysis_sql = (
        "WITH ordered AS ("
        "SELECT " + ", ".join(ordered_columns) + " "
        "FROM telemetry_pings "
        "WHERE received_at >= datetime('now', ?) "
        "WINDOW install_order AS ("
        "PARTITION BY install_id ORDER BY received_at ASC, rowid ASC"
        ")"
        "), target_rows AS ("
        "SELECT " + ", ".join(ordered_output_columns) + ", "
        "ROW_NUMBER() OVER ("
        "PARTITION BY install_id ORDER BY received_at ASC, source_rowid ASC"
        ") AS first_rank "
        ", ROW_NUMBER() OVER ("
        "PARTITION BY install_id ORDER BY received_at DESC, source_rowid DESC"
        ") AS last_rank "
        "FROM ordered WHERE " + target_match +
        ") SELECT " + ", ".join(target_selects) + " "
        "FROM target_rows GROUP BY install_id"
    )
output = gzip.GzipFile(fileobj=sys.stdout.buffer, mode="wb", compresslevel=1)

def emit(value):
    output.write(json.dumps(value, separators=(",", ":")).encode("utf-8") + b"\\n")

try:
    db_stats = dict(conn.execute(db_stats_sql).fetchone())
    emit({
        "db_stats": db_stats,
        "row_columns": column_names,
        "unavailable_columns": missing_columns,
    })
    cutoff = f"-{since_days} days"
    for row in conn.execute(analysis_sql, (cutoff,)):
        first_paid_at = row[3]
        signal_fields = []
        free_signal_fields = []
        for signal_index, name in enumerate(intelligence_columns):
            value_index = 4 + (signal_index * 2)
            if row[value_index]:
                signal_fields.append(name)
            free_observed_at = row[value_index + 1]
            if free_observed_at is not None and (
                first_paid_at is None or free_observed_at < first_paid_at
            ):
                free_signal_fields.append(name)
        emit({"a": [
            row[0], row[1], row[2], first_paid_at,
            signal_fields, free_signal_fields,
        ]})
    if target_analysis_sql is not None:
        for analysis_version in analysis_versions:
            for row in conn.execute(
                target_analysis_sql,
                (
                    cutoff,
                    analysis_version,
                    "v" + analysis_version,
                    *(
                        value
                        for _ in range(1 + (len(target_activity_columns) * 2))
                        for value in (analysis_version, "v" + analysis_version)
                    ),
                ),
            ):
                first_counts = []
                increase_totals = []
                decrease_totals = []
                for signal_index in range(len(target_activity_columns)):
                    value_index = 7 + (signal_index * 3)
                    first_counts.append(row[value_index])
                    increase_totals.append(row[value_index + 1])
                    decrease_totals.append(row[value_index + 2])
                emit({"t": [
                    analysis_version, row[0], row[1], row[2], row[3], row[4],
                    row[5], row[6], first_counts, increase_totals, decrease_totals,
                ]})
    for row in conn.execute(rows_sql, (cutoff,)):
        emit({"r": list(row)})
finally:
    conn.close()
    output.close()
"""
    result = subprocess.run(
        [
            "ssh",
            ssh_host,
            "python3",
            "-",
            db_path,
            str(since_days),
            ",".join(REPORT_ROW_COLUMNS),
            ",".join(REPORT_HISTORY_SIGNAL_COLUMNS),
            normalize_release_tag(target_version or ""),
            ",".join(TARGET_RELEASE_ACTIVITY_COUNT_FIELDS),
            normalize_release_tag(baseline_version or ""),
        ],
        input=remote_script.encode("utf-8"),
        capture_output=True,
        check=True,
    )
    lines = (
        line
        for line in gzip.decompress(result.stdout).decode("utf-8").splitlines()
        if line.strip()
    )
    try:
        header = json.loads(next(lines))
    except StopIteration:
        raise RuntimeError(f"empty response from remote telemetry fetch on {ssh_host}") from None
    row_columns = header.get("row_columns")
    if row_columns != list(REPORT_ROW_COLUMNS):
        raise RuntimeError(f"invalid row schema from remote telemetry fetch on {ssh_host}")
    rows: list[dict[str, Any]] = []
    analysis_facts: list[dict[str, Any]] = []
    target_release_facts: list[dict[str, Any]] = []
    for line in lines:
        record = json.loads(line)
        if "r" in record:
            values = record["r"]
            if len(values) != len(row_columns):
                raise RuntimeError(f"invalid row from remote telemetry fetch on {ssh_host}")
            rows.append(dict(zip(row_columns, values)))
        elif "a" in record:
            values = record["a"]
            if len(values) != 6:
                raise RuntimeError(f"invalid analysis from remote telemetry fetch on {ssh_host}")
            analysis_facts.append(
                {
                    "install_id": values[0],
                    "latest_received_at": values[1],
                    "first_free_at": values[2],
                    "first_paid_at": values[3],
                    "signal_fields": values[4],
                    "free_signal_fields": values[5],
                }
            )
        elif "t" in record:
            values = record["t"]
            if len(values) not in {8, 11}:
                raise RuntimeError(f"invalid target release analysis from remote telemetry fetch on {ssh_host}")
            if len(values) == 8:
                values = [
                    normalize_release_tag(target_version or ""),
                    values[0], values[1], values[2], values[3], values[4],
                    0, "unknown", values[5], values[6], values[7],
                ]
            target_release_facts.append(
                {
                    "version": values[0],
                    "install_id": values[1],
                    "heartbeat_count": values[2],
                    "same_version_pair_count": values[3],
                    "first_received_at": values[4],
                    "latest_received_at": values[5],
                    "latest_schema_version": values[6],
                    "latest_known_install_age_bucket": values[7],
                    "first_counts": values[8],
                    "increase_totals": values[9],
                    "decrease_totals": values[10],
                }
            )
        else:
            raise RuntimeError(f"invalid record from remote telemetry fetch on {ssh_host}")
    return {
        "db_stats": header["db_stats"],
        "rows": rows,
        "pulse_intelligence_analysis_facts": analysis_facts,
        "target_release_analysis_facts": target_release_facts,
        "unavailable_columns": header.get("unavailable_columns", []),
    }


def counter_entries(counter: Counter[str], key_name: str) -> list[dict[str, Any]]:
    return [
        {key_name: value, "installs": installs}
        for value, installs in sorted(counter.items(), key=lambda item: (-item[1], item[0]))
    ]


def summarize_latest_install_windows(
    latest_by_install: dict[str, dict[str, Any]],
    published_versions: set[str],
    *,
    now: datetime | None = None,
    windows: tuple[tuple[str, timedelta], ...] = DEFAULT_LATEST_INSTALL_WINDOWS,
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    summary: dict[str, Any] = {}

    for label, limit in windows:
        version_split: Counter[str] = Counter()
        published_split: Counter[str] = Counter()
        non_release_split: Counter[str] = Counter()
        platform_split: Counter[str] = Counter()
        adoption_counts: Counter[str] = Counter()
        feature_counts: Counter[str] = Counter()

        for row in latest_by_install.values():
            received_at = parse_received_at(str(row["received_at"]))
            if current_time - received_at > limit:
                continue
            platform = str(row.get("platform") or "unknown").strip() or "unknown"
            identity = classify_row_version(row, published_versions)
            version_split[identity.version] += 1
            platform_split[platform] += 1
            target = published_split if identity.is_published_release else non_release_split
            target[identity.version] += 1
            for key, _ in ADOPTION_COUNT_FIELDS:
                adoption_counts[key] += parse_optional_nonnegative_int(row.get(key))
            for key, _ in FEATURE_BOOL_FIELDS:
                if parse_optional_bool(row.get(key)):
                    feature_counts[key] += 1

        summary[label] = {
            "active_installs": sum(version_split.values()),
            "latest_versions": counter_entries(version_split, "version"),
            "published_versions": counter_entries(published_split, "version"),
            "non_release_versions": counter_entries(non_release_split, "version"),
            "platforms": counter_entries(platform_split, "platform"),
            "adoption_counts": [
                {"field": key, "label": label, "total": adoption_counts[key]}
                for key, label in ADOPTION_COUNT_FIELDS
            ],
            "feature_enabled_installs": [
                {"field": key, "label": label, "installs": feature_counts[key]}
                for key, label in FEATURE_BOOL_FIELDS
            ],
        }

    return summary


def summarize_deep_signal_sources(
    latest_by_install: dict[str, dict[str, Any]],
    published_versions: set[str],
    *,
    now: datetime | None = None,
    window: timedelta = timedelta(days=7),
) -> list[dict[str, Any]]:
    current_time = now or datetime.now(timezone.utc)
    by_field: dict[str, dict[str, dict[str, Any]]] = {key: {} for key, _, _ in DEEP_SIGNAL_FIELDS}

    for row in latest_by_install.values():
        received_at = parse_received_at(str(row["received_at"]))
        if current_time - received_at > window:
            continue
        identity = classify_row_version(row, published_versions)

        for key, _, kind in DEEP_SIGNAL_FIELDS:
            if kind == "bool":
                value = 1 if parse_optional_bool(row.get(key)) else 0
            else:
                value = parse_optional_nonnegative_int(row.get(key))
            if value <= 0:
                continue

            source = by_field[key].setdefault(
                identity.version,
                {
                    "version": identity.version,
                    "installs": 0,
                    "total": 0,
                    "is_published_release": identity.is_published_release,
                },
            )
            source["installs"] += 1
            source["total"] += value
            source["is_published_release"] = source["is_published_release"] or identity.is_published_release

    result: list[dict[str, Any]] = []
    for key, label, kind in DEEP_SIGNAL_FIELDS:
        versions = list(by_field[key].values())
        if not versions:
            continue
        versions.sort(key=lambda source: (-int(source["installs"]), str(source["version"])))
        result.append(
            {
                "field": key,
                "label": label,
                "type": kind,
                "versions": versions,
            }
        )
    return result


def summarize_pulse_intelligence_value_loop(
    latest_by_install: dict[str, dict[str, Any]],
    *,
    now: datetime | None = None,
    window: timedelta = timedelta(days=7),
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    active_installs = 0
    paid_installs = 0
    free_installs = 0

    bool_signals = {
        field: {
            "field": field,
            "label": label,
            "installs": 0,
            "paid_installs": 0,
            "free_installs": 0,
        }
        for field, label in PULSE_INTELLIGENCE_BOOL_FIELDS
    }
    count_signals = {
        field: {
            "field": field,
            "label": label,
            "installs": 0,
            "paid_installs": 0,
            "free_installs": 0,
            "total": 0,
            "paid_total": 0,
            "free_total": 0,
        }
        for field, label in PULSE_INTELLIGENCE_COUNT_FIELDS
    }

    for row in latest_by_install.values():
        received_at = parse_received_at(str(row["received_at"]))
        if current_time - received_at > window:
            continue

        active_installs += 1
        paid = parse_optional_bool(row.get("paid_license")) is True
        if paid:
            paid_installs += 1
        else:
            free_installs += 1

        for field, _ in PULSE_INTELLIGENCE_BOOL_FIELDS:
            if not parse_optional_bool(row.get(field)):
                continue
            signal = bool_signals[field]
            signal["installs"] += 1
            if paid:
                signal["paid_installs"] += 1
            else:
                signal["free_installs"] += 1

        for field, _ in PULSE_INTELLIGENCE_COUNT_FIELDS:
            value = parse_optional_nonnegative_int(row.get(field))
            if value <= 0:
                continue
            signal = count_signals[field]
            signal["installs"] += 1
            signal["total"] += value
            if paid:
                signal["paid_installs"] += 1
                signal["paid_total"] += value
            else:
                signal["free_installs"] += 1
                signal["free_total"] += value

    count_signal_list = list(count_signals.values())
    totals = {signal["field"]: int(signal["total"]) for signal in count_signal_list}
    attempts = totals["pulse_intelligence_approved_action_attempts_30d"]
    accounted = sum(
        totals[field]
        for field in (
            "pulse_intelligence_approved_action_successes_30d",
            "pulse_intelligence_approved_action_failures_pre_dispatch_30d",
            "pulse_intelligence_approved_action_failures_execution_30d",
            "pulse_intelligence_approved_action_failures_unverified_30d",
            "pulse_intelligence_approved_action_stuck_executing_30d",
            "pulse_intelligence_approved_action_in_flight_30d",
            "pulse_intelligence_approved_action_unclassified_30d",
        )
    )
    pre_dispatch = totals["pulse_intelligence_approved_action_failures_pre_dispatch_30d"]
    refusal_accounted = sum(
        totals[field]
        for field in (
            "pulse_intelligence_approved_action_refusals_plan_stale_30d",
            "pulse_intelligence_approved_action_refusals_policy_30d",
            "pulse_intelligence_approved_action_refusals_capability_30d",
            "pulse_intelligence_approved_action_refusals_other_30d",
        )
    )

    return {
        "window": "7d",
        "active_installs": active_installs,
        "paid_installs": paid_installs,
        "free_installs": free_installs,
        "boolean_signals": list(bool_signals.values()),
        "count_signals": count_signal_list,
        "approved_action_outcome_accounting": {
            "attempts": attempts,
            "accounted": accounted,
            "gap": max(0, attempts - accounted),
            "overflow": max(0, accounted - attempts),
            "pre_dispatch_refusals": pre_dispatch,
            "refusal_categories_accounted": refusal_accounted,
            "refusal_category_gap": max(0, pre_dispatch - refusal_accounted),
            "refusal_category_overflow": max(0, refusal_accounted - pre_dispatch),
        },
    }


def pulse_intelligence_row_matches_cohort(
    row: dict[str, Any],
    bool_fields: tuple[str, ...],
    count_fields: tuple[str, ...],
) -> bool:
    return any(parse_optional_bool(row.get(field)) for field in bool_fields) or any(
        parse_optional_nonnegative_int(row.get(field)) > 0 for field in count_fields
    )


def pulse_intelligence_timed_rows(
    install_rows: Iterable[dict[str, Any]],
) -> list[tuple[datetime, dict[str, Any], bool | None]]:
    timed_rows = [
        (
            parse_received_at(str(row["received_at"])),
            row,
            parse_optional_bool(row.get("paid_license")),
        )
        for row in install_rows
    ]
    timed_rows.sort(key=lambda entry: entry[0])
    return timed_rows


def pulse_intelligence_row_analysis_keys(
    row: dict[str, Any],
) -> tuple[set[str], set[str]]:
    cohort_keys: set[str] = set()
    signal_groups: set[str] = set()
    for field in PULSE_INTELLIGENCE_ANALYSIS_BOOL_FIELDS:
        if not parse_optional_bool(row.get(field)):
            continue
        cohort_keys.update(PULSE_INTELLIGENCE_COHORT_BOOL_KEYS_BY_FIELD[field])
        signal_groups.update(PULSE_INTELLIGENCE_SIGNAL_GROUP_BOOL_KEYS_BY_FIELD[field])
    for field in PULSE_INTELLIGENCE_ANALYSIS_COUNT_FIELDS:
        if parse_optional_nonnegative_int(row.get(field)) <= 0:
            continue
        cohort_keys.update(PULSE_INTELLIGENCE_COHORT_COUNT_KEYS_BY_FIELD[field])
        signal_groups.update(PULSE_INTELLIGENCE_SIGNAL_GROUP_COUNT_KEYS_BY_FIELD[field])
    return cohort_keys, signal_groups


def pulse_intelligence_field_analysis_keys(
    fields: Iterable[str],
) -> tuple[set[str], set[str]]:
    cohort_keys: set[str] = set()
    signal_groups: set[str] = set()
    for field in fields:
        cohort_keys.update(PULSE_INTELLIGENCE_COHORT_BOOL_KEYS_BY_FIELD.get(field, ()))
        cohort_keys.update(PULSE_INTELLIGENCE_COHORT_COUNT_KEYS_BY_FIELD.get(field, ()))
        signal_groups.update(PULSE_INTELLIGENCE_SIGNAL_GROUP_BOOL_KEYS_BY_FIELD.get(field, ()))
        signal_groups.update(PULSE_INTELLIGENCE_SIGNAL_GROUP_COUNT_KEYS_BY_FIELD.get(field, ()))
    pulse_intelligence_derive_signal_groups(signal_groups)
    return cohort_keys, signal_groups


def analyze_pulse_intelligence_facts(
    facts: Iterable[dict[str, Any]],
) -> dict[str, PulseIntelligenceInstallAnalysis]:
    analyses: dict[str, PulseIntelligenceInstallAnalysis] = {}
    for fact in facts:
        install_id = str(fact.get("install_id") or "").strip()
        latest_raw = str(fact.get("latest_received_at") or "").strip()
        if not install_id or not latest_raw:
            continue
        first_free_raw = str(fact.get("first_free_at") or "").strip()
        first_paid_raw = str(fact.get("first_paid_at") or "").strip()
        first_free_at = parse_received_at(first_free_raw) if first_free_raw else None
        first_paid_at = parse_received_at(first_paid_raw) if first_paid_raw else None
        cohort_keys, signal_groups = pulse_intelligence_field_analysis_keys(
            fact.get("signal_fields") or ()
        )
        free_cohort_keys, free_signal_groups = pulse_intelligence_field_analysis_keys(
            fact.get("free_signal_fields") or ()
        )
        observed_free_start = first_free_at is not None and (
            first_paid_at is None or first_free_at < first_paid_at
        )
        analyses[install_id] = PulseIntelligenceInstallAnalysis(
            latest_received_at=parse_received_at(latest_raw),
            first_paid_at=first_paid_at,
            observed_free_start=observed_free_start,
            observed_free_to_paid=observed_free_start and first_paid_at is not None,
            cohort_keys=frozenset(cohort_keys),
            free_cohort_keys=frozenset(free_cohort_keys),
            signal_groups=frozenset(signal_groups),
            free_signal_groups=frozenset(free_signal_groups),
        )
    return analyses


def analyze_pulse_intelligence_install(
    install_rows: Iterable[dict[str, Any]],
) -> PulseIntelligenceInstallAnalysis:
    accumulator = _PulseIntelligenceInstallAccumulator()
    for row in install_rows:
        accumulator.observe(row)
    return accumulator.finalize()


def pulse_intelligence_first_paid_at(
    timed_rows: Iterable[tuple[datetime, dict[str, Any], bool | None]],
) -> datetime | None:
    paid_times = [received_at for received_at, _, posture in timed_rows if posture is True]
    return min(paid_times) if paid_times else None


def pulse_intelligence_observed_conversion(
    install_rows: Iterable[dict[str, Any]],
) -> tuple[bool, bool]:
    explicit_postures: list[tuple[datetime, bool]] = []
    for received_at, _, posture in pulse_intelligence_timed_rows(install_rows):
        if posture is not None:
            explicit_postures.append((received_at, posture))
    if not explicit_postures:
        return False, False
    first_free_at = next(
        (received_at for received_at, posture in explicit_postures if posture is False),
        None,
    )
    first_paid_at = next(
        (received_at for received_at, posture in explicit_postures if posture is True),
        None,
    )
    observed_free_start = first_free_at is not None and (
        first_paid_at is None or first_free_at < first_paid_at
    )
    observed_free_to_paid = (
        observed_free_start
        and first_paid_at is not None
        and first_free_at is not None
        and first_free_at < first_paid_at
    )
    return observed_free_start, observed_free_to_paid


def pulse_intelligence_signal_observed_conversion(
    install_rows: Iterable[dict[str, Any]],
    bool_fields: tuple[str, ...],
    count_fields: tuple[str, ...],
) -> tuple[bool, bool]:
    timed_rows = pulse_intelligence_timed_rows(install_rows)
    first_paid_at = pulse_intelligence_first_paid_at(timed_rows)
    first_free_signal_at: datetime | None = None
    for received_at, row, posture in timed_rows:
        if posture is not False:
            continue
        if first_paid_at is not None and received_at >= first_paid_at:
            continue
        if pulse_intelligence_row_matches_cohort(row, bool_fields, count_fields):
            first_free_signal_at = received_at
            break

    observed_free_signal = first_free_signal_at is not None
    observed_signal_to_paid = observed_free_signal and first_paid_at is not None
    return observed_free_signal, observed_signal_to_paid


def pulse_intelligence_row_signal_groups(row: dict[str, Any]) -> set[str]:
    groups: set[str] = set()
    for group, (bool_fields, count_fields) in PULSE_INTELLIGENCE_OPERATION_SIGNAL_GROUPS.items():
        if pulse_intelligence_row_matches_cohort(row, bool_fields, count_fields):
            groups.add(group)
    return pulse_intelligence_derive_signal_groups(groups)


def pulse_intelligence_derive_signal_groups(groups: set[str]) -> set[str]:
    if {"patrol_issue", "collaboration", "governed_decision"}.issubset(groups):
        groups.add("complete_operations_loop")
    if {"patrol_issue", "collaboration", "approved_execution"}.issubset(groups):
        groups.add("approved_execution_loop")
    if {"patrol_resolution", "collaboration", "approved_success"}.issubset(groups):
        groups.add("resolved_operations_loop")
    if {"patrol_issue", "assistant_collaboration", "governed_decision"}.issubset(groups):
        groups.add("assistant_operations_loop")
    if {"patrol_issue", "assistant_collaboration", "approved_execution"}.issubset(groups):
        groups.add("assistant_approved_execution_loop")
    if {"patrol_issue", "assistant_collaboration", "approved_success"}.issubset(groups):
        groups.add("assistant_approved_success_loop")
    if {"patrol_resolution", "assistant_collaboration", "approved_success"}.issubset(groups):
        groups.add("assistant_resolved_operations_loop")
    if {"patrol_issue", "external_agent_collaboration", "governed_decision"}.issubset(groups):
        groups.add("external_agent_operations_loop")
    if {"patrol_issue", "external_agent_collaboration", "approved_execution"}.issubset(groups):
        groups.add("external_agent_approved_execution_loop")
    if {"patrol_issue", "external_agent_collaboration", "approved_success"}.issubset(groups):
        groups.add("external_agent_approved_success_loop")
    if {"patrol_resolution", "external_agent_collaboration", "approved_success"}.issubset(groups):
        groups.add("external_agent_resolved_operations_loop")
    if {"patrol_issue", "mcp_adapter", "governed_decision"}.issubset(groups):
        groups.add("mcp_adapter_operations_loop")
    if {"patrol_issue", "mcp_adapter", "approved_execution"}.issubset(groups):
        groups.add("mcp_adapter_approved_execution_loop")
    if {"patrol_issue", "mcp_adapter", "approved_success"}.issubset(groups):
        groups.add("mcp_adapter_approved_success_loop")
    if {"patrol_resolution", "mcp_adapter", "approved_success"}.issubset(groups):
        groups.add("mcp_adapter_resolved_operations_loop")
    return groups


def pulse_intelligence_stage_signal_observed_conversion(
    install_rows: Iterable[dict[str, Any]],
    required_groups: tuple[str, ...],
) -> tuple[bool, bool]:
    timed_rows = pulse_intelligence_timed_rows(install_rows)
    first_paid_at = pulse_intelligence_first_paid_at(timed_rows)
    groups_seen_while_free: set[str] = set()
    for received_at, row, posture in timed_rows:
        if posture is not False:
            continue
        if first_paid_at is not None and received_at >= first_paid_at:
            continue
        groups_seen_while_free.update(pulse_intelligence_row_signal_groups(row))
    pulse_intelligence_derive_signal_groups(groups_seen_while_free)

    observed_free_signal = all(group in groups_seen_while_free for group in required_groups)
    observed_signal_to_paid = observed_free_signal and first_paid_at is not None
    return observed_free_signal, observed_signal_to_paid


def pulse_intelligence_rate_pct(part: int, total: int) -> float:
    if total <= 0 or part <= 0:
        return 0.0
    return round((part / total) * 100, 2)


def summarize_pulse_intelligence_install_outcomes(
    install_ids: Iterable[str],
    latest_by_install: dict[str, dict[str, Any]],
    rows_by_install: dict[str, list[dict[str, Any]]],
    current_time: datetime,
    retention_window: timedelta,
    signal_outcomes_by_install: dict[str, tuple[bool, bool]] | None = None,
    analysis_by_install: dict[str, PulseIntelligenceInstallAnalysis] | None = None,
) -> dict[str, int]:
    install_id_list = list(install_ids)
    retained_installs = 0
    paid_latest = 0
    free_latest = 0
    observed_free_starts = 0
    observed_free_to_paid = 0
    observed_signal_free_starts = 0
    observed_signal_free_to_paid = 0

    for install_id in install_id_list:
        latest = latest_by_install.get(install_id)
        if latest is None:
            continue
        analysis = (
            analysis_by_install.get(install_id)
            if analysis_by_install is not None
            else None
        )
        latest_received_at = (
            analysis.latest_received_at
            if analysis is not None
            else parse_received_at(str(latest["received_at"]))
        )
        if current_time - latest_received_at <= retention_window:
            retained_installs += 1
        if parse_optional_bool(latest.get("paid_license")) is True:
            paid_latest += 1
        else:
            free_latest += 1
        if analysis is not None:
            free_start = analysis.observed_free_start
            converted = analysis.observed_free_to_paid
        else:
            free_start, converted = pulse_intelligence_observed_conversion(
                rows_by_install.get(install_id, [])
            )
        if free_start:
            observed_free_starts += 1
        if converted:
            observed_free_to_paid += 1
        if signal_outcomes_by_install is not None:
            signal_free_start, signal_converted = signal_outcomes_by_install.get(
                install_id,
                (False, False),
            )
            if signal_free_start:
                observed_signal_free_starts += 1
            if signal_converted:
                observed_signal_free_to_paid += 1

    return {
        "installs": len(install_id_list),
        "retained_7d": retained_installs,
        "retained_7d_rate_pct": pulse_intelligence_rate_pct(
            retained_installs,
            len(install_id_list),
        ),
        "paid_latest": paid_latest,
        "paid_latest_rate_pct": pulse_intelligence_rate_pct(
            paid_latest,
            len(install_id_list),
        ),
        "free_latest": free_latest,
        "observed_free_starts": observed_free_starts,
        "observed_free_to_paid": observed_free_to_paid,
        "observed_free_to_paid_rate_pct": pulse_intelligence_rate_pct(
            observed_free_to_paid,
            observed_free_starts,
        ),
        "observed_signal_free_starts": observed_signal_free_starts,
        "observed_signal_free_to_paid": observed_signal_free_to_paid,
        "observed_signal_free_to_paid_rate_pct": pulse_intelligence_rate_pct(
            observed_signal_free_to_paid,
            observed_signal_free_starts,
        ),
    }


def analyze_pulse_intelligence_rows(
    rows: Iterable[dict[str, Any]],
) -> dict[str, PulseIntelligenceInstallAnalysis]:
    accumulators: dict[str, _PulseIntelligenceInstallAccumulator] = {}
    for row in rows:
        install_id = str(row.get("install_id") or "").strip()
        if not install_id:
            continue
        accumulator = accumulators.get(install_id)
        if accumulator is None:
            accumulator = _PulseIntelligenceInstallAccumulator()
            accumulators[install_id] = accumulator
        accumulator.observe(row)
    return {
        install_id: accumulator.finalize()
        for install_id, accumulator in accumulators.items()
    }


def summarize_pulse_intelligence_outcome_cohorts(
    rows: Iterable[dict[str, Any]],
    latest_by_install: dict[str, dict[str, Any]],
    *,
    now: datetime | None = None,
    retention_window: timedelta = timedelta(days=7),
    analysis_by_install: dict[str, PulseIntelligenceInstallAnalysis] | None = None,
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    cohort_install_ids: dict[str, set[str]] = {
        key: set() for key, _, _, _ in PULSE_INTELLIGENCE_OUTCOME_COHORTS
    }
    analyses = (
        analysis_by_install
        if analysis_by_install is not None
        else analyze_pulse_intelligence_rows(rows)
    )

    for install_id, analysis in analyses.items():
        for key in analysis.cohort_keys:
            cohort_install_ids[key].add(install_id)

    cohorts: list[dict[str, Any]] = []
    for key, label, _, _ in PULSE_INTELLIGENCE_OUTCOME_COHORTS:
        install_ids = cohort_install_ids[key]
        signal_outcomes_by_install = {
            install_id: (
                key in analyses[install_id].free_cohort_keys,
                key in analyses[install_id].free_cohort_keys
                and analyses[install_id].first_paid_at is not None,
            )
            for install_id in install_ids
        }
        cohorts.append(
            {
                "key": key,
                "label": label,
                **summarize_pulse_intelligence_install_outcomes(
                    install_ids,
                    latest_by_install,
                    {},
                    current_time,
                    retention_window,
                    signal_outcomes_by_install,
                    analyses,
                ),
            }
        )

    return {
        "retention_window": "7d",
        "cohorts": cohorts,
    }


def summarize_pulse_intelligence_operations_funnel(
    rows: Iterable[dict[str, Any]],
    latest_by_install: dict[str, dict[str, Any]],
    *,
    now: datetime | None = None,
    retention_window: timedelta = timedelta(days=7),
    analysis_by_install: dict[str, PulseIntelligenceInstallAnalysis] | None = None,
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    analyses = (
        analysis_by_install
        if analysis_by_install is not None
        else analyze_pulse_intelligence_rows(rows)
    )

    stages: list[dict[str, Any]] = []
    for key, label, required_groups in PULSE_INTELLIGENCE_OPERATIONS_FUNNEL_STAGES:
        install_ids = [
            install_id
            for install_id, analysis in analyses.items()
            if all(group in analysis.signal_groups for group in required_groups)
        ]
        signal_outcomes_by_install: dict[str, tuple[bool, bool]] = {}
        for install_id in install_ids:
            observed_free_signal = all(
                group in analyses[install_id].free_signal_groups
                for group in required_groups
            )
            signal_outcomes_by_install[install_id] = (
                observed_free_signal,
                observed_free_signal and analyses[install_id].first_paid_at is not None,
            )
        stages.append(
            {
                "key": key,
                "label": label,
                "required_signal_groups": list(required_groups),
                **summarize_pulse_intelligence_install_outcomes(
                    install_ids,
                    latest_by_install,
                    {},
                    current_time,
                    retention_window,
                    signal_outcomes_by_install,
                    analyses,
                ),
            }
        )

    return {
        "retention_window": "7d",
        "stages": stages,
    }


def telemetry_signal_specs() -> list[dict[str, str]]:
    deep_fields = {key for key, _, _ in DEEP_SIGNAL_FIELDS}
    specs: list[dict[str, str]] = []
    for key, label in ADOPTION_COUNT_FIELDS:
        specs.append(
            {
                "field": key,
                "label": label,
                "type": "count",
                "group": "deep" if key in deep_fields else "core",
            }
        )
    for key, label in FEATURE_BOOL_FIELDS:
        specs.append(
            {
                "field": key,
                "label": label,
                "type": "bool",
                "group": "deep" if key in deep_fields else "core",
            }
        )
    return specs


def summarize_user_base_signals(
    latest_by_install: dict[str, dict[str, Any]],
    *,
    now: datetime | None = None,
    window: timedelta = timedelta(days=7),
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    active_rows = [
        row
        for row in latest_by_install.values()
        if current_time - parse_received_at(str(row["received_at"])) <= window
    ]

    schema_versions: Counter[str] = Counter()
    categories: dict[str, Counter[str]] = {
        field: Counter() for field, _ in USER_BASE_CATEGORY_FIELDS
    }
    boolean_signals = {
        field: {"field": field, "label": label, "installs": 0}
        for field, label in USER_BASE_BOOL_FIELDS
    }
    count_signals = {
        field: {"field": field, "label": label, "installs": 0, "total": 0}
        for field, label in USER_BASE_COUNT_FIELDS + NOTIFICATION_FAILURE_COUNT_SIGNALS
    }

    for row in active_rows:
        schema_version = parse_optional_nonnegative_int(row.get("schema_version"))
        schema_versions[str(schema_version or "legacy")] += 1
        for field, _ in USER_BASE_CATEGORY_FIELDS:
            fallback = (
                "not_reported"
                if field == "update_last_failure_category"
                else "legacy_unknown"
            )
            value = str(row.get(field) or fallback).strip() or fallback
            categories[field][value] += 1
        for field, _ in USER_BASE_BOOL_FIELDS:
            if parse_optional_bool(row.get(field)):
                boolean_signals[field]["installs"] += 1
        for field, _ in USER_BASE_COUNT_FIELDS:
            value = parse_optional_nonnegative_int(row.get(field))
            if value > 0:
                count_signals[field]["installs"] += 1
                count_signals[field]["total"] += value
        failure_value = parse_optional_nonnegative_int(row.get("notification_failures_7d"))
        if failure_value > 0:
            failure_field = (
                "notification_terminal_failures_7d_schema_v3"
                if schema_version >= 3
                else "notification_attempt_failures_7d_schema_v2"
            )
            count_signals[failure_field]["installs"] += 1
            count_signals[failure_field]["total"] += failure_value

    return {
        "window": "7d",
        "active_installs": len(active_rows),
        "schema_versions": counter_entries(schema_versions, "version"),
        "category_signals": [
            {
                "field": field,
                "label": label,
                "buckets": counter_entries(categories[field], "bucket"),
            }
            for field, label in USER_BASE_CATEGORY_FIELDS
        ],
        "boolean_signals": list(boolean_signals.values()),
        "count_signals": list(count_signals.values()),
    }


def analyze_target_release_rows(
    rows: Iterable[dict[str, Any]],
    published_versions: set[str],
    target_version: str,
) -> dict[str, TargetReleaseInstallAnalysis]:
    normalized_target = normalize_release_tag(target_version)
    accumulators: dict[str, _TargetReleaseInstallAccumulator] = {}
    for row in rows:
        install_id = str(row["install_id"])
        accumulator = accumulators.setdefault(
            install_id,
            _TargetReleaseInstallAccumulator(),
        )
        accumulator.observe(
            row,
            parse_received_at(str(row["received_at"])),
            classify_row_version(row, published_versions).version == normalized_target,
        )
    return {
        install_id: accumulator.finalize()
        for install_id, accumulator in accumulators.items()
        if accumulator.heartbeat_count > 0
    }


def analyze_target_release_facts(
    facts: Iterable[dict[str, Any]],
    target_version: str | None = None,
) -> dict[str, TargetReleaseInstallAnalysis]:
    analyses: dict[str, TargetReleaseInstallAnalysis] = {}
    expected_count_length = len(TARGET_RELEASE_ACTIVITY_COUNT_FIELDS)
    normalized_target = normalize_release_tag(target_version or "")
    for fact in facts:
        fact_version = normalize_release_tag(str(fact.get("version") or ""))
        if normalized_target and fact_version and fact_version != normalized_target:
            continue
        first_counts = tuple(
            parse_optional_nonnegative_int(value)
            for value in fact.get("first_counts") or ()
        )
        increase_totals = tuple(
            parse_optional_nonnegative_int(value)
            for value in fact.get("increase_totals") or ()
        )
        decrease_totals = tuple(
            parse_optional_nonnegative_int(value)
            for value in fact.get("decrease_totals") or ()
        )
        if (
            len(first_counts) != expected_count_length
            or len(increase_totals) != expected_count_length
            or len(decrease_totals) != expected_count_length
        ):
            raise ValueError("invalid target release activity fact count projection")
        analyses[str(fact["install_id"])] = TargetReleaseInstallAnalysis(
            first_received_at=parse_received_at(str(fact["first_received_at"])),
            latest_received_at=parse_received_at(str(fact["latest_received_at"])),
            heartbeat_count=parse_optional_nonnegative_int(fact.get("heartbeat_count")),
            same_version_pair_count=parse_optional_nonnegative_int(
                fact.get("same_version_pair_count")
            ),
            first_counts=first_counts,
            increase_totals=increase_totals,
            decrease_totals=decrease_totals,
            latest_schema_version=parse_optional_nonnegative_int(
                fact.get("latest_schema_version")
            ),
            latest_known_install_age_bucket=(
                str(fact.get("latest_known_install_age_bucket") or "unknown").strip()
                or "unknown"
            ),
        )
    return analyses


def summarize_target_release_followup(
    latest_by_install: dict[str, dict[str, Any]],
    analyses_by_install: dict[str, TargetReleaseInstallAnalysis],
    published_versions: set[str],
    target_version: str,
) -> dict[str, Any]:
    normalized_target = normalize_release_tag(target_version)
    target_identity = classify_reported_version(normalized_target, published_versions)
    analyses = {
        install_id: analysis
        for install_id, analysis in analyses_by_install.items()
        if install_id in latest_by_install
    }
    return _summarize_target_release_followup_analysis(
        latest_by_install,
        published_versions,
        normalized_target,
        target_identity,
        analyses,
    )


def _heartbeat_exposure_bucket(heartbeat_count: int) -> str:
    if heartbeat_count <= 1:
        return "1"
    if heartbeat_count == 2:
        return "2"
    if heartbeat_count <= 6:
        return "3_6"
    return "7_plus"


def _alert_quality_release_cohort(
    version: str,
    analyses: dict[str, TargetReleaseInstallAnalysis],
) -> dict[str, Any]:
    age_buckets: Counter[str] = Counter()
    heartbeat_buckets: Counter[str] = Counter()
    schema_versions: Counter[str] = Counter()
    schema_capable = 0
    comparable = 0
    for analysis in analyses.values():
        age_buckets[analysis.latest_known_install_age_bucket] += 1
        heartbeat_buckets[_heartbeat_exposure_bucket(analysis.heartbeat_count)] += 1
        schema_versions[str(analysis.latest_schema_version)] += 1
        if analysis.latest_schema_version >= ALERT_QUALITY_SCHEMA_VERSION:
            schema_capable += 1
            if analysis.same_version_pair_count > 0:
                comparable += 1
    return {
        "version": normalize_release_tag(version),
        "installs": len(analyses),
        "heartbeat_rows": sum(item.heartbeat_count for item in analyses.values()),
        "first_heartbeat_baseline_only_installs": sum(
            1 for item in analyses.values() if item.same_version_pair_count == 0
        ),
        "schema_capable_installs": schema_capable,
        "same_version_comparable_installs": comparable,
        "known_age_buckets": counter_entries(age_buckets, "bucket"),
        "heartbeat_count_buckets": counter_entries(heartbeat_buckets, "bucket"),
        "schema_versions": counter_entries(schema_versions, "schema_version"),
    }


def summarize_alert_quality_release_comparison(
    target_version: str,
    baseline_version: str,
    target_analyses: dict[str, TargetReleaseInstallAnalysis],
    baseline_analyses: dict[str, TargetReleaseInstallAnalysis],
) -> dict[str, Any]:
    target_cohort = _alert_quality_release_cohort(target_version, target_analyses)
    baseline_cohort = _alert_quality_release_cohort(baseline_version, baseline_analyses)
    result: dict[str, Any] = {
        "target": target_cohort,
        "baseline": baseline_cohort,
        "matched_exposure_strata": [],
        "interpretation": {
            "first_heartbeat": (
                "The first heartbeat for a version is baseline-only. Alert-quality changes require "
                "a later consecutive heartbeat from the same version and install."
            ),
            "exposure": (
                "Comparisons are separated by the latest known-install-age bucket and version heartbeat-count "
                "bucket. Unmatched strata are not pooled into a release outcome."
            ),
            "rolling_window": (
                "Positive and negative rolling-window changes remain separate. Ratios below use positive "
                "same-version changes only and are descriptive, not causal release attribution."
            ),
        },
    }
    if target_cohort["schema_capable_installs"] == 0 or baseline_cohort["schema_capable_installs"] == 0:
        result["status"] = "schema_not_comparable"
        result["reason"] = (
            f"Both releases need telemetry schema {ALERT_QUALITY_SCHEMA_VERSION} or newer. "
            f"{normalize_release_tag(target_version)} and {normalize_release_tag(baseline_version)} "
            "cannot be compared with alert-quality outcomes from legacy volume counters."
        )
        return result

    quality_index = {
        field: TARGET_RELEASE_ACTIVITY_COUNT_FIELDS.index(field)
        for field in (
            "alerts_fired_30d",
            "alerts_resolved_30d",
            "alerts_repeat_occurrences_30d",
            "alerts_snoozed_occurrences_30d",
            "alerts_resolved_while_snoozed_30d",
            "alerts_resolution_under_15m_30d",
            "alerts_resolution_15m_1h_30d",
            "alerts_resolution_1h_24h_30d",
            "alerts_resolution_1d_7d_30d",
            "alerts_resolution_7d_plus_30d",
        )
    }

    def stratify(
        analyses: dict[str, TargetReleaseInstallAnalysis],
    ) -> dict[tuple[str, str], list[TargetReleaseInstallAnalysis]]:
        strata: dict[tuple[str, str], list[TargetReleaseInstallAnalysis]] = defaultdict(list)
        for analysis in analyses.values():
            if (
                analysis.latest_schema_version < ALERT_QUALITY_SCHEMA_VERSION
                or analysis.same_version_pair_count == 0
            ):
                continue
            strata[(
                analysis.latest_known_install_age_bucket,
                _heartbeat_exposure_bucket(analysis.heartbeat_count),
            )].append(analysis)
        return strata

    def summarize_stratum(items: list[TargetReleaseInstallAnalysis]) -> dict[str, Any]:
        totals = {
            field: sum(item.increase_totals[index] for item in items)
            for field, index in quality_index.items()
        }
        fired = totals["alerts_fired_30d"]
        resolved = totals["alerts_resolved_30d"]
        snoozed = totals["alerts_snoozed_occurrences_30d"]
        return {
            "installs": len(items),
            "same_version_pairs": sum(item.same_version_pair_count for item in items),
            "positive_changes": totals,
            "resolution_per_fired": (resolved / fired) if fired else None,
            "repeat_per_fired": (
                totals["alerts_repeat_occurrences_30d"] / fired if fired else None
            ),
            "resolved_while_snoozed_per_snoozed": (
                totals["alerts_resolved_while_snoozed_30d"] / snoozed
                if snoozed
                else None
            ),
        }

    target_strata = stratify(target_analyses)
    baseline_strata = stratify(baseline_analyses)
    for age_bucket, heartbeat_bucket in sorted(set(target_strata) & set(baseline_strata)):
        key = (age_bucket, heartbeat_bucket)
        result["matched_exposure_strata"].append({
            "known_install_age_bucket": age_bucket,
            "heartbeat_count_bucket": heartbeat_bucket,
            "target": summarize_stratum(target_strata[key]),
            "baseline": summarize_stratum(baseline_strata[key]),
        })
    if not result["matched_exposure_strata"]:
        result["status"] = "no_matched_exposure"
        result["reason"] = (
            "Schema-capable rows exist, but no known-age and heartbeat-count stratum has "
            "same-version follow-up in both releases."
        )
    else:
        result["status"] = "descriptive_matched_strata"
    return result


def _summarize_target_release_followup_analysis(
    latest_by_install: dict[str, dict[str, Any]],
    published_versions: set[str],
    normalized_target: str,
    target_identity: ClassifiedVersion,
    analyses: dict[str, TargetReleaseInstallAnalysis],
) -> dict[str, Any]:
    signals = {
        field: {
            "field": field,
            "label": TARGET_RELEASE_ACTIVITY_LABELS[field],
            "first_heartbeat_baseline_nonzero_installs": 0,
            "first_heartbeat_baseline_total": 0,
            "same_version_increased_installs": 0,
            "same_version_total_increase": 0,
            "same_version_decreased_installs": 0,
            "same_version_total_decrease": 0,
            "same_version_unchanged_installs": 0,
            "current_target_increased_installs": 0,
            "current_target_total_increase": 0,
            "current_target_decreased_installs": 0,
            "current_target_total_decrease": 0,
            "departed_increased_installs": 0,
            "departed_total_increase": 0,
            "departed_decreased_installs": 0,
            "departed_total_decrease": 0,
        }
        for field in TARGET_RELEASE_ACTIVITY_COUNT_FIELDS
    }
    current_target_installs = 0
    same_version_followup_installs = 0
    current_target_followup_installs = 0
    departed_followup_installs = 0
    without_later_same_version_heartbeat = 0
    target_heartbeat_rows = 0
    transition_counts: dict[str, Counter[str]] = {
        "rollback": Counter(),
        "forward": Counter(),
        "unclassified": Counter(),
    }

    for install_id, analysis in analyses.items():
        target_heartbeat_rows += analysis.heartbeat_count
        has_same_version_followup = analysis.same_version_pair_count > 0
        latest_row = latest_by_install[install_id]
        latest_identity = classify_row_version(latest_row, published_versions)
        is_current_target = latest_identity.version == normalized_target
        if is_current_target:
            current_target_installs += 1
        if has_same_version_followup:
            same_version_followup_installs += 1
            if is_current_target:
                current_target_followup_installs += 1
            else:
                departed_followup_installs += 1
        else:
            without_later_same_version_heartbeat += 1

        for index, field in enumerate(TARGET_RELEASE_ACTIVITY_COUNT_FIELDS):
            signal = signals[field]
            first_value = analysis.first_counts[index]
            if first_value > 0:
                signal["first_heartbeat_baseline_nonzero_installs"] += 1
                signal["first_heartbeat_baseline_total"] += first_value
            if not has_same_version_followup:
                continue
            increase = analysis.increase_totals[index]
            decrease = analysis.decrease_totals[index]
            if increase > 0:
                signal["same_version_increased_installs"] += 1
                signal["same_version_total_increase"] += increase
                destination = "current_target" if is_current_target else "departed"
                signal[f"{destination}_increased_installs"] += 1
                signal[f"{destination}_total_increase"] += increase
            if decrease > 0:
                signal["same_version_decreased_installs"] += 1
                signal["same_version_total_decrease"] += decrease
                destination = "current_target" if is_current_target else "departed"
                signal[f"{destination}_decreased_installs"] += 1
                signal[f"{destination}_total_decrease"] += decrease
            if increase == 0 and decrease == 0:
                signal["same_version_unchanged_installs"] += 1

        if is_current_target:
            continue
        latest_received_at = parse_received_at(str(latest_row["received_at"]))
        if latest_received_at <= analysis.latest_received_at:
            continue
        transition_kind = "unclassified"
        if (
            target_identity.channel in {"stable", "rc", "prerelease"}
            and latest_identity.channel in {"stable", "rc", "prerelease"}
            and latest_identity.is_published_release
        ):
            comparison = compare_semver_precedence(latest_identity.version, normalized_target)
            if comparison is not None and comparison < 0:
                transition_kind = "rollback"
            elif comparison is not None and comparison > 0:
                transition_kind = "forward"
        transition_counts[transition_kind][latest_identity.version] += 1

    def transition_entries(kind: str) -> list[dict[str, Any]]:
        return counter_entries(transition_counts[kind], "destination_version")

    return {
        "version": normalized_target,
        "installs_seen": len(analyses),
        "target_heartbeat_rows": target_heartbeat_rows,
        "current_target_installs": current_target_installs,
        "same_version_followup_installs": same_version_followup_installs,
        "current_target_followup_installs": current_target_followup_installs,
        "departed_followup_installs": departed_followup_installs,
        "without_later_same_version_heartbeat": without_later_same_version_heartbeat,
        "departed_after_target_installs": sum(
            sum(counter.values()) for counter in transition_counts.values()
        ),
        "rollback_installs": sum(transition_counts["rollback"].values()),
        "forward_transition_installs": sum(transition_counts["forward"].values()),
        "unclassified_transition_installs": sum(transition_counts["unclassified"].values()),
        "rollback_transitions": transition_entries("rollback"),
        "forward_transitions": transition_entries("forward"),
        "unclassified_transitions": transition_entries("unclassified"),
        "activity_signals": list(signals.values()),
        "interpretation": {
            "first_heartbeat": (
                "Rolling counters on the first target-version heartbeat in the source window "
                "are baseline only and are not attributed to the target release."
            ),
            "same_version_change": (
                "Activity is the observed counter change across consecutive target-version "
                "heartbeats from the same pseudonymous install; a version departure breaks "
                "the comparison chain."
            ),
            "counter_decrease": (
                "Decreases are reported separately because rolling windows and local resets can "
                "reduce a counter; they are never subtracted from observed increases."
            ),
        },
    }


def summarize_target_version_coverage(
    latest_by_install: dict[str, dict[str, Any]],
    published_versions: set[str],
    target_version: str,
    *,
    now: datetime | None = None,
    window: timedelta = timedelta(days=7),
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    normalized_target = normalize_release_tag(target_version)
    platform_split: Counter[str] = Counter()
    target_rows: list[dict[str, Any]] = []

    for row in latest_by_install.values():
        received_at = parse_received_at(str(row["received_at"]))
        if current_time - received_at > window:
            continue
        identity = classify_row_version(row, published_versions)
        if identity.version != normalized_target:
            continue
        target_rows.append(row)
        platform = str(row.get("platform") or "unknown").strip() or "unknown"
        platform_split[platform] += 1

    signals: list[dict[str, Any]] = []
    for spec in telemetry_signal_specs():
        values: list[int] = []
        for row in target_rows:
            if spec["type"] == "bool":
                values.append(1 if parse_optional_bool(row.get(spec["field"])) else 0)
            else:
                values.append(parse_optional_nonnegative_int(row.get(spec["field"])))
        signals.append(
            {
                **spec,
                "nonzero_installs": sum(1 for value in values if value > 0),
                "total": sum(values),
            }
        )

    return {
        "version": normalized_target,
        "active_installs": len(target_rows),
        "platforms": counter_entries(platform_split, "platform"),
        "signals": signals,
    }


def summarize_target_release_service_health(
    latest_by_install: dict[str, dict[str, Any]],
    published_versions: set[str],
    target_version: str,
    *,
    now: datetime | None = None,
    window: timedelta = timedelta(days=7),
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    normalized_target = normalize_release_tag(target_version)
    target_rows = [
        row
        for row in latest_by_install.values()
        if current_time - parse_received_at(str(row["received_at"])) <= window
        and classify_row_version(row, published_versions).version == normalized_target
    ]

    failure_categories: Counter[str] = Counter()
    cohorts: Counter[str] = Counter()
    previous_versions: Counter[str] = Counter()
    transitions: Counter[str] = Counter()
    observed = 0
    healthy = 0
    comparable_version_changes = 0

    for row in target_rows:
        if not parse_optional_bool(row.get("service_health_observed")):
            continue
        observed += 1
        current_healthy = parse_optional_bool(row.get("service_health_healthy"))
        if current_healthy:
            healthy += 1
            failure_categories["healthy"] += 1
        else:
            category = str(row.get("service_health_failure_category") or "unknown").strip() or "unknown"
            failure_categories[category] += 1

        cohort = str(row.get("service_health_cohort") or "unknown").strip() or "unknown"
        cohorts[cohort] += 1
        if not parse_optional_bool(row.get("service_health_previous_observed")):
            continue

        previous_version = normalize_release_tag(str(row.get("service_health_previous_version") or ""))
        if not previous_version or previous_version == normalized_target:
            continue
        comparable_version_changes += 1
        previous_versions[previous_version] += 1
        previous_healthy = parse_optional_bool(row.get("service_health_previous_healthy"))
        transition = (
            ("healthy" if previous_healthy else "unhealthy")
            + "_to_"
            + ("healthy" if current_healthy else "unhealthy")
        )
        transitions[transition] += 1

    return {
        "version": normalized_target,
        "window": "7d",
        "target_installs": len(target_rows),
        "observed_installs": observed,
        "unobserved_installs": len(target_rows) - observed,
        "healthy_installs": healthy,
        "unhealthy_installs": observed - healthy,
        "failure_categories": counter_entries(failure_categories, "category"),
        "cohorts": counter_entries(cohorts, "cohort"),
        "comparable_version_change_installs": comparable_version_changes,
        "previous_versions": counter_entries(previous_versions, "version"),
        "transitions": counter_entries(transitions, "transition"),
        "interpretation": (
            "These are direct latest per-install local service observations. "
            "Version-change transitions compare the retained immediately previous release observation "
            "with the target release and do not attribute rolling historical counters to the target."
        ),
    }


def summarize_rows(
    db_stats: dict[str, Any],
    rows: Iterable[dict[str, Any]],
    published_versions: set[str],
    target_version: str | None = None,
    baseline_version: str | None = None,
    include_mock_fleet: bool = False,
    *,
    now: datetime | None = None,
    source_window_days: int | None = None,
    pulse_intelligence_analysis_facts: Iterable[dict[str, Any]] | None = None,
    target_release_analysis_facts: Iterable[dict[str, Any]] | None = None,
) -> dict[str, Any]:
    row_list: list[dict[str, Any]] = []
    mock_fleet_rows = 0
    mock_fleet_installs: set[str] = set()
    for row in rows:
        if not include_mock_fleet and is_mock_fleet_row(row):
            mock_fleet_rows += 1
            mock_fleet_installs.add(str(row["install_id"]))
            continue
        row_list.append(row)

    latest_by_install: dict[str, dict[str, Any]] = {}
    for row in row_list:
        install_id = str(row["install_id"])
        existing = latest_by_install.get(install_id)
        if existing is None or str(row["received_at"]) > str(existing["received_at"]):
            latest_by_install[install_id] = row

    current_time = now or datetime.now(timezone.utc)
    pulse_intelligence_analysis = (
        analyze_pulse_intelligence_facts(pulse_intelligence_analysis_facts)
        if pulse_intelligence_analysis_facts is not None
        else analyze_pulse_intelligence_rows(row_list)
    )
    pulse_intelligence_analysis = {
        install_id: analysis
        for install_id, analysis in pulse_intelligence_analysis.items()
        if install_id in latest_by_install
    }
    release_fact_list = (
        list(target_release_analysis_facts)
        if target_release_analysis_facts is not None
        else None
    )
    target_release_analysis: dict[str, TargetReleaseInstallAnalysis] = {}
    if target_version:
        target_release_analysis = (
            analyze_target_release_facts(release_fact_list, target_version)
            if release_fact_list is not None
            else analyze_target_release_rows(
                row_list,
                published_versions,
                target_version,
            )
        )
    baseline_release_analysis: dict[str, TargetReleaseInstallAnalysis] = {}
    if baseline_version:
        baseline_release_analysis = (
            analyze_target_release_facts(release_fact_list, baseline_version)
            if release_fact_list is not None
            else analyze_target_release_rows(
                row_list,
                published_versions,
                baseline_version,
            )
        )
    latest_install_windows = summarize_latest_install_windows(
        latest_by_install,
        published_versions,
        now=current_time,
    )
    summary_72h = latest_install_windows["72h"]
    summary_7d = latest_install_windows["7d"]
    target_release_followup = None
    if target_version:
        target_release_followup = summarize_target_release_followup(
            latest_by_install,
            target_release_analysis,
            published_versions,
            target_version,
        )
        if source_window_days is not None:
            target_release_followup["source_window_days"] = source_window_days

    return {
        "db_stats": db_stats,
        "mock_fleet_exclusions": {
            "enabled": not include_mock_fleet,
            "rows": mock_fleet_rows,
            "installs": len(mock_fleet_installs),
        },
        "latest_install_windows": latest_install_windows,
        "user_base_signals_7d": summarize_user_base_signals(
            latest_by_install,
            now=current_time,
        ),
        "deep_signal_sources_7d": summarize_deep_signal_sources(
            latest_by_install,
            published_versions,
            now=current_time,
        ),
        "pulse_intelligence_value_loop_7d": summarize_pulse_intelligence_value_loop(
            latest_by_install,
            now=current_time,
        ),
        "pulse_intelligence_outcome_cohorts": summarize_pulse_intelligence_outcome_cohorts(
            row_list,
            latest_by_install,
            now=current_time,
            analysis_by_install=pulse_intelligence_analysis,
        ),
        "pulse_intelligence_operations_loop_funnel": summarize_pulse_intelligence_operations_funnel(
            row_list,
            latest_by_install,
            now=current_time,
            analysis_by_install=pulse_intelligence_analysis,
        ),
        "target_release_coverage_7d": summarize_target_version_coverage(
            latest_by_install,
            published_versions,
            target_version,
            now=current_time,
        )
        if target_version
        else None,
        "target_release_service_health_7d": summarize_target_release_service_health(
            latest_by_install,
            published_versions,
            target_version,
            now=current_time,
        )
        if target_version
        else None,
        "target_release_followup": target_release_followup,
        "alert_quality_release_comparison": summarize_alert_quality_release_comparison(
            target_version,
            baseline_version,
            target_release_analysis,
            baseline_release_analysis,
        )
        if target_version and baseline_version
        else None,
        "active_latest": {
            "active_24h": latest_install_windows["24h"]["active_installs"],
            "active_72h": summary_72h["active_installs"],
            "active_7d": summary_7d["active_installs"],
        },
        "latest_version_split_72h": summary_72h["latest_versions"],
        "published_version_split_72h": summary_72h["published_versions"],
        "non_release_version_split_72h": summary_72h["non_release_versions"],
        "latest_platform_split_72h": summary_72h["platforms"],
    }


def format_target_signal(signal: dict[str, Any]) -> str:
    install_word = "install" if signal["nonzero_installs"] == 1 else "installs"
    text = f"{signal['label']}: {signal['nonzero_installs']} {install_word}"
    if signal["type"] == "count":
        text += f", total {signal['total']}"
    return text


def format_paid_install_split(installs: int, paid_installs: int, free_installs: int) -> str:
    install_word = "install" if installs == 1 else "installs"
    return f"{installs} {install_word} (paid {paid_installs}, free/community {free_installs})"


def format_paid_count_split(signal: dict[str, Any]) -> str:
    install_word = "install" if signal["installs"] == 1 else "installs"
    return (
        f"{signal['installs']} {install_word}, total {signal['total']} "
        f"(paid {signal['paid_installs']} / {signal['paid_total']}; "
        f"free/community {signal['free_installs']} / {signal['free_total']})"
    )


def format_rate(part: int, total: int) -> str:
    if total <= 0:
        return "0.0%"
    return f"{(part / total) * 100:.1f}%"


def format_pulse_intelligence_cohort(entry: dict[str, Any]) -> str:
    install_word = "install" if entry["installs"] == 1 else "installs"
    observed_free_starts = int(entry.get("observed_free_starts", 0))
    observed_free_to_paid = int(entry.get("observed_free_to_paid", 0))
    observed_signal_free_starts = int(entry.get("observed_signal_free_starts", 0))
    observed_signal_free_to_paid = int(entry.get("observed_signal_free_to_paid", 0))
    return (
        f"{entry['label']}: {entry['installs']} {install_word}, "
        f"retained 7d {entry['retained_7d']} ({format_rate(entry['retained_7d'], entry['installs'])}), "
        f"latest paid {entry['paid_latest']}, latest free/community {entry['free_latest']}, "
        f"observed free/community starts {observed_free_starts}, "
        f"free-to-paid {observed_free_to_paid} ({format_rate(observed_free_to_paid, observed_free_starts)}), "
        f"signal while free/community {observed_signal_free_starts}, "
        f"signal-to-paid {observed_signal_free_to_paid} "
        f"({format_rate(observed_signal_free_to_paid, observed_signal_free_starts)})"
    )


def format_text(summary: dict[str, Any], repo: str, since_days: int) -> str:
    lines = [
        "Pulse telemetry adoption report",
        f"source window: last {since_days} day(s)",
        f"published release validation: {repo}",
        f"latest ping: {summary['db_stats'].get('latest_ping') or 'unknown'}",
        f"total rows: {summary['db_stats'].get('total_rows', 0)}",
        f"total distinct installs: {summary['db_stats'].get('total_distinct_installs', 0)}",
    ]

    unavailable_columns = summary.get("source_schema", {}).get("unavailable_columns", [])
    if unavailable_columns:
        lines.append(
            "source schema warning: unavailable fields defaulted to zero: "
            + ", ".join(unavailable_columns)
        )

    exclusions = summary.get("mock_fleet_exclusions")
    if exclusions:
        if exclusions.get("enabled"):
            lines.append(
                "mock fixture fleet excluded from window: "
                f"{exclusions['rows']} row(s) across {exclusions['installs']} install(s)"
            )
        else:
            lines.append("mock fixture fleet rows INCLUDED (--include-mock-fleet)")

    for label, _ in DEFAULT_LATEST_INSTALL_WINDOWS:
        window_summary = summary["latest_install_windows"][label]
        lines.extend(
            [
                "",
                f"Latest install state ({label}):",
                f"- active installs: {window_summary['active_installs']}",
                "- published versions:",
            ]
        )
        if window_summary["published_versions"]:
            lines.extend(f"  - {entry['version']}: {entry['installs']}" for entry in window_summary["published_versions"])
        else:
            lines.append("  - none")
        lines.append("- non-release or unpublished versions:")
        if window_summary["non_release_versions"]:
            lines.extend(
                f"  - {entry['version']}: {entry['installs']}" for entry in window_summary["non_release_versions"]
            )
        else:
            lines.append("  - none")
        lines.append("- platforms:")
        if window_summary["platforms"]:
            lines.extend(f"  - {entry['platform']}: {entry['installs']}" for entry in window_summary["platforms"])
        else:
            lines.append("  - none")
        lines.append("- aggregate adoption counts:")
        adoption_counts = [entry for entry in window_summary.get("adoption_counts", []) if entry["total"] > 0]
        if adoption_counts:
            lines.extend(f"  - {entry['label']}: {entry['total']}" for entry in adoption_counts)
        else:
            lines.append("  - none")
        lines.append("- feature-enabled installs:")
        feature_counts = [entry for entry in window_summary.get("feature_enabled_installs", []) if entry["installs"] > 0]
        if feature_counts:
            lines.extend(f"  - {entry['label']}: {entry['installs']}" for entry in feature_counts)
        else:
            lines.append("  - none")

    user_base = summary.get("user_base_signals_7d")
    if user_base:
        lines.extend(
            [
                "",
                "User-base lifecycle and outcomes (7d):",
                f"- active installs: {user_base['active_installs']}",
                "- payload schema coverage:",
            ]
        )
        lines.extend(
            f"  - {entry['version']}: {entry['installs']}"
            for entry in user_base.get("schema_versions", [])
        )
        lines.append("- lifecycle, audience, and update buckets:")
        for signal in user_base.get("category_signals", []):
            buckets = ", ".join(
                f"{entry['bucket']} {entry['installs']}"
                for entry in signal.get("buckets", [])
            )
            lines.append(f"  - {signal['label']}: {buckets or 'none'}")
        lines.append("- current/observed posture:")
        for signal in user_base.get("boolean_signals", []):
            lines.append(f"  - {signal['label']}: {signal['installs']} installs")
        lines.append("- aggregate outcomes:")
        for signal in user_base.get("count_signals", []):
            lines.append(
                f"  - {signal['label']}: {signal['total']} across {signal['installs']} installs"
            )
        lines.append(
            "- interpretation: known install age begins when schema v2 lifecycle tracking is first initialized; "
            "for upgraded installs it is only a lower bound, not original installation age"
        )
        lines.append(
            "- interpretation: deployment method is best-effort current-runtime evidence; "
            "upgraded installs often fall back to container_other or binary_other, so these buckets "
            "must not be read as precise original installation provenance"
        )
        lines.append(
            "- notification failure semantics: schema v2 counted unsuccessful retry attempts; "
            "schema v3+ counts only terminal failed or dead-letter outcomes"
        )

    pulse_loop = summary.get("pulse_intelligence_value_loop_7d")
    if pulse_loop:
        lines.extend(
            [
                "",
                "Pulse Intelligence value loop (7d):",
                f"- active installs: {pulse_loop['active_installs']}",
                f"- paid posture: paid {pulse_loop['paid_installs']}, free/community {pulse_loop['free_installs']}",
                "- adoption flags:",
            ]
        )
        bool_signals = [entry for entry in pulse_loop.get("boolean_signals", []) if entry["installs"] > 0]
        if bool_signals:
            lines.extend(
                f"  - {entry['label']}: "
                + format_paid_install_split(entry["installs"], entry["paid_installs"], entry["free_installs"])
                for entry in bool_signals
            )
        else:
            lines.append("  - none")

        lines.append("- activity counters:")
        count_signals = [entry for entry in pulse_loop.get("count_signals", []) if entry["total"] > 0]
        if count_signals:
            lines.extend(f"  - {entry['label']}: {format_paid_count_split(entry)}" for entry in count_signals)
        else:
            lines.append("  - none")
        accounting = pulse_loop.get("approved_action_outcome_accounting", {})
        lines.extend(
            [
                "- approved-action outcome accounting:",
                "  - "
                f"attempts {accounting.get('attempts', 0)}, "
                f"accounted {accounting.get('accounted', 0)}, "
                f"gap {accounting.get('gap', 0)}, "
                f"overflow {accounting.get('overflow', 0)}",
                "  - "
                f"pre-dispatch refusals {accounting.get('pre_dispatch_refusals', 0)}, "
                f"categorized {accounting.get('refusal_categories_accounted', 0)}, "
                f"gap {accounting.get('refusal_category_gap', 0)}, "
                f"overflow {accounting.get('refusal_category_overflow', 0)}",
                "  - interpretation: non-zero gaps are expected from pre-schema-v4 rows; "
                "schema v4 installs must reconcile exactly",
            ]
        )

    pulse_cohorts = summary.get("pulse_intelligence_outcome_cohorts")
    if pulse_cohorts:
        lines.extend(
            [
                "",
                "Pulse Intelligence activation and retention:",
                f"- source window: last {since_days} day(s)",
                f"- retention definition: latest ping within {pulse_cohorts.get('retention_window', '7d')}",
                "- cohorts:",
            ]
        )
        cohorts = [entry for entry in pulse_cohorts.get("cohorts", []) if entry["installs"] > 0]
        if cohorts:
            lines.extend(f"  - {format_pulse_intelligence_cohort(entry)}" for entry in cohorts)
        else:
            lines.append("  - none")

    pulse_funnel = summary.get("pulse_intelligence_operations_loop_funnel")
    if pulse_funnel:
        lines.extend(
            [
                "",
                "Pulse Intelligence operations loop funnel:",
                f"- source window: last {since_days} day(s)",
                f"- retention definition: latest ping within {pulse_funnel.get('retention_window', '7d')}",
                "- stages:",
            ]
        )
        stages = [entry for entry in pulse_funnel.get("stages", []) if entry["installs"] > 0]
        if stages:
            lines.extend(f"  - {format_pulse_intelligence_cohort(entry)}" for entry in stages)
        else:
            lines.append("  - none")

    service_health = summary.get("target_release_service_health_7d")
    if service_health:
        lines.extend(
            [
                "",
                f"Target release local service health ({service_health['version']}, {service_health['window']}):",
                f"- target installs: {service_health['target_installs']}",
                f"- direct observations: {service_health['observed_installs']}",
                f"- no schema-v13 observation: {service_health['unobserved_installs']}",
                f"- healthy: {service_health['healthy_installs']}",
                f"- unhealthy: {service_health['unhealthy_installs']}",
                "- latest direct result categories:",
            ]
        )
        categories = service_health.get("failure_categories", [])
        if categories:
            lines.extend(
                f"  - {entry['category']}: {entry['installs']} install(s)"
                for entry in categories
            )
        else:
            lines.append("  - none")
        lines.append(
            "- comparable version-change observations: "
            f"{service_health['comparable_version_change_installs']}"
        )
        transitions = service_health.get("transitions", [])
        if transitions:
            lines.extend(
                f"  - {entry['transition']}: {entry['installs']} install(s)"
                for entry in transitions
            )
        else:
            lines.append("  - none")
        lines.append(
            "- interpretation: direct local API/UI/asset observations only; the before/after "
            "cohort uses the immediately previous release observation and never rolls historical "
            "usage or update counters into the target release"
        )

    target_followup = summary.get("target_release_followup")
    if target_followup:
        lines.extend(
            [
                "",
                f"Target release follow-up ({target_followup['version']}):",
                "- source window: last "
                f"{target_followup.get('source_window_days', since_days)} day(s)",
                f"- installs seen on target: {target_followup['installs_seen']}",
                f"- target heartbeat rows: {target_followup['target_heartbeat_rows']}",
                f"- latest version still target: {target_followup['current_target_installs']}",
                "- attribution-ready installs with a later heartbeat on the same version: "
                f"{target_followup['same_version_followup_installs']}",
                "  - still running target: "
                f"{target_followup['current_target_followup_installs']}",
                "  - later departed target: "
                f"{target_followup['departed_followup_installs']}",
                "- installs without a later same-version heartbeat: "
                f"{target_followup['without_later_same_version_heartbeat']}",
                "- first target-version heartbeat in the source window is baseline only; its "
                "rolling counters are excluded from "
                "target-release activity",
                "- observed same-version counter increases on installs still running target:",
            ]
        )
        increased_signals = [
            signal
            for signal in target_followup.get("activity_signals", [])
            if signal["current_target_total_increase"] > 0
        ]
        if increased_signals:
            lines.extend(
                "  - "
                f"{signal['label']}: +{signal['current_target_total_increase']} across "
                f"{signal['current_target_increased_installs']} install(s)"
                for signal in increased_signals
            )
        else:
            lines.append("  - none")

        departed_increased_signals = [
            signal
            for signal in target_followup.get("activity_signals", [])
            if signal["departed_total_increase"] > 0
        ]
        lines.append("- same-version counter increases on installs that later departed target:")
        if departed_increased_signals:
            lines.extend(
                "  - "
                f"{signal['label']}: +{signal['departed_total_increase']} across "
                f"{signal['departed_increased_installs']} install(s)"
                for signal in departed_increased_signals
            )
        else:
            lines.append("  - none")

        decreased_signals = [
            signal
            for signal in target_followup.get("activity_signals", [])
            if signal["current_target_total_decrease"] > 0
        ]
        lines.append(
            "- rolling-counter decreases on installs still running target "
            "(reported separately, never netted against increases):"
        )
        if decreased_signals:
            lines.extend(
                "  - "
                f"{signal['label']}: -{signal['current_target_total_decrease']} across "
                f"{signal['current_target_decreased_installs']} install(s)"
                for signal in decreased_signals
            )
        else:
            lines.append("  - none")

        departed_decreased_signals = [
            signal
            for signal in target_followup.get("activity_signals", [])
            if signal["departed_total_decrease"] > 0
        ]
        lines.append("- rolling-counter decreases on installs that later departed target:")
        if departed_decreased_signals:
            lines.extend(
                "  - "
                f"{signal['label']}: -{signal['departed_total_decrease']} across "
                f"{signal['departed_decreased_installs']} install(s)"
                for signal in departed_decreased_signals
            )
        else:
            lines.append("  - none")

        for key, heading in (
            ("rollback_transitions", "rollback transitions"),
            ("forward_transitions", "forward transitions"),
            ("unclassified_transitions", "unclassified version departures"),
        ):
            transitions = target_followup.get(key, [])
            lines.append(f"- {heading}:")
            if transitions:
                lines.extend(
                    f"  - {entry['destination_version']}: {entry['installs']} install(s)"
                    for entry in transitions
                )
            else:
                lines.append("  - none")

    quality_comparison = summary.get("alert_quality_release_comparison")
    if quality_comparison:
        target = quality_comparison["target"]
        baseline = quality_comparison["baseline"]
        lines.extend([
            "",
            f"Alert-quality release comparison ({target['version']} vs {baseline['version']}):",
            f"- status: {quality_comparison['status']}",
            f"- target exposure: {target['installs']} install(s), {target['heartbeat_rows']} heartbeat(s), "
            f"{target['same_version_comparable_installs']} schema-capable same-version follow-up install(s)",
            f"- baseline exposure: {baseline['installs']} install(s), {baseline['heartbeat_rows']} heartbeat(s), "
            f"{baseline['same_version_comparable_installs']} schema-capable same-version follow-up install(s)",
            "- first heartbeat for each version and install is baseline-only",
            "- comparisons use only matched known-age and heartbeat-count exposure strata",
        ])
        if quality_comparison.get("reason"):
            lines.append(f"- reason: {quality_comparison['reason']}")
        strata = quality_comparison.get("matched_exposure_strata", [])
        lines.append("- matched exposure strata:")
        if strata:
            for stratum in strata:
                lines.append(
                    "  - age " + stratum["known_install_age_bucket"] +
                    ", heartbeats " + stratum["heartbeat_count_bucket"] +
                    f": target {stratum['target']['installs']} install(s), "
                    f"baseline {stratum['baseline']['installs']} install(s)"
                )
        else:
            lines.append("  - none")

    target_coverage = summary.get("target_release_coverage_7d")
    if target_coverage:
        lines.extend(
            [
                "",
                f"Target release latest-state signal coverage (7d, {target_coverage['version']}):",
                "- interpretation: latest rolling totals show signal availability, not activity "
                "caused by this release",
                f"- active installs: {target_coverage['active_installs']}",
                "- platforms:",
            ]
        )
        if target_coverage["platforms"]:
            lines.extend(f"  - {entry['platform']}: {entry['installs']}" for entry in target_coverage["platforms"])
        else:
            lines.append("  - none")

        for group, heading in (("core", "core signals with data"), ("deep", "deep signals with data")):
            signals = [
                signal
                for signal in target_coverage["signals"]
                if signal["group"] == group and signal["nonzero_installs"] > 0
            ]
            lines.append(f"- {heading}:")
            if signals:
                lines.extend(f"  - {format_target_signal(signal)}" for signal in signals)
            else:
                lines.append("  - none")

        missing_deep = [
            signal["label"]
            for signal in target_coverage["signals"]
            if signal["group"] == "deep" and signal["nonzero_installs"] == 0
        ]
        lines.append("- deep signals with no target-release data:")
        if missing_deep:
            lines.append("  - " + ", ".join(missing_deep))
        else:
            lines.append("  - none")

    lines.extend(["", "Deep telemetry signal sources (7d):"])
    deep_sources = summary.get("deep_signal_sources_7d", [])
    if deep_sources:
        for entry in deep_sources:
            versions = []
            for source in entry["versions"]:
                install_word = "install" if source["installs"] == 1 else "installs"
                source_text = f"{source['version']}: {source['installs']} {install_word}"
                if entry["type"] == "count":
                    source_text += f", total {source['total']}"
                versions.append(source_text)
            lines.append(f"- {entry['label']}: " + "; ".join(versions))
    else:
        lines.append("- none")
    return "\n".join(lines)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db-path", default=DEFAULT_DB_PATH, help="path to licenses.sqlite")
    parser.add_argument("--ssh-host", help="optional SSH host to query remotely, e.g. root@pulse-license")
    parser.add_argument("--since-days", type=int, default=7, help="history window to inspect")
    parser.add_argument(
        "--github-repo",
        default=DEFAULT_GITHUB_REPO,
        help="GitHub repo used to validate actually published release tags",
    )
    parser.add_argument(
        "--target-version",
        help=(
            "release version to highlight for latest-state coverage and same-version follow-up; defaults to "
            "the latest published stable release, falling back to the latest RC"
        ),
    )
    parser.add_argument(
        "--baseline-version",
        help=(
            "optional earlier release for schema-gated alert-quality comparison; "
            "cohorts are separated by known-age and heartbeat-count exposure"
        ),
    )
    parser.add_argument(
        "--include-mock-fleet",
        action="store_true",
        help=(
            "keep mock fixture fleet rows (120×N Kubernetes pods with 7×N VMware hosts) "
            "instead of excluding them from adoption reads"
        ),
    )
    parser.add_argument(
        "--format",
        choices=("text", "json"),
        default="text",
        help="output format",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    if args.since_days < 3:
        raise SystemExit("--since-days must be at least 3 so the 72h view is meaningful")

    published_releases = fetch_published_releases(args.github_repo)
    published_versions = {release["version"] for release in published_releases}
    target_version = args.target_version or latest_target_release_version(published_releases)
    source = (
        fetch_rows_remote(
            args.ssh_host,
            args.db_path,
            args.since_days,
            target_version=target_version,
            baseline_version=args.baseline_version,
        )
        if args.ssh_host
        else fetch_rows_local(args.db_path, args.since_days)
    )
    summary = summarize_rows(
        source["db_stats"],
        source["rows"],
        published_versions,
        target_version=target_version,
        baseline_version=args.baseline_version,
        include_mock_fleet=args.include_mock_fleet,
        source_window_days=args.since_days,
        pulse_intelligence_analysis_facts=source.get(
            "pulse_intelligence_analysis_facts"
        ),
        target_release_analysis_facts=source.get(
            "target_release_analysis_facts"
        ),
    )
    summary["source_schema"] = {
        "unavailable_columns": source.get("unavailable_columns", []),
    }

    if args.format == "json":
        print(json.dumps(summary, indent=2, sort_keys=True))
    else:
        print(format_text(summary, args.github_repo, args.since_days))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
