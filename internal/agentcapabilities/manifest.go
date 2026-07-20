package agentcapabilities

import (
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func agentObjectInputSchemaMap(required []string, properties map[string]any) map[string]any {
	return StrictObjectInputSchemaMap(required, properties)
}

func agentInputSchema(schema map[string]any) json.RawMessage {
	return RawInputSchema(schema)
}

func agentObjectInputSchema(required []string, properties map[string]any) json.RawMessage {
	return StrictObjectInputSchema(required, properties)
}

func agentObjectOutputSchema(required []string, properties map[string]any) json.RawMessage {
	return ObjectInputSchema(required, properties, true)
}

func fleetContextOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"resources", "generatedAt"}, map[string]any{
		"resources":   objectArrayOption("Per-resource triage summaries visible to the authenticated agent."),
		"generatedAt": dateTimeOption("Server timestamp for this fleet context snapshot."),
	})
}

// fleetContextInputSchema declares the optional additive filters for
// get_fleet_context. None are required; omitting all returns the full fleet
// (backward compatible). Forwarded as URL query parameters for the GET.
func fleetContextInputSchema() json.RawMessage {
	return agentObjectInputSchema(nil, map[string]any{
		"hasFindings": map[string]any{
			"type":        "boolean",
			"description": "When true, return only resources with at least one active finding. Omit to include healthy resources.",
		},
		"severity": map[string]any{
			"type":        "string",
			"enum":        []string{"critical", "warning", "info"},
			"description": "Return only resources with at least one finding at this severity. Composes with hasFindings and the dimension filters.",
		},
		"technology": map[string]any{
			"type":        "string",
			"description": "Return only resources whose technology matches exactly (case-insensitive), such as proxmox, docker, lxc, qemu.",
		},
		"resourceType": map[string]any{
			"type":        "string",
			"description": "Return only resources whose canonical type matches exactly (case-insensitive), such as vm, system-container, app-container, agent, storage.",
		},
	})
}

func operationsLoopStatusOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"nextAction", "progressLabel", "steps", "patrolEvidenceCount", "patrolIssueEvidenceCount", "activeFindingCount", "pendingApprovalCount", "governedActionCount", "approvedDecisionCount", "rejectedDecisionCount", "verifiedOutcomeCount", "operationsLoopStarterCount", "assistantOperationsLoopStarterCount", "patrolOperationsLoopStarterCount", "patrolControlOperationsLoopStarterCount", "patrolControlCompletedOperationsLoopCount", "patrolControlResolvedOperationsLoopCount", "patrolControlValueState", "patrolAutonomyOperationsLoopStarterCount", "patrolAutonomyCompletedOperationsLoopCount", "patrolAutonomyResolvedOperationsLoopCount", "patrolAutonomyValueState", "proActivationOperationsLoopStarterCount", "proActivationCompletedOperationsLoopCount", "proActivationResolvedOperationsLoopCount", "proActivationValueProofState", "mcpOperationsLoopStarterCount", "externalAgentReady", "windowStart", "generatedAt"}, map[string]any{
		"nextAction":                                 stringEnumOption([]string{"run_patrol", "review_findings", "open_assistant", "review_approvals", "open_mcp", "complete"}, "Next recommended Patrol work action an agent or UI should take."),
		"progressLabel":                              stringOption("Operator-readable status summary for the current Patrol work stage."),
		"steps":                                      objectArrayOption("Four-stage Patrol, Assistant, governance, and verification status rollup. Step counts are content-free stage evidence: Assistant counts contextual collaboration, governance counts pending approvals until an approved/rejected decision exists, and verification counts verified outcomes or terminal rejected decisions. External-agent readiness is exposed separately through externalAgentReady."),
		"patrolEvidenceCount":                        integerOption("Count-only evidence that Pulse Intelligence has observed Patrol work activity in the current window."),
		"patrolIssueEvidenceCount":                   integerOption("Count-only evidence that Patrol has an actionable issue, approval, governed decision, or verified outcome."),
		"activeFindingCount":                         integerOption("Total active Patrol findings across the visible fleet."),
		"pendingApprovalCount":                       integerOption("Total pending governed-action approvals across the visible fleet."),
		"governedActionCount":                        integerOption("Count of recent decision-backed governed actions without action ids or command content."),
		"approvedDecisionCount":                      integerOption("Count of recent approved governed-action decisions without action ids or command content."),
		"rejectedDecisionCount":                      integerOption("Count of recent rejected governed-action decisions without action ids or command content."),
		"verifiedOutcomeCount":                       integerOption("Count of recent approved governed actions with a verified post-action outcome."),
		"operationsLoopStarterCount":                 integerOption("Count of content-free Patrol work prompt starter events in the evidence window."),
		"assistantOperationsLoopStarterCount":        integerOption("Count of Patrol work prompt starters rendered by Pulse Assistant in the evidence window."),
		"patrolOperationsLoopStarterCount":           integerOption("Count of Patrol work prompt starters launched from Pulse Patrol in the evidence window."),
		"patrolControlOperationsLoopStarterCount":    integerOption("Count of Patrol work prompt starters launched from the first-party Patrol control journey in the evidence window."),
		"patrolControlCompletedOperationsLoopCount":  integerOption("Count-only evidence that the Patrol control journey reached a terminal Patrol work result in the evidence window: Patrol issue evidence, contextual Assistant or external-agent collaboration, and either a rejected governed decision or an approved governed decision with a verified outcome."),
		"patrolControlResolvedOperationsLoopCount":   integerOption("Count-only evidence that the Patrol control journey reached a verified Patrol work result in the evidence window: Patrol issue evidence, contextual Assistant or external-agent collaboration, approved governed decision, and verified outcome."),
		"patrolControlValueState":                    stringEnumOption([]string{"not_started", "in_progress", "governed_decision_recorded", "verified_needs_mcp", "verified"}, "Content-safe Patrol control value state. Only verified means Patrol completed a verified work outcome; governed_decision_recorded means a safe terminal decision exists without verified outcome evidence."),
		"patrolAutonomyOperationsLoopStarterCount":   integerOption("Compatibility alias for patrolControlOperationsLoopStarterCount."),
		"patrolAutonomyCompletedOperationsLoopCount": integerOption("Compatibility alias for patrolControlCompletedOperationsLoopCount."),
		"patrolAutonomyResolvedOperationsLoopCount":  integerOption("Compatibility alias for patrolControlResolvedOperationsLoopCount."),
		"patrolAutonomyValueState":                   stringEnumOption([]string{"not_started", "in_progress", "governed_decision_recorded", "verified_needs_mcp", "verified"}, "Compatibility alias for patrolControlValueState."),
		"proActivationOperationsLoopStarterCount":    integerOption("Compatibility field retained for older external-agent clients; primary clients should use patrolControlOperationsLoopStarterCount."),
		"proActivationCompletedOperationsLoopCount":  integerOption("Compatibility alias for patrolControlCompletedOperationsLoopCount."),
		"proActivationResolvedOperationsLoopCount":   integerOption("Compatibility alias for patrolControlResolvedOperationsLoopCount."),
		"proActivationValueProofState":               stringEnumOption([]string{"not_started", "in_progress", "governed_decision_recorded", "verified_needs_mcp", "verified"}, "Compatibility alias for patrolControlValueState."),
		"mcpOperationsLoopStarterCount":              integerOption("Count of Patrol work prompt starters rendered by the Pulse MCP surface in the evidence window."),
		"externalAgentReady":                         booleanOption("Whether the Pulse MCP Patrol work contract is available and one non-expired API token covers every scope required by the published Pulse MCP Patrol work capability set."),
		"windowStart":                                dateTimeOption("Start of the action-audit evidence window used for governed/verified counts."),
		"generatedAt":                                dateTimeOption("Server timestamp for this Patrol work status snapshot."),
	})
}

func resourceContextOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"canonicalId", "resourceType", "resourceName", "activeFindings", "pendingApprovals", "recentActions", "contextSections", "generatedAt"}, map[string]any{
		"canonicalId":      stringOption("Canonical Pulse resource id."),
		"resourceType":     stringOption("Canonical resource type."),
		"resourceName":     stringOption("Display name for the resource."),
		"technology":       stringOption("Platform or technology family when known."),
		"operatorState":    openObjectOption("Operator-set resource state when present."),
		"activeFindings":   objectArrayOption("Active Patrol finding summaries for this resource."),
		"pendingApprovals": objectArrayOption("Pending action approvals scoped to this resource."),
		"recentActions":    objectArrayOption("Recent action attempts and outcomes scoped to this resource."),
		"contextSections":  objectArrayOption("Additive typed context sections with provenance and redaction metadata."),
		"generatedAt":      dateTimeOption("Server timestamp for this resource context snapshot."),
	})
}

func resourceCapabilitiesOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"resourceId", "capabilities", "generatedAt"}, map[string]any{
		"resourceId":   stringOption("Canonical Pulse resource id the capabilities belong to."),
		"capabilities": objectArrayOption("Advertised governed capabilities with full parameter schemas; empty array when the resource advertises none."),
		"generatedAt":  dateTimeOption("Server timestamp for this capability snapshot."),
	})
}

func operatorStateOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"canonicalId", "intentionallyOffline", "neverAutoRemediate", "setAt"}, map[string]any{
		"canonicalId":             stringOption("Canonical Pulse resource id."),
		"intentionallyOffline":    booleanOption("Whether this resource is expected to be offline."),
		"neverAutoRemediate":      booleanOption("Whether automated remediation must be refused for this resource."),
		"maintenanceStartAt":      dateTimeOption("Maintenance window start time when set."),
		"maintenanceEndAt":        dateTimeOption("Maintenance window end time when set."),
		"maintenanceReason":       stringOption("Operator note attached to the maintenance window."),
		"criticality":             stringOption("Operator-set finding sort hint: high, medium, low, or empty."),
		"note":                    stringOption("Operator note for this resource."),
		"setAt":                   dateTimeOption("Server timestamp for the last state mutation."),
		"setBy":                   stringOption("Resolved actor id that last changed the state."),
		"maintenanceWindowActive": booleanOption("Whether the maintenance window is active at the server evaluation time when included in agent context."),
	})
}

func toolArrayOutputSchema(itemDescription string) json.RawMessage {
	return agentObjectOutputSchema([]string{"items", "count"}, map[string]any{
		"items": objectArrayOption(itemDescription),
		"count": integerOption("Number of items returned in the structured content wrapper."),
	})
}

func nodeMutationOutputSchema(required []string) json.RawMessage {
	return agentObjectOutputSchema(required, map[string]any{
		"status":  stringEnumOption([]string{"success"}, "Status returned by node mutation routes when the normal save path is used."),
		"success": booleanOption("Whether the node mutation succeeded when the route returns the cluster-merge envelope."),
		"message": stringOption("Operator-readable mutation result when included."),
	})
}

func nodeConnectionTestOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"status", "message"}, map[string]any{
		"status":           stringEnumOption([]string{"success", "error"}, "Connection test result."),
		"message":          stringOption("Operator-readable connection test result."),
		"latency":          integerOption("Connection probe latency in milliseconds when measured."),
		"isCluster":        booleanOption("Whether a proposed PVE source appears to be part of a cluster."),
		"nodeCount":        integerOption("Number of PVE nodes observed during the probe."),
		"clusterNodeCount": integerOption("Number of PVE cluster endpoints observed during the probe."),
		"datastoreCount":   integerOption("Number of PBS datastores observed during the probe."),
		"version":          stringOption("Platform version returned by PMG when available."),
		"release":          stringOption("Platform release returned by PMG when available."),
		"warnings":         stringArrayOption("Non-fatal probe warnings for platform-specific monitoring endpoints."),
	})
}

func clusterRefreshOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"status", "clusterName", "oldNodeCount", "newNodeCount", "nodesAdded", "clusterNodes"}, map[string]any{
		"status":       stringEnumOption([]string{"success"}, "Cluster refresh result."),
		"clusterName":  stringOption("Resolved PVE cluster name."),
		"oldNodeCount": integerOption("Endpoint count before refresh."),
		"newNodeCount": integerOption("Endpoint count after refresh."),
		"nodesAdded":   integerOption("Number of newly detected endpoints."),
		"clusterNodes": objectArrayOption("Cluster endpoint records returned after refresh."),
	})
}

func discoveryOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"servers", "errors", "structured_errors", "environment", "cached"}, map[string]any{
		"servers":           nullableObjectArrayOption("Discovered infrastructure servers."),
		"errors":            nullableStringArrayOption("Legacy human-readable discovery errors."),
		"structured_errors": nullableObjectArrayOption("Structured discovery errors with phase, type, message, and timestamp."),
		"environment":       nullableOpenObjectOption("Detected environment metadata for the scan when available."),
		"cached":            booleanOption("Whether this response came from the cached discovery result."),
		"scanning":          booleanOption("Whether a manual scan is still running."),
		"updated":           integerOption("Unix timestamp for the cached result."),
		"age":               map[string]any{"type": "number", "description": "Age of the cached result in seconds."},
	})
}

func findingActionOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{"success", "message"}, map[string]any{
		"success": booleanOption("Whether the finding lifecycle action succeeded."),
		"message": stringOption("Operator-readable finding lifecycle result."),
	})
}

func actionPlanOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{ActionIDArgumentName, RequestIDArgumentName, "allowed", "requiresApproval", "approvalPolicy", "rollbackAvailable", "plannedAt", "expiresAt", "resourceVersion", "policyVersion", "planHash"}, map[string]any{
		ActionIDArgumentName:   stringOption("Durable action id used by " + DecideActionCapabilityName + " and " + ExecuteActionCapabilityName + "."),
		RequestIDArgumentName:  stringOption("Caller-provided idempotency key or external correlation id."),
		"allowed":              booleanOption("Whether Pulse allows this action to be executed."),
		"requiresApproval":     booleanOption("Whether the action must be approved before execution."),
		"approvalPolicy":       stringOption("Approval policy required by the target capability."),
		"predictedBlastRadius": stringArrayOption("Related resources that could be affected by the action."),
		"rollbackAvailable":    booleanOption("Whether Pulse has a rollback path for the planned action."),
		"message":              stringOption("Operator-readable planning result."),
		"plannedAt":            dateTimeOption("Server timestamp when the plan was created."),
		"expiresAt":            dateTimeOption("Server timestamp when the plan expires."),
		"resourceVersion":      stringOption("Resource version hash captured at plan time."),
		"policyVersion":        stringOption("Capability policy version captured at plan time."),
		"planHash":             stringOption("Hash binding request parameters, resource state, and policy state."),
		"preflight":            openObjectOption("Deterministic pre-execution readout when available."),
	})
}

func actionDecisionOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{ActionIDArgumentName, "state", "approval", "audit"}, map[string]any{
		ActionIDArgumentName: stringOption("Durable action id that received the decision."),
		"state":              stringOption("Action lifecycle state after the decision."),
		"approval":           openObjectOption("Approval or rejection record persisted for this decision."),
		"audit":              openObjectOption("Current action audit record after applying the decision."),
	})
}

func actionExecutionOutputSchema() json.RawMessage {
	return agentObjectOutputSchema([]string{ActionIDArgumentName, "state", "audit"}, map[string]any{
		ActionIDArgumentName: stringOption("Durable action id that was executed."),
		"state":              stringOption("Action lifecycle state after execution."),
		"result":             openObjectOption("Execution result when the action reached the executor."),
		"audit":              openObjectOption("Current action audit record after execution."),
	})
}

func nodeIDInputSchema() json.RawMessage {
	return agentObjectInputSchema([]string{"nodeId"}, map[string]any{
		"nodeId": map[string]any{
			"type":        "string",
			"description": "Configured node id from list_nodes, such as pve:lab or pve-0.",
		},
	})
}

func discoverLANInputSchema() json.RawMessage {
	return agentObjectInputSchema(nil, map[string]any{
		"subnet": map[string]any{
			"type":        "string",
			"description": "CIDR to scan, such as 192.168.1.0/24, or auto to let Pulse choose the local subnet.",
			"default":     "auto",
		},
		"use_cache": map[string]any{
			"type":        "boolean",
			"description": "Return cached discovery results when available instead of starting a new scan.",
			"default":     false,
		},
	})
}

func addNodeInputSchema() json.RawMessage {
	schema := agentObjectInputSchemaMap([]string{"type", "name", "host"}, nodeConfigInputProperties(false))
	schema["oneOf"] = []map[string]any{
		{"required": []string{"user", "password"}},
		{"required": []string{"tokenName", "tokenValue"}},
	}
	return agentInputSchema(schema)
}

func testNodeCredentialsInputSchema() json.RawMessage {
	schema := agentObjectInputSchemaMap([]string{"type", "host"}, nodeConfigInputProperties(false))
	schema["oneOf"] = []map[string]any{
		{"required": []string{"user", "password"}},
		{"required": []string{"tokenName", "tokenValue"}},
	}
	return agentInputSchema(schema)
}

func updateNodeInputSchema() json.RawMessage {
	properties := nodeConfigInputProperties(true)
	return agentObjectInputSchema([]string{"nodeId"}, properties)
}

func operatorStateInputSchema() json.RawMessage {
	schema := agentObjectInputSchemaMap([]string{ResourceIDArgumentName, "intentionallyOffline", "neverAutoRemediate"}, map[string]any{
		ResourceIDArgumentName: stringOption("Canonical resource id from get_fleet_context or get_resource_context. This path value wins over any body id."),
		"intentionallyOffline": map[string]any{
			"type":        "boolean",
			"description": "Whether the resource is expected to be offline. This suppresses offline findings for this resource.",
		},
		"neverAutoRemediate": map[string]any{
			"type":        "boolean",
			"description": "Whether governed automated remediation must be refused for this resource.",
		},
		"maintenanceStartAt": map[string]any{
			"type":        "string",
			"format":      "date-time",
			"description": "Maintenance window start time. Set together with maintenanceEndAt, or omit both.",
		},
		"maintenanceEndAt": map[string]any{
			"type":        "string",
			"format":      "date-time",
			"description": "Maintenance window end time. Must be after maintenanceStartAt when both are set.",
		},
		"maintenanceReason": stringOption("Optional reason shown when findings are quieted by the maintenance window."),
		"criticality": map[string]any{
			"type":        "string",
			"enum":        []string{"high", "medium", "low", ""},
			"description": "Optional finding-sort hint. Empty string clears the operator-set criticality.",
		},
		"note": stringOption("Optional operator note surfaced with the resource state."),
	})
	schema["dependencies"] = map[string][]string{
		"maintenanceStartAt": {"maintenanceEndAt"},
		"maintenanceEndAt":   {"maintenanceStartAt"},
	}
	return agentInputSchema(schema)
}

func findingIDInputSchema(description string) json.RawMessage {
	return agentObjectInputSchema([]string{FindingIDArgumentName}, map[string]any{
		FindingIDArgumentName: stringOption(description),
	})
}

func resolveFindingInputSchema() json.RawMessage {
	return agentObjectInputSchema([]string{FindingIDArgumentName}, map[string]any{
		FindingIDArgumentName:      stringOption("Patrol finding id to resolve, returned by " + ListFindingsCapabilityName + "."),
		ResolutionNoteArgumentName: stringOption("Optional operator-readable note describing why the finding is resolved."),
	})
}

func snoozeFindingInputSchema() json.RawMessage {
	return agentObjectInputSchema([]string{FindingIDArgumentName, "duration_hours"}, map[string]any{
		FindingIDArgumentName: stringOption("Patrol finding id returned by " + ListFindingsCapabilityName + "."),
		"duration_hours": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     168,
			"description": "Number of hours to hide the finding. Pulse caps the value at 168 hours.",
		},
	})
}

