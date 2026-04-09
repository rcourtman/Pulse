package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

type monitoredSystemLimitBlockedPayload struct {
	Error                  string                               `json:"error"`
	Feature                string                               `json:"feature"`
	Message                string                               `json:"message"`
	MonitoredSystemPreview MonitoredSystemLedgerPreviewResponse `json:"monitored_system_preview"`
}

func decodeMonitoredSystemLimitBlockedPayload(
	t *testing.T,
	body []byte,
) monitoredSystemLimitBlockedPayload {
	t.Helper()

	var payload monitoredSystemLimitBlockedPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode monitored-system limit blocked payload: %v", err)
	}
	return payload
}

func readAPIPackageFile(t *testing.T, name string) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve API package test file")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(filename), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

func requireContainsSnippet(t *testing.T, source string, snippet string) {
	t.Helper()

	if !strings.Contains(source, snippet) {
		t.Fatalf("missing snippet %q", snippet)
	}
}

func requireSnippetCountAtLeast(t *testing.T, source string, snippet string, min int) {
	t.Helper()

	if got := strings.Count(source, snippet); got < min {
		t.Fatalf("snippet %q count = %d, want at least %d", snippet, got, min)
	}
}

func requireSnippetBefore(t *testing.T, source string, earlier string, later string) {
	t.Helper()

	earlierIndex := strings.Index(source, earlier)
	if earlierIndex < 0 {
		t.Fatalf("missing earlier snippet %q", earlier)
	}
	laterIndex := strings.Index(source, later)
	if laterIndex < 0 {
		t.Fatalf("missing later snippet %q", later)
	}
	if earlierIndex > laterIndex {
		t.Fatalf("snippet %q must appear before %q", earlier, later)
	}
}

func requireSourceSegment(t *testing.T, source string, start string, end string) string {
	t.Helper()

	startIndex := strings.Index(source, start)
	if startIndex < 0 {
		t.Fatalf("missing segment start %q", start)
	}
	remainder := source[startIndex:]
	endIndex := strings.Index(remainder[len(start):], end)
	if endIndex < 0 {
		t.Fatalf("missing segment end %q after %q", end, start)
	}
	return remainder[:len(start)+endIndex]
}

func TestMonitoredSystemLimitDecisionOnlyBlocksNetNewSystems(t *testing.T) {
	ctx := context.Background()

	atLimitExisting := monitoredSystemLimitDecisionFromAdditional(ctx, 5, 5, 0)
	if atLimitExisting.exceeded {
		t.Fatalf("existing monitored systems must continue reporting at the limit: %+v", atLimitExisting)
	}

	atLimitNew := monitoredSystemLimitDecisionFromAdditional(ctx, 5, 5, 1)
	if !atLimitNew.exceeded {
		t.Fatalf("net-new monitored systems must be blocked when the cap is full: %+v", atLimitNew)
	}
}

func TestMonitoredSystemLimitDecisionForInactiveCandidateBypassesUsageAvailability(t *testing.T) {
	ctx := context.Background()
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	decision := monitoredSystemLimitDecisionForCandidate(ctx, nil, unifiedresources.MonitoredSystemCandidate{
		Source:   unifiedresources.SourceTrueNAS,
		Type:     unifiedresources.ResourceTypeAgent,
		Name:     "tower",
		Hostname: "tower.local",
		HostURL:  "https://tower.local",
		State:    unifiedresources.MonitoredSystemCandidateStateInactive,
	})
	if !decision.usageAvailable {
		t.Fatalf("inactive candidate should not require usage availability: %+v", decision)
	}
	if decision.exceeded {
		t.Fatalf("inactive candidate must not exceed the limit: %+v", decision)
	}

	replacementDecision := monitoredSystemLimitDecisionForCandidateReplacement(
		ctx,
		nil,
		unifiedresources.MonitoredSystemReplacement{
			Source: unifiedresources.SourceTrueNAS,
			Selector: unifiedresources.MonitoredSystemReplacementSelector{
				Hostname: "tower.local",
			},
		},
		unifiedresources.MonitoredSystemCandidate{
			Source:   unifiedresources.SourceTrueNAS,
			Type:     unifiedresources.ResourceTypeAgent,
			Name:     "tower",
			Hostname: "tower.local",
			HostURL:  "https://tower.local",
			State:    unifiedresources.MonitoredSystemCandidateStateInactive,
		},
	)
	if !replacementDecision.usageAvailable {
		t.Fatalf("inactive replacement candidate should not require usage availability: %+v", replacementDecision)
	}
	if replacementDecision.exceeded {
		t.Fatalf("inactive replacement candidate must not exceed the limit: %+v", replacementDecision)
	}
}

