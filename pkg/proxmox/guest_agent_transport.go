package proxmox

import "net/http"

// Guest-agent GETs enqueue serial QGA work and are not replayable resource
// reads. A pooled net/http Transport can transparently resubmit a GET after
// losing a reused connection's reply, independently of application retries.
// HTTP/2 can also retry streams. Use a fresh, single-use HTTP/1 connection and
// refuse redirects, while preserving this client's TLS, proxy, dial and timeout
// policy. Ordinary API reads keep their existing pooled client and recovery.
func guestAgentCommandClient(client *http.Client) (*http.Client, error) {
	if client == nil {
		return nil, &guestAgentDeferredError{reason: "agent-transport-unverified"}
	}
	roundTripper := client.Transport
	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}
	base, ok := roundTripper.(*http.Transport)
	if !ok || base == nil || base.DialTLS != nil || base.DialTLSContext != nil {
		// An opaque transport or TLS dialer cannot establish these no-replay
		// properties. Do not silently fall back to another trust/dial policy.
		return nil, &guestAgentDeferredError{reason: "agent-transport-unverified"}
	}
	transport := base.Clone()
	transport.DisableKeepAlives = true
	transport.ForceAttemptHTTP2 = false
	transport.Protocols = new(http.Protocols)
	transport.Protocols.SetHTTP1(true)
	transport.TLSNextProto = nil
	if transport.TLSClientConfig != nil {
		transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
	}
	commandClient := *client
	commandClient.Transport = transport
	commandClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &commandClient, nil
}
