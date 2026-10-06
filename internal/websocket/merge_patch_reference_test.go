package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
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
