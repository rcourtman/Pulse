package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

const resourceDeltaField = "resourceDelta"
const infrastructureField = "connectedInfrastructure"
const infrastructureDeltaField = "connectedInfrastructureDelta"

type clientStateSnapshot struct {
	fields        map[string]json.RawMessage
	resources     map[string]json.RawMessage
	resourceOrder []string
	// The connected-infrastructure projection is keyed the same way as
	// resources so per-item merge patches replace re-shipping the whole
	// projection on every broadcast. When the payload cannot be keyed (an
	// entry without an id), it stays in fields and diffs whole, so the
	// keyed path can never corrupt the projection.
	infrastructure      map[string]json.RawMessage
	infrastructureOrder []string
	infrastructureRaw   json.RawMessage
	infrastructureKeyed bool
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
	byID := make(map[string]json.RawMessage)
	order := make([]string, 0, len(entries))
	for _, entry := range entries {
		var identity struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(entry, &identity); err != nil {
			return nil, nil, fmt.Errorf("decode state %s identity: %w", field, err)
		}
		if identity.ID == "" {
			return nil, nil, fmt.Errorf("state %s entry is missing id", field)
		}
		if _, exists := byID[identity.ID]; exists {
			return nil, nil, fmt.Errorf("state %s id %q is duplicated", field, identity.ID)
		}
		byID[identity.ID] = append(json.RawMessage(nil), entry...)
		order = append(order, identity.ID)
	}
	return byID, order, nil
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
	}

	if encodedInfrastructure, ok := fields[infrastructureField]; ok {
		infrastructure, infrastructureOrder, keyErr := extractKeyedEntries(
			encodedInfrastructure,
			infrastructureField,
		)
		if keyErr == nil {
			snapshot.infrastructure = infrastructure
			snapshot.infrastructureOrder = infrastructureOrder
			snapshot.infrastructureRaw = append(json.RawMessage(nil), encodedInfrastructure...)
			snapshot.infrastructureKeyed = true
			delete(fields, infrastructureField)
		}
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

	switch {
	case previous.infrastructureKeyed && current.infrastructureKeyed:
		infrastructureDelta, err := buildKeyedArrayDelta(
			previous.infrastructure,
			current.infrastructure,
			previous.infrastructureOrder,
			current.infrastructureOrder,
			infrastructureField,
		)
		if err != nil {
			return nil, err
		}
		if !infrastructureDelta.isEmpty() {
			delta[infrastructureDeltaField] = infrastructureDelta
		}
	case current.infrastructureKeyed:
		// The previous snapshot carried the projection as a plain field (or
		// not at all): fall back to shipping the full array once so the
		// client re-adopts a clean baseline.
		if previousRaw, ok := previous.fields[infrastructureField]; !ok ||
			!bytes.Equal(previousRaw, current.infrastructureRaw) {
			delta[infrastructureField] = current.infrastructureRaw
		}
	case previous.infrastructureKeyed:
		// The projection left the keyed path. If the current snapshot still
		// carries it as a plain field, the fields loop above already shipped
		// it whole; if it vanished entirely, clear it like any removed field.
		if _, ok := current.fields[infrastructureField]; !ok {
			delta[infrastructureField] = nil
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
