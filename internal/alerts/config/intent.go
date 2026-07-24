package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	CurrentAlertIntentPolicySchemaVersion = 1
	maxAlertIntentGraceSeconds            = 30 * 24 * 60 * 60
)

// AlertIntentSignal is a stable policy key shared by configuration, preview,
// and runtime evaluation. Metric signals use the metric.<name> namespace.
type AlertIntentSignal string

const (
	AlertIntentSignalDefault      AlertIntentSignal = "*"
	AlertIntentSignalOffline      AlertIntentSignal = "state.offline"
	AlertIntentSignalAvailability AlertIntentSignal = "incident.availability"
)

// BackupOfflineIntentPolicy extends an offline candidate while Pulse has fresh
// evidence that a Proxmox backup is responsible for the transient state.
type BackupOfflineIntentPolicy struct {
	Enabled            bool `json:"enabled"`
	PostGraceSeconds   int  `json:"postGraceSeconds,omitempty"`
	MaxDeferralSeconds int  `json:"maxDeferralSeconds,omitempty"`
}

// AlertIntentRule is deliberately small and typed. Pointer fields distinguish
// inheritance from an explicit zero/false override.
type AlertIntentRule struct {
	GraceSeconds       *int                       `json:"graceSeconds,omitempty"`
	HonorOperatorState *bool                      `json:"honorOperatorState,omitempty"`
	BackupOffline      *BackupOfflineIntentPolicy `json:"backupOffline,omitempty"`
}

// AlertIntentPolicyDocument contains alert-intent defaults and overrides.
// Resources are keyed by canonical resource ID. ResourceTypes use canonical
// alert resource-type keys. Signal maps accept "*", state.offline,
// incident.availability, and metric.<name> keys.
type AlertIntentPolicyDocument struct {
	SchemaVersion int                                   `json:"schemaVersion"`
	Revision      int64                                 `json:"revision"`
	UpdatedAt     *time.Time                            `json:"updatedAt,omitempty"`
	Defaults      map[string]AlertIntentRule            `json:"defaults,omitempty"`
	ResourceTypes map[string]map[string]AlertIntentRule `json:"resourceTypes,omitempty"`
	Resources     map[string]map[string]AlertIntentRule `json:"resources,omitempty"`
}

func NewAlertIntentPolicyDocument() AlertIntentPolicyDocument {
	return AlertIntentPolicyDocument{
		SchemaVersion: CurrentAlertIntentPolicySchemaVersion,
		Defaults:      make(map[string]AlertIntentRule),
		ResourceTypes: make(map[string]map[string]AlertIntentRule),
		Resources:     make(map[string]map[string]AlertIntentRule),
	}
}

func MetricAlertIntentSignal(metric string) string {
	metric = strings.ToLower(strings.TrimSpace(metric))
	if metric == "" {
		return ""
	}
	return "metric." + metric
}

// NormalizeAlertIntentPolicyDocument returns a detached, canonical document.
func NormalizeAlertIntentPolicyDocument(input AlertIntentPolicyDocument) AlertIntentPolicyDocument {
	out := NewAlertIntentPolicyDocument()
	if input.SchemaVersion > 0 {
		out.SchemaVersion = input.SchemaVersion
	}
	out.Revision = input.Revision
	if input.UpdatedAt != nil {
		updatedAt := input.UpdatedAt.UTC()
		out.UpdatedAt = &updatedAt
	}
	out.Defaults = normalizeIntentSignalRules(input.Defaults)
	out.ResourceTypes = normalizeIntentScopedRules(input.ResourceTypes, true)
	out.Resources = normalizeIntentScopedRules(input.Resources, false)
	return out
}

func normalizeIntentScopedRules(input map[string]map[string]AlertIntentRule, lowerScope bool) map[string]map[string]AlertIntentRule {
	out := make(map[string]map[string]AlertIntentRule, len(input))
	for rawScope, rules := range input {
		scope := strings.TrimSpace(rawScope)
		if lowerScope {
			scope = CanonicalAlertResourceType(scope)
		}
		if scope == "" {
			continue
		}
		normalized := normalizeIntentSignalRules(rules)
		if len(normalized) > 0 {
			out[scope] = normalized
		}
	}
	return out
}

func normalizeIntentSignalRules(input map[string]AlertIntentRule) map[string]AlertIntentRule {
	out := make(map[string]AlertIntentRule, len(input))
	for rawSignal, rule := range input {
		signal := strings.ToLower(strings.TrimSpace(rawSignal))
		if signal == "default" || signal == "_default" {
			signal = string(AlertIntentSignalDefault)
		}
		if signal == "" {
			continue
		}
		out[signal] = cloneAlertIntentRule(rule)
	}
	return out
}

