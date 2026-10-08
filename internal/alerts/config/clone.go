package config

import (
	"maps"
	"slices"
)

// Clone returns a deep copy of the configuration that shares no maps, slices
// or pointers with c, so the copy can be changed, normalized or encoded
// without touching c. Nil and empty maps and slices keep their distinction.
// Custom rule filter values are copied as the JSON values the API and
// alerts.json decode them to; other Go values stored there stay shared.
//
// The alert manager hands out its live configuration only through Clone;
// callers persist, encode and edit what it returns outside the manager lock.
func (c AlertConfig) Clone() AlertConfig {
	clone := c
	clone.ActivationTime = clonePointer(c.ActivationTime)
	clone.GuestDefaults = c.GuestDefaults.Clone()
	clone.NodeDefaults = c.NodeDefaults.Clone()
	clone.AgentDefaults = c.AgentDefaults.Clone()
	clone.DiskFillByType = maps.Clone(c.DiskFillByType)
	clone.DiskTempByType = maps.Clone(c.DiskTempByType)
	clone.DockerIgnoredContainerPrefixes = slices.Clone(c.DockerIgnoredContainerPrefixes)
	clone.IgnoredGuestPrefixes = slices.Clone(c.IgnoredGuestPrefixes)
	clone.GuestTagWhitelist = slices.Clone(c.GuestTagWhitelist)
	clone.GuestTagBlacklist = slices.Clone(c.GuestTagBlacklist)
	clone.PBSDefaults = c.PBSDefaults.Clone()
	clone.KubernetesDefaults = c.KubernetesDefaults.Clone()
	clone.TrueNASDefaults = c.TrueNASDefaults.Clone()
	clone.TrueNASDiskDefaults = c.TrueNASDiskDefaults.Clone()
	clone.VMwareDefaults = c.VMwareDefaults.Clone()
	clone.BackupDefaults = c.BackupDefaults.clone()
	if c.Overrides != nil {
		clone.Overrides = make(map[string]ThresholdConfig, len(c.Overrides))
		for id, override := range c.Overrides {
			clone.Overrides[id] = override.Clone()
		}
	}
	if c.CustomRules != nil {
		clone.CustomRules = make([]CustomAlertRule, len(c.CustomRules))
		for i, rule := range c.CustomRules {
			clone.CustomRules[i] = rule.clone()
		}
	}
	clone.Schedule.QuietHours.Days = maps.Clone(c.Schedule.QuietHours.Days)
	if c.Schedule.Escalation.Levels != nil {
		clone.Schedule.Escalation.Levels = make([]EscalationLevel, len(c.Schedule.Escalation.Levels))
		for i, level := range c.Schedule.Escalation.Levels {
			level.DestinationIDs = slices.Clone(level.DestinationIDs)
			clone.Schedule.Escalation.Levels[i] = level
		}
	}
	clone.TimeThresholds = maps.Clone(c.TimeThresholds)
	clone.MetricTimeThresholds = cloneNestedIntMap(c.MetricTimeThresholds)
	clone.MetricEvaluationWindows = cloneNestedIntMap(c.MetricEvaluationWindows)
	return clone
}

// Clone returns a deep copy of the thresholds that shares no pointers with t.
func (t ThresholdConfig) Clone() ThresholdConfig {
	clone := t
	clone.CPU = clonePointer(t.CPU)
	clone.Memory = clonePointer(t.Memory)
	clone.Disk = clonePointer(t.Disk)
	clone.DiskRead = clonePointer(t.DiskRead)
	clone.DiskWrite = clonePointer(t.DiskWrite)
	clone.NetworkIn = clonePointer(t.NetworkIn)
	clone.NetworkOut = clonePointer(t.NetworkOut)
	clone.Usage = clonePointer(t.Usage)
	clone.Temperature = clonePointer(t.Temperature)
	clone.DiskTemperature = clonePointer(t.DiskTemperature)
	clone.SMARTHealthFailure = clonePointer(t.SMARTHealthFailure)
	clone.SMARTReallocated = clonePointer(t.SMARTReallocated)
	clone.SMARTPending = clonePointer(t.SMARTPending)
	clone.SMARTUncorrectable = clonePointer(t.SMARTUncorrectable)
	clone.SMARTMediaErrors = clonePointer(t.SMARTMediaErrors)
	clone.SMARTCRCErrorDelta = clonePointer(t.SMARTCRCErrorDelta)
	clone.SMARTLifeWarning = clonePointer(t.SMARTLifeWarning)
	clone.SMARTLifeCritical = clonePointer(t.SMARTLifeCritical)
	clone.SMARTSpareWarning = clonePointer(t.SMARTSpareWarning)
	clone.SMARTSpareCritical = clonePointer(t.SMARTSpareCritical)
	if t.Backup != nil {
		backup := t.Backup.clone()
		clone.Backup = &backup
	}
	clone.Snapshot = clonePointer(t.Snapshot)
	clone.Note = clonePointer(t.Note)
	return clone
}

func (b BackupAlertConfig) clone() BackupAlertConfig {
	clone := b
	clone.AlertOrphaned = clonePointer(b.AlertOrphaned)
	clone.IgnoreVMIDs = slices.Clone(b.IgnoreVMIDs)
	return clone
}

func (r CustomAlertRule) clone() CustomAlertRule {
	clone := r
	if r.FilterConditions.Filters != nil {
		clone.FilterConditions.Filters = make([]FilterCondition, len(r.FilterConditions.Filters))
		for i, filter := range r.FilterConditions.Filters {
			filter.Value = cloneJSONValue(filter.Value)
			clone.FilterConditions.Filters[i] = filter
		}
	}
	clone.Thresholds = r.Thresholds.Clone()
	if r.Notifications.Email != nil {
		email := *r.Notifications.Email
		email.Recipients = slices.Clone(email.Recipients)
		clone.Notifications.Email = &email
	}
	clone.Notifications.Webhook = clonePointer(r.Notifications.Webhook)
	return clone
}

// clonePointer copies the value p points at. Only use it for types that hold
// no maps, slices or pointers of their own.
func clonePointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	clone := *p
	return &clone
}

func cloneNestedIntMap(input map[string]map[string]int) map[string]map[string]int {
	if input == nil {
		return nil
	}
	clone := make(map[string]map[string]int, len(input))
	for key, inner := range input {
		clone[key] = maps.Clone(inner)
	}
	return clone
}

// cloneJSONValue copies a filter value as encoding/json decodes one: arrays
// and objects are copied, scalars are values already.
func cloneJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case []interface{}:
		if typed == nil {
			return typed
		}
		clone := make([]interface{}, len(typed))
		for i, item := range typed {
			clone[i] = cloneJSONValue(item)
		}
		return clone
	case map[string]interface{}:
		if typed == nil {
			return typed
		}
		clone := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			clone[key] = cloneJSONValue(item)
		}
		return clone
	default:
		return value
	}
}
