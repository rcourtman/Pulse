package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
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
	guestMetadataEmptyTTL    = 30 * time.Second
	defaultGuestMetadataHold = 15 * time.Second

	// Guest agent timeout defaults (configurable via environment variables)
	// Increased from 3-5s to 10-15s to handle high-load environments better (refs #592)
	defaultGuestAgentFSInfoTimeout   = 15 * time.Second // GUEST_AGENT_FSINFO_TIMEOUT
	defaultGuestAgentNetworkTimeout  = 10 * time.Second // GUEST_AGENT_NETWORK_TIMEOUT
	defaultGuestAgentOSInfoTimeout   = 10 * time.Second // GUEST_AGENT_OSINFO_TIMEOUT
	defaultGuestAgentVersionTimeout  = 10 * time.Second // GUEST_AGENT_VERSION_TIMEOUT
	defaultGuestAgentRetries         = 1                // GUEST_AGENT_RETRIES (0 = no retry, 1 = one retry)
	defaultGuestAgentRetryDelay      = 500 * time.Millisecond
	defaultGuestAgentVMMaxConcurrent = 8 // GUEST_AGENT_VM_MAX_CONCURRENT

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
	fetchedAt          time.Time // Last accepted useful metadata, not the last attempt.
	windowsGuest       bool      // An actual OS reply, not retained display identity.
	osInfoObservedAt   time.Time // Network/version refreshes cannot renew OS evidence.
	osInfoFailureCount int       // Track consecutive OS info failures
	osInfoSkip         bool      // Skip OS info calls after repeated failures (refs #692)
}

// A cancelled fetch still owns its key until it returns. Cancellation cannot
// prove that a guest command stopped, and its late reply cannot repopulate a
// deliberately invalidated lifecycle. Guarded by guestMetadataLimiterMu.
type guestMetadataFetch struct {
	invalidated bool
	cancel      context.CancelFunc
}

func guestMetadataCacheHasUsefulData(entry guestMetadataCacheEntry) bool {
	return len(entry.ipAddresses) > 0 ||
		len(entry.networkInterfaces) > 0 ||
		entry.osName != "" ||
		entry.osVersion != "" ||
		entry.agentVersion != ""
}

func guestMetadataCacheHasCompleteNetworkData(entry guestMetadataCacheEntry) bool {
	for _, iface := range entry.networkInterfaces {
		if strings.TrimSpace(iface.Name) != "" || strings.TrimSpace(iface.MAC) != "" {
			return true
		}
	}
	return false
}

func guestMetadataCacheEntryTTL(entry guestMetadataCacheEntry) time.Duration {
	// Treat identity-only and IP-only metadata as incomplete so VMs that answered
	// guest-info/version or partial network calls but not full interface inventory
	// are retried soon instead of freezing incomplete VM Summary data for minutes.
	if guestMetadataCacheHasCompleteNetworkData(entry) {
		return guestMetadataCacheTTL
	}
	return guestMetadataEmptyTTL
}

func (m *Monitor) hasRecentGuestMetadataEvidence(instanceName, nodeName string, vmid int, now time.Time) bool {
	if m == nil {
		return false
	}

	key := guestMetadataCacheKey(instanceName, nodeName, vmid)
	m.guestMetadataMu.RLock()
	entry, ok := m.guestMetadataCache[key]
	m.guestMetadataMu.RUnlock()
	if !ok || entry.fetchedAt.IsZero() || !guestMetadataCacheHasUsefulData(entry) {
		return false
	}
	return !entry.fetchedAt.After(now) && now.Sub(entry.fetchedAt) < guestMetadataCacheEntryTTL(entry)
}

func (m *Monitor) guestMetadataEvidenceTime(instanceName, nodeName string, vmid int) time.Time {
	key := guestMetadataCacheKey(instanceName, nodeName, vmid)
	m.guestMetadataMu.RLock()
	entry := m.guestMetadataCache[key]
	m.guestMetadataMu.RUnlock()
	if !guestMetadataCacheHasUsefulData(entry) {
		return time.Time{}
	}
	return entry.fetchedAt
}

