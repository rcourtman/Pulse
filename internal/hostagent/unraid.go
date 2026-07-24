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
	"github.com/rs/zerolog/log"
)

const hostAgentUnraidVersionPath = "/etc/unraid-version"
const hostAgentUnraidDisksINIPath = "/var/local/emhttp/disks.ini"

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

	// disks.ini is Unraid's native membership inventory. Read it before mdcmd
	// so assigned disks remain reportable even when the status command is
	// unavailable, slow, or canceled by the caller.
	var iniDisks []agentshost.UnraidDisk
	if data, err := collector.ReadFile(hostAgentUnraidDisksINIPath); err == nil {
		iniDisks = parseUnraidDisksINI(string(data))
	} else {
		log.Debug().
			Str("component", "unraid_collector").
			Str("action", "native_inventory_unavailable").
			Err(err).
			Msg("Unable to read Unraid native disk inventory; mdcmd remains available as fallback")
	}

	mdcmdPath, err := resolveUnraidMdcmdBinary(collector)
	if err != nil {
		if len(iniDisks) > 0 {
			return unraidNativeInventoryFallback(iniDisks, err), nil
		}
		return nil, err
	}

	statusCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	output, err := collector.CommandCombinedOutput(statusCtx, mdcmdPath, "status")
	if err != nil {
		if len(iniDisks) > 0 {
			return unraidNativeInventoryFallback(iniDisks, fmt.Errorf("run mdcmd status: %w", err)), nil
		}
		return nil, fmt.Errorf("run mdcmd status: %w", err)
	}

	storage, err := parseUnraidStatusOutput(output)
	if err != nil {
		if len(iniDisks) > 0 {
			return unraidNativeInventoryFallback(iniDisks, err), nil
		}
		return nil, err
	}
	storage = mergeUnraidDiskINI(storage, iniDisks)
	return reconcileUnraidDiskCounts(storage), nil
}

func unraidNativeInventoryFallback(disks []agentshost.UnraidDisk, cause error) *agentshost.UnraidStorage {
	log.Debug().
		Str("component", "unraid_collector").
		Str("action", "native_inventory_fallback").
		Int("disk_count", len(disks)).
		Err(cause).
		Msg("Reporting Unraid native disk inventory without mdcmd runtime state")
	return reconcileUnraidDiskCounts(mergeUnraidDiskINI(nil, disks))
}

// reconcileUnraidDiskCounts makes structured native disk states authoritative
// over aggregate mdcmd counters when those states are available. This avoids
// stale or capability-related aggregate values turning healthy assigned disks
// into false missing/disabled alerts, while retaining aggregate fallback on
// older Unraid responses that do not expose per-disk state.
func reconcileUnraidDiskCounts(storage *agentshost.UnraidStorage) *agentshost.UnraidStorage {
	if storage == nil {
		return nil
	}

	hasStructuredStatus := false
	disabled, invalid, missing := 0, 0, 0
	for _, disk := range storage.Disks {
		if isUnraidEmptySlot(disk) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(disk.Status)) {
		case "":
			continue
		case "disabled":
			hasStructuredStatus = true
			disabled++
		case "invalid":
			hasStructuredStatus = true
			invalid++
		case "missing":
			hasStructuredStatus = true
			missing++
		default:
			hasStructuredStatus = true
		}
	}
	if hasStructuredStatus {
		storage.NumDisabled = disabled
		storage.NumInvalid = invalid
		storage.NumMissing = missing
	}
	return storage
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

	syncAction := unraidSyncAction(fields)
	storage := &agentshost.UnraidStorage{
		ArrayState:   strings.ToUpper(strings.TrimSpace(fields["mdState"])),
		ArrayStarted: strings.EqualFold(strings.TrimSpace(fields["mdState"]), "STARTED"),
		SyncAction:   syncAction,
		SyncProgress: unraidSyncProgress(fields, syncAction),
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
			Serial:     strings.TrimSpace(firstNonEmpty(fields[fmt.Sprintf("rdevSerial.%d", idx)], fields[fmt.Sprintf("diskSerial.%d", idx)], fields[fmt.Sprintf("rdevId.%d", idx)], fields[fmt.Sprintf("diskId.%d", idx)])),
			Filesystem: strings.TrimSpace(firstNonEmpty(fields[fmt.Sprintf("diskFsType.%d", idx)], fields[fmt.Sprintf("fsType.%d", idx)])),
			SizeBytes:  parseUnraidKiBAsBytes(firstNonEmpty(fields[fmt.Sprintf("diskSize.%d", idx)], fields[fmt.Sprintf("rdevSize.%d", idx)])),
			Slot:       idx,
		}
		disk.Role = inferUnraidDiskRole(disk.Name, idx)
		disk.Status = normalizeUnraidDiskStatus(disk.RawStatus, disk.Device)
		if isUnraidEmptySlot(disk) {
			continue
		}
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

