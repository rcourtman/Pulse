package alerts

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type guestOverrideIdentity struct {
	instance string
	node     string
	vmid     int
}

func isCanonicalGuestKeyPart(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.Contains(value, "/")
}

func stableGuestOverrideKey(instance string, vmid int) string {
	return fmt.Sprintf("guest:%s:%d", strings.TrimSpace(instance), vmid)
}

func legacyGuestStableOverrideKey(instance string, vmid int) string {
	return fmt.Sprintf("%s-%d", strings.TrimSpace(instance), vmid)
}

func legacyClusterGuestOverrideKey(instance, node string, vmid int) string {
	return fmt.Sprintf("%s-%s-%d", strings.TrimSpace(instance), strings.TrimSpace(node), vmid)
}

func parseCanonicalGuestKey(guestID string) (guestOverrideIdentity, bool) {
	parts := strings.Split(strings.TrimSpace(guestID), ":")
	if len(parts) != 3 || strings.EqualFold(parts[0], "guest") {
		return guestOverrideIdentity{}, false
	}

	vmid, err := strconv.Atoi(parts[2])
	if err != nil || vmid <= 0 {
		return guestOverrideIdentity{}, false
	}

	instance := strings.TrimSpace(parts[0])
	node := strings.TrimSpace(parts[1])
	if !isCanonicalGuestKeyPart(instance) || !isCanonicalGuestKeyPart(node) {
		return guestOverrideIdentity{}, false
	}
	if instance == "" {
		instance = node
	}
	if instance == "" || node == "" {
		return guestOverrideIdentity{}, false
	}

	return guestOverrideIdentity{
		instance: instance,
		node:     node,
		vmid:     vmid,
	}, true
}

func parseStableGuestOverrideKey(key string) (guestOverrideIdentity, bool) {
	parts := strings.Split(strings.TrimSpace(key), ":")
	if len(parts) != 3 || parts[0] != "guest" {
		return guestOverrideIdentity{}, false
	}

	vmid, err := strconv.Atoi(parts[2])
	if err != nil || vmid <= 0 {
		return guestOverrideIdentity{}, false
	}

	instance := strings.TrimSpace(parts[1])
	if !isCanonicalGuestKeyPart(instance) {
		return guestOverrideIdentity{}, false
	}

	return guestOverrideIdentity{
		instance: instance,
		node:     instance,
		vmid:     vmid,
	}, true
}

func guestOverrideIdentityFromGuestOrID(guest any, guestID string) (guestOverrideIdentity, bool) {
	switch g := guest.(type) {
	case models.VM:
		return guestOverrideIdentityFromParts(g.Instance, g.Node, g.VMID)
	case *models.VM:
		if g == nil {
			return guestOverrideIdentity{}, false
		}
		return guestOverrideIdentityFromParts(g.Instance, g.Node, g.VMID)
	case models.Container:
		return guestOverrideIdentityFromParts(g.Instance, g.Node, g.VMID)
	case *models.Container:
		if g == nil {
			return guestOverrideIdentity{}, false
		}
		return guestOverrideIdentityFromParts(g.Instance, g.Node, g.VMID)
	default:
		if parsed, ok := parseCanonicalGuestKey(guestID); ok {
			return parsed, true
		}
		return parseStableGuestOverrideKey(guestID)
	}
}

func guestOverrideIdentityFromParts(instance, node string, vmid int) (guestOverrideIdentity, bool) {
	instance = strings.TrimSpace(instance)
	node = strings.TrimSpace(node)
	if instance == "" {
		instance = node
	}
	if instance == "" || node == "" || vmid <= 0 {
		return guestOverrideIdentity{}, false
	}
	return guestOverrideIdentity{
		instance: instance,
		node:     node,
		vmid:     vmid,
	}, true
}

func clusteredGuestOverrideKey(ident guestOverrideIdentity) string {
	if ident.instance == "" || ident.node == "" || ident.vmid <= 0 || ident.instance == ident.node {
		return ""
	}
	return stableGuestOverrideKey(ident.instance, ident.vmid)
}

func lookupGuestOverride(overrides map[string]ThresholdConfig, guest any, guestID string) (ThresholdConfig, bool) {
	ident, ok := guestOverrideIdentityFromGuestOrID(guest, guestID)
	if ok {
		if stableKey := clusteredGuestOverrideKey(ident); stableKey != "" {
			if override, exists := overrides[stableKey]; exists {
				return override, true
			}
		}
	}

	guestID = strings.TrimSpace(guestID)
	if guestID != "" {
		if override, exists := overrides[guestID]; exists {
			return override, true
		}
	}

	if !ok {
		return ThresholdConfig{}, false
	}

	legacyStableKey := legacyGuestStableOverrideKey(ident.instance, ident.vmid)
	if override, exists := overrides[legacyStableKey]; exists {
		return override, true
	}

	if ident.instance == ident.node {
		return ThresholdConfig{}, false
	}

	legacyClusterKey := legacyClusterGuestOverrideKey(ident.instance, ident.node, ident.vmid)
	if override, exists := overrides[legacyClusterKey]; exists {
		return override, true
	}

	for key, override := range overrides {
		parsed, ok := parseCanonicalGuestKey(key)
		if !ok {
			continue
		}
		if parsed.instance == ident.instance && parsed.vmid == ident.vmid {
			return override, true
		}
	}

	return ThresholdConfig{}, false
}

func isGuestThresholdResourceType(typeKey string) bool {
	switch strings.ToLower(strings.TrimSpace(typeKey)) {
	case "vm", "system-container":
		return true
	default:
		return false
	}
}
