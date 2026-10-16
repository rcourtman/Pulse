package pbs

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/testutil/proxyfixture"
)

func TestClientPlaintextProxyRouting(t *testing.T) {
	if !proxyfixture.Isolated(t, "TestClientPlaintextProxyRouting") {
		return
	}
	for _, host := range []string{"pve.example:8006", "203.0.113.10:8007", "[2001:db8::10]:8006"} {
		for _, mode := range []string{"token", "json-login", "form-login"} {
			t.Run(host+"/"+mode, func(t *testing.T) {
				c, err := NewClient(ClientConfig{Host: "http://" + host + "/proxy", TokenName: "fixture@pam!pulse", TokenValue: "fixture-token", VerifySSL: true, Timeout: 2 * time.Second})
				if err != nil {
					t.Fatal(err)
				}
				observed := proxyfixture.Pipe(t, c.httpClient.Transport.(*http.Transport), nil, func(r *http.Request, _ []byte) (int, string) {
					if strings.HasSuffix(r.URL.Path, "/access/ticket") {
						if mode == "form-login" && r.Header.Get("Content-Type") == "application/json" {
							return 415, `{"errors":{"message":"fixture form required"}}`
						}
						return 200, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`
					}
					if strings.HasSuffix(r.URL.Path, "/version") {
						return 200, `{"data":{"version":"fixture"}}`
					}
					return 200, `{"data":null}`
				})
				next := func(path, method string) proxyfixture.Observation {
					got := observed()
					if got.Err != nil {
						t.Fatal(got.Err)
					}
					if got.Address != host || got.Method != method || got.URI != "/proxy/api2/json"+path || got.Host != host {
						t.Errorf("provider credential request escaped configured endpoint: address=%s method=%s uri=%s host=%s", got.Address, got.Method, got.URI, got.Host)
					}
					if got.Headers.Get("Proxy-Authorization") != "" {
						t.Error("proxy credentials reached provider")
					}
					return got
				}
				if mode != "token" {
					// Retain the constructor's actual factory/transport; seed password mode
					// before exercising the real JSON/form login and subsequent ordinary reads.
					c.config.Password = "fixture-password"
					c.auth.tokenName, c.auth.tokenValue = "", ""
					if err := c.authenticate(context.Background()); err != nil {
						t.Fatal(err)
					}
					login := next("/access/ticket", http.MethodPost)
					if string(login.Body) != `{"password":"fixture-password","username":"fixture@pam"}` || login.Headers.Get("Content-Type") != "application/json" {
						t.Error("JSON login bytes changed")
					}
					if mode == "form-login" {
						form := next("/access/ticket", http.MethodPost)
						if string(form.Body) != "password=fixture-password&username=fixture%40pam" || form.Headers.Get("Content-Type") != "application/x-www-form-urlencoded" {
							t.Error("form fallback bytes changed")
						}
					}
				}
				version, err := c.GetVersion(context.Background())
				if err != nil || version == nil || version.Version != "fixture" {
					t.Fatalf("ordinary version read: error=%v", err)
				}
				read := next("/version", http.MethodGet)
				if len(read.Body) != 0 {
					t.Error("ordinary GET acquired a body")
				}
				if mode == "token" {
					if read.Headers.Get("Authorization") != "PBSAPIToken=fixture@pam!pulse:fixture-token" || read.Headers.Get("Cookie") != "" {
						t.Error("provider token format changed")
					}
				} else if read.Headers.Get("Authorization") != "" || read.Headers.Get("Cookie") != "PBSAuthCookie=fixture-ticket" || read.Headers.Get("CSRFPreventionToken") != "" {
					t.Error("ordinary ticket read changed")
				}
				response, err := c.request(context.Background(), http.MethodPost, "/fixture-write", url.Values{"fixture": {"value"}})
				if err != nil {
					t.Fatal(err)
				}
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
				write := next("/fixture-write", http.MethodPost)
				if string(write.Body) != "fixture=value" {
					t.Error("write body changed")
				}
				if mode != "token" && (write.Headers.Get("Cookie") != "PBSAuthCookie=fixture-ticket" || write.Headers.Get("CSRFPreventionToken") != "fixture-csrf") {
					t.Error("ticket/CSRF write changed")
				}
			})
		}
	}
}
