package handoff

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"html/template"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

const (
	// Issuance normally finishes within seconds of a client turning healthy.
	clientCertificateWait = 20 * time.Second
	// Traefik does not retry a refused order on its own. Restarting it asks again
	// for every name it lacks, without touching any client runtime, but it briefly
	// interrupts every client. Let's Encrypt allows 5 failed checks per name per
	// hour, and a name may never be issuable, so the gap doubles until the client
	// that started it gets its certificate. A new client's first order gets time
	// to finish.
	clientCertificateRetryAfter       = time.Minute
	clientCertificateRetryMinInterval = 15 * time.Minute
	clientCertificateRetryMaxInterval = 6 * time.Hour
)

// ClientCertificateGate holds a handoff until the client's own certificate
// exists, for installs that issue one per client over HTTP-01. Without it, the
// handoff lands on a browser TLS error, because Traefik refuses a name it holds
// no certificate for (sniStrict).
type ClientCertificateGate struct {
	Registry *registry.TenantRegistry
	// IngressAddr is where Traefik's TLS entrypoint answers on the provider network.
	IngressAddr string
	// RootCAs verifies the served certificate. Nil means the system roots.
	RootCAs *x509.CertPool
	// RestartIngress restarts Traefik.
	RestartIngress func(ctx context.Context) error
	// Now and Wait can be replaced in tests.
	Now  func() time.Time
	Wait time.Duration

	mu            sync.Mutex
	lastRetry     time.Time
	retryInterval time.Duration
	retryTenantID string
}

type clientCertificateState int

const (
	// clientCertificateUnknown means the check could not reach an answer. It says
	// nothing about the certificate and must never start a retry.
	clientCertificateUnknown clientCertificateState = iota
	clientCertificateMissing
	clientCertificateReady
)

// state asks Traefik for host's certificate and verifies it as a browser would.
func (g *ClientCertificateGate) state(ctx context.Context, host string) clientCertificateState {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 3 * time.Second},
		Config:    &tls.Config{ServerName: host, RootCAs: g.RootCAs, Time: g.now, MinVersion: tls.VersionTLS12},
	}
	conn, err := dialer.DialContext(ctx, "tcp", g.IngressAddr)
	if err != nil {
		// A TLS alert from Traefik is its answer that it holds nothing for this
		// name, and a failed verification would fail in the browser too. Timeouts,
		// refused connections and cancelled requests say nothing about the
		// certificate.
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "remote error" {
			return clientCertificateMissing
		}
		var verifyErr *tls.CertificateVerificationError
		if errors.As(err, &verifyErr) {
			return clientCertificateMissing
		}
		return clientCertificateUnknown
	}
	_ = conn.Close()
	return clientCertificateReady
}

func (g *ClientCertificateGate) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

func (g *ClientCertificateGate) wait() time.Duration {
	if g.Wait > 0 {
		return g.Wait
	}
	return clientCertificateWait
}

// await returns as soon as the certificate is served, otherwise the last answer
// when the wait ends.
func (g *ClientCertificateGate) await(ctx context.Context, host string) clientCertificateState {
	ctx, cancel := context.WithTimeout(ctx, g.wait())
	defer cancel()
	last := clientCertificateUnknown
	for {
		if state := g.state(ctx, host); ctx.Err() == nil {
			last = state
		}
		if last == clientCertificateReady {
			return last
		}
		select {
		case <-ctx.Done():
			return last
		case <-time.After(time.Second):
		}
	}
}

// issued ends the backoff once the client that started it has its certificate. A
// healthy client's handoff does not, so opening one cannot shorten the gap.
func (g *ClientCertificateGate) issued(tenantID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.retryTenantID == tenantID {
		g.lastRetry = time.Time{}
		g.retryInterval = 0
		g.retryTenantID = ""
	}
}

