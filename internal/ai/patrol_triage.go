package ai

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TriageFlag represents a single deterministic issue detected during triage.
type TriageFlag struct {
	ResourceID   string  // e.g., "node/pve1", "qemu/100"
	ResourceName string  // e.g., "pve1", "webserver-01"
	ResourceType string  // "node", "vm", "container", "storage", "docker_host", "pbs", "pmg"
	Category     string  // "performance", "capacity", "backup", "health", "connectivity", "anomaly"
	Severity     string  // "critical", "warning", "watch"
	Reason       string  // Human-readable: "Memory at 92% (threshold: 80%)"
	Metric       string  // "cpu", "memory", "disk", "usage" (optional, empty if N/A)
	Value        float64 // Current value as percent (optional)
	Threshold    float64 // Threshold exceeded as percent (optional)
}

// TriageResult is the output of deterministic triage.
type TriageResult struct {
	Flags      []TriageFlag
	Summary    TriageSummary
	IsQuiet    bool                          // True when no flags AND no active findings
	Intel      seedIntelligence              // Pre-computed intelligence (anomalies, forecasts, etc.)
	GuestIntel map[string]*GuestIntelligence // Guest reachability data
	FlaggedIDs map[string]bool               // Set of flagged resource IDs for scoped seed building
}

// TriageSummary provides top-level infrastructure counts.
type TriageSummary struct {
	TotalNodes    int
	TotalGuests   int
	RunningGuests int
	StoppedGuests int
	TotalStorage  int
	TotalDocker   int
	TotalPBS      int
	TotalPMG      int
	FlaggedCount  int
}

// RunDeterministicTriage runs deterministic checks and returns triage output for seed/context use.
func (p *PatrolService) RunDeterministicTriage(
	ctx context.Context,
	state models.StateSnapshot,
	scope *PatrolScope,
	guestIntel map[string]*GuestIntelligence,
) *TriageResult {
	_ = ctx

	scopedSet := p.buildScopedSet(scope)
	intel := p.seedPrecomputeIntelligence(state, scopedSet, time.Now())

	thresholds := p.GetThresholds()
	if triageThresholdsUnset(thresholds) {
		thresholds = DefaultPatrolThresholds()
	}

	flags := make([]TriageFlag, 0)
	flags = append(flags, triageThresholdChecks(state, scopedSet, thresholds)...)
	flags = append(flags, triageAnomalyChecks(intel)...)
	flags = append(flags, triageBackupChecks(state, scopedSet)...)
	flags = append(flags, triageDiskHealthChecks(state, scopedSet)...)
	flags = append(flags, triageAlertChecks(state, scopedSet)...)
	flags = append(flags, triageConnectivityChecks(state, guestIntel, scopedSet)...)
	flags = append(flags, triageRecentChanges(intel)...)
	flags = deduplicateTriageFlags(flags)

	flaggedIDs := make(map[string]bool, len(flags))
	for _, flag := range flags {
		if flag.ResourceID != "" {
			flaggedIDs[flag.ResourceID] = true
		}
	}

	summary := triageBuildSummary(state, flaggedIDs)

	activeFindings := 0
	if p != nil && p.findings != nil {
		activeFindings = len(p.findings.GetActive(FindingSeverityInfo))
	}

	return &TriageResult{
		Flags:      flags,
		Summary:    summary,
		IsQuiet:    len(flags) == 0 && activeFindings == 0,
		Intel:      intel,
		GuestIntel: guestIntel,
		FlaggedIDs: flaggedIDs,
	}
}