func dismissFindingInputSchema() json.RawMessage {
	return agentObjectInputSchema([]string{FindingIDArgumentName, ReasonArgumentName}, map[string]any{
		FindingIDArgumentName: stringOption("Patrol finding id returned by " + ListFindingsCapabilityName + "."),
		ReasonArgumentName: map[string]any{
			"type":        "string",
			"enum":        []string{"not_an_issue", "expected_behavior", "will_fix_later"},
			"description": "Dismissal reason: not_an_issue suppresses a false positive, expected_behavior records known/accepted behavior, will_fix_later creates a reminder commitment.",
		},
		NoteArgumentName: stringOption("Optional operator note explaining the dismissal."),
	})
}

func actionPlanInputSchema() json.RawMessage {
	return agentObjectInputSchema([]string{RequestIDArgumentName, ResourceIDArgumentName, CapabilityNameArgumentName, ReasonArgumentName, RequestedByArgumentName}, map[string]any{
		RequestIDArgumentName:      stringOption("Caller idempotency key or external correlation id for this plan request."),
		ResourceIDArgumentName:     stringOption("Canonical resource id from get_fleet_context or get_resource_context."),
		CapabilityNameArgumentName: stringOption("Resource capability name to plan, as advertised on the target resource."),
		ReasonArgumentName:         stringOption("Operator-readable reason for planning the action."),
		RequestedByArgumentName:    stringOption("Stable requester identity, such as agent:oncall-helper."),
		"params": map[string]any{
			"type":                 "object",
			"description":          "Capability-specific parameters. Use the target resource capability's parameter contract.",
			"additionalProperties": true,
		},
	})
}

func actionDecisionInputSchema() json.RawMessage {
	schema := agentObjectInputSchemaMap([]string{ActionIDArgumentName, OutcomeArgumentName}, map[string]any{
		ActionIDArgumentName: actionIDProperty(),
		OutcomeArgumentName: map[string]any{
			"type":        "string",
			"enum":        []string{"approved", "rejected"},
			"description": "Approval decision to record for the pending action.",
		},
		ReasonArgumentName: stringOption("Optional operator-readable decision reason."),
	})
	return agentInputSchema(schema)
}

func actionExecutionInputSchema() json.RawMessage {
	return agentObjectInputSchema([]string{ActionIDArgumentName}, map[string]any{
		ActionIDArgumentName: actionIDProperty(),
		ReasonArgumentName:   stringOption("Optional operator-readable execution reason."),
	})
}

func actionIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^[a-zA-Z0-9_-]+$",
		"maxLength":   128,
		"description": "Action id returned by plan_action.",
	}
}

func nodeConfigInputProperties(includeNodeID bool) map[string]any {
	properties := map[string]any{
		"type": map[string]any{
			"type":        "string",
			"enum":        []string{"pve", "pbs", "pmg"},
			"description": "Infrastructure source type: pve, pbs, or pmg.",
		},
		"name": map[string]any{
			"type":        "string",
			"description": "Human-readable source name to show in Pulse.",
		},
		"host": map[string]any{
			"type":        "string",
			"description": "Source endpoint URL or host, including scheme and port when needed.",
		},
		"guestURL": map[string]any{
			"type":        "string",
			"description": "Optional guest-accessible URL for navigation.",
		},
		"user": map[string]any{
			"type":        "string",
			"description": "Username for password authentication, or the token owner when needed by the platform.",
		},
		"password": map[string]any{
			"type":        "string",
			"description": "Password used only for setup or password-backed monitoring.",
		},
		"tokenName": map[string]any{
			"type":        "string",
			"description": "API token id or name, such as root@pam!pulse-monitor.",
		},
		"tokenValue": map[string]any{
			"type":        "string",
			"description": "API token secret value. Pulse stores this as a credential and never returns it from list_nodes.",
		},
		"fingerprint": map[string]any{
			"type":        "string",
			"description": "Optional TLS certificate fingerprint for pinned self-signed endpoints.",
		},
		"verifySSL": map[string]any{
			"type":        "boolean",
			"description": "Whether Pulse should require normal TLS certificate validation.",
		},
		"monitorVMs":                   platformBool("PVE", "virtual machines"),
		"monitorContainers":            platformBool("PVE", "containers"),
		"monitorStorage":               platformBool("PVE", "storage"),
		"monitorBackups":               platformBool("PVE or PBS", "backups"),
		"monitorPhysicalDisks":         platformBool("PVE", "physical disks"),
		"physicalDiskPollingMinutes":   integerOption("PVE physical disk polling interval in minutes. Use 0 or omit for the default."),
		"temperatureMonitoringEnabled": platformBool("All source types", "temperature monitoring"),
		"monitorDatastores":            platformBool("PBS", "datastores"),
		"monitorSyncJobs":              platformBool("PBS", "sync jobs"),
		"monitorVerifyJobs":            platformBool("PBS", "verify jobs"),
		"monitorPruneJobs":             platformBool("PBS", "prune jobs"),
		"monitorGarbageJobs":           platformBool("PBS", "garbage collection jobs"),
		"monitorMailStats":             platformBool("PMG", "mail statistics"),
		"monitorQueues":                platformBool("PMG", "mail queues"),
		"monitorQuarantine":            platformBool("PMG", "quarantine"),
		"monitorDomainStats":           platformBool("PMG", "domain statistics"),
		"enabled":                      platformBool("All source types", "collection from this source"),
		"excludeDatastores":            stringArrayOption("PBS datastore names to exclude from monitoring."),
	}
	if includeNodeID {
		properties["nodeId"] = map[string]any{
			"type":        "string",
			"description": "Configured node id from list_nodes, such as pve:lab or pve-0.",
		}
	}
	return properties
}

func platformBool(platform, subject string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": platform + " option for " + subject + ".",
	}
}

func integerOption(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"minimum":     0,
		"description": description,
	}
}

func booleanOption(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
	}
}

func stringOption(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func stringEnumOption(values []string, description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        append([]string(nil), values...),
		"description": description,
	}
}

func dateTimeOption(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"format":      "date-time",
		"description": description,
	}
}

func stringArrayOption(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items": map[string]any{
			"type": "string",
		},
	}
}

func openObjectOption(description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          description,
		"additionalProperties": true,
	}
}

func nullableOpenObjectOption(description string) map[string]any {
	return map[string]any{
		"description": description,
		"oneOf": []map[string]any{
			{
				"type":                 "object",
				"additionalProperties": true,
			},
			{"type": "null"},
		},
	}
}

func objectArrayOption(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
	}
}

