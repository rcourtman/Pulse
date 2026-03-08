package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func StorageRiskFromAssessment(assessment storagehealth.Assessment) *StorageRisk {
	if assessment.Level == storagehealth.RiskHealthy && len(assessment.Reasons) == 0 {
		return nil
	}

	reasons := make([]StorageRiskReason, 0, len(assessment.Reasons))
	for _, reason := range assessment.Reasons {
		reasons = append(reasons, StorageRiskReason{
			Code:     reason.Code,
			Severity: reason.Severity,
			Summary:  reason.Summary,
		})
	}

	return &StorageRisk{
		Level:   assessment.Level,
		Reasons: reasons,
	}
}

func storageRiskFromAssessment(assessment storagehealth.Assessment) *StorageRisk {
	return StorageRiskFromAssessment(assessment)
}

func storageStatus(base ResourceStatus, risk *StorageRisk) ResourceStatus {
	if risk == nil {
		return base
	}
	switch risk.Level {
	case storagehealth.RiskCritical, storagehealth.RiskWarning:
		if base == StatusOffline {
			return base
		}
		return StatusWarning
	default:
		return base
	}
}

func isInternalHostRAIDDevice(device string) bool {
	deviceLower := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(device), "/dev/"))
	return deviceLower == "md0" || deviceLower == "md1"
}