func TestMonitoredSystemAdmissionSurfacesStayBehindSharedLimitGate(t *testing.T) {
	router := readAPIPackageFile(t, "router_routes_registration.go")

	type sourceContract struct {
		file              string
		requiredSnippets  []string
		requiredCounts    map[string]int
		requiredOrderings [][2]string
	}
	type proofContract struct {
		file     string
		snippets []string
	}
	type admissionSurface struct {
		name           string
		routerSnippets []string
		sources        []sourceContract
		proofs         []proofContract
	}

	surfaces := []admissionSurface{
		{
			name: "unified agent report",
			routerSnippets: []string{
				`"/api/agents/agent/report"`,
				`"/api/agents/host/report"`,
				"r.unifiedAgentHandlers.HandleReport",
			},
			sources: []sourceContract{{
				file: "agent_ingest.go",
				requiredSnippets: []string{
					"enforceMonitoredSystemLimitForHostReport(",
					"ApplyHostReport(report, tokenRecord)",
				},
				requiredOrderings: [][2]string{
					{"enforceMonitoredSystemLimitForHostReport(", "ApplyHostReport(report, tokenRecord)"},
				},
			}},
			proofs: []proofContract{{
				file: "unified_agent_handlers_test.go",
				snippets: []string{
					"TestUnifiedAgentHandlers_HandleReport_EnforcesMaxMonitoredSystemsForNewHostsOnly",
					"Existing host should continue to report at the limit.",
					"New host should be blocked.",
					"http.StatusPaymentRequired",
				},
			}},
		},
		{
			name: "docker agent report",
			routerSnippets: []string{
				`"/api/agents/docker/report"`,
				"r.dockerAgentHandlers.HandleReport",
			},
			sources: []sourceContract{{
				file: "docker_agents.go",
				requiredSnippets: []string{
					"enforceMonitoredSystemLimitForDockerReport(",
					"ApplyDockerReport(report, tokenRecord)",
				},
				requiredOrderings: [][2]string{
					{"enforceMonitoredSystemLimitForDockerReport(", "ApplyDockerReport(report, tokenRecord)"},
				},
			}},
			proofs: []proofContract{{
				file: "docker_agents_additional_test.go",
				snippets: []string{
					"TestDockerAgentHandlers_HandleReport_BlocksNewMonitoredSystemAtLimit",
					"http.StatusPaymentRequired",
				},
			}},
		},
		{
			name: "kubernetes agent report",
			routerSnippets: []string{
				`"/api/agents/kubernetes/report"`,
				"r.kubernetesAgentHandlers.HandleReport",
			},
			sources: []sourceContract{{
				file: "kubernetes_agents.go",
				requiredSnippets: []string{
					"enforceMonitoredSystemLimitForKubernetesReport(",
					"ApplyKubernetesReport(report, tokenRecord)",
				},
				requiredOrderings: [][2]string{
					{"enforceMonitoredSystemLimitForKubernetesReport(", "ApplyKubernetesReport(report, tokenRecord)"},
				},
			}},
			proofs: []proofContract{{
				file: "kubernetes_agents_additional_test.go",
				snippets: []string{
					"TestKubernetesAgentHandlers_HandleReport_BlocksNewMonitoredSystemAtLimit",
					"http.StatusPaymentRequired",
				},
			}},
		},
		{
			name: "proxmox family config add and update",
			routerSnippets: []string{
				`"/api/config/nodes"`,
				`"/api/config/nodes/"`,
				"r.configHandlers.HandleAddNode",
				"r.configHandlers.HandleUpdateNode",
			},
			sources: []sourceContract{{
				file: "config_node_handlers.go",
				requiredSnippets: []string{
					"enforceMonitoredSystemLimitForConfigRegistration(",
					"enforceMonitoredSystemLimitForConfigReplacement(",
					"SaveNodesConfig(",
				},
				requiredCounts: map[string]int{
					"enforceMonitoredSystemLimitForConfigRegistration(": 3,
					"enforceMonitoredSystemLimitForConfigReplacement(":  3,
				},
				requiredOrderings: [][2]string{
					{"enforceMonitoredSystemLimitForConfigRegistration(", "PVEInstances = append"},
					{"enforceMonitoredSystemLimitForConfigReplacement(", "*pve = updated"},
				},
			}},
			proofs: []proofContract{
				{
					file: "config_handlers_add_test.go",
					snippets: []string{
						"TestHandleAddNode_BlocksNewCountedSystemAtLimit",
						"http.StatusPaymentRequired",
					},
				},
				{
					file: "config_handlers_update_test.go",
					snippets: []string{
						"TestHandleUpdateNode_BlocksProjectedNetNewSystemAtLimit",
						"http.StatusPaymentRequired",
					},
				},
			},
		},
		{
			name: "auto registration",
			routerSnippets: []string{
				`"/api/auto-register"`,
				"r.configHandlers.HandleAutoRegister",
			},
			sources: []sourceContract{{
				file: "config_setup_handlers.go",
				requiredSnippets: []string{
					"enforceMonitoredSystemLimitForConfigRegistration(",
					"SaveNodesConfig(",
				},
				requiredCounts: map[string]int{
					"enforceMonitoredSystemLimitForConfigRegistration(": 2,
				},
				requiredOrderings: [][2]string{
					{"enforceMonitoredSystemLimitForConfigRegistration(", "PVEInstances = append"},
				},
			}},
			proofs: []proofContract{{
				file: "config_handlers_auto_register_test.go",
				snippets: []string{
					"TestHandleAutoRegister_BlocksNewCountedSystemAtLimit",
					"http.StatusPaymentRequired",
				},
			}},
		},
		{
			name: "truenas connection add and update",
			routerSnippets: []string{
				`"/api/truenas/connections"`,
				`"/api/truenas/connections/"`,
				"r.trueNASHandlers.HandleAdd",
				"r.trueNASHandlers.HandleUpdate",
			},
			sources: []sourceContract{{
				file: "truenas_handlers.go",
				requiredSnippets: []string{
					"monitoredSystemLimitDecisionForCandidate(",
					"monitoredSystemLimitDecisionForCandidateReplacement(",
					"SaveTrueNASConfig(",
				},
				requiredOrderings: [][2]string{
					{"h.enforceMonitoredSystemLimit(w, r, instance)", "existing = append(existing, instance)"},
					{"h.enforceMonitoredSystemLimitReplacement(w, r, instances[index], instance)", "instances[index] = instance"},
				},
			}},
			proofs: []proofContract{{
				file: "truenas_handlers_test.go",
				snippets: []string{
					"TestTrueNASHandlers_HandleAdd_BlocksNewCountedSystemAtLimit",
					"TestTrueNASHandlers_HandleUpdate_BlocksProjectedNetNewSystemAtLimit",
					"http.StatusPaymentRequired",
					"license_required",
				},
			}},
		},
		{
			name: "vmware connection add and update",
			routerSnippets: []string{
				`"/api/vmware/connections"`,
				`"/api/vmware/connections/"`,
				"r.vmwareHandlers.HandleAdd",
				"r.vmwareHandlers.HandleUpdate",
			},
			sources: []sourceContract{{
				file: "vmware_handlers.go",
				requiredSnippets: []string{
					"monitoredSystemLimitDecisionForRecordsFromUsage(",
					"monitoredSystemLimitDecisionForRecordsReplacementFromUsage(",
					"SaveVMwareConfig(",
				},
				requiredOrderings: [][2]string{
					{"h.enforceMonitoredSystemLimit(w, r, instance)", "instances = append(instances, instance)"},
					{"h.enforceMonitoredSystemLimitReplacement(w, r, instances[index], instance)", "instances[index] = instance"},
				},
			}},
			proofs: []proofContract{{
				file: "vmware_handlers_test.go",
				snippets: []string{
					"TestVMwareHandlers_HandleAdd_BlocksProjectedNetNewSystemsAtLimit",
					"TestVMwareHandlers_HandleUpdate_BlocksProjectedNetNewSystemsAtLimit",
					"http.StatusPaymentRequired",
					"license_required",
				},
			}},
		},
		{
			name: "agent deployment jobs",
			routerSnippets: []string{
				`"/api/clusters/"`,
				`"/api/agent-deploy/jobs/"`,
				"r.deployHandlers.HandleCreateJob",
				"r.deployHandlers.HandleRetryJob",
			},
			sources: []sourceContract{{
				file: "deploy_handlers.go",
				requiredSnippets: []string{
					"monitoredSystemLimitDecisionForAdditionalSlots(ctx, h.monitor, 0)",
					`Reason: "skipped_license"`,
					`"license_limit"`,
				},
				requiredCounts: map[string]int{
					"monitoredSystemLimitDecisionForAdditionalSlots(ctx, h.monitor, 0)": 2,
				},
				requiredOrderings: [][2]string{
					{"monitoredSystemLimitDecisionForAdditionalSlots(ctx, h.monitor, 0)", `Reason: "skipped_license"`},
					{"monitoredSystemLimitDecisionForAdditionalSlots(ctx, h.monitor, 0)", `"license_limit"`},
				},
			}},
			proofs: []proofContract{{
				file: "deploy_handlers_test.go",
				snippets: []string{
					"TestHandleCreateJob_TruncatesTargetsToAvailableLicenseSlots",
					"TestHandleRetryJob_BlocksWhenNoLicenseSlotsAvailable",
					"skipped_license",
					"license_limit",
				},
			}},
		},
	}

	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			for _, snippet := range surface.routerSnippets {
				requireContainsSnippet(t, router, snippet)
			}
			for _, contract := range surface.sources {
				source := readAPIPackageFile(t, contract.file)
				for _, snippet := range contract.requiredSnippets {
					requireContainsSnippet(t, source, snippet)
				}
				for snippet, min := range contract.requiredCounts {
					requireSnippetCountAtLeast(t, source, snippet, min)
				}
				for _, ordering := range contract.requiredOrderings {
					requireSnippetBefore(t, source, ordering[0], ordering[1])
				}
			}
			for _, proof := range surface.proofs {
				source := readAPIPackageFile(t, proof.file)
				for _, snippet := range proof.snippets {
					requireContainsSnippet(t, source, snippet)
				}
			}
		})
	}
}