func triageThresholdChecks(state models.StateSnapshot, scopedSet map[string]bool, thresholds PatrolThresholds) []TriageFlag {
	flags := make([]TriageFlag, 0)

	for _, n := range state.Nodes {
		if !seedIsInScope(scopedSet, n.ID) {
			continue
		}

		cpu := n.CPU * 100
		if sev, threshold := triageWarnWatchSeverity(cpu, thresholds.NodeCPUWarning, thresholds.NodeCPUWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   n.ID,
				ResourceName: n.Name,
				ResourceType: "node",
				Category:     "performance",
				Severity:     sev,
				Reason:       fmt.Sprintf("CPU at %.0f%% (threshold: %.0f%%)", cpu, threshold),
				Metric:       "cpu",
				Value:        cpu,
				Threshold:    threshold,
			})
		}

		mem := n.Memory.Usage * 100
		if sev, threshold := triageWarnWatchSeverity(mem, thresholds.NodeMemWarning, thresholds.NodeMemWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   n.ID,
				ResourceName: n.Name,
				ResourceType: "node",
				Category:     "performance",
				Severity:     sev,
				Reason:       fmt.Sprintf("Memory at %.0f%% (threshold: %.0f%%)", mem, threshold),
				Metric:       "memory",
				Value:        mem,
				Threshold:    threshold,
			})
		}
	}

	for _, vm := range state.VMs {
		if vm.Template || vm.Status != "running" || !seedIsInScope(scopedSet, vm.ID) {
			continue
		}

		mem := vm.Memory.Usage * 100
		if sev, threshold := triageWarnWatchSeverity(mem, thresholds.GuestMemWarning, thresholds.GuestMemWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   vm.ID,
				ResourceName: vm.Name,
				ResourceType: "vm",
				Category:     "performance",
				Severity:     sev,
				Reason:       fmt.Sprintf("Memory at %.0f%% (threshold: %.0f%%)", mem, threshold),
				Metric:       "memory",
				Value:        mem,
				Threshold:    threshold,
			})
		}

		disk := vm.Disk.Usage * 100
		if sev, threshold := triageCriticalWarnWatchSeverity(disk, thresholds.GuestDiskCrit, thresholds.GuestDiskWarn, thresholds.GuestDiskWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   vm.ID,
				ResourceName: vm.Name,
				ResourceType: "vm",
				Category:     "capacity",
				Severity:     sev,
				Reason:       fmt.Sprintf("Disk at %.0f%% (threshold: %.0f%%)", disk, threshold),
				Metric:       "disk",
				Value:        disk,
				Threshold:    threshold,
			})
		}

		cpu := vm.CPU * 100
		if cpu > thresholds.NodeCPUWarning {
			flags = append(flags, TriageFlag{
				ResourceID:   vm.ID,
				ResourceName: vm.Name,
				ResourceType: "vm",
				Category:     "performance",
				Severity:     "warning",
				Reason:       fmt.Sprintf("CPU at %.0f%% (threshold: %.0f%%)", cpu, thresholds.NodeCPUWarning),
				Metric:       "cpu",
				Value:        cpu,
				Threshold:    thresholds.NodeCPUWarning,
			})
		}
	}

	for _, ct := range state.Containers {
		if ct.Template || ct.Status != "running" || !seedIsInScope(scopedSet, ct.ID) {
			continue
		}

		mem := ct.Memory.Usage * 100
		if sev, threshold := triageWarnWatchSeverity(mem, thresholds.GuestMemWarning, thresholds.GuestMemWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   ct.ID,
				ResourceName: ct.Name,
				ResourceType: "container",
				Category:     "performance",
				Severity:     sev,
				Reason:       fmt.Sprintf("Memory at %.0f%% (threshold: %.0f%%)", mem, threshold),
				Metric:       "memory",
				Value:        mem,
				Threshold:    threshold,
			})
		}

		disk := ct.Disk.Usage * 100
		if sev, threshold := triageCriticalWarnWatchSeverity(disk, thresholds.GuestDiskCrit, thresholds.GuestDiskWarn, thresholds.GuestDiskWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   ct.ID,
				ResourceName: ct.Name,
				ResourceType: "container",
				Category:     "capacity",
				Severity:     sev,
				Reason:       fmt.Sprintf("Disk at %.0f%% (threshold: %.0f%%)", disk, threshold),
				Metric:       "disk",
				Value:        disk,
				Threshold:    threshold,
			})
		}

		cpu := ct.CPU * 100
		if cpu > thresholds.NodeCPUWarning {
			flags = append(flags, TriageFlag{
				ResourceID:   ct.ID,
				ResourceName: ct.Name,
				ResourceType: "container",
				Category:     "performance",
				Severity:     "warning",
				Reason:       fmt.Sprintf("CPU at %.0f%% (threshold: %.0f%%)", cpu, thresholds.NodeCPUWarning),
				Metric:       "cpu",
				Value:        cpu,
				Threshold:    thresholds.NodeCPUWarning,
			})
		}
	}

	for _, s := range state.Storage {
		if !seedIsInScope(scopedSet, s.ID) {
			continue
		}
		usage := s.Usage * 100
		if sev, threshold := triageCriticalWarnWatchSeverity(usage, thresholds.StorageCritical, thresholds.StorageWarning, thresholds.StorageWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   s.ID,
				ResourceName: s.Name,
				ResourceType: "storage",
				Category:     "capacity",
				Severity:     sev,
				Reason:       fmt.Sprintf("Usage at %.0f%% (threshold: %.0f%%)", usage, threshold),
				Metric:       "usage",
				Value:        usage,
				Threshold:    threshold,
			})
		}
	}

	return flags
}

