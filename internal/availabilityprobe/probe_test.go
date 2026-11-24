package availabilityprobe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

func TestDetailedResultUDPMatchesWholeDatagram(t *testing.T) {
	// Use multibyte text at the byte limit, not the character limit.
	maxReply := strings.Repeat("é", config.MaxAvailabilityUDPExpectedResponseBytes/2)
	for _, tc := range []struct {
		name, expected, reply string
		mode                  config.AvailabilityUDPMode
		want                  Outcome
	}{
		{"ordinary match", "PONG", "PONG", config.AvailabilityUDPResponseRequired, OutcomeReachable},
		{"ordinary mismatch", "PONG", "PONG!", config.AvailabilityUDPResponseRequired, OutcomeUnreachable},
		{"maximum match", maxReply, maxReply, config.AvailabilityUDPResponseRequired, OutcomeReachable},
		{"longer reply", maxReply, maxReply + "!", config.AvailabilityUDPResponseRequired, OutcomeUnreachable},
		{"longer reply in open or filtered mode", maxReply, maxReply + "!", config.AvailabilityUDPOpenOrFiltered, OutcomeUnreachable},
		{"larger reply with truncation", maxReply, maxReply + strings.Repeat("!", 32), config.AvailabilityUDPResponseRequired, OutcomeUnreachable},
		{"no expectation", "", maxReply, config.AvailabilityUDPResponseRequired, OutcomeReachable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			responder, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			defer responder.Close()
			if err := responder.SetDeadline(time.Now().Add(4 * time.Second)); err != nil {
				t.Fatal(err)
			}
			served := make(chan error, 1)
			go func() {
				var request [32]byte
				n, peer, err := responder.ReadFromUDP(request[:])
				if err == nil && string(request[:n]) != "PING" {
					err = errors.New("unexpected fixture request")
				}
				if err == nil {
					_, err = responder.WriteToUDP([]byte(tc.reply), peer)
				}
				served <- err
			}()
			result, probeErr := DetailedResult(context.Background(), config.AvailabilityTarget{
				Name: "Fixture UDP service", Address: "127.0.0.1", Port: responder.LocalAddr().(*net.UDPAddr).Port,
				Protocol: config.AvailabilityProbeUDP, TimeoutMillis: 2000,
				UDPMode: tc.mode, UDPRequest: "PING", UDPExpected: tc.expected,
			})
			if err := <-served; err != nil {
				t.Fatalf("loopback fixture did not send its reply: %v", err)
			}
			if result.Outcome != tc.want || result.TransportOutcome != tc.want || (probeErr == nil) != (tc.want == OutcomeReachable) {
				t.Fatalf("DetailedResult() = %+v, %v; want %s", result, probeErr, tc.want)
			}
			if len(tc.reply) <= config.MaxAvailabilityUDPExpectedResponseBytes+1 && tc.want == OutcomeUnreachable &&
				probeErr.Error() != "UDP response did not match the expected payload" {
				t.Fatalf("expected mismatch, not another probe failure: %v", probeErr)
			}
		})
	}
}

// Moved verbatim from internal/monitoring/availability_poller_test.go when the
// probe execution core was extracted into this package.
func TestAvailabilityHTTPOutboundOptionsUsesSharedPeerCertificateCapture(t *testing.T) {
	tlsConfig := httpOutboundOptions().TLSConfig
	if tlsConfig == nil || !tlsConfig.InsecureSkipVerify {
		t.Fatal("availability TLS config must enter explicit peer-certificate capture mode")
	}
	if tlsConfig.VerifyPeerCertificate == nil {
		t.Fatal("availability TLS config must reject missing or malformed peer certificates")
	}
}

