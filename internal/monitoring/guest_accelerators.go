package monitoring

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	maxReportedAccelerators   = 8
	maxAcceleratorLabelLength = 24
)

var (
	proxmoxHostPCIKey = regexp.MustCompile(`^hostpci\d+$`)
	proxmoxLXCDevKey  = regexp.MustCompile(`^(dev|mp)\d+$`)
)

// dockerAcceleratorsFromReport sanitizes the bounded list of hardware
// accelerator labels reported by a Docker agent (for example GPU, iGPU,
// AMD GPU, TPU, NPU). Returns nil when empty.
func dockerAcceleratorsFromReport(reported []string) []string {
	if len(reported) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, min(len(reported), maxReportedAccelerators))
	for _, raw := range reported {
		label := strings.TrimSpace(raw)
		if label == "" || len(label) > maxAcceleratorLabelLength {
			continue
		}
		if hasControlChar(label) {
			continue
		}
		seen[label] = struct{}{}
		if len(seen) >= maxReportedAccelerators {
			break
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for label := range seen {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

// parseProxmoxVMConfigAccelerators reads hostpciN entries from a Proxmox QEMU
// guest configuration. It labels clear GPU/TPU/NPU passthrough entries and
// falls back to "PCIe" for other PCI passthrough devices.
func parseProxmoxVMConfigAccelerators(config map[string]interface{}) []string {
	if len(config) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	for key, rawVal := range config {
		if !proxmoxHostPCIKey.MatchString(strings.ToLower(strings.TrimSpace(key))) {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(fmt.Sprint(rawVal)))
		if value == "" {
			continue
		}
		switch {
		case strings.Contains(value, "coral") || strings.Contains(value, "tpu") || strings.Contains(value, "apex"):
			seen["TPU"] = struct{}{}
		case strings.Contains(value, "hailo") || strings.Contains(value, "npu"):
			seen["NPU"] = struct{}{}
		case strings.Contains(value, "x-vga=1") ||
			strings.Contains(value, "gpu") ||
			strings.Contains(value, "vga") ||
			strings.Contains(value, "nvidia") ||
			strings.Contains(value, "amd") ||
			strings.Contains(value, "radeon") ||
			strings.Contains(value, "quadro") ||
			strings.Contains(value, "rtx") ||
			strings.Contains(value, "gtx") ||
			strings.Contains(value, "arc"):
			seen["GPU"] = struct{}{}
		default:
			seen["PCIe"] = struct{}{}
		}
	}
	return sortedAcceleratorLabels(seen)
}

// parseProxmoxLXCConfigAccelerators reads Proxmox LXC device passthrough lines
// (devN, mpN, lxc.mount.entry) for GPU, iGPU, AMD GPU, Coral TPU, and Hailo
// NPU devices.
func parseProxmoxLXCConfigAccelerators(config map[string]interface{}) []string {
	if len(config) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	scanValue := func(raw string) {
		lower := strings.ToLower(strings.TrimSpace(raw))
		if lower == "" {
			return
		}
		if strings.Contains(lower, "/dev/nvidia") || strings.Contains(lower, "dev/nvidia") {
			seen["GPU"] = struct{}{}
		}
		if strings.Contains(lower, "/dev/kfd") || strings.Contains(lower, "dev/kfd") {
			seen["AMD GPU"] = struct{}{}
		}
		if strings.Contains(lower, "/dev/dri") || strings.Contains(lower, "dev/dri") {
			seen["iGPU"] = struct{}{}
		}
		if strings.Contains(lower, "/dev/apex") || strings.Contains(lower, "dev/apex") {
			seen["TPU"] = struct{}{}
		}
		if strings.Contains(lower, "/dev/hailo") || strings.Contains(lower, "dev/hailo") {
			seen["NPU"] = struct{}{}
		}
	}

	for key, rawVal := range config {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		if !proxmoxLXCDevKey.MatchString(lowerKey) &&
			lowerKey != "lxc" &&
			!strings.HasPrefix(lowerKey, "lxc.mount.entry") {
			continue
		}
		switch typed := rawVal.(type) {
		case string:
			scanValue(typed)
		case []interface{}:
			for _, item := range typed {
				scanValue(fmt.Sprint(item))
			}
		case []string:
			for _, item := range typed {
				scanValue(item)
			}
		default:
			scanValue(fmt.Sprint(rawVal))
		}
	}
	return sortedAcceleratorLabels(seen)
}

func sortedAcceleratorLabels(seen map[string]struct{}) []string {
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for label := range seen {
		out = append(out, label)
	}
	sort.Strings(out)
	if len(out) > maxReportedAccelerators {
		out = out[:maxReportedAccelerators]
	}
	return out
}

func hasControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
