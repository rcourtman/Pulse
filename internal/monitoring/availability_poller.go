package monitoring

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

// AvailabilityProbeStatus captures the last observed state of an agentless
// endpoint probe.
type AvailabilityProbeStatus struct {
	TargetID            string    `json:"targetId"`
	Name                string    `json:"name"`
	TargetKind          string    `json:"targetKind,omitempty"`
	Address             string    `json:"address"`
	Protocol            string    `json:"protocol"`
	Outcome             string    `json:"outcome,omitempty"`
	Enabled             bool      `json:"enabled"`
	Available           bool      `json:"available"`
	LastChecked         time.Time `json:"lastChecked,omitempty"`
	LastSuccess         time.Time `json:"lastSuccess,omitempty"`
	LatencyMillis       int64     `json:"latencyMillis,omitempty"`
	ConsecutiveFailures int       `json:"consecutiveFailures,omitempty"`
	LastError           string    `json:"lastError,omitempty"`
	FailureThreshold    int       `json:"failureThreshold,omitempty"`
}

type AvailabilityProbeOutcome string

const (
	AvailabilityProbeReachable     AvailabilityProbeOutcome = "reachable"
	AvailabilityProbeUnreachable   AvailabilityProbeOutcome = "unreachable"
	AvailabilityProbeIndeterminate AvailabilityProbeOutcome = "indeterminate"
)

type availabilityPollProvider struct{}

func newAvailabilityPollProvider() PollProvider {
	return availabilityPollProvider{}
}

func (availabilityPollProvider) Type() InstanceType {
	return InstanceTypeAvailability
}

func (availabilityPollProvider) ListInstances(m *Monitor) []string {
	targets := m.availabilityTargets()
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		names = append(names, target.ID)
	}
	sort.Strings(names)
	return names
}

func (availabilityPollProvider) BaseInterval(m *Monitor) time.Duration {
	targets := m.availabilityTargets()
	minInterval := time.Duration(config.DefaultAvailabilityPollIntervalSecs) * time.Second
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		interval := time.Duration(target.EffectivePollIntervalSecs()) * time.Second
		if interval > 0 && interval < minInterval {
			minInterval = interval
		}
	}
	return clampInterval(minInterval, 10*time.Second, time.Hour)
}

func (availabilityPollProvider) FixedInstanceInterval(m *Monitor, instanceName string) time.Duration {
	target, ok := m.availabilityTargetByID(instanceName)
	if !ok || !target.Enabled {
		return 0
	}
	return clampInterval(time.Duration(target.EffectivePollIntervalSecs())*time.Second, 10*time.Second, time.Hour)
}

func (availabilityPollProvider) BuildPollTask(m *Monitor, instanceName string) (PollTask, error) {
	target, ok := m.availabilityTargetByID(instanceName)
	if !ok || !target.Enabled {
		return PollTask{}, fmt.Errorf("availability target %q is not enabled", instanceName)
	}
	return PollTask{
		InstanceName: target.ID,
		InstanceType: string(InstanceTypeAvailability),
		Run: func(ctx context.Context) {
			m.pollAvailabilityTarget(ctx, target)
		},
	}, nil
}

func (availabilityPollProvider) DescribeInstances(m *Monitor) []PollProviderInstanceInfo {
	targets := m.availabilityTargets()
	infos := make([]PollProviderInstanceInfo, 0, len(targets))
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		infos = append(infos, PollProviderInstanceInfo{
			Name:        target.ID,
			DisplayName: target.DisplayName(),
			Connection:  availabilityConnectionKey(target.ID),
			Metadata: map[string]string{
				"address":  target.Address,
				"protocol": string(target.Protocol),
			},
		})
	}
	return infos
}

func (availabilityPollProvider) ConnectionStatuses(m *Monitor) map[string]bool {
	statuses := m.AvailabilityStatusSnapshot()
	out := make(map[string]bool, len(statuses))
	for targetID, status := range statuses {
		out[availabilityConnectionKey(targetID)] = status.Enabled && status.Available
	}
	return out
}

func (availabilityPollProvider) ConnectionHealthKey(_ *Monitor, instanceName string) string {
	return availabilityConnectionKey(instanceName)
}

