package hostagent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// resolvePctPath mirrors resolveSmartctlPath: PULSE_PCT_PATH lets a
// least-privilege install point the read-only pct list/df queries at a scoped
// privilege helper instead of requiring the whole agent to run as root. The
// override must be absolute so a PATH-relative name cannot be hijacked.
func resolvePctPath(lookPath func(string) (string, error)) (string, error) {
	if configured := strings.TrimSpace(os.Getenv("PULSE_PCT_PATH")); configured != "" {
		if !filepath.IsAbs(configured) {
			return "", fmt.Errorf("PULSE_PCT_PATH must be an absolute path")
		}
		return configured, nil
	}
	return lookPath("pct")
}

const (
	proxmoxLXCQueryTimeout       = 10 * time.Second
	proxmoxLXCMaxContainers      = 128
	proxmoxLXCMaxDisks           = 257
	proxmoxLXCMaxListOutputBytes = 64 * 1024
	proxmoxLXCMaxDFOutputBytes   = 64 * 1024
	proxmoxLXCMaxNameBytes       = 256
	proxmoxLXCMaxLabelBytes      = 512
)

type proxmoxLXCRunningContainer struct {
	VMID int
	Name string
}

// collectProxmoxLXCFilesystems auto-detects the local Proxmox Container
// Toolkit. It lists node-local guests, then runs pct df only for containers
// that the same list reported as running. Collection is read-only and
// best-effort; a failed per-container query does not suppress other results.
func (a *Agent) collectProxmoxLXCFilesystems(ctx context.Context) *agentshost.ProxmoxLXCInventory {
	if a.collector.GOOS() != "linux" {
		return nil
	}
	pctPath, err := resolvePctPath(a.collector.LookPath)
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) && !os.IsNotExist(err) {
			a.logger.Debug().Err(err).Msg("Failed to locate pct for Proxmox LXC filesystems")
		}
		return nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, proxmoxLXCQueryTimeout)
	defer cancel()

	listOutput, err := collectCommandOutputLimited(
		queryCtx,
		a.collector,
		proxmoxLXCMaxListOutputBytes,
		pctPath,
		"list",
	)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to list local Proxmox LXC containers")
		return nil
	}
	containers, err := parseProxmoxLXCRunningContainers(listOutput)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse local Proxmox LXC container list")
		return nil
	}

	inventory := &agentshost.ProxmoxLXCInventory{
		Containers:  []agentshost.ProxmoxLXCContainer{},
		CollectedAt: a.collector.Now().UTC(),
	}
	for _, container := range containers {
		if queryCtx.Err() != nil {
			break
		}
		output, outputErr := collectCommandOutputLimited(
			queryCtx,
			a.collector,
			proxmoxLXCMaxDFOutputBytes,
			pctPath,
			"df",
			strconv.Itoa(container.VMID),
		)
		if outputErr != nil {
			a.logger.Debug().
				Err(outputErr).
				Int("vmid", container.VMID).
				Msg("Failed to collect Proxmox LXC filesystem usage")
			continue
		}
		disks, parseErr := parseProxmoxLXCDF(output)
		if parseErr != nil || len(disks) == 0 {
			a.logger.Debug().
				Err(parseErr).
				Int("vmid", container.VMID).
				Msg("Failed to parse Proxmox LXC filesystem usage")
			continue
		}
		inventory.Containers = append(inventory.Containers, agentshost.ProxmoxLXCContainer{
			VMID:  container.VMID,
			Name:  container.Name,
			Disks: disks,
		})
	}

	return inventory
}

