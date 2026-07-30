package hostagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	xcpngQueryTimeout       = 8 * time.Second
	xcpngMaxPoolOutputBytes = 64 * 1024
	xcpngMaxHostOutputBytes = 256 * 1024
	xcpngMaxVMOutputBytes   = 1024 * 1024
	xcpngMaxVMs             = 1024
	xcpngMaxNameBytes       = 256
)

var (
	xcpngPoolListArgs = []string{"pool-list", "params=uuid,name-label,master"}
	xcpngHostListArgs = []string{"host-list", "params=uuid,name-label,hostname"}
	xcpngVMListArgs   = []string{
		"vm-list",
		"is-control-domain=false",
		"is-a-template=false",
		"is-a-snapshot=false",
		"params=uuid,name-label,power-state,VCPUs-number,memory-actual,memory-static-max,resident-on",
	}
)

// collectXCPNGInventory auto-detects the pool-wide, read-only xe CLI exposed
// by an XCP-ng control domain. Hosts without xe continue reporting normally.
func (a *Agent) collectXCPNGInventory(ctx context.Context) *agentshost.XCPNGInventory {
	if a.collector.GOOS() != "linux" {
		return nil
	}
	path, err := a.collector.LookPath("xe")
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) && !os.IsNotExist(err) {
			a.logger.Debug().Err(err).Msg("Failed to locate xe for XCP-ng inventory")
		}
		return nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, xcpngQueryTimeout)
	defer cancel()

	poolOutput, err := collectCommandOutputLimited(
		queryCtx, a.collector, xcpngMaxPoolOutputBytes, path, xcpngPoolListArgs...,
	)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to list XCP-ng pools")
		return nil
	}
	pools, err := parseXERecords(poolOutput, xcpngMaxPoolOutputBytes, 2)
	if err != nil || len(pools) != 1 {
		a.logger.Debug().Err(err).Int("pools", len(pools)).Msg("Failed to parse one XCP-ng pool")
		return nil
	}
	poolUUID := canonicalXCPNGUUID(pools[0]["uuid"])
	if poolUUID == "" {
		return nil
	}

	hostOutput, err := collectCommandOutputLimited(
		queryCtx, a.collector, xcpngMaxHostOutputBytes, path, xcpngHostListArgs...,
	)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to list XCP-ng hosts")
		return nil
	}
	hosts, err := parseXERecords(hostOutput, xcpngMaxHostOutputBytes, 64)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse XCP-ng hosts")
		return nil
	}

	vmOutput, err := collectCommandOutputLimited(
		queryCtx, a.collector, xcpngMaxVMOutputBytes, path, xcpngVMListArgs...,
	)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to list XCP-ng virtual machines")
		return nil
	}
	records, err := parseXERecords(vmOutput, xcpngMaxVMOutputBytes, xcpngMaxVMs)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse XCP-ng virtual machines")
		return nil
	}

	inventory := &agentshost.XCPNGInventory{
		PoolUUID:      poolUUID,
		PoolName:      safeXCPNGName(pools[0]["name-label"]),
		MasterUUID:    canonicalXCPNGUUID(pools[0]["master"]),
		LocalHostUUID: localXCPNGHostUUID(hosts, a.hostname),
		VMs:           make([]agentshost.XCPNGVM, 0, len(records)),
		CollectedAt:   a.collector.Now().UTC(),
	}
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		uuid := canonicalXCPNGUUID(record["uuid"])
		name := safeXCPNGName(record["name-label"])
		if uuid == "" || name == "" {
			continue
		}
		if _, exists := seen[uuid]; exists {
			continue
		}
		seen[uuid] = struct{}{}
		vcpus, _ := strconv.Atoi(strings.TrimSpace(record["vcpus-number"]))
		memory, _ := strconv.ParseInt(strings.TrimSpace(record["memory-actual"]), 10, 64)
		memoryMax, _ := strconv.ParseInt(strings.TrimSpace(record["memory-static-max"]), 10, 64)
		inventory.VMs = append(inventory.VMs, agentshost.XCPNGVM{
			UUID:             uuid,
			Name:             name,
			PowerState:       normalizeXCPNGPowerState(record["power-state"]),
			VCPUs:            max(0, min(vcpus, 4096)),
			MemoryActual:     max(int64(0), memory),
			MemoryStaticMax:  max(int64(0), memoryMax),
			ResidentHostUUID: canonicalXCPNGUUID(record["resident-on"]),
		})
	}
	sort.Slice(inventory.VMs, func(i, j int) bool {
		return strings.ToLower(inventory.VMs[i].Name) < strings.ToLower(inventory.VMs[j].Name)
	})
	return inventory
}

func parseXERecords(output string, maxBytes, maxRecords int) ([]map[string]string, error) {
	if len(output) > maxBytes {
		return nil, fmt.Errorf("xe output exceeds %d bytes", maxBytes)
	}
	records := make([]map[string]string, 0)
	current := make(map[string]string)
	flush := func() {
		if len(current) == 0 || len(records) == maxRecords {
			current = make(map[string]string)
			return
		}
		records = append(records, current)
		current = make(map[string]string)
	}
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" {
			flush()
			continue
		}
		closeParen := strings.IndexByte(line, ')')
		if closeParen < 0 {
			continue
		}
		colon := strings.IndexByte(line[closeParen+1:], ':')
		if colon < 0 {
			continue
		}
		colon += closeParen + 1
		key := strings.ToLower(strings.TrimSpace(strings.SplitN(line[:closeParen], "(", 2)[0]))
		value := strings.TrimSpace(line[colon+1:])
		if key == "" || len(key) > 64 || len(value) > xcpngMaxNameBytes*4 || !utf8.ValidString(value) {
			continue
		}
		current[key] = value
	}
	flush()
	return records, nil
}

func localXCPNGHostUUID(hosts []map[string]string, hostname string) string {
	hostname = strings.TrimSpace(hostname)
	short := strings.SplitN(hostname, ".", 2)[0]
	for _, host := range hosts {
		for _, candidate := range []string{host["hostname"], host["name-label"]} {
			candidate = strings.TrimSpace(candidate)
			candidateShort := strings.SplitN(candidate, ".", 2)[0]
			if candidate != "" &&
				(strings.EqualFold(candidate, hostname) || strings.EqualFold(candidateShort, short)) {
				return canonicalXCPNGUUID(host["uuid"])
			}
		}
	}
	if len(hosts) == 1 {
		return canonicalXCPNGUUID(hosts[0]["uuid"])
	}
	return ""
}

func canonicalXCPNGUUID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return ""
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return ""
		}
	}
	return value
}

func safeXCPNGName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > xcpngMaxNameBytes || !utf8.ValidString(value) {
		return ""
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return value
}

func normalizeXCPNGPowerState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "running", "halted", "paused", "suspended":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}
