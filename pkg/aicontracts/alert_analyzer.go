package aicontracts

import (
	"reflect"
	"time"
)

// Finding severity constants — must match the internal FindingSeverity values
// in the ai package so the OSS adapter cast is safe.
const (
	SeverityInfo     = "info"
	SeverityWatch    = "watch"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Finding category constants — must match the internal FindingCategory values.
const (
	CategorySecurity    = "security"
	CategoryReliability = "reliability"
	CategoryBackup      = "backup"
)

// IsNilAlertPayload detects typed-nil interface values that would panic on
// method calls. Use this instead of a bare == nil check.
func IsNilAlertPayload(p AlertPayload) bool {
	if p == nil {
		return true
	}
	v := reflect.ValueOf(p)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

// AlertAnalyzerFinding is the contract type passed from the enterprise
// AlertTriggeredAnalyzer back to the OSS FindingRecorder adapter.
type AlertAnalyzerFinding struct {
	ID              string
	Key             string
	Severity        string
	Category        string
	ResourceID      string
	ResourceName    string
	ResourceType    string
	Title           string
	Description     string
	Recommendation  string
	Evidence        string
	AlertIdentifier string
	DetectedAt      time.Time
	LastSeenAt      time.Time
}

// FindingRecorder records alert-triggered findings through the patrol pipeline.
// The OSS binary provides an adapter that converts AlertAnalyzerFinding into
// the internal Finding type and calls PatrolService.recordFinding().
type FindingRecorder interface {
	RecordAlertFinding(finding *AlertAnalyzerFinding)
}

// IncidentRecorder records AI analysis events for alerts.
// The OSS binary provides an adapter that delegates to Service.RecordIncidentAnalysis().
type IncidentRecorder interface {
	RecordIncidentAnalysis(alertIdentifier, summary string, details map[string]interface{})
}

// AlertAnalyzerDeps bundles the dependencies that the enterprise
// AlertTriggeredAnalyzer needs from the OSS binary.
type AlertAnalyzerDeps struct {
	FindingRecorder  FindingRecorder
	IncidentRecorder IncidentRecorder
}