func (availabilityPollProvider) SupplementalSource() unifiedresources.DataSource {
	return unifiedresources.SourceAvailability
}

func (availabilityPollProvider) SupplementalRecords(m *Monitor, orgID string) []unifiedresources.IngestRecord {
	targets := m.availabilityTargets()
	statuses := m.AvailabilityStatusSnapshot()
	records := make([]unifiedresources.IngestRecord, 0, len(targets))
	now := time.Now().UTC()
	for _, target := range targets {
		status := statuses[target.ID]
		if status.TargetID == "" {
			status = availabilityStatusFromTarget(target)
		}
		resource, identity := availabilityResourceFromTarget(target, status, orgID, now)
		records = append(records, unifiedresources.IngestRecord{
			SourceID: target.ID,
			Resource: resource,
			Identity: identity,
		})
	}
	return records
}

func (m *Monitor) availabilityTargets() []config.AvailabilityTarget {
	if m == nil || m.configPersist == nil {
		return nil
	}
	targets, err := m.configPersist.LoadAvailabilityTargets()
	if err != nil {
		return nil
	}
	out := make([]config.AvailabilityTarget, 0, len(targets))
	for _, target := range targets {
		target = config.NormalizeAvailabilityTarget(target)
		if strings.TrimSpace(target.ID) == "" {
			continue
		}
		out = append(out, target)
	}
	sort.Slice(out, func(i, j int) bool {
		left := strings.ToLower(out[i].DisplayName())
		right := strings.ToLower(out[j].DisplayName())
		if left == right {
			return out[i].ID < out[j].ID
		}
		return left < right
	})
	return out
}

func (m *Monitor) availabilityTargetByID(id string) (config.AvailabilityTarget, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return config.AvailabilityTarget{}, false
	}
	for _, target := range m.availabilityTargets() {
		if target.ID == id {
			return target, true
		}
	}
	return config.AvailabilityTarget{}, false
}

func (m *Monitor) AvailabilityStatusSnapshot() map[string]AvailabilityProbeStatus {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]AvailabilityProbeStatus, len(m.availabilityStatuses))
	for id, status := range m.availabilityStatuses {
		out[id] = status
	}
	return out
}

func (m *Monitor) RefreshAvailabilityTargets() {
	if m == nil {
		return
	}
	targets := m.availabilityTargets()
	activeIDs := make(map[string]struct{}, len(targets))
	now := time.Now()
	for _, target := range targets {
		activeIDs[target.ID] = struct{}{}
		if m.taskQueue == nil {
			continue
		}
		task := ScheduledTask{
			InstanceName: target.ID,
			InstanceType: InstanceTypeAvailability,
			NextRun:      now,
			Interval:     clampInterval(time.Duration(target.EffectivePollIntervalSecs())*time.Second, 10*time.Second, time.Hour),
		}
		if target.Enabled {
			m.taskQueue.Upsert(task)
		} else {
			m.taskQueue.Remove(InstanceTypeAvailability, target.ID)
			m.removeProviderConnectionHealth(InstanceTypeAvailability, target.ID)
		}
	}

	removedIDs := make([]string, 0)
	m.mu.Lock()
	for id := range m.availabilityStatuses {
		if _, ok := activeIDs[id]; !ok {
			delete(m.availabilityStatuses, id)
			removedIDs = append(removedIDs, id)
		}
	}
	m.mu.Unlock()
	for _, id := range removedIDs {
		m.removeProviderConnectionHealth(InstanceTypeAvailability, id)
	}

	m.refreshInstanceInfoCacheFromProviders()
	m.updateResourceStore(m.GetState())
}

func (m *Monitor) pollAvailabilityTarget(ctx context.Context, target config.AvailabilityTarget) {
	target = config.NormalizeAvailabilityTarget(target)
	start := time.Now()
	outcome, err := ProbeAvailabilityTargetResult(ctx, target)
	latency := time.Since(start)
	checkedAt := time.Now().UTC()
	m.setAvailabilityStatus(target, checkedAt, latency, outcome, err)

	if err == nil {
		if m.stalenessTracker != nil {
			m.stalenessTracker.UpdateSuccess(InstanceTypeAvailability, target.ID, nil)
		}
		m.setProviderConnectionHealth(InstanceTypeAvailability, target.ID, true)
	} else {
		if m.stalenessTracker != nil {
			m.stalenessTracker.UpdateSuccess(InstanceTypeAvailability, target.ID, []byte(err.Error()))
		}
		m.setProviderConnectionHealth(InstanceTypeAvailability, target.ID, false)
	}
	m.recordTaskResult(InstanceTypeAvailability, target.ID, nil)
	m.updateResourceStore(m.GetState())
}

