package handoff

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// ingressWithCertificateFor answers TLS for one name and refuses the rest with a
// TLS alert, as Traefik does with sniStrict.
func ingressWithCertificateFor(t *testing.T, name string, notAfter time.Time) (string, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	certificate := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if hello.ServerName == name {
				return &certificate, nil
			}
			return nil, tls.AlertError(112)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if tlsConn, ok := c.(*tls.Conn); ok {
					_ = tlsConn.Handshake()
				}
				_, _ = io.Copy(io.Discard, c)
			}(conn)
		}
	}()
	return listener.Addr().String(), roots
}

// closedAddr returns an address nothing is listening on, like Traefik mid-restart.
func closedAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func gateFixture(t *testing.T, createdAt time.Time) (*registry.TenantRegistry, *registry.Tenant) {
	t.Helper()
	reg, err := registry.NewTenantRegistry(filepath.Join(t.TempDir(), "registry"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "MSP"}); err != nil {
		t.Fatal(err)
	}
	tenant := &registry.Tenant{
		ID:          "t-ABC123",
		AccountID:   accountID,
		DisplayName: "Client",
		State:       registry.TenantStateActive,
		ContainerID: "container-1",
		CreatedAt:   createdAt,
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatal(err)
	}
	return reg, tenant
}

func gatedRequestWithContext(t *testing.T, ctx context.Context, gate *ClientCertificateGate, accountID, tenantID string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	passed := false
	handler := gate.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		passed = true
		w.WriteHeader(http.StatusOK)
	}))
	mux := http.NewServeMux()
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}/handoff", handler)
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants/"+tenantID+"/handoff?target_path=%2Fsettings", nil).WithContext(ctx)
	req.Host = "pulse.203-0-113-7.sslip.io"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec, passed
}

func gatedRequest(t *testing.T, gate *ClientCertificateGate, accountID, tenantID string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	return gatedRequestWithContext(t, context.Background(), gate, accountID, tenantID)
}

func TestClientCertificateGatePassesOnceTheNameHasACertificate(t *testing.T) {
	reg, tenant := gateFixture(t, time.Now().Add(-time.Hour))
	addr, roots := ingressWithCertificateFor(t, "t-abc123.pulse.203-0-113-7.sslip.io", time.Now().Add(time.Hour))
	var restarts atomic.Int32
	gate := &ClientCertificateGate{
		Registry:       reg,
		IngressAddr:    addr,
		RootCAs:        roots,
		Wait:           2 * time.Second,
		RestartIngress: func(context.Context) error { restarts.Add(1); return nil },
	}
	rec, passed := gatedRequest(t, gate, tenant.AccountID, tenant.ID)
	if !passed || rec.Code != http.StatusOK {
		t.Fatalf("handoff must continue, got passed=%v code=%d", passed, rec.Code)
	}
	if restarts.Load() != 0 {
		t.Fatal("a client with a certificate must not cause a retry")
	}
}

