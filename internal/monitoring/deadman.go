package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog/log"
)

const (
	deadManHeartbeatInterval            = time.Minute
	deadManMonitoringFreshness          = 45 * time.Second
	deadManRestartReportThreshold       = 2 * time.Minute
	deadManRequestTimeout               = 10 * time.Second
	deadManDeliveryAlertThreshold       = 3
	deadManStateSchemaVersion           = 1
	deadManStateMaxBytes          int64 = 64 << 10
	deadManStateDirPerm                 = 0o700
	deadManStateFilePerm                = 0o600
)

// DeadManInterruption is the durable, privacy-preserving record of the most
// recent monitoring availability gap observed across a Pulse restart.
type DeadManInterruption struct {
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
	DurationSecs  int64     `json:"durationSeconds"`
	CleanShutdown bool      `json:"cleanShutdown"`
}

// DeadManStatus is the tenant-scoped read model for the Alerts destination UI.
// It deliberately never returns the ping URL or its fingerprint.
type DeadManStatus struct {
	Configured             bool                 `json:"configured"`
	State                  string               `json:"state"`
	HeartbeatIntervalSecs  int64                `json:"heartbeatIntervalSeconds"`
	RecommendedGraceSecs   int64                `json:"recommendedGraceSeconds"`
	LastMonitoringProgress *time.Time           `json:"lastMonitoringProgress,omitempty"`
	LastAttemptAt          *time.Time           `json:"lastAttemptAt,omitempty"`
	LastSuccessAt          *time.Time           `json:"lastSuccessAt,omitempty"`
	ConsecutiveFailures    int                  `json:"consecutiveFailures"`
	LastError              string               `json:"lastError,omitempty"`
	LastInterruption       *DeadManInterruption `json:"lastInterruption,omitempty"`
}

type deadManPersistedState struct {
	SchemaVersion       int                  `json:"schemaVersion"`
	EndpointFingerprint string               `json:"endpointFingerprint"`
	Enabled             bool                 `json:"enabled"`
	StartedAt           time.Time            `json:"startedAt"`
	LastHealthyAt       time.Time            `json:"lastHealthyAt,omitempty"`
	LastSuccessfulPing  time.Time            `json:"lastSuccessfulPing,omitempty"`
	StoppedAt           time.Time            `json:"stoppedAt,omitempty"`
	LastInterruption    *DeadManInterruption `json:"lastInterruption,omitempty"`
}

type deadManSignalError struct {
	message   string
	retryable bool
}

func (e *deadManSignalError) Error() string { return e.message }

type deadManRuntime struct {
	mu        sync.RWMutex
	persistMu sync.Mutex

	statePath          string
	startupAt          time.Time
	previous           deadManPersistedState
	persisted          deadManPersistedState
	initialized        bool
	pendingGap         *DeadManInterruption
	loadError          error
	consecutiveFail    int
	stopping           bool
	status             DeadManStatus
	wake               chan struct{}
	signalGeneration   uint64
	activeSignalCancel context.CancelFunc

	client            *http.Client
	now               func() time.Time
	interval          time.Duration
	progressFreshness time.Duration
	restartThreshold  time.Duration
	retryDelays       []time.Duration
}

