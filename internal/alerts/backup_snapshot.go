package alerts

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/proxmoxidentity"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

func backupIgnoreVMID(vmID string, ignoreList []string) bool {
	if vmID == "" || len(ignoreList) == 0 {
		return false
	}
	for _, entry := range ignoreList {
		value := strings.TrimSpace(entry)
		if value == "" {
			continue
		}
		if strings.HasSuffix(value, "*") {
			prefix := strings.TrimSuffix(value, "*")
			if prefix != "" && strings.HasPrefix(vmID, prefix) {
				return true
			}
			continue
		}
		if vmID == value {
			return true
		}
	}
	return false
}

func (m *Manager) resolvedSnapshotAlertConfigNoLock(thresholds ThresholdConfig) SnapshotAlertConfig {
	cfg := m.config.SnapshotDefaults
	if thresholds.Snapshot != nil {
		cfg = *thresholds.Snapshot
	}
	return cfg
}

func (m *Manager) resolvedBackupAlertConfigNoLock(thresholds ThresholdConfig) BackupAlertConfig {
	cfg := m.config.BackupDefaults
	if thresholds.Backup != nil {
		cfg = *thresholds.Backup
	}
	if cfg.AlertOrphaned == nil {
		alertOrphaned := true
		cfg.AlertOrphaned = &alertOrphaned
	}
	return cfg
}

func snapshotAlertStillTriggered(alert *Alert, cfg SnapshotAlertConfig) bool {
	if alert == nil || !cfg.Enabled {
		return false
	}

	ageValue, _ := metadataFloatValue(alert.Metadata, "snapshotAgeDays")
	sizeValue, _ := metadataFloatValue(alert.Metadata, "snapshotSizeGiB")

	if cfg.CriticalDays > 0 && ageValue >= float64(cfg.CriticalDays) {
		return true
	}
	if cfg.WarningDays > 0 && ageValue >= float64(cfg.WarningDays) {
		return true
	}
	if cfg.CriticalSizeGiB > 0 && sizeValue >= cfg.CriticalSizeGiB {
		return true
	}
	if cfg.WarningSizeGiB > 0 && sizeValue >= cfg.WarningSizeGiB {
		return true
	}

	return false
}

func backupAlertStillTriggered(alert *Alert, cfg BackupAlertConfig) bool {
	if alert == nil || !cfg.Enabled {
		return false
	}

	vmid := metadataStringValue(alert.Metadata, "guestVmid")
	if vmid == "" {
		if parsed := metadataIntValue(alert.Metadata["guestVmid"]); parsed > 0 {
			vmid = strconv.Itoa(parsed)
		}
	}
	if backupIgnoreVMID(vmid, cfg.IgnoreVMIDs) {
		return false
	}
	if metadataBoolValue(alert.Metadata, "orphaned") && cfg.AlertOrphaned != nil && !*cfg.AlertOrphaned {
		return false
	}

	ageValue, ok := metadataFloatValue(alert.Metadata, "ageDays")
	if !ok {
		ageValue = alert.Value
	}

	if cfg.CriticalDays > 0 && ageValue >= float64(cfg.CriticalDays) {
		return true
	}
	if cfg.WarningDays > 0 && ageValue >= float64(cfg.WarningDays) {
		return true
	}

	return false
}

// BuildGuestKey constructs a unique key for a guest from instance, node, and VMID.
// Uses the canonical format: instance:node:vmid
// This matches the format used by makeGuestID in the monitoring package.
func BuildGuestKey(instance, node string, vmID int) string {
	instance = strings.TrimSpace(instance)
	node = strings.TrimSpace(node)
	if instance == "" {
		instance = node
	}
	return fmt.Sprintf("%s:%s:%d", instance, node, vmID)
}

type backupRecord struct {
	key          string
	vmID         string
	lookup       GuestLookup
	fallbackName string
	instance     string
	node         string
	subjectType  string
	source       string
	rollupID     string
	providers    []recovery.Provider
	lastTime     time.Time
}

// BackupInventoryScope carries monitoring-owned inventory readiness into backup
// alert evaluation. It keeps orphan detection from racing ahead of Proxmox
// guest/template discovery while preserving the direct CheckBackups API for
// unit tests and non-monitoring callers.
type BackupInventoryScope struct {
	PVEOrphanInventoryReady map[string]map[string]bool
	PVETemplateSubjects     map[string]struct{}
}

