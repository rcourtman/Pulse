package proxmox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// ErrGuestAgentDeferred means no further guest-agent commands should be queued
// by the caller. In particular, a timed-out HTTP request may still be running
// in pvedaemon; another command is not a safe way to recover it.
var ErrGuestAgentDeferred = errors.New("guest agent deferred")

type guestAgentDeferredError struct {
	reason string
	cause  error
}

func (e *guestAgentDeferredError) Error() string        { return "guest agent deferred: " + e.reason }
func (e *guestAgentDeferredError) Is(target error) bool { return target == ErrGuestAgentDeferred }
func (e *guestAgentDeferredError) Unwrap() error        { return e.cause }

// GuestAgentDeferredReason returns a fixed reason, never provider response text.
func GuestAgentDeferredReason(err error) string {
	var deferred *guestAgentDeferredError
	if errors.As(err, &deferred) {
		return deferred.reason
	}
	return ""
}

const guestAgentUncertainCooldown = time.Minute
const maxGuestAgentGuardEntries = 4096

type guestAgentGuardKey struct {
	endpoint string
	vmid     int // VMIDs are cluster-wide, including while a VM migrates nodes.
}
type guestAgentGuardEntry struct {
	busy  bool
	until time.Time
}

// Share admission with independently constructed diagnostic clients, not just
// the polling client. Only coordination is shared: no credentials or guest data.
var guestAgentGuards = struct {
	sync.Mutex
	aliases     map[string][]string
	entries     map[guestAgentGuardKey]guestAgentGuardEntry
	nextCleanup time.Time
}{aliases: make(map[string][]string), entries: make(map[guestAgentGuardKey]guestAgentGuardEntry)}

func registerGuestAgentEndpoints(host string, endpoints []string) {
	guestAgentGuards.Lock()
	defer guestAgentGuards.Unlock()
	group := make(map[string]bool)
	for _, endpoint := range append([]string{host}, endpoints...) {
		u, err := securityutil.NormalizeHTTPBaseURL(endpoint, "https")
		if err != nil {
			continue
		}
		key := securityutil.AppendURLPath(u, "api2", "json").String()
		group[key] = true
		for _, alias := range guestAgentGuards.aliases[key] {
			group[alias] = true
		}
	}
	keys := make([]string, 0, len(group))
	for key := range group {
		keys = append(keys, key)
	}
	for key := range group {
		guestAgentGuards.aliases[key] = keys
	}
}

func acquireGuestAgent(endpoint string, vmid int) (func(bool), error) {
	guestAgentGuards.Lock()
	defer guestAgentGuards.Unlock()
	now := time.Now()
	if !now.Before(guestAgentGuards.nextCleanup) || len(guestAgentGuards.entries) >= maxGuestAgentGuardEntries {
		for key, entry := range guestAgentGuards.entries {
			if !entry.busy && !now.Before(entry.until) {
				delete(guestAgentGuards.entries, key)
			}
		}
		guestAgentGuards.nextCleanup = now.Add(time.Minute)
	}
	endpoints := guestAgentGuards.aliases[endpoint]
	if len(endpoints) == 0 {
		endpoints = []string{endpoint}
	}
	keys := make([]guestAgentGuardKey, 0, len(endpoints))
	for _, endpoint := range endpoints {
		key := guestAgentGuardKey{endpoint: endpoint, vmid: vmid}
		entry := guestAgentGuards.entries[key]
		if entry.busy {
			return nil, &guestAgentDeferredError{reason: "agent-busy"}
		}
		if now.Before(entry.until) {
			return nil, &guestAgentDeferredError{reason: "agent-cooldown"}
		}
		keys = append(keys, key)
	}
	if len(guestAgentGuards.entries)+len(keys) > maxGuestAgentGuardEntries {
		return nil, &guestAgentDeferredError{reason: "agent-capacity"}
	}
	for _, key := range keys {
		guestAgentGuards.entries[key] = guestAgentGuardEntry{busy: true}
	}
	return func(uncertain bool) {
		guestAgentGuards.Lock()
		defer guestAgentGuards.Unlock()
		for _, key := range keys {
			if uncertain {
				guestAgentGuards.entries[key] = guestAgentGuardEntry{until: time.Now().Add(guestAgentUncertainCooldown)}
			} else {
				delete(guestAgentGuards.entries, key)
			}
		}
	}, nil
}