type unraidINISection struct {
	name   string
	fields map[string]string
}

func parseUnraidDisksINI(input string) []agentshost.UnraidDisk {
	sections := parseUnraidINISections(input)
	disks := make([]agentshost.UnraidDisk, 0, len(sections))
	for _, section := range sections {
		if section.name == "" {
			continue
		}
		fields := section.fields
		idx := parseUnraidIntField(fields, "idx")
		name := firstNonEmpty(fields["name"], section.name)
		role := normalizeUnraidDiskRole(firstNonEmpty(fields["type"], inferUnraidDiskRole(name, idx)))
		device := normalizeBlockDevice(fields["device"])
		rawStatus := strings.TrimSpace(fields["status"])
		disk := agentshost.UnraidDisk{
			Name:        strings.TrimSpace(name),
			Device:      device,
			Role:        role,
			Status:      normalizeUnraidDiskStatus(rawStatus, device),
			RawStatus:   rawStatus,
			Filesystem:  strings.TrimSpace(fields["fsType"]),
			Transport:   normalizeUnraidTransport(fields["transport"], fields["rotational"]),
			SizeBytes:   parseUnraidSizeBytes(fields),
			UsedBytes:   parseUnraidKiBAsBytes(fields["fsUsed"]),
			FreeBytes:   parseUnraidKiBAsBytes(fields["fsFree"]),
			Temperature: parseUnraidTemperature(fields["temp"]),
			SpunDown:    parseUnraidBool(fields["spundown"]),
			ReadCount:   parseFirstInt64(fields["numReads"]),
			WriteCount:  parseFirstInt64(fields["numWrites"]),
			ErrorCount:  parseFirstInt64(fields["numErrors"]),
			Slot:        idx,
		}
		disk.Model, disk.Serial = parseUnraidDiskIdentity(firstNonEmpty(fields["id"], fields["idSb"]))
		if disk.Serial == "" {
			disk.Serial = strings.TrimSpace(firstNonEmpty(fields["id"], fields["idSb"]))
		}
		if isUnraidEmptySlot(disk) {
			continue
		}
		if disk.Name == "" {
			disk.Name = defaultUnraidDiskName(disk.Role, idx)
		}
		if disk.Name == "" && disk.Device == "" && disk.RawStatus == "" && disk.Serial == "" && disk.SizeBytes == 0 {
			continue
		}
		disks = append(disks, disk)
	}
	return disks
}

func parseUnraidINISections(input string) []unraidINISection {
	lines := strings.Split(input, "\n")
	sections := make([]unraidINISection, 0)
	var current *unraidINISection
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			name = strings.Trim(name, `"`)
			sections = append(sections, unraidINISection{name: name, fields: make(map[string]string)})
			current = &sections[len(sections)-1]
			continue
		}
		if current == nil || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		if key != "" {
			current.fields[key] = value
		}
	}
	return sections
}

func mergeUnraidDiskINI(storage *agentshost.UnraidStorage, iniDisks []agentshost.UnraidDisk) *agentshost.UnraidStorage {
	if storage == nil {
		storage = &agentshost.UnraidStorage{}
	}
	if len(iniDisks) == 0 {
		return storage
	}

	merged := make([]agentshost.UnraidDisk, 0, len(storage.Disks)+len(iniDisks))
	bySlot := make(map[int]int, len(storage.Disks))
	for _, disk := range storage.Disks {
		bySlot[disk.Slot] = len(merged)
		merged = append(merged, disk)
	}

	for _, disk := range iniDisks {
		if pos, ok := bySlot[disk.Slot]; ok {
			merged[pos] = mergeUnraidDisk(merged[pos], disk)
			continue
		}
		bySlot[disk.Slot] = len(merged)
		merged = append(merged, disk)
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Slot < merged[j].Slot
	})
	storage.Disks = merged
	return storage
}

