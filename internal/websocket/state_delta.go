package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

const resourceDeltaField = "resourceDelta"

type clientStateSnapshot struct {
	fields        map[string]json.RawMessage
	resources     map[string]json.RawMessage
	resourceOrder []string
}

type resourceDeltaPayload struct {
	Upserts []json.RawMessage `json:"upserts,omitempty"`
	Removed []string          `json:"removed,omitempty"`
	Order   []string          `json:"order,omitempty"`
}

// buildClientStateSnapshot builds the per-client delta baseline and returns the
// marshalled state alongside it. Callers that are about to put the same state on
// the wire reuse those bytes instead of marshalling the payload a second time.
func buildClientStateSnapshot(state interface{}) (*clientStateSnapshot, []byte, error) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal state snapshot: %w", err)
	}

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, nil, fmt.Errorf("decode state snapshot: %w", err)
	}

	resources := make(map[string]json.RawMessage)
	resourceOrder := make([]string, 0)
	if encodedResources, ok := fields["resources"]; ok {
		var entries []json.RawMessage
		if err := json.Unmarshal(encodedResources, &entries); err != nil {
			return nil, nil, fmt.Errorf("decode state resources: %w", err)
		}
		for _, entry := range entries {
			var identity struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(entry, &identity); err != nil {
				return nil, nil, fmt.Errorf("decode state resource identity: %w", err)
			}
			if identity.ID == "" {
				return nil, nil, fmt.Errorf("state resource is missing id")
			}
			if _, exists := resources[identity.ID]; exists {
				return nil, nil, fmt.Errorf("state resource id %q is duplicated", identity.ID)
			}
			resources[identity.ID] = append(json.RawMessage(nil), entry...)
			resourceOrder = append(resourceOrder, identity.ID)
		}
		delete(fields, "resources")
	}

	return &clientStateSnapshot{
		fields:        fields,
		resources:     resources,
		resourceOrder: resourceOrder,
	}, encoded, nil
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

	resourceDelta := resourceDeltaPayload{}
	for _, id := range current.resourceOrder {
		currentResource := current.resources[id]
		previousResource, exists := previous.resources[id]
		if !exists {
			resourceDelta.Upserts = append(resourceDelta.Upserts, currentResource)
			continue
		}
		if bytes.Equal(previousResource, currentResource) {
			continue
		}
		patch, err := createJSONMergePatch(previousResource, currentResource)
		if err != nil {
			return nil, fmt.Errorf("build resource %q patch: %w", id, err)
		}
		resourceDelta.Upserts = append(resourceDelta.Upserts, patch)
	}
	for id := range previous.resources {
		if _, exists := current.resources[id]; !exists {
			resourceDelta.Removed = append(resourceDelta.Removed, id)
		}
	}
	sort.Strings(resourceDelta.Removed)
	if !reflect.DeepEqual(previous.resourceOrder, current.resourceOrder) {
		resourceDelta.Order = append([]string(nil), current.resourceOrder...)
	}
	if len(resourceDelta.Upserts) > 0 || len(resourceDelta.Removed) > 0 || len(resourceDelta.Order) > 0 {
		delta[resourceDeltaField] = resourceDelta
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
