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

// SMARTThresholds controls the discrete health evidence that becomes a disk
// risk. Counter values at zero disable that rule; the endurance percentages
// use the same convention. Temperature remains a separate metric threshold.
type SMARTThresholds struct {
	HealthFailure        bool
	ReallocatedSectors   int64
	PendingSectors       int64
	OfflineUncorrectable int64
	MediaErrors          int64
	LifeWarning          int
	LifeCritical         int
	AvailableSpareWarn   int
	AvailableSpareCrit   int
}

func DefaultSMARTThresholds() SMARTThresholds {
	return SMARTThresholds{
		HealthFailure:        true,
		ReallocatedSectors:   1,
		PendingSectors:       1,
		OfflineUncorrectable: 1,
		MediaErrors:          1,
		LifeWarning:          10,
		LifeCritical:         5,
		AvailableSpareWarn:   20,
		AvailableSpareCrit:   10,
	}
}

type Sample struct {
	Model                string
	Health               string
	Temperature          int
	Wearout              int
	WearoutKnown         bool
	PowerOnHours         int64
	PowerCycles          int64
	ReallocatedSectors   int64
	PendingSectors       int64
	OfflineUncorrectable int64
	UDMACRCErrors        int64
	PercentageUsed       int
	AvailableSpare       int
	AvailableSpareKnown  bool
	MediaErrors          int64
	UnsafeShutdowns      int64
}

func AssessPhysicalDisk(disk models.PhysicalDisk) Assessment {
	sample := Sample{
		Model:        disk.Model,
		Health:       disk.Health,
		Temperature:  disk.Temperature,
		Wearout:      disk.Wearout,
		WearoutKnown: WearoutReported(disk.Wearout, disk.Type),
	}
	applySMARTAttributes(&sample, disk.SmartAttributes)
	return AssessSample(sample)
}

func AssessHostSMARTDisk(disk models.HostDiskSMART) Assessment {
	return AssessHostSMARTDiskWithThresholds(disk, DefaultSMARTThresholds())
}

func AssessHostSMARTDiskWithThresholds(disk models.HostDiskSMART, thresholds SMARTThresholds) Assessment {
	sample := Sample{
		Model:       disk.Model,
		Health:      disk.Health,
		Temperature: disk.Temperature,
		Wearout:     -1,
	}
	applySMARTAttributes(&sample, disk.Attributes)
	return AssessSampleWithThresholds(sample, thresholds)
}

func applySMARTAttributes(sample *Sample, attrs *models.SMARTAttributes) {
	if attrs == nil {
		return
	}
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
		if remaining := RemainingLifeFromPercentageUsed(*attrs.PercentageUsed); remaining >= 0 {
			sample.Wearout = remaining
			sample.WearoutKnown = true
		} else {
			sample.Wearout = -1
			sample.WearoutKnown = false
		}
	}
	if attrs.AvailableSpare != nil {
		sample.AvailableSpare = *attrs.AvailableSpare
		sample.AvailableSpareKnown = true
	}
	if attrs.MediaErrors != nil {
		sample.MediaErrors = *attrs.MediaErrors
	}
	if attrs.UnsafeShutdowns != nil {
		sample.UnsafeShutdowns = *attrs.UnsafeShutdowns
	}
}

func AssessSample(sample Sample) Assessment {
	return AssessSampleWithThresholds(sample, DefaultSMARTThresholds())
}