func TestClientCertificateGateHoldsAndRetriesWhenTraefikHasNoCertificate(t *testing.T) {
	reg, tenant := gateFixture(t, time.Now().Add(-time.Hour))
	addr, roots := ingressWithCertificateFor(t, "some-other-client.example.test", time.Now().Add(time.Hour))
	restarted := make(chan struct{}, 4)
	gate := &ClientCertificateGate{
		Registry:       reg,
		IngressAddr:    addr,
		RootCAs:        roots,
		Wait:           50 * time.Millisecond,
		RestartIngress: func(context.Context) error { restarted <- struct{}{}; return nil },
	}
	rec, passed := gatedRequest(t, gate, tenant.AccountID, tenant.ID)
	if passed {
		t.Fatal("handoff must not continue to an address that refuses TLS")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"has not issued this client's certificate",
		"port 80",
		`method="POST"`,
		"target_path=%2Fsettings",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("wait page is missing %q:\n%s", want, body)
		}
	}
	select {
	case <-restarted:
	case <-time.After(2 * time.Second):
		t.Fatal("a missing certificate must make Traefik ask again")
	}

	// The wait page resubmits every few seconds and Let's Encrypt allows only 5
	// failed checks an hour, so a second hit inside the interval must not retry.
	gatedRequest(t, gate, tenant.AccountID, tenant.ID)
	select {
	case <-restarted:
		t.Fatal("retry must be throttled")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestClientCertificateGateGivesANewClientTimeBeforeRetrying(t *testing.T) {
	reg, tenant := gateFixture(t, time.Now())
	addr, roots := ingressWithCertificateFor(t, "some-other-client.example.test", time.Now().Add(time.Hour))
	var restarts atomic.Int32
	gate := &ClientCertificateGate{
		Registry:       reg,
		IngressAddr:    addr,
		RootCAs:        roots,
		Wait:           50 * time.Millisecond,
		RestartIngress: func(context.Context) error { restarts.Add(1); return nil },
	}
	if _, passed := gatedRequest(t, gate, tenant.AccountID, tenant.ID); passed {
		t.Fatal("handoff must wait")
	}
	time.Sleep(50 * time.Millisecond)
	if restarts.Load() != 0 {
		t.Fatal("first issuance is still running; restarting Traefik would only interrupt it")
	}
}

func TestClientCertificateGateTreatsAnExpiredCertificateAsMissing(t *testing.T) {
	addr, roots := ingressWithCertificateFor(t, "t-abc123.example.test", time.Now().Add(-time.Minute))
	gate := &ClientCertificateGate{IngressAddr: addr, RootCAs: roots}
	if got := gate.state(context.Background(), "t-abc123.example.test"); got != clientCertificateMissing {
		t.Fatalf("an expired certificate would still fail in the browser, got %d", got)
	}
}

func TestClientCertificateGateTreatsAnUntrustedCertificateAsMissing(t *testing.T) {
	addr, _ := ingressWithCertificateFor(t, "t-abc123.example.test", time.Now().Add(time.Hour))
	gate := &ClientCertificateGate{IngressAddr: addr, RootCAs: x509.NewCertPool()}
	if got := gate.state(context.Background(), "t-abc123.example.test"); got != clientCertificateMissing {
		t.Fatalf("a certificate the browser would reject must not count as ready, got %d", got)
	}
}

// Traefik being down says nothing about the certificate. Waiting is right, but
// nothing may be restarted on the strength of it.
func TestClientCertificateGateDoesNotRetryWhenTraefikCannotBeReached(t *testing.T) {
	reg, tenant := gateFixture(t, time.Now().Add(-time.Hour))
	var restarts atomic.Int32
	gate := &ClientCertificateGate{
		Registry:       reg,
		IngressAddr:    closedAddr(t),
		Wait:           50 * time.Millisecond,
		RestartIngress: func(context.Context) error { restarts.Add(1); return nil },
	}
	rec, passed := gatedRequest(t, gate, tenant.AccountID, tenant.ID)
	if passed || rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("handoff must wait while Traefik is unreachable: passed=%v code=%d", passed, rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "could not check this client's certificate") || strings.Contains(body, "has not issued") {
		t.Fatalf("the page must not claim a failed check is a failed issuance:\n%s", body)
	}
	time.Sleep(50 * time.Millisecond)
	if restarts.Load() != 0 {
		t.Fatal("an unreachable Traefik must not count as a missing certificate")
	}
}

// A user closing the tab mid-wait must not be able to start a retry.
func TestClientCertificateGateDoesNotRetryForACancelledRequest(t *testing.T) {
	reg, tenant := gateFixture(t, time.Now().Add(-time.Hour))
	addr, roots := ingressWithCertificateFor(t, "some-other-client.example.test", time.Now().Add(time.Hour))
	var restarts atomic.Int32
	gate := &ClientCertificateGate{
		Registry:       reg,
		IngressAddr:    addr,
		RootCAs:        roots,
		Wait:           time.Minute,
		RestartIngress: func(context.Context) error { restarts.Add(1); return nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, passed := gatedRequestWithContext(t, ctx, gate, tenant.AccountID, tenant.ID)
	if passed {
		t.Fatal("a cancelled request must not continue to the handoff")
	}
	time.Sleep(50 * time.Millisecond)
	if restarts.Load() != 0 {
		t.Fatal("a cancelled request must not start a retry")
	}
}

// A name that can never be issued must not keep restarting the only ingress, and
// opening some other healthy client must not shorten the gap.
func TestClientCertificateGateBacksOffAndOnlyTheStuckClientResetsIt(t *testing.T) {
	reg, tenant := gateFixture(t, time.Now().Add(-time.Hour))
	addr, roots := ingressWithCertificateFor(t, "some-other-client.example.test", time.Now().Add(time.Hour))
	restarted := make(chan struct{}, 16)
	now := time.Now()
	gate := &ClientCertificateGate{
		Registry:       reg,
		IngressAddr:    addr,
		RootCAs:        roots,
		Wait:           20 * time.Millisecond,
		Now:            func() time.Time { return now },
		RestartIngress: func(context.Context) error { restarted <- struct{}{}; return nil },
	}
	hit := func() bool {
		t.Helper()
		gatedRequest(t, gate, tenant.AccountID, tenant.ID)
		select {
		case <-restarted:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}
	if !hit() {
		t.Fatal("first miss must retry")
	}
	for _, gap := range []time.Duration{15, 30, 60, 120, 240, 360, 360} {
		now = now.Add((gap - 1) * time.Minute)
		if hit() {
			t.Fatalf("retried before the %d minute gap ended", gap)
		}
		gate.issued("t-SOMEONE-ELSE")
		if hit() {
			t.Fatal("another client's certificate must not shorten the gap")
		}
		now = now.Add(time.Minute)
		if !hit() {
			t.Fatalf("expected a retry after the %d minute gap", gap)
		}
	}
	gate.issued(tenant.ID)
	if !hit() {
		t.Fatal("once the stuck client is issued, a later miss may retry straight away")
	}
}

func TestClientCertificateGateBackoffStaysWithTheClientThatStartedIt(t *testing.T) {
	first := &registry.Tenant{ID: "t-FIRST", CreatedAt: time.Now().Add(-time.Hour)}
	second := &registry.Tenant{ID: "t-SECOND", CreatedAt: time.Now().Add(-time.Hour)}
	restarted := make(chan struct{}, 16)
	now := time.Now()
	gate := &ClientCertificateGate{
		Now:            func() time.Time { return now },
		RestartIngress: func(context.Context) error { restarted <- struct{}{}; return nil },
	}
	retried := func(tenant *registry.Tenant) bool {
		t.Helper()
		gate.retryIssuance(tenant)
		select {
		case <-restarted:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}
	if !retried(first) {
		t.Fatal("first miss must retry")
	}
	now = now.Add(15 * time.Minute)
	if !retried(second) {
		t.Fatal("a second stuck client may use the next retry")
	}
	gate.issued(second.ID)
	if retried(first) {
		t.Fatal("the second client's certificate must not end the first client's backoff")
	}
	now = now.Add(30 * time.Minute)
	if !retried(first) {
		t.Fatal("expected a retry after the doubled gap")
	}
	gate.issued(first.ID)
	if !retried(second) {
		t.Fatal("the client that started the backoff ends it")
	}
}

// Who may enter which client stays with the handoff handler: the gate must not
// answer differently, retry or probe for a tenant outside the account.
func TestClientCertificateGateLeavesForeignTenantsToTheHandoffHandler(t *testing.T) {
	reg, _ := gateFixture(t, time.Now().Add(-time.Hour))
	var restarts atomic.Int32
	gate := &ClientCertificateGate{
		Registry:       reg,
		IngressAddr:    closedAddr(t),
		Wait:           time.Hour,
		RestartIngress: func(context.Context) error { restarts.Add(1); return nil },
	}
	rec, passed := gatedRequest(t, gate, "a-other", "t-ABC123")
	if !passed || rec.Code != http.StatusOK || restarts.Load() != 0 {
		t.Fatalf("foreign tenant must pass straight to the handoff handler: passed=%v code=%d restarts=%d", passed, rec.Code, restarts.Load())
	}
}
