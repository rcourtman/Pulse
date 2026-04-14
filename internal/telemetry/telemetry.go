// Package telemetry provides anonymous usage telemetry for Pulse.
//
// Pulse sends a lightweight ping on startup and once every 24 hours to help the
// developer understand how many active installations exist and which features are
// in use. Telemetry is enabled by default and can be opted out at any time.
//
// # What is sent (the full list — nothing else)
//
// Identity:
//   - A rotating install ID (UUID, generated locally and rotated periodically, not tied to any account)
//   - Pulse version identity (normalized version plus raw build string when it differs)
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
//   - Whether a paid license is active
//   - Whether any API tokens are configured
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
// "Anonymous outbound telemetry" in Settings → System → General.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

// pingEndpoint is the URL that receives anonymous telemetry pings.
// It is a var (not const) so that tests can redirect it to a local server.
var pingEndpoint = "https://license.pulserelay.pro/v1/telemetry/ping"

var errInstallIDUnavailable = errors.New("telemetry install id unavailable")

const (
	// heartbeatInterval is the base interval between daily pings.
	// Each cycle adds random jitter of ±maxHeartbeatJitter to prevent
	// thundering-herd effects when many installations start simultaneously.
	heartbeatInterval = 24 * time.Hour

	// maxHeartbeatJitter is the maximum random offset added to each heartbeat.
	maxHeartbeatJitter = 30 * time.Minute

	// startupDelay is how long to wait after startup before sending the first
	// ping, giving the monitor time to connect to nodes and populate state.
	startupDelay = 2 * time.Minute

	// httpTimeout is the maximum time for a single telemetry request.
	httpTimeout = 10 * time.Second

	// installIDFile is the filename persisted in the data directory.
	installIDFile = ".install_id"

	// installIDRotationWindow limits how long the same pseudonymous identifier
	// can be reused before it is rotated locally.
	installIDRotationWindow = 30 * 24 * time.Hour
)

type installIDRecord struct {
	InstallID string    `json:"install_id"`
	IssuedAt  time.Time `json:"issued_at"`
}