func (m *Monitor) setAvailabilityStatus(target config.AvailabilityTarget, checkedAt time.Time, latency time.Duration, outcome AvailabilityProbeOutcome, probeErr error) {
	if m == nil {
		return
	}
	status := availabilityStatusFromTarget(target)
	status.Outcome = string(outcome)
	status.LastChecked = checkedAt
	latencyMs := latency.Milliseconds()
	if probeErr == nil && latencyMs == 0 {
		latencyMs = 1
	}
	status.LatencyMillis = latencyMs
	if probeErr == nil {
		// An open-or-filtered UDP timeout is healthy probe execution but does
		// not prove endpoint reachability. Keep it non-failing without claiming
		// the endpoint is available.
		status.Available = outcome == AvailabilityProbeReachable
		if outcome == AvailabilityProbeReachable {
			status.LastSuccess = checkedAt
		}
	} else {
		status.Available = false
		status.LastError = probeErr.Error()
	}

	m.mu.Lock()
	if m.availabilityStatuses == nil {
		m.availabilityStatuses = make(map[string]AvailabilityProbeStatus)
	}
	if previous, ok := m.availabilityStatuses[target.ID]; ok {
		status.LastSuccess = previous.LastSuccess
		if probeErr == nil {
			status.ConsecutiveFailures = 0
			status.LastError = ""
			if outcome == AvailabilityProbeReachable {
				status.LastSuccess = checkedAt
			}
		} else {
			status.ConsecutiveFailures = previous.ConsecutiveFailures + 1
		}
	} else if probeErr != nil {
		status.ConsecutiveFailures = 1
	}
	m.availabilityStatuses[target.ID] = status
	m.mu.Unlock()
}

// ProbeAvailabilityTarget executes one agentless availability check.
func ProbeAvailabilityTarget(ctx context.Context, target config.AvailabilityTarget) error {
	_, err := ProbeAvailabilityTargetResult(ctx, target)
	return err
}

// ProbeAvailabilityTargetResult preserves UDP's open-or-filtered state rather
// than incorrectly claiming that a silent UDP endpoint was proven reachable.
func ProbeAvailabilityTargetResult(ctx context.Context, target config.AvailabilityTarget) (AvailabilityProbeOutcome, error) {
	target = config.NormalizeAvailabilityTarget(target)
	if err := target.Validate(); err != nil {
		return AvailabilityProbeUnreachable, err
	}

	timeout := time.Duration(target.EffectiveTimeoutMillis()) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(config.DefaultAvailabilityTimeoutMillis) * time.Millisecond
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch target.Protocol {
	case config.AvailabilityProbeICMP:
		return outcomeFromProbeError(probeICMP(probeCtx, target))
	case config.AvailabilityProbeTCP:
		return outcomeFromProbeError(probeTCP(probeCtx, target))
	case config.AvailabilityProbeUDP:
		return probeUDP(probeCtx, target)
	case config.AvailabilityProbeHTTP, config.AvailabilityProbeHTTPS:
		return outcomeFromProbeError(probeHTTP(probeCtx, target, timeout))
	default:
		return AvailabilityProbeUnreachable, fmt.Errorf("unsupported availability protocol %q", target.Protocol)
	}
}

func outcomeFromProbeError(err error) (AvailabilityProbeOutcome, error) {
	if err != nil {
		return AvailabilityProbeUnreachable, err
	}
	return AvailabilityProbeReachable, nil
}

