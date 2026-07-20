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

func mergeFieldStatus(existing, incoming FieldStatus) FieldStatus {
	if incoming.State == "" {
		return existing
	}
	if existing.State == FieldAvailable && incoming.State != FieldAvailable {
		return existing
	}
	return incoming
}
