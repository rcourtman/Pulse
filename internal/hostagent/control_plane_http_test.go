package hostagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type controlPlaneTestTransport func(*http.Request) (*http.Response, error)

func (f controlPlaneTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type controlPlaneTestBody struct{ reads, closes int }

func (b *controlPlaneTestBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("synthetic-private-body-read")
}

func (b *controlPlaneTestBody) Close() error {
	b.closes++
	return errors.New("synthetic-private-body-close")
}

func TestControlPlaneRedirectClosesWithoutReading(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			body := &controlPlaneTestBody{}
			client := &http.Client{Transport: controlPlaneTestTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Status: "synthetic-private-status", Header: http.Header{"Location": {"http://[synthetic-private-location"}}, Body: body}, nil
			})}
			request, err := http.NewRequest(http.MethodPost, "https://pulse.example/api/agents/agent/report", strings.NewReader("synthetic-private-request"))
			if err != nil {
				t.Fatal(err)
			}
			response, err := doControlPlaneRequest(client, request)
			var redirect *controlPlaneRedirectError
			if response != nil || !errors.As(err, &redirect) || redirect.statusCode != status {
				t.Fatal("redirect lost its local status or returned an open response")
			}
			if _, wrapped := err.(*url.Error); wrapped {
				t.Fatal("redirect retained net/http URL wrapper")
			}
			assertPrivateRedirectDiagnostic(t, err.Error(), "synthetic-private")
			if body.reads != 0 || body.closes != 1 {
				t.Fatalf("redirect body reads/closes = %d/%d; want 0/1", body.reads, body.closes)
			}
		})
	}
}

func TestControlPlaneRequestPreservesClientAndOrdinaryResponses(t *testing.T) {
	for _, status := range []int{200, 204, 300, 301, 302, 303, 307, 308, 401, 403, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			base, _ := url.Parse("https://pulse.example")
			jar.SetCookies(base, []*http.Cookie{{Name: "synthetic-cookie", Value: "saved"}})
			body := io.NopCloser(strings.NewReader("synthetic-response-body"))
			original := &http.Response{StatusCode: status, Header: http.Header{"Set-Cookie": {"synthetic-cookie=updated; Path=/"}}, Body: body}
			calls := 0
			transport := controlPlaneTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Header.Get("Authorization") != "Bearer synthetic-credential" || r.Header.Get("X-API-Token") != "synthetic-credential" || r.Method != http.MethodPost || r.URL.String() != "https://pulse.example/api/agents/agent/report" || r.Context().Err() != nil {
					t.Error("ordinary request changed")
				}
				if c, err := r.Cookie("synthetic-cookie"); err != nil || c.Value != "saved" {
					t.Error("client cookie handling was lost")
				}
				return original, nil
			})
			policy := errors.New("synthetic-caller-policy")
			client := &http.Client{Transport: transport, Timeout: time.Second, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return policy }}
			request, _ := http.NewRequest(http.MethodPost, "https://pulse.example/api/agents/agent/report", strings.NewReader("synthetic-report"))
			request.Header.Set("Authorization", "Bearer synthetic-credential")
			request.Header.Set("X-API-Token", "synthetic-credential")
			response, err := doControlPlaneRequest(client, request)
			if err != nil || response != original || calls != 1 || response.StatusCode != status {
				t.Fatal("non-redirect or no-Location response changed")
			}
			defer response.Body.Close()
			got, err := io.ReadAll(response.Body)
			if err != nil || string(got) != "synthetic-response-body" {
				t.Fatal("ordinary response body changed")
			}
			if cookies := jar.Cookies(base); len(cookies) != 1 || cookies[0].Value != "updated" {
				t.Fatal("ordinary response cookie handling was lost")
			}
			if client.Timeout != time.Second || client.Jar != jar || client.CheckRedirect(nil, nil) != policy {
				t.Fatal("caller client was mutated")
			}
			if _, ok := client.Transport.(controlPlaneTestTransport); !ok {
				t.Fatal("caller transport was replaced")
			}
		})
	}
}

func TestControlPlaneRequestRetainsCancellationAndTimeout(t *testing.T) {
	for _, canceled := range []bool{true, false} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			client := &http.Client{Timeout: 20 * time.Millisecond, Transport: controlPlaneTestTransport(func(r *http.Request) (*http.Response, error) {
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := context.DeadlineExceeded
			if canceled {
				cancel()
				want = context.Canceled
			}
			request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://pulse.example/api/agents/agent/report", nil)
			response, err := doControlPlaneRequest(client, request)
			if response != nil || !errors.Is(err, want) {
				t.Fatal("transport error lost cancellation/timeout identity")
			}
		})
	}
}

func TestControlPlaneRedirectRetainsTLSVerification(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Location", "https://synthetic-private.example/?credential=synthetic-private")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(server.Certificate().Raw)
	for _, mode := range []string{"custom-ca", "exact-pin", "untrusted", "wrong-pin"} {
		t.Run(mode, func(t *testing.T) {
			ca, pin, insecure := "", "", false
			switch mode {
			case "custom-ca":
				ca = caPath
			case "exact-pin":
				pin, insecure = hex.EncodeToString(digest[:]), true
			case "wrong-pin":
				pin, insecure = strings.Repeat("00", 32), true
			}
			client, err := newAgentHTTPClient(ca, insecure, pin)
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			before := requests.Load()
			request, _ := http.NewRequest(http.MethodPost, server.URL+enrollEndpoint, nil)
			request.Header.Set("Authorization", "Bearer synthetic-private")
			response, err := doControlPlaneRequest(client, request)
			var redirect *controlPlaneRedirectError
			if response != nil || err == nil {
				t.Fatal("TLS/redirect failure returned success")
			}
			if mode == "custom-ca" || mode == "exact-pin" {
				if !errors.As(err, &redirect) || requests.Load() != before+1 {
					t.Fatal("configured trust did not reach the no-redirect boundary")
				}
				assertPrivateRedirectDiagnostic(t, err.Error(), "synthetic-private")
			} else if errors.As(err, &redirect) || requests.Load() != before {
				t.Fatal("invalid trust reached the authenticated HTTP handler")
			}
		})
	}
}