func guestAgentPath(path string) (string, int, bool) {
	parts := strings.Split(strings.SplitN(path, "?", 2)[0], "/")
	if len(parts) != 7 || parts[1] != "nodes" || parts[3] != "qemu" || parts[5] != "agent" {
		return "", 0, false
	}
	vmid, err := strconv.Atoi(parts[4])
	return parts[2], vmid, err == nil && vmid > 0
}

func (c *Client) verifyGuestAgentUnlocked(ctx context.Context, node string, vmid int) error {
	// The config endpoint contains the authoritative PVE operation lock even
	// when a PVE version omits it from status/current or cluster/resources.
	// Unlike an ordinary config read, redirected or ambiguous evidence cannot
	// establish the lock on the endpoint about to receive this guest command.
	lockClient := *c.httpClient
	lockClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := c.requestWithRetryUsingClient(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid), nil, false, &lockClient)
	if err != nil {
		return &guestAgentDeferredError{reason: "lock-unverified", cause: err}
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return &guestAgentDeferredError{reason: "lock-unverified"}
	}
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return &guestAgentDeferredError{reason: "lock-unverified", cause: errors.Join(readErr, closeErr)}
	}
	lock, valid := guestAgentConfigLock(body)
	if !valid {
		return &guestAgentDeferredError{reason: "lock-unverified"}
	}
	if strings.TrimSpace(lock) != "" {
		return &guestAgentDeferredError{reason: "vm-locked"}
	}
	return nil
}

func (c *Client) getGuestAgent(ctx context.Context, path, node string, vmid int) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	release, err := acquireGuestAgent(c.baseURL, vmid)
	if err != nil {
		return nil, err
	}
	uncertain := false
	defer func() { release(uncertain) }()
	commandClient, err := guestAgentCommandClient(c.httpClient)
	if err != nil {
		return nil, err
	}
	defer commandClient.CloseIdleConnections()
	if err := c.verifyGuestAgentUnlocked(ctx, node, vmid); err != nil {
		return nil, err
	}
	// A password-session 401 must not replay this command after re-authentication:
	// the operation lock could change between attempts. Ordinary API reads still
	// own their usual session recovery; this guest admission is single-attempt.
	resp, err := c.requestWithRetryUsingClient(ctx, http.MethodGet, path, nil, true, commandClient)
	if err != nil {
		var incomplete *responseBodyReadError
		if errors.As(err, &incomplete) {
			uncertain = true
			return nil, &guestAgentDeferredError{reason: "agent-response-incomplete", cause: err}
		}
		// A complete gateway/server error can follow a consumed command. Use
		// the actual wire status, never an "API error" quoted in body text.
		// Specific terminal rejections remain errors, not successful telemetry.
		lower := strings.ToLower(err.Error())
		uncertain = ctx.Err() != nil || strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") || strings.Contains(lower, "wrong command id")
		if uncertain {
			return nil, &guestAgentDeferredError{reason: "agent-timeout", cause: err}
		}
		var response *apiResponseError
		if !errors.As(err, &response) {
			uncertain = true
			return nil, &guestAgentDeferredError{reason: "agent-timeout", cause: err}
		}
		if response.statusCode == http.StatusRequestTimeout ||
			(response.statusCode >= 500 && !response.guestCommandRejected) {
			uncertain = true
			return nil, &guestAgentDeferredError{reason: "agent-completion-unverified", cause: err}
		}
		return nil, err
	}
	// Hold admission until the entire bounded response has arrived. Returning
	// at response headers would allow another command while the first stalls.
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		uncertain = true
		return nil, &guestAgentDeferredError{reason: "agent-response-incomplete", cause: errors.Join(readErr, closeErr)}
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		// The original endpoint may have consumed the command. Neither its
		// completion nor another endpoint's operation lock is established.
		uncertain = true
		return nil, &guestAgentDeferredError{reason: "agent-redirect"}
	}
	// A backup may have started while the command was in flight. Do not publish
	// its payload as fresh telemetry if lock clearance cannot still be verified.
	if err := c.verifyGuestAgentUnlocked(ctx, node, vmid); err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

// Guest calls do not fail over or replay: another HTTP worker still addresses
// the same serial QGA channel. Transport recovery belongs to ordinary API reads.
func (cc *ClusterClient) executeGuestAgent(ctx context.Context, fn func(*Client) error) error {
	client, err := cc.getHealthyClient(ctx)
	if err != nil {
		return fmt.Errorf("select guest agent endpoint: %w", err)
	}
	return fn(client)
}