func BuildBackupPVETemplateSubjectKey(instance, guestType, node string, vmid int) string {
	instance = strings.TrimSpace(instance)
	guestType = normalizeBackupGuestType(guestType)
	node = strings.TrimSpace(node)
	if instance == "" || guestType == "" || node == "" || vmid <= 0 {
		return ""
	}
	return strings.Join([]string{instance, guestType, node, strconv.Itoa(vmid)}, "\x00")
}

func normalizeBackupGuestType(guestType string) string {
	switch strings.ToLower(strings.TrimSpace(guestType)) {
	case "qemu", "vm", "proxmox-vm":
		return "qemu"
	case "lxc", "ct", "container", "system-container", "proxmox-lxc":
		return "lxc"
	default:
		return strings.ToLower(strings.TrimSpace(guestType))
	}
}

func backupOrphanInventoryReady(scope *BackupInventoryScope, record backupRecord) bool {
	if scope == nil || scope.PVEOrphanInventoryReady == nil {
		return true
	}
	if record.source != "PVE" {
		return true
	}
	instance := strings.TrimSpace(record.instance)
	guestType := normalizeBackupGuestType(record.subjectType)
	if instance == "" || guestType == "" {
		return false
	}
	return scope.PVEOrphanInventoryReady[instance][guestType]
}

func backupMatchesKnownPVETemplate(scope *BackupInventoryScope, record backupRecord) bool {
	if scope == nil || len(scope.PVETemplateSubjects) == 0 || record.source != "PVE" {
		return false
	}
	vmid, err := strconv.Atoi(strings.TrimSpace(record.vmID))
	if err != nil || vmid <= 0 {
		return false
	}
	key := BuildBackupPVETemplateSubjectKey(record.instance, record.subjectType, record.node, vmid)
	if key == "" {
		return false
	}
	_, exists := scope.PVETemplateSubjects[key]
	return exists
}

func canonicalGuestResourceType(guestType string) unifiedresources.ResourceType {
	switch strings.ToLower(strings.TrimSpace(guestType)) {
	case "lxc":
		return unifiedresources.ResourceTypeSystemContainer
	default:
		return unifiedresources.ResourceTypeVM
	}
}

func canonicalBackupSubjectResourceType(record backupRecord) unifiedresources.ResourceType {
	if record.lookup.Type != "" {
		return canonicalGuestResourceType(record.lookup.Type)
	}
	switch normalizeBackupGuestType(record.subjectType) {
	case "lxc":
		return unifiedresources.ResourceTypeSystemContainer
	case "qemu":
		return unifiedresources.ResourceTypeVM
	}
	if strings.TrimSpace(record.vmID) != "" {
		return unifiedresources.ResourceTypeVM
	}
	return unifiedresources.ResourceType("backup-subject")
}

func canonicalBackupSubjectResourceID(alertKey string, record backupRecord) string {
	if record.instance != "" && record.node != "" && record.vmID != "" {
		if vmid, err := strconv.Atoi(record.vmID); err == nil && vmid > 0 {
			return BuildGuestKey(record.instance, record.node, vmid)
		}
	}
	return "backup-subject:" + sanitizeAlertKey(alertKey)
}

