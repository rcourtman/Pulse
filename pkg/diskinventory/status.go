package diskinventory

import "strings"

// FieldState describes whether a physical-disk field was observed during the
// current collection pass. It deliberately separates provider limitations
// from transient collection failures and from fields that should have been
// present but were not.
type FieldState string

const (
	FieldAvailable   FieldState = "available"
	FieldUnavailable FieldState = "unavailable"
	FieldUnsupported FieldState = "unsupported"
	FieldMissing     FieldState = "missing"
)

// LegacyHostAgentSource is the provenance recorded for a host agent field
// reported before collection provenance existed (agents before 6.2). Those
// agents send only readings they collected.
const LegacyHostAgentSource = "host_agent"

// LegacyHostAgentStatus returns a host agent field's collection state with
// the legacy agent's provenance filled in. A report from before collection
// provenance carries no state, so a present reading was collected from
// LegacyHostAgentSource, and a state recorded for such a report without a
// source (its lease-expiry withdrawal) is that source's too. Monitoring
// stamps the legacy source on the Proxmox disk's copy of the reading and the
// unified-resources registry matches the agent's withdrawal to that copy, so
// both apply this one rule.
func LegacyHostAgentStatus(status FieldStatus, hasReading bool) FieldStatus {
	switch {
	case status.State == "" && hasReading:
		return Available(LegacyHostAgentSource)
	case status.State != "" && strings.TrimSpace(status.Source) == "":
		status.Source = LegacyHostAgentSource
	}
	return status
}

// FieldStatus carries collection state and provenance for one disk signal.
type FieldStatus struct {
	State  FieldState `json:"state"`
	Source string     `json:"source,omitempty"`
	Reason string     `json:"reason,omitempty"`
}

// CollectionStatus is the field-level payload carried by collection evidence;
// it is not a separate trust posture or mutation lifecycle. Operational trust
// may wrap this payload in its shared EvidenceEnvelope, while Collection Trust
// remains responsible for evaluating the observation. Empty means the report
// predates this contract.
type CollectionStatus struct {
	Serial      FieldStatus `json:"serial,omitempty"`
	Temperature FieldStatus `json:"temperature,omitempty"`
	IO          FieldStatus `json:"io,omitempty"`
	Controller  FieldStatus `json:"controller,omitempty"`
	Pool        FieldStatus `json:"pool,omitempty"`
}

func Available(source string) FieldStatus {
	return FieldStatus{State: FieldAvailable, Source: strings.TrimSpace(source)}
}

func Unavailable(source, reason string) FieldStatus {
	return FieldStatus{
		State:  FieldUnavailable,
		Source: strings.TrimSpace(source),
		Reason: strings.TrimSpace(reason),
	}
}

func Unsupported(source, reason string) FieldStatus {
	return FieldStatus{
		State:  FieldUnsupported,
		Source: strings.TrimSpace(source),
		Reason: strings.TrimSpace(reason),
	}
}

func Missing(source, reason string) FieldStatus {
	return FieldStatus{
		State:  FieldMissing,
		Source: strings.TrimSpace(source),
		Reason: strings.TrimSpace(reason),
	}
}

func CloneStatus(status *CollectionStatus) *CollectionStatus {
	if status == nil {
		return nil
	}
	clone := *status
	return &clone
}

// TemperatureCollected reports whether a disk temperature was collected by the
// observation that carries it. Normalization may keep a last-known temperature
// it did not collect (a disk in standby, a host agent past its reporting
// lease) under a non-available state; that value must not be recorded or
// presented as a current reading. A temperature without collection state
// predates this contract and counts as collected.
func TemperatureCollected(temperature int, status *CollectionStatus) bool {
	if temperature <= 0 {
		return false
	}
	if status == nil {
		return true
	}
	state := status.Temperature.State
	return state == "" || state == FieldAvailable
}

// MergeStatus keeps an available observation over a weaker state while still
// allowing a current available observation to replace older provenance.
func MergeStatus(existing, incoming *CollectionStatus) *CollectionStatus {
	if existing == nil {
		return CloneStatus(incoming)
	}
	if incoming == nil {
		return CloneStatus(existing)
	}
	merged := *existing
	merged.Serial = mergeFieldStatus(merged.Serial, incoming.Serial)
	merged.Temperature = mergeFieldStatus(merged.Temperature, incoming.Temperature)
	merged.IO = mergeFieldStatus(merged.IO, incoming.IO)
	merged.Controller = mergeFieldStatus(merged.Controller, incoming.Controller)
	merged.Pool = mergeFieldStatus(merged.Pool, incoming.Pool)
	return &merged
}

// MergeReportedStatus merges a source's own report into existing state that
// may still hold an earlier copy of that report. It behaves like MergeStatus,
// except that a field the report no longer marks available supersedes an
// available state that existing carries from the same source: the source's
// later word on its own field wins, so a stale copy cannot keep claiming the
// field was collected.
func MergeReportedStatus(existing, reported *CollectionStatus) *CollectionStatus {
	merged := MergeStatus(existing, reported)
	if existing == nil || reported == nil {
		return merged
	}
	merged.Serial = supersedeFieldStatus(merged.Serial, existing.Serial, reported.Serial)
	merged.Temperature = supersedeFieldStatus(merged.Temperature, existing.Temperature, reported.Temperature)
	merged.IO = supersedeFieldStatus(merged.IO, existing.IO, reported.IO)
	merged.Controller = supersedeFieldStatus(merged.Controller, existing.Controller, reported.Controller)
	merged.Pool = supersedeFieldStatus(merged.Pool, existing.Pool, reported.Pool)
	return merged
}

func supersedeFieldStatus(merged, existing, reported FieldStatus) FieldStatus {
	source := strings.TrimSpace(existing.Source)
	if existing.State == FieldAvailable &&
		reported.State != "" && reported.State != FieldAvailable &&
		source != "" && strings.EqualFold(source, strings.TrimSpace(reported.Source)) {
		return reported
	}
	return merged
}

func mergeFieldStatus(existing, incoming FieldStatus) FieldStatus {
	if incoming.State == "" {
		return existing
	}
	if existing.State == FieldAvailable && incoming.State != FieldAvailable {
		return existing
	}
	return incoming
}
