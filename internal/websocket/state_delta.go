package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const resourceDeltaField = "resourceDelta"
const infrastructureField = "connectedInfrastructure"
const infrastructureDeltaField = "connectedInfrastructureDelta"
const activeAlertsField = "activeAlerts"
const activeAlertsDeltaField = "activeAlertsDelta"

// Keyed delta fields are id-keyed arrays that travel as per-item merge
// patches once a client baseline exists, instead of re-shipping the whole
// array on every broadcast. When a payload cannot be keyed (an entry without
// an id), it stays in fields and diffs whole, so the keyed path can never
// corrupt the projection.
var keyedDeltaFields = []struct {
	field      string
	deltaField string
}{
	{infrastructureField, infrastructureDeltaField},
	{activeAlertsField, activeAlertsDeltaField},
}

type keyedFieldSnapshot struct {
	entries map[string]json.RawMessage
	order   []string
	raw     json.RawMessage
}

type clientStateSnapshot struct {
	fields        map[string]json.RawMessage
	resources     map[string]json.RawMessage
	resourceOrder []string
	// Keyed snapshots by field name; a field is present only when every
	// entry keyed by id.
	keyed map[string]*keyedFieldSnapshot
}

type resourceDeltaPayload struct {
	Upserts []json.RawMessage `json:"upserts,omitempty"`
	Removed []string          `json:"removed,omitempty"`
	Order   []string          `json:"order,omitempty"`
}

func decodeEntryID(entry json.RawMessage, field string) (string, error) {
	var identity struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(entry, &identity); err != nil {
		return "", fmt.Errorf("decode state %s identity: %w", field, err)
	}
	return identity.ID, nil
}

// extractKeyedEntries splits an encoded array into per-entry RawMessages
// keyed by id. knownIDs, when its length matches the decoded entry count,
// carries ids already known from the Go value that was marshaled into
// encoded - encoding/json marshals a slice in its original order, so
// knownIDs[i] is entries[i]'s id without needing to decode that entry just
// to find it. One entry (the first) is always verified against its knownIDs
// counterpart before the rest are trusted; on any mismatch every entry falls
// back to being decoded directly, exactly as if knownIDs had not been
// supplied. This runs on every current-state broadcast, and on a busy
// instance the array here can hold well over a thousand entries, so skipping
// a redundant per-entry decode is worth doing - but never at the cost of
// silently mis-keying a client's delta baseline.
func extractKeyedEntries(
	encoded json.RawMessage,
	field string,
	knownIDs []string,
) (map[string]json.RawMessage, []string, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(encoded, &entries); err != nil {
		return nil, nil, fmt.Errorf("decode state %s: %w", field, err)
	}

	useKnownIDs := len(knownIDs) == len(entries) && len(entries) > 0
	if useKnownIDs {
		firstID, err := decodeEntryID(entries[0], field)
		if err != nil {
			return nil, nil, err
		}
		useKnownIDs = firstID == knownIDs[0]
	}

	byID := make(map[string]json.RawMessage, len(entries))
	order := make([]string, 0, len(entries))
	for i, entry := range entries {
		id := ""
		if useKnownIDs {
			id = knownIDs[i]
		} else {
			var err error
			id, err = decodeEntryID(entry, field)
			if err != nil {
				return nil, nil, err
			}
		}
		if id == "" {
			return nil, nil, fmt.Errorf("state %s entry is missing id", field)
		}
		if _, exists := byID[id]; exists {
			return nil, nil, fmt.Errorf("state %s id %q is duplicated", field, id)
		}
		byID[id] = append(json.RawMessage(nil), entry...)
		order = append(order, id)
	}
	return byID, order, nil
}

