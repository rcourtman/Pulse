package hostagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const hostAgentUnraidVersionPath = "/etc/unraid-version"

var commonUnraidMdcmdPaths = []string{
	"/usr/local/sbin/mdcmd",
	"/usr/sbin/mdcmd",
	"/sbin/mdcmd",
	"/usr/local/bin/mdcmd",
	"/usr/bin/mdcmd",
	"/bin/mdcmd",
}

// CollectUnraidStorage returns best-effort Unraid array topology for hosts
// running Unraid. Non-Unraid hosts return nil with no error.
func CollectUnraidStorage(ctx context.Context, collector SystemCollector) (*agentshost.UnraidStorage, error) {
	if collector == nil || collector.GOOS() != "linux" {
		return nil, nil
	}

	if _, err := collector.Stat(hostAgentUnraidVersionPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", hostAgentUnraidVersionPath, err)
	}

	mdcmdPath, err := resolveUnraidMdcmdBinary(collector)
	if err != nil {
		return nil, err
	}

	statusCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	output, err := collector.CommandCombinedOutput(statusCtx, mdcmdPath, "status")
	if err != nil {
		return nil, fmt.Errorf("run mdcmd status: %w", err)
	}

	return parseUnraidStatusOutput(output)
}

func resolveUnraidMdcmdBinary(collector SystemCollector) (string, error) {
	for _, candidate := range commonUnraidMdcmdPaths {
		if _, err := collector.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	path, err := collector.LookPath("mdcmd")
	if err != nil {
		return "", fmt.Errorf("mdcmd binary not found in PATH or common locations")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("mdcmd path is not absolute: %q", path)
	}
	if _, err := collector.Stat(path); err != nil {
		return "", fmt.Errorf("mdcmd path unavailable: %w", err)
	}
	return path, nil
}

func parseUnraidStatusOutput(output string) (*agentshost.UnraidStorage, error) {
	fields := parseUnraidKeyValueOutput(output)
	if len(fields) == 0 {
		return nil, fmt.Errorf("mdcmd status returned no key=value fields")
	}

	storage := &agentshost.UnraidStorage{
		ArrayState:   strings.ToUpper(strings.TrimSpace(fields["mdState"])),
		ArrayStarted: strings.EqualFold(strings.TrimSpace(fields["mdState"]), "STARTED"),
		SyncAction:   normalizeUnraidSyncAction(fields["mdResyncAction"]),
		SyncProgress: unraidSyncProgress(fields),
		SyncErrors:   parseUnraidInt64Field(fields, "mdResyncCorr"),
		NumProtected: parseUnraidIntField(fields, "mdNumProtected"),
		NumDisabled:  parseUnraidIntField(fields, "mdNumDisabled"),
		NumInvalid:   parseUnraidIntField(fields, "mdNumInvalid"),
		NumMissing:   parseUnraidIntField(fields, "mdNumMissing"),
	}

	indexes := collectUnraidIndexes(fields)
	disks := make([]agentshost.UnraidDisk, 0, len(indexes))
	for _, idx := range indexes {
		disk := agentshost.UnraidDisk{
			Name:       strings.TrimSpace(fields[fmt.Sprintf("diskName.%d", idx)]),
			Device:     normalizeBlockDevice(firstNonEmpty(fields[fmt.Sprintf("rdevName.%d", idx)], fields[fmt.Sprintf("diskDevice.%d", idx)])),
			RawStatus:  strings.TrimSpace(firstNonEmpty(fields[fmt.Sprintf("rdevStatus.%d", idx)], fields[fmt.Sprintf("diskState.%d", idx)])),
			Serial:     strings.TrimSpace(firstNonEmpty(fields[fmt.Sprintf("rdevSerial.%d", idx)], fields[fmt.Sprintf("diskSerial.%d", idx)])),
			Filesystem: strings.TrimSpace(firstNonEmpty(fields[fmt.Sprintf("diskFsType.%d", idx)], fields[fmt.Sprintf("fsType.%d", idx)])),
			SizeBytes:  parseFirstInt64(fields[fmt.Sprintf("diskSize.%d", idx)], fields[fmt.Sprintf("rdevSize.%d", idx)]),
			Slot:       idx,
		}
		disk.Role = inferUnraidDiskRole(disk.Name, idx)
		disk.Status = normalizeUnraidDiskStatus(disk.RawStatus, disk.Device)
		if disk.Name == "" {
			disk.Name = defaultUnraidDiskName(disk.Role, idx)
		}
		if disk.Name == "" && disk.Device == "" && disk.RawStatus == "" && disk.Serial == "" && disk.SizeBytes == 0 {
			continue
		}
		disks = append(disks, disk)
	}

	if len(disks) > 0 {
		storage.Disks = disks
	}

	return storage, nil
}

func parseUnraidKeyValueOutput(output string) map[string]string {
	lines := strings.Split(output, "\n")
	fields := make(map[string]string, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		fields[key] = value
	}
	return fields
}

func collectUnraidIndexes(fields map[string]string) []int {
	indexes := make(map[int]struct{})
	prefixes := []string{
		"diskName.",
		"diskSize.",
		"diskState.",
		"diskFsType.",
		"rdevName.",
		"rdevStatus.",
		"rdevSerial.",
	}

	for key := range fields {
		for _, prefix := range prefixes {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			idx, err := strconv.Atoi(strings.TrimPrefix(key, prefix))
			if err != nil {
				continue
			}
			indexes[idx] = struct{}{}
			break
		}
	}

	out := make([]int, 0, len(indexes))
	for idx := range indexes {
		out = append(out, idx)
	}
	sort.Ints(out)
	return out
}

func normalizeBlockDevice(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return ""
	}
	if strings.HasPrefix(device, "/dev/") {
		return device
	}
	return "/dev/" + device
}

