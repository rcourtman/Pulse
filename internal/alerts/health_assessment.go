package alerts

import (
	"fmt"
	"slices"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

var (
	zfsPoolAssessmentCodes = []string{
		"zfs_pool_state",
	}
	zfsPoolErrorAssessmentCodes = []string{
		"zfs_pool_errors",
	}
	zfsDeviceAssessmentCodes = []string{
		"zfs_device_state",
		"zfs_device_errors",
	}
)

type canonicalHealthAssessmentAlertParams struct {
	SpecID         string
	Signal         string
	Codes          []string
	Reasons        []storagehealth.Reason
	AlertID        string
	AlertType      string
	SpecResourceID string
	ResourceID     string
	ResourceName   string
	ResourceType   unifiedresources.ResourceType
	Node           string
	Instance       string
	Metadata       map[string]interface{}
	Disabled       bool
	MessageBuilder func(alertspecs.EvaluationResult) (string, float64, float64)
}

func storageHealthReasonCodes(reasons []storagehealth.Reason) []string {
	codes := make([]string, 0, len(reasons))
	seen := make(map[string]struct{}, len(reasons))
	for _, reason := range reasons {
		code := strings.TrimSpace(reason.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

func storageHealthReasonSummaries(reasons []storagehealth.Reason) []string {
	summaries := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		summary := strings.TrimSpace(reason.Summary)
		if summary == "" {
			continue
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func storageHealthAssessmentSeverity(reasons []storagehealth.Reason) alertspecs.AlertSeverity {
	severity := alertspecs.AlertSeverity("")
	for _, reason := range reasons {
		switch reason.Severity {
		case storagehealth.RiskCritical:
			return alertspecs.AlertSeverityCritical
		case storagehealth.RiskWarning:
			severity = alertspecs.AlertSeverityWarning
		}
	}
	return severity
}

func (m *Manager) activeAlertValue(alertID string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.getActiveAlertNoLock(alertID)
	if !ok || alert == nil {
		return 0, false
	}
	return alert.Value, true
}

func filterStorageHealthReasonsByCodes(reasons []storagehealth.Reason, codes []string) []storagehealth.Reason {
	if len(reasons) == 0 || len(codes) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		allowed[code] = struct{}{}
	}

	filtered := make([]storagehealth.Reason, 0, len(reasons))
	for _, reason := range reasons {
		if _, ok := allowed[strings.TrimSpace(reason.Code)]; ok {
			filtered = append(filtered, reason)
		}
	}
	return filtered
}

func zfsDeviceAssessment(device models.ZFSDevice) storagehealth.Assessment {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	addReason := func(code string, severity storagehealth.RiskLevel, summary string) {
		if strings.TrimSpace(summary) == "" {
			return
		}
		assessment.Reasons = append(assessment.Reasons, storagehealth.Reason{
			Code:     code,
			Severity: severity,
			Summary:  summary,
		})
		switch severity {
		case storagehealth.RiskCritical:
			assessment.Level = storagehealth.RiskCritical
		case storagehealth.RiskWarning:
			if assessment.Level != storagehealth.RiskCritical {
				assessment.Level = storagehealth.RiskWarning
			}
		}
	}

	state := strings.ToUpper(strings.TrimSpace(device.State))
	switch state {
	case "", "ONLINE", "SPARE":
	case "DEGRADED":
		addReason("zfs_device_state", storagehealth.RiskWarning, fmt.Sprintf("ZFS device %s is DEGRADED", device.Name))
	default:
		addReason("zfs_device_state", storagehealth.RiskCritical, fmt.Sprintf("ZFS device %s is %s", device.Name, state))
	}

	errors := device.ReadErrors + device.WriteErrors + device.ChecksumErrors
	if errors > 0 {
		addReason(
			"zfs_device_errors",
			storagehealth.RiskWarning,
			fmt.Sprintf("ZFS device %s has errors: %d read, %d write, %d checksum", device.Name, device.ReadErrors, device.WriteErrors, device.ChecksumErrors),
		)
	}

	return assessment
}

func (m *Manager) syncCanonicalHealthAssessmentAlert(params canonicalHealthAssessmentAlertParams) (alertspecs.EvaluationResult, bool) {
	if len(params.Reasons) == 0 {
		m.clearAlert(buildCanonicalStateID(params.SpecResourceID, params.SpecID))
		return alertspecs.EvaluationResult{}, true
	}

	spec, err := buildCanonicalHealthAssessmentSpec(
		params.SpecID,
		params.SpecResourceID,
		params.ResourceName,
		params.ResourceType,
		params.Signal,
		params.Codes,
		params.Disabled,
	)
	if err != nil {
		log.Warn().
			Err(err).
			Str("alertID", params.AlertID).
			Str("resourceID", params.SpecResourceID).
			Msg("Skipping invalid canonical health assessment spec")
		return alertspecs.EvaluationResult{}, false
	}

	now := time.Now()
	return m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: now,
			HealthAssessment: &alertspecs.HealthAssessmentEvidence{
				Signal:   params.Signal,
				Severity: storageHealthAssessmentSeverity(params.Reasons),
				Codes:    storageHealthReasonCodes(params.Reasons),
			},
		},
		AlertID:        params.AlertID,
		AlertType:      params.AlertType,
		ResourceID:     params.ResourceID,
		ResourceName:   params.ResourceName,
		Node:           params.Node,
		Instance:       params.Instance,
		Message:        strings.Join(storageHealthReasonSummaries(params.Reasons), "; "),
		Metadata:       cloneMetadata(params.Metadata),
		AddToRecent:    true,
		AddToHistory:   true,
		MessageBuilder: params.MessageBuilder,
	})
}
