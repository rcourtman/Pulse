#!/usr/bin/env python3
"""Summarize Pulse anonymous telemetry for operator-facing adoption reads.

This script intentionally normalizes version strings before aggregation so
manual builds, dev builds, and accidental `v` prefixes do not pollute
published-release reporting.
"""

from __future__ import annotations

import argparse
from collections import Counter
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
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
    ("active_alerts", "Active alerts"),
)
FEATURE_BOOL_FIELDS = (
    ("ai_enabled", "AI enabled"),
    ("patrol_enabled", "Patrol enabled"),
    ("discovery_enabled", "Discovery enabled"),
    ("notifications_enabled", "Notifications enabled"),
    ("ai_actions_enabled", "AI actions enabled"),
    ("relay_enabled", "Relay enabled"),
    ("sso_enabled", "SSO enabled"),
    ("multi_tenant", "Multi-tenant"),
    ("paid_license", "Paid license"),
    ("has_api_tokens", "Has API tokens"),
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
    ("pulse_intelligence_external_agent_operations_loop_30d", "External-agent operations loop 30d"),
    (
        "pulse_intelligence_external_agent_approved_execution_loop_30d",
        "External-agent approved execution loop 30d",
    ),
    (
        "pulse_intelligence_external_agent_approved_action_success_loop_30d",
        "External-agent approved action success loop 30d",
    ),
    (
        "pulse_intelligence_external_agent_resolved_operations_loop_30d",
        "External-agent resolved operations loop 30d",
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
    ("pulse_intelligence_external_agent_enabled", "External agent enabled"),
    ("pulse_intelligence_external_agent_used_30d", "External agent used 30d"),
    ("pulse_intelligence_mcp_adapter_used_30d", "Pulse MCP adapter used 30d"),
)
PULSE_INTELLIGENCE_EXTERNAL_AGENT_CAPABILITY_COUNT_FIELDS = (
    (
        "pulse_intelligence_external_agent_context_requests_30d",
        "External agent context requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_event_stream_requests_30d",
        "External agent event-stream requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_provisioning_requests_30d",
        "External agent provisioning requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_operator_state_requests_30d",
        "External agent operator-state requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_finding_requests_30d",
        "External agent finding requests 30d",
    ),
    (
        "pulse_intelligence_external_agent_action_requests_30d",
        "External agent action requests 30d",
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
        "External-agent operations loop 30d",
        (
            "pulse_intelligence_external_agent_operations_loop_30d",
            "pulse_intelligence_mcp_adapter_operations_loop_30d",
        ),
        (),
    ),
    (
        "external_agent_approved_execution_loop_30d",
        "External-agent approved execution loop 30d",
        (
            "pulse_intelligence_external_agent_approved_execution_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_execution_loop_30d",
        ),
        (),
    ),
    (
        "external_agent_approved_action_success_loop_30d",
        "External-agent approved action success loop 30d",
        (
            "pulse_intelligence_external_agent_approved_action_success_loop_30d",
            "pulse_intelligence_mcp_adapter_approved_action_success_loop_30d",
        ),
        (),
    ),
    (
        "external_agent_resolved_operations_loop_30d",
        "External-agent resolved operations loop 30d",
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
        "External agent/MCP used 30d",
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
        "External-agent operations loop",
        ("external_agent_operations_loop",),
    ),
    (
        "external_agent_approved_execution_loop",
        "External-agent approved execution loop",
        ("external_agent_approved_execution_loop",),
    ),
    (
        "external_agent_approved_success_loop",
        "External-agent approved action success loop",
        ("external_agent_approved_success_loop",),
    ),
    (
        "external_agent_resolved_operations_loop",
        "External-agent resolved operations loop",
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
    ("patrol_enabled", "Patrol enabled", "bool"),
    ("discovery_enabled", "Discovery enabled", "bool"),
    ("notifications_enabled", "Notifications enabled", "bool"),
    ("ai_actions_enabled", "AI actions enabled", "bool"),
    *(
        (field, label, "bool")
        for field, label in PULSE_INTELLIGENCE_BOOL_FIELDS
    ),
    *(
        (field, label, "count")
        for field, label in PULSE_INTELLIGENCE_COUNT_FIELDS
    ),
)
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
        rows = [
            dict(row)
            for row in conn.execute(
                """
                SELECT *
                FROM telemetry_pings
                WHERE julianday(received_at) >= julianday('now') - ?
                ORDER BY received_at DESC
                """,
                (since_days,),
            ).fetchall()
        ]
        return {"db_stats": db_stats, "rows": rows}
    finally:
        conn.close()


def fetch_rows_remote(ssh_host: str, db_path: str, since_days: int) -> dict[str, Any]:
    remote_script = """
import json
import sqlite3
import sys

db_path = sys.argv[1]
since_days = int(sys.argv[2])
conn = sqlite3.connect(db_path)
conn.row_factory = sqlite3.Row
db_stats_sql = (
    "SELECT MAX(received_at) AS latest_ping, "
    "COUNT(*) AS total_rows, "
    "COUNT(DISTINCT install_id) AS total_distinct_installs "
    "FROM telemetry_pings"
)
rows_sql = (
    "SELECT * "
    "FROM telemetry_pings "
    "WHERE julianday(received_at) >= julianday('now') - ? "
    "ORDER BY received_at DESC"
)
try:
    db_stats = dict(conn.execute(db_stats_sql).fetchone())
    rows = [
        dict(row)
        for row in conn.execute(rows_sql, (since_days,)).fetchall()
    ]
    print(json.dumps({"db_stats": db_stats, "rows": rows}))
finally:
    conn.close()
"""
    result = subprocess.run(
        ["ssh", ssh_host, "python3", "-", db_path, str(since_days)],
        input=remote_script,
        text=True,
        capture_output=True,
        check=True,
    )
    return json.loads(result.stdout)


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

    return {
        "window": "7d",
        "active_installs": active_installs,
        "paid_installs": paid_installs,
        "free_installs": free_installs,
        "boolean_signals": list(bool_signals.values()),
        "count_signals": list(count_signals.values()),
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
        latest_received_at = parse_received_at(str(latest["received_at"]))
        if current_time - latest_received_at <= retention_window:
            retained_installs += 1
        if parse_optional_bool(latest.get("paid_license")) is True:
            paid_latest += 1
        else:
            free_latest += 1
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


def group_pulse_intelligence_rows_by_install(
    rows: Iterable[dict[str, Any]],
) -> dict[str, list[dict[str, Any]]]:
    rows_by_install: dict[str, list[dict[str, Any]]] = {}
    for row in rows:
        install_id = str(row.get("install_id") or "").strip()
        if install_id:
            rows_by_install.setdefault(install_id, []).append(row)
    return rows_by_install


def summarize_pulse_intelligence_outcome_cohorts(
    rows: Iterable[dict[str, Any]],
    latest_by_install: dict[str, dict[str, Any]],
    *,
    now: datetime | None = None,
    retention_window: timedelta = timedelta(days=7),
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    cohort_install_ids: dict[str, set[str]] = {
        key: set() for key, _, _, _ in PULSE_INTELLIGENCE_OUTCOME_COHORTS
    }
    rows_by_install = group_pulse_intelligence_rows_by_install(rows)

    for install_id, install_rows in rows_by_install.items():
        for row in install_rows:
            for key, _, bool_fields, count_fields in PULSE_INTELLIGENCE_OUTCOME_COHORTS:
                if pulse_intelligence_row_matches_cohort(row, bool_fields, count_fields):
                    cohort_install_ids[key].add(install_id)

    cohorts: list[dict[str, Any]] = []
    for key, label, bool_fields, count_fields in PULSE_INTELLIGENCE_OUTCOME_COHORTS:
        install_ids = cohort_install_ids[key]
        signal_outcomes_by_install = {
            install_id: pulse_intelligence_signal_observed_conversion(
                install_rows,
                bool_fields,
                count_fields,
            )
            for install_id, install_rows in rows_by_install.items()
        }
        cohorts.append(
            {
                "key": key,
                "label": label,
                **summarize_pulse_intelligence_install_outcomes(
                    install_ids,
                    latest_by_install,
                    rows_by_install,
                    current_time,
                    retention_window,
                    signal_outcomes_by_install,
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
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    rows_by_install = group_pulse_intelligence_rows_by_install(rows)
    signal_groups_by_install: dict[str, set[str]] = {
        install_id: set() for install_id in rows_by_install
    }

    for install_id, install_rows in rows_by_install.items():
        for row in install_rows:
            signal_groups_by_install[install_id].update(
                pulse_intelligence_row_signal_groups(row)
            )
        pulse_intelligence_derive_signal_groups(signal_groups_by_install[install_id])

    stages: list[dict[str, Any]] = []
    for key, label, required_groups in PULSE_INTELLIGENCE_OPERATIONS_FUNNEL_STAGES:
        install_ids = [
            install_id
            for install_id, groups in signal_groups_by_install.items()
            if all(group in groups for group in required_groups)
        ]
        signal_outcomes_by_install = {
            install_id: pulse_intelligence_stage_signal_observed_conversion(
                install_rows,
                required_groups,
            )
            for install_id, install_rows in rows_by_install.items()
        }
        stages.append(
            {
                "key": key,
                "label": label,
                "required_signal_groups": list(required_groups),
                **summarize_pulse_intelligence_install_outcomes(
                    install_ids,
                    latest_by_install,
                    rows_by_install,
                    current_time,
                    retention_window,
                    signal_outcomes_by_install,
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


def summarize_rows(
    db_stats: dict[str, Any],
    rows: Iterable[dict[str, Any]],
    published_versions: set[str],
    target_version: str | None = None,
    include_mock_fleet: bool = False,
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

    current_time = datetime.now(timezone.utc)
    latest_install_windows = summarize_latest_install_windows(
        latest_by_install,
        published_versions,
        now=current_time,
    )
    summary_72h = latest_install_windows["72h"]
    summary_7d = latest_install_windows["7d"]

    return {
        "db_stats": db_stats,
        "mock_fleet_exclusions": {
            "enabled": not include_mock_fleet,
            "rows": mock_fleet_rows,
            "installs": len(mock_fleet_installs),
        },
        "latest_install_windows": latest_install_windows,
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
        ),
        "pulse_intelligence_operations_loop_funnel": summarize_pulse_intelligence_operations_funnel(
            row_list,
            latest_by_install,
            now=current_time,
        ),
        "target_release_coverage_7d": summarize_target_version_coverage(
            latest_by_install,
            published_versions,
            target_version,
            now=current_time,
        )
        if target_version
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

    target_coverage = summary.get("target_release_coverage_7d")
    if target_coverage:
        lines.extend(
            [
                "",
                f"Target release signal coverage (7d, {target_coverage['version']}):",
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
        help="release version to highlight for per-signal coverage; defaults to the latest published RC",
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
    target_version = args.target_version or latest_rc_version(published_releases)
    source = (
        fetch_rows_remote(args.ssh_host, args.db_path, args.since_days)
        if args.ssh_host
        else fetch_rows_local(args.db_path, args.since_days)
    )
    summary = summarize_rows(
        source["db_stats"],
        source["rows"],
        published_versions,
        target_version=target_version,
        include_mock_fleet=args.include_mock_fleet,
    )

    if args.format == "json":
        print(json.dumps(summary, indent=2, sort_keys=True))
    else:
        print(format_text(summary, args.github_repo, args.since_days))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