func triageAnomalyChecks(intel seedIntelligence) []TriageFlag {
	flags := make([]TriageFlag, 0)

	for _, anomaly := range intel.anomalies {
		flagSeverity, ok := triageMapAnomalySeverity(anomaly)
		if !ok {
			continue
		}

		resourceName := anomaly.ResourceName
		if resourceName == "" {
			resourceName = anomaly.ResourceID
		}

		current := triageNormalizePercent(anomaly.CurrentValue, anomaly.Metric)
		baselineMean := triageNormalizePercent(anomaly.BaselineMean, anomaly.Metric)

		flags = append(flags, TriageFlag{
			ResourceID:   anomaly.ResourceID,
			ResourceName: resourceName,
			ResourceType: triageResourceType(anomaly.ResourceType, anomaly.ResourceID),
			Category:     "anomaly",
			Severity:     flagSeverity,
			Reason: fmt.Sprintf("%s at %.0f%% vs baseline %.0f%% (z-score %.1f)",
				triageMetricLabel(anomaly.Metric), current, baselineMean, anomaly.ZScore),
			Metric:    anomaly.Metric,
			Value:     current,
			Threshold: baselineMean,
		})
	}

	for _, f := range intel.forecasts {
		if f.daysToFull > 30 || f.daysToFull <= 0 {
			continue
		}

		severity := "watch"
		if f.daysToFull <= 7 {
			severity = "warning"
		}

		flags = append(flags, TriageFlag{
			ResourceID:   f.resourceID,
			ResourceName: f.name,
			ResourceType: triageResourceType("", f.resourceID),
			Category:     "capacity",
			Severity:     severity,
			Reason: fmt.Sprintf("%s will be full in %d days (%+.1f%%/day)",
				triageMetricLabel(f.metric), f.daysToFull, f.dailyChange),
			Metric: f.metric,
			Value:  triageNormalizePercent(f.current, f.metric),
		})
	}

	return flags
}

func triageBackupChecks(state models.StateSnapshot, scopedSet map[string]bool) []TriageFlag {
	flags := make([]TriageFlag, 0)
	now := time.Now()

	for _, vm := range state.VMs {
		if vm.Template || vm.Status != "running" || !seedIsInScope(scopedSet, vm.ID) {
			continue
		}
		flags = append(flags, triageBackupFlag(vm.ID, vm.Name, "vm", vm.LastBackup, now)...)
	}

	for _, ct := range state.Containers {
		if ct.Template || ct.Status != "running" || !seedIsInScope(scopedSet, ct.ID) {
			continue
		}
		flags = append(flags, triageBackupFlag(ct.ID, ct.Name, "container", ct.LastBackup, now)...)
	}

	return flags
}

