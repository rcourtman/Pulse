package storagehealth

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type RiskLevel string

const (
	RiskHealthy  RiskLevel = "healthy"
	RiskMonitor  RiskLevel = "monitor"
	RiskWarning  RiskLevel = "warning"
	RiskCritical RiskLevel = "critical"
)

type Reason struct {
	Code     string    `json:"code"`
	Severity RiskLevel `json:"severity"`
	Summary  string    `json:"summary"`
}

type Assessment struct {
	Level   RiskLevel `json:"level"`
	Reasons []Reason  `json:"reasons,omitempty"`
}

type Sample struct {
	Model                string
	Health               string
	Temperature          int
	Wearout              int
	PowerOnHours         int64
	PowerCycles          int64
	ReallocatedSectors   int64
	PendingSectors       int64
	OfflineUncorrectable int64
	UDMACRCErrors        int64
	PercentageUsed       int
	AvailableSpare       int
	MediaErrors          int64
	UnsafeShutdowns      int64
}

func AssessPhysicalDisk(disk models.PhysicalDisk) Assessment {
	sample := Sample{
		Model:       disk.Model,
		Health:      disk.Health,
		Temperature: disk.Temperature,
		Wearout:     disk.Wearout,
	}
	if attrs := disk.SmartAttributes; attrs != nil {
		if attrs.PowerOnHours != nil {
			sample.PowerOnHours = *attrs.PowerOnHours
		}
		if attrs.PowerCycles != nil {
			sample.PowerCycles = *attrs.PowerCycles
		}
		if attrs.ReallocatedSectors != nil {
			sample.ReallocatedSectors = *attrs.ReallocatedSectors
		}
		if attrs.PendingSectors != nil {
			sample.PendingSectors = *attrs.PendingSectors
		}
		if attrs.OfflineUncorrectable != nil {
			sample.OfflineUncorrectable = *attrs.OfflineUncorrectable
		}
		if attrs.UDMACRCErrors != nil {
			sample.UDMACRCErrors = *attrs.UDMACRCErrors
		}
		if attrs.PercentageUsed != nil {
			sample.PercentageUsed = *attrs.PercentageUsed
		}
		if attrs.AvailableSpare != nil {
			sample.AvailableSpare = *attrs.AvailableSpare
		}
		if attrs.MediaErrors != nil {
			sample.MediaErrors = *attrs.MediaErrors
		}
		if attrs.UnsafeShutdowns != nil {
			sample.UnsafeShutdowns = *attrs.UnsafeShutdowns
		}
	}
	return AssessSample(sample)
}

func AssessHostSMARTDisk(disk models.HostDiskSMART) Assessment {
	sample := Sample{
		Model:       disk.Model,
		Health:      disk.Health,
		Temperature: disk.Temperature,
		Wearout:     -1,
	}
	if attrs := disk.Attributes; attrs != nil {
		if attrs.PowerOnHours != nil {
			sample.PowerOnHours = *attrs.PowerOnHours
		}
		if attrs.PowerCycles != nil {
			sample.PowerCycles = *attrs.PowerCycles
		}
		if attrs.ReallocatedSectors != nil {
			sample.ReallocatedSectors = *attrs.ReallocatedSectors
		}
		if attrs.PendingSectors != nil {
			sample.PendingSectors = *attrs.PendingSectors
		}
		if attrs.OfflineUncorrectable != nil {
			sample.OfflineUncorrectable = *attrs.OfflineUncorrectable
		}
		if attrs.UDMACRCErrors != nil {
			sample.UDMACRCErrors = *attrs.UDMACRCErrors
		}
		if attrs.PercentageUsed != nil {
			sample.PercentageUsed = *attrs.PercentageUsed
			sample.Wearout = 100 - *attrs.PercentageUsed
		}
		if attrs.AvailableSpare != nil {
			sample.AvailableSpare = *attrs.AvailableSpare
		}
		if attrs.MediaErrors != nil {
			sample.MediaErrors = *attrs.MediaErrors
		}
		if attrs.UnsafeShutdowns != nil {
			sample.UnsafeShutdowns = *attrs.UnsafeShutdowns
		}
	}
	return AssessSample(sample)
}