func TestDetailedResultEvaluatesBoundedHTTPApplicationContract(t *testing.T) {
	requestBody := `{"operation":"health"}`
	password := "secret-password"
	headerValue := "tenant-a"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		username, gotPassword, ok := r.BasicAuth()
		if !ok || username != "pulse" || gotPassword != password {
			t.Errorf("basic auth = %q/%q/%v", username, gotPassword, ok)
		}
		if got := r.Header.Get("X-Tenant"); got != headerValue {
			t.Errorf("X-Tenant = %q, want %q", got, headerValue)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != requestBody {
			t.Errorf("body = %q, want %q", body, requestBody)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"status":"healthy"}}`))
	}))
	defer server.Close()

	result, err := DetailedResult(context.Background(), config.AvailabilityTarget{
		Address: server.URL, Protocol: config.AvailabilityProbeHTTP, Enabled: true, TimeoutMillis: 1000,
		HTTP: &config.AvailabilityHTTPConfig{
			Method:         config.AvailabilityHTTPMethodPOST,
			Headers:        []config.AvailabilityHTTPHeader{{ID: "tenant", Name: "X-Tenant", Value: &headerValue}},
			Authentication: config.AvailabilityHTTPAuthentication{Type: config.AvailabilityHTTPAuthBasic, Username: "pulse", Password: &password},
			Body:           &requestBody, ExpectedStatusMin: 200, ExpectedStatusMax: 299,
			TextContains: "healthy", JSONPath: "data.status", JSONEquals: "healthy",
		},
	})
	if err != nil {
		t.Fatalf("DetailedResult() error = %v", err)
	}
	if result.Outcome != OutcomeReachable || result.TransportOutcome != OutcomeReachable {
		t.Fatalf("result outcomes = %+v, want reachable transport and overall", result)
	}
	if result.Application == nil || result.Application.Outcome != ApplicationPassed || result.Application.StatusCode != http.StatusCreated {
		t.Fatalf("application result = %+v, want passed HTTP 201", result.Application)
	}
}

func TestDetailedResultKeepsReachabilityWhenHTTPApplicationContractFails(t *testing.T) {
	const responseSecret = "top-secret-response-content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(responseSecret))
	}))
	defer server.Close()

	result, err := DetailedResult(context.Background(), config.AvailabilityTarget{
		Address: server.URL, Protocol: config.AvailabilityProbeHTTP, Enabled: true, TimeoutMillis: 1000,
		HTTP: &config.AvailabilityHTTPConfig{
			Method:            config.AvailabilityHTTPMethodGET,
			Authentication:    config.AvailabilityHTTPAuthentication{Type: config.AvailabilityHTTPAuthNone},
			ExpectedStatusMin: 200, ExpectedStatusMax: 299,
		},
	})
	if err == nil {
		t.Fatal("DetailedResult() error = nil, want status assertion failure")
	}
	if strings.Contains(err.Error(), responseSecret) {
		t.Fatalf("error leaked response content: %q", err)
	}
	if result.Outcome != OutcomeUnreachable || result.TransportOutcome != OutcomeReachable {
		t.Fatalf("result outcomes = %+v, want unreachable overall but reachable transport", result)
	}
	if result.Application == nil || result.Application.Outcome != ApplicationFailed || result.Application.FailureCode != "status_mismatch" {
		t.Fatalf("application result = %+v, want typed status mismatch", result.Application)
	}
}

func TestDetailedResultCapturesHTTPSCertificatePosture(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result, err := DetailedResult(context.Background(), config.AvailabilityTarget{
		Address:       server.URL,
		Protocol:      config.AvailabilityProbeHTTPS,
		Enabled:       true,
		TimeoutMillis: 1000,
	})
	if err != nil {
		t.Fatalf("DetailedResult() error = %v", err)
	}
	if result.Outcome != OutcomeReachable {
		t.Fatalf("outcome = %q, want reachable", result.Outcome)
	}
	if result.Certificate == nil {
		t.Fatal("certificate observation = nil")
	}
	if result.Certificate.TrustStatus != tlsutil.CertificateTrustSelfSigned {
		t.Fatalf("trust status = %q, want self-signed", result.Certificate.TrustStatus)
	}
	if !result.Certificate.HostnameValid || result.Certificate.FingerprintSHA256 == "" {
		t.Fatalf("certificate projection = %+v", result.Certificate)
	}
}

func TestDetailedResultSendsStoredValuesOnlyToAuthenticatedHTTPSServer(t *testing.T) {
	password := "secret-password"
	headerValue := "header-secret"
	requestBody := "body-secret"
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	leaf := sha256.Sum256(server.Certificate().Raw)
	pinned := hex.EncodeToString(leaf[:])
	other := sha256.Sum256([]byte("another certificate"))

	basic := config.AvailabilityHTTPAuthentication{Type: config.AvailabilityHTTPAuthBasic, Username: "pulse", Password: &password}
	bearer := config.AvailabilityHTTPAuthentication{Type: config.AvailabilityHTTPAuthBearer, BearerToken: &password}
	tests := []struct {
		name     string
		contract config.AvailabilityHTTPConfig
		wantSent bool
	}{
		{name: "basic credentials", contract: config.AvailabilityHTTPConfig{Authentication: basic}},
		{name: "bearer token", contract: config.AvailabilityHTTPConfig{Authentication: bearer}},
		{name: "header value", contract: config.AvailabilityHTTPConfig{Headers: []config.AvailabilityHTTPHeader{{ID: "key", Name: "X-Api-Key", Value: &headerValue}}}},
		{name: "request body", contract: config.AvailabilityHTTPConfig{Method: config.AvailabilityHTTPMethodPOST, Body: &requestBody}},
		{name: "another pinned certificate", contract: config.AvailabilityHTTPConfig{TLSFingerprintSHA256: hex.EncodeToString(other[:])}},
		{name: "pinned certificate", contract: config.AvailabilityHTTPConfig{Authentication: basic, TLSFingerprintSHA256: pinned}, wantSent: true},
		{name: "explicitly unverified", contract: config.AvailabilityHTTPConfig{Authentication: basic, InsecureSkipVerify: true}, wantSent: true},
		{name: "no stored values", contract: config.AvailabilityHTTPConfig{}, wantSent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests.Store(0)
			contract := tt.contract
			result, err := DetailedResult(context.Background(), config.AvailabilityTarget{
				Address: server.URL, Protocol: config.AvailabilityProbeHTTPS, Enabled: true, TimeoutMillis: 1000, HTTP: &contract,
			})
			if !tt.wantSent {
				if err == nil || requests.Load() != 0 {
					t.Fatalf("error = %v with %d requests served, want the request withheld", err, requests.Load())
				}
				if result.Outcome != OutcomeUnreachable || result.TransportOutcome != OutcomeUnreachable {
					t.Fatalf("result outcomes = %+v, want unreachable", result)
				}
				return
			}
			if err != nil || requests.Load() != 1 {
				t.Fatalf("error = %v with %d requests served, want one accepted request", err, requests.Load())
			}
			if result.Certificate == nil || result.Certificate.FingerprintSHA256 != pinned {
				t.Fatalf("certificate observation = %+v, want the server's leaf", result.Certificate)
			}
		})
	}
}

func TestDetailedResultPublishesTheCertificateItRefused(t *testing.T) {
	password := "secret-password"
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("request reached a server whose certificate is not trusted")
	}))
	defer server.Close()
	leaf := sha256.Sum256(server.Certificate().Raw)

	result, err := DetailedResult(context.Background(), config.AvailabilityTarget{
		Address: server.URL, Protocol: config.AvailabilityProbeHTTPS, Enabled: true, TimeoutMillis: 1000,
		HTTP: &config.AvailabilityHTTPConfig{
			Authentication: config.AvailabilityHTTPAuthentication{Type: config.AvailabilityHTTPAuthBasic, Username: "pulse", Password: &password},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "certificate is not trusted") {
		t.Fatalf("DetailedResult() error = %v, want a certificate refusal", err)
	}
	if result.Certificate == nil || result.Certificate.FingerprintSHA256 != hex.EncodeToString(leaf[:]) {
		t.Fatalf("certificate observation = %+v, want the refused leaf so it can be pinned", result.Certificate)
	}
	if result.Certificate.ChainValid || result.Certificate.TrustStatus == tlsutil.CertificateTrustTrusted {
		t.Fatalf("certificate observation = %+v, want an untrusted chain", result.Certificate)
	}
}

func TestDetailedResultBoundsHTTPTransportErrorQuotingTheEndpoint(t *testing.T) {
	if limit := httpOutboundOptions().MaxResponseHeaderBytes; limit != httpProbeMaxResponseHeaderBytes {
		t.Fatalf("availability response header limit = %d, want %d", limit, httpProbeMaxResponseHeaderBytes)
	}

	// Not HTTP: net/http quotes the whole first line in its error, and every
	// NUL byte becomes four characters.
	for _, tc := range []struct {
		name      string
		firstLine int
		want      string
	}{
		{name: "within the header limit", firstLine: httpProbeMaxResponseHeaderBytes / 2, want: "malformed HTTP response"},
		{name: "beyond the header limit", firstLine: 4 * httpProbeMaxResponseHeaderBytes, want: "server response headers exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer := append(make([]byte, tc.firstLine), "\r\n\r\n"...)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			defer listener.Close()
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					go func() {
						defer conn.Close()
						if _, err := conn.Read(make([]byte, 4096)); err != nil {
							return
						}
						_, _ = conn.Write(answer)
					}()
				}
			}()

			result, err := DetailedResult(context.Background(), config.AvailabilityTarget{
				Address: "http://" + listener.Addr().String(), Protocol: config.AvailabilityProbeHTTP, Enabled: true, TimeoutMillis: 5000,
			})
			if err == nil {
				t.Fatal("DetailedResult() error = nil, want transport failure")
			}
			if len(err.Error()) > httpProbeErrorLimit {
				t.Fatalf("error is %d bytes, want at most %d", len(err.Error()), httpProbeErrorLimit)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want the transport's reason %q kept", err, tc.want)
			}
			if result.Outcome != OutcomeUnreachable || result.TransportOutcome != OutcomeUnreachable {
				t.Fatalf("result outcomes = %+v, want unreachable", result)
			}
		})
	}
}

// startTCPListener serves each accepted loopback connection with handle and
// returns the port.
func startTCPListener(t *testing.T, handle func(net.Conn)) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handle(conn)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

// holdOpen keeps an accepted connection open until the test ends.
func holdOpen(t *testing.T) func(net.Conn) {
	t.Helper()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	return func(conn net.Conn) {
		<-done
		_ = conn.Close()
	}
}

func setTCPAcceptGrace(t *testing.T, grace time.Duration) {
	t.Helper()
	previous := tcpAcceptGrace
	tcpAcceptGrace = grace
	t.Cleanup(func() { tcpAcceptGrace = previous })
}

func tcpTarget(port, timeoutMillis int) config.AvailabilityTarget {
	return config.AvailabilityTarget{
		Address: "127.0.0.1", Port: port, Protocol: config.AvailabilityProbeTCP,
		Enabled: true, TimeoutMillis: timeoutMillis,
	}
}

// A forwarder with nothing behind it (Docker's published-port proxy) completes
// the handshake and then closes. That must not read as a listening service.
func TestTCPProbeTreatsAcceptThenCloseAsUnreachable(t *testing.T) {
	cases := map[string]func(net.Conn){
		"closed": func(conn net.Conn) { _ = conn.Close() },
		"reset": func(conn net.Conn) {
			if tcp, ok := conn.(*net.TCPConn); ok {
				_ = tcp.SetLinger(0)
			}
			_ = conn.Close()
		},
	}
	for name, handle := range cases {
		t.Run(name, func(t *testing.T) {
			// A wide grace keeps a slow test host from missing the close; the
			// probe returns as soon as the close arrives.
			setTCPAcceptGrace(t, 5*time.Second)
			port := startTCPListener(t, handle)

			result, err := DetailedResult(context.Background(), tcpTarget(port, 10000))
			if err == nil {
				t.Fatal("DetailedResult() error = nil, want an accepted-then-closed failure")
			}
			if !strings.Contains(err.Error(), "closed it at once") {
				t.Fatalf("error = %q, want it to say the port closed the connection at once", err)
			}
			if result.Outcome != OutcomeUnreachable || result.TransportOutcome != OutcomeUnreachable {
				t.Fatalf("result outcomes = %+v, want unreachable", result)
			}
		})
	}
}

// Many services wait for the client to speak first. Silence is not a failure.
func TestTCPProbeTreatsSilentListenerAsReachable(t *testing.T) {
	setTCPAcceptGrace(t, 50*time.Millisecond)
	port := startTCPListener(t, holdOpen(t))

	result, err := DetailedResult(context.Background(), tcpTarget(port, 10000))
	if err != nil {
		t.Fatalf("DetailedResult() error = %v, want a silent listener to be reachable", err)
	}
	if result.Outcome != OutcomeReachable {
		t.Fatalf("outcome = %q, want reachable", result.Outcome)
	}
}

func TestTCPProbeTreatsBannerAsReachableWithoutWaitingOutTheGrace(t *testing.T) {
	setTCPAcceptGrace(t, 30*time.Second)
	hold := holdOpen(t)
	port := startTCPListener(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("SSH-2.0-test\r\n"))
		hold(conn)
	})

	started := time.Now()
	result, err := DetailedResult(context.Background(), tcpTarget(port, 60000))
	if err != nil {
		t.Fatalf("DetailedResult() error = %v, want a banner to be reachable", err)
	}
	if result.Outcome != OutcomeReachable {
		t.Fatalf("outcome = %q, want reachable", result.Outcome)
	}
	if elapsed := time.Since(started); elapsed > 20*time.Second {
		t.Fatalf("probe took %s, want it to return when the banner arrives", elapsed)
	}
}

// The wait never outlasts the check's own timeout, and running out of time
// while a connected service stays silent is still a pass.
func TestTCPProbeGraceIsBoundedByTheCheckTimeout(t *testing.T) {
	setTCPAcceptGrace(t, 30*time.Second)
	port := startTCPListener(t, holdOpen(t))

	started := time.Now()
	result, err := DetailedResult(context.Background(), tcpTarget(port, 1000))
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("DetailedResult() error = %v, want a silent listener to be reachable", err)
	}
	if result.Outcome != OutcomeReachable {
		t.Fatalf("outcome = %q, want reachable", result.Outcome)
	}
	if elapsed < 500*time.Millisecond {
		t.Fatalf("probe took %s, want it to wait for a close until the check timeout", elapsed)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("probe took %s, want it to stop at the 1 s check timeout, not the 30 s grace", elapsed)
	}
}

func TestTCPProbeStopsWaitingWhenCancelled(t *testing.T) {
	setTCPAcceptGrace(t, 30*time.Second)
	port := startTCPListener(t, holdOpen(t))

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	started := time.Now()
	_, err := DetailedResult(ctx, tcpTarget(port, 60000))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DetailedResult() error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("probe took %s, want cancellation to end the wait", elapsed)
	}
}

// A dial that fails because the far side answered needs no second opinion
// from the system fallback, on either family of error codes.
func TestTCPDialAnswerNamesResetsAndRefusals(t *testing.T) {
	dialError := func(errno syscall.Errno) error {
		return &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", errno)}
	}
	for _, tc := range []struct {
		name string
		goos string
		err  error
		want error
	}{
		{"reset", "linux", dialError(syscall.ECONNRESET), errTCPClosedAtOnce},
		{"refused", "linux", dialError(syscall.ECONNREFUSED), errTCPRefused},
		{"windows reset", "windows", dialError(syscall.Errno(10054)), errTCPClosedAtOnce},
		{"windows refused", "windows", dialError(syscall.Errno(10061)), errTCPRefused},
		{"no route leaves the fallback to decide", "darwin", dialError(syscall.EHOSTUNREACH), nil},
		{"not a socket error", "linux", errors.New("lookup failed"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tcpDialAnswer(tc.err, tc.goos); got != tc.want {
				t.Fatalf("tcpDialAnswer() = %v, want %v", got, tc.want)
			}
		})
	}
}
