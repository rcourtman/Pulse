// Package telemetry provides anonymous usage telemetry for Pulse.
//
// Pulse sends a lightweight ping on startup and once every 24 hours to help the
// developer understand how many active installations exist and which features are
// in use. Telemetry is enabled by default and can be opted out at any time.
//
// # What is sent (the full list — nothing else)
//
// Identity:
//   - A random install ID (UUID, generated locally, not tied to any account)
//   - Pulse version
//   - Platform: "docker" or "binary"
//   - OS and architecture (e.g. "linux/amd64")
//
// Scale (counts only, no names):
//   - Number of PVE nodes, PBS instances, PMG instances
//   - Number of VMs, LXC containers
//   - Number of Docker hosts and Kubernetes clusters
//
// Feature usage (booleans and counts, no content):
//   - Whether AI features are enabled
//   - Number of active alerts
//   - Whether relay/remote access is enabled
//   - Whether SSO/OIDC is configured
//   - Whether multi-tenant mode is enabled
//   - License tier (free/pro/etc.)
//   - Number of API tokens configured
//
// # What is NOT sent
//
//   - No IP addresses are stored server-side
//   - No hostnames, node names, VM names, or any infrastructure identifiers
//   - No Proxmox credentials, API tokens, or passwords
//   - No alert content, AI prompts, or chat messages
//   - No personally identifiable information of any kind
//
// # How to disable
//
// Set the environment variable PULSE_TELEMETRY=false, or toggle off
// "Anonymous telemetry" in Settings → System → General.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// pingEndpoint is the URL that receives anonymous telemetry pings.
// It is a var (not const) so that tests can redirect it to a local server.
var pingEndpoint = "https://license.pulserelay.pro/v1/telemetry/ping"

const (
	// heartbeatInterval is how often a running instance phones home.
	heartbeatInterval = 24 * time.Hour

	// startupDelay is how long to wait after startup before sending the first
	// ping, giving the monitor time to connect to nodes and populate state.
	startupDelay = 2 * time.Minute

	// httpTimeout is the maximum time for a single telemetry request.
	httpTimeout = 10 * time.Second

	// installIDFile is the filename persisted in the data directory.
	installIDFile = ".install_id"
)

// Ping is the payload sent to the telemetry endpoint.
// Every field is documented here so users can audit exactly what leaves their server.
type Ping struct {
	// Identity
	InstallID string `json:"install_id"` // Random UUID, not tied to any account
	Version   string `json:"version"`    // Pulse version (e.g. "6.0.0")
	Platform  string `json:"platform"`   // "docker" or "binary"
	OS        string `json:"os"`         // runtime.GOOS (e.g. "linux")
	Arch      string `json:"arch"`       // runtime.GOARCH (e.g. "amd64")
	Event     string `json:"event"`      // "startup" or "heartbeat"

	// Scale (counts only — no names, IPs, or identifiers)
	PVENodes           int `json:"pve_nodes"`
	PBSInstances       int `json:"pbs_instances"`
	PMGInstances       int `json:"pmg_instances"`
	VMs                int `json:"vms"`
	Containers         int `json:"containers"`
	DockerHosts        int `json:"docker_hosts"`
	KubernetesClusters int `json:"kubernetes_clusters"`

	// Feature usage (booleans and counts — no content)
	AIEnabled    bool   `json:"ai_enabled"`
	ActiveAlerts int    `json:"active_alerts"`
	RelayEnabled bool   `json:"relay_enabled"`
	SSOEnabled   bool   `json:"sso_enabled"`
	MultiTenant  bool   `json:"multi_tenant"`
	LicenseTier  string `json:"license_tier"` // "free", "pro", "pro_annual", "lifetime", etc.
	APITokens    int    `json:"api_tokens"`
}

// Snapshot holds the dynamic state gathered at ping time.
// The telemetry package calls a user-provided SnapshotFunc to populate this,
// keeping the package decoupled from monitor/config internals.
type Snapshot struct {
	PVENodes           int
	PBSInstances       int
	PMGInstances       int
	VMs                int
	Containers         int
	DockerHosts        int
	KubernetesClusters int
	AIEnabled          bool
	ActiveAlerts       int
	RelayEnabled       bool
	SSOEnabled         bool
	MultiTenant        bool
	LicenseTier        string
	APITokens          int
}

// SnapshotFunc returns the current state snapshot for telemetry.
// It is called on each heartbeat to gather fresh data.
type SnapshotFunc func() Snapshot

// Config holds the static configuration for the telemetry runner.
type Config struct {
	Version     string
	DataDir     string
	IsDocker    bool
	Enabled     bool // From cfg.TelemetryEnabled (system settings or env var)
	GetSnapshot SnapshotFunc
}

