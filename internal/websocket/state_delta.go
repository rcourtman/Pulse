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

func extractKeyedEntries(
	encoded json.RawMessage,
	field string,
) (map[string]json.RawMessage, []string, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(encoded, &entries); err != nil {
		return nil, nil, fmt.Errorf("decode state %s: %w", field, err)
	}
	// Decode all identities in one pass rather than allocating a decoder and
	// throwaway struct for every entry. Keys must come from the encoded entries,
	// not hints from the source value: checking one hinted ID cannot establish
	// the identity of the rest, especially if a marshaler changes their order.
	identities := make([]struct {
		ID string `json:"id"`
	}, len(entries))
	if err := json.Unmarshal(encoded, &identities); err != nil {
		return nil, nil, fmt.Errorf("decode state %s identity: %w", field, err)
	}

	byID := make(map[string]json.RawMessage, len(entries))
	order := make([]string, 0, len(entries))
	for i, entry := range entries {
		id := identities[i].ID
		if id == "" {
			return nil, nil, fmt.Errorf("state %s entry is missing id", field)
		}
		if _, exists := byID[id]; exists {
			return nil, nil, fmt.Errorf("state %s id %q is duplicated", field, id)
		}
		// RawMessage.UnmarshalJSON already copied each entry into its own buffer.
		// Retaining that buffer is safe; copying it a second time is redundant.
		byID[id] = entry
		order = append(order, id)
	}
	return byID, order, nil
}

func buildClientStateSnapshot(state interface{}) (*clientStateSnapshot, error) {
	// Unknown shapes and custom marshalers retain the generic wire-authoritative
	// path. Only the concrete frontend projection can avoid the whole resources
	// array's encode/decode/copy round trip.
	if _, custom := state.(json.Marshaler); !custom {
		switch frontend := state.(type) {
		case models.StateFrontend:
			return buildFrontendStateSnapshot(frontend)
		case *models.StateFrontend:
			if frontend != nil {
				return buildFrontendStateSnapshot(*frontend)
			}
		}
	}
	return buildGenericClientStateSnapshot(state)
}

func buildFrontendStateSnapshot(state models.StateFrontend) (*clientStateSnapshot, error) {
	resources := state.Resources
	// Encode the rest through the existing generic path so future top-level
	// fields, omitempty, nil/empty arrays and keyed-field fallback stay identical.
	// This is a local value copy; no source slices/maps are modified or cached.
	state.Resources = nil
	snapshot, err := buildGenericClientStateSnapshot(state)
	if err != nil {
		return nil, err
	}
	snapshot.resources = make(map[string]json.RawMessage, len(resources))
	snapshot.resourceOrder = make([]string, 0, len(resources))
	for i := range resources {
		encoded, err := json.Marshal(&resources[i])
		if err != nil {
			return nil, fmt.Errorf("marshal state resource: %w", err)
		}
		// Identity still comes from EVERY encoded entry, never a source-ID hint.
		// json.Marshal owns this buffer; it can be retained directly as immutable
		// snapshot data without whole-state and RawMessage decoding copies.
		var identity struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(encoded, &identity); err != nil {
			return nil, fmt.Errorf("decode state resource identity: %w", err)
		}
		if identity.ID == "" {
			return nil, fmt.Errorf("state resource entry is missing id")
		}
		if _, exists := snapshot.resources[identity.ID]; exists {
			return nil, fmt.Errorf("state resource id %q is duplicated", identity.ID)
		}
		snapshot.resources[identity.ID] = encoded
		snapshot.resourceOrder = append(snapshot.resourceOrder, identity.ID)
	}
	return snapshot, nil
}

func buildGenericClientStateSnapshot(state interface{}) (*clientStateSnapshot, error) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshal state snapshot: %w", err)
	}

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("decode state snapshot: %w", err)
	}

	resources := make(map[string]json.RawMessage)
	resourceOrder := make([]string, 0)
	if encodedResources, ok := fields["resources"]; ok {
		resources, resourceOrder, err = extractKeyedEntries(encodedResources, "resource")
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

	for _, keyedField := range keyedDeltaFields {
		encodedField, ok := fields[keyedField.field]
		if !ok {
			continue
		}
		entries, order, keyErr := extractKeyedEntries(encodedField, keyedField.field)
		if keyErr != nil {
			continue
		}
		snapshot.keyed[keyedField.field] = &keyedFieldSnapshot{
			entries: entries,
			order:   order,
			raw:     encodedField,
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
