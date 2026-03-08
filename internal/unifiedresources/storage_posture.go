package unifiedresources

import (
	"strconv"
	"strings"
)

const maxStorageImpactNames = 3

func (rr *ResourceRegistry) refreshStoragePostureLocked() {
	for _, resource := range rr.resources {
		if resource == nil || resource.Storage == nil {
			continue
		}

		resource.Storage.RiskSummary = StorageRiskSummary(resource.Storage.Risk)
		resource.Storage.ConsumerImpactSummary = StorageConsumerImpactSummary(resource.Storage)
		resource.Storage.PostureSummary = StoragePostureSummary(resource.Storage)
		_, resource.Storage.ProtectionReduced, resource.Storage.RebuildInProgress, resource.Storage.ProtectionSummary, resource.Storage.RebuildSummary = StorageRiskSemantics(resource.Storage.Risk)
	}
}

func StorageRiskSummary(risk *StorageRisk) string {
	if risk == nil {
		return ""
	}
	for _, reason := range risk.Reasons {
		summary := strings.TrimSpace(reason.Summary)
		if summary != "" {
			return summary
		}
	}
	return ""
}

func StorageConsumerImpactSummary(storage *StorageMeta) string {
	if storage == nil || storage.ConsumerCount <= 0 {
		return ""
	}

	names := make([]string, 0, min(len(storage.TopConsumers), maxStorageImpactNames))
	for _, consumer := range storage.TopConsumers {
		name := strings.TrimSpace(consumer.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
		if len(names) == maxStorageImpactNames {
			break
		}
	}

	if IsBackupStorageResource(storage) {
		return summarizeBackupConsumerImpact(storage.ConsumerCount, names)
	}
	return summarizeDependentConsumerImpact(storage.ConsumerCount, names)
}

func StoragePostureSummary(storage *StorageMeta) string {
	if storage == nil {
		return ""
	}
	riskSummary := strings.TrimSpace(storage.RiskSummary)
	if riskSummary == "" {
		riskSummary = StorageRiskSummary(storage.Risk)
	}
	consumerSummary := strings.TrimSpace(storage.ConsumerImpactSummary)
	if consumerSummary == "" {
		consumerSummary = StorageConsumerImpactSummary(storage)
	}
	switch {
	case riskSummary != "" && consumerSummary != "":
		return riskSummary + ". " + consumerSummary
	case riskSummary != "":
		return riskSummary
	default:
		return consumerSummary
	}
}

func IsBackupStorageResource(storage *StorageMeta) bool {
	if storage == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(storage.Protection), "backup-repository") {
		return true
	}
	for _, contentType := range storage.ContentTypes {
		if strings.EqualFold(strings.TrimSpace(contentType), "backup") {
			return true
		}
	}
	return false
}

func StorageRiskSemantics(risk *StorageRisk) ([]string, bool, bool, string, string) {
	if risk == nil || len(risk.Reasons) == 0 {
		return nil, false, false, "", ""
	}

	codes := make([]string, 0, len(risk.Reasons))
	protectionReduced := false
	rebuildInProgress := false
	protectionSummary := ""
	rebuildSummary := ""
	for _, reason := range risk.Reasons {
		code := strings.TrimSpace(reason.Code)
		if code != "" {
			codes = append(codes, code)
		}
		switch code {
		case "raid_degraded", "raid_unavailable", "unraid_invalid_disks", "unraid_disabled_disks", "unraid_missing_disks", "unraid_parity_unavailable", "unraid_no_parity", "zfs_pool_state":
			protectionReduced = true
			if protectionSummary == "" {
				protectionSummary = strings.TrimSpace(reason.Summary)
			}
		case "raid_rebuilding", "unraid_sync_active":
			rebuildInProgress = true
			if rebuildSummary == "" {
				rebuildSummary = strings.TrimSpace(reason.Summary)
			}
		}
	}

	return codes, protectionReduced, rebuildInProgress, protectionSummary, rebuildSummary
}

func summarizeDependentConsumerImpact(count int, names []string) string {
	if count <= 0 {
		return ""
	}

	resourceLabel := "dependent resource"
	if count != 1 {
		resourceLabel = "dependent resources"
	}
	if len(names) == 0 {
		return "Affects " + strconv.Itoa(count) + " " + resourceLabel
	}
	if remaining := count - len(names); remaining > 0 {
		return "Affects " + strconv.Itoa(count) + " " + resourceLabel + ": " + strings.Join(names, ", ") + ", and " + strconv.Itoa(remaining) + " more"
	}
	return "Affects " + strconv.Itoa(count) + " " + resourceLabel + ": " + strings.Join(names, ", ")
}

func summarizeBackupConsumerImpact(count int, names []string) string {
	if count <= 0 {
		return ""
	}

	workloadLabel := "protected workload"
	if count != 1 {
		workloadLabel = "protected workloads"
	}
	if len(names) == 0 {
		return "Puts backups for " + strconv.Itoa(count) + " " + workloadLabel + " at risk"
	}
	if remaining := count - len(names); remaining > 0 {
		return "Puts backups for " + strconv.Itoa(count) + " " + workloadLabel + " at risk: " + strings.Join(names, ", ") + ", and " + strconv.Itoa(remaining) + " more"
	}
	return "Puts backups for " + strconv.Itoa(count) + " " + workloadLabel + " at risk: " + strings.Join(names, ", ")
}
