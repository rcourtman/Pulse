package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

const (
	guestMetadataCacheTTL    = 5 * time.Minute
	defaultGuestMetadataHold = 15 * time.Second

	// Guest agent timeout defaults (configurable via environment variables)
	// Increased from 3-5s to 10-15s to handle high-load environments better (refs #592)
	defaultGuestAgentFSInfoTimeout  = 15 * time.Second // GUEST_AGENT_FSINFO_TIMEOUT
	defaultGuestAgentNetworkTimeout = 10 * time.Second // GUEST_AGENT_NETWORK_TIMEOUT
	defaultGuestAgentOSInfoTimeout  = 10 * time.Second // GUEST_AGENT_OSINFO_TIMEOUT
	defaultGuestAgentVersionTimeout = 10 * time.Second // GUEST_AGENT_VERSION_TIMEOUT
	defaultGuestAgentRetries        = 1                // GUEST_AGENT_RETRIES (0 = no retry, 1 = one retry)
	defaultGuestAgentRetryDelay     = 500 * time.Millisecond

	// Skip OS info calls after this many consecutive failures to avoid triggering buggy guest agents (refs #692)
	guestAgentOSInfoFailureThreshold = 3
)

// guestMetadataCacheEntry holds cached guest agent metadata for a VM.
type guestMetadataCacheEntry struct {
	ipAddresses        []string
	networkInterfaces  []models.GuestNetworkInterface
	osName             string
	osVersion          string
	agentVersion       string
	fetchedAt          time.Time
	osInfoFailureCount int  // Track consecutive OS info failures
	osInfoSkip         bool // Skip OS info calls after repeated failures (refs #692)
}

func (m *Monitor) tryReserveGuestMetadataFetch(key string, now time.Time) bool {
	if m == nil {
		return false
	}
	m.guestMetadataLimiterMu.Lock()
	defer m.guestMetadataLimiterMu.Unlock()

	if next, ok := m.guestMetadataLimiter[key]; ok && now.Before(next) {
		return false
	}
	hold := m.guestMetadataHoldDuration
	if hold <= 0 {
		hold = defaultGuestMetadataHold
	}
	m.guestMetadataLimiter[key] = now.Add(hold)
	return true
}

func (m *Monitor) scheduleNextGuestMetadataFetch(key string, now time.Time) {
	if m == nil {
		return
	}
	interval := m.guestMetadataMinRefresh
	if interval <= 0 {
		interval = config.DefaultGuestMetadataMinRefresh
	}
	jitter := m.guestMetadataRefreshJitter
	if jitter > 0 && m.rng != nil {
		interval += time.Duration(m.rng.Int63n(int64(jitter)))
	}
	m.guestMetadataLimiterMu.Lock()
	m.guestMetadataLimiter[key] = now.Add(interval)
	m.guestMetadataLimiterMu.Unlock()
}

func (m *Monitor) deferGuestMetadataRetry(key string, now time.Time) {
	if m == nil {
		return
	}
	backoff := m.guestMetadataRetryBackoff
	if backoff <= 0 {
		backoff = config.DefaultGuestMetadataRetryBackoff
	}
	m.guestMetadataLimiterMu.Lock()
	m.guestMetadataLimiter[key] = now.Add(backoff)
	m.guestMetadataLimiterMu.Unlock()
}