func TestVMwareAdmissionEnforcementChecksUsageBeforeExternalInventory(t *testing.T) {
	source := readAPIPackageFile(t, "vmware_handlers.go")

	addEnforcement := requireSourceSegment(
		t,
		source,
		"func (h *VMwareHandlers) enforceMonitoredSystemLimit(",
		"func (h *VMwareHandlers) enforceMonitoredSystemLimitReplacement(",
	)
	requireSnippetBefore(
		t,
		addEnforcement,
		"usage := monitoredSystemUsage(monitor)",
		"records, invalidConfig, err := h.previewMonitoredSystemRecords(r.Context(), instance)",
	)
	requireSnippetBefore(
		t,
		addEnforcement,
		"if !usage.available",
		"records, invalidConfig, err := h.previewMonitoredSystemRecords(r.Context(), instance)",
	)

	replacementEnforcement := requireSourceSegment(
		t,
		source,
		"func (h *VMwareHandlers) enforceMonitoredSystemLimitReplacement(",
		"func (h *VMwareHandlers) previewMonitoredSystemRecords(",
	)
	requireSnippetBefore(
		t,
		replacementEnforcement,
		"candidate := vmwareMonitoredSystemCandidate(next)",
		"usage := monitoredSystemUsage(monitor)",
	)
	requireSnippetBefore(
		t,
		replacementEnforcement,
		"if !candidate.CountsTowardMonitoredSystems()",
		"usage := monitoredSystemUsage(monitor)",
	)
	requireSnippetBefore(
		t,
		replacementEnforcement,
		"usage := monitoredSystemUsage(monitor)",
		"records, invalidConfig, err := h.previewMonitoredSystemRecords(r.Context(), next)",
	)
	requireSnippetBefore(
		t,
		replacementEnforcement,
		"if !usage.available",
		"records, invalidConfig, err := h.previewMonitoredSystemRecords(r.Context(), next)",
	)
}

