package remoteconfig

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

const redirectPrivacyToken = "synthetic-remote-credential-not-for-diagnostics"
const redirectPrivacySecret = "synthetic-remote-destination-not-for-diagnostics"

func assertRemoteRedirectPrivate(t *testing.T, err error, logs string, status int) {
	t.Helper()
	if err == nil {
		t.Error("redirect returned success")
		return
	}
	for _, marker := range []string{redirectPrivacyToken, redirectPrivacySecret} {
		if strings.Contains(err.Error(), marker) {
			t.Error("returned error disclosed synthetic destination/credential")
		}
		if strings.Contains(logs, marker) {
			t.Error("remote-config log disclosed synthetic destination/credential")
		}
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d redirect", status)) || !strings.Contains(err.Error(), "final Pulse URL") {
		t.Error("redirect diagnostic lost numeric refusal or final-URL guidance")
	}
}

func signedRemoteRedirectRecovery(t *testing.T) []byte {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pub))
	t.Setenv("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED", "true")
	commands := false
	settings := map[string]interface{}{"interval": "60s"}
	desired, err := BuildDesiredConfigMetadata(&commands, settings)
	if err != nil {
		t.Fatal(err)
	}
	issued := time.Now().UTC()
	payload := SignedConfigPayload{AgentID: "synthetic-bound-agent", IssuedAt: issued, ExpiresAt: issued.Add(time.Hour), CommandsEnabled: &commands, Settings: settings}
	signature, err := SignConfigPayload(payload, priv)
	if err != nil {
		t.Fatal(err)
	}
	response := Response{Success: true, AgentID: payload.AgentID}
	response.Config.CommandsEnabled, response.Config.Settings = &commands, settings
	response.Config.IssuedAt, response.Config.ExpiresAt, response.Config.Signature = payload.IssuedAt, payload.ExpiresAt, signature
	response.Config.DesiredConfig = &desired
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestRemoteConfigRedirectDiagnosticsStayPrivate(t *testing.T) {
	recovery := signedRemoteRedirectRecovery(t)
	for _, stage := range []string{"lookup", "config"} {
		for _, status := range []int{301, 302, 303, 307, 308} {
			for _, shape := range []string{"absolute", "same-origin", "scheme-relative", "malformed-host", "malformed-path"} {
				t.Run(fmt.Sprintf("%s/%d/%s", stage, status, shape), func(t *testing.T) {
					var destinationHits, unexpected, sourceRequests atomic.Int32
					var restored atomic.Bool
					destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						destinationHits.Add(1)
						w.WriteHeader(http.StatusNoContent)
					}))
					defer destination.Close()
					var location string
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						sourceRequests.Add(1)
						if r.Header.Get("Authorization") != "Bearer "+redirectPrivacyToken || r.Header.Get("X-API-Token") != redirectPrivacyToken || r.Method != http.MethodGet {
							t.Error("remote-config request changed credential or method")
						}
						path := r.URL.Path
						if path != agentLookupPath && path != "/api/agents/agent/synthetic-bound-agent/config" {
							unexpected.Add(1)
							http.NotFound(w, r)
							return
						}
						if path == agentLookupPath && r.URL.Query().Get("hostname") != "synthetic-host" {
							t.Error("lookup changed hostname")
						}
						if !restored.Load() && ((stage == "lookup" && path == agentLookupPath) || (stage == "config" && path != agentLookupPath)) {
							w.Header().Set("Location", location)
							w.WriteHeader(status)
							_, _ = io.WriteString(w, redirectPrivacySecret)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						if path == agentLookupPath {
							_, _ = io.WriteString(w, `{"success":true,"agent":{"id":"synthetic-bound-agent"}}`)
							return
						}
						_, _ = w.Write(recovery)
					}))
					defer server.Close()
					switch shape {
					case "absolute":
						location = strings.Replace(destination.URL, "://", "://"+redirectPrivacySecret+":"+redirectPrivacyToken+"@", 1) + "/" + redirectPrivacySecret + "?token=" + redirectPrivacyToken + "#" + redirectPrivacySecret
					case "same-origin":
						location = server.URL + "/" + redirectPrivacySecret + "?token=" + redirectPrivacyToken
					case "scheme-relative":
						location = strings.TrimPrefix(destination.URL, "http:") + "/" + redirectPrivacySecret + "?token=" + redirectPrivacyToken
					case "malformed-host":
						location = "http://[" + redirectPrivacySecret + "?token=" + redirectPrivacyToken
					case "malformed-path":
						location = destination.URL + "/%zz/" + redirectPrivacySecret + "?token=" + redirectPrivacyToken
					}
					var logs bytes.Buffer
					client := New(Config{PulseURL: server.URL, APIToken: redirectPrivacyToken, AgentID: "synthetic-saved-agent", Hostname: "synthetic-host", Logger: zerolog.New(&logs)})
					defer client.Close()
					settings, commands, err := client.Fetch(context.Background())
					if settings != nil || commands != nil {
						t.Error("refused response released configuration")
					}
					assertRemoteRedirectPrivate(t, err, logs.String(), status)
					if destinationHits.Load() != 0 || unexpected.Load() != 0 {
						t.Error("redirect reached a destination or unexpected endpoint")
					}
					want := int32(1)
					if stage == "config" {
						want = 2
					}
					if sourceRequests.Load() != want {
						t.Error("refusal retried or advanced to another config read")
					}
					restored.Store(true)
					settings, commands, err = client.Fetch(context.Background())
					if err != nil || commands == nil || *commands || settings["interval"] != "60s" {
						t.Error("same-credential signed config recovery failed")
					}
					if client.cfg.APIToken != redirectPrivacyToken || client.cfg.AgentID != "synthetic-saved-agent" {
						t.Error("refusal/recovery replaced configured authority")
					}
					if sourceRequests.Load() != want+2 || destinationHits.Load() != 0 || unexpected.Load() != 0 {
						t.Error("recovery changed request routing")
					}
				})
			}
		}
	}
}