func probeUDP(ctx context.Context, target config.AvailabilityTarget) (AvailabilityProbeOutcome, error) {
	host := target.ProbeAddress()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return AvailabilityProbeUnreachable, fmt.Errorf("resolve UDP availability target: %w", err)
	}
	var selected net.IP
	for _, address := range addresses {
		if address.IP == nil || address.IP.IsUnspecified() || address.IP.IsMulticast() || address.IP.Equal(net.IPv4bcast) {
			continue
		}
		selected = address.IP
		break
	}
	if selected == nil {
		return AvailabilityProbeUnreachable, fmt.Errorf("UDP availability target did not resolve to an allowed unicast address")
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(selected.String(), strconv.Itoa(target.Port)))
	if err != nil {
		return AvailabilityProbeUnreachable, fmt.Errorf("UDP probe dial failed: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return AvailabilityProbeUnreachable, fmt.Errorf("set UDP probe deadline: %w", err)
		}
	}
	payload := []byte(target.UDPRequest)
	if len(payload) == 0 {
		// A one-byte datagram gives the kernel an opportunity to surface an
		// ICMP port-unreachable result in open-or-filtered mode.
		payload = []byte{0}
	}
	if _, err := conn.Write(payload); err != nil {
		return AvailabilityProbeUnreachable, fmt.Errorf("UDP probe write failed: %w", err)
	}

	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err == nil {
		if target.UDPExpected != "" && string(response[:n]) != target.UDPExpected {
			return AvailabilityProbeUnreachable, fmt.Errorf("UDP response did not match the expected payload")
		}
		return AvailabilityProbeReachable, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if target.UDPMode == config.AvailabilityUDPOpenOrFiltered && ctxErr == context.DeadlineExceeded {
			return AvailabilityProbeIndeterminate, nil
		}
		return AvailabilityProbeUnreachable, ctxErr
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		if target.UDPMode == config.AvailabilityUDPOpenOrFiltered {
			return AvailabilityProbeIndeterminate, nil
		}
		return AvailabilityProbeUnreachable, fmt.Errorf("UDP probe timed out waiting for a response")
	}
	return AvailabilityProbeUnreachable, fmt.Errorf("UDP probe failed: %w", err)
}

func probeICMP(ctx context.Context, target config.AvailabilityTarget) error {
	host := target.ProbeAddress()
	if host == "" {
		return fmt.Errorf("icmp availability target host is required")
	}
	args := pingArgs(host, target.EffectiveTimeoutMillis())
	cmd := exec.CommandContext(ctx, "ping", args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	details := strings.TrimSpace(string(output))
	if details == "" {
		return fmt.Errorf("icmp probe failed: %w", err)
	}
	if len(details) > 240 {
		details = details[:240]
	}
	return fmt.Errorf("icmp probe failed: %s", details)
}

func pingArgs(host string, timeoutMillis int) []string {
	if timeoutMillis <= 0 {
		timeoutMillis = config.DefaultAvailabilityTimeoutMillis
	}
	switch runtime.GOOS {
	case "windows":
		return []string{"-n", "1", "-w", strconv.Itoa(timeoutMillis), host}
	case "darwin", "freebsd", "openbsd", "netbsd":
		return []string{"-n", "-c", "1", "-W", strconv.Itoa(timeoutMillis), host}
	default:
		timeoutSeconds := (timeoutMillis + 999) / 1000
		if timeoutSeconds <= 0 {
			timeoutSeconds = 1
		}
		return []string{"-n", "-c", "1", "-W", strconv.Itoa(timeoutSeconds), host}
	}
}

func probeTCP(ctx context.Context, target config.AvailabilityTarget) error {
	host := target.ProbeAddress()
	if host == "" {
		return fmt.Errorf("tcp availability target host is required")
	}
	addr := net.JoinHostPort(host, strconv.Itoa(target.Port))

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err == nil {
		conn.Close()
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	return probeTCPViaSystem(ctx, host, target.Port, target.EffectiveTimeoutMillis())
}

func probeTCPViaSystem(ctx context.Context, host string, port, timeoutMillis int) error {
	timeoutSecs := (timeoutMillis + 999) / 1000
	if timeoutSecs < 1 {
		timeoutSecs = 1
	}
	portStr := strconv.Itoa(port)

	var args []string
	if runtime.GOOS == "darwin" {
		args = []string{"-z", "-G", strconv.Itoa(timeoutSecs), host, portStr}
	} else {
		args = []string{"-z", "-w", strconv.Itoa(timeoutSecs), host, portStr}
	}

	cmd := exec.CommandContext(ctx, "nc", args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	details := strings.TrimSpace(string(output))
	if details == "" {
		return fmt.Errorf("tcp probe failed: %w", err)
	}
	if len(details) > 240 {
		details = details[:240]
	}
	return fmt.Errorf("tcp probe failed: %s", details)
}

func probeHTTP(ctx context.Context, target config.AvailabilityTarget, timeout time.Duration) error {
	u, err := target.HTTPURL()
	if err != nil {
		return err
	}
	opts := availabilityHTTPOutboundOptions()
	u, err = securityutil.ValidateOutboundFetchURL(ctx, u.String(), opts)
	if err != nil {
		return fmt.Errorf("http availability target URL validation failed: %w", err)
	}
	client := securityutil.NewRestrictedOutboundHTTPClient(timeout, opts)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build http availability request: %w", err)
	}
	req.Header.Set("User-Agent", "Pulse availability probe")
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			return probeHTTPGet(ctx, client, u)
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("http probe returned %s", resp.Status)
		}
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	return fmt.Errorf("http probe failed: %w", err)
}

func availabilityHTTPOutboundOptions() securityutil.RestrictedOutboundHTTPOptions {
	return securityutil.RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
		TLSConfig:       tlsutil.UnverifiedPeerCertificateCaptureTLSConfig(),
	}
}