func parseProxmoxLXCRunningContainers(output string) ([]proxmoxLXCRunningContainer, error) {
	if len(output) > proxmoxLXCMaxListOutputBytes {
		return nil, fmt.Errorf("pct list output exceeds %d bytes", proxmoxLXCMaxListOutputBytes)
	}

	result := make([]proxmoxLXCRunningContainer, 0)
	seen := make(map[int]struct{})
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "VMID") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[1], "running") {
			continue
		}
		vmid, err := strconv.Atoi(fields[0])
		if err != nil || vmid < 100 || vmid > 999999999 {
			continue
		}
		if _, exists := seen[vmid]; exists {
			continue
		}

		name := ""
		if len(line) > 35 {
			name = strings.TrimSpace(line[35:])
		} else if len(fields) >= 3 {
			name = fields[len(fields)-1]
		}
		if !safeProxmoxLXCText(name, proxmoxLXCMaxNameBytes) {
			continue
		}

		seen[vmid] = struct{}{}
		result = append(result, proxmoxLXCRunningContainer{VMID: vmid, Name: name})
		if len(result) == proxmoxLXCMaxContainers {
			break
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].VMID < result[j].VMID })
	return result, nil
}

func parseProxmoxLXCDF(output string) ([]agentshost.Disk, error) {
	if len(output) > proxmoxLXCMaxDFOutputBytes {
		return nil, fmt.Errorf("pct df output exceeds %d bytes", proxmoxLXCMaxDFOutputBytes)
	}

	result := make([]agentshost.Disk, 0)
	seen := make(map[string]struct{})
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "MP ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		diskType := strings.ToLower(strings.TrimSpace(fields[0]))
		if diskType != "rootfs" && !validProxmoxLXCMountKey(diskType) {
			continue
		}
		device := strings.TrimSpace(fields[1])
		if !safeProxmoxLXCText(device, proxmoxLXCMaxLabelBytes) {
			continue
		}
		total, totalOK := parseProxmoxLXCSize(fields[2])
		used, usedOK := parseProxmoxLXCSize(fields[3])
		free, freeOK := parseProxmoxLXCSize(fields[4])
		usage, usageOK := parseProxmoxLXCUsage(fields[5])
		if !totalOK || !usedOK || !freeOK || !usageOK || total <= 0 {
			continue
		}
		if used > total {
			used = total
		}
		if free > total {
			free = total
		}

		mountpoint := strings.TrimSpace(strings.Join(fields[6:], " "))
		if !validProxmoxLXCMountpoint(mountpoint) {
			continue
		}
		key := strings.ToLower(mountpoint)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		result = append(result, agentshost.Disk{
			Device:     device,
			Mountpoint: mountpoint,
			Type:       diskType,
			TotalBytes: total,
			UsedBytes:  used,
			FreeBytes:  free,
			Usage:      usage,
		})
		if len(result) == proxmoxLXCMaxDisks {
			break
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Mountpoint == "/" {
			return true
		}
		if result[j].Mountpoint == "/" {
			return false
		}
		return result[i].Mountpoint < result[j].Mountpoint
	})
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func parseProxmoxLXCSize(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	multiplier := float64(1)
	last := value[len(value)-1]
	switch last {
	case 'K', 'k':
		multiplier = 1 << 10
	case 'M', 'm':
		multiplier = 1 << 20
	case 'G', 'g':
		multiplier = 1 << 30
	case 'T', 't':
		multiplier = 1 << 40
	default:
		last = 0
	}
	if last != 0 {
		value = strings.TrimSpace(value[:len(value)-1])
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		return 0, false
	}
	bytes := parsed * multiplier
	if bytes > math.MaxInt64 {
		return 0, false
	}
	return int64(math.Round(bytes)), true
}

func parseProxmoxLXCUsage(value string) (float64, bool) {
	value = strings.TrimSuffix(strings.TrimSpace(value), "%")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 || parsed > 100 {
		return 0, false
	}
	return parsed, true
}

func validProxmoxLXCMountKey(value string) bool {
	if !strings.HasPrefix(value, "mp") || len(value) < 3 || len(value) > 5 {
		return false
	}
	index, err := strconv.Atoi(value[2:])
	return err == nil && index >= 0 && index <= 255
}

func validProxmoxLXCMountpoint(value string) bool {
	if !safeProxmoxLXCText(value, proxmoxLXCMaxLabelBytes) || !strings.HasPrefix(value, "/") {
		return false
	}
	cleaned := path.Clean(value)
	return cleaned == value && !strings.Contains(value, "\x00")
}

func safeProxmoxLXCText(value string, maxBytes int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}
