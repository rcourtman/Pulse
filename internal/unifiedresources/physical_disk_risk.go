package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func PhysicalDiskRiskFromAssessment(assessment storagehealth.Assessment) *PhysicalDiskRisk {
	return PhysicalDiskRiskFromAssessmentAndIncidents(assessment, nil)
}

func PhysicalDiskRiskFromAssessmentAndIncidents(assessment storagehealth.Assessment, incidents []ResourceIncident) *PhysicalDiskRisk {
	reasons := make([]PhysicalDiskRiskReason, 0, len(assessment.Reasons)+len(incidents))
	seen := make(map[string]struct{}, len(assessment.Reasons)+len(incidents))
	level := assessment.Level

	for _, reason := range assessment.Reasons {
		appendPhysicalDiskRiskReason(&reasons, seen, reason.Code, reason.Severity, reason.Summary)
		if incidentSeverityRank(reason.Severity) > incidentSeverityRank(level) {
			level = reason.Severity
		}
	}

	for _, incident := range incidents {
		if !physicalDiskIncidentAffectsRisk(incident) {
			continue
		}
		appendPhysicalDiskRiskReason(&reasons, seen, incident.Code, incident.Severity, incident.Summary)
		if incidentSeverityRank(incident.Severity) > incidentSeverityRank(level) {
			level = incident.Severity
		}
	}

	if level == storagehealth.RiskHealthy && len(reasons) == 0 {
		return nil
	}

	return &PhysicalDiskRisk{
		Level:   level,
		Reasons: reasons,
	}
}

func appendPhysicalDiskRiskReason(reasons *[]PhysicalDiskRiskReason, seen map[string]struct{}, code string, severity storagehealth.RiskLevel, summary string) {
	code = strings.TrimSpace(code)
	summary = strings.TrimSpace(summary)
	if code == "" || summary == "" {
		return
	}

	key := code + "\x00" + summary
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*reasons = append(*reasons, PhysicalDiskRiskReason{
		Code:     code,
		Severity: severity,
		Summary:  summary,
	})
}

func physicalDiskIncidentAffectsRisk(incident ResourceIncident) bool {
	resource := &Resource{Type: ResourceTypePhysicalDisk}
	return IncidentCategoryForResource(resource, incident) == IncidentCategoryDiskHealth
}

func physicalDiskRiskFromMeta(meta *PhysicalDiskMeta, incidents []ResourceIncident) *PhysicalDiskRisk {
	return PhysicalDiskRiskFromAssessmentAndIncidents(physicalDiskAssessmentFromMeta(meta), incidents)
}

func physicalDiskRiskFromAssessment(assessment storagehealth.Assessment) *PhysicalDiskRisk {
	return PhysicalDiskRiskFromAssessmentAndIncidents(assessment, nil)
}

func physicalDiskAssessmentFromMeta(meta *PhysicalDiskMeta) storagehealth.Assessment {
	if meta == nil {
		return storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	}

	sample := storagehealth.Sample{
		Model:       meta.Model,
		Health:      meta.Health,
		Temperature: meta.Temperature,
		Wearout:     meta.Wearout,
		WearoutKnown: meta.Wearout > 0 ||
			(meta.Wearout == 0 && physicalDiskTypeIsNonRotational(meta.DiskType)),
	}
	if meta.SMART != nil {
		if meta.SMART.PowerOnHours != nil {
			sample.PowerOnHours = *meta.SMART.PowerOnHours
		}
		if meta.SMART.PowerCycles != nil {
			sample.PowerCycles = *meta.SMART.PowerCycles
		}
		if meta.SMART.ReallocatedSectors != nil {
			sample.ReallocatedSectors = *meta.SMART.ReallocatedSectors
		}
		if meta.SMART.PendingSectors != nil {
			sample.PendingSectors = *meta.SMART.PendingSectors
		}
		if meta.SMART.OfflineUncorrectable != nil {
			sample.OfflineUncorrectable = *meta.SMART.OfflineUncorrectable
		}
		if meta.SMART.UDMACRCErrors != nil {
			sample.UDMACRCErrors = *meta.SMART.UDMACRCErrors
		}
		if meta.SMART.PercentageUsed != nil {
			sample.PercentageUsed = *meta.SMART.PercentageUsed
			if remaining := storagehealth.RemainingLifeFromPercentageUsed(*meta.SMART.PercentageUsed); remaining >= 0 {
				sample.Wearout = remaining
				sample.WearoutKnown = true
			} else {
				sample.Wearout = -1
				sample.WearoutKnown = false
			}
		}
		if meta.SMART.AvailableSpare != nil {
			sample.AvailableSpare = *meta.SMART.AvailableSpare
			sample.AvailableSpareKnown = true
		}
		if meta.SMART.MediaErrors != nil {
			sample.MediaErrors = *meta.SMART.MediaErrors
		}
		if meta.SMART.UnsafeShutdowns != nil {
			sample.UnsafeShutdowns = *meta.SMART.UnsafeShutdowns
		}
	}
	return storagehealth.AssessSample(sample)
}

func physicalDiskTypeIsNonRotational(diskType string) bool {
	switch strings.ToLower(strings.TrimSpace(diskType)) {
	case "nvme", "ssd":
		return true
	default:
		return false
	}
}

func physicalDiskStatus(model, health string, assessment storagehealth.Assessment) ResourceStatus {
	switch assessment.Level {
	case storagehealth.RiskCritical, storagehealth.RiskWarning:
		return StatusWarning
	}

	switch strings.ToUpper(strings.TrimSpace(health)) {
	case "PASSED", "OK":
		return StatusOnline
	case "FAILED":
		if storagehealth.HasKnownFirmwareBug(model) {
			return StatusUnknown
		}
		return StatusOffline
	default:
		return StatusUnknown
	}
}

// PhysicalDiskStatus applies the canonical shared physical-disk status policy
// for providers that already calculated a disk-health assessment.
func PhysicalDiskStatus(model, health string, assessment storagehealth.Assessment) ResourceStatus {
	return physicalDiskStatus(model, health, assessment)
}
