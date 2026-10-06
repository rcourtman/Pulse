package websocket

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// Exact pre-repair helper from assigned 0a99cb5a, kept independent for paired
// same-executable cost comparisons. These are synthetic patch costs, not
// installed CPU, transport/compression or native-fleet acceptance.
func parentNumberMergePatch(previous, current json.RawMessage) (json.RawMessage, error) {
	var previousValue interface{}
	if err := json.Unmarshal(previous, &previousValue); err != nil {
		return nil, err
	}
	var currentValue interface{}
	if err := json.Unmarshal(current, &currentValue); err != nil {
		return nil, err
	}

	patchValue, changed := parentNumberMergeValue(previousValue, currentValue)
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

func parentNumberMergeValue(previous, current interface{}) (interface{}, bool) {
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
		if nestedPatch, changed := parentNumberMergeValue(previousValue, currentValue); changed {
			patch[key] = nestedPatch
		}
	}
	return patch, len(patch) > 0
}

func BenchmarkJSONMergeNumberFidelity(b *testing.B) {
	for _, metadataKeys := range []int{0, 32} {
		previous := map[string]interface{}{"id": "r", "cpu": map[string]interface{}{"current": 10}, "lastSeen": int64(100), "counter": uint64(9007199254740993)}
		metadata := make(map[string]interface{}, metadataKeys)
		for i := 0; i < metadataKeys; i++ {
			metadata[fmt.Sprintf("key-%d", i)] = map[string]interface{}{"name": "unchanged", "enabled": true, "values": []string{"one", "two", "three"}}
		}
		previous["platformData"] = metadata
		before, err := json.Marshal(previous)
		if err != nil {
			b.Fatal(err)
		}
		previous["lastSeen"] = int64(200)
		previous["cpu"] = map[string]interface{}{"current": 20}
		after, err := json.Marshal(previous)
		if err != nil {
			b.Fatal(err)
		}
		for _, implementation := range []struct {
			name  string
			patch func(json.RawMessage, json.RawMessage) (json.RawMessage, error)
		}{{"parent", parentNumberMergePatch}, {"exact", createJSONMergePatch}} {
			b.Run(fmt.Sprintf("metadata-%d/%s", metadataKeys, implementation.name), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := implementation.patch(before, after); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
