package monitoring

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// containerNetworkDetails holds parsed network interface information from container config.
type containerNetworkDetails struct {
	Name      string
	MAC       string
	Addresses []string
}

// containerMountMetadata holds parsed mount point information from container config.
type containerMountMetadata struct {
	Key        string
	Mountpoint string
	Source     string
}

// ensureContainerRootDiskEntry adds a root disk entry to a container if none exists.
func ensureContainerRootDiskEntry(container *models.Container) {
	if container == nil || len(container.Disks) > 0 {
		return
	}

	total := container.Disk.Total
	used := container.Disk.Used
	if total > 0 && used > total {
		used = total
	}

	free := total - used
	if free < 0 {
		free = 0
	}

	usage := container.Disk.Usage
	if total > 0 && usage <= 0 {
		usage = safePercentage(float64(used), float64(total))
	}

	container.Disks = []models.Disk{
		{
			Total:      total,
			Used:       used,
			Free:       free,
			Usage:      usage,
			Mountpoint: "/",
			Type:       "rootfs",
		},
	}
}

// convertContainerDiskInfo converts Proxmox container disk info to the models format.
func convertContainerDiskInfo(status *proxmox.Container, metadata map[string]containerMountMetadata) []models.Disk {
	if status == nil || len(status.DiskInfo) == 0 {
		return nil
	}

	disks := make([]models.Disk, 0, len(status.DiskInfo))
	for name, info := range status.DiskInfo {
		total := clampToInt64(info.Total)
		used := clampToInt64(info.Used)
		if total > 0 && used > total {
			used = total
		}
		free := total - used
		if free < 0 {
			free = 0
		}

		disk := models.Disk{
			Total: total,
			Used:  used,
			Free:  free,
		}

		if total > 0 {
			disk.Usage = safePercentage(float64(used), float64(total))
		}

		label := strings.TrimSpace(name)
		lowerLabel := strings.ToLower(label)
		mountpoint := ""
		device := ""

		if metadata != nil {
			if meta, ok := metadata[lowerLabel]; ok {
				mountpoint = strings.TrimSpace(meta.Mountpoint)
				device = strings.TrimSpace(meta.Source)
			}
		}

		if strings.EqualFold(label, "rootfs") || label == "" {
			if mountpoint == "" {
				mountpoint = "/"
			}
			disk.Type = "rootfs"
			if device == "" {
				device = sanitizeRootFSDevice(status.RootFS)
			}
		} else {
			if mountpoint == "" {
				mountpoint = label
			}
			disk.Type = lowerLabel
		}

		disk.Mountpoint = mountpoint
		if disk.Device == "" && device != "" {
			disk.Device = device
		}

		disks = append(disks, disk)
	}

	if len(disks) > 1 {
		sort.SliceStable(disks, func(i, j int) bool {
			return disks[i].Mountpoint < disks[j].Mountpoint
		})
	}

	return disks
}

// sanitizeRootFSDevice extracts the device path from a rootfs config string.
func sanitizeRootFSDevice(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	if idx := strings.Index(root, ","); idx != -1 {
		root = root[:idx]
	}
	return root
}

// parseContainerRawIPs extracts IP addresses from raw JSON data.
func parseContainerRawIPs(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var data interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil
	}
	return collectIPsFromInterface(data)
}

// collectIPsFromInterface recursively extracts IP addresses from various data types.
func collectIPsFromInterface(value interface{}) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return sanitizeGuestAddressStrings(v)
	case []interface{}:
		results := make([]string, 0, len(v))
		for _, item := range v {
			results = append(results, collectIPsFromInterface(item)...)
		}
		return results
	case []string:
		results := make([]string, 0, len(v))
		for _, item := range v {
			results = append(results, sanitizeGuestAddressStrings(item)...)
		}
		return results
	case map[string]interface{}:
		results := make([]string, 0)
		for _, key := range []string{"ip", "ip6", "ipv4", "ipv6", "address", "value"} {
			if val, ok := v[key]; ok {
				results = append(results, collectIPsFromInterface(val)...)
			}
		}
		return results
	case json.Number:
		return sanitizeGuestAddressStrings(v.String())
	default:
		return nil
	}
}

// sanitizeGuestAddressStrings cleans and validates IP address strings.
func sanitizeGuestAddressStrings(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	lower := strings.ToLower(value)
	switch lower {
	case "dhcp", "manual", "static", "auto", "none", "n/a", "unknown", "0.0.0.0", "::", "::1":
		return nil
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';'
	})

	if len(parts) > 1 {
		results := make([]string, 0, len(parts))
		for _, part := range parts {
			results = append(results, sanitizeGuestAddressStrings(part)...)
		}
		return results
	}

	if idx := strings.Index(value, "/"); idx > 0 {
		value = strings.TrimSpace(value[:idx])
	}

	lower = strings.ToLower(value)

	if idx := strings.Index(value, "%"); idx > 0 {
		value = strings.TrimSpace(value[:idx])
		lower = strings.ToLower(value)
	}

	if strings.HasPrefix(value, "127.") || strings.HasPrefix(lower, "0.0.0.0") {
		return nil
	}

	if strings.HasPrefix(lower, "fe80") {
		return nil
	}

	if strings.HasPrefix(lower, "::1") {
		return nil
	}

	return []string{value}
}

