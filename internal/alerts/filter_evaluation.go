package alerts

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// evaluateFilterCondition evaluates a single filter condition against a guest
func (m *Manager) evaluateFilterCondition(guest interface{}, condition FilterCondition) bool {
	switch g := guest.(type) {
	case models.VM:
		return m.evaluateVMCondition(g, condition)
	case models.Container:
		return m.evaluateContainerCondition(g, condition)
	default:
		return false
	}
}

// evaluateVMCondition evaluates a filter condition against a VM
func (m *Manager) evaluateVMCondition(vm models.VM, condition FilterCondition) bool {
	switch condition.Type {
	case "metric":
		value := 0.0
		switch strings.ToLower(condition.Field) {
		case "cpu":
			value = vm.CPU * 100
		case "memory":
			value = vm.Memory.Usage
		case "disk":
			value = vm.Disk.Usage
		case "diskread":
			value = float64(vm.DiskRead) / 1024 / 1024 // Convert to MB/s
		case "diskwrite":
			value = float64(vm.DiskWrite) / 1024 / 1024
		case "networkin":
			value = float64(vm.NetworkIn) / 1024 / 1024
		case "networkout":
			value = float64(vm.NetworkOut) / 1024 / 1024
		default:
			return false
		}

		condValue, ok := condition.Value.(float64)
		if !ok {
			// Try to convert from int
			if intVal, ok := condition.Value.(int); ok {
				condValue = float64(intVal)
			} else {
				return false
			}
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

	case "text":
		searchValue := strings.ToLower(fmt.Sprintf("%v", condition.Value))
		switch strings.ToLower(condition.Field) {
		case "name":
			return strings.Contains(strings.ToLower(vm.Name), searchValue)
		case "node":
			return strings.Contains(strings.ToLower(vm.Node), searchValue)
		case "vmid":
			return strings.Contains(vm.ID, searchValue)
		}

	case "raw":
		if condition.RawText != "" {
			term := strings.ToLower(condition.RawText)
			return strings.Contains(strings.ToLower(vm.Name), term) ||
				strings.Contains(vm.ID, term) ||
				strings.Contains(strings.ToLower(vm.Node), term) ||
				strings.Contains(strings.ToLower(vm.Status), term)
		}
	}

	return false
}

// evaluateContainerCondition evaluates a filter condition against a Container
func (m *Manager) evaluateContainerCondition(ct models.Container, condition FilterCondition) bool {
	// Similar logic to evaluateVMCondition but for Container type
	switch condition.Type {
	case "metric":
		value := 0.0
		switch strings.ToLower(condition.Field) {
		case "cpu":
			value = ct.CPU * 100
		case "memory":
			value = ct.Memory.Usage
		case "disk":
			value = ct.Disk.Usage
		case "diskread":
			value = float64(ct.DiskRead) / 1024 / 1024
		case "diskwrite":
			value = float64(ct.DiskWrite) / 1024 / 1024
		case "networkin":
			value = float64(ct.NetworkIn) / 1024 / 1024
		case "networkout":
			value = float64(ct.NetworkOut) / 1024 / 1024
		default:
			return false
		}

		condValue, ok := condition.Value.(float64)
		if !ok {
			if intVal, ok := condition.Value.(int); ok {
				condValue = float64(intVal)
			} else {
				return false
			}
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

	case "text":
		searchValue := strings.ToLower(fmt.Sprintf("%v", condition.Value))
		switch strings.ToLower(condition.Field) {
		case "name":
			return strings.Contains(strings.ToLower(ct.Name), searchValue)
		case "node":
			return strings.Contains(strings.ToLower(ct.Node), searchValue)
		case "vmid":
			return strings.Contains(ct.ID, searchValue)
		}

	case "raw":
		if condition.RawText != "" {
			term := strings.ToLower(condition.RawText)
			return strings.Contains(strings.ToLower(ct.Name), term) ||
				strings.Contains(ct.ID, term) ||
				strings.Contains(strings.ToLower(ct.Node), term) ||
				strings.Contains(strings.ToLower(ct.Status), term)
		}
	}

	return false
}

// evaluateFilterStack evaluates a filter stack against a guest
func (m *Manager) evaluateFilterStack(guest interface{}, stack FilterStack) bool {
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
func (m *Manager) getGuestThresholds(guest interface{}, guestID string) ThresholdConfig {
	// Start with defaults
	thresholds := m.config.GuestDefaults

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
		if applicableRule.Thresholds.CPU != nil {
			thresholds.CPU = ensureHysteresisThreshold(applicableRule.Thresholds.CPU)
		} else if applicableRule.Thresholds.CPULegacy != nil {
			thresholds.CPU = m.convertLegacyThreshold(applicableRule.Thresholds.CPULegacy)
		}
		if applicableRule.Thresholds.Memory != nil {
			thresholds.Memory = ensureHysteresisThreshold(applicableRule.Thresholds.Memory)
		} else if applicableRule.Thresholds.MemoryLegacy != nil {
			thresholds.Memory = m.convertLegacyThreshold(applicableRule.Thresholds.MemoryLegacy)
		}
		if applicableRule.Thresholds.Disk != nil {
			thresholds.Disk = ensureHysteresisThreshold(applicableRule.Thresholds.Disk)
		} else if applicableRule.Thresholds.DiskLegacy != nil {
			thresholds.Disk = m.convertLegacyThreshold(applicableRule.Thresholds.DiskLegacy)
		}
		if applicableRule.Thresholds.DiskRead != nil {
			thresholds.DiskRead = ensureHysteresisThreshold(applicableRule.Thresholds.DiskRead)
		} else if applicableRule.Thresholds.DiskReadLegacy != nil {
			thresholds.DiskRead = m.convertLegacyThreshold(applicableRule.Thresholds.DiskReadLegacy)
		}
		if applicableRule.Thresholds.DiskWrite != nil {
			thresholds.DiskWrite = ensureHysteresisThreshold(applicableRule.Thresholds.DiskWrite)
		} else if applicableRule.Thresholds.DiskWriteLegacy != nil {
			thresholds.DiskWrite = m.convertLegacyThreshold(applicableRule.Thresholds.DiskWriteLegacy)
		}
		if applicableRule.Thresholds.NetworkIn != nil {
			thresholds.NetworkIn = ensureHysteresisThreshold(applicableRule.Thresholds.NetworkIn)
		} else if applicableRule.Thresholds.NetworkInLegacy != nil {
			thresholds.NetworkIn = m.convertLegacyThreshold(applicableRule.Thresholds.NetworkInLegacy)
		}
		if applicableRule.Thresholds.NetworkOut != nil {
			thresholds.NetworkOut = ensureHysteresisThreshold(applicableRule.Thresholds.NetworkOut)
		} else if applicableRule.Thresholds.NetworkOutLegacy != nil {
			thresholds.NetworkOut = m.convertLegacyThreshold(applicableRule.Thresholds.NetworkOutLegacy)
		}
		if applicableRule.Thresholds.DisableConnectivity {
			thresholds.DisableConnectivity = true
		}

		log.Debug().
			Str("guest", guestID).
			Str("rule", applicableRule.Name).
			Int("priority", applicableRule.Priority).
			Msg("Applied custom alert rule")
	}

	// Finally check guest-specific overrides (highest priority)
	// First try the new stable ID format (instance-VMID)
	override, exists := m.config.Overrides[guestID]

	// If not found, try legacy ID formats for migration
	if !exists {
		var legacyID string
		var node string
		var vmid int
		var instance string

		// Extract node, vmid, and instance from the guest object
		switch g := guest.(type) {
		case models.VM:
			node = g.Node
			vmid = g.VMID
			instance = g.Instance
		case models.Container:
			node = g.Node
			vmid = g.VMID
			instance = g.Instance
		default:
			// Not a VM or container, skip legacy migration
			goto skipLegacyMigration
		}

		// Try legacy format: instance-node-VMID
		if instance != node {
			legacyID = fmt.Sprintf("%s-%s-%d", instance, node, vmid)
			if legacyOverride, legacyExists := m.config.Overrides[legacyID]; legacyExists {
				log.Info().
					Str("legacyID", legacyID).
					Str("newID", guestID).
					Msg("Migrating guest override from legacy ID format")

				// Move to new ID
				m.config.Overrides[guestID] = legacyOverride
				delete(m.config.Overrides, legacyID)

				// Config will be persisted on next save cycle
				override = legacyOverride
				exists = true
			}
		}

		// If still not found, try standalone format: node-VMID
		if !exists && instance == node {
			legacyID = fmt.Sprintf("%s-%d", node, vmid)
			if legacyOverride, legacyExists := m.config.Overrides[legacyID]; legacyExists {
				log.Info().
					Str("legacyID", legacyID).
					Str("newID", guestID).
					Msg("Migrating guest override from legacy standalone ID format")

				// Move to new ID
				m.config.Overrides[guestID] = legacyOverride
				delete(m.config.Overrides, legacyID)

				// Config will be persisted on next save cycle
				override = legacyOverride
				exists = true
			}
		}
	}

skipLegacyMigration:
	if exists {
		// Apply the disabled flag if set
		if override.Disabled {
			thresholds.Disabled = true
		}
		if override.DisableConnectivity {
			thresholds.DisableConnectivity = true
		}

		if override.CPU != nil {
			thresholds.CPU = ensureHysteresisThreshold(override.CPU)
		} else if override.CPULegacy != nil {
			thresholds.CPU = m.convertLegacyThreshold(override.CPULegacy)
		}
		if override.Memory != nil {
			thresholds.Memory = ensureHysteresisThreshold(override.Memory)
		} else if override.MemoryLegacy != nil {
			thresholds.Memory = m.convertLegacyThreshold(override.MemoryLegacy)
		}
		if override.Disk != nil {
			thresholds.Disk = ensureHysteresisThreshold(override.Disk)
		} else if override.DiskLegacy != nil {
			thresholds.Disk = m.convertLegacyThreshold(override.DiskLegacy)
		}
		if override.DiskRead != nil {
			thresholds.DiskRead = ensureHysteresisThreshold(override.DiskRead)
		} else if override.DiskReadLegacy != nil {
			thresholds.DiskRead = m.convertLegacyThreshold(override.DiskReadLegacy)
		}
		if override.DiskWrite != nil {
			thresholds.DiskWrite = ensureHysteresisThreshold(override.DiskWrite)
		} else if override.DiskWriteLegacy != nil {
			thresholds.DiskWrite = m.convertLegacyThreshold(override.DiskWriteLegacy)
		}
		if override.NetworkIn != nil {
			thresholds.NetworkIn = ensureHysteresisThreshold(override.NetworkIn)
		} else if override.NetworkInLegacy != nil {
			thresholds.NetworkIn = m.convertLegacyThreshold(override.NetworkInLegacy)
		}
		if override.NetworkOut != nil {
			thresholds.NetworkOut = ensureHysteresisThreshold(override.NetworkOut)
		} else if override.NetworkOutLegacy != nil {
			thresholds.NetworkOut = m.convertLegacyThreshold(override.NetworkOutLegacy)
		}
	}

	return thresholds
}