func newDeadManRuntime(dataDir string) *deadManRuntime {
	now := time.Now().UTC()
	d := &deadManRuntime{
		statePath:         filepath.Join(dataDir, "alerts", "deadman-state.json"),
		startupAt:         now,
		now:               func() time.Time { return time.Now().UTC() },
		interval:          deadManHeartbeatInterval,
		progressFreshness: deadManMonitoringFreshness,
		restartThreshold:  deadManRestartReportThreshold,
		retryDelays:       []time.Duration{2 * time.Second, 5 * time.Second},
		status: DeadManStatus{
			State:                 "disabled",
			HeartbeatIntervalSecs: int64(deadManHeartbeatInterval / time.Second),
			RecommendedGraceSecs:  int64((3 * deadManHeartbeatInterval) / time.Second),
		},
		wake: make(chan struct{}, 1),
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// A credential-bearing health URL must not inherit a process-wide proxy:
	// doing so would disclose the full secret path to that proxy. Resolve the
	// endpoint at dial time as well as at configuration time so a hostname that
	// changes to a loopback or link-local address fails closed.
	transport.Proxy = nil
	transport.DialContext = deadManDialContext
	d.client = &http.Client{
		Timeout:   deadManRequestTimeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	state, err := loadDeadManState(d.statePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		d.loadError = err
		log.Error().Err(err).Msg("failed to load durable dead-man state")
	} else if err == nil {
		d.previous = state
		d.persisted = state
		d.status.LastInterruption = cloneDeadManInterruption(state.LastInterruption)
	}
	return d
}

func cloneDeadManInterruption(value *DeadManInterruption) *DeadManInterruption {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func deadManEndpointFingerprint(endpoint string) string {
	digest := sha256.Sum256([]byte(endpoint))
	return hex.EncodeToString(digest[:])
}

func (d *deadManRuntime) run(
	ctx context.Context,
	configURL func() string,
	monitorProgress func() time.Time,
	manager *alerts.Manager,
) {
	if d == nil {
		return
	}
	d.runCycle(ctx, configURL, monitorProgress, manager)

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.runCycle(ctx, configURL, monitorProgress, manager)
		case <-d.wake:
			d.runCycle(ctx, configURL, monitorProgress, manager)
		case <-ctx.Done():
			return
		}
	}
}

func (d *deadManRuntime) runCycle(
	ctx context.Context,
	configURL func() string,
	monitorProgress func() time.Time,
	manager *alerts.Manager,
) {
	now := d.now()
	d.mu.RLock()
	stopping := d.stopping
	d.mu.RUnlock()
	if stopping {
		return
	}
	endpoint := ""
	if configURL != nil {
		endpoint = strings.TrimSpace(configURL())
	}
	if endpoint == "" {
		d.disable(now, manager)
		return
	}

	if err := notifications.ValidateDeadManPingURL(endpoint); err != nil {
		d.setMisconfigured(err.Error(), manager)
		return
	}
	fingerprint := deadManEndpointFingerprint(endpoint)
	d.ensureEndpoint(now, fingerprint, manager)

	progress := time.Time{}
	if monitorProgress != nil {
		progress = monitorProgress().UTC()
	}
	fresh := !progress.IsZero() && !progress.After(now.Add(5*time.Second)) && now.Sub(progress) <= d.progressFreshness
	d.setMonitoringProgress(progress)
	if fresh {
		d.recordHealthyProgress(progress, manager)
		d.clearSystemAlert(manager, alerts.DeadManMonitoringStalledAlertType)
	} else {
		d.setState("monitor_stalled", "Pulse monitoring loop has stopped making progress")
		d.raiseMonitoringStalled(manager, progress)
	}

	message := ""
	signalURL := endpoint
	if !fresh {
		signalURL = deadManFailureURL(endpoint)
		message = "Pulse watchdog is alive, but the monitoring loop has stopped making progress."
	} else if gap := d.pendingInterruption(); gap != nil {
		message = deadManInterruptionMessage(gap)
	}

	attemptAt := d.now()
	d.setAttempt(attemptAt)
	signalCtx, generation, ok := d.beginSignal(ctx)
	if !ok {
		return
	}
	defer d.finishSignal(generation)
	err := d.sendWithRetry(signalCtx, signalURL, message)
	if signalCtx.Err() != nil {
		return
	}
	if err != nil {
		d.recordSignalFailure(err.Error(), manager)
		return
	}
	d.recordSignalSuccess(d.now(), fresh, manager)
}

func (d *deadManRuntime) ensureEndpoint(now time.Time, fingerprint string, manager *alerts.Manager) {
	d.mu.Lock()
	if d.initialized && d.persisted.EndpointFingerprint == fingerprint && d.persisted.Enabled {
		d.mu.Unlock()
		return
	}

	var gap *DeadManInterruption
	previous := d.previous
	if previous.Enabled && previous.EndpointFingerprint == fingerprint {
		from := previous.LastHealthyAt
		clean := false
		if !previous.StoppedAt.IsZero() {
			from = previous.StoppedAt
			clean = true
		}
		if !from.IsZero() && d.startupAt.After(from) && d.startupAt.Sub(from) >= d.restartThreshold {
			gap = &DeadManInterruption{
				From:          from,
				To:            d.startupAt,
				DurationSecs:  int64(d.startupAt.Sub(from).Round(time.Second) / time.Second),
				CleanShutdown: clean,
			}
		}
	}

	d.persisted = deadManPersistedState{
		SchemaVersion:       deadManStateSchemaVersion,
		EndpointFingerprint: fingerprint,
		Enabled:             true,
		StartedAt:           d.startupAt,
		LastInterruption:    cloneDeadManInterruption(previous.LastInterruption),
	}
	d.previous = d.persisted
	if gap != nil {
		d.persisted.LastInterruption = cloneDeadManInterruption(gap)
		d.pendingGap = cloneDeadManInterruption(gap)
		d.status.LastInterruption = cloneDeadManInterruption(gap)
	} else {
		d.pendingGap = nil
	}
	d.initialized = true
	d.consecutiveFail = 0
	d.status.Configured = true
	d.status.State = "starting"
	d.status.LastError = ""
	d.status.ConsecutiveFailures = 0
	loadErr := d.loadError
	d.loadError = nil
	d.mu.Unlock()

	if loadErr != nil && manager != nil {
		manager.RaiseSystemAlert(alerts.SystemAlertInput{
			Type:        alerts.DeadManStateAlertType,
			Level:       alerts.AlertLevelWarning,
			Message:     "Pulse could not read its previous dead-man restart record. Future outage reporting has restarted from a clean baseline.",
			Fingerprint: "state-load-failed",
		})
	}
	if err := d.persistCurrentState(); err != nil {
		d.handlePersistenceError(err, manager)
	} else {
		d.clearSystemAlert(manager, alerts.DeadManStateAlertType)
	}
	if gap != nil && manager != nil {
		level := alerts.AlertLevelWarning
		if !gap.CleanShutdown {
			level = alerts.AlertLevelCritical
		}
		manager.RaiseSystemAlert(alerts.SystemAlertInput{
			Type:        alerts.DeadManInterruptionAlertType,
			Level:       level,
			Message:     deadManInterruptionAlertMessage(gap),
			Fingerprint: gap.From.Format(time.RFC3339Nano) + "/" + gap.To.Format(time.RFC3339Nano),
			Metadata: map[string]interface{}{
				"interruptionFrom": gap.From,
				"interruptionTo":   gap.To,
				"durationSeconds":  gap.DurationSecs,
				"cleanShutdown":    gap.CleanShutdown,
			},
		})
	}
}

func (d *deadManRuntime) disable(now time.Time, manager *alerts.Manager) {
	d.mu.Lock()
	wasEnabled := d.initialized && d.persisted.Enabled
	if wasEnabled {
		d.persisted.Enabled = false
		d.persisted.StoppedAt = now
	}
	d.previous = d.persisted
	d.initialized = false
	d.pendingGap = nil
	d.consecutiveFail = 0
	d.status.Configured = false
	d.status.State = "disabled"
	d.status.ConsecutiveFailures = 0
	d.status.LastError = ""
	d.mu.Unlock()

	if wasEnabled {
		if err := d.persistCurrentState(); err != nil {
			d.handlePersistenceError(err, manager)
		}
	}
	for _, alertType := range []string{
		alerts.DeadManDeliveryAlertType,
		alerts.DeadManMonitoringStalledAlertType,
		alerts.DeadManInterruptionAlertType,
	} {
		d.clearSystemAlert(manager, alertType)
	}
}

func (d *deadManRuntime) stop(now time.Time, manager *alerts.Manager) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.stopping = true
	cancel := d.activeSignalCancel
	d.activeSignalCancel = nil
	if !d.initialized || !d.persisted.Enabled {
		d.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	d.persisted.StoppedAt = now.UTC()
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := d.persistCurrentState(); err != nil {
		d.handlePersistenceError(err, manager)
	}
}

func (d *deadManRuntime) statusSnapshot() DeadManStatus {
	if d == nil {
		return DeadManStatus{
			State:                 "disabled",
			HeartbeatIntervalSecs: int64(deadManHeartbeatInterval / time.Second),
			RecommendedGraceSecs:  int64((3 * deadManHeartbeatInterval) / time.Second),
		}
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	status := d.status
	status.LastInterruption = cloneDeadManInterruption(d.status.LastInterruption)
	return status
}

func (d *deadManRuntime) setMonitoringProgress(progress time.Time) {
	d.mu.Lock()
	if progress.IsZero() {
		d.status.LastMonitoringProgress = nil
	} else {
		copyTime := progress
		d.status.LastMonitoringProgress = &copyTime
	}
	d.mu.Unlock()
}

func (d *deadManRuntime) recordHealthyProgress(progress time.Time, manager *alerts.Manager) {
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		return
	}
	d.persisted.LastHealthyAt = progress
	d.persisted.StoppedAt = time.Time{}
	d.mu.Unlock()
	if err := d.persistCurrentState(); err != nil {
		d.handlePersistenceError(err, manager)
	} else {
		d.clearSystemAlert(manager, alerts.DeadManStateAlertType)
	}
}

func (d *deadManRuntime) setAttempt(at time.Time) {
	d.mu.Lock()
	d.status.LastAttemptAt = &at
	d.mu.Unlock()
}

func (d *deadManRuntime) setState(state, message string) {
	d.mu.Lock()
	d.status.State = state
	d.status.LastError = message
	d.mu.Unlock()
}

func (d *deadManRuntime) pendingInterruption() *DeadManInterruption {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return cloneDeadManInterruption(d.pendingGap)
}

func (d *deadManRuntime) setMisconfigured(message string, manager *alerts.Manager) {
	d.mu.Lock()
	d.status.Configured = true
	d.status.State = "misconfigured"
	d.status.LastError = message
	d.status.ConsecutiveFailures = 0
	d.mu.Unlock()
	if manager != nil {
		manager.RaiseSystemAlert(alerts.SystemAlertInput{
			Type:        alerts.DeadManDeliveryAlertType,
			Level:       alerts.AlertLevelWarning,
			Message:     "The external dead-man destination is invalid and Pulse cannot send watchdog signals.",
			Fingerprint: "misconfigured",
		})
	}
}

func (d *deadManRuntime) recordSignalFailure(message string, manager *alerts.Manager) {
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		return
	}
	d.consecutiveFail++
	d.status.ConsecutiveFailures = d.consecutiveFail
	if d.status.State != "monitor_stalled" {
		d.status.State = "delivery_failed"
	}
	d.status.LastError = message
	failures := d.consecutiveFail
	d.mu.Unlock()

	if failures >= deadManDeliveryAlertThreshold && manager != nil {
		manager.RaiseSystemAlert(alerts.SystemAlertInput{
			Type:        alerts.DeadManDeliveryAlertType,
			Level:       alerts.AlertLevelWarning,
			Message:     fmt.Sprintf("Pulse monitoring is running, but the external dead-man destination has failed %d consecutive heartbeat cycles.", failures),
			Fingerprint: "delivery-failing",
			Metadata: map[string]interface{}{
				"consecutiveFailures": failures,
			},
		})
	}
}