func nullableObjectArrayOption(description string) map[string]any {
	return map[string]any{
		"description": description,
		"oneOf": []map[string]any{
			{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
			{"type": "null"},
		},
	}
}

func nullableStringArrayOption(description string) map[string]any {
	return map[string]any{
		"description": description,
		"oneOf": []map[string]any{
			{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
			{"type": "null"},
		},
	}
}

const (
	agentCapabilityActionModeRead  = ActionModeRead
	agentCapabilityActionModeMixed = ActionModeMixed
	agentCapabilityActionModeWrite = ActionModeWrite

	agentCapabilityApprovalPolicyScopeOnly  = ApprovalPolicyScopeOnly
	agentCapabilityApprovalPolicyActionPlan = ApprovalPolicyActionPlan

	agentCapabilityScopeMonitoringRead  = auth.ScopeMonitoringRead
	agentCapabilityScopeMonitoringWrite = auth.ScopeMonitoringWrite
	agentCapabilityScopeSettingsRead    = auth.ScopeSettingsRead
	agentCapabilityScopeSettingsWrite   = auth.ScopeSettingsWrite
	agentCapabilityScopeAIExecute       = auth.ScopeAIExecute
	agentCapabilityScopeActionsPlan     = auth.ScopeActionsPlan
	agentCapabilityScopeActionsApprove  = auth.ScopeActionsApprove
	agentCapabilityScopeActionsExecute  = auth.ScopeActionsExecute
)

var (
	agentCapabilityFindingErrorCodes = []string{
		AgentErrCodeInvalidFindingRequest,
		AgentErrCodeFindingNotFound,
		AgentErrCodeFindingActionNotAllowed,
		AgentErrCodePatrolUnavailable,
	}
	agentCapabilityPlanActionErrorCodes = []string{
		AgentErrCodeInvalidActionRequest,
		AgentErrCodeMockModeEnabled,
		AgentErrCodeActionActorUnavailable,
		AgentErrCodeResourceNotFound,
		AgentErrCodeCapabilityNotFound,
		AgentErrCodeActionExecutionUnavailable,
	}
	agentCapabilityDecisionActionErrorCodes = []string{
		AgentErrCodeMockModeEnabled,
		AgentErrCodeMissingID,
		AgentErrCodeInvalidID,
		AgentErrCodeInvalidActionDecision,
		AgentErrCodeActionNotFound,
		AgentErrCodeActionNotPending,
		AgentErrCodeActionPlanExpired,
		AgentErrCodeActionPlanIdentityMismatch,
		AgentErrCodeActionActorUnavailable,
		AgentErrCodeActionApprovalForbidden,
		AgentErrCodeActionStepUpUnavailable,
		AgentErrCodeActionDecisionConflict,
		AgentErrCodeActionSeparationRequired,
		AgentErrCodeActionReplanRequired,
	}
	agentCapabilityExecuteActionErrorCodes = []string{
		AgentErrCodeMockModeEnabled,
		AgentErrCodeMissingID,
		AgentErrCodeInvalidID,
		AgentErrCodeInvalidActionExecution,
		AgentErrCodeActionNotFound,
		AgentErrCodeActionNotApproved,
		AgentErrCodeActionAlreadyExecuting,
		AgentErrCodeActionExecutionFinal,
		AgentErrCodeActionDryRunOnly,
		AgentErrCodeActionPlanExpired,
		AgentErrCodeActionExecutionUnavailable,
		AgentErrCodeActionPlanDrift,
		AgentErrCodeActionPlanIdentityMismatch,
		AgentErrCodeResourceRemediationLocked,
		AgentErrCodeActionExecutorUnavailable,
		AgentErrCodeActionActorUnavailable,
		AgentErrCodeActionExecutionForbidden,
		AgentErrCodeActionNotExecuting,
		AgentErrCodeActionReplanRequired,
	}
)

// canonicalManifest is the v1 declaration of Pulse's
// agent-consumable surface. Hand-authored rather than auto-generated
// because the contract decisions (which capabilities are
// agent-stable, what the stable error codes are, what category each
// belongs to) are product-shaping and must not drift behind code
// changes. Adding a capability here is a deliberate "this is part of
// the agent surface" commitment.
var canonicalManifest = Manifest{
	Version: "v1",
	SurfaceContract: SurfaceContract{
		Core: SurfaceContractComponent{
			ID:          "pulse_intelligence_core",
			Label:       "Pulse Intelligence Core",
			Description: "Canonical context, governed actions, safety gates, approval state, action audit, and verification shared by Pulse Assistant, Pulse MCP, and Pulse Patrol.",
		},
		ProactiveEngine: SurfaceContractComponent{
			ID:          "pulse_patrol",
			Label:       "Pulse Patrol",
			Description: "Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened.",
		},
		OperatorSurfaces: []OperatorSurfaceContract{
			{
				ID:              SurfaceIDPulseAssistant,
				Label:           "Pulse Assistant",
				Description:     "The contextual explanation, approval, and handoff surface for Patrol findings, governed actions, verification, and operator questions.",
				Native:          true,
				ExternalAdapter: false,
				Affordances: SurfaceAffordanceContract{
					Tools:                true,
					InteractiveQuestions: true,
				},
			},
			{
				ID:              SurfaceIDPulseMCP,
				Label:           "Pulse MCP",
				Description:     "The external-agent adapter that projects canonical Pulse Intelligence capabilities as MCP tools.",
				Native:          false,
				ExternalAdapter: true,
				Affordances: SurfaceAffordanceContract{
					Tools:              true,
					Resources:          true,
					Prompts:            true,
					CapabilityMetadata: true,
				},
			},
		},
	},
	SurfaceToolContracts: []SurfaceToolContract{
		{
			SurfaceID:  SurfaceIDPulseMCP,
			ToolSource: SurfaceToolSourceCapabilityManifest,
			ToolNames:  canonicalPulseMCPSurfaceToolNames(),
		},
	},
	MCPAdapter: DefaultMCPAdapterContract(),
	Categories: []CapabilityCategory{
		{
			ID:          "context",
			Label:       "Context (read-only)",
			Description: "Discovery and read-only situated reads. Agents start here.",
		},
		{
			ID:          "operator-state",
			Label:       "Operator state (per-resource intent)",
			Description: "Per-resource intent: intentionally-offline, never-auto-remediate, maintenance windows.",
		},
		{
			ID:          "finding",
			Label:       "Findings (Patrol lifecycle)",
			Description: "Acknowledge, snooze, dismiss, or resolve findings the Patrol runtime raised.",
		},
		{
			ID:          "action",
			Label:       "Actions (governed plan/approval/execute)",
			Description: "Plan, approve, and execute capability invocations against a resource.",
		},
		{
			ID:          "provisioning",
			Label:       "Provisioning (infrastructure onboarding)",
			Description: "List, test, add, update, or remove infrastructure sources through governed setup routes.",
		},
	},
	Capabilities: []Capability{
		{
			Name:           "get_resource_context",
			Title:          "Get resource context",
			Description:    "Return the situated picture of a resource — identity, operator-set state with maintenance-window-active flag, active findings, pending approvals scoped to this resource, recent actions including refused dispatches. Command fields are redacted for monitoring-read API tokens unless the token also has ai:execute.",
			Category:       "context",
			Method:         http.MethodGet,
			Path:           ResourceContextCapabilityPath,
			Scope:          agentCapabilityScopeMonitoringRead,
			ActionMode:     agentCapabilityActionModeRead,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "AgentResourceContext",
			OutputSchema:   resourceContextOutputSchema(),
			ErrorCodes:     []string{AgentErrCodeResourceNotFound},
		},
		{
			Name:           ListResourceCapabilitiesCapabilityName,
			Title:          "List resource capabilities",
			Description:    "Return the structured governed capabilities a resource advertises (name, type, approval level, platform, and full parameter schemas). Companion to get_resource_context, which renders capabilities as count-limited prose; this is the structured surface an agent reads before calling plan_action so it can populate capabilityName and params without guessing. A resource with no advertised capabilities returns an empty array.",
			Category:       "context",
			Method:         http.MethodGet,
			Path:           ListResourceCapabilitiesCapabilityPath,
			Scope:          agentCapabilityScopeMonitoringRead,
			ActionMode:     agentCapabilityActionModeRead,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "AgentResourceCapabilities",
			OutputSchema:   resourceCapabilitiesOutputSchema(),
			ErrorCodes:     []string{AgentErrCodeResourceNotFound},
		},
		{
			Name:           "get_fleet_context",
			Title:          "Get fleet context",
			Description:    "Return a thin per-resource triage rollup across every resource visible to the org — identity, operator flags (intentionallyOffline, neverAutoRemediate, maintenanceWindowActive), per-severity finding counts (total/critical/warning/info), and pending-approval count. One read for 'where do I focus?'; follow up via get_resource_context for depth. Optional additive filters (hasFindings, severity, technology, resourceType) narrow the result to a relevant subset so agents triaging a large fleet do not receive or page through healthy resources.",
			Category:       "context",
			Method:         http.MethodGet,
			Path:           FleetContextCapabilityPath,
			Scope:          agentCapabilityScopeMonitoringRead,
			ActionMode:     agentCapabilityActionModeRead,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "AgentFleetContext",
			InputSchema:    fleetContextInputSchema(),
			OutputSchema:   fleetContextOutputSchema(),
		},
		{
			Name:           OperationsLoopStatusCapabilityName,
			Title:          "Get Patrol work status",
			Description:    "Return the current content-safe Patrol work status: Patrol issue evidence, pending approvals, governed decisions/actions, verified outcomes, Patrol control outcome evidence, compatibility aliases, and optional MCP readiness. The payload is count-only and deliberately omits finding ids, action ids, prompts, commands, resource names, and output.",
			Category:       "context",
			Method:         http.MethodGet,
			Path:           OperationsLoopStatusCapabilityPath,
			Scope:          agentCapabilityScopeMonitoringRead,
			ActionMode:     agentCapabilityActionModeRead,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "AgentOperationsLoopStatus",
			OutputSchema:   operationsLoopStatusOutputSchema(),
		},
		{
			Name:           ListNodesCapabilityName,
			Title:          "List nodes",
			Description:    "List configured infrastructure sources that Pulse can monitor or manage. Credential secret values are redacted; use the returned id with update_node, remove_node, test_node_connection, or refresh_node_cluster_membership.",
			Category:       "provisioning",
			Method:         http.MethodGet,
			Path:           "/api/config/nodes",
			Scope:          agentCapabilityScopeSettingsRead,
			ActionMode:     agentCapabilityActionModeRead,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "NodeResponse[]",
			OutputSchema:   toolArrayOutputSchema("Configured infrastructure source records visible to the agent."),
		},
		{
			Name:             AddNodeCapabilityName,
			Title:            "Add node",
			Description:      "Add a Proxmox VE, Proxmox Backup Server, or Proxmox Mail Gateway source to Pulse after credentials have been collected, generated, or approved.",
			Category:         "provisioning",
			Method:           http.MethodPost,
			Path:             "/api/config/nodes",
			Scope:            agentCapabilityScopeSettingsWrite,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "NodeConfigRequest",
			ResponseShape:    "{ status?: \"success\", success?: true, message?: string }",
			InputSchema:      addNodeInputSchema(),
			OutputSchema:     nodeMutationOutputSchema(nil),
		},
		{
			Name:             UpdateNodeCapabilityName,
			Title:            "Update node",
			Description:      "Update a configured infrastructure source. Omitted fields preserve the current value; tokenValue or password only changes when supplied.",
			Category:         "provisioning",
			Method:           http.MethodPut,
			Path:             "/api/config/nodes/{nodeId}",
			Scope:            agentCapabilityScopeSettingsWrite,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "NodeConfigRequest",
			ResponseShape:    "{ status: \"success\" }",
			InputSchema:      updateNodeInputSchema(),
			OutputSchema:     nodeMutationOutputSchema([]string{"status"}),
		},
		{
			Name:           RemoveNodeCapabilityName,
			Title:          "Remove node",
			Description:    "Remove a configured infrastructure source from Pulse by node id.",
			Category:       "provisioning",
			Method:         http.MethodDelete,
			Path:           "/api/config/nodes/{nodeId}",
			Scope:          agentCapabilityScopeSettingsWrite,
			ActionMode:     agentCapabilityActionModeWrite,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "{ status: \"success\" }",
			InputSchema:    nodeIDInputSchema(),
			OutputSchema:   nodeMutationOutputSchema([]string{"status"}),
		},
		{
			Name:             TestNodeCredentialsCapabilityName,
			Title:            "Test node credentials",
			Description:      "Validate proposed source credentials and connection details without saving them to Pulse.",
			Category:         "provisioning",
			Method:           http.MethodPost,
			Path:             "/api/config/nodes/test-config",
			Scope:            agentCapabilityScopeSettingsWrite,
			ActionMode:       agentCapabilityActionModeMixed,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "NodeConfigRequest",
			ResponseShape:    "{ status: \"success\"|\"error\", message: string }",
			InputSchema:      testNodeCredentialsInputSchema(),
			OutputSchema:     nodeConnectionTestOutputSchema(),
		},
		{
			Name:           TestNodeConnectionCapabilityName,
			Title:          "Test node connection",
			Description:    "Validate the saved connection for an existing configured infrastructure source.",
			Category:       "provisioning",
			Method:         http.MethodPost,
			Path:           "/api/config/nodes/{nodeId}/test",
			Scope:          agentCapabilityScopeSettingsWrite,
			ActionMode:     agentCapabilityActionModeMixed,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "{ status: \"success\"|\"error\", message: string }",
			InputSchema:    nodeIDInputSchema(),
			OutputSchema:   nodeConnectionTestOutputSchema(),
		},
		{
			Name:           RefreshNodeClusterMembershipCapabilityName,
			Title:          "Refresh node cluster membership",
			Description:    "Re-detect Proxmox VE cluster membership and endpoint metadata for a configured source.",
			Category:       "provisioning",
			Method:         http.MethodPost,
			Path:           "/api/config/nodes/{nodeId}/refresh-cluster",
			Scope:          agentCapabilityScopeSettingsWrite,
			ActionMode:     agentCapabilityActionModeWrite,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "ClusterRefreshResponse",
			InputSchema:    nodeIDInputSchema(),
			OutputSchema:   clusterRefreshOutputSchema(),
		},
		{
			Name:             DiscoverLANCapabilityName,
			Title:            "Discover LAN",
			Description:      "Scan a subnet, or return cached scan results, to find candidate infrastructure hosts before deciding which sources to add to Pulse.",
			Category:         "provisioning",
			Method:           http.MethodPost,
			Path:             "/api/discover",
			Scope:            agentCapabilityScopeSettingsWrite,
			ActionMode:       agentCapabilityActionModeMixed,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "{ subnet?: string, use_cache?: boolean }",
			ResponseShape:    "ManualDiscoveryResult",
			InputSchema:      discoverLANInputSchema(),
			OutputSchema:     discoveryOutputSchema(),
		},
		{
			Name:           GetOperatorStateCapabilityName,
			Title:          "Get operator state",
			Description:    "Read the operator-set state for a resource (intentionally offline, never auto-remediate, maintenance window, criticality).",
			Category:       "operator-state",
			Method:         http.MethodGet,
			Path:           OperatorStateCapabilityPath,
			Scope:          agentCapabilityScopeMonitoringRead,
			ActionMode:     agentCapabilityActionModeRead,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "ResourceOperatorState",
			OutputSchema:   operatorStateOutputSchema(),
			ErrorCodes:     []string{AgentErrCodeOperatorStateNotSet},
		},
		{
			Name:             SetOperatorStateCapabilityName,
			Title:            "Set operator state",
			Description:      "Replace the operator-set state for a resource. URL canonicalId wins over body; server populates setAt and setBy from the authenticated identity.",
			Category:         "operator-state",
			Method:           http.MethodPut,
			Path:             OperatorStateCapabilityPath,
			Scope:            agentCapabilityScopeMonitoringWrite,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "ResourceOperatorStateInput",
			ResponseShape:    "ResourceOperatorState",
			OutputSchema:     operatorStateOutputSchema(),
			ErrorCodes:       []string{AgentErrCodeOperatorStateInvalid},
			InputSchema:      operatorStateInputSchema(),
		},
		{
			Name:           ClearOperatorStateCapabilityName,
			Title:          "Clear operator state",
			Description:    "Remove any operator-set state for a resource. Idempotent — succeeds whether or not an entry was present.",
			Category:       "operator-state",
			Method:         http.MethodDelete,
			Path:           OperatorStateCapabilityPath,
			Scope:          agentCapabilityScopeMonitoringWrite,
			ActionMode:     agentCapabilityActionModeWrite,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
		},
		{
			Name:           "subscribe_events",
			Title:          "Subscribe events",
			Description:    SubscribeEventsDescription(),
			Category:       "context",
			Method:         http.MethodGet,
			Path:           "/api/agent/events",
			Scope:          agentCapabilityScopeMonitoringRead,
			ActionMode:     agentCapabilityActionModeRead,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "text/event-stream of AgentEvent",
		},
		{
			Name:           ListFindingsCapabilityName,
			Title:          "List findings",
			Description:    "List all Patrol findings (active, dismissed, resolved). Filter client-side on returned shape.",
			Category:       "finding",
			Method:         http.MethodGet,
			Path:           "/api/ai/patrol/findings",
			Scope:          agentCapabilityScopeAIExecute,
			ActionMode:     agentCapabilityActionModeRead,
			ApprovalPolicy: agentCapabilityApprovalPolicyScopeOnly,
			ResponseShape:  "Finding[]",
			OutputSchema:   toolArrayOutputSchema("Patrol finding records returned by the lifecycle endpoint."),
		},
		{
			Name:             AcknowledgeFindingCapabilityName,
			Title:            "Acknowledge finding",
			Description:      "Mark a finding as seen but keep it visible. Auto-resolves when the underlying condition clears.",
			Category:         "finding",
			Method:           http.MethodPost,
			Path:             "/api/ai/patrol/acknowledge",
			Scope:            agentCapabilityScopeAIExecute,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "{ finding_id: string }",
			ResponseShape:    "{ success: true, message: string }",
			ErrorCodes:       agentCapabilityFindingErrorCodes,
			InputSchema:      findingIDInputSchema("Patrol finding id to acknowledge, returned by " + ListFindingsCapabilityName + "."),
			OutputSchema:     findingActionOutputSchema(),
		},
		{
			Name:             SnoozeFindingCapabilityName,
			Title:            "Snooze finding",
			Description:      "Hide a finding for a defined duration in hours.",
			Category:         "finding",
			Method:           http.MethodPost,
			Path:             "/api/ai/patrol/snooze",
			Scope:            agentCapabilityScopeAIExecute,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "{ finding_id: string, duration_hours: number }",
			ResponseShape:    "{ success: true, message: string }",
			ErrorCodes:       agentCapabilityFindingErrorCodes,
			InputSchema:      snoozeFindingInputSchema(),
			OutputSchema:     findingActionOutputSchema(),
		},
		{
			Name:             DismissFindingCapabilityName,
			Title:            "Dismiss finding",
			Description:      "Dismiss a finding with a reason: not_an_issue (permanent suppression), expected_behavior (acknowledged forever), or will_fix_later (7-day reminder commitment).",
			Category:         "finding",
			Method:           http.MethodPost,
			Path:             "/api/ai/patrol/dismiss",
			Scope:            agentCapabilityScopeAIExecute,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "{ finding_id: string, reason: \"not_an_issue\"|\"expected_behavior\"|\"will_fix_later\", note?: string }",
			ResponseShape:    "{ success: true, message: string }",
			ErrorCodes:       agentCapabilityFindingErrorCodes,
			InputSchema:      dismissFindingInputSchema(),
			OutputSchema:     findingActionOutputSchema(),
		},
		{
			Name:             ResolveFindingCapabilityName,
			Title:            "Resolve finding",
			Description:      "Manually mark a finding as resolved when the underlying issue has been fixed out-of-band.",
			Category:         "finding",
			Method:           http.MethodPost,
			Path:             "/api/ai/patrol/resolve",
			Scope:            agentCapabilityScopeAIExecute,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyScopeOnly,
			RequestBodyShape: "{ finding_id: string, resolution_note?: string }",
			ResponseShape:    "{ success: true, message: string }",
			ErrorCodes:       agentCapabilityFindingErrorCodes,
			InputSchema:      resolveFindingInputSchema(),
			OutputSchema:     findingActionOutputSchema(),
		},
		{
			Name:             PlanActionCapabilityName,
			Title:            "Plan action",
			Description:      "Plan an action against a resource. The planner validates the request, looks up the capability on the resource, checks executor-owned live availability, and returns an ActionPlan with the approval policy, blast radius, plan hash, and preflight summary. The plan is persisted to the audit history at the planned/pending state only after the live availability check passes, so subsequent decide_action and execute_action calls can reference it by id. Plan-and-execute is a two-step flow when the resulting plan requires approval, one-step otherwise.",
			Category:         "action",
			Method:           http.MethodPost,
			Path:             PlanActionCapabilityPath,
			Scope:            agentCapabilityScopeActionsPlan,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyActionPlan,
			RequestBodyShape: "ActionRequest",
			ResponseShape:    "ActionPlan",
			ErrorCodes:       agentCapabilityPlanActionErrorCodes,
			InputSchema:      actionPlanInputSchema(),
			OutputSchema:     actionPlanOutputSchema(),
		},
		{
			Name:             DecideActionCapabilityName,
			Title:            "Decide action",
			Description:      "Record an approval decision (approved or rejected) on a previously planned action. The actor is taken from the authenticated identity; an explicit reason can be passed in the body. An exact retry returns the authoritative persisted decision without adding an approval or lifecycle event; a conflicting retry fails closed.",
			Category:         "action",
			Method:           http.MethodPost,
			Path:             ActionDecisionCapabilityPath,
			Scope:            agentCapabilityScopeActionsApprove,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyActionPlan,
			RequestBodyShape: "{ outcome: \"approved\"|\"rejected\", reason?: string }",
			ResponseShape:    "ActionDecisionResponse",
			ErrorCodes:       agentCapabilityDecisionActionErrorCodes,
			InputSchema:      actionDecisionInputSchema(),
			OutputSchema:     actionDecisionOutputSchema(),
		},
		{
			Name:             ExecuteActionCapabilityName,
			Title:            "Execute action",
			Description:      "Execute a previously planned and (when required) approved action. Returns the persisted audit record with the execution result attached. Refuses with stable codes when the action is in the wrong lifecycle state (action_not_approved, action_already_executing, action_execution_final, action_dry_run_only, action_plan_expired), when executor-owned live readiness is no longer available (action_execution_unavailable), when the approved plan no longer matches the current resource/capability contract (action_plan_drift), when the target is operator-locked against automated remediation (resource_remediation_locked), or when the API instance has no executor wired (action_executor_unavailable). Both human and automatic policy execution recheck readiness before dispatch admission. action.completed SSE events fire on every terminal state so agents watching the stream do not need to poll this endpoint after dispatch.",
			Category:         "action",
			Method:           http.MethodPost,
			Path:             ActionExecutionCapabilityPath,
			Scope:            agentCapabilityScopeActionsExecute,
			ActionMode:       agentCapabilityActionModeWrite,
			ApprovalPolicy:   agentCapabilityApprovalPolicyActionPlan,
			RequestBodyShape: "{ reason?: string }",
			ResponseShape:    "ActionExecutionResponse",
			ErrorCodes:       agentCapabilityExecuteActionErrorCodes,
			InputSchema:      actionExecutionInputSchema(),
			OutputSchema:     actionExecutionOutputSchema(),
		},
	},
}

// CanonicalManifest returns Pulse's shared agent capabilities manifest. The
// returned value is detached from the package-owned manifest so callers cannot
// mutate the process-wide contract by accident.
func CanonicalManifest() Manifest {
	capabilities := CloneCapabilities(canonicalManifest.Capabilities)
	surfaceContract := CloneSurfaceContract(canonicalManifest.SurfaceContract)
	publishedSurfaceToolContracts := CloneSurfaceToolContracts(canonicalManifest.SurfaceToolContracts)
	surfaceToolContracts := ProjectManifestSurfaceToolContracts(Manifest{
		SurfaceContract:      surfaceContract,
		SurfaceToolContracts: publishedSurfaceToolContracts,
		Capabilities:         capabilities,
	})
	return Manifest{
		Version:              canonicalManifest.Version,
		SurfaceContract:      surfaceContract,
		SurfaceToolContracts: CloneSurfaceToolContracts(surfaceToolContracts),
		MCPAdapter:           CloneMCPAdapterContract(canonicalManifest.MCPAdapter),
		RequiredScopes:       RequiredCapabilityScopes(capabilities),
		Categories:           CloneCapabilityCategories(canonicalManifest.Categories),
		WorkflowPrompts:      ClonePulseWorkflowPrompts(ProjectPulseWorkflowPrompts(capabilities)),
		Capabilities:         capabilities,
	}
}

func canonicalPulseMCPSurfaceToolNames() []string {
	return []string{
		ResourceContextCapabilityName,
		ListResourceCapabilitiesCapabilityName,
		FleetContextCapabilityName,
		OperationsLoopStatusCapabilityName,
		ListNodesCapabilityName,
		AddNodeCapabilityName,
		UpdateNodeCapabilityName,
		RemoveNodeCapabilityName,
		TestNodeCredentialsCapabilityName,
		TestNodeConnectionCapabilityName,
		RefreshNodeClusterMembershipCapabilityName,
		DiscoverLANCapabilityName,
		GetOperatorStateCapabilityName,
		SetOperatorStateCapabilityName,
		ClearOperatorStateCapabilityName,
		ListFindingsCapabilityName,
		AcknowledgeFindingCapabilityName,
		SnoozeFindingCapabilityName,
		DismissFindingCapabilityName,
		ResolveFindingCapabilityName,
		PlanActionCapabilityName,
		DecideActionCapabilityName,
		ExecuteActionCapabilityName,
	}
}