func (m *Monitor) acquireGuestMetadataSlot(ctx context.Context) bool {
	if m == nil || m.guestMetadataSlots == nil {
		return true
	}
	select {
	case m.guestMetadataSlots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *Monitor) releaseGuestMetadataSlot() {
	if m == nil || m.guestMetadataSlots == nil {
		return
	}
	select {
	case <-m.guestMetadataSlots:
	default:
	}
}

// retryGuestAgentCall executes a guest agent API call with timeout and retry logic (refs #592)
func (m *Monitor) retryGuestAgentCall(ctx context.Context, timeout time.Duration, maxRetries int, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	if fn == nil {
		return nil, fmt.Errorf("guest agent call function is nil")
	}

	if timeout <= 0 {
		log.Warn().
			Dur("timeout", timeout).
			Dur("default", defaultGuestAgentNetworkTimeout).
			Msg("Guest agent timeout must be greater than zero, using default")
		timeout = defaultGuestAgentNetworkTimeout
	}

	if maxRetries < 0 {
		log.Warn().
			Int("maxRetries", maxRetries).
			Msg("Guest agent retries must be non-negative, using 0")
		maxRetries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		result, err := fn(callCtx)
		cancel()

		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry non-timeout errors or if this was the last attempt
		if attempt >= maxRetries || !strings.Contains(err.Error(), "timeout") {
			break
		}

		// Brief delay before retry to avoid hammering the API
		select {
		case <-time.After(defaultGuestAgentRetryDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
}

func (m *Monitor) fetchGuestAgentMetadata(ctx context.Context, client PVEClientInterface, instanceName, nodeName, vmName string, vmid int, vmStatus *proxmox.VMStatus) ([]string, []models.GuestNetworkInterface, string, string, string) {
	if vmStatus == nil || client == nil {
		m.clearGuestMetadataCache(instanceName, nodeName, vmid)
		return nil, nil, "", "", ""
	}

	if vmStatus.Agent.Value <= 0 {
		m.clearGuestMetadataCache(instanceName, nodeName, vmid)
		return nil, nil, "", "", ""
	}

	key := guestMetadataCacheKey(instanceName, nodeName, vmid)
	now := time.Now()

	m.guestMetadataMu.RLock()
	cached, ok := m.guestMetadataCache[key]
	m.guestMetadataMu.RUnlock()

	if ok && now.Sub(cached.fetchedAt) < guestMetadataCacheTTL {
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion
	}

	needsFetch := !ok || now.Sub(cached.fetchedAt) >= guestMetadataCacheTTL
	if !needsFetch {
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion
	}

	reserved := m.tryReserveGuestMetadataFetch(key, now)
	if !reserved && ok {
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion
	}
	if !reserved && !ok {
		reserved = true
	}

	// Start with cached values as fallback in case new calls fail
	ipAddresses := cloneStringSlice(cached.ipAddresses)
	networkIfaces := cloneGuestNetworkInterfaces(cached.networkInterfaces)
	osName := cached.osName
	osVersion := cached.osVersion
	agentVersion := cached.agentVersion

	if reserved {
		if !m.acquireGuestMetadataSlot(ctx) {
			m.deferGuestMetadataRetry(key, time.Now())
			return ipAddresses, networkIfaces, osName, osVersion, agentVersion
		}
		defer m.releaseGuestMetadataSlot()
		defer func() {
			m.scheduleNextGuestMetadataFetch(key, time.Now())
		}()
	}

	// Network interfaces with configurable timeout and retry (refs #592)
	interfaces, err := m.retryGuestAgentCall(ctx, m.guestAgentNetworkTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
		return client.GetVMNetworkInterfaces(ctx, nodeName, vmid)
	})
	if err != nil {
		log.Debug().
			Str("instance", instanceName).
			Str("vm", vmName).
			Int("vmid", vmid).
			Err(err).
			Msg("Guest agent network interfaces unavailable")
	} else if ifaces, ok := interfaces.([]proxmox.VMNetworkInterface); ok && len(ifaces) > 0 {
		ipAddresses, networkIfaces = processGuestNetworkInterfaces(ifaces)
	} else {
		ipAddresses = nil
		networkIfaces = nil
	}

	// OS info with configurable timeout and retry (refs #592)
	// Skip OS info calls if we've seen repeated failures (refs #692 - OpenBSD qemu-ga issue)
	osInfoFailureCount := cached.osInfoFailureCount
	osInfoSkip := cached.osInfoSkip

	if !osInfoSkip {
		agentInfoRaw, err := m.retryGuestAgentCall(ctx, m.guestAgentOSInfoTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
			return client.GetVMAgentInfo(ctx, nodeName, vmid)
		})
		if err != nil {
			if isGuestAgentOSInfoUnsupportedError(err) {
				osInfoSkip = true
				osInfoFailureCount = guestAgentOSInfoFailureThreshold
				log.Warn().
					Str("instance", instanceName).
					Str("vm", vmName).
					Int("vmid", vmid).
					Err(err).
					Msg("Guest agent OS info unsupported (missing os-release). Skipping future calls to avoid qemu-ga issues (refs #692)")
			} else {
				osInfoFailureCount++
				if osInfoFailureCount >= guestAgentOSInfoFailureThreshold {
					osInfoSkip = true
					log.Info().
						Str("instance", instanceName).
						Str("vm", vmName).
						Int("vmid", vmid).
						Int("failureCount", osInfoFailureCount).
						Msg("Guest agent OS info consistently fails, skipping future calls to avoid triggering buggy guest agents")
				} else {
					log.Debug().
						Str("instance", instanceName).
						Str("vm", vmName).
						Int("vmid", vmid).
						Int("failureCount", osInfoFailureCount).
						Err(err).
						Msg("Guest agent OS info unavailable")
				}
			}
		} else if agentInfo, ok := agentInfoRaw.(map[string]interface{}); ok && len(agentInfo) > 0 {
			osName, osVersion = extractGuestOSInfo(agentInfo)
			osInfoFailureCount = 0 // Reset on success
			osInfoSkip = false
		} else {
			osName = ""
			osVersion = ""
		}
	} else {
		// Skipping OS info call due to repeated failures
		log.Debug().
			Str("instance", instanceName).
			Str("vm", vmName).
			Int("vmid", vmid).
			Msg("Skipping guest agent OS info call (disabled after repeated failures)")
	}

	// Agent version with configurable timeout and retry (refs #592)
	versionRaw, err := m.retryGuestAgentCall(ctx, m.guestAgentVersionTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
		return client.GetVMAgentVersion(ctx, nodeName, vmid)
	})
	if err != nil {
		log.Debug().
			Str("instance", instanceName).
			Str("vm", vmName).
			Int("vmid", vmid).
			Err(err).
			Msg("Guest agent version unavailable")
	} else if version, ok := versionRaw.(string); ok && version != "" {
		agentVersion = version
	} else {
		agentVersion = ""
	}

	entry := guestMetadataCacheEntry{
		ipAddresses:        cloneStringSlice(ipAddresses),
		networkInterfaces:  cloneGuestNetworkInterfaces(networkIfaces),
		osName:             osName,
		osVersion:          osVersion,
		agentVersion:       agentVersion,
		fetchedAt:          time.Now(),
		osInfoFailureCount: osInfoFailureCount,
		osInfoSkip:         osInfoSkip,
	}

	m.guestMetadataMu.Lock()
	if m.guestMetadataCache == nil {
		m.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	}
	m.guestMetadataCache[key] = entry
	m.guestMetadataMu.Unlock()

	return ipAddresses, networkIfaces, osName, osVersion, agentVersion
}

func guestMetadataCacheKey(instanceName, nodeName string, vmid int) string {
	return fmt.Sprintf("%s|%s|%d", instanceName, nodeName, vmid)
}

func (m *Monitor) clearGuestMetadataCache(instanceName, nodeName string, vmid int) {
	if m == nil {
		return
	}

	key := guestMetadataCacheKey(instanceName, nodeName, vmid)
	m.guestMetadataMu.Lock()
	if m.guestMetadataCache != nil {
		delete(m.guestMetadataCache, key)
	}
	m.guestMetadataMu.Unlock()
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneGuestNetworkInterfaces(src []models.GuestNetworkInterface) []models.GuestNetworkInterface {
	if len(src) == 0 {
		return nil
	}
	dst := make([]models.GuestNetworkInterface, len(src))
	for i, iface := range src {
		dst[i] = iface
		if len(iface.Addresses) > 0 {
			dst[i].Addresses = cloneStringSlice(iface.Addresses)
		}
	}
	return dst
}

func processGuestNetworkInterfaces(raw []proxmox.VMNetworkInterface) ([]string, []models.GuestNetworkInterface) {
	ipSet := make(map[string]struct{})
	ipAddresses := make([]string, 0)
	guestIfaces := make([]models.GuestNetworkInterface, 0, len(raw))

	for _, iface := range raw {
		ifaceName := strings.TrimSpace(iface.Name)
		mac := strings.TrimSpace(iface.HardwareAddr)

		addrSet := make(map[string]struct{})
		addresses := make([]string, 0, len(iface.IPAddresses))

		for _, addr := range iface.IPAddresses {
			ip := strings.TrimSpace(addr.Address)
			if ip == "" {
				continue
			}
			lower := strings.ToLower(ip)
			if strings.HasPrefix(ip, "127.") || strings.HasPrefix(lower, "fe80") || ip == "::1" {
				continue
			}

			if _, exists := addrSet[ip]; !exists {
				addrSet[ip] = struct{}{}
				addresses = append(addresses, ip)
			}

			if _, exists := ipSet[ip]; !exists {
				ipSet[ip] = struct{}{}
				ipAddresses = append(ipAddresses, ip)
			}
		}

		if len(addresses) > 1 {
			sort.Strings(addresses)
		}

		rxBytes := parseInterfaceStat(iface.Statistics, "rx-bytes")
		txBytes := parseInterfaceStat(iface.Statistics, "tx-bytes")

		if len(addresses) == 0 && rxBytes == 0 && txBytes == 0 {
			continue
		}

		guestIfaces = append(guestIfaces, models.GuestNetworkInterface{
			Name:      ifaceName,
			MAC:       mac,
			Addresses: addresses,
			RXBytes:   rxBytes,
			TXBytes:   txBytes,
		})
	}

	if len(ipAddresses) > 1 {
		sort.Strings(ipAddresses)
	}

	if len(guestIfaces) > 1 {
		sort.SliceStable(guestIfaces, func(i, j int) bool {
			return guestIfaces[i].Name < guestIfaces[j].Name
		})
	}

	return ipAddresses, guestIfaces
}

func parseInterfaceStat(stats interface{}, key string) int64 {
	if stats == nil {
		return 0
	}
	statsMap, ok := stats.(map[string]interface{})
	if !ok {
		return 0
	}
	val, ok := statsMap[key]
	if !ok {
		return 0
	}
	return anyToInt64(val)
}

func extractGuestOSInfo(data map[string]interface{}) (string, string) {
	if data == nil {
		return "", ""
	}

	if result, ok := data["result"]; ok {
		if resultMap, ok := result.(map[string]interface{}); ok {
			data = resultMap
		}
	}

	name := stringValue(data["name"])
	prettyName := stringValue(data["pretty-name"])
	version := stringValue(data["version"])
	versionID := stringValue(data["version-id"])

	osName := name
	if osName == "" {
		osName = prettyName
	}
	if osName == "" {
		osName = stringValue(data["id"])
	}

	osVersion := version
	if osVersion == "" && versionID != "" {
		osVersion = versionID
	}
	if osVersion == "" && prettyName != "" && prettyName != osName {
		osVersion = prettyName
	}
	if osVersion == "" {
		osVersion = stringValue(data["kernel-release"])
	}
	if osVersion == osName {
		osVersion = ""
	}

	return osName, osVersion
}

func isGuestAgentOSInfoUnsupportedError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	// OpenBSD qemu-ga emits "Failed to open file '/etc/os-release'" (refs #692)
	if strings.Contains(msg, "os-release") &&
		(strings.Contains(msg, "failed to open file") || strings.Contains(msg, "no such file or directory")) {
		return true
	}

	// Some Proxmox builds bubble up "unsupported command: guest-get-osinfo"
	if strings.Contains(msg, "guest-get-osinfo") && strings.Contains(msg, "unsupported") {
		return true
	}

	return false
}

func stringValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case float32:
		return strings.TrimSpace(strconv.FormatFloat(float64(v), 'f', -1, 32))
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	default:
		return ""
	}
}

func anyToInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint32:
		return int64(v)
	case uint64:
		if v > math.MaxInt64 {
			return math.MaxInt64
		}
		return int64(v)
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		if v == "" {
			return 0
		}
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
		if parsedFloat, err := strconv.ParseFloat(v, 64); err == nil {
			return int64(parsedFloat)
		}
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return parsed
		}
		if parsedFloat, err := v.Float64(); err == nil {
			return int64(parsedFloat)
		}
	}
	return 0
}