func (d *deadManRuntime) recordSignalSuccess(at time.Time, monitoringFresh bool, manager *alerts.Manager) {
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		return
	}
	d.consecutiveFail = 0
	d.status.ConsecutiveFailures = 0
	d.status.LastError = ""
	if monitoringFresh {
		d.status.LastSuccessAt = &at
		d.status.State = "healthy"
		d.pendingGap = nil
		d.persisted.LastSuccessfulPing = at
	} else {
		d.status.State = "monitor_stalled"
		d.status.LastError = "Pulse monitoring loop has stopped making progress"
	}
	d.mu.Unlock()

	if err := d.persistCurrentState(); err != nil {
		d.handlePersistenceError(err, manager)
	}
	d.clearSystemAlert(manager, alerts.DeadManDeliveryAlertType)
	if monitoringFresh {
		d.clearSystemAlert(manager, alerts.DeadManInterruptionAlertType)
	}
}

func (d *deadManRuntime) notifyConfigChanged() {
	if d == nil {
		return
	}
	d.mu.Lock()
	cancel := d.activeSignalCancel
	d.activeSignalCancel = nil
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *deadManRuntime) beginSignal(ctx context.Context) (context.Context, uint64, bool) {
	signalCtx, cancel := context.WithCancel(ctx)
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		cancel()
		return signalCtx, 0, false
	}
	d.signalGeneration++
	generation := d.signalGeneration
	d.activeSignalCancel = cancel
	d.mu.Unlock()
	return signalCtx, generation, true
}

