package unifiedresources

import (
	"fmt"
	"sort"
	"strings"
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
	if check.LastChecked != nil && !check.Available {
		return 0
	}
	if check.LastChecked == nil ||
		check.CorrelationState == AvailabilityCorrelationAmbiguous ||
		check.CorrelationState == AvailabilityCorrelationUnresolved {
		return 1
	}
	return 2
}