func triageDiskHealthChecks(state models.StateSnapshot, scopedSet map[string]bool) []TriageFlag {
	flags := make([]TriageFlag, 0)

	for _, d := range state.PhysicalDisks {
		resourceID := d.ID
		if resourceID == "" {
			resourceID = d.DevPath
		}
		if !seedIsInScope(scopedSet, resourceID) {
			continue
		}

		resourceName := d.DevPath
		if resourceName == "" {
			resourceName = d.Model
		}
		if resourceName == "" {
			resourceName = resourceID
		}

		health := strings.TrimSpace(d.Health)
		if health != "" && !strings.EqualFold(health, "PASSED") {
			flags = append(flags, TriageFlag{
				ResourceID:   resourceID,
				ResourceName: resourceName,
				ResourceType: "storage",
				Category:     "health",
				Severity:     "critical",
				Reason:       fmt.Sprintf("Disk health reported %s", health),
				Metric:       "disk",
			})
		}

		if d.Wearout >= 0 && d.Wearout < 20 {
			flags = append(flags, TriageFlag{
				ResourceID:   resourceID,
				ResourceName: resourceName,
				ResourceType: "storage",
				Category:     "health",
				Severity:     "warning",
				Reason:       fmt.Sprintf("SSD wearout at %d%% remaining", d.Wearout),
				Metric:       "disk",
				Value:        float64(d.Wearout),
				Threshold:    20,
			})
		}

		if d.Temperature > 55 {
			flags = append(flags, TriageFlag{
				ResourceID:   resourceID,
				ResourceName: resourceName,
				ResourceType: "storage",
				Category:     "health",
				Severity:     "warning",
				Reason:       fmt.Sprintf("Disk temperature %d°C", d.Temperature),
				Metric:       "disk",
				Value:        float64(d.Temperature),
				Threshold:    55,
			})
		}
	}

	return flags
}

func triageAlertChecks(state models.StateSnapshot, scopedSet map[string]bool) []TriageFlag {
	flags := make([]TriageFlag, 0, len(state.ActiveAlerts))

	for _, alert := range state.ActiveAlerts {
		if !seedIsInScope(scopedSet, alert.ResourceID) {
			continue
		}

		severity := "watch"
		switch strings.ToLower(strings.TrimSpace(alert.Level)) {
		case "critical":
			severity = "critical"
		case "warning":
			severity = "warning"
		}

		reason := strings.TrimSpace(alert.Message)
		if reason == "" {
			reason = fmt.Sprintf("%s alert (%s)", alert.Type, alert.Level)
		}

		resourceName := alert.ResourceName
		if resourceName == "" {
			resourceName = alert.ResourceID
		}

		flags = append(flags, TriageFlag{
			ResourceID:   alert.ResourceID,
			ResourceName: resourceName,
			ResourceType: triageResourceType("", alert.ResourceID),
			Category:     triageAlertCategory(alert.Type),
			Severity:     severity,
			Reason:       reason,
			Metric:       triageAlertMetric(alert.Type),
			Value:        alert.Value,
			Threshold:    alert.Threshold,
		})
	}

	return flags
}

func triageConnectivityChecks(state models.StateSnapshot, guestIntel map[string]*GuestIntelligence, scopedSet map[string]bool) []TriageFlag {
	flags := make([]TriageFlag, 0)

	for resourceID, healthy := range state.ConnectionHealth {
		if healthy || !seedIsInScope(scopedSet, resourceID) {
			continue
		}
		flags = append(flags, TriageFlag{
			ResourceID:   resourceID,
			ResourceName: triageResourceName(resourceID, resourceID),
			ResourceType: triageConnectionResourceType(resourceID),
			Category:     "connectivity",
			Severity:     "critical",
			Reason:       "Instance disconnected",
		})
	}

	for guestID, intel := range guestIntel {
		if intel == nil || intel.Reachable == nil || *intel.Reachable || !seedIsInScope(scopedSet, guestID) {
			continue
		}

		resourceType := "vm"
		if strings.EqualFold(intel.GuestType, "lxc") || strings.EqualFold(intel.GuestType, "container") {
			resourceType = "container"
		}

		flags = append(flags, TriageFlag{
			ResourceID:   guestID,
			ResourceName: triageResourceName(intel.Name, guestID),
			ResourceType: resourceType,
			Category:     "connectivity",
			Severity:     "warning",
			Reason:       "Guest unreachable (ping failed)",
		})
	}

	return flags
}

func triageRecentChanges(intel seedIntelligence) []TriageFlag {
	flags := make([]TriageFlag, 0, len(intel.predictions))

	for _, pred := range intel.predictions {
		reason := fmt.Sprintf("%s predicted in %.1f days (confidence: %.0f%%)",
			strings.ReplaceAll(string(pred.EventType), "_", " "), pred.DaysUntil, pred.Confidence*100)
		if pred.Basis != "" {
			reason = reason + ": " + pred.Basis
		}

		flags = append(flags, TriageFlag{
			ResourceID:   pred.ResourceID,
			ResourceName: triageResourceName("", pred.ResourceID),
			ResourceType: triageResourceType("", pred.ResourceID),
			Category:     "health",
			Severity:     "watch",
			Reason:       reason,
		})
	}

	return flags
}