func (d *deadManRuntime) finishSignal(generation uint64) {
	d.mu.Lock()
	var cancel context.CancelFunc
	if d.signalGeneration == generation {
		cancel = d.activeSignalCancel
		d.activeSignalCancel = nil
	}
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (d *deadManRuntime) persistCurrentState() error {
	if d == nil {
		return nil
	}
	d.persistMu.Lock()
	defer d.persistMu.Unlock()
	d.mu.RLock()
	state := d.persisted
	d.mu.RUnlock()
	return persistDeadManState(d.statePath, state)
}

func (d *deadManRuntime) raiseMonitoringStalled(manager *alerts.Manager, progress time.Time) {
	if manager == nil {
		return
	}
	message := "Pulse's external watchdog is running, but the monitoring loop has stopped making progress."
	metadata := map[string]interface{}{}
	if !progress.IsZero() {
		metadata["lastMonitoringProgress"] = progress
	}
	manager.RaiseSystemAlert(alerts.SystemAlertInput{
		Type:        alerts.DeadManMonitoringStalledAlertType,
		Level:       alerts.AlertLevelCritical,
		Message:     message,
		Fingerprint: "monitor-stalled",
		Metadata:    metadata,
	})
}

func (d *deadManRuntime) handlePersistenceError(err error, manager *alerts.Manager) {
	log.Error().Err(err).Msg("failed to persist dead-man restart state")
	if manager != nil {
		manager.RaiseSystemAlert(alerts.SystemAlertInput{
			Type:        alerts.DeadManStateAlertType,
			Level:       alerts.AlertLevelWarning,
			Message:     "Pulse cannot persist its dead-man restart record, so a later restart may not report the full monitoring gap.",
			Fingerprint: "state-write-failed",
		})
	}
}

func (d *deadManRuntime) clearSystemAlert(manager *alerts.Manager, alertType string) {
	if manager != nil {
		manager.ClearSystemAlert(alertType)
	}
}

func deadManFailureURL(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/fail"
	return parsed.String()
}

func deadManInterruptionMessage(gap *DeadManInterruption) string {
	if gap == nil {
		return ""
	}
	kind := "unexpected shutdown"
	if gap.CleanShutdown {
		kind = "clean shutdown"
	}
	return fmt.Sprintf(
		"Pulse monitoring resumed at %s after an interruption from %s to %s (%s; previous %s).",
		gap.To.Format(time.RFC3339),
		gap.From.Format(time.RFC3339),
		gap.To.Format(time.RFC3339),
		time.Duration(gap.DurationSecs)*time.Second,
		kind,
	)
}

func deadManInterruptionAlertMessage(gap *DeadManInterruption) string {
	if gap == nil {
		return "Pulse monitoring resumed after an interruption."
	}
	shutdown := "unexpectedly"
	if gap.CleanShutdown {
		shutdown = "cleanly"
	}
	return fmt.Sprintf(
		"Pulse monitoring was unavailable for %s, from %s until %s. The previous process stopped %s.",
		time.Duration(gap.DurationSecs)*time.Second,
		gap.From.Format(time.RFC3339),
		gap.To.Format(time.RFC3339),
		shutdown,
	)
}

func (d *deadManRuntime) sendWithRetry(ctx context.Context, endpoint, message string) error {
	var lastErr error
	for attempt := 0; ; attempt++ {
		err := d.sendSignal(ctx, endpoint, message)
		if err == nil {
			return nil
		}
		lastErr = err
		signalErr, retryable := err.(*deadManSignalError)
		if !retryable || !signalErr.retryable || attempt >= len(d.retryDelays) {
			return lastErr
		}
		timer := time.NewTimer(d.retryDelays[attempt])
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return &deadManSignalError{message: "heartbeat cancelled", retryable: false}
		}
	}
}