func TestVMwareAdmissionEnforcementSkipsUsageAndInventoryForDisabledConnections(t *testing.T) {
	source := readAPIPackageFile(t, "vmware_handlers.go")

	addEnforcement := requireSourceSegment(
		t,
		source,
		"func (h *VMwareHandlers) enforceMonitoredSystemLimit(",
		"func (h *VMwareHandlers) enforceMonitoredSystemLimitReplacement(",
	)
	requireSnippetBefore(
		t,
		addEnforcement,
		"candidate := vmwareMonitoredSystemCandidate(instance)",
		"usage := monitoredSystemUsage(monitor)",
	)
	requireSnippetBefore(
		t,
		addEnforcement,
		"if !candidate.CountsTowardMonitoredSystems()",
		"usage := monitoredSystemUsage(monitor)",
	)
	requireSnippetBefore(
		t,
		addEnforcement,
		"if !candidate.CountsTowardMonitoredSystems()",
		"records, invalidConfig, err := h.previewMonitoredSystemRecords(r.Context(), instance)",
	)

	replacementEnforcement := requireSourceSegment(
		t,
		source,
		"func (h *VMwareHandlers) enforceMonitoredSystemLimitReplacement(",
		"func (h *VMwareHandlers) previewMonitoredSystemRecords(",
	)
	requireSnippetBefore(
		t,
		replacementEnforcement,
		"candidate := vmwareMonitoredSystemCandidate(next)",
		"usage := monitoredSystemUsage(monitor)",
	)
	requireSnippetBefore(
		t,
		replacementEnforcement,
		"if !candidate.CountsTowardMonitoredSystems()",
		"usage := monitoredSystemUsage(monitor)",
	)
	requireSnippetBefore(
		t,
		replacementEnforcement,
		"if !candidate.CountsTowardMonitoredSystems()",
		"records, invalidConfig, err := h.previewMonitoredSystemRecords(r.Context(), next)",
	)
}