func mergeUnraidDisk(base, incoming agentshost.UnraidDisk) agentshost.UnraidDisk {
	out := base
	if strings.TrimSpace(incoming.Name) != "" {
		out.Name = strings.TrimSpace(incoming.Name)
	}
	if strings.TrimSpace(incoming.Device) != "" {
		out.Device = strings.TrimSpace(incoming.Device)
	}
	if strings.TrimSpace(incoming.Role) != "" {
		out.Role = strings.TrimSpace(incoming.Role)
	}
	if strings.TrimSpace(incoming.Status) != "" {
		out.Status = strings.TrimSpace(incoming.Status)
	}
	if strings.TrimSpace(incoming.RawStatus) != "" {
		out.RawStatus = strings.TrimSpace(incoming.RawStatus)
	}
	if strings.TrimSpace(incoming.Model) != "" {
		out.Model = strings.TrimSpace(incoming.Model)
	}
	if strings.TrimSpace(incoming.Serial) != "" {
		out.Serial = strings.TrimSpace(incoming.Serial)
	}
	if strings.TrimSpace(incoming.Filesystem) != "" {
		out.Filesystem = strings.TrimSpace(incoming.Filesystem)
	}
	if strings.TrimSpace(incoming.Transport) != "" {
		out.Transport = strings.TrimSpace(incoming.Transport)
	}
	if incoming.SizeBytes > 0 {
		out.SizeBytes = incoming.SizeBytes
	}
	if incoming.UsedBytes > 0 {
		out.UsedBytes = incoming.UsedBytes
	}
	if incoming.FreeBytes > 0 {
		out.FreeBytes = incoming.FreeBytes
	}
	if incoming.Temperature > 0 {
		out.Temperature = incoming.Temperature
	}
	out.SpunDown = incoming.SpunDown
	if incoming.ReadCount > 0 {
		out.ReadCount = incoming.ReadCount
	}
	if incoming.WriteCount > 0 {
		out.WriteCount = incoming.WriteCount
	}
	if incoming.ErrorCount > 0 {
		out.ErrorCount = incoming.ErrorCount
	}
	if incoming.Slot > 0 || out.Slot == 0 {
		out.Slot = incoming.Slot
	}
	return out
}

func collectUnraidIndexes(fields map[string]string) []int {
	indexes := make(map[int]struct{})
	prefixes := []string{
		"diskName.",
		"diskSize.",
		"diskState.",
		"diskFsType.",
		"diskId.",
		"rdevName.",
		"rdevStatus.",
		"rdevSerial.",
		"rdevId.",
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
	case idx == 29:
		return "parity"
	case idx > 0 && idx < 29:
		return "data"
	case idx >= 30:
		return "cache"
	default:
		return ""
	}
}

func normalizeUnraidDiskRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "parity", "data", "cache", "flash":
		return role
	case "array":
		return "data"
	default:
		return role
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

func isUnraidEmptySlot(disk agentshost.UnraidDisk) bool {
	rawStatus := strings.ToUpper(strings.TrimSpace(disk.RawStatus))
	status := strings.ToLower(strings.TrimSpace(disk.Status))
	if !strings.Contains(rawStatus, "DISK_NP") && status != "missing" {
		return false
	}
	return strings.TrimSpace(disk.Name) == "" &&
		strings.TrimSpace(disk.Device) == "" &&
		strings.TrimSpace(disk.Serial) == "" &&
		strings.TrimSpace(disk.Filesystem) == "" &&
		disk.SizeBytes == 0
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

// unraidSyncAction returns the active sync action only when a resync is
// actually running. Unraid's mdResyncAction field can retain its last value
// after the sync is canceled (e.g., "check" stays set even after the parity
// check is stopped). The mdResync/mdResyncPos field is the authoritative
// indicator: it is 0 when no resync is running and non-zero (the current
// position) when one is active.
func unraidSyncAction(fields map[string]string) string {
	pos := parseFirstInt64(fields["mdResyncPos"], fields["mdResync"])
	if pos <= 0 {
		return ""
	}
	return normalizeUnraidSyncAction(fields["mdResyncAction"])
}

func unraidSyncProgress(fields map[string]string, syncAction string) float64 {
	if strings.TrimSpace(syncAction) == "" {
		return 0
	}
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

func parseUnraidKiBAsBytes(value string) int64 {
	parsed := parseFirstInt64(value)
	if parsed <= 0 {
		return 0
	}
	return parsed * 1024
}

func parseUnraidSizeBytes(fields map[string]string) int64 {
	sectors := parseFirstInt64(fields["sectors"])
	sectorSize := parseFirstInt64(fields["sector_size"])
	if sectors > 0 && sectorSize > 0 {
		return sectors * sectorSize
	}
	return parseUnraidKiBAsBytes(firstNonEmpty(fields["size"], fields["sizeSb"]))
}

func parseUnraidTemperature(value string) int {
	value = strings.TrimSpace(value)
	if value == "" || value == "*" || strings.EqualFold(value, "na") {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func parseUnraidBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeUnraidTransport(transport string, rotational string) string {
	transport = strings.ToLower(strings.TrimSpace(transport))
	switch transport {
	case "ata":
		return "sata"
	case "nvme", "sata", "sas", "usb":
		return transport
	}
	if strings.TrimSpace(rotational) == "0" {
		return "ssd"
	}
	return transport
}

func parseUnraidDiskIdentity(id string) (string, string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ""
	}
	idx := strings.LastIndex(id, "_")
	if idx <= 0 || idx >= len(id)-1 {
		return strings.ReplaceAll(id, "_", " "), ""
	}
	model := strings.ReplaceAll(strings.TrimSpace(id[:idx]), "_", " ")
	serial := strings.TrimSpace(id[idx+1:])
	return model, serial
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
