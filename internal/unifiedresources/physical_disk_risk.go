package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func PhysicalDiskRiskFromAssessment(assessment storagehealth.Assessment) *PhysicalDiskRisk {
	if assessment.Level == storagehealth.RiskHealthy && len(assessment.Reasons) == 0 {
		return nil
	}

	reasons := make([]PhysicalDiskRiskReason, 0, len(assessment.Reasons))
	for _, reason := range assessment.Reasons {
		reasons = append(reasons, PhysicalDiskRiskReason{
			Code:     reason.Code,
			Severity: reason.Severity,
			Summary:  reason.Summary,
		})
	}

	return &PhysicalDiskRisk{
		Level:   assessment.Level,
		Reasons: reasons,
	}
}

func physicalDiskRiskFromAssessment(assessment storagehealth.Assessment) *PhysicalDiskRisk {
	return PhysicalDiskRiskFromAssessment(assessment)
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
	}
	if meta.SMART != nil {
		sample.PowerOnHours = meta.SMART.PowerOnHours
		sample.PowerCycles = meta.SMART.PowerCycles
		sample.ReallocatedSectors = meta.SMART.ReallocatedSectors
		sample.PendingSectors = meta.SMART.PendingSectors
		sample.OfflineUncorrectable = meta.SMART.OfflineUncorrectable
		sample.UDMACRCErrors = meta.SMART.UDMACRCErrors
		sample.PercentageUsed = meta.SMART.PercentageUsed
		sample.AvailableSpare = meta.SMART.AvailableSpare
		sample.MediaErrors = meta.SMART.MediaErrors
		sample.UnsafeShutdowns = meta.SMART.UnsafeShutdowns
	}
	return storagehealth.AssessSample(sample)
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
