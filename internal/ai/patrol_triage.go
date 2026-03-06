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
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TriageFlag represents a single deterministic issue detected during triage.
type TriageFlag struct {
	ResourceID   string  // e.g., "node/pve1", "qemu/100"
	ResourceName string  // e.g., "pve1", "webserver-01"
	ResourceType string  // "node", "vm", "system-container", "storage", "docker-host", "pbs", "pmg"
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
	TotalNodes         int
	TotalGuests        int
	RunningGuests      int
	StoppedGuests      int
	TotalStoragePools  int
	TotalPhysicalDisks int
	TotalStorage       int
	TotalDocker        int
	TotalPBS           int
	TotalPMG           int
	FlaggedCount       int
}

func (p *PatrolService) runDeterministicTriageState(
	ctx context.Context,
	snap patrolRuntimeState,
	scope *PatrolScope,
	guestIntel map[string]*GuestIntelligence,
) *TriageResult {
	_ = ctx

	scopedSet := p.buildScopedSetForRuntime(scope, snap)
	intel := p.seedPrecomputeIntelligenceState(snap, scopedSet, time.Now())

	thresholds := p.GetThresholds()
	if triageThresholdsUnset(thresholds) {
		thresholds = DefaultPatrolThresholds()
	}

	flags := make([]TriageFlag, 0)
	flags = append(flags, triageThresholdChecksState(snap, scopedSet, thresholds)...)
	flags = append(flags, triageAnomalyChecks(intel)...)
	flags = append(flags, triageBackupChecksState(snap, scopedSet)...)
	flags = append(flags, triageDiskHealthChecksState(snap, scopedSet)...)
	flags = append(flags, triageAlertChecksState(snap, scopedSet)...)
	flags = append(flags, triageConnectivityChecksState(snap, guestIntel, scopedSet)...)
	flags = append(flags, triageRecentChanges(intel)...)
	flags = deduplicateTriageFlags(flags)

	flaggedIDs := make(map[string]bool, len(flags))
	for _, flag := range flags {
		if flag.ResourceID != "" {
			flaggedIDs[flag.ResourceID] = true
		}
	}

	summary := triageBuildSummaryState(snap, flaggedIDs)

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

func triageThresholdChecksState(snap patrolRuntimeState, scopedSet map[string]bool, thresholds PatrolThresholds) []TriageFlag {
	flags := make([]TriageFlag, 0)
	for _, n := range patrolNodeInventoryRows(snap, scopedSet) {
		if sev, threshold := triageWarnWatchSeverity(n.cpu, thresholds.NodeCPUWarning, thresholds.NodeCPUWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   n.id,
				ResourceName: n.name,
				ResourceType: "node",
				Category:     "performance",
				Severity:     sev,
				Reason:       fmt.Sprintf("CPU at %.0f%% (threshold: %.0f%%)", n.cpu, threshold),
				Metric:       "cpu",
				Value:        n.cpu,
				Threshold:    threshold,
			})
		}
		if sev, threshold := triageWarnWatchSeverity(n.mem, thresholds.NodeMemWarning, thresholds.NodeMemWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   n.id,
				ResourceName: n.name,
				ResourceType: "node",
				Category:     "performance",
				Severity:     sev,
				Reason:       fmt.Sprintf("Memory at %.0f%% (threshold: %.0f%%)", n.mem, threshold),
				Metric:       "memory",
				Value:        n.mem,
				Threshold:    threshold,
			})
		}
	}

	for _, guest := range patrolGuestInventoryRows(snap, scopedSet, nil) {
		if !strings.EqualFold(guest.status, "running") {
			continue
		}
		resourceType := "system-container"
		if guest.gType == "VM" {
			resourceType = "vm"
		}
		if sev, threshold := triageWarnWatchSeverity(guest.mem, thresholds.GuestMemWarning, thresholds.GuestMemWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   guest.id,
				ResourceName: guest.name,
				ResourceType: resourceType,
				Category:     "performance",
				Severity:     sev,
				Reason:       fmt.Sprintf("Memory at %.0f%% (threshold: %.0f%%)", guest.mem, threshold),
				Metric:       "memory",
				Value:        guest.mem,
				Threshold:    threshold,
			})
		}
		if sev, threshold := triageCriticalWarnWatchSeverity(guest.disk, thresholds.GuestDiskCrit, thresholds.GuestDiskWarn, thresholds.GuestDiskWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   guest.id,
				ResourceName: guest.name,
				ResourceType: resourceType,
				Category:     "capacity",
				Severity:     sev,
				Reason:       fmt.Sprintf("Disk at %.0f%% (threshold: %.0f%%)", guest.disk, threshold),
				Metric:       "disk",
				Value:        guest.disk,
				Threshold:    threshold,
			})
		}
		if guest.cpu > thresholds.NodeCPUWarning {
			flags = append(flags, TriageFlag{
				ResourceID:   guest.id,
				ResourceName: guest.name,
				ResourceType: resourceType,
				Category:     "performance",
				Severity:     "warning",
				Reason:       fmt.Sprintf("CPU at %.0f%% (threshold: %.0f%%)", guest.cpu, thresholds.NodeCPUWarning),
				Metric:       "cpu",
				Value:        guest.cpu,
				Threshold:    thresholds.NodeCPUWarning,
			})
		}
	}

	for _, storage := range patrolStoragePoolRows(snap, scopedSet) {
		if sev, threshold := triageCriticalWarnWatchSeverity(storage.usage, thresholds.StorageCritical, thresholds.StorageWarning, thresholds.StorageWatch); sev != "" {
			flags = append(flags, TriageFlag{
				ResourceID:   storage.id,
				ResourceName: storage.name,
				ResourceType: "storage",
				Category:     "capacity",
				Severity:     sev,
				Reason:       fmt.Sprintf("Usage at %.0f%% (threshold: %.0f%%)", storage.usage, threshold),
				Metric:       "usage",
				Value:        storage.usage,
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

func triageBackupChecksState(snap patrolRuntimeState, scopedSet map[string]bool) []TriageFlag {
	flags := make([]TriageFlag, 0)
	now := time.Now()
	for _, guest := range patrolGuestInventoryRows(snap, scopedSet, nil) {
		if !strings.EqualFold(guest.status, "running") {
			continue
		}
		resourceType := "system-container"
		if guest.gType == "VM" {
			resourceType = "vm"
		}
		flags = append(flags, triageBackupFlag(guest.id, guest.name, resourceType, guest.lastBackup, now)...)
	}

	return flags
}

func triageDiskHealthChecksState(snap patrolRuntimeState, scopedSet map[string]bool) []TriageFlag {
	flags := make([]TriageFlag, 0)

	if snap.unifiedResourceProvider != nil {
		for _, disk := range snap.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypePhysicalDisk) {
			if disk.PhysicalDisk == nil {
				continue
			}
			resourceID, resourceName, health, wearout, temperature, ok := patrolUnifiedPhysicalDiskState(disk, scopedSet)
			if !ok {
				continue
			}
			flags = append(flags, triagePhysicalDiskFlags(resourceID, resourceName, health, wearout, temperature)...)
		}
		return flags
	}

	for _, disk := range snap.PhysicalDisks {
		resourceID, resourceName, health, wearout, temperature, ok := patrolSnapshotPhysicalDiskState(disk, scopedSet)
		if !ok {
			continue
		}
		flags = append(flags, triagePhysicalDiskFlags(resourceID, resourceName, health, wearout, temperature)...)
	}

	return flags
}

func patrolUnifiedPhysicalDiskState(disk unifiedresources.Resource, scopedSet map[string]bool) (resourceID, resourceName, health string, wearout, temperature int, ok bool) {
	if disk.PhysicalDisk == nil {
		return "", "", "", 0, 0, false
	}
	d := disk.PhysicalDisk
	resourceID = strings.TrimSpace(disk.ID)
	if resourceID == "" {
		resourceID = strings.TrimSpace(d.DevPath)
	}
	if !seedIsInScope(scopedSet, resourceID) {
		return "", "", "", 0, 0, false
	}
	resourceName = strings.TrimSpace(d.DevPath)
	if resourceName == "" {
		resourceName = strings.TrimSpace(disk.Name)
	}
	if resourceName == "" {
		resourceName = strings.TrimSpace(d.Model)
	}
	if resourceName == "" {
		resourceName = resourceID
	}
	return resourceID, resourceName, strings.TrimSpace(d.Health), d.Wearout, d.Temperature, true
}

func patrolSnapshotPhysicalDiskState(disk models.PhysicalDisk, scopedSet map[string]bool) (resourceID, resourceName, health string, wearout, temperature int, ok bool) {
	resourceID = strings.TrimSpace(disk.ID)
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.DevPath)
	}
	if !seedIsInScope(scopedSet, resourceID) {
		return "", "", "", 0, 0, false
	}
	resourceName = strings.TrimSpace(disk.DevPath)
	if resourceName == "" {
		resourceName = strings.TrimSpace(disk.Model)
	}
	if resourceName == "" {
		resourceName = resourceID
	}
	return resourceID, resourceName, strings.TrimSpace(disk.Health), disk.Wearout, disk.Temperature, true
}

func triagePhysicalDiskFlags(resourceID, resourceName, health string, wearout, temperature int) []TriageFlag {
	flags := make([]TriageFlag, 0, 3)
	if health != "" && !strings.EqualFold(health, "PASSED") {
		flags = append(flags, TriageFlag{
			ResourceID:   resourceID,
			ResourceName: resourceName,
			ResourceType: "physical_disk",
			Category:     "health",
			Severity:     "critical",
			Reason:       fmt.Sprintf("Disk health reported %s", health),
			Metric:       "disk",
		})
	}
	if wearout >= 0 && wearout < 20 {
		flags = append(flags, TriageFlag{
			ResourceID:   resourceID,
			ResourceName: resourceName,
			ResourceType: "physical_disk",
			Category:     "health",
			Severity:     "warning",
			Reason:       fmt.Sprintf("SSD wearout at %d%% remaining", wearout),
			Metric:       "disk",
			Value:        float64(wearout),
			Threshold:    20,
		})
	}
	if temperature > 55 {
		flags = append(flags, TriageFlag{
			ResourceID:   resourceID,
			ResourceName: resourceName,
			ResourceType: "physical_disk",
			Category:     "health",
			Severity:     "warning",
			Reason:       fmt.Sprintf("Disk temperature %d°C", temperature),
			Metric:       "disk",
			Value:        float64(temperature),
			Threshold:    55,
		})
	}
	return flags
}

func triageAlertChecksState(snap patrolRuntimeState, scopedSet map[string]bool) []TriageFlag {
	flags := make([]TriageFlag, 0, len(snap.ActiveAlerts))

	for _, alert := range snap.ActiveAlerts {
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

func triageConnectivityChecksState(snap patrolRuntimeState, guestIntel map[string]*GuestIntelligence, scopedSet map[string]bool) []TriageFlag {
	flags := make([]TriageFlag, 0)

	for resourceID, healthy := range snap.ConnectionHealth {
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

		flags = append(flags, TriageFlag{
			ResourceID:   guestID,
			ResourceName: triageResourceName(intel.Name, guestID),
			ResourceType: triageGuestResourceType(intel.GuestType, guestID),
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
		"Scanned %d resources: %d nodes, %d guests, %d storage resources (%d pools, %d physical disks), %d docker hosts, %d PBS, %d PMG.\n\n",
		scanned,
		triage.Summary.TotalNodes,
		triage.Summary.TotalGuests,
		triage.Summary.TotalStorage,
		triage.Summary.TotalStoragePools,
		triage.Summary.TotalPhysicalDisks,
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

	healthyGuests := triage.Summary.TotalGuests - triageUniqueFlaggedByTypes(triage.Flags, "vm", "system-container")
	if healthyGuests < 0 {
		healthyGuests = 0
	}
	healthyNodes := triage.Summary.TotalNodes - triageUniqueFlaggedByType(triage.Flags, "node")
	if healthyNodes < 0 {
		healthyNodes = 0
	}
	healthyStorage := triage.Summary.TotalStorage - triageUniqueFlaggedByTypes(triage.Flags, "storage", "physical_disk")
	if healthyStorage < 0 {
		healthyStorage = 0
	}

	healthyDocker := triage.Summary.TotalDocker - triageUniqueFlaggedByType(triage.Flags, "docker-host")
	if healthyDocker < 0 {
		healthyDocker = 0
	}
	healthyPBS := triage.Summary.TotalPBS - triageUniqueFlaggedByType(triage.Flags, "pbs")
	if healthyPBS < 0 {
		healthyPBS = 0
	}
	healthyPMG := triage.Summary.TotalPMG - triageUniqueFlaggedByType(triage.Flags, "pmg")
	if healthyPMG < 0 {
		healthyPMG = 0
	}

	totalHealthy := healthyNodes + healthyGuests + healthyStorage + healthyDocker + healthyPBS + healthyPMG
	sb.WriteString(fmt.Sprintf("## Healthy Resources (%d)\n", totalHealthy))
	sb.WriteString(fmt.Sprintf("Nodes: %d healthy\n", healthyNodes))
	sb.WriteString(fmt.Sprintf("Guests: %d running, %d stopped\n", triage.Summary.RunningGuests, triage.Summary.StoppedGuests))
	sb.WriteString(fmt.Sprintf("Storage: %d resources monitored (%d pools, %d physical disks)\n",
		healthyStorage,
		triage.Summary.TotalStoragePools,
		triage.Summary.TotalPhysicalDisks))
	sb.WriteString(fmt.Sprintf("Docker: %d hosts\n", healthyDocker))
	sb.WriteString(fmt.Sprintf("PBS: %d instances\n", healthyPBS))
	sb.WriteString(fmt.Sprintf("PMG: %d instances\n", healthyPMG))

	return sb.String()
}

func triageBuildSummaryState(snap patrolRuntimeState, flaggedIDs map[string]bool) TriageSummary {
	counts := patrolRuntimeCountResources(snap)
	storagePools := patrolStoragePoolRows(snap, nil)
	physicalDisks := patrolPhysicalDiskRows(snap, nil)
	guests := patrolGuestInventoryRows(snap, nil, nil)

	summary := TriageSummary{
		TotalNodes:         len(patrolNodeInventoryRows(snap, nil)),
		TotalStoragePools:  len(storagePools),
		TotalPhysicalDisks: len(physicalDisks),
		TotalStorage:       len(storagePools) + len(physicalDisks),
		TotalDocker:        counts.docker,
		TotalPBS:           counts.pbs,
		TotalPMG:           counts.pmg,
		FlaggedCount:       len(flaggedIDs),
	}

	for _, guest := range guests {
		summary.TotalGuests++
		if strings.EqualFold(guest.status, "running") {
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
	return id + "|" + flag.Category + "|" + flag.Metric
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
	if normalized := triageCanonicalResourceType(knownType); normalized != "" {
		return normalized
	}
	return triageConnectionResourceType(resourceID)
}

func triageGuestResourceType(knownType, resourceID string) string {
	canonicalType := triageCanonicalResourceType(knownType)
	switch canonicalType {
	case "vm", "system-container":
		return canonicalType
	}

	derivedType := triageConnectionResourceType(resourceID)
	switch derivedType {
	case "vm", "system-container":
		return derivedType
	default:
		return "vm"
	}
}

func triageCanonicalResourceType(resourceType string) string {
	switch normalized := strings.ToLower(strings.TrimSpace(resourceType)); normalized {
	case "vm", "system-container", "app-container", "node", "storage", "docker-host", "pbs", "pmg", "agent", "physical_disk", "physical-disk":
		if normalized == "physical-disk" {
			return "physical_disk"
		}
		return normalized
	default:
		return ""
	}
}

func triageConnectionResourceType(resourceID string) string {
	id := strings.ToLower(strings.TrimSpace(resourceID))
	switch {
	case strings.HasPrefix(id, "vm:"), strings.HasPrefix(id, "vm/"), strings.Contains(id, "qemu/"):
		return "vm"
	case strings.HasPrefix(id, "system-container:"), strings.HasPrefix(id, "system-container/"), strings.HasPrefix(id, "oci-container:"), strings.HasPrefix(id, "oci-container/"), strings.Contains(id, "lxc/"):
		return "system-container"
	case strings.HasPrefix(id, "node/"), strings.HasPrefix(id, "node:"), strings.HasPrefix(id, "pve"), strings.Contains(id, "/node/"):
		return "node"
	case strings.HasPrefix(id, "storage/"), strings.Contains(id, "storage"):
		return "storage"
	case strings.HasPrefix(id, "docker-host:"), strings.HasPrefix(id, "docker-host/"), strings.HasPrefix(id, "app-container:"), strings.HasPrefix(id, "app-container/"), strings.HasPrefix(id, "docker-"), strings.Contains(id, "docker"):
		return "docker-host"
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
	case strings.HasPrefix(id, "agent-"):
		return strings.TrimPrefix(id, "agent-")
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
