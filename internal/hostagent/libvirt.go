package hostagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
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
	libvirtQueryTimeout        = 8 * time.Second
	libvirtMaxDomains          = 128
	libvirtMaxDomainNameBytes  = 256
	libvirtMaxListOutputBytes  = 64 * 1024
	libvirtMaxStatsOutputBytes = 1024 * 1024
)

var libvirtStatsArgs = []string{
	"--readonly",
	"domstats",
	"--raw",
	"--nowait",
	"--state",
	"--cpu-total",
	"--balloon",
	"--vcpu",
	"--interface",
	"--block",
}

type limitedCommandOutputCollector interface {
	CommandCombinedOutputLimited(
		ctx context.Context,
		maxBytes int,
		name string,
		arg ...string,
	) (string, error)
}

func collectCommandOutputLimited(
	ctx context.Context,
	collector SystemCollector,
	maxBytes int,
	name string,
	arg ...string,
) (string, error) {
	if limited, ok := collector.(limitedCommandOutputCollector); ok {
		return limited.CommandCombinedOutputLimited(ctx, maxBytes, name, arg...)
	}
	output, err := collector.CommandCombinedOutput(ctx, name, arg...)
	if len(output) > maxBytes {
		return output[:maxBytes], fmt.Errorf("command output exceeds %d bytes", maxBytes)
	}
	return output, err
}

// collectLibvirtInventory auto-detects a local read-only libvirt connection.
// It is deliberately best-effort: hosts without virsh, socket access, or a
// compatible driver continue reporting normal host telemetry.
func (a *Agent) collectLibvirtInventory(ctx context.Context) *agentshost.LibvirtInventory {
	if a.collector.GOOS() != "linux" {
		return nil
	}

	path, err := a.collector.LookPath("virsh")
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) && !os.IsNotExist(err) {
			a.logger.Debug().Err(err).Msg("Failed to locate virsh for libvirt inventory")
		}
		return nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, libvirtQueryTimeout)
	defer cancel()

	listOutput, err := collectCommandOutputLimited(
		queryCtx,
		a.collector,
		libvirtMaxListOutputBytes,
		path,
		"--readonly",
		"list",
		"--all",
		"--name",
	)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to list local libvirt domains")
		return nil
	}
	names, err := parseLibvirtDomainList(listOutput)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse local libvirt domain list")
		return nil
	}

	inventory := &agentshost.LibvirtInventory{
		Domains:     []agentshost.LibvirtDomain{},
		CollectedAt: a.collector.Now().UTC(),
	}
	if len(names) == 0 {
		return inventory
	}

	args := make([]string, 0, len(libvirtStatsArgs)+len(names))
	args = append(args, libvirtStatsArgs...)
	args = append(args, names...)
	statsOutput, err := collectCommandOutputLimited(
		queryCtx,
		a.collector,
		libvirtMaxStatsOutputBytes,
		path,
		args...,
	)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect local libvirt domain stats")
		return nil
	}
	domains, err := parseLibvirtDomainStats(statsOutput, names)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse local libvirt domain stats")
		return nil
	}
	inventory.Domains = domains
	return inventory
}