// knownEntryIDs returns state's resource/infrastructure/alert ids in
// marshal order when state is the concrete frontend type, so
// extractKeyedEntries can skip re-deriving them from JSON. It returns three
// nil slices for any other state shape (mock/test payloads, etc.), which
// extractKeyedEntries treats as "no known ids" and decodes normally.
func knownEntryIDs(state interface{}) (resourceIDs, infrastructureIDs, alertIDs []string) {
	typed, ok := state.(models.StateFrontend)
	if !ok {
		if ptr, ptrOK := state.(*models.StateFrontend); ptrOK && ptr != nil {
			typed, ok = *ptr, true
		}
	}
	if !ok {
		return nil, nil, nil
	}

	resourceIDs = make([]string, len(typed.Resources))
	for i, r := range typed.Resources {
		resourceIDs[i] = r.ID
	}
	infrastructureIDs = make([]string, len(typed.ConnectedInfrastructure))
	for i, item := range typed.ConnectedInfrastructure {
		infrastructureIDs[i] = item.ID
	}
	alertIDs = make([]string, len(typed.ActiveAlerts))
	for i, a := range typed.ActiveAlerts {
		alertIDs[i] = a.ID
	}
	return resourceIDs, infrastructureIDs, alertIDs
}

func buildClientStateSnapshot(state interface{}) (*clientStateSnapshot, error) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshal state snapshot: %w", err)
	}

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("decode state snapshot: %w", err)
	}

	resourceIDs, infrastructureIDs, alertIDs := knownEntryIDs(state)

	resources := make(map[string]json.RawMessage)
	resourceOrder := make([]string, 0)
	if encodedResources, ok := fields["resources"]; ok {
		resources, resourceOrder, err = extractKeyedEntries(encodedResources, "resource", resourceIDs)
		if err != nil {
			return nil, err
		}
		delete(fields, "resources")
	}

	snapshot := &clientStateSnapshot{
		fields:        fields,
		resources:     resources,
		resourceOrder: resourceOrder,
		keyed:         make(map[string]*keyedFieldSnapshot),
	}

	knownIDsByField := map[string][]string{
		infrastructureField: infrastructureIDs,
		activeAlertsField:   alertIDs,
	}
	for _, keyedField := range keyedDeltaFields {
		encodedField, ok := fields[keyedField.field]
		if !ok {
			continue
		}
		entries, order, keyErr := extractKeyedEntries(encodedField, keyedField.field, knownIDsByField[keyedField.field])
		if keyErr != nil {
			continue
		}
		snapshot.keyed[keyedField.field] = &keyedFieldSnapshot{
			entries: entries,
			order:   order,
			raw:     append(json.RawMessage(nil), encodedField...),
		}
		delete(fields, keyedField.field)
	}

	return snapshot, nil
}

func buildKeyedArrayDelta(
	previousEntries, currentEntries map[string]json.RawMessage,
	previousOrder, currentOrder []string,
	label string,
) (resourceDeltaPayload, error) {
	payload := resourceDeltaPayload{}
	for _, id := range currentOrder {
		currentEntry := currentEntries[id]
		previousEntry, exists := previousEntries[id]
		if !exists {
			payload.Upserts = append(payload.Upserts, currentEntry)
			continue
		}
		if bytes.Equal(previousEntry, currentEntry) {
			continue
		}
		patch, err := createJSONMergePatch(previousEntry, currentEntry)
		if err != nil {
			return resourceDeltaPayload{}, fmt.Errorf("build %s %q patch: %w", label, id, err)
		}
		payload.Upserts = append(payload.Upserts, patch)
	}
	for id := range previousEntries {
		if _, exists := currentEntries[id]; !exists {
			payload.Removed = append(payload.Removed, id)
		}
	}
	sort.Strings(payload.Removed)
	if !reflect.DeepEqual(previousOrder, currentOrder) {
		payload.Order = append([]string(nil), currentOrder...)
	}
	return payload, nil
}

func (p resourceDeltaPayload) isEmpty() bool {
	return len(p.Upserts) == 0 && len(p.Removed) == 0 && len(p.Order) == 0
}

