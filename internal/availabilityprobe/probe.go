// Package availabilityprobe executes a single agentless availability check
// against a configured target (ICMP, TCP, UDP, HTTP/HTTPS).
//
// It holds only the probe execution core, deliberately free of scheduling,
// status bookkeeping and resource projection, so that it can be shared between
// the monitoring poller (which schedules probes and records their outcomes) and
// the host agent's external-probe module (which runs the same checks from a
// remote vantage point) without either side pulling in the other's
// dependencies.
package availabilityprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

// Outcome describes what a completed probe proved about the target.
type Outcome string

const (
	// OutcomeReachable means the probe proved the endpoint responds.
	OutcomeReachable Outcome = "reachable"
	// OutcomeUnreachable means the probe ran and the endpoint did not respond.
	OutcomeUnreachable Outcome = "unreachable"
	// OutcomeIndeterminate means the probe ran cleanly but could not prove
	// reachability either way (an open-or-filtered UDP timeout).
	OutcomeIndeterminate Outcome = "indeterminate"
)

// ApplicationOutcome is separate from transport reachability so Pulse can
// distinguish "the endpoint answered" from "the service returned what the
// operator defined as healthy".
type ApplicationOutcome string

const (
	ApplicationNotConfigured ApplicationOutcome = "not_configured"
	ApplicationPassed        ApplicationOutcome = "passed"
	ApplicationFailed        ApplicationOutcome = "failed"
)

type ApplicationResult struct {
	Outcome     ApplicationOutcome `json:"outcome"`
	StatusCode  int                `json:"statusCode,omitempty"`
	FailureCode string             `json:"failureCode,omitempty"`
}

// ProbeResult carries the reachability outcome plus HTTPS certificate posture
// when the target completed a TLS handshake.
type ProbeResult struct {
	Outcome          Outcome                         `json:"outcome"`
	TransportOutcome Outcome                         `json:"transportOutcome"`
	Application      *ApplicationResult              `json:"application,omitempty"`
	Certificate      *tlsutil.CertificateObservation `json:"certificate,omitempty"`
}

// Run executes one agentless availability check.
func Run(ctx context.Context, target config.AvailabilityTarget) error {
	_, err := Result(ctx, target)
	return err
}

// Result preserves UDP's open-or-filtered state rather than incorrectly
// claiming that a silent UDP endpoint was proven reachable.
func Result(ctx context.Context, target config.AvailabilityTarget) (Outcome, error) {
	result, err := DetailedResult(ctx, target)
	return result.Outcome, err
}

// DetailedResult preserves the legacy reachability result while publishing
// certificate posture for HTTPS checks through the same probe execution path.
func DetailedResult(ctx context.Context, target config.AvailabilityTarget) (ProbeResult, error) {
	target = config.NormalizeAvailabilityTarget(target)
	if err := target.Validate(); err != nil {
		return ProbeResult{Outcome: OutcomeUnreachable}, err
	}

	timeout := time.Duration(target.EffectiveTimeoutMillis()) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(config.DefaultAvailabilityTimeoutMillis) * time.Millisecond
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch target.Protocol {
	case config.AvailabilityProbeICMP:
		outcome, err := outcomeFromError(probeICMP(probeCtx, target))
		return ProbeResult{Outcome: outcome, TransportOutcome: outcome}, err
	case config.AvailabilityProbeTCP:
		outcome, err := outcomeFromError(probeTCP(probeCtx, target))
		return ProbeResult{Outcome: outcome, TransportOutcome: outcome}, err
	case config.AvailabilityProbeUDP:
		outcome, err := probeUDP(probeCtx, target)
		return ProbeResult{Outcome: outcome, TransportOutcome: outcome}, err
	case config.AvailabilityProbeHTTP, config.AvailabilityProbeHTTPS:
		return probeHTTP(probeCtx, target, timeout)
	default:
		return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeUnreachable}, fmt.Errorf("unsupported availability protocol %q", target.Protocol)
	}
}

func outcomeFromError(err error) (Outcome, error) {
	if err != nil {
		return OutcomeUnreachable, err
	}
	return OutcomeReachable, nil
}

