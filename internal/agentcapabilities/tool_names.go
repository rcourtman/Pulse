package agentcapabilities

const (
	PulseQueryToolName      = "pulse_query"
	PulseDiscoveryToolName  = "pulse_discovery"
	PulseMetricsToolName    = "pulse_metrics"
	PulseStorageToolName    = "pulse_storage"
	PulseDockerToolName     = "pulse_docker"
	PulseKubernetesToolName = "pulse_kubernetes"
	PulseAlertsToolName     = "pulse_alerts"
	PulseReadToolName       = "pulse_read"
	PulseControlToolName    = "pulse_control"
	PulseFileEditToolName   = "pulse_file_edit"
	PulseKnowledgeToolName  = "pulse_knowledge"
	PulsePMGToolName        = "pulse_pmg"
	PulseSummarizeToolName  = "pulse_summarize"

	PulseRunCommandToolName    = "pulse_run_command"
	PulseControlGuestToolName  = "pulse_control_guest"
	PulseControlDockerToolName = "pulse_control_docker"

	PulseSearchResourcesToolName       = "pulse_search_resources"
	PulseGetResourceToolName           = "pulse_get_resource"
	PulseGetTopologyToolName           = "pulse_get_topology"
	PulseListInfrastructureToolName    = "pulse_list_infrastructure"
	PulseGetConnectionHealthToolName   = "pulse_get_connection_health"
	PulseGetDockerLogsToolName         = "pulse_get_docker_logs"
	PulseGetPerformanceMetricsToolName = "pulse_get_performance_metrics"
	PulseGetTemperaturesToolName       = "pulse_get_temperatures"
	PulseGetBaselinesToolName          = "pulse_get_baselines"
	PulseGetPatternsToolName           = "pulse_get_patterns"

	PatrolGetFindingsToolName    = "patrol_get_findings"
	PatrolReportFindingToolName  = "patrol_report_finding"
	PatrolResolveFindingToolName = "patrol_resolve_finding"
	// PatrolProposeActionToolName is the side-effect-free typed action
	// proposal capture for Patrol investigations. Mutation-none: it
	// records a validated proposal in the request-local capture sink;
	// planning, approval, and execution stay on the canonical action
	// lifecycle.
	PatrolProposeActionToolName = "patrol_propose_action"
)