// runner holds the state for the background heartbeat goroutine.
type runner struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

var (
	mu      sync.Mutex
	current *runner
)

// Start begins anonymous telemetry if enabled.
// It reads or creates a stable install ID in dataDir, waits for the monitor
// to populate state, sends a startup ping, and schedules a daily heartbeat.
// Call Stop() on shutdown.
//
// This is a no-op when telemetry is not opted in.
func Start(ctx context.Context, cfg Config) {
	if !cfg.Enabled {
		log.Info().Msg("Anonymous telemetry is disabled (enable via PULSE_TELEMETRY=true or Settings → System)")
		return
	}

	installID := getOrCreateInstallID(cfg.DataDir)
	if installID == "" {
		log.Warn().Msg("Could not determine install ID; telemetry will not run")
		return
	}

	platform := "binary"
	if cfg.IsDocker {
		platform = "docker"
	}

	base := Ping{
		InstallID: installID,
		Version:   cfg.Version,
		Platform:  platform,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	ctx, cancel := context.WithCancel(ctx)
	r := &runner{cancel: cancel}

	mu.Lock()
	if current != nil {
		current.cancel()
	}
	current = r
	mu.Unlock()

	// Log only the first 8 chars of the install ID to avoid a stable pseudonymous identifier in logs.
	idPrefix := installID
	if len(idPrefix) > 8 {
		idPrefix = idPrefix[:8] + "…"
	}
	log.Info().
		Str("install_id", idPrefix).
		Str("platform", platform).
		Msg("Anonymous telemetry enabled — sends install ID, version, platform, OS/arch, resource counts, and feature flags (nothing else)")

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		// Wait for the monitor to connect and populate state before the first ping.
		select {
		case <-ctx.Done():
			return
		case <-time.After(startupDelay):
		}

		// Send startup ping with current snapshot.
		ping := applySnapshot(base, cfg.GetSnapshot)
		ping.Event = "startup"
		send(ctx, ping)

		// Daily heartbeat.
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ping = applySnapshot(base, cfg.GetSnapshot)
				ping.Event = "heartbeat"
				send(ctx, ping)
			}
		}
	}()
}

// Stop shuts down the telemetry background goroutine.
func Stop() {
	mu.Lock()
	r := current
	current = nil
	mu.Unlock()

	if r != nil {
		r.cancel()
		r.wg.Wait()
	}
}

// IsEnabled reports whether telemetry is enabled.
// Telemetry is on by default; set PULSE_TELEMETRY=false to disable.
func IsEnabled() bool {
	v := os.Getenv("PULSE_TELEMETRY")
	if v == "" {
		return true // enabled by default
	}
	return v == "true" || v == "1"
}

// applySnapshot merges dynamic state into the base ping.
func applySnapshot(base Ping, fn SnapshotFunc) Ping {
	ping := base
	if fn == nil {
		return ping
	}
	s := fn()
	ping.PVENodes = s.PVENodes
	ping.PBSInstances = s.PBSInstances
	ping.PMGInstances = s.PMGInstances
	ping.VMs = s.VMs
	ping.Containers = s.Containers
	ping.DockerHosts = s.DockerHosts
	ping.KubernetesClusters = s.KubernetesClusters
	ping.AIEnabled = s.AIEnabled
	ping.ActiveAlerts = s.ActiveAlerts
	ping.RelayEnabled = s.RelayEnabled
	ping.SSOEnabled = s.SSOEnabled
	ping.MultiTenant = s.MultiTenant
	ping.LicenseTier = s.LicenseTier
	ping.APITokens = s.APITokens
	return ping
}

// getOrCreateInstallID reads or generates a random install ID in dataDir.
func getOrCreateInstallID(dataDir string) string {
	p := filepath.Join(dataDir, installIDFile)

	data, err := os.ReadFile(p)
	if err == nil {
		id := string(bytes.TrimSpace(data))
		if _, err := uuid.Parse(id); err == nil {
			return id
		}
		// Invalid content — regenerate.
	}

	id := uuid.New().String()
	if err := os.WriteFile(p, []byte(id+"\n"), 0600); err != nil {
		log.Warn().Err(err).Str("path", p).Msg("Failed to persist install ID")
		// Still use the generated ID for this session.
	}
	return id
}

// send posts a ping to the telemetry endpoint. Failures are silently ignored
// — telemetry must never interfere with normal operation.
func send(ctx context.Context, ping Ping) {
	body, err := json.Marshal(ping)
	if err != nil {
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, pingEndpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("Telemetry ping failed (will retry at next heartbeat)")
		return
	}
	resp.Body.Close()
}
