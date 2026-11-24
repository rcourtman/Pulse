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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
	case config.AvailabilityProbeDNS:
		return probeDNS(probeCtx, target, timeout)
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

	// Read one byte past the longest valid expectation. A datagram read can
	// discard the unread suffix, so a full buffer must not equal an expectation.
	var response [config.MaxAvailabilityUDPExpectedResponseBytes + 1]byte
	n, err := conn.Read(response[:])
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
	// The check runs on the selected server or agent, and a permission failure
	// does not identify a particular unit setting. Keep the guidance within the
	// agent's 240-byte error limit without recommending privilege escalation.
	if strings.Contains(details, "Operation not permitted") || strings.Contains(details, "cap_net_raw") {
		return fmt.Errorf("icmp probe blocked by local permissions on the selected observation host (server or agent). See docs/CONFIGURATION.md#icmp-probe-privileges. Preserve NoNewPrivileges and intentional capability/container restrictions.")
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
		defer conn.Close()
		return confirmTCPStaysOpen(ctx, conn)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if answer := tcpDialAnswer(err, runtime.GOOS); answer != nil {
		return answer
	}

	return probeTCPViaSystem(ctx, host, target.Port, target.EffectiveTimeoutMillis())
}

var (
	errTCPClosedAtOnce = errors.New("tcp probe: the port accepted the connection and closed it at once without sending anything, which a proxy such as Docker's does when nothing is listening behind it")
	errTCPRefused      = errors.New("tcp probe: connection refused, nothing is listening on the port")
)

// Winsock reports resets and refusals with its own codes, which the syscall
// package's Unix-named constants do not equal on Windows.
const (
	wsaConnReset   syscall.Errno = 10054
	wsaConnRefused syscall.Errno = 10061
)

// tcpDialAnswer returns the probe's verdict when a failed dial already shows
// that the far side answered, and nil when it does not. The system fallback is
// for a host the Go dialer cannot route to; it must not run for these, because
// its connect-only test would call a port open that reset the connection, and
// it reports a refusal only as "exit status 1".
//
// A reset that lands between the handshake and the dialer's own check surfaces
// as a dial error. That is the same case as a close straight after connecting.
func tcpDialAnswer(err error, goos string) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return nil
	}
	reset, refused := syscall.ECONNRESET, syscall.ECONNREFUSED
	if goos == "windows" {
		reset, refused = wsaConnReset, wsaConnRefused
	}
	switch errno {
	case reset:
		return errTCPClosedAtOnce
	case refused:
		return errTCPRefused
	}
	return nil
}

// tcpAcceptGrace is how long a TCP probe waits after connecting to see whether
// the far side closes the connection straight away. A variable so tests can
// widen or narrow it.
var tcpAcceptGrace = 250 * time.Millisecond

// confirmTCPStaysOpen separates a listening service from a forwarder with
// nothing behind it. A completed handshake does not prove a listener: Docker's
// published-port proxy accepts the connection, fails to reach the container
// port, and closes it. Measured on Docker 29.8, that close arrives within about
// 3 ms, while a real service either sends something or keeps the connection
// open. So the probe waits a short moment for the first read:
//
//   - data, or silence until the grace runs out: reachable;
//   - closed or reset with nothing sent: unreachable.
//
// A forwarder that takes longer than the grace to give up (its backend drops
// packets instead of refusing) still reads as reachable.
func confirmTCPStaysOpen(ctx context.Context, conn net.Conn) error {
	wait := tcpAcceptGrace
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
	}
	if wait <= 0 {
		return nil
	}
	if err := conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		return nil
	}
	// A read does not watch the context, so end it early on cancellation.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetReadDeadline(time.Now()) })
	defer stop()
	var first [1]byte
	n, err := conn.Read(first[:])
	if n > 0 || err == nil {
		return nil
	}
	// Cancelled is not a verdict. Running out of the check's own time while
	// the connection stayed open is: the port connected and did not close.
	if ctxErr := ctx.Err(); errors.Is(ctxErr, context.Canceled) {
		return ctxErr
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return nil
	}
	return errTCPClosedAtOnce
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
	if target.HTTP != nil {
		opts.TLSConfig = httpContractTLSConfig(target.HTTP)
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

// httpProbeErrorLimit bounds the failure text of an HTTP check to the limit the
// ICMP and TCP probes and the agent's probe module use. net/http quotes what
// the endpoint sent (a malformed status line, a redirect location), and the
// text is kept in status, alerts and resource history on every failed poll.
const httpProbeErrorLimit = 240

// httpProbeMaxResponseHeaderBytes bounds the response headers a check reads.
// The transport default of 10 MiB is also how much of a malformed response
// net/http quotes, before httpTransportFailure can cut the text.
const httpProbeMaxResponseHeaderBytes = 64 << 10

func httpTransportFailure(err error) (ProbeResult, error) {
	if message := err.Error(); len(message) > httpProbeErrorLimit {
		err = errors.New(strings.ToValidUTF8(message[:httpProbeErrorLimit], ""))
	}
	return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeUnreachable}, err
}

func httpOutboundOptions() securityutil.RestrictedOutboundHTTPOptions {
	return securityutil.RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
		TLSConfig:       tlsutil.UnverifiedPeerCertificateCaptureTLSConfig(),

		MaxResponseHeaderBytes: httpProbeMaxResponseHeaderBytes,
	}
}

// httpContractTLSConfig decides how an HTTPS check authenticates the server
// before its request is written. Peer-certificate capture accepts any server,
// which suits a check that only asks whether an endpoint answers and which
// certificate it shows. A request that carries stored credentials, header
// values or a body must not go to whichever server answers at the address: it
// requires the pinned leaf certificate, or a chain and host name this host
// trusts, unless the operator explicitly accepted an unverified certificate.
func httpContractTLSConfig(contract *config.AvailabilityHTTPConfig) *tls.Config {
	switch {
	case contract.TLSFingerprintSHA256 != "":
		return tlsutil.FingerprintVerifier(contract.TLSFingerprintSHA256)
	case contract.InsecureSkipVerify || !contract.SendsStoredValues():
		return tlsutil.UnverifiedPeerCertificateCaptureTLSConfig()
	default:
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
}

// httpCertificateRefusal reports a server whose certificate failed
// verification. Nothing was sent to it. The refused certificate is still
// published, so the operator can see what answered and pin it if it is theirs.
func httpCertificateRefusal(u *url.URL, refused *tls.CertificateVerificationError) (ProbeResult, error) {
	presented := &tls.ConnectionState{PeerCertificates: refused.UnverifiedCertificates}
	result := ProbeResult{
		Outcome:          OutcomeUnreachable,
		TransportOutcome: OutcomeUnreachable,
		Certificate:      tlsutil.ObservePeerCertificate(presented, u.Hostname(), time.Now().UTC()),
	}
	return result, fmt.Errorf("https certificate is not trusted, so the request was not sent (pin its fingerprint or allow an unverified certificate for this check): %w", refused.Err)
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
		var refused *tls.CertificateVerificationError
		if errors.As(err, &refused) {
			return httpCertificateRefusal(u, refused)
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