func AssessSample(sample Sample) Assessment {
	assessment := Assessment{Level: RiskHealthy}
	addReason := func(code string, severity RiskLevel, summary string) {
		if summary == "" {
			return
		}
		assessment.Reasons = append(assessment.Reasons, Reason{
			Code:     code,
			Severity: severity,
			Summary:  summary,
		})
		if severityRank(severity) > severityRank(assessment.Level) {
			assessment.Level = severity
		}
	}

	normalizedHealth := normalizeHealth(sample.Health)
	if normalizedHealth != "" && normalizedHealth != "UNKNOWN" && normalizedHealth != "PASSED" && normalizedHealth != "OK" && !HasKnownFirmwareBug(sample.Model) {
		addReason("health_status", RiskCritical, fmt.Sprintf("Disk reports health status %s", normalizedHealth))
	}
	if sample.PendingSectors > 0 {
		addReason("pending_sectors", RiskCritical, fmt.Sprintf("Pending sectors detected (%d)", sample.PendingSectors))
	}
	if sample.OfflineUncorrectable > 0 {
		addReason("offline_uncorrectable", RiskCritical, fmt.Sprintf("Offline uncorrectable sectors detected (%d)", sample.OfflineUncorrectable))
	}
	if sample.MediaErrors > 0 {
		addReason("media_errors", RiskCritical, fmt.Sprintf("Media errors detected (%d)", sample.MediaErrors))
	}
	if sample.Wearout > 0 && sample.Wearout <= 5 {
		addReason("wearout_low", RiskCritical, fmt.Sprintf("SSD life remaining is %d%%", sample.Wearout))
	} else if sample.Wearout > 0 && sample.Wearout < 10 {
		addReason("wearout_low", RiskWarning, fmt.Sprintf("SSD life remaining is %d%%", sample.Wearout))
	}
	if sample.AvailableSpare > 0 && sample.AvailableSpare <= 10 {
		addReason("nvme_available_spare_low", RiskCritical, fmt.Sprintf("NVMe available spare is %d%%", sample.AvailableSpare))
	} else if sample.AvailableSpare > 0 && sample.AvailableSpare < 20 {
		addReason("nvme_available_spare_low", RiskWarning, fmt.Sprintf("NVMe available spare is %d%%", sample.AvailableSpare))
	}
	if sample.PercentageUsed >= 95 {
		addReason("nvme_percentage_used_high", RiskCritical, fmt.Sprintf("NVMe endurance used is %d%%", sample.PercentageUsed))
	} else if sample.PercentageUsed >= 90 {
		addReason("nvme_percentage_used_high", RiskWarning, fmt.Sprintf("NVMe endurance used is %d%%", sample.PercentageUsed))
	}
	if sample.Temperature >= 70 {
		addReason("temperature_high", RiskCritical, fmt.Sprintf("Disk temperature is %dC", sample.Temperature))
	} else if sample.Temperature >= 60 {
		addReason("temperature_high", RiskWarning, fmt.Sprintf("Disk temperature is %dC", sample.Temperature))
	}
	if sample.ReallocatedSectors > 0 {
		addReason("reallocated_sectors", RiskWarning, fmt.Sprintf("Reallocated sectors detected (%d)", sample.ReallocatedSectors))
	}
	if sample.UDMACRCErrors > 0 {
		addReason("crc_errors", RiskMonitor, fmt.Sprintf("UDMA CRC errors detected (%d)", sample.UDMACRCErrors))
	}

	sort.SliceStable(assessment.Reasons, func(i, j int) bool {
		left := assessment.Reasons[i]
		right := assessment.Reasons[j]
		if severityRank(left.Severity) != severityRank(right.Severity) {
			return severityRank(left.Severity) > severityRank(right.Severity)
		}
		return left.Code < right.Code
	})

	return assessment
}

func HasKnownFirmwareBug(model string) bool {
	normalizedModel := strings.ToUpper(strings.TrimSpace(model))
	knownProblematicModels := []string{
		"SAMSUNG SSD 980",
		"SAMSUNG 980",
		"SAMSUNG SSD 990",
		"SAMSUNG 990",
	}

	for _, problematic := range knownProblematicModels {
		if strings.Contains(normalizedModel, problematic) {
			return true
		}
	}
	return false
}

func normalizeHealth(health string) string {
	return strings.ToUpper(strings.TrimSpace(health))
}

func severityRank(level RiskLevel) int {
	switch level {
	case RiskCritical:
		return 3
	case RiskWarning:
		return 2
	case RiskMonitor:
		return 1
	default:
		return 0
	}
}