func deduplicateTriageFlags(flags []TriageFlag) []TriageFlag {
	if len(flags) <= 1 {
		return flags
	}

	result := make([]TriageFlag, 0, len(flags))
	indexByKey := make(map[string]int, len(flags))

	for _, flag := range flags {
		key := triageDedupKey(flag)
		if idx, ok := indexByKey[key]; ok {
			if triageSeverityRank(flag.Severity) > triageSeverityRank(result[idx].Severity) {
				result[idx] = flag
			}
			continue
		}

		indexByKey[key] = len(result)
		result = append(result, flag)
	}

	return result
}

// FormatTriageBriefing renders deterministic triage output as compact markdown.
func FormatTriageBriefing(triage *TriageResult) string {
	if triage == nil {
		return "# Deterministic Triage Results\nNo triage data available.\n"
	}

	var sb strings.Builder
	sb.WriteString("# Deterministic Triage Results\n")

	scanned := triage.Summary.TotalNodes + triage.Summary.TotalGuests + triage.Summary.TotalStorage +
		triage.Summary.TotalDocker + triage.Summary.TotalPBS + triage.Summary.TotalPMG
	sb.WriteString(fmt.Sprintf(
		"Scanned %d resources: %d nodes, %d guests, %d storage, %d docker hosts, %d PBS, %d PMG.\n\n",
		scanned,
		triage.Summary.TotalNodes,
		triage.Summary.TotalGuests,
		triage.Summary.TotalStorage,
		triage.Summary.TotalDocker,
		triage.Summary.TotalPBS,
		triage.Summary.TotalPMG,
	))

	if len(triage.Flags) > 0 {
		flags := append([]TriageFlag(nil), triage.Flags...)
		sort.Slice(flags, func(i, j int) bool {
			ri := triageSeverityRank(flags[i].Severity)
			rj := triageSeverityRank(flags[j].Severity)
			if ri != rj {
				return ri > rj
			}
			if flags[i].ResourceType != flags[j].ResourceType {
				return flags[i].ResourceType < flags[j].ResourceType
			}
			return flags[i].ResourceName < flags[j].ResourceName
		})

		sb.WriteString(fmt.Sprintf("## Flagged Resources (%d)\n", len(flags)))
		sb.WriteString("| Resource | Type | Flag | Severity | Detail |\n")
		sb.WriteString("|----------|------|------|----------|--------|\n")
		for _, flag := range flags {
			resource := triageResourceName(flag.ResourceName, flag.ResourceID)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				triageTableEscape(resource),
				triageTableEscape(flag.ResourceType),
				triageTableEscape(triageFlagLabel(flag)),
				triageTableEscape(flag.Severity),
				triageTableEscape(flag.Reason),
			))
		}
		sb.WriteString("\n")
	}

	healthyGuests := triage.Summary.TotalGuests - triageUniqueFlaggedByTypes(triage.Flags, "vm", "container")
	if healthyGuests < 0 {
		healthyGuests = 0
	}
	healthyNodes := triage.Summary.TotalNodes - triageUniqueFlaggedByType(triage.Flags, "node")
	if healthyNodes < 0 {
		healthyNodes = 0
	}
	healthyStorage := triage.Summary.TotalStorage - triageUniqueFlaggedByType(triage.Flags, "storage")
	if healthyStorage < 0 {
		healthyStorage = 0
	}

	sb.WriteString(fmt.Sprintf("## Healthy Resources (%d)\n", healthyNodes+healthyGuests+healthyStorage))
	sb.WriteString(fmt.Sprintf("Nodes: %d healthy\n", healthyNodes))
	sb.WriteString(fmt.Sprintf("Guests: %d running, %d stopped\n", triage.Summary.RunningGuests, triage.Summary.StoppedGuests))
	sb.WriteString(fmt.Sprintf("Storage: %d pools monitored\n", triage.Summary.TotalStorage))
	sb.WriteString(fmt.Sprintf("Docker: %d hosts\n", triage.Summary.TotalDocker))
	sb.WriteString(fmt.Sprintf("PBS: %d instances\n", triage.Summary.TotalPBS))
	sb.WriteString(fmt.Sprintf("PMG: %d instances\n", triage.Summary.TotalPMG))

	return sb.String()
}