func inferUnraidDiskRole(name string, idx int) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch {
	case name == "parity" || name == "parity1" || strings.HasPrefix(name, "parity"):
		return "parity"
	case strings.HasPrefix(name, "disk"):
		return "data"
	case strings.HasPrefix(name, "cache") || strings.HasPrefix(name, "pool"):
		return "cache"
	case name == "flash":
		return "flash"
	case idx == 0:
		return "parity"
	default:
		return ""
	}
}

func defaultUnraidDiskName(role string, idx int) string {
	switch role {
	case "parity":
		if idx <= 0 {
			return "parity"
		}
		return fmt.Sprintf("parity%d", idx)
	case "data":
		return fmt.Sprintf("disk%d", idx)
	default:
		return ""
	}
}

func normalizeUnraidDiskStatus(raw string, device string) string {
	status := strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case status == "":
		if device != "" {
			return "online"
		}
		return ""
	case strings.Contains(status, "DISK_OK") || status == "OK":
		return "online"
	case strings.Contains(status, "DISK_DSBL") || strings.Contains(status, "DISABLED"):
		return "disabled"
	case strings.Contains(status, "DISK_NP") || strings.Contains(status, "MISSING") || strings.Contains(status, "NOT_INSTALLED"):
		return "missing"
	case strings.Contains(status, "DISK_INVALID") || strings.Contains(status, "INVALID"):
		return "invalid"
	case strings.Contains(status, "DISK_WRONG") || strings.Contains(status, "WRONG"):
		return "wrong"
	case strings.Contains(status, "DISK_ERROR") || strings.Contains(status, "ERROR"):
		return "error"
	default:
		return strings.ToLower(status)
	}
}

func normalizeUnraidSyncAction(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "", "idle", "none", "unknown":
		return ""
	default:
		return action
	}
}

func unraidSyncProgress(fields map[string]string) float64 {
	pos := parseFirstInt64(fields["mdResyncPos"], fields["mdResync"])
	size := parseFirstInt64(fields["mdResyncSize"], fields["mdSize"])
	if pos > 0 && size > 0 {
		progress := (float64(pos) / float64(size)) * 100
		if progress < 0 {
			return 0
		}
		if progress > 100 {
			return 100
		}
		return progress
	}
	if direct, ok := parseFloatField(fields, "mdResyncPct"); ok {
		if direct < 0 {
			return 0
		}
		if direct > 100 {
			return 100
		}
		return direct
	}
	return 0
}

func parseUnraidIntField(fields map[string]string, key string) int {
	value := strings.TrimSpace(fields[key])
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func parseUnraidInt64Field(fields map[string]string, key string) int64 {
	return parseFirstInt64(fields[key])
}

func parseFirstInt64(values ...string) int64 {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func parseFloatField(fields map[string]string, key string) (float64, bool) {
	value := strings.TrimSpace(fields[key])
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
