package availabilityprobe

import "testing"

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