func (m *Monitor) tryReserveGuestMetadataFetch(key string, now time.Time) bool {
	if m == nil {
		return false
	}
	m.guestMetadataLimiterMu.Lock()
	defer m.guestMetadataLimiterMu.Unlock()

	// A time-based hold limits retry rate, but is not completion evidence.
	// Keep ownership until the fetch returns, even if its hold has expired.
	if m.guestMetadataInFlight[key] != nil {
		return false
	}
	if next, ok := m.guestMetadataLimiter[key]; ok && now.Before(next) {
		return false
	}
	hold := m.guestMetadataHoldDuration
	if hold <= 0 {
		hold = defaultGuestMetadataHold
	}
	if m.guestMetadataLimiter == nil {
		m.guestMetadataLimiter = make(map[string]time.Time)
	}
	if m.guestMetadataInFlight == nil {
		m.guestMetadataInFlight = make(map[string]*guestMetadataFetch)
	}
	m.guestMetadataLimiter[key] = now.Add(hold)
	m.guestMetadataInFlight[key] = &guestMetadataFetch{}
	return true
}

func (m *Monitor) releaseGuestMetadataFetch(key string) {
	m.guestMetadataLimiterMu.Lock()
	delete(m.guestMetadataInFlight, key)
	m.guestMetadataLimiterMu.Unlock()
}

func (m *Monitor) invalidateGuestMetadataFetchLocked(key string) {
	if fetch := m.guestMetadataInFlight[key]; fetch != nil {
		fetch.invalidated = true
		if fetch.cancel != nil {
			fetch.cancel()
		}
	}
}

func (m *Monitor) guestMetadataFetchInvalidated(key string) bool {
	m.guestMetadataLimiterMu.Lock()
	defer m.guestMetadataLimiterMu.Unlock()
	return m.guestMetadataInFlight[key] != nil && m.guestMetadataInFlight[key].invalidated
}

// Serialize publication with explicit invalidation, using the same lock order
// as the janitor. Both complete metadata and partial OS-command safety state
// belong to the lifecycle that reserved this fetch, not a subsequent one.
func (m *Monitor) updateGuestMetadataFetch(key string, update func(*guestMetadataCacheEntry)) bool {
	m.guestMetadataLimiterMu.Lock()
	defer m.guestMetadataLimiterMu.Unlock()
	if fetch := m.guestMetadataInFlight[key]; fetch == nil || fetch.invalidated {
		return false
	}
	m.guestMetadataMu.Lock()
	defer m.guestMetadataMu.Unlock()
	if m.guestMetadataCache == nil {
		m.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	}
	entry := m.guestMetadataCache[key]
	update(&entry)
	m.guestMetadataCache[key] = entry
	return true
}

// Activity keeps an actively monitored guest's command-safety state alive even
// when it supplies no useful metadata. It cannot renew any observation or make
// an otherwise ineligible guest command queryable.
func (m *Monitor) recordGuestMetadataAccess(key string, now time.Time) {
	m.guestMetadataLimiterMu.Lock()
	if m.guestMetadataLastAccess == nil {
		m.guestMetadataLastAccess = make(map[string]time.Time)
	}
	m.guestMetadataLastAccess[key] = now
	m.guestMetadataLimiterMu.Unlock()
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
	if jitter > 0 {
		m.rngMu.Lock()
		if m.rng != nil {
			interval += time.Duration(m.rng.Int63n(int64(jitter)))
		}
		m.rngMu.Unlock()
	}
	m.guestMetadataLimiterMu.Lock()
	if fetch := m.guestMetadataInFlight[key]; fetch != nil && fetch.invalidated {
		m.guestMetadataLimiterMu.Unlock()
		return
	}
	m.guestMetadataLimiter[key] = now.Add(interval)
	m.guestMetadataLimiterMu.Unlock()
}