func probeHTTPGet(ctx context.Context, client *http.Client, u *url.URL) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build http availability fallback request: %w", err)
	}
	req.Header.Set("User-Agent", "Pulse availability probe")
	resp, err := client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("http probe failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("http probe returned %s", resp.Status)
	}
	return nil
}

func availabilityStatusFromTarget(target config.AvailabilityTarget) AvailabilityProbeStatus {
	return AvailabilityProbeStatus{
		TargetID:         target.ID,
		Name:             target.DisplayName(),
		TargetKind:       string(target.TargetKind),
		Address:          target.Address,
		Protocol:         string(target.Protocol),
		Enabled:          target.Enabled,
		FailureThreshold: target.EffectiveFailureThreshold(),
	}
}

func availabilityResourceFromTarget(target config.AvailabilityTarget, status AvailabilityProbeStatus, _ string, now time.Time) (unifiedresources.Resource, unifiedresources.ResourceIdentity) {
	lastSeen := status.LastChecked
	if lastSeen.IsZero() {
		lastSeen = now
	}
	resourceStatus := availabilityResourceStatus(target, status)
	data := &unifiedresources.AvailabilityData{
		TargetID:            target.ID,
		LinkedResourceID:    strings.TrimSpace(target.LinkedResourceID),
		Name:                target.DisplayName(),
		TargetKind:          string(target.TargetKind),
		Address:             target.Address,
		Protocol:            string(target.Protocol),
		ProbeOutcome:        status.Outcome,
		UDPMode:             string(target.UDPMode),
		Port:                target.Port,
		Path:                target.Path,
		Enabled:             target.Enabled,
		Available:           status.Available,
		LastChecked:         timePointerIfSet(status.LastChecked),
		LastSuccess:         timePointerIfSet(status.LastSuccess),
		LatencyMillis:       status.LatencyMillis,
		ConsecutiveFailures: status.ConsecutiveFailures,
		LastError:           status.LastError,
		FailureThreshold:    target.EffectiveFailureThreshold(),
		PollIntervalSeconds: target.EffectivePollIntervalSecs(),
		TimeoutMillis:       target.EffectiveTimeoutMillis(),
	}
	data.Evidence = availabilityEvidenceEnvelope(target, status, lastSeen, now)
	resource := unifiedresources.Resource{
		Type:         unifiedresources.ResourceTypeNetworkEndpoint,
		Technology:   string(target.Protocol),
		Name:         target.DisplayName(),
		Status:       resourceStatus,
		LastSeen:     lastSeen,
		UpdatedAt:    now,
		Sources:      []unifiedresources.DataSource{unifiedresources.SourceAvailability},
		Tags:         availabilityResourceTags(target),
		Availability: data,
	}
	if incident := availabilityIncident(target, status, lastSeen); incident != nil {
		resource.Incidents = []unifiedresources.ResourceIncident{*incident}
	}

	identity := unifiedresources.ResourceIdentity{}
	if ip := net.ParseIP(target.ProbeAddress()); ip != nil {
		identity.IPAddresses = []string{ip.String()}
	} else if host := target.ProbeAddress(); host != "" {
		identity.Hostnames = []string{host}
	}
	return resource, identity
}

