package truenas

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// poolStateResponse preserves integer bytes when the RPC envelope decodes its
// result. An intermediate float64 cannot represent every native int64 value.
// Other RPC result shapes retain their existing decoder.
type poolStateResponse map[string]any

func (p *poolStateResponse) UnmarshalJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode((*map[string]any)(p))
}

var poolTotalKeys = []string{"size", "total", "total_bytes", "totalBytes"}
var poolUsedKeys = []string{"allocated", "used", "used_bytes", "usedBytes"}
var poolFreeKeys = []string{"free", "free_bytes", "freeBytes", "available"}

// poolCapacity accepts a coherent pool-specific reading, not a dataset/disk
// sum. Missing or invalid total/allocated bytes mean unknown usage, not an
// empty pool. Free may be derived from those two pool bytes only when absent;
// a supplied free reading must agree. Never mix flat and property sources,
// or replace an explicitly supplied zero/null/invalid field via a fallback.
func poolCapacity(item, properties map[string]any) (total, used, free int64) {
	source := item
	nested := false
	if !hasPoolCapacityFields(item) {
		source = properties
		nested = true
	}
	total, totalOK, _ := poolByteField(source, poolTotalKeys, nested)
	used, usedOK, _ := poolByteField(source, poolUsedKeys, nested)
	if !totalOK || !usedOK || total <= 0 || used > total {
		return 0, 0, 0
	}
	free, freeOK, supplied := poolByteField(source, poolFreeKeys, nested)
	if supplied && (!freeOK || free != total-used) {
		return 0, 0, 0
	}
	return total, used, total - used
}

func hasPoolCapacityFields(item map[string]any) bool {
	for _, keys := range [][]string{poolTotalKeys, poolUsedKeys, poolFreeKeys} {
		for _, key := range keys {
			if _, ok := item[key]; ok {
				return true
			}
		}
	}
	return false
}

func poolByteField(source map[string]any, keys []string, nested bool) (int64, bool, bool) {
	for _, key := range keys {
		value, present := source[key]
		if !present {
			continue
		}
		if nested {
			if property, ok := value.(map[string]any); ok {
				value, _ = firstAny(property, "parsed", "value", "rawvalue", "raw")
			}
		}
		parsed, valid := poolByteValue(value)
		return parsed, valid, true
	}
	return 0, false, false
}

func poolByteValue(value any) (int64, bool) {
	var parsed int64
	switch value := value.(type) {
	case int64:
		parsed = value
	case int:
		parsed = int64(value)
	case json.Number:
		var err error
		parsed, err = value.Int64()
		if err != nil {
			return 0, false
		}
	case string:
		var err error
		parsed, err = strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return 0, false
		}
	case float64:
		// Retain safe direct-map callers, without rounding an unsafe float,
		// truncating fractions or accepting NaN/infinity as bytes.
		if math.IsNaN(value) || value < 0 || value > 1<<53 || math.Trunc(value) != value {
			return 0, false
		}
		parsed = int64(value)
	default:
		return 0, false
	}
	return parsed, parsed >= 0
}

func poolRESTCapacity(item poolResponse) (int64, int64, int64) {
	fields := make(map[string]any, 3)
	for key, raw := range map[string]json.RawMessage{"size": item.Size, "allocated": item.Allocated, "free": item.Free} {
		if len(raw) == 0 {
			continue
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		// The enclosing JSON decoder has already validated syntax. If that
		// ever changes, an invalid field still cannot become a measurement.
		if err := decoder.Decode(&value); err != nil {
			return 0, 0, 0
		}
		fields[key] = value
	}
	return poolCapacity(fields, nil)
}