// CheckSnapshotsForInstance evaluates guest snapshots for age-based alerts.
func (m *Manager) CheckSnapshotsForInstance(instanceName string, snapshots []models.GuestSnapshot, guestNames map[string]string) {
	m.mu.RLock()
	enabled := m.config.Enabled
	snapshotCfg := m.config.SnapshotDefaults
	m.mu.RUnlock()

	if !enabled {
		return
	}

	if !snapshotCfg.Enabled {
		m.clearSnapshotAlertsForInstance(instanceName)
		return
	}

	now := time.Now()
	validAlerts := make(map[string]struct{})

	for _, snapshot := range snapshots {
		if instanceName != "" && snapshot.Instance != "" && snapshot.Instance != instanceName {
			continue
		}
		if snapshot.Time.IsZero() {
			continue
		}

		ageHours := now.Sub(snapshot.Time).Hours()
		if ageHours < 0 {
			continue
		}
		ageDays := ageHours / 24

		const gib = 1024.0 * 1024 * 1024
		sizeGiB := 0.0
		if snapshot.SizeBytes > 0 {
			sizeGiB = float64(snapshot.SizeBytes) / gib
		}

		// Determine thresholds for this snapshot
		resourceID := fmt.Sprintf("%s:%s:%d", snapshot.Instance, snapshot.Node, snapshot.VMID)
		guestName := strings.TrimSpace(guestNames[BuildGuestKey(snapshot.Instance, snapshot.Node, snapshot.VMID)])
		guestContext := guestSnapshotFromIdentity(resourceID, guestName, snapshot.Node, snapshot.Instance, snapshot.Type, "")
		m.mu.RLock()
		gh := m.getGuestThresholds(guestContext, resourceID)
		m.mu.RUnlock()

		if gh.Disabled {
			continue
		}

		currentSnapshotCfg := snapshotCfg
		if gh.Snapshot != nil {
			currentSnapshotCfg = *gh.Snapshot
		}

		if !currentSnapshotCfg.Enabled {
			continue
		}

		var ageLevel AlertLevel
		var ageThreshold int
		var sizeLevel AlertLevel
		var sizeThreshold float64
		var triggeredStats []string

		if currentSnapshotCfg.CriticalDays > 0 && ageDays >= float64(currentSnapshotCfg.CriticalDays) {
			ageLevel = AlertLevelCritical
			ageThreshold = currentSnapshotCfg.CriticalDays
			triggeredStats = append(triggeredStats, "age")
		} else if currentSnapshotCfg.WarningDays > 0 && ageDays >= float64(currentSnapshotCfg.WarningDays) {
			ageLevel = AlertLevelWarning
			ageThreshold = currentSnapshotCfg.WarningDays
			triggeredStats = append(triggeredStats, "age")
		}

		if snapshot.SizeBytes > 0 {
			if currentSnapshotCfg.CriticalSizeGiB > 0 && sizeGiB >= currentSnapshotCfg.CriticalSizeGiB {
				sizeLevel = AlertLevelCritical
				sizeThreshold = currentSnapshotCfg.CriticalSizeGiB
				triggeredStats = append(triggeredStats, "size")
			} else if currentSnapshotCfg.WarningSizeGiB > 0 && sizeGiB >= currentSnapshotCfg.WarningSizeGiB {
				sizeLevel = AlertLevelWarning
				sizeThreshold = currentSnapshotCfg.WarningSizeGiB
				triggeredStats = append(triggeredStats, "size")
			}
		}

		if ageLevel == "" && sizeLevel == "" {
			continue
		}

		useSizePrimary := false
		if sizeLevel == AlertLevelCritical && ageLevel != AlertLevelCritical {
			useSizePrimary = true
		} else if sizeLevel != "" && ageLevel == "" {
			useSizePrimary = true
		}

		alertID := fmt.Sprintf("snapshot-age-%s", snapshot.ID)

		guestKey := BuildGuestKey(snapshot.Instance, snapshot.Node, snapshot.VMID)

		guestType := "VM"
		if strings.EqualFold(snapshot.Type, "lxc") {
			guestType = "Container"
		}

		if guestName == "" {
			switch guestType {
			case "Container":
				guestName = fmt.Sprintf("CT %d", snapshot.VMID)
			default:
				guestName = fmt.Sprintf("VM %d", snapshot.VMID)
			}
		}

		snapshotName := strings.TrimSpace(snapshot.Name)
		if snapshotName == "" {
			snapshotName = "(unnamed)"
		}

		ageDaysRounded := math.Round(ageDays*10) / 10
		sizeGiBRounded := math.Round(sizeGiB*10) / 10
		reasons := make([]string, 0, 2)
		if ageLevel != "" {
			reasons = append(reasons, fmt.Sprintf("%.1f days old (threshold %d days)", ageDaysRounded, ageThreshold))
		}
		if sizeLevel != "" {
			reasons = append(reasons, fmt.Sprintf("%.1f GiB (threshold %.1f GiB)", sizeGiBRounded, sizeThreshold))
		}
		reasonText := strings.Join(reasons, " and ")
		message := fmt.Sprintf(
			"%s snapshot '%s' for %s is %s on %s",
			guestType,
			snapshotName,
			guestName,
			reasonText,
			snapshot.Node,
		)

		alertValue := ageDays
		alertThreshold := float64(ageThreshold)
		thresholdTime := now
		if useSizePrimary {
			alertValue = sizeGiB
			alertThreshold = sizeThreshold
		} else if ageThreshold > 0 {
			thresholdTime = snapshot.Time.Add(time.Duration(ageThreshold) * 24 * time.Hour)
			if thresholdTime.After(now) {
				thresholdTime = now
			}
		}

		metadata := map[string]interface{}{
			"snapshotName":      snapshot.Name,
			"snapshotCreatedAt": snapshot.Time,
			"snapshotAgeDays":   ageDays,
			"snapshotAgeHours":  ageHours,
			"snapshotSizeBytes": snapshot.SizeBytes,
			"snapshotSizeGiB":   sizeGiB,
			"guestName":         guestName,
			"guestType":         guestType,
			"guestInstance":     snapshot.Instance,
			"guestNode":         snapshot.Node,
			"guestVmid":         snapshot.VMID,
			"triggeredMetrics":  triggeredStats,
			"primaryMetric":     "age",
		}
		if useSizePrimary {
			metadata["primaryMetric"] = "size"
		}
		if ageLevel != "" {
			metadata["thresholdDays"] = ageThreshold
		}
		if sizeLevel != "" {
			metadata["thresholdSizeGiB"] = sizeThreshold
		}

		resourceName := fmt.Sprintf("%s snapshot '%s'", guestName, snapshotName)
		guestResourceType := canonicalGuestResourceType(snapshot.Type)
		guestResourceID := guestKey
		sizeMetric := ""
		var sizeValue *float64
		if currentSnapshotCfg.WarningSizeGiB > 0 || currentSnapshotCfg.CriticalSizeGiB > 0 {
			sizeMetric = "snapshot-size-gib"
			sizeValue = &sizeGiB
		}
		ageMetric := ""
		if currentSnapshotCfg.WarningDays > 0 || currentSnapshotCfg.CriticalDays > 0 {
			ageMetric = "snapshot-age-days"
		}

		spec, err := buildCanonicalPostureThresholdSpec(
			guestResourceID+"/snapshot:"+snapshot.ID,
			guestResourceID,
			resourceName,
			guestResourceType,
			ageMetric,
			float64(currentSnapshotCfg.WarningDays),
			float64(currentSnapshotCfg.CriticalDays),
			sizeMetric,
			currentSnapshotCfg.WarningSizeGiB,
			currentSnapshotCfg.CriticalSizeGiB,
			false,
		)
		if err != nil {
			log.Warn().
				Err(err).
				Str("snapshotID", snapshot.ID).
				Str("resourceID", guestResourceID).
				Msg("Skipping invalid canonical snapshot posture spec")
			continue
		}
		validAlerts[canonicalTrackingKeyForSpec(spec, alertID)] = struct{}{}

		result, _ := m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
			Spec: spec,
			Evidence: alertspecs.AlertEvidence{
				ObservedAt: now,
				PostureThreshold: &alertspecs.PostureThresholdEvidence{
					AgeMetric:  ageMetric,
					AgeValue:   ageDays,
					SizeMetric: sizeMetric,
					SizeValue:  sizeValue,
				},
			},
			AlertID:           alertID,
			AlertType:         "snapshot-age",
			ResourceID:        spec.ResourceID,
			ResourceName:      resourceName,
			Node:              snapshot.Node,
			Instance:          snapshot.Instance,
			Value:             alertValue,
			Threshold:         alertThreshold,
			StartTimeOverride: thresholdTime,
			Metadata:          metadata,
			AddToRecent:       true,
			AddToHistory:      true,
			RateLimit:         true,
			DispatchAsync:     true,
			MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
				return message, alertValue, alertThreshold
			},
		})
		if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
			asyncSaveActiveAlerts("snapshot", m.SaveActiveAlerts)
		}
	}

	m.mu.Lock()
	for storageKey, alert := range m.activeAlerts {
		if alert == nil || alert.Type != "snapshot-age" {
			continue
		}
		if instanceName != "" && alert.Instance != instanceName {
			continue
		}
		if _, ok := validAlerts[storageKey]; ok {
			continue
		}
		m.clearAlertNoLock(storageKey)
	}
	m.mu.Unlock()
}

