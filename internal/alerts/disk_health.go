package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func proxmoxDiskCanonicalResourceID(instance, node, devPath string) string {
	return fmt.Sprintf("%s:%s:disk:%s", strings.TrimSpace(instance), strings.TrimSpace(node), sanitizeAlertKey(devPath))
}

func proxmoxDiskAlertMetadata(disk proxmox.Disk) map[string]interface{} {
	return map[string]interface{}{
		"disk_path":   disk.DevPath,
		"disk_model":  disk.Model,
		"disk_serial": disk.Serial,
		"disk_type":   disk.Type,
		"disk_size":   disk.Size,
	}
}

// CheckDiskHealth checks disk health and creates alerts if needed
func (m *Manager) CheckDiskHealth(instance, node string, disk proxmox.Disk) {
	// Create unique alert ID for this disk
	alertID := fmt.Sprintf("disk-health-%s-%s-%s", instance, node, disk.DevPath)
	wearoutAlertID := fmt.Sprintf("disk-wearout-%s-%s-%s", instance, node, disk.DevPath)
	resourceID := fmt.Sprintf("%s-%s", node, disk.DevPath)
	resourceName := fmt.Sprintf("%s (%s)", disk.Model, disk.DevPath)
	canonicalResourceID := proxmoxDiskCanonicalResourceID(instance, node, disk.DevPath)
	resourceType := unifiedresources.ResourceType("proxmox-disk")
	canonicalHealthAlertID := buildCanonicalStateID(canonicalResourceID, canonicalResourceID+"-health")
	canonicalWearoutAlertID := buildCanonicalStateID(canonicalResourceID, canonicalResourceID+"-wearout")

	clearDiskAlert := func(ids ...string) {
		seen := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			m.clearAlert(id)
		}
	}

	// Check if disk health is not PASSED
	normalizedHealth := strings.ToUpper(strings.TrimSpace(disk.Health))
	healthCheckNeeded := normalizedHealth != "" && normalizedHealth != "UNKNOWN" && normalizedHealth != "PASSED" && normalizedHealth != "OK"

	// Skip health alerts for drives with known firmware bugs that cause false reports
	// These drives may report FAILED status due to firmware issues even when healthy
	// We still monitor wearout below, which is more reliable for these drives
	if healthCheckNeeded && storagehealth.HasKnownFirmwareBug(disk.Model) {
		log.Debug().
			Str("node", node).
			Str("disk", disk.DevPath).
			Str("model", disk.Model).
			Str("health", disk.Health).
			Msg("Skipping health alert for drive with known firmware bug - health status unreliable")

		// Clear any existing health alert since we now recognize this is a false positive.
		// Older installs may still have the legacy alert key persisted on disk.
		clearDiskAlert(alertID, canonicalHealthAlertID)
		healthCheckNeeded = false // Skip to wearout check
	}

	if healthCheckNeeded {
		spec, err := buildCanonicalHealthAssessmentSpec(canonicalResourceID+"-health", canonicalResourceID, resourceName, resourceType, "proxmox-disk-health", nil, false)
		if err != nil {
			log.Warn().
				Err(err).
				Str("node", node).
				Str("disk", disk.DevPath).
				Msg("Skipping invalid canonical proxmox disk health spec")
		} else {
			metadata := proxmoxDiskAlertMetadata(disk)
			metadata["disk_health"] = disk.Health

			_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
				Spec: spec,
				Evidence: alertspecs.AlertEvidence{
					ObservedAt: time.Now(),
					HealthAssessment: &alertspecs.HealthAssessmentEvidence{
						Signal:   "proxmox-disk-health",
						Severity: alertspecs.AlertSeverityCritical,
						Codes:    []string{normalizedHealth},
					},
				},
				AlertID:      alertID,
				AlertType:    "disk-health",
				ResourceID:   resourceID,
				ResourceName: resourceName,
				Node:         node,
				Instance:     instance,
				Message:      fmt.Sprintf("Disk health check failed: %s", disk.Health),
				Metadata:     metadata,
				AddToRecent:  true,
				AddToHistory: true,
			})

			log.Error().
				Str("node", node).
				Str("disk", disk.DevPath).
				Str("model", disk.Model).
				Str("health", disk.Health).
				Msg("Disk health alert created")
		}
	} else {
		// Disk is healthy, clear alert if it exists
		clearDiskAlert(alertID, canonicalHealthAlertID)
	}

	// Check for low wearout (SSD life remaining)
	if disk.Wearout > 0 && disk.Wearout < 10 {
		message := fmt.Sprintf("SSD has less than 10%% life remaining (%d%% wearout)", disk.Wearout)
		spec, err := buildCanonicalSeverityThresholdSpecWithDirection(canonicalResourceID+"-wearout", canonicalResourceID, resourceName, resourceType, "wearout-remaining", alertspecs.ThresholdDirectionBelow, 10, 0, false)
		if err != nil {
			log.Warn().
				Err(err).
				Str("node", node).
				Str("disk", disk.DevPath).
				Msg("Skipping invalid canonical proxmox disk wearout spec")
		} else {
			metadata := proxmoxDiskAlertMetadata(disk)
			metadata["disk_wearout"] = disk.Wearout

			_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
				Spec: spec,
				Evidence: alertspecs.AlertEvidence{
					ObservedAt: time.Now(),
					SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
						Metric:    "wearout-remaining",
						Direction: alertspecs.ThresholdDirectionBelow,
						Observed:  float64(disk.Wearout),
					},
				},
				AlertID:      wearoutAlertID,
				AlertType:    "disk-wearout",
				ResourceID:   resourceID,
				ResourceName: resourceName,
				Node:         node,
				Instance:     instance,
				Message:      message,
				Value:        float64(disk.Wearout),
				Threshold:    10.0,
				Metadata:     metadata,
				AddToRecent:  true,
				AddToHistory: true,
			})

			log.Warn().
				Str("node", node).
				Str("disk", disk.DevPath).
				Str("model", disk.Model).
				Int("wearout", disk.Wearout).
				Msg("Disk wearout alert created")
		}
	} else if disk.Wearout >= 10 {
		// Wearout is acceptable, clear alert if it exists
		clearDiskAlert(wearoutAlertID, canonicalWearoutAlertID)
	}
}