func (m *Monitor) scheduleGuestMetadataFetchForEntry(key string, now time.Time, entry guestMetadataCacheEntry) {
	if m == nil {
		return
	}
	if guestMetadataCacheEntryTTL(entry) == guestMetadataEmptyTTL {
		m.guestMetadataLimiterMu.Lock()
		if fetch := m.guestMetadataInFlight[key]; fetch != nil && fetch.invalidated {
			m.guestMetadataLimiterMu.Unlock()
			return
		}
		m.guestMetadataLimiter[key] = now.Add(guestMetadataEmptyTTL)
		m.guestMetadataLimiterMu.Unlock()
		return
	}
	m.scheduleNextGuestMetadataFetch(key, now)
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
	if fetch := m.guestMetadataInFlight[key]; fetch != nil && fetch.invalidated {
		m.guestMetadataLimiterMu.Unlock()
		return
	}
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

// This marker is evidence that the callback never started, not a classification
// of a cancellation returned by an already-started guest command.
var errGuestAgentCallNotStarted = errors.New("guest agent call not started")

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
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				// A prior attempt did start. Do not relabel its outcome as an
				// unstarted command merely because the retry budget ended.
				return nil, errors.Join(lastErr, err)
			}
			return nil, fmt.Errorf("%w: %w", errGuestAgentCallNotStarted, err)
		}
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		result, err := fn(callCtx)
		cancel()

		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry non-timeout errors or if this was the last attempt
		if errors.Is(err, proxmox.ErrGuestAgentDeferred) || attempt >= maxRetries || !strings.Contains(err.Error(), "timeout") {
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

// The final result reports command deferral separately from retained metadata.
// A previously successful filesystem must not conceal a later shared pause.
// Invalidation also defers this fetch: no old-lifecycle metadata is usable.
func (m *Monitor) fetchGuestAgentMetadata(ctx context.Context, client PVEClientInterface, instanceName, nodeName, vmName string, vmid int, vmStatus *proxmox.VMStatus, allowWithoutStatus bool) ([]string, []models.GuestNetworkInterface, string, string, string, bool) {
	key := guestMetadataCacheKey(instanceName, nodeName, vmid)
	now := time.Now()
	m.recordGuestMetadataAccess(key, now)

	m.guestMetadataMu.RLock()
	cached, ok := m.guestMetadataCache[key]
	m.guestMetadataMu.RUnlock()

	if vmStatus != nil && vmStatus.Lock != "" {
		// Deliberately retained identity, not a newly fetched observation.
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion, false
	}

	agentAvailable := client != nil && ((vmStatus != nil && vmStatus.Agent.IsAvailable()) || allowWithoutStatus)
	if !agentAvailable {
		// Display identity can remain useful, but a known-unavailable agent ends
		// the memory observation lifecycle even while metadata is still fresh.
		m.clearGuestMemoryCache(instanceName, nodeName, vmid)
		if ok && now.Sub(cached.fetchedAt) < guestMetadataCacheEntryTTL(cached) {
			return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion, false
		}
		m.clearGuestMetadataCache(instanceName, nodeName, vmid)
		return nil, nil, "", "", "", false
	}

	if ok && now.Sub(cached.fetchedAt) < guestMetadataCacheEntryTTL(cached) {
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion, false
	}

	needsFetch := !ok || now.Sub(cached.fetchedAt) >= guestMetadataCacheEntryTTL(cached)
	if !needsFetch {
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion, false
	}

	reserved := m.tryReserveGuestMetadataFetch(key, now)
	// An empty cache does not grant permission to bypass another fetch or the
	// backoff from an early deferral. Return only existing identity, if any.
	if !reserved {
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion, false
	}
	defer m.releaseGuestMetadataFetch(key)

	// Attach cancellation and re-read the fallback in the reservation's lifecycle.
	// Invalidation may have happened between the initial cache read and admission;
	// neither that old snapshot nor an obsolete context can survive this check.
	fetchCtx, cancelFetch := context.WithCancel(ctx)
	defer cancelFetch()
	m.guestMetadataLimiterMu.Lock()
	fetch := m.guestMetadataInFlight[key]
	if fetch.invalidated {
		m.guestMetadataLimiterMu.Unlock()
		return nil, nil, "", "", "", true
	}
	fetch.cancel = cancelFetch
	m.guestMetadataMu.RLock()
	cached = m.guestMetadataCache[key]
	m.guestMetadataMu.RUnlock()
	m.guestMetadataLimiterMu.Unlock()
	ctx = fetchCtx

	// Start with cached values as fallback in case new calls fail
	ipAddresses := cloneStringSlice(cached.ipAddresses)
	networkIfaces := cloneGuestNetworkInterfaces(cached.networkInterfaces)
	osName := cached.osName
	osVersion := cached.osVersion
	agentVersion := cached.agentVersion
	metadataObserved := false

	if reserved {
		if !m.acquireGuestMetadataSlot(ctx) {
			if m.guestMetadataFetchInvalidated(key) {
				return nil, nil, "", "", "", true
			}
			m.deferGuestMetadataRetry(key, time.Now())
			return ipAddresses, networkIfaces, osName, osVersion, agentVersion, false
		}
		defer m.releaseGuestMetadataSlot()
	}
	if m.guestMetadataFetchInvalidated(key) {
		return nil, nil, "", "", "", true
	}

	// Network interfaces with configurable timeout and retry (refs #592)
	interfaces, err := m.retryGuestAgentCall(ctx, m.guestAgentNetworkTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
		return client.GetVMNetworkInterfaces(ctx, nodeName, vmid)
	})
	if m.guestMetadataFetchInvalidated(key) {
		return nil, nil, "", "", "", true
	}
	if errors.Is(err, proxmox.ErrGuestAgentDeferred) {
		m.deferGuestMetadataRetry(key, time.Now())
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion, true
	}
	if err != nil {
		log.Debug().
			Str("instance", instanceName).
			Str("vm", vmName).
			Int("vmid", vmid).
			Err(err).
			Msg("Guest agent network interfaces unavailable")
	} else if ifaces, ok := interfaces.([]proxmox.VMNetworkInterface); ok && len(ifaces) > 0 {
		processedIPs, processedIfaces := processGuestNetworkInterfaces(ifaces)
		if len(processedIPs) > 0 || len(processedIfaces) > 0 {
			ipAddresses, networkIfaces = processedIPs, processedIfaces
			metadataObserved = true
		} else if len(cached.ipAddresses) == 0 && len(cached.networkInterfaces) == 0 {
			ipAddresses = nil
			networkIfaces = nil
		} else {
			log.Debug().
				Str("instance", instanceName).
				Str("vm", vmName).
				Int("vmid", vmid).
				Msg("Guest agent returned empty network metadata; preserving last known interfaces")
		}
	} else {
		if len(cached.ipAddresses) == 0 && len(cached.networkInterfaces) == 0 {
			ipAddresses = nil
			networkIfaces = nil
		}
	}

	// OS info with configurable timeout and retry (refs #592)
	// Skip OS info calls if we've seen repeated failures (refs #692 - OpenBSD qemu-ga issue)
	osInfoFailureCount := cached.osInfoFailureCount
	osInfoSkip := cached.osInfoSkip
	windowsGuest, osInfoObservedAt := cached.windowsGuest, cached.osInfoObservedAt

	if !osInfoSkip {
		agentInfoRaw, err := m.retryGuestAgentCall(ctx, m.guestAgentOSInfoTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
			return client.GetVMAgentInfo(ctx, nodeName, vmid)
		})
		if m.guestMetadataFetchInvalidated(key) {
			return nil, nil, "", "", "", true
		}
		if errors.Is(err, proxmox.ErrGuestAgentDeferred) {
			m.deferGuestMetadataRetry(key, time.Now())
			return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion, true
		}
		if err != nil {
			status, statusKnown := proxmox.APIErrorStatus(err)
			if reason := proxmox.GuestAgentErrorReason(err); errors.Is(err, errGuestAgentCallNotStarted) || reason == "permission-denied" || reason == "agent-not-running" || (statusKnown && status == http.StatusTooManyRequests) {
				// Access/rate refusal or stopped QGA did not exercise this
				// command. Use the wire status, not OS-like or status-like body
				// text: a temporary rate limit must not disable OS discovery.
				// An ended caller budget before the callback starts likewise
				// cannot count as a failed OS command. A cancellation after
				// admission still owns its existing outcome/uncertainty fence.
				// Preserve any real prior failures and the normal refresh backoff.
				log.Debug().
					Str("instance", instanceName).
					Str("vm", vmName).
					Int("vmid", vmid).
					Err(err).
					Msg("Guest agent OS info request not admitted; preserving command failure state")
			} else if isGuestAgentOSInfoUnsupportedError(err) {
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
			extractedOSName, extractedOSVersion := extractGuestOSInfo(agentInfo)
			windowsGuest, osInfoObservedAt = false, time.Time{}
			if extractedOSName != "" || extractedOSVersion != "" {
				osName, osVersion = extractedOSName, extractedOSVersion
				metadataObserved = true
				windowsGuest = guestOSIsWindows(extractedOSName)
				osInfoObservedAt = time.Now()
			}
			osInfoFailureCount = 0 // Reset on success
			osInfoSkip = false
		} else {
			// A complete empty reply is not a new observation of retained OS
			// strings. Keep those strings for display, not meminfo admission.
			windowsGuest, osInfoObservedAt = false, time.Time{}
			if cached.osName == "" && cached.osVersion == "" {
				osName = ""
				osVersion = ""
			}
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
	if m.guestMetadataFetchInvalidated(key) {
		return nil, nil, "", "", "", true
	}
	versionRaw, err := m.retryGuestAgentCall(ctx, m.guestAgentVersionTimeout, m.guestAgentRetries, func(ctx context.Context) (interface{}, error) {
		return client.GetVMAgentVersion(ctx, nodeName, vmid)
	})
	if m.guestMetadataFetchInvalidated(key) {
		return nil, nil, "", "", "", true
	}
	if errors.Is(err, proxmox.ErrGuestAgentDeferred) {
		// A later shared pause does not undo a completed OS-info outcome.
		// Retain its safety suppression without publishing partial metadata or
		// renewing the age of last-known guest-agent evidence.
		if !m.updateGuestMetadataFetch(key, func(entry *guestMetadataCacheEntry) {
			entry.osInfoFailureCount = osInfoFailureCount
			entry.osInfoSkip = osInfoSkip
			entry.windowsGuest, entry.osInfoObservedAt = windowsGuest, osInfoObservedAt
		}) {
			return nil, nil, "", "", "", true
		}
		m.deferGuestMetadataRetry(key, time.Now())
		return cloneStringSlice(cached.ipAddresses), cloneGuestNetworkInterfaces(cached.networkInterfaces), cached.osName, cached.osVersion, cached.agentVersion, true
	}
	if err != nil {
		log.Debug().
			Str("instance", instanceName).
			Str("vm", vmName).
			Int("vmid", vmid).
			Err(err).
			Msg("Guest agent version unavailable")
	} else if version, ok := versionRaw.(string); ok && version != "" {
		agentVersion = version
		metadataObserved = true
	} else if cached.agentVersion == "" {
		agentVersion = ""
	}

	// Errors and empty/unusable replies may retain display identity, but cannot
	// make that identity new evidence that the guest agent is still responsive.
	// Retry timing is tracked separately by the limiter, not by renewing its age.
	fetchedAt := cached.fetchedAt
	if metadataObserved {
		fetchedAt = time.Now()
	}
	entry := guestMetadataCacheEntry{
		ipAddresses:        cloneStringSlice(ipAddresses),
		networkInterfaces:  cloneGuestNetworkInterfaces(networkIfaces),
		osName:             osName,
		osVersion:          osVersion,
		agentVersion:       agentVersion,
		fetchedAt:          fetchedAt,
		windowsGuest:       windowsGuest,
		osInfoObservedAt:   osInfoObservedAt,
		osInfoFailureCount: osInfoFailureCount,
		osInfoSkip:         osInfoSkip,
	}

	if !m.updateGuestMetadataFetch(key, func(current *guestMetadataCacheEntry) { *current = entry }) {
		return nil, nil, "", "", "", true
	}
	if reserved {
		if metadataObserved {
			m.scheduleGuestMetadataFetchForEntry(key, time.Now(), entry)
		} else {
			m.deferGuestMetadataRetry(key, time.Now())
		}
	}
	if m.guestMetadataFetchInvalidated(key) {
		return nil, nil, "", "", "", true
	}

	return ipAddresses, networkIfaces, osName, osVersion, agentVersion, false
}

func guestMetadataCacheKey(instanceName, nodeName string, vmid int) string {
	return fmt.Sprintf("%s|%s|%d", instanceName, nodeName, vmid)
}

// PVE node names and VM IDs cannot contain '|', but connection labels can.
// Read the two fixed suffixes from the right: "site|other" is not owned by
// "site". Retirement must never erase another connection's command-safety
// state, cancel its fetch, or discard its refresh/backoff deadline.
func guestMetadataCacheBelongsToInstance(key, instanceName string) bool {
	end := strings.LastIndexByte(key, '|')
	if end < 0 {
		return false
	}
	end = strings.LastIndexByte(key[:end], '|')
	return end >= 0 && key[:end] == instanceName
}

func (m *Monitor) clearGuestMetadataCache(instanceName, nodeName string, vmid int) {
	if m == nil {
		return
	}
	m.clearGuestMemoryCache(instanceName, nodeName, vmid)

	key := guestMetadataCacheKey(instanceName, nodeName, vmid)
	// Keep the old owner until completion, but cancel its context and exclude all
	// subsequent publication/scheduling. Never release the shared command fence.
	m.guestMetadataLimiterMu.Lock()
	defer m.guestMetadataLimiterMu.Unlock()
	m.invalidateGuestMetadataFetchLocked(key)
	m.guestMetadataMu.Lock()
	if m.guestMetadataCache != nil {
		delete(m.guestMetadataCache, key)
	}
	m.guestMetadataMu.Unlock()
	// Deliberate invalidation when the agent is unavailable starts a new
	// metadata lifecycle. Do not carry its obsolete refresh/backoff deadline
	// into a later available poll, or discard an outstanding fetch's ownership.
	delete(m.guestMetadataLimiter, key)
	delete(m.guestMetadataLastAccess, key)
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
		hadRawAddresses := len(iface.IPAddresses) > 0

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
			sortGuestAddresses(addresses)
		}

		rxBytes := parseInterfaceStat(iface.Statistics, "rx-bytes")
		txBytes := parseInterfaceStat(iface.Statistics, "tx-bytes")

		if len(addresses) == 0 && rxBytes == 0 && txBytes == 0 {
			lowerName := strings.ToLower(ifaceName)
			if lowerName == "lo" || lowerName == "loopback" {
				continue
			}
			if ifaceName == "" && mac == "" {
				continue
			}
			if hadRawAddresses {
				// Preserve the interface identity even when every reported address
				// was filtered out. Early guest-agent payloads often surface only
				// link-local addresses first, and hiding the NIC entirely is worse.
			}
		}

		guestIfaces = append(guestIfaces, models.GuestNetworkInterface{
			Name:      ifaceName,
			MAC:       mac,
			Addresses: addresses,
			RXBytes:   rxBytes,
			TXBytes:   txBytes,
		})
	}

	if len(guestIfaces) > 1 {
		sort.SliceStable(guestIfaces, func(i, j int) bool {
			return guestIfaces[i].Name < guestIfaces[j].Name
		})
	}

	return guestIPAddressesByInterface(ipAddresses, guestIfaces), guestIfaces
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