func triageBuildSummary(state models.StateSnapshot, flaggedIDs map[string]bool) TriageSummary {
	summary := TriageSummary{
		TotalNodes:   len(state.Nodes),
		TotalStorage: len(state.Storage),
		TotalDocker:  len(state.DockerHosts),
		TotalPBS:     len(state.PBSInstances),
		TotalPMG:     len(state.PMGInstances),
		FlaggedCount: len(flaggedIDs),
	}

	for _, vm := range state.VMs {
		if vm.Template {
			continue
		}
		summary.TotalGuests++
		if strings.EqualFold(vm.Status, "running") {
			summary.RunningGuests++
		} else {
			summary.StoppedGuests++
		}
	}

	for _, ct := range state.Containers {
		if ct.Template {
			continue
		}
		summary.TotalGuests++
		if strings.EqualFold(ct.Status, "running") {
			summary.RunningGuests++
		} else {
			summary.StoppedGuests++
		}
	}

	return summary
}

func triageBackupFlag(resourceID, resourceName, resourceType string, lastBackup, now time.Time) []TriageFlag {
	if lastBackup.IsZero() {
		return []TriageFlag{{
			ResourceID:   resourceID,
			ResourceName: resourceName,
			ResourceType: resourceType,
			Category:     "backup",
			Severity:     "warning",
			Reason:       "Never backed up",
		}}
	}

	age := now.Sub(lastBackup)
	if age <= 48*time.Hour {
		return nil
	}

	hours := int(math.Round(age.Hours()))
	return []TriageFlag{{
		ResourceID:   resourceID,
		ResourceName: resourceName,
		ResourceType: resourceType,
		Category:     "backup",
		Severity:     "warning",
		Reason:       fmt.Sprintf("Last backup %dh ago (threshold: 48h)", hours),
		Value:        age.Hours(),
		Threshold:    48,
	}}
}

func triageMapAnomalySeverity(anomaly baseline.AnomalyReport) (string, bool) {
	switch anomaly.Severity {
	case baseline.AnomalyCritical, baseline.AnomalyHigh:
		return "warning", true
	case baseline.AnomalyMedium:
		return "watch", true
	default:
		if math.Abs(anomaly.ZScore) >= 2.5 {
			return "watch", true
		}
		return "", false
	}
}

func triageAlertCategory(alertType string) string {
	typ := strings.ToLower(strings.TrimSpace(alertType))
	switch {
	case strings.Contains(typ, "cpu"), strings.Contains(typ, "memory"):
		return "performance"
	case strings.Contains(typ, "disk"), strings.Contains(typ, "storage"):
		return "capacity"
	case strings.Contains(typ, "backup"):
		return "backup"
	default:
		return "health"
	}
}

func triageAlertMetric(alertType string) string {
	typ := strings.ToLower(strings.TrimSpace(alertType))
	switch {
	case strings.Contains(typ, "cpu"):
		return "cpu"
	case strings.Contains(typ, "memory"):
		return "memory"
	case strings.Contains(typ, "disk"):
		return "disk"
	case strings.Contains(typ, "storage"):
		return "usage"
	default:
		return ""
	}
}

func triageWarnWatchSeverity(value, warningThreshold, watchThreshold float64) (string, float64) {
	if value > warningThreshold {
		return "warning", warningThreshold
	}
	if value > watchThreshold {
		return "watch", watchThreshold
	}
	return "", 0
}

func triageCriticalWarnWatchSeverity(value, criticalThreshold, warningThreshold, watchThreshold float64) (string, float64) {
	if value > criticalThreshold {
		return "critical", criticalThreshold
	}
	if value > warningThreshold {
		return "warning", warningThreshold
	}
	if value > watchThreshold {
		return "watch", watchThreshold
	}
	return "", 0
}

func triageDedupKey(flag TriageFlag) string {
	id := flag.ResourceID
	if id == "" {
		id = flag.ResourceName
	}
	return id + "|" + flag.Category
}

func triageSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 3
	case "warning":
		return 2
	case "watch":
		return 1
	default:
		return 0
	}
}

func triageNormalizePercent(value float64, metric string) float64 {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "cpu", "memory", "disk", "usage":
		if value <= 1.0 {
			return value * 100
		}
	}
	return value
}