func availabilityEvidenceEnvelope(
	target config.AvailabilityTarget,
	status AvailabilityProbeStatus,
	observedAt time.Time,
	ingestedAt time.Time,
) *operationaltrust.EvidenceEnvelope {
	if observedAt.IsZero() {
		return nil
	}
	if ingestedAt.IsZero() {
		ingestedAt = observedAt
	}

	source := operationaltrust.EvidenceSource{
		Provider:  string(unifiedresources.SourceAvailability),
		Collector: "availability-poller",
	}
	subject := operationaltrust.EvidenceSubject{
		ProviderRef:   target.ID,
		ProviderScope: "availability-target",
	}
	evidenceID, err := operationaltrust.NewEvidenceID(
		source,
		subject,
		observedAt,
		target.ID,
	)
	if err != nil {
		return nil
	}

	validUntil := observedAt.Add(
		time.Duration(target.EffectivePollIntervalSecs()*2) * time.Second,
	)
	completeness := operationaltrust.EvidenceComplete
	confidence := operationaltrust.EvidenceConfirmed
	var reason *operationaltrust.EvidenceReason
	if status.LastChecked.IsZero() {
		completeness = operationaltrust.EvidencePartial
		confidence = operationaltrust.EvidenceUnknown
		reason = &operationaltrust.EvidenceReason{
			Code:    "availability_not_observed",
			Message: "The availability target has not completed its first probe.",
		}
	}

	envelope := operationaltrust.EvidenceEnvelope{
		ID:           evidenceID,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   ingestedAt,
		ValidUntil:   &validUntil,
		Completeness: completeness,
		Confidence:   confidence,
		Reason:       reason,
		Permissions:  operationaltrust.EvidencePermissionsSufficient,
		PayloadRef: &operationaltrust.EvidencePayloadRef{
			Kind: "availability-target",
			ID:   target.ID,
		},
	}
	if err := envelope.Validate(); err != nil {
		return nil
	}
	return &envelope
}

func availabilityResourceTags(target config.AvailabilityTarget) []string {
	tags := []string{"agentless"}
	if target.TargetKind != "" {
		tags = append(tags, string(target.TargetKind))
	}
	return tags
}

func availabilityResourceStatus(target config.AvailabilityTarget, status AvailabilityProbeStatus) unifiedresources.ResourceStatus {
	if !target.Enabled {
		return unifiedresources.StatusUnknown
	}
	if status.LastChecked.IsZero() {
		return unifiedresources.StatusUnknown
	}
	if status.Available {
		return unifiedresources.StatusOnline
	}
	if status.ConsecutiveFailures >= target.EffectiveFailureThreshold() {
		return unifiedresources.StatusOffline
	}
	return unifiedresources.StatusWarning
}

func availabilityIncident(target config.AvailabilityTarget, status AvailabilityProbeStatus, startedAt time.Time) *unifiedresources.ResourceIncident {
	if !target.Enabled || status.Available || status.LastChecked.IsZero() {
		return nil
	}
	if status.ConsecutiveFailures < target.EffectiveFailureThreshold() {
		return nil
	}
	summary := fmt.Sprintf("%s is unreachable by %s probe", target.DisplayName(), strings.ToUpper(string(target.Protocol)))
	if status.LastError != "" {
		summary = summary + ": " + status.LastError
	}
	return &unifiedresources.ResourceIncident{
		Provider:  string(unifiedresources.SourceAvailability),
		NativeID:  target.ID,
		Code:      "availability_unreachable",
		Severity:  storagehealth.RiskCritical,
		Source:    string(unifiedresources.SourceAvailability),
		Summary:   summary,
		StartedAt: startedAt,
	}
}

func availabilityConnectionKey(targetID string) string {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return ""
	}
	return "availability-" + targetID
}
