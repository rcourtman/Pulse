package alerts

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
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
	CPU        float64 // CPU usage as percentage (0-100)
	MemUsage   float64 // Memory usage percentage
	DiskUsage  float64 // Disk usage percentage
	DiskRead   int64   // Bytes/s
	DiskWrite  int64   // Bytes/s
	NetworkIn  int64   // Bytes/s
	NetworkOut int64   // Bytes/s
	Name       string
	Node       string
	ID         string
	Status     string
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
		value := 0.0
		switch strings.ToLower(condition.Field) {
		case "cpu":
			value = metrics.CPU
		case "memory":
			value = metrics.MemUsage
		case "disk":
			value = metrics.DiskUsage
		case "diskread":
			value = float64(metrics.DiskRead) / 1024 / 1024 // Convert to MB/s
		case "diskwrite":
			value = float64(metrics.DiskWrite) / 1024 / 1024
		case "networkin":
			value = float64(metrics.NetworkIn) / 1024 / 1024
		case "networkout":
			value = float64(metrics.NetworkOut) / 1024 / 1024
		default:
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
	if len(stack.Filters) == 0 {
		return true
	}

	results := make([]bool, len(stack.Filters))
	for i, filter := range stack.Filters {
		results[i] = m.evaluateFilterCondition(guest, filter)
	}

	// Apply logical operator
	if stack.LogicalOperator == "AND" {
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	}
	// OR
	for _, result := range results {
		if result {
			return true
		}
	}
	return false
}

// getGuestThresholds returns the appropriate thresholds for a guest
// Priority: Guest-specific overrides > Custom rules (by priority) > Global defaults
func (m *Manager) getGuestThresholds(guest any, guestID string) ThresholdConfig {
	thresholds := cloneThresholdConfig(m.config.GuestDefaults)

	// Check custom rules (sorted by priority, highest first)
	var applicableRule *CustomAlertRule
	highestPriority := -1

	for i := range m.config.CustomRules {
		rule := &m.config.CustomRules[i]
		if !rule.Enabled {
			continue
		}

		// Check if this rule applies to the guest
		if m.evaluateFilterStack(guest, rule.FilterConditions) {
			if rule.Priority > highestPriority {
				applicableRule = rule
				highestPriority = rule.Priority
			}
		}
	}

	// Apply custom rule thresholds if found
	if applicableRule != nil {
		thresholds = m.applyThresholdOverride(thresholds, applicableRule.Thresholds)

		log.Debug().
			Str("guest", guestID).
			Str("rule", applicableRule.Name).
			Int("priority", applicableRule.Priority).
			Msg("Applied custom alert rule")
	}

	return m.resolveThresholdOverride(thresholds, guestID)
}
