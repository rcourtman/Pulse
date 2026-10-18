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

// poolCapacityTopologyGroups are the top-level vdev classes that zpool's own
// size/allocated properties count. OpenZFS spa_prop_get_config sums the
// normal, special, dedup and embedded-log metaslab classes (embedded-log
// metaslabs live on normal vdevs); separate log, cache and spare vdevs are
// not pool capacity.
var poolCapacityTopologyGroups = []string{"data", "special", "dedup"}

// poolCapacity accepts a coherent pool-specific reading, not a dataset/disk
// sum. Missing or invalid total/allocated bytes mean unknown usage, not an
// empty pool. Free may be derived from those two pool bytes only when absent;
// a supplied free reading must agree. Never mix flat, property and topology
// sources, or replace an explicitly supplied zero/null/invalid field via a
// fallback.
//
// Sources in order: flat pool.query fields (SCALE), zpool properties
// (zfs.pool.query, which boot.get_state returns), then the top-level vdev
// stats of the pool topology. CORE 13's pool.query (REST GET /pool) returns
// neither flat bytes nor properties; its pool_extend only attaches the
// libzfs vdev groups, whose top-level stats carry vs_space/vs_alloc as
// size/allocated (#2510).
func poolCapacity(item, properties map[string]any, topology any) (total, used, free int64) {
	source := item
	nested := false
	if !hasPoolCapacityFields(item) {
		if !hasPoolCapacityFields(properties) {
			return poolTopologyCapacity(topology)
		}
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

// poolTopologyCapacity sums the size/allocated stats of the top-level data,
// special and dedup vdevs, which equals zpool's size/allocated properties.
// Leaf vdevs carry no space accounting and are never read. Every counted
// top-level vdev must supply both readings: a partial sum would understate
// the pool, so it is reported as unknown instead.
func poolTopologyCapacity(topology any) (int64, int64, int64) {
	groups, ok := topology.(map[string]any)
	if !ok {
		return 0, 0, 0
	}
	if data, ok := groups["data"].([]any); !ok || len(data) == 0 {
		return 0, 0, 0
	}
	var total, used int64
	for _, groupName := range poolCapacityTopologyGroups {
		raw, present := groups[groupName]
		if !present || raw == nil {
			continue
		}
		nodes, ok := raw.([]any)
		if !ok {
			return 0, 0, 0
		}
		for _, node := range nodes {
			vdev, ok := node.(map[string]any)
			if !ok {
				return 0, 0, 0
			}
			stats, ok := vdev["stats"].(map[string]any)
			if !ok {
				return 0, 0, 0
			}
			size, sizeOK, _ := poolByteField(stats, []string{"size"}, false)
			allocated, allocatedOK, _ := poolByteField(stats, []string{"allocated"}, false)
			if !sizeOK || !allocatedOK || total > math.MaxInt64-size || used > math.MaxInt64-allocated {
				return 0, 0, 0
			}
			total += size
			used += allocated
		}
	}
	if total <= 0 || used > total {
		return 0, 0, 0
	}
	return total, used, total - used
}

// decodePoolTopology keeps native integer bytes exact; an intermediate
// float64 cannot represent every vdev size above 2^53.
func decodePoolTopology(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var topology any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&topology); err != nil {
		return nil
	}
	return topology
}

func poolRESTCapacity(item poolResponse, topology any) (int64, int64, int64) {
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
	return poolCapacity(fields, nil, topology)
}