func buildClientStateDelta(previous, current *clientStateSnapshot) (map[string]interface{}, error) {
	if previous == nil || current == nil {
		return nil, fmt.Errorf("state delta requires previous and current snapshots")
	}

	delta := make(map[string]interface{})
	for key, currentValue := range current.fields {
		if previousValue, ok := previous.fields[key]; !ok || !bytes.Equal(previousValue, currentValue) {
			delta[key] = currentValue
		}
	}
	for key := range previous.fields {
		if _, ok := current.fields[key]; !ok {
			delta[key] = nil
		}
	}

	resourceDelta, err := buildKeyedArrayDelta(
		previous.resources,
		current.resources,
		previous.resourceOrder,
		current.resourceOrder,
		"resource",
	)
	if err != nil {
		return nil, err
	}
	if !resourceDelta.isEmpty() {
		delta[resourceDeltaField] = resourceDelta
	}

	for _, keyedField := range keyedDeltaFields {
		previousSnapshot := previous.keyed[keyedField.field]
		currentSnapshot := current.keyed[keyedField.field]
		switch {
		case previousSnapshot != nil && currentSnapshot != nil:
			keyedDelta, err := buildKeyedArrayDelta(
				previousSnapshot.entries,
				currentSnapshot.entries,
				previousSnapshot.order,
				currentSnapshot.order,
				keyedField.field,
			)
			if err != nil {
				return nil, err
			}
			if !keyedDelta.isEmpty() {
				delta[keyedField.deltaField] = keyedDelta
			}
		case currentSnapshot != nil:
			// The previous snapshot carried the array as a plain field (or
			// not at all): fall back to shipping the full array once so the
			// client re-adopts a clean baseline.
			if previousRaw, ok := previous.fields[keyedField.field]; !ok ||
				!bytes.Equal(previousRaw, currentSnapshot.raw) {
				delta[keyedField.field] = currentSnapshot.raw
			}
		case previousSnapshot != nil:
			// The array left the keyed path. If the current snapshot still
			// carries it as a plain field, the fields loop above already
			// shipped it whole; if it vanished entirely, clear it like any
			// removed field.
			if _, ok := current.fields[keyedField.field]; !ok {
				delta[keyedField.field] = nil
			}
		}
	}

	return delta, nil
}

func createJSONMergePatch(previous, current json.RawMessage) (json.RawMessage, error) {
	var previousValue interface{}
	if err := json.Unmarshal(previous, &previousValue); err != nil {
		return nil, err
	}
	var currentValue interface{}
	if err := json.Unmarshal(current, &currentValue); err != nil {
		return nil, err
	}

	patchValue, changed := diffJSONMergeValue(previousValue, currentValue)
	if !changed {
		return json.RawMessage(`{}`), nil
	}
	if patchObject, ok := patchValue.(map[string]interface{}); ok {
		if currentObject, ok := currentValue.(map[string]interface{}); ok {
			if id, ok := currentObject["id"]; ok {
				patchObject["id"] = id
			}
		}
	}
	patch, err := json.Marshal(patchValue)
	if err != nil {
		return nil, err
	}
	return patch, nil
}

func diffJSONMergeValue(previous, current interface{}) (interface{}, bool) {
	if reflect.DeepEqual(previous, current) {
		return nil, false
	}

	previousObject, previousIsObject := previous.(map[string]interface{})
	currentObject, currentIsObject := current.(map[string]interface{})
	if !previousIsObject || !currentIsObject {
		return current, true
	}

	patch := make(map[string]interface{})
	for key := range previousObject {
		if _, exists := currentObject[key]; !exists {
			patch[key] = nil
		}
	}
	for key, currentValue := range currentObject {
		previousValue, exists := previousObject[key]
		if !exists {
			patch[key] = currentValue
			continue
		}
		if nestedPatch, changed := diffJSONMergeValue(previousValue, currentValue); changed {
			patch[key] = nestedPatch
		}
	}
	return patch, len(patch) > 0
}