func parseLibvirtDomainList(output string) ([]string, error) {
	if len(output) > libvirtMaxListOutputBytes {
		return nil, fmt.Errorf("libvirt domain list exceeds %d bytes", libvirtMaxListOutputBytes)
	}

	names := make([]string, 0)
	seen := make(map[string]struct{})
	for _, raw := range strings.Split(strings.TrimSpace(output), "\n") {
		name := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if !validLibvirtDomainName(name) {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
		if len(names) == libvirtMaxDomains {
			break
		}
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names, nil
}

func validLibvirtDomainName(name string) bool {
	if name == "" ||
		len(name) > libvirtMaxDomainNameBytes ||
		!utf8.ValidString(name) ||
		strings.HasPrefix(name, "-") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func parseLibvirtDomainStats(output string, expectedNames []string) ([]agentshost.LibvirtDomain, error) {
	if len(output) > libvirtMaxStatsOutputBytes {
		return nil, fmt.Errorf("libvirt domain stats exceed %d bytes", libvirtMaxStatsOutputBytes)
	}

	expected := make(map[string]string, len(expectedNames))
	for _, name := range expectedNames {
		if validLibvirtDomainName(name) {
			expected[strings.ToLower(name)] = name
		}
	}

	records := make(map[string]*agentshost.LibvirtDomain, len(expected))
	var current *agentshost.LibvirtDomain
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if strings.HasPrefix(line, "Domain: '") && strings.HasSuffix(line, "'") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "Domain: '"), "'")
			canonical, ok := expected[strings.ToLower(name)]
			if !ok {
				current = nil
				continue
			}
			key := strings.ToLower(canonical)
			if existing := records[key]; existing != nil {
				current = existing
				continue
			}
			domain := agentshost.LibvirtDomain{
				ID:    libvirtDomainID(canonical),
				Name:  canonical,
				State: "unknown",
			}
			records[key] = &domain
			current = &domain
			continue
		}
		if current == nil {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		applyLibvirtDomainStat(current, strings.TrimSpace(key), strings.TrimSpace(value))
	}

	result := make([]agentshost.LibvirtDomain, 0, len(expected))
	for _, name := range expectedNames {
		if !validLibvirtDomainName(name) {
			continue
		}
		key := strings.ToLower(name)
		if domain := records[key]; domain != nil {
			result = append(result, *domain)
		} else {
			result = append(result, agentshost.LibvirtDomain{
				ID:    libvirtDomainID(name),
				Name:  name,
				State: "unknown",
			})
		}
		if len(result) == libvirtMaxDomains {
			break
		}
	}
	return result, nil
}

func applyLibvirtDomainStat(domain *agentshost.LibvirtDomain, key, value string) {
	if domain == nil {
		return
	}
	switch key {
	case "state.state":
		if state, ok := parseLibvirtUint(value); ok {
			domain.State = libvirtDomainState(state)
		}
	case "cpu.time":
		if parsed, ok := parseLibvirtUint(value); ok {
			domain.CPUTimeNanoseconds = parsed
		}
	case "vcpu.current":
		if parsed, ok := parseLibvirtUint(value); ok && parsed <= math.MaxInt32 {
			domain.VCPUs = int(parsed)
		}
	case "balloon.current":
		if parsed, ok := parseLibvirtUint(value); ok {
			domain.MemoryCurrentBytes = libvirtKiBToBytes(parsed)
		}
	case "balloon.maximum":
		if parsed, ok := parseLibvirtUint(value); ok {
			domain.MemoryMaximumBytes = libvirtKiBToBytes(parsed)
		}
	default:
		parsed, ok := parseLibvirtUint(value)
		if !ok {
			return
		}
		switch {
		case strings.HasPrefix(key, "net.") && strings.HasSuffix(key, ".rx.bytes"):
			domain.NetworkRXBytes = saturatingAddUint64(domain.NetworkRXBytes, parsed)
		case strings.HasPrefix(key, "net.") && strings.HasSuffix(key, ".tx.bytes"):
			domain.NetworkTXBytes = saturatingAddUint64(domain.NetworkTXBytes, parsed)
		case strings.HasPrefix(key, "block.") && strings.HasSuffix(key, ".rd.bytes"):
			domain.DiskReadBytes = saturatingAddUint64(domain.DiskReadBytes, parsed)
		case strings.HasPrefix(key, "block.") && strings.HasSuffix(key, ".wr.bytes"):
			domain.DiskWriteBytes = saturatingAddUint64(domain.DiskWriteBytes, parsed)
		}
	}
}

func parseLibvirtUint(value string) (uint64, bool) {
	value = strings.TrimSpace(strings.Trim(value, "'\""))
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil
}

func libvirtDomainState(value uint64) string {
	switch value {
	case 1:
		return "running"
	case 2:
		return "blocked"
	case 3:
		return "paused"
	case 4:
		return "shutting-down"
	case 5:
		return "shutoff"
	case 6:
		return "crashed"
	case 7:
		return "suspended"
	default:
		return "unknown"
	}
}

func libvirtKiBToBytes(value uint64) int64 {
	if value > uint64(math.MaxInt64/1024) {
		return math.MaxInt64
	}
	return int64(value * 1024)
}

func saturatingAddUint64(left, right uint64) uint64 {
	if math.MaxUint64-left < right {
		return math.MaxUint64
	}
	return left + right
}

func libvirtDomainID(name string) string {
	sum := sha256.Sum256([]byte(name))
	return "domain-" + hex.EncodeToString(sum[:8])
}