// CheckBackups evaluates storage, PBS, and PMG backups for age-based alerts.
func (m *Manager) CheckBackups(
	rollups []recovery.ProtectionRollup,
	guestsByKey map[string]GuestLookup,
	guestsByVMID map[string][]GuestLookup,
) {
	m.CheckBackupsWithInventory(rollups, guestsByKey, guestsByVMID, nil)
}

// CheckBackupsWithInventory evaluates backup rollups with optional monitoring
// inventory readiness for orphan detection.
func (m *Manager) CheckBackupsWithInventory(
	rollups []recovery.ProtectionRollup,
	guestsByKey map[string]GuestLookup,
	guestsByVMID map[string][]GuestLookup,
	inventoryScope *BackupInventoryScope,
) {
	m.mu.RLock()
	enabled := m.config.Enabled
	backupCfg := m.config.BackupDefaults
	m.mu.RUnlock()

	if backupCfg.AlertOrphaned == nil {
		alertOrphaned := true
		backupCfg.AlertOrphaned = &alertOrphaned
	}

	if !enabled || !backupCfg.Enabled {
		m.clearBackupAlerts()
		return
	}

	if backupCfg.WarningDays <= 0 && backupCfg.CriticalDays <= 0 {
		m.clearBackupAlerts()
		return
	}

	records := make(map[string]*backupRecord)

	updateRecord := func(key string, candidate backupRecord) {
		if key == "" {
			return
		}
		if existing, ok := records[key]; ok {
			if candidate.lastTime.After(existing.lastTime) {
				*existing = candidate
			}
			return
		}
		record := candidate
		records[key] = &record
	}

	now := time.Now()

	for _, rollup := range rollups {
		if rollup.LastSuccessAt == nil || rollup.LastSuccessAt.IsZero() {
			continue
		}

		lastTime := rollup.LastSuccessAt.UTC()
		providers := append([]recovery.Provider(nil), rollup.Providers...)

		source := "Recovery"
		if slicesContainsProvider(providers, recovery.ProviderProxmoxPMG) {
			source = "PMG"
		} else if slicesContainsProvider(providers, recovery.ProviderProxmoxPBS) {
			source = "PBS"
		} else if slicesContainsProvider(providers, recovery.ProviderProxmoxPVE) {
			source = "PVE"
		}

		var (
			info        GuestLookup
			key         string
			displayName string
			instance    string
			node        string
			vmID        string
			subjectType string
		)

		ref := rollup.SubjectRef
		if ref != nil {
			subjectType = normalizeBackupGuestType(ref.Type)
		}

		// Primary: subjectRef.ID is the canonical proxmox guest source ID (instance:node:vmid) when linked.
		if ref != nil && strings.TrimSpace(ref.ID) != "" {
			if inst, nd, vmid, ok := parseGuestID(ref.ID); ok {
				key = BuildGuestKey(inst, nd, vmid)
				info = guestsByKey[key]
				instance = inst
				node = nd
				vmID = strconv.Itoa(vmid)
			}
		}

		// Secondary: attempt to map by VMID for orphaned/ambiguous backups.
		if key == "" && ref != nil {
			vmidStr := strings.TrimSpace(ref.ID)
			if vmidStr == "" {
				vmidStr = strings.TrimSpace(ref.Name)
			}
			if vmidStr != "" {
				if vmid, err := strconv.Atoi(vmidStr); err == nil && vmid > 0 {
					vmID = vmidStr
					guests := guestsByVMID[vmidStr]
					if len(guests) == 1 {
						info = guests[0]
					} else if len(guests) > 1 && strings.TrimSpace(ref.Namespace) != "" {
						bestScore := 0
						matchedMultiple := false
						for _, g := range guests {
							score := proxmoxidentity.BackupGuestMatchScore(
								ref.Namespace,
								ref.Name,
								vmidStr,
								g.Name,
								g.Instance,
								g.Node,
							)
							if score > bestScore {
								bestScore = score
								info = g
								matchedMultiple = false
							} else if score > 0 && score == bestScore {
								matchedMultiple = true
							}
						}
						if matchedMultiple {
							info = GuestLookup{}
						}
					}
					if info.Instance != "" && info.Node != "" {
						key = BuildGuestKey(info.Instance, info.Node, info.VMID)
						instance = info.Instance
						node = info.Node
					}
				}
			}
		}

		if key == "" {
			// Stable fallback for non-guest subjects and orphans.
			key = strings.TrimSpace(rollup.RollupID)
			if key == "" {
				continue
			}
		}

		displayName = strings.TrimSpace(info.Name)
		if displayName == "" && ref != nil {
			displayName = strings.TrimSpace(ref.Name)
		}
		if displayName == "" && vmID != "" {
			displayName = fmt.Sprintf("VMID %s", vmID)
		}
		if displayName == "" {
			displayName = "Unknown"
		}

		updateRecord(key, backupRecord{
			key:          key,
			vmID:         vmID,
			lookup:       info,
			fallbackName: displayName,
			instance:     instance,
			node:         node,
			subjectType:  subjectType,
			source:       source,
			rollupID:     strings.TrimSpace(rollup.RollupID),
			providers:    providers,
			lastTime:     lastTime,
		})
	}

	if len(records) == 0 {
		m.clearBackupAlerts()
		return
	}

	validAlerts := make(map[string]struct{})

	for key, record := range records {
		age := now.Sub(record.lastTime)
		if age < 0 {
			continue
		}

		ageDays := age.Hours() / 24
		if ageDays < 0 {
			continue
		}
		ageDaysRounded := math.Round(ageDays*10) / 10

		// Determine thresholds for this backup
		currentBackupCfg := backupCfg
		guestContext := guestSnapshotFromLookup(record.lookup, record.fallbackName)
		guestResourceID := strings.TrimSpace(record.lookup.ResourceID)
		if guestResourceID == "" {
			guestResourceID = guestContext.ID
		}
		if guestResourceID != "" {
			m.mu.RLock()
			gh := m.getGuestThresholds(guestContext, guestResourceID)
			m.mu.RUnlock()
			if gh.Disabled {
				continue
			}
			if gh.Backup != nil {
				currentBackupCfg = *gh.Backup
			}
		}

		currentBackupCfg.AlertOrphaned = backupCfg.AlertOrphaned
		currentBackupCfg.IgnoreVMIDs = backupCfg.IgnoreVMIDs

		if backupIgnoreVMID(record.vmID, currentBackupCfg.IgnoreVMIDs) {
			continue
		}
		if record.vmID != "" && record.lookup.ResourceID == "" {
			if backupMatchesKnownPVETemplate(inventoryScope, *record) {
				continue
			}
			if !backupOrphanInventoryReady(inventoryScope, *record) {
				continue
			}
			if currentBackupCfg.AlertOrphaned != nil && !*currentBackupCfg.AlertOrphaned {
				continue
			}
		}

		if !currentBackupCfg.Enabled {
			continue
		}

		var threshold int
		switch {
		case currentBackupCfg.CriticalDays > 0 && ageDays >= float64(currentBackupCfg.CriticalDays):
			threshold = currentBackupCfg.CriticalDays
		case currentBackupCfg.WarningDays > 0 && ageDays >= float64(currentBackupCfg.WarningDays):
			threshold = currentBackupCfg.WarningDays
		default:
			continue
		}

		alertKey := sanitizeAlertKey(key)
		alertID := fmt.Sprintf("backup-age-%s", alertKey)

		displayName := record.lookup.Name
		if displayName == "" {
			displayName = record.fallbackName
		}
		if displayName == "" {
			displayName = "Unknown guest"
		}

		node := record.node
		if node == "" {
			node = record.lookup.Node
		}
		instance := record.instance
		if instance == "" {
			instance = record.lookup.Instance
		}

		thresholdTime := record.lastTime.Add(time.Duration(threshold) * 24 * time.Hour)
		if thresholdTime.After(now) {
			thresholdTime = now
		}

		var sourceLabel string
		sourceLabel = record.source
		if len(record.providers) > 0 {
			parts := make([]string, 0, len(record.providers))
			for _, p := range record.providers {
				if s := strings.TrimSpace(string(p)); s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				sourceLabel = strings.Join(parts, ", ")
			}
		}

		message := fmt.Sprintf(
			"%s backup via %s is %.1f days old (threshold: %d days)",
			displayName,
			sourceLabel,
			ageDaysRounded,
			threshold,
		)

		metadata := map[string]interface{}{
			"source":         record.source,
			"providers":      record.providers,
			"rollupId":       record.rollupID,
			"lastBackupTime": record.lastTime,
			"ageDays":        ageDays,
			"thresholdDays":  threshold,
			"guestName":      displayName,
			"guestType":      record.lookup.Type,
			"guestInstance":  instance,
			"guestNode":      node,
			"guestVmid":      metadataIntValue(record.vmID),
			"orphaned":       record.vmID != "" && guestResourceID == "",
		}
		specResourceID := canonicalBackupSubjectResourceID(alertKey, *record)
		specResourceType := canonicalBackupSubjectResourceType(*record)
		spec, err := buildCanonicalPostureThresholdSpec(
			specResourceID+"-backup-age",
			specResourceID,
			displayName+" backup",
			specResourceType,
			"backup-age-days",
			float64(currentBackupCfg.WarningDays),
			float64(currentBackupCfg.CriticalDays),
			"",
			0,
			0,
			false,
		)
		if err != nil {
			log.Warn().
				Err(err).
				Str("alertID", alertID).
				Str("resourceID", specResourceID).
				Msg("Skipping invalid canonical backup posture spec")
			continue
		}
		validAlerts[canonicalTrackingKeyForSpec(spec, alertID)] = struct{}{}

		result, _ := m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
			Spec: spec,
			Evidence: alertspecs.AlertEvidence{
				ObservedAt: now,
				PostureThreshold: &alertspecs.PostureThresholdEvidence{
					AgeMetric: "backup-age-days",
					AgeValue:  ageDays,
				},
			},
			AlertID:           alertID,
			AlertType:         "backup-age",
			ResourceID:        spec.ResourceID,
			ResourceName:      fmt.Sprintf("%s backup", displayName),
			Node:              node,
			Instance:          instance,
			Value:             ageDays,
			Threshold:         float64(threshold),
			StartTimeOverride: thresholdTime,
			Metadata:          metadata,
			AddToRecent:       true,
			AddToHistory:      true,
			RateLimit:         true,
			DispatchAsync:     true,
			MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
				return message, ageDays, float64(threshold)
			},
		})
		if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
			asyncSaveActiveAlerts("backup", m.SaveActiveAlerts)
		}
	}

	m.mu.Lock()
	for storageKey, alert := range m.activeAlerts {
		if alert == nil || alert.Type != "backup-age" {
			continue
		}
		if _, ok := validAlerts[storageKey]; ok {
			continue
		}
		m.clearAlertNoLock(storageKey)
	}
	m.mu.Unlock()
}