// Ping is the payload sent to the telemetry endpoint.
// Every field is documented here so users can audit exactly what leaves their server.
type Ping struct {
	// Identity
	InstallID          string `json:"install_id"`                   // Rotating UUID, not tied to any account
	Version            string `json:"version"`                      // Normalized Pulse version (e.g. "6.0.0-rc.1")
	VersionRaw         string `json:"version_raw,omitempty"`        // Original version/build string when it differs
	VersionChannel     string `json:"version_channel"`              // "stable", "rc", "dev", or "prerelease"
	VersionBuild       string `json:"version_build,omitempty"`      // Build metadata when present (e.g. git describe suffix)
	VersionDevelopment bool   `json:"version_is_development"`       // True for development/manual builds
	VersionPublished   bool   `json:"version_is_published_release"` // True for published stable/RC asset versions
	Platform           string `json:"platform"`                     // "docker" or "binary"
	OS                 string `json:"os"`                           // runtime.GOOS (e.g. "linux")
	Arch               string `json:"arch"`                         // runtime.GOARCH (e.g. "amd64")
	Event              string `json:"event"`                        // "startup" or "heartbeat"

	// Scale (counts only — no names, IPs, or identifiers)
	PVENodes           int `json:"pve_nodes"`
	PBSInstances       int `json:"pbs_instances"`
	PMGInstances       int `json:"pmg_instances"`
	VMs                int `json:"vms"`
	Containers         int `json:"containers"`
	DockerHosts        int `json:"docker_hosts"`
	KubernetesClusters int `json:"kubernetes_clusters"`

	// Feature usage (booleans and counts — no content)
	AIEnabled    bool `json:"ai_enabled"`
	ActiveAlerts int  `json:"active_alerts"`
	RelayEnabled bool `json:"relay_enabled"`
	SSOEnabled   bool `json:"sso_enabled"`
	MultiTenant  bool `json:"multi_tenant"`
	PaidLicense  bool `json:"paid_license"`
	HasAPITokens bool `json:"has_api_tokens"`
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
	PaidLicense        bool
	HasAPITokens       bool
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

// Start begins anonymous outbound telemetry if enabled.
// It reads or creates a rotating install ID in dataDir, waits for the monitor
// to populate state, sends a startup ping, and schedules a daily heartbeat.
// Call Stop() on shutdown.
//
// This is a no-op when anonymous outbound telemetry is not opted in.
func Start(ctx context.Context, cfg Config) {
	if !cfg.Enabled {
		log.Info().Msg("Anonymous outbound telemetry is disabled (enable via PULSE_TELEMETRY=true or Settings → System)")
		return
	}

	installID := getOrCreateInstallID(cfg.DataDir)
	if installID == "" {
		log.Warn().Msg("Could not determine install ID; telemetry will not run")
		return
	}

	base := basePing(cfg, installID)

	ctx, cancel := context.WithCancel(ctx)
	r := &runner{cancel: cancel}

	mu.Lock()
	if current != nil {
		current.cancel()
	}
	current = r
	mu.Unlock()

	log.Info().
		Str("platform", base.Platform).
		Msg("Anonymous outbound telemetry enabled — sends a rotating install ID, version identity, platform, OS/arch, resource counts, and feature flags (nothing else)")

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		// Wait for the monitor to connect and populate state before the first ping.
		startTimer := time.NewTimer(startupDelay)
		select {
		case <-ctx.Done():
			startTimer.Stop()
			return
		case <-startTimer.C:
		}

		// Send startup ping with current snapshot.
		ping := applySnapshot(base, cfg.GetSnapshot)
		ping.Event = "startup"
		send(ctx, ping)

		// Daily heartbeat with jitter.
		for {
			timer := time.NewTimer(jitteredHeartbeat())
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
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

// BuildPreview returns the current heartbeat payload without sending it.
func BuildPreview(cfg Config) (Ping, error) {
	installID := getOrCreateInstallID(cfg.DataDir)
	if installID == "" {
		return Ping{}, errInstallIDUnavailable
	}

	ping := applySnapshot(basePing(cfg, installID), cfg.GetSnapshot)
	ping.Event = "heartbeat"
	return ping, nil
}

// ResetInstallID rotates the locally stored telemetry install ID immediately
// and returns the new pseudonymous identifier.
func ResetInstallID(dataDir string) (string, error) {
	return resetInstallIDAt(dataDir, time.Now().UTC())
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

// jitteredHeartbeat returns heartbeatInterval ± a random offset up to maxHeartbeatJitter.
func jitteredHeartbeat() time.Duration {
	jitter := time.Duration(rand.Int63n(int64(2*maxHeartbeatJitter)+1)) - maxHeartbeatJitter
	return heartbeatInterval + jitter
}

func basePing(cfg Config, installID string) Ping {
	versionIdentity := updates.DescribeUsageDataVersion(cfg.Version)
	return Ping{
		InstallID:          installID,
		Version:            versionIdentity.Version,
		VersionRaw:         versionIdentity.RawVersion,
		VersionChannel:     versionIdentity.Channel,
		VersionBuild:       versionIdentity.Build,
		VersionDevelopment: versionIdentity.IsDevelopment,
		VersionPublished:   versionIdentity.IsPublishedRelease,
		Platform:           platformName(cfg.IsDocker),
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
	}
}

func platformName(isDocker bool) string {
	if isDocker {
		return "docker"
	}
	return "binary"
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
	ping.PaidLicense = s.PaidLicense
	ping.HasAPITokens = s.HasAPITokens
	return ping
}

// getOrCreateInstallID reads or generates a rotating install ID in dataDir.
func getOrCreateInstallID(dataDir string) string {
	return getOrCreateInstallIDAt(dataDir, time.Now().UTC())
}

func getOrCreateInstallIDAt(dataDir string, now time.Time) string {
	p := filepath.Join(dataDir, installIDFile)
	now = now.UTC()

	data, err := os.ReadFile(p)
	if err == nil {
		record, ok := parseInstallIDRecord(data)
		if ok && shouldKeepInstallIDRecord(record, now) {
			return record.InstallID
		}
	}

	record := installIDRecord{
		InstallID: uuid.New().String(),
		IssuedAt:  now,
	}
	if err := writeInstallIDRecordAt(dataDir, record); err != nil {
		log.Warn().Err(err).Str("path", p).Msg("Failed to persist install ID")
		// Still use the generated ID for this session.
	}
	return record.InstallID
}

func resetInstallIDAt(dataDir string, now time.Time) (string, error) {
	record := installIDRecord{
		InstallID: uuid.New().String(),
		IssuedAt:  now.UTC(),
	}
	if err := writeInstallIDRecordAt(dataDir, record); err != nil {
		return "", err
	}
	return record.InstallID, nil
}

func writeInstallIDRecordAt(dataDir string, record installIDRecord) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, installIDFile), append(encoded, '\n'), 0600)
}

func parseInstallIDRecord(data []byte) (installIDRecord, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return installIDRecord{}, false
	}

	var record installIDRecord
	if err := json.Unmarshal(trimmed, &record); err == nil {
		record.InstallID = string(bytes.TrimSpace([]byte(record.InstallID)))
		if _, err := uuid.Parse(record.InstallID); err == nil && !record.IssuedAt.IsZero() {
			return record, true
		}
		return installIDRecord{}, false
	}

	legacyID := string(trimmed)
	if _, err := uuid.Parse(legacyID); err == nil {
		// Legacy plaintext IDs are accepted as migration input only. Rotate to a
		// new record immediately instead of preserving an unbounded stable ID.
		return installIDRecord{}, false
	}
	return installIDRecord{}, false
}

func shouldKeepInstallIDRecord(record installIDRecord, now time.Time) bool {
	if _, err := uuid.Parse(record.InstallID); err != nil {
		return false
	}
	issuedAt := record.IssuedAt.UTC()
	if issuedAt.IsZero() || issuedAt.After(now) {
		return false
	}
	return now.Sub(issuedAt) < installIDRotationWindow
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
