package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersWithConfigDisallowsDocumentBaseOverrides(t *testing.T) {
	for _, dev := range []bool{false, true} {
		for _, embedded := range []bool{false, true} {
			for _, secure := range []bool{false, true} {
				t.Run(fmt.Sprintf("development=%t/embedding=%t/tls=%t", dev, embedded, secure), func(t *testing.T) {
					const body = `<html><body><a href="/docs/TROUBLESHOOTING">Help</a></body></html>`
					nonce := ""
					handler := SecurityHeadersWithConfig(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						nonce = CSPNonceFromContext(r.Context())
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						_, _ = w.Write([]byte(body))
					}), embedded, "https://dashboard.example.invalid", dev)
					scheme := "http"
					if secure {
						scheme = "https"
					}
					req := httptest.NewRequest(http.MethodGet, scheme+"://pulse.example.invalid/", nil)
					req.RemoteAddr = "192.0.2.10:12345"
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, req)
					directives := map[string][]string{}
					for _, raw := range strings.Split(rec.Header().Get("Content-Security-Policy"), ";") {
						fields := strings.Fields(raw)
						if len(fields) == 0 {
							continue
						}
						if _, duplicate := directives[fields[0]]; duplicate {
							t.Fatalf("duplicate CSP directive %q", fields[0])
						}
						directives[fields[0]] = fields[1:]
					}
					if got := strings.Join(directives["base-uri"], " "); got != "'none'" {
						t.Errorf("base-uri = %q, want 'none': default-src does not restrict HTML base elements", got)
					}
					// This adds a restriction on unused base tags, never weakens the
					// existing script/style, resource, embedding or transport policy.
					script := "'self' 'unsafe-inline' 'unsafe-eval'"
					style := "'self' 'unsafe-inline'"
					if !dev {
						if nonce == "" {
							t.Fatal("production handler did not receive its per-request nonce")
						}
						script = "'self' 'nonce-" + nonce + "'"
						style = script
					} else if nonce != "" {
						t.Fatal("development must retain its nonce-free HMR policy")
					}
					want := map[string]string{"default-src": "'self'", "script-src": script, "style-src": style, "img-src": "'self' data: blob:", "connect-src": "'self' ws: wss:", "font-src": "'self' data:", "frame-ancestors": "'none'"}
					if embedded {
						want["frame-ancestors"] = "'self' https://dashboard.example.invalid"
					}
					for directive, value := range want {
						if got := strings.Join(directives[directive], " "); got != value {
							t.Errorf("%s = %q, want %q", directive, got, value)
						}
					}
					_, upgrade := directives["upgrade-insecure-requests"]
					if upgrade != secure {
						t.Errorf("upgrade-insecure-requests = %t, want %t", upgrade, secure)
					}
					if (rec.Header().Get("Strict-Transport-Security") != "") != secure {
						t.Error("HSTS transport policy changed")
					}
					if embedded && rec.Header().Get("X-Frame-Options") != "" {
						t.Error("explicit embedding was disabled")
					}
					if !embedded && rec.Header().Get("X-Frame-Options") != "DENY" {
						t.Error("embedding denial was removed")
					}
					if rec.Code != http.StatusOK || rec.Body.String() != body {
						t.Error("middleware changed the downstream response")
					}
				})
			}
		}
	}
}