func cloneAlertIntentRule(rule AlertIntentRule) AlertIntentRule {
	out := rule
	if rule.GraceSeconds != nil {
		value := *rule.GraceSeconds
		out.GraceSeconds = &value
	}
	if rule.HonorOperatorState != nil {
		value := *rule.HonorOperatorState
		out.HonorOperatorState = &value
	}
	if rule.BackupOffline != nil {
		value := *rule.BackupOffline
		out.BackupOffline = &value
	}
	return out
}

func ValidateAlertIntentPolicyDocument(input AlertIntentPolicyDocument) error {
	doc := NormalizeAlertIntentPolicyDocument(input)
	if doc.SchemaVersion != CurrentAlertIntentPolicySchemaVersion {
		return fmt.Errorf("unsupported alert intent policy schema version %d", doc.SchemaVersion)
	}
	if doc.Revision < 0 {
		return fmt.Errorf("alert intent policy revision must be non-negative")
	}
	if err := validateIntentSignalRules("defaults", input.Defaults); err != nil {
		return err
	}
	if err := validateIntentScopedRules("resource type", input.ResourceTypes, true); err != nil {
		return err
	}
	if err := validateIntentScopedRules("resource", input.Resources, false); err != nil {
		return err
	}
	return nil
}

func validateIntentScopedRules(label string, scopes map[string]map[string]AlertIntentRule, resourceType bool) error {
	seen := make(map[string]string, len(scopes))
	for rawScope, rules := range scopes {
		scope := strings.TrimSpace(rawScope)
		if resourceType {
			scope = CanonicalAlertResourceType(scope)
		}
		if scope == "" {
			return fmt.Errorf("alert intent %s is required", label)
		}
		if previous, exists := seen[scope]; exists {
			return fmt.Errorf("alert intent %s keys %q and %q normalize to the same value", label, previous, rawScope)
		}
		seen[scope] = rawScope
		if err := validateIntentSignalRules(label+" "+scope, rules); err != nil {
			return err
		}
	}
	return nil
}

func validateIntentSignalRules(scope string, rules map[string]AlertIntentRule) error {
	seen := make(map[string]string, len(rules))
	for rawSignal, rule := range rules {
		signal := strings.ToLower(strings.TrimSpace(rawSignal))
		if signal == "default" || signal == "_default" {
			signal = string(AlertIntentSignalDefault)
		}
		if signal == "" {
			return fmt.Errorf("%s has an empty alert intent signal", scope)
		}
		if previous, exists := seen[signal]; exists {
			return fmt.Errorf("%s signal keys %q and %q normalize to the same value", scope, previous, rawSignal)
		}
		seen[signal] = rawSignal
		if !validAlertIntentSignal(signal) {
			return fmt.Errorf("%s has unsupported alert intent signal %q", scope, signal)
		}
		if rule.GraceSeconds != nil && (*rule.GraceSeconds < 0 || *rule.GraceSeconds > maxAlertIntentGraceSeconds) {
			return fmt.Errorf("%s signal %q graceSeconds must be between 0 and %d", scope, signal, maxAlertIntentGraceSeconds)
		}
		if backup := rule.BackupOffline; backup != nil {
			if signal != string(AlertIntentSignalOffline) && signal != string(AlertIntentSignalDefault) {
				return fmt.Errorf("%s signal %q may not configure backupOffline", scope, signal)
			}
			if backup.PostGraceSeconds < 0 || backup.PostGraceSeconds > maxAlertIntentGraceSeconds {
				return fmt.Errorf("%s signal %q postGraceSeconds must be between 0 and %d", scope, signal, maxAlertIntentGraceSeconds)
			}
			if backup.MaxDeferralSeconds < 0 || backup.MaxDeferralSeconds > maxAlertIntentGraceSeconds {
				return fmt.Errorf("%s signal %q maxDeferralSeconds must be between 0 and %d", scope, signal, maxAlertIntentGraceSeconds)
			}
			if backup.Enabled && backup.MaxDeferralSeconds <= 0 {
				return fmt.Errorf("%s signal %q backupOffline maxDeferralSeconds must be positive when enabled", scope, signal)
			}
		}
	}
	return nil
}

func validAlertIntentSignal(signal string) bool {
	if signal == string(AlertIntentSignalDefault) || signal == string(AlertIntentSignalOffline) || signal == string(AlertIntentSignalAvailability) {
		return true
	}
	return strings.HasPrefix(signal, "metric.") && strings.TrimPrefix(signal, "metric.") != ""
}

// ValidAlertIntentSignal reports whether signal is accepted by both persisted
// policy rules and preview requests.
func ValidAlertIntentSignal(signal string) bool {
	return validAlertIntentSignal(strings.ToLower(strings.TrimSpace(signal)))
}
