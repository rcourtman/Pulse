package unifiedresources

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// AvailabilityChecksForResource returns the complete canonical availability
// facet set. The singular Availability field is retained as an additive
// compatibility summary and is folded into the result when older payloads do
// not yet carry AvailabilityChecks.
func AvailabilityChecksForResource(resource Resource) []AvailabilityData {
	return mergeAvailabilityChecks(nil, nil, resource.AvailabilityChecks, resource.Availability)
}

// AvailabilityCheckByTargetID returns the observation facet associated with a
// provider target. Alert and evidence consumers use this instead of assuming
// the compatibility summary is the incident's originating check.
func AvailabilityCheckByTargetID(resource Resource, targetID string) *AvailabilityData {
	targetID = strings.TrimSpace(targetID)
	for _, check := range AvailabilityChecksForResource(resource) {
		if strings.TrimSpace(check.TargetID) == targetID {
			cloned := cloneAvailabilityData(&check)
			return cloned
		}
	}
	return nil
}

// availabilityChecksProveOnline reports whether any availability check
// projected onto a monitored resource proves, at now, that the resource
// answers. Only a passing check with current evidence does. Each check is
// judged by its own evidence window (two poll intervals for a local check,
// the report window for a remote probe), so one check's recent run cannot
// vouch for another check's old pass, and a local check that missed its
// cadence keeps Available but proves nothing. A failing check proves only
// that one port or service does not answer, so it never decides the
// resource's status at all: not online, offline or warning, current or quiet.
func availabilityChecksProveOnline(checks []AvailabilityData, now time.Time) bool {
	for _, check := range checks {
		if check.Enabled && check.Available && check.Evidence != nil &&
			check.Evidence.FreshnessAt(now) == operationaltrust.EvidenceFresh {
			return true
		}
	}
	return false
}

func normalizeResourceAvailability(resource *Resource) {
	if resource == nil {
		return
	}
	resource.AvailabilityChecks = mergeAvailabilityChecks(
		nil,
		nil,
		resource.AvailabilityChecks,
		resource.Availability,
	)
	resource.Availability = primaryAvailabilityCheck(resource.AvailabilityChecks)
}

func mergeAvailabilityChecks(
	existing []AvailabilityData,
	existingPrimary *AvailabilityData,
	incoming []AvailabilityData,
	incomingPrimary *AvailabilityData,
) []AvailabilityData {
	byKey := make(map[string]AvailabilityData, len(existing)+len(incoming)+2)
	add := func(check AvailabilityData) {
		key := availabilityCheckKey(check)
		if key == "" {
			return
		}
		cloned := cloneAvailabilityData(&check)
		if cloned != nil {
			byKey[key] = *cloned
		}
	}
	for _, check := range existing {
		add(check)
	}
	if existingPrimary != nil {
		add(*existingPrimary)
	}
	for _, check := range incoming {
		add(check)
	}
	if incomingPrimary != nil {
		add(*incomingPrimary)
	}
	if len(byKey) == 0 {
		return nil
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]AvailabilityData, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func availabilityCheckKey(check AvailabilityData) string {
	if targetID := strings.TrimSpace(check.TargetID); targetID != "" {
		return "target:" + targetID
	}
	address := strings.ToLower(strings.TrimSpace(check.Address))
	protocol := strings.ToLower(strings.TrimSpace(check.Protocol))
	path := strings.TrimSpace(check.Path)
	if address == "" && protocol == "" && check.Port == 0 && path == "" {
		return ""
	}
	return fmt.Sprintf("endpoint:%s:%s:%d:%s", protocol, address, check.Port, path)
}

// primaryAvailabilityCheck selects the singular compatibility summary: the
// worst check, ties going to the first by key. Health and row presentation
// read only the summary, so a confirmed outage on any check must outrank a
// failure that has not yet reached its threshold.
func primaryAvailabilityCheck(checks []AvailabilityData) *AvailabilityData {
	if len(checks) == 0 {
		return nil
	}
	best := 0
	for index := 1; index < len(checks); index++ {
		if availabilityCheckPriority(checks[index]) < availabilityCheckPriority(checks[best]) {
			best = index
		}
	}
	return cloneAvailabilityData(&checks[best])
}

func availabilityCheckPriority(check AvailabilityData) int {
	if availabilityOutageConfirmed(check) {
		return 0
	}
	if check.LastChecked != nil && !check.Available {
		return 1
	}
	if check.LastChecked == nil ||
		check.CorrelationState == AvailabilityCorrelationAmbiguous ||
		check.CorrelationState == AvailabilityCorrelationUnresolved {
		return 2
	}
	return 3
}

// availabilityOutageConfirmed reports whether an enabled check has failed at
// least its failure threshold in a row, by the same gate the availability
// poller uses before raising availability_unreachable: an observed failure
// whose aggregate, when present, is unavailable. A probe agent that stops
// reporting keeps its failure count but reads indeterminate with an unknown
// aggregate, and confirms nothing.
func availabilityOutageConfirmed(check AvailabilityData) bool {
	if !check.Enabled || check.Available || check.LastChecked == nil {
		return false
	}
	if state := strings.TrimSpace(check.AggregateState); state != "" && !strings.EqualFold(state, "unavailable") {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(check.ProbeOutcome), "indeterminate") {
		return false
	}
	threshold := check.FailureThreshold
	if threshold <= 0 {
		threshold = 1
	}
	return check.ConsecutiveFailures >= threshold
}
