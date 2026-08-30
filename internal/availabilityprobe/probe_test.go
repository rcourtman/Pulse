package availabilityprobe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

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
