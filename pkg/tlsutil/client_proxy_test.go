package tlsutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/testutil/proxyfixture"
)

func TestInfrastructureProxyRouting(t *testing.T) {
	if !proxyfixture.Isolated(t, "TestInfrastructureProxyRouting") {
		return
	}
	for _, host := range []string{"pve.example:8006", "203.0.113.10:8007", "[2001:db8::10]:8006", "192.168.50.10:8006", "[fd00::10]:8006", "pve", "pve.local", "100.127.50.10:8006"} {
		for _, mode := range []string{"token", "ticket", "json-login", "form-login"} {
			t.Run("http/"+host+"/"+mode, func(t *testing.T) {
				client := CreateHTTPClientWithTimeout(true, "", 2*time.Second)
				observed := proxyfixture.Pipe(t, client.Transport.(*http.Transport), nil, func(*http.Request, []byte) (int, string) { return 200, `{"data":[]}` })
				req, wantBody := infrastructureProxyRequest(t, "http://"+host+"/proxy/api2/json/nodes?view=full", mode)
				response, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				_, readErr := io.ReadAll(response.Body)
				response.Body.Close()
				got := observed()
				if readErr != nil || got.Err != nil {
					t.Fatalf("body=%v wire=%v", readErr, got.Err)
				}
				if got.Address != host && got.Address != host+":80" {
					t.Errorf("plaintext dial=%s, want configured endpoint %s (credential header=%t, login body=%t)", got.Address, host, got.Headers.Get("Authorization") != "" || got.Headers.Get("Cookie") != "", len(got.Body) > 0)
				}
				checkInfrastructureProxyRequest(t, got, req, wantBody)
			})
		}
	}

	for _, tc := range []struct {
		name, host, mode string
		direct, fail     bool
	}{
		{name: "ca", host: "pve.example:8006", mode: "ca"},
		{name: "matching-pin", host: "pve.example:8006", mode: "pin"},
		{name: "explicit-opt-out", host: "pve.example:8006", mode: "opt-out"},
		{name: "wrong-pin", host: "pve.example:8006", mode: "wrong-pin", fail: true},
		{name: "untrusted", host: "pve.example:8006", mode: "untrusted", fail: true},
		{name: "no-proxy", host: "excluded.example:8006", mode: "ca", direct: true},
		{name: "local-host", host: "pve.local:8006", mode: "ca", direct: true},
	} {
		t.Run("https/"+tc.name, func(t *testing.T) {
			target, err := url.Parse("https://" + tc.host + "/proxy/api2/json/nodes?view=full")
			if err != nil {
				t.Fatal(err)
			}
			certificate, roots := proxyfixture.Certificate(t, target.Hostname())
			pin := ""
			if tc.mode == "pin" {
				sum := sha256.Sum256(certificate.Certificate[0])
				pin = hex.EncodeToString(sum[:])
			}
			if tc.mode == "wrong-pin" {
				pin = strings.Repeat("0", 64)
			}
			client := CreateHTTPClient(tc.mode != "opt-out", pin)
			transport := client.Transport.(*http.Transport)
			if tc.mode == "ca" {
				transport.TLSClientConfig.RootCAs = roots
			}
			observed := proxyfixture.Pipe(t, transport, &certificate, func(*http.Request, []byte) (int, string) { return 200, `{"data":[]}` })
			req, body := infrastructureProxyRequest(t, target.String(), "ticket")
			response, err := client.Do(req)
			if response != nil {
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
			}
			got := observed()
			if tc.direct {
				if got.Address != tc.host || got.Connect {
					t.Errorf("direct HTTPS policy changed: address=%s connect=%t", got.Address, got.Connect)
				}
			} else {
				if got.Address != proxyfixture.Address || !got.Connect || got.ConnectAuthority != tc.host {
					t.Fatalf("HTTPS lost original CONNECT routing: address=%s connect=%t", got.Address, got.Connect)
				}
				for _, key := range []string{"Authorization", "Cookie", "CSRFPreventionToken"} {
					if got.ConnectHeaders.Get(key) != "" {
						t.Errorf("credential %s entered CONNECT", key)
					}
				}
				if got.ConnectHeaders.Get("Proxy-Authorization") == "" {
					t.Error("explicit environment proxy authentication lost")
				}
			}
			if tc.fail {
				if err == nil || got.Err == nil || got.Headers != nil || len(got.Body) != 0 {
					t.Fatalf("unverified TLS carried credentials: client=%v server=%v", err, got.Err)
				}
				if tc.mode == "wrong-pin" && !strings.Contains(err.Error(), "fingerprint mismatch") {
					t.Errorf("wrong pin diagnostic: %v", err)
				}
				return
			}
			if err != nil || got.Err != nil {
				t.Fatalf("verified TLS: client=%v server=%v", err, got.Err)
			}
			if got.ServerName != target.Hostname() {
				t.Errorf("TLS identity changed: %q", got.ServerName)
			}
			checkInfrastructureProxyRequest(t, got, req, body)
		})
	}
}

func infrastructureProxyRequest(t *testing.T, target, mode string) (*http.Request, []byte) {
	t.Helper()
	method, body, contentType := http.MethodGet, "", ""
	if mode == "json-login" {
		method, body, contentType = http.MethodPost, `{"password":"fixture-password","username":"fixture@pam"}`, "application/json"
	}
	if mode == "form-login" {
		method, body, contentType = http.MethodPost, "password=fixture-password&username=fixture%40pam", "application/x-www-form-urlencoded"
	}
	req, err := http.NewRequest(method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if mode == "token" {
		req.Header.Set("Authorization", "PVEAPIToken=fixture@pam!pulse=fixture-token")
	}
	if mode == "ticket" {
		req.Header.Set("Cookie", "PVEAuthCookie=fixture-ticket")
		req.Header.Set("CSRFPreventionToken", "fixture-csrf")
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, []byte(body)
}

func checkInfrastructureProxyRequest(t *testing.T, got proxyfixture.Observation, req *http.Request, body []byte) {
	t.Helper()
	if got.Method != req.Method || got.URI != req.URL.RequestURI() || got.Host != req.URL.Host || !bytes.Equal(got.Body, body) {
		t.Error("wire request destination, encoding or body changed")
	}
	for _, key := range []string{"Authorization", "Cookie", "CSRFPreventionToken", "Content-Type"} {
		if got.Headers.Get(key) != req.Header.Get(key) {
			t.Errorf("wire %s changed", key)
		}
	}
	if got.Headers.Get("Proxy-Authorization") != "" {
		t.Error("proxy credentials entered infrastructure request")
	}
}