type remoteRedirectRoundTripper func(*http.Request) (*http.Response, error)

func (f remoteRedirectRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type remoteRedirectBody struct{ reads, closes int }

func (b *remoteRedirectBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New(redirectPrivacySecret)
}
func (b *remoteRedirectBody) Close() error { b.closes++; return errors.New(redirectPrivacySecret) }

func TestRemoteConfigRedirectClosesBeforeParsing(t *testing.T) {
	for _, stage := range []string{"lookup", "config"} {
		for _, status := range []int{301, 302, 303, 307, 308} {
			t.Run(fmt.Sprintf("%s/%d", stage, status), func(t *testing.T) {
				body := &remoteRedirectBody{}
				var logs bytes.Buffer
				cfg := Config{PulseURL: "https://pulse.example", APIToken: redirectPrivacyToken, AgentID: "synthetic-saved-agent", Logger: zerolog.New(&logs)}
				if stage == "lookup" {
					cfg.Hostname = "synthetic-host"
				}
				client := New(cfg)
				defer client.Close()
				client.httpClient = &http.Client{Transport: remoteRedirectRoundTripper(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: status, Status: redirectPrivacySecret, Header: http.Header{"Location": {"http://[" + redirectPrivacySecret}}, Body: body}, nil
				})}
				_, _, err := client.Fetch(context.Background())
				assertRemoteRedirectPrivate(t, err, logs.String(), status)
				if body.reads != 0 || body.closes != 1 {
					t.Errorf("rejected body reads/closes = %d/%d; want 0/1", body.reads, body.closes)
				}
			})
		}
	}
}

func TestRemoteConfigRedirectRetainsTLS(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Location", "http://["+redirectPrivacySecret)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(server.Certificate().Raw)
	for _, stage := range []string{"lookup", "config"} {
		for _, trust := range []string{"custom-ca", "exact-pin", "untrusted", "wrong-pin"} {
			t.Run(stage+"/"+trust, func(t *testing.T) {
				var logs bytes.Buffer
				cfg := Config{PulseURL: server.URL, APIToken: redirectPrivacyToken, AgentID: "synthetic-saved-agent", Logger: zerolog.New(&logs)}
				if stage == "lookup" {
					cfg.Hostname = "synthetic-host"
				}
				switch trust {
				case "custom-ca":
					cfg.CACertPath = ca
				case "exact-pin":
					cfg.InsecureSkipVerify = true
					cfg.ServerFingerprint = hex.EncodeToString(digest[:])
				case "wrong-pin":
					cfg.InsecureSkipVerify = true
					cfg.ServerFingerprint = strings.Repeat("00", 32)
				}
				client := New(cfg)
				defer client.Close()
				before := requests.Load()
				settings, commands, err := client.Fetch(context.Background())
				if err == nil || settings != nil || commands != nil {
					t.Fatal("invalid trust/redirect returned config")
				}
				if trust == "custom-ca" || trust == "exact-pin" {
					assertRemoteRedirectPrivate(t, err, logs.String(), http.StatusTemporaryRedirect)
					if requests.Load() != before+1 {
						t.Error("configured trust did not reach refusal boundary")
					}
				} else if requests.Load() != before {
					t.Error("invalid trust reached authenticated HTTP handler")
				}
			})
		}
	}
}

func TestRemoteConfigRedirectKeepsContext(t *testing.T) {
	for _, stage := range []string{"lookup", "config"} {
		for _, canceled := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/canceled=%t", stage, canceled), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
				defer cancel()
				cfg := Config{PulseURL: "https://pulse.example", APIToken: redirectPrivacyToken, AgentID: "synthetic-saved-agent", Logger: zerolog.Nop()}
				if stage == "lookup" {
					cfg.Hostname = "synthetic-host"
				}
				client := New(cfg)
				defer client.Close()
				client.httpClient = &http.Client{Transport: remoteRedirectRoundTripper(func(r *http.Request) (*http.Response, error) {
					if canceled {
						cancel()
					}
					<-r.Context().Done()
					return nil, r.Context().Err()
				})}
				_, _, err := client.Fetch(ctx)
				want := context.DeadlineExceeded
				if canceled {
					want = context.Canceled
				}
				if !errors.Is(err, want) {
					t.Error("config read lost cancellation/deadline identity")
				}
			})
		}
	}
}

func TestRemoteConfigRedirectLeavesNoLocationHandling(t *testing.T) {
	for _, stage := range []string{"lookup", "config"} {
		for _, status := range []int{301, 302, 303, 307, 308} {
			t.Run(fmt.Sprintf("%s/%d", stage, status), func(t *testing.T) {
				cfg := Config{PulseURL: "https://pulse.example", APIToken: redirectPrivacyToken, AgentID: "synthetic-saved-agent", Logger: zerolog.Nop()}
				if stage == "lookup" {
					cfg.Hostname = "synthetic-host"
				}
				client := New(cfg)
				defer client.Close()
				client.httpClient = &http.Client{Transport: remoteRedirectRoundTripper(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
				})}
				_, _, err := client.Fetch(context.Background())
				if err == nil || !strings.Contains(err.Error(), "responded with status") || strings.Contains(err.Error(), "final Pulse URL") {
					t.Error("no-Location response changed ordinary status handling")
				}
			})
		}
	}
}
