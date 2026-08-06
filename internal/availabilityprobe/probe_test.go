package availabilityprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
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
