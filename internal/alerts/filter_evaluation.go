package alerts

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// evaluateFilterCondition evaluates a single filter condition against a guest
func (m *Manager) evaluateFilterCondition(guest any, condition FilterCondition) bool {
	snapshot, ok := extractGuestSnapshot(guest)
	if !ok {
		return false
	}
	return evaluateGuestCondition(snapshot.metrics(), condition)
}

// guestMetrics holds common metrics for filter evaluation
type guestMetrics struct {
	CPU               float64 // CPU usage as percentage (0-100)
	MemUsage          float64 // Memory usage percentage
	DiskUsage         float64 // Disk usage percentage
	MemoryUnavailable bool
	DiskUnavailable   bool
	DiskRead          int64 // Bytes/s
	DiskWrite         int64 // Bytes/s
	NetworkIn         int64 // Bytes/s
	NetworkOut        int64 // Bytes/s
	IORateValidity    models.IORateValidity
	Name              string
	Node              string
	ID                string
	Status            string
}

// A missing rule input is neither a match nor a non-match. Keep that third
// state until policy selection; boolean callers still accept only a match.
type guestFilterResult uint8

const (
	guestFilterNoMatch guestFilterResult = iota
	guestFilterMatch
	guestFilterUnknown
)

func guestMetricFilterValue(metrics guestMetrics, field string) (value float64, available, supported bool) {
	validity := metrics.IORateValidity.EffectiveForRates(metrics.DiskRead, metrics.DiskWrite, metrics.NetworkIn, metrics.NetworkOut)
	available, supported = true, true
	switch strings.ToLower(field) {
	case "cpu":
		value = metrics.CPU
	case "memory":
		value, available = metrics.MemUsage, !metrics.MemoryUnavailable
	case "disk":
		value, available = metrics.DiskUsage, !metrics.DiskUnavailable
	case "diskread":
		value, available = float64(metrics.DiskRead)/1024/1024, validity.DiskRead
	case "diskwrite":
		value, available = float64(metrics.DiskWrite)/1024/1024, validity.DiskWrite
	case "networkin":
		value, available = float64(metrics.NetworkIn)/1024/1024, validity.NetworkIn
	case "networkout":
		value, available = float64(metrics.NetworkOut)/1024/1024, validity.NetworkOut
	default:
		supported = false
	}
	available = available && value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
	return
}

func evaluateGuestConditionResult(metrics guestMetrics, condition FilterCondition) guestFilterResult {
	if condition.Type == "metric" {
		// Invalid rules must not become unknown and fence legitimate policy.
		if value, ok := numericConditionValue(condition.Value); !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return guestFilterNoMatch
		}
		switch condition.Operator {
		case ">", "<", ">=", "<=", "=", "==":
		default:
			return guestFilterNoMatch
		}
		if _, available, supported := guestMetricFilterValue(metrics, condition.Field); supported && !available {
			return guestFilterUnknown
		}
	}
	if evaluateGuestCondition(metrics, condition) {
		return guestFilterMatch
	}
	return guestFilterNoMatch
}

// extractGuestMetrics extracts common metrics from a VM or Container
func extractGuestMetrics(guest any) (guestMetrics, bool) {
	snapshot, ok := extractGuestSnapshot(guest)
	if !ok {
		return guestMetrics{}, false
	}
	return snapshot.metrics(), true
}

// evaluateGuestCondition evaluates a filter condition against guest metrics
func evaluateGuestCondition(metrics guestMetrics, condition FilterCondition) bool {
	switch condition.Type {
	case "metric":
		value, available, supported := guestMetricFilterValue(metrics, condition.Field)
		if !supported || !available {
			return false
		}
		return evaluateNumericCondition(value, condition)

	case "text":
		searchValue := strings.ToLower(fmt.Sprintf("%v", condition.Value))
		switch strings.ToLower(condition.Field) {
		case "name":
			return strings.Contains(strings.ToLower(metrics.Name), searchValue)
		case "node":
			return strings.Contains(strings.ToLower(metrics.Node), searchValue)
		case "vmid":
			return strings.Contains(metrics.ID, searchValue)
		}

	case "raw":
		if condition.RawText != "" {
			term := strings.ToLower(condition.RawText)
			return strings.Contains(strings.ToLower(metrics.Name), term) ||
				strings.Contains(metrics.ID, term) ||
				strings.Contains(strings.ToLower(metrics.Node), term) ||
				strings.Contains(strings.ToLower(metrics.Status), term)
		}
	}

	return false
}

// evaluateNumericCondition evaluates a numeric comparison
func evaluateNumericCondition(value float64, condition FilterCondition) bool {
	condValue, ok := numericConditionValue(condition.Value)
	if !ok {
		return false
	}

	switch condition.Operator {
	case ">":
		return value > condValue
	case "<":
		return value < condValue
	case ">=":
		return value >= condValue
	case "<=":
		return value <= condValue
	case "=", "==":
		return value >= condValue-0.5 && value <= condValue+0.5
	}
	return false
}

func numericConditionValue(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int8:
		return float64(value), true
	case int16:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint:
		return float64(value), true
	case uint8:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint64:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// evaluateVMCondition evaluates a filter condition against a VM
func (m *Manager) evaluateVMCondition(vm models.VM, condition FilterCondition) bool {
	return evaluateGuestCondition(guestSnapshotFromVM(vm).metrics(), condition)
}

// evaluateContainerCondition evaluates a filter condition against a Container
func (m *Manager) evaluateContainerCondition(ct models.Container, condition FilterCondition) bool {
	return evaluateGuestCondition(guestSnapshotFromContainer(ct).metrics(), condition)
}

// evaluateFilterStack evaluates a filter stack against a guest
func (m *Manager) evaluateFilterStack(guest any, stack FilterStack) bool {
	return m.evaluateFilterStackResult(guest, stack) == guestFilterMatch
}

func (m *Manager) evaluateFilterStackResult(guest any, stack FilterStack) guestFilterResult {
	if len(stack.Filters) == 0 {
		return guestFilterMatch
	}
	snapshot, ok := extractGuestSnapshot(guest)
	if !ok {
		return guestFilterNoMatch
	}
	and := stack.LogicalOperator == "AND"
	unknown := false
	metrics := snapshot.metrics()
	for _, filter := range stack.Filters {
		result := evaluateGuestConditionResult(metrics, filter)
		if and && result == guestFilterNoMatch {
			return guestFilterNoMatch
		}
		if !and && result == guestFilterMatch {
			return guestFilterMatch
		}
		unknown = unknown || result == guestFilterUnknown
	}
	if unknown {
		return guestFilterUnknown
	}
	if and {
		return guestFilterMatch
	}
	return guestFilterNoMatch
}

// getGuestThresholds returns the appropriate thresholds for a guest
// Priority: Guest-specific overrides > Custom rules (by priority) > Global defaults
func (m *Manager) getGuestThresholds(guest any, guestID string) ThresholdConfig {
	return m.effectiveAlertPolicyNoLock(alertPolicyQuery{TypeKey: "vm", ResourceID: guestID, Guest: guest}).Thresholds
}