// dedupeStringsPreserveOrder removes duplicates from a string slice while preserving order.
func dedupeStringsPreserveOrder(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseContainerConfigNetworks extracts network interface details from container config.
func parseContainerConfigNetworks(config map[string]interface{}) []containerNetworkDetails {
	if len(config) == 0 {
		return nil
	}

	keys := make([]string, 0, len(config))
	for key := range config {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), "net") {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)

	results := make([]containerNetworkDetails, 0, len(keys))
	for _, key := range keys {
		raw := fmt.Sprint(config[key])
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		detail := containerNetworkDetails{}
		parts := strings.Split(raw, ",")
		for _, part := range parts {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.ToLower(strings.TrimSpace(kv[0]))
			value := strings.TrimSpace(kv[1])
			switch k {
			case "name":
				detail.Name = value
			case "hwaddr", "mac", "macaddr":
				detail.MAC = strings.ToUpper(value)
			case "ip", "ip6", "ips", "ip6addr", "ip6prefix":
				detail.Addresses = append(detail.Addresses, sanitizeGuestAddressStrings(value)...)
			}
		}

		if detail.Name == "" {
			detail.Name = strings.TrimSpace(key)
		}
		if len(detail.Addresses) > 0 {
			detail.Addresses = dedupeStringsPreserveOrder(detail.Addresses)
		}

		if detail.Name != "" || detail.MAC != "" || len(detail.Addresses) > 0 {
			results = append(results, detail)
		}
	}

	if len(results) == 0 {
		return nil
	}

	return results
}

// parseContainerMountMetadata extracts mount point metadata from container config.
func parseContainerMountMetadata(config map[string]interface{}) map[string]containerMountMetadata {
	if len(config) == 0 {
		return nil
	}

	results := make(map[string]containerMountMetadata)
	for rawKey, rawValue := range config {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if key != "rootfs" && !strings.HasPrefix(key, "mp") {
			continue
		}

		value := strings.TrimSpace(fmt.Sprint(rawValue))
		if value == "" {
			continue
		}

		meta := containerMountMetadata{
			Key: key,
		}

		parts := strings.Split(value, ",")
		if len(parts) > 0 {
			meta.Source = strings.TrimSpace(parts[0])
		}

		for _, part := range parts[1:] {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.ToLower(strings.TrimSpace(kv[0]))
			v := strings.TrimSpace(kv[1])
			switch k {
			case "mp", "mountpoint":
				meta.Mountpoint = v
			}
		}

		if meta.Mountpoint == "" && key == "rootfs" {
			meta.Mountpoint = "/"
		}

		results[key] = meta
	}

	if len(results) == 0 {
		return nil
	}

	return results
}

// mergeContainerNetworkInterface merges network interface details into the target slice.
func mergeContainerNetworkInterface(target *[]models.GuestNetworkInterface, detail containerNetworkDetails) {
	if target == nil {
		return
	}
	if len(detail.Addresses) > 0 {
		detail.Addresses = dedupeStringsPreserveOrder(detail.Addresses)
	}

	findMatch := func() int {
		for i := range *target {
			if detail.Name != "" && (*target)[i].Name != "" && strings.EqualFold((*target)[i].Name, detail.Name) {
				return i
			}
			if detail.MAC != "" && (*target)[i].MAC != "" && strings.EqualFold((*target)[i].MAC, detail.MAC) {
				return i
			}
		}
		return -1
	}

	if idx := findMatch(); idx >= 0 {
		if detail.Name != "" && (*target)[idx].Name == "" {
			(*target)[idx].Name = detail.Name
		}
		if detail.MAC != "" && (*target)[idx].MAC == "" {
			(*target)[idx].MAC = detail.MAC
		}
		if len(detail.Addresses) > 0 {
			combined := append((*target)[idx].Addresses, detail.Addresses...)
			(*target)[idx].Addresses = dedupeStringsPreserveOrder(combined)
		}
		return
	}

	newIface := models.GuestNetworkInterface{
		Name: detail.Name,
		MAC:  detail.MAC,
	}
	if len(detail.Addresses) > 0 {
		newIface.Addresses = dedupeStringsPreserveOrder(detail.Addresses)
	}
	*target = append(*target, newIface)
}

// extractContainerRootDeviceFromConfig extracts the root device path from container config.
func extractContainerRootDeviceFromConfig(config map[string]interface{}) string {
	if len(config) == 0 {
		return ""
	}
	raw, ok := config["rootfs"]
	if !ok {
		return ""
	}

	value := strings.TrimSpace(fmt.Sprint(raw))
	if value == "" {
		return ""
	}

	parts := strings.Split(value, ",")
	device := strings.TrimSpace(parts[0])
	return device
}