func TestMonitoredSystemCountNilMonitor(t *testing.T) {
	got := monitoredSystemCount(nil)
	if got != 0 {
		t.Fatalf("expected 0 for nil monitor, got %d", got)
	}
}

func TestLegacyConnectionCountsFromReadState(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{
		{
			SourceID: "pve-1",
			Resource: unifiedresources.Resource{
				ID:      "pve-1",
				Name:    "pve-1",
				Type:    unifiedresources.ResourceTypeAgent,
				Status:  unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{},
			},
		},
		{
			SourceID: "pve-2",
			Resource: unifiedresources.Resource{
				ID:      "pve-2",
				Name:    "pve-2",
				Type:    unifiedresources.ResourceTypeAgent,
				Status:  unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{},
				Agent:   &unifiedresources.AgentData{},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceDocker, []unifiedresources.IngestRecord{
		{
			SourceID: "docker-1",
			Resource: unifiedresources.Resource{
				ID:     "docker-1",
				Name:   "docker-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Status: unifiedresources.StatusOnline,
				Docker: &unifiedresources.DockerData{},
			},
		},
		{
			SourceID: "docker-2",
			Resource: unifiedresources.Resource{
				ID:     "docker-2",
				Name:   "docker-2",
				Type:   unifiedresources.ResourceTypeAgent,
				Status: unifiedresources.StatusOnline,
				Docker: &unifiedresources.DockerData{},
				Agent:  &unifiedresources.AgentData{},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceK8s, []unifiedresources.IngestRecord{
		{
			SourceID: "k8s-1",
			Resource: unifiedresources.Resource{
				ID:         "k8s-1",
				Name:       "prod",
				Type:       unifiedresources.ResourceTypeK8sCluster,
				Status:     unifiedresources.StatusOnline,
				Kubernetes: &unifiedresources.K8sData{AgentID: "legacy-k8s-1"},
			},
		},
	})

	counts := legacyConnectionCountsFromReadState(unifiedresources.NewMonitorAdapter(registry))
	if counts.KubernetesClusters != 0 {
		t.Fatalf("expected kubernetes_clusters=0, got %d", counts.KubernetesClusters)
	}
	if counts.ProxmoxNodes != 0 || counts.DockerHosts != 0 {
		t.Fatalf("expected legacy connection counts to stay zero under monitored-system counting, got %+v", counts)
	}
}

func TestLegacyConnectionCountsUsesSnapshotFallback(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{
		{
			SourceID: "pve-1",
			Resource: unifiedresources.Resource{
				ID:      "pve-1",
				Name:    "pve-1",
				Type:    unifiedresources.ResourceTypeAgent,
				Status:  unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceK8s, []unifiedresources.IngestRecord{
		{
			SourceID: "k8s-1",
			Resource: unifiedresources.Resource{
				ID:         "k8s-1",
				Name:       "prod",
				Type:       unifiedresources.ResourceTypeK8sCluster,
				Status:     unifiedresources.StatusOnline,
				Kubernetes: &unifiedresources.K8sData{AgentID: "legacy-k8s-1"},
			},
		},
	})

	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))

	counts := legacyConnectionCounts(monitor)
	if counts.ProxmoxNodes != 0 || counts.DockerHosts != 0 || counts.KubernetesClusters != 0 {
		t.Fatalf("expected legacy connection counts to stay zero, got %+v", counts)
	}
}

func TestHostReportTargetsExistingHostBridge(t *testing.T) {
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{{ID: "host-1"}},
	}

	t.Run("matches_existing", func(t *testing.T) {
		report := agentshost.Report{
			Host: agentshost.HostInfo{ID: "host-1"},
		}
		if !hostReportTargetsExistingHost(snapshot.Hosts, report, nil) {
			t.Fatal("expected match by host ID")
		}
	})

	t.Run("no_match_for_new_host", func(t *testing.T) {
		report := agentshost.Report{
			Host: agentshost.HostInfo{ID: "host-new", Hostname: "new-server"},
		}
		if hostReportTargetsExistingHost(snapshot.Hosts, report, nil) {
			t.Fatal("expected no match for unknown host")
		}
	})

	t.Run("token_record_forwarded", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			Hosts: []models.Host{{Hostname: "srv-1", TokenID: "token-a"}},
		}
		report := agentshost.Report{
			Host: agentshost.HostInfo{Hostname: "srv-1"},
		}
		token := &config.APITokenRecord{ID: "token-a"}
		if !hostReportTargetsExistingHost(snapshot.Hosts, report, token) {
			t.Fatal("expected match with matching token")
		}
		wrongToken := &config.APITokenRecord{ID: "token-b"}
		if hostReportTargetsExistingHost(snapshot.Hosts, report, wrongToken) {
			t.Fatal("expected no match with different token")
		}
	})
}

func TestDeployReservedCount(t *testing.T) {
	// Nil counter returns 0.
	SetDeployReservationCounter(nil)
	if got := deployReservedCount(context.Background()); got != 0 {
		t.Fatalf("expected 0 with nil counter, got %d", got)
	}

	// Wired counter returns value.
	SetDeployReservationCounter(func(_ context.Context) int { return 5 })
	t.Cleanup(func() { SetDeployReservationCounter(nil) })

	if got := deployReservedCount(context.Background()); got != 5 {
		t.Fatalf("expected 5, got %d", got)
	}
}