func triageMetricLabel(metric string) string {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "cpu":
		return "CPU"
	case "memory":
		return "Memory"
	case "disk":
		return "Disk"
	case "usage":
		return "Usage"
	default:
		if metric == "" {
			return "Metric"
		}
		return strings.ToUpper(metric)
	}
}

func triageResourceType(knownType, resourceID string) string {
	knownType = strings.TrimSpace(strings.ToLower(knownType))
	if knownType != "" {
		if knownType == "lxc" {
			return "container"
		}
		if knownType == "docker" {
			return "docker_host"
		}
		return knownType
	}
	return triageConnectionResourceType(resourceID)
}

func triageConnectionResourceType(resourceID string) string {
	id := strings.ToLower(strings.TrimSpace(resourceID))
	switch {
	case strings.Contains(id, "qemu/"):
		return "vm"
	case strings.Contains(id, "lxc/"):
		return "container"
	case strings.HasPrefix(id, "node/"), strings.HasPrefix(id, "pve"), strings.Contains(id, "/node/"):
		return "node"
	case strings.HasPrefix(id, "storage/"), strings.Contains(id, "storage"):
		return "storage"
	case strings.HasPrefix(id, "docker-"), strings.Contains(id, "docker"):
		return "docker_host"
	case strings.HasPrefix(id, "pbs-"), strings.HasPrefix(id, "pbs/"), strings.Contains(id, "pbs"):
		return "pbs"
	case strings.HasPrefix(id, "pmg-"), strings.HasPrefix(id, "pmg/"), strings.Contains(id, "pmg"):
		return "pmg"
	default:
		return "node"
	}
}

func triageResourceName(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	id := strings.TrimSpace(fallback)
	switch {
	case strings.HasPrefix(id, "node/"):
		return strings.TrimPrefix(id, "node/")
	case strings.HasPrefix(id, "pbs-"):
		return strings.TrimPrefix(id, "pbs-")
	case strings.HasPrefix(id, "pmg-"):
		return strings.TrimPrefix(id, "pmg-")
	case strings.HasPrefix(id, "docker-"):
		return strings.TrimPrefix(id, "docker-")
	case strings.HasPrefix(id, "host-"):
		return strings.TrimPrefix(id, "host-")
	default:
		return id
	}
}

func triageThresholdsUnset(thresholds PatrolThresholds) bool {
	return thresholds == (PatrolThresholds{})
}

func triageFlagLabel(flag TriageFlag) string {
	switch {
	case flag.Category == "performance" && flag.Metric == "cpu":
		return "High CPU"
	case flag.Category == "performance" && flag.Metric == "memory":
		return "High Memory"
	case flag.Category == "capacity" && (flag.Metric == "disk" || flag.Metric == "usage"):
		return "High Disk Usage"
	case flag.Category == "backup":
		return "Backup Risk"
	case flag.Category == "connectivity":
		return "Connectivity"
	case flag.Category == "anomaly":
		return "Baseline Anomaly"
	case flag.Category == "health":
		return "Health Risk"
	default:
		return "Issue"
	}
}

func triageTableEscape(v string) string {
	v = strings.ReplaceAll(v, "|", "\\|")
	v = strings.ReplaceAll(v, "\n", " ")
	return strings.TrimSpace(v)
}

func triageUniqueFlaggedByType(flags []TriageFlag, resourceType string) int {
	unique := make(map[string]bool)
	for _, flag := range flags {
		if flag.ResourceType != resourceType {
			continue
		}
		id := flag.ResourceID
		if id == "" {
			id = flag.ResourceName
		}
		if id != "" {
			unique[id] = true
		}
	}
	return len(unique)
}

func triageUniqueFlaggedByTypes(flags []TriageFlag, resourceTypes ...string) int {
	allowed := make(map[string]bool, len(resourceTypes))
	for _, rt := range resourceTypes {
		allowed[rt] = true
	}
	unique := make(map[string]bool)
	for _, flag := range flags {
		if !allowed[flag.ResourceType] {
			continue
		}
		id := flag.ResourceID
		if id == "" {
			id = flag.ResourceName
		}
		if id != "" {
			unique[id] = true
		}
	}
	return len(unique)
}