func slicesContainsProvider(providers []recovery.Provider, target recovery.Provider) bool {
	for _, p := range providers {
		if p == target {
			return true
		}
	}
	return false
}

func parseGuestID(raw string) (instance string, node string, vmid int, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", 0, false
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 3 {
		return "", "", 0, false
	}
	last := parts[len(parts)-1]
	prev := parts[len(parts)-2]
	inst := strings.Join(parts[:len(parts)-2], ":")
	n, err := strconv.Atoi(strings.TrimSpace(last))
	if err != nil || n <= 0 {
		return "", "", 0, false
	}
	return strings.TrimSpace(inst), strings.TrimSpace(prev), n, true
}

func (m *Manager) clearSnapshotAlertsForInstance(instance string) {
	m.mu.Lock()
	m.clearSnapshotAlertsForInstanceLocked(instance)
	m.mu.Unlock()
}

func (m *Manager) clearSnapshotAlertsForInstanceLocked(instance string) {
	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if alert == nil || alert.Type != "snapshot-age" {
			continue
		}
		if instance != "" && alert.Instance != instance {
			continue
		}
		m.clearAlertNoLock(alertID)
	}
}

func (m *Manager) clearBackupAlerts() {
	m.mu.Lock()
	m.clearBackupAlertsLocked()
	m.mu.Unlock()
}

func (m *Manager) clearBackupAlertsLocked() {
	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if alert == nil || alert.Type != "backup-age" {
			continue
		}
		m.clearAlertNoLock(alertID)
	}
}