func (g *ClientCertificateGate) retryIssuance(tenant *registry.Tenant) {
	if g.RestartIngress == nil {
		return
	}
	now := g.now()
	if now.Sub(tenant.CreatedAt) < clientCertificateRetryAfter {
		return
	}
	g.mu.Lock()
	if !g.lastRetry.IsZero() && now.Sub(g.lastRetry) < g.retryInterval {
		g.mu.Unlock()
		return
	}
	g.lastRetry = now
	if g.retryTenantID == "" {
		g.retryTenantID = tenant.ID
	}
	switch {
	case g.retryInterval == 0:
		g.retryInterval = clientCertificateRetryMinInterval
	case g.retryInterval*2 < clientCertificateRetryMaxInterval:
		g.retryInterval *= 2
	default:
		g.retryInterval = clientCertificateRetryMaxInterval
	}
	g.mu.Unlock()

	tenantID := tenant.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := g.RestartIngress(ctx); err != nil {
			log.Warn().Err(err).Str("tenant_id", tenantID).Msg("Could not restart Traefik to retry a client certificate")
			return
		}
		log.Info().Str("tenant_id", tenantID).Msg("Restarted Traefik to retry a client certificate")
	}()
}

var clientCertificateWaitTemplate = template.Must(template.New("client-certificate-wait").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style nonce="{{.Nonce}}">
body{font:16px/1.5 system-ui,sans-serif;max-width:34rem;margin:15vh auto;padding:0 1rem;color:#1f2937}
@media (prefers-color-scheme:dark){body{background:#111827;color:#e5e7eb}}
h1{font-size:1.25rem}
</style></head><body>
<h1>{{.Title}}</h1>
{{if .Unavailable}}<p>Pulse could not check this client's certificate just now, usually because Traefik, its web proxy, is restarting. This page keeps trying and opens the client when it can.</p>
<p>If this lasts more than a few minutes, check that the traefik container is running on this server.</p>
{{else}}<p>Let's Encrypt has not issued this client's certificate yet. This page keeps trying and opens the client when it can.</p>
<p>If this lasts more than a few minutes, check that port 80 on this server is reachable from the internet. Let's Encrypt also limits new certificates per domain. Trial addresses share that limit with everyone using sslip.io and nip.io, so it can take minutes or longer to clear.</p>
{{end}}<form method="POST" action="{{.Action}}"></form>
<script nonce="{{.Nonce}}">setTimeout(function(){document.forms[0].submit()},10000)</script>
</body></html>
`))

type clientCertificateWaitPage struct {
	Title, Nonce, Action string
	Unavailable          bool
}

// Require wraps the handoff handler. Requests it cannot judge pass through, so
// the handoff handler keeps the only say on who may enter which client.
func (g *ClientCertificateGate) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID := strings.TrimSpace(r.PathValue("account_id"))
		tenantID := strings.TrimSpace(r.PathValue("tenant_id"))
		baseDomain := deriveBaseDomain(r)
		if r.Method != http.MethodPost || g == nil || g.Registry == nil || accountID == "" || tenantID == "" || baseDomain == "" {
			next.ServeHTTP(w, r)
			return
		}
		tenant, err := g.Registry.GetTenantForAccount(accountID, tenantID)
		if err != nil || tenant == nil {
			next.ServeHTTP(w, r)
			return
		}
		host := strings.ToLower(tenantID + "." + baseDomain)
		state := g.await(r.Context(), host)
		if r.Context().Err() != nil {
			return
		}
		if state == clientCertificateReady {
			g.issued(tenant.ID)
			next.ServeHTTP(w, r)
			return
		}
		if state == clientCertificateMissing {
			g.retryIssuance(tenant)
		}
		log.Info().Str("tenant_id", tenantID).Bool("certificate_missing", state == clientCertificateMissing).Msg("Handoff held: client certificate not served yet")

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		page := clientCertificateWaitPage{
			Title:  "Waiting for this client's certificate",
			Nonce:  cpsec.NonceFromContext(r.Context()),
			Action: r.URL.RequestURI(),
		}
		if state == clientCertificateUnknown {
			page.Title = "Checking this client's certificate"
			page.Unavailable = true
		}
		if err := clientCertificateWaitTemplate.Execute(w, page); err != nil {
			log.Error().Err(err).Str("tenant_id", tenantID).Msg("cloudcp.handoff: render certificate wait page")
		}
	})
}