func probeUDP(ctx context.Context, target config.AvailabilityTarget) (Outcome, error) {
	host := target.ProbeAddress()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return OutcomeUnreachable, fmt.Errorf("resolve UDP availability target: %w", err)
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
		return OutcomeUnreachable, fmt.Errorf("UDP availability target did not resolve to an allowed unicast address")
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(selected.String(), strconv.Itoa(target.Port)))
	if err != nil {
		return OutcomeUnreachable, fmt.Errorf("UDP probe dial failed: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return OutcomeUnreachable, fmt.Errorf("set UDP probe deadline: %w", err)
		}
	}
	payload := []byte(target.UDPRequest)
	if len(payload) == 0 {
		// A one-byte datagram gives the kernel an opportunity to surface an
		// ICMP port-unreachable result in open-or-filtered mode.
		payload = []byte{0}
	}
	if _, err := conn.Write(payload); err != nil {
		return OutcomeUnreachable, fmt.Errorf("UDP probe write failed: %w", err)
	}

	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err == nil {
		if target.UDPExpected != "" && string(response[:n]) != target.UDPExpected {
			return OutcomeUnreachable, fmt.Errorf("UDP response did not match the expected payload")
		}
		return OutcomeReachable, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if target.UDPMode == config.AvailabilityUDPOpenOrFiltered && ctxErr == context.DeadlineExceeded {
			return OutcomeIndeterminate, nil
		}
		return OutcomeUnreachable, ctxErr
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		if target.UDPMode == config.AvailabilityUDPOpenOrFiltered {
			return OutcomeIndeterminate, nil
		}
		return OutcomeUnreachable, fmt.Errorf("UDP probe timed out waiting for a response")
	}
	return OutcomeUnreachable, fmt.Errorf("UDP probe failed: %w", err)
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
	// Units written before v6.1.0-rc.1 lack AmbientCapabilities=CAP_NET_RAW and
	// in-place updates never rewrite the unit, so ping fails like this on every
	// upgraded install (#1554). Point at the unit instead of echoing ping stderr.
	if strings.Contains(details, "Operation not permitted") || strings.Contains(details, "cap_net_raw") {
		return fmt.Errorf("icmp probe blocked. The Pulse service unit does not grant CAP_NET_RAW, so ping cannot open a socket. Re-run the Pulse installer to regenerate the unit, or add a systemd override with AmbientCapabilities=CAP_NET_RAW and CapabilityBoundingSet=CAP_NET_RAW, then restart the service")
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

func probeHTTP(ctx context.Context, target config.AvailabilityTarget, timeout time.Duration) (ProbeResult, error) {
	u, err := target.HTTPURL()
	if err != nil {
		return httpTransportFailure(err)
	}
	opts := httpOutboundOptions()
	u, err = securityutil.ValidateOutboundFetchURL(ctx, u.String(), opts)
	if err != nil {
		return httpTransportFailure(fmt.Errorf("http availability target URL validation failed: %w", err))
	}
	client := securityutil.NewRestrictedOutboundHTTPClient(timeout, opts)
	if target.HTTP != nil {
		return probeHTTPContract(ctx, client, u, target.HTTP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	if err != nil {
		return httpTransportFailure(fmt.Errorf("build http availability request: %w", err))
	}
	req.Header.Set("User-Agent", "Pulse availability probe")
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		certificate := certificateObservationFromResponse(resp)
		if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
			return probeHTTPGet(ctx, client, u)
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeReachable, Certificate: certificate}, fmt.Errorf("http probe returned status %d", resp.StatusCode)
		}
		return ProbeResult{Outcome: OutcomeReachable, TransportOutcome: OutcomeReachable, Certificate: certificate}, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return httpTransportFailure(ctxErr)
	}

	return httpTransportFailure(fmt.Errorf("http probe failed: %w", err))
}

func httpTransportFailure(err error) (ProbeResult, error) {
	return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeUnreachable}, err
}

func httpOutboundOptions() securityutil.RestrictedOutboundHTTPOptions {
	return securityutil.RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
		TLSConfig:       tlsutil.UnverifiedPeerCertificateCaptureTLSConfig(),
	}
}

func probeHTTPGet(ctx context.Context, client *http.Client, u *url.URL) (ProbeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return httpTransportFailure(fmt.Errorf("build http availability fallback request: %w", err))
	}
	req.Header.Set("User-Agent", "Pulse availability probe")
	resp, err := client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return httpTransportFailure(ctxErr)
		}
		return httpTransportFailure(fmt.Errorf("http probe failed: %w", err))
	}
	defer resp.Body.Close()
	certificate := certificateObservationFromResponse(resp)
	if resp.StatusCode >= http.StatusInternalServerError {
		return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeReachable, Certificate: certificate}, fmt.Errorf("http probe returned status %d", resp.StatusCode)
	}
	return ProbeResult{Outcome: OutcomeReachable, TransportOutcome: OutcomeReachable, Certificate: certificate}, nil
}