func AssessSampleWithThresholds(sample Sample, thresholds SMARTThresholds) Assessment {
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
	if thresholds.HealthFailure && normalizedHealth != "" && normalizedHealth != "UNKNOWN" && normalizedHealth != "PASSED" && normalizedHealth != "OK" && !HasKnownFirmwareBug(sample.Model) {
		addReason("health_status", RiskCritical, fmt.Sprintf("Disk reports health status %s", normalizedHealth))
	}
	if thresholds.PendingSectors > 0 && sample.PendingSectors >= thresholds.PendingSectors {
		addReason("pending_sectors", RiskCritical, fmt.Sprintf("Pending sectors detected (%d)", sample.PendingSectors))
	}
	if thresholds.OfflineUncorrectable > 0 && sample.OfflineUncorrectable >= thresholds.OfflineUncorrectable {
		addReason("offline_uncorrectable", RiskCritical, fmt.Sprintf("Offline uncorrectable sectors detected (%d)", sample.OfflineUncorrectable))
	}
	if thresholds.MediaErrors > 0 && sample.MediaErrors >= thresholds.MediaErrors {
		addReason("media_errors", RiskCritical, fmt.Sprintf("Media errors detected (%d)", sample.MediaErrors))
	}
	if thresholds.LifeCritical > 0 && (sample.WearoutKnown || sample.Wearout > 0) && sample.Wearout >= 0 && sample.Wearout <= thresholds.LifeCritical {
		addReason("wearout_low", RiskCritical, fmt.Sprintf("SSD life remaining is %d%%", sample.Wearout))
	} else if thresholds.LifeWarning > 0 && sample.Wearout > thresholds.LifeCritical && sample.Wearout < thresholds.LifeWarning {
		addReason("wearout_low", RiskWarning, fmt.Sprintf("SSD life remaining is %d%%", sample.Wearout))
	}
	if thresholds.AvailableSpareCrit > 0 && (sample.AvailableSpareKnown || sample.AvailableSpare > 0) && sample.AvailableSpare <= thresholds.AvailableSpareCrit {
		addReason("nvme_available_spare_low", RiskCritical, fmt.Sprintf("NVMe available spare is %d%%", sample.AvailableSpare))
	} else if thresholds.AvailableSpareWarn > 0 && sample.AvailableSpare > thresholds.AvailableSpareCrit && sample.AvailableSpare < thresholds.AvailableSpareWarn {
		addReason("nvme_available_spare_low", RiskWarning, fmt.Sprintf("NVMe available spare is %d%%", sample.AvailableSpare))
	}
	percentageCritical := thresholds.LifeCritical > 0 && sample.PercentageUsed >= 100-thresholds.LifeCritical
	percentageWarning := thresholds.LifeWarning > 0 && sample.PercentageUsed >= 100-thresholds.LifeWarning
	if percentageCritical {
		addReason("nvme_percentage_used_high", RiskCritical, fmt.Sprintf("NVMe endurance used is %d%%", sample.PercentageUsed))
	} else if percentageWarning {
		addReason("nvme_percentage_used_high", RiskWarning, fmt.Sprintf("NVMe endurance used is %d%%", sample.PercentageUsed))
	}
	if sample.Temperature >= 70 {
		addReason("temperature_high", RiskCritical, fmt.Sprintf("Disk temperature is %dC", sample.Temperature))
	} else if sample.Temperature >= 60 {
		addReason("temperature_high", RiskWarning, fmt.Sprintf("Disk temperature is %dC", sample.Temperature))
	}
	if thresholds.ReallocatedSectors > 0 && sample.ReallocatedSectors >= thresholds.ReallocatedSectors {
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

// RemainingLifeFromPercentageUsed converts the NVMe percentage-used counter
// into Pulse's remaining-life representation. Negative controller values are
// invalid and stay unknown; values above 100 mean endurance is exhausted.
func RemainingLifeFromPercentageUsed(used int) int {
	if used < 0 {
		return -1
	}
	if used > 100 {
		used = 100
	}
	return 100 - used
}

// WearoutReported reports whether a wearout reading is real evidence rather
// than an absent value. -1 is the canonical unreported sentinel. 0 is a real
// reading meaning no endurance remains, but only from a device that reports
// endurance at all: rotational disks never do, so a 0 from one is treated as
// absent. Every consumer of wearout must gate on this rather than reinventing
// the boundary, or the UI and alerting drift apart on the same disk.
func WearoutReported(wearout int, diskType string) bool {
	if wearout > 0 {
		return true
	}
	return wearout == 0 && isNonRotationalDiskType(diskType)
}

func isNonRotationalDiskType(diskType string) bool {
	switch strings.ToLower(strings.TrimSpace(diskType)) {
	case "nvme", "ssd":
		return true
	default:
		return false
	}
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