func (d *deadManRuntime) sendSignal(ctx context.Context, endpoint, message string) error {
	method := http.MethodGet
	var body io.Reader
	if message != "" {
		method = http.MethodPost
		body = strings.NewReader(message)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return &deadManSignalError{message: "could not create heartbeat request", retryable: false}
	}
	request.Header.Set("User-Agent", "Pulse dead-man monitor")
	if message != "" {
		request.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}

	response, err := d.client.Do(request)
	if err != nil {
		return sanitizeDeadManRequestError(err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
	if readErr != nil {
		return &deadManSignalError{message: "could not read heartbeat response", retryable: true}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &deadManSignalError{
			message:   fmt.Sprintf("heartbeat endpoint returned HTTP %d", response.StatusCode),
			retryable: response.StatusCode >= 500,
		}
	}
	normalizedBody := strings.ToLower(strings.TrimSpace(string(responseBody)))
	if strings.Contains(normalizedBody, "not found") || strings.Contains(normalizedBody, "rate limit") {
		return &deadManSignalError{message: "heartbeat endpoint rejected the signal", retryable: false}
	}
	return nil
}

func sanitizeDeadManRequestError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &deadManSignalError{message: "heartbeat request timed out", retryable: true}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		return &deadManSignalError{message: "heartbeat network request failed", retryable: true}
	}
	return &deadManSignalError{message: "heartbeat request failed", retryable: true}
}

func deadManDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid watchdog address")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("resolve watchdog host")
	}
	for _, candidate := range addresses {
		ip := candidate.IP
		sameHost, localErr := notifications.IsDeadManSameHostIP(ip)
		if localErr != nil {
			return nil, fmt.Errorf("verify watchdog host separation")
		}
		if sameHost {
			return nil, fmt.Errorf("watchdog host resolved to a same-host address")
		}
	}

	dialer := &net.Dialer{}
	var lastErr error
	for _, candidate := range addresses {
		connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("connect to watchdog host")
}

func loadDeadManState(path string) (deadManPersistedState, error) {
	data, err := securityutil.ReadSecureStorageFile(path, deadManStateMaxBytes)
	if err != nil {
		return deadManPersistedState{}, err
	}
	var state deadManPersistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return deadManPersistedState{}, fmt.Errorf("decode dead-man state: %w", err)
	}
	if state.SchemaVersion != deadManStateSchemaVersion {
		return deadManPersistedState{}, fmt.Errorf("unsupported dead-man state schema %d", state.SchemaVersion)
	}
	return state, nil
}

func persistDeadManState(path string, state deadManPersistedState) error {
	state.SchemaVersion = deadManStateSchemaVersion
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode dead-man state: %w", err)
	}
	dir := filepath.Dir(path)
	if err := securityutil.EnsureSecureStorageDir(dir, deadManStateDirPerm); err != nil {
		return fmt.Errorf("prepare dead-man state directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".deadman-state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create dead-man state temp file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(deadManStateFilePerm); err != nil {
		_ = temp.Close()
		return fmt.Errorf("set dead-man state permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write dead-man state: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync dead-man state: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close dead-man state: %w", err)
	}
	if err := replaceDeadManStateFile(tempPath, path); err != nil {
		return err
	}
	cleanup = false
	if err := os.Chmod(path, deadManStateFilePerm); err != nil {
		return fmt.Errorf("harden dead-man state permissions: %w", err)
	}
	if err := syncDeadManStateDirectory(dir); err != nil {
		return err
	}
	return nil
}