func probeHTTPContract(ctx context.Context, client *http.Client, u *url.URL, contract *config.AvailabilityHTTPConfig) (ProbeResult, error) {
	var body io.Reader
	if contract.Body != nil {
		body = bytes.NewBufferString(*contract.Body)
	}
	req, err := http.NewRequestWithContext(ctx, string(contract.Method), u.String(), body)
	if err != nil {
		return httpTransportFailure(fmt.Errorf("build http availability request: %w", err))
	}
	req.Header.Set("User-Agent", "Pulse availability probe")
	for _, header := range contract.Headers {
		if header.Value != nil {
			req.Header.Set(header.Name, *header.Value)
		}
	}
	switch contract.Authentication.Type {
	case config.AvailabilityHTTPAuthBasic:
		req.SetBasicAuth(contract.Authentication.Username, valueOrEmpty(contract.Authentication.Password))
	case config.AvailabilityHTTPAuthBearer:
		req.Header.Set("Authorization", "Bearer "+valueOrEmpty(contract.Authentication.BearerToken))
	}

	resp, err := client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return httpTransportFailure(ctxErr)
		}
		return httpTransportFailure(fmt.Errorf("http probe failed: %w", err))
	}
	defer resp.Body.Close()
	certificate := certificateObservationFromResponse(resp)
	result := ProbeResult{
		Outcome:          OutcomeReachable,
		TransportOutcome: OutcomeReachable,
		Certificate:      certificate,
		Application: &ApplicationResult{
			Outcome:    ApplicationPassed,
			StatusCode: resp.StatusCode,
		},
	}
	if resp.StatusCode < contract.ExpectedStatusMin || resp.StatusCode > contract.ExpectedStatusMax {
		return applicationFailure(result, "status_mismatch", fmt.Sprintf("http response status %d was outside the expected %d-%d range", resp.StatusCode, contract.ExpectedStatusMin, contract.ExpectedStatusMax))
	}
	if contract.TextContains == "" && contract.JSONPath == "" {
		return result, nil
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, config.MaxAvailabilityHTTPResponseBytes+1))
	if readErr != nil {
		return applicationFailure(result, "response_read_failed", "http response body could not be read")
	}
	if len(responseBody) > config.MaxAvailabilityHTTPResponseBytes {
		return applicationFailure(result, "response_too_large", fmt.Sprintf("http response body exceeded the %d byte assertion limit", config.MaxAvailabilityHTTPResponseBytes))
	}
	if contract.TextContains != "" && !bytes.Contains(responseBody, []byte(contract.TextContains)) {
		return applicationFailure(result, "text_mismatch", "http response did not contain the expected text")
	}
	if contract.JSONPath != "" {
		var document any
		decoder := json.NewDecoder(bytes.NewReader(responseBody))
		decoder.UseNumber()
		if err := decoder.Decode(&document); err != nil {
			return applicationFailure(result, "json_invalid", "http response was not valid JSON")
		}
		value, ok := lookupJSONPath(document, contract.JSONPath)
		if !ok {
			return applicationFailure(result, "json_path_missing", "http response did not contain the expected JSON path")
		}
		if contract.JSONEquals != "" && normalizedJSONAssertionValue(value) != contract.JSONEquals {
			return applicationFailure(result, "json_value_mismatch", "http response JSON value did not match the expected value")
		}
	}
	return result, nil
}

func applicationFailure(result ProbeResult, code, message string) (ProbeResult, error) {
	result.Outcome = OutcomeUnreachable
	result.Application.Outcome = ApplicationFailed
	result.Application.FailureCode = code
	return result, fmt.Errorf("%s", message)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizedJSONAssertionValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// lookupJSONPath supports a deliberately bounded field/index vocabulary such
// as "status", "data.healthy", or "items[0].state". It is not a scripting
// language and cannot execute operator-authored expressions.
func lookupJSONPath(document any, rawPath string) (any, bool) {
	path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rawPath), "$"))
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return document, true
	}
	current := document
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			return nil, false
		}
		name := segment
		indexes := ""
		if bracket := strings.Index(segment, "["); bracket >= 0 {
			name = segment[:bracket]
			indexes = segment[bracket:]
		}
		if name != "" {
			object, ok := current.(map[string]any)
			if !ok {
				return nil, false
			}
			current, ok = object[name]
			if !ok {
				return nil, false
			}
		}
		for indexes != "" {
			if !strings.HasPrefix(indexes, "[") {
				return nil, false
			}
			end := strings.IndexByte(indexes, ']')
			if end <= 1 {
				return nil, false
			}
			index, err := strconv.Atoi(indexes[1:end])
			array, ok := current.([]any)
			if err != nil || !ok || index < 0 || index >= len(array) {
				return nil, false
			}
			current = array[index]
			indexes = indexes[end+1:]
		}
	}
	return current, true
}

func certificateObservationFromResponse(response *http.Response) *tlsutil.CertificateObservation {
	if response == nil || response.TLS == nil {
		return nil
	}
	serverName := ""
	if response.Request != nil && response.Request.URL != nil {
		serverName = response.Request.URL.Hostname()
	}
	return tlsutil.ObservePeerCertificate(response.TLS, serverName, time.Now().UTC())
}
