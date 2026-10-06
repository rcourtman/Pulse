package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
)

// Exact d14a3891 pre-change implementation, independent of the new raw-object path.
func referenceJSONMergePatch(previous, current json.RawMessage) (json.RawMessage, error) {
	previousValue, err := referenceDecodeJSONMergeValue(previous)
	if err != nil {
		return nil, err
	}
	currentValue, err := referenceDecodeJSONMergeValue(current)
	if err != nil {
		return nil, err
	}

	patchValue, changed := referenceDiffJSONMergeValue(previousValue, currentValue)
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

// A delta must preserve the authoritative encoded counters, not round them
// through float64. Otherwise adjacent int64/uint64 readings above 2^53 can
// compare equal, and a changed nested value/array can be sent with altered
// numbers. Keep number tokens exact while retaining the existing merge rules.
func referenceDecodeJSONMergeValue(encoded json.RawMessage) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	// Decode accepts a stream by default. Preserve Unmarshal's one-value
	// contract, including rejection of a valid prefix followed by more JSON.
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON merge values")
		}
		return nil, err
	}
	return value, nil
}

func referenceDiffJSONMergeValue(previous, current interface{}) (interface{}, bool) {
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
		if nestedPatch, changed := referenceDiffJSONMergeValue(previousValue, currentValue); changed {
			patch[key] = nestedPatch
		}
	}
	return patch, len(patch) > 0
}

func referenceBuildKeyedArrayDelta(
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
		patch, err := referenceJSONMergePatch(previousEntry, currentEntry)
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

func referenceBuildClientStateDelta(previous, current *clientStateSnapshot) (map[string]interface{}, error) {
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

	resourceDelta, err := referenceBuildKeyedArrayDelta(
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
			keyedDelta, err := referenceBuildKeyedArrayDelta(
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
