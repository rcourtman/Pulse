package hostagent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

// Exercise actual runtime consumers, not just CheckRedirect: net/http can put
// Location in its own error wrapper, including when the URL cannot be parsed.
func TestControlPlaneRedirectDiagnosticsStayPrivate(t *testing.T) {
	const token = "synthetic-redirect-credential"
	const secret = "synthetic-redirect-private"
	for _, entry := range []string{"primary", "observer", "enrollment", "runner-cancel", "runner-revoke", "runner-activate"} {
		for _, status := range []int{301, 302, 303, 307, 308} {
			for _, shape := range []string{"off-origin", "same-origin", "relative", "userinfo", "malformed"} {
				t.Run(fmt.Sprintf("%s/%d/%s", entry, status, shape), func(t *testing.T) {
					var targetHits, sourceHits atomic.Int32
					var recovered atomic.Bool
					target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						targetHits.Add(1)
						w.WriteHeader(http.StatusNoContent)
					}))
					defer target.Close()
					location := ""
					source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						sourceHits.Add(1)
						if strings.Contains(r.URL.RequestURI(), secret) {
							t.Error("same-origin redirect was followed")
						}
						if r.Header.Get("Authorization") != "Bearer "+token {
							t.Error("source credential changed")
						}
						if recovered.Load() {
							if strings.HasPrefix(entry, "runner-") {
								w.WriteHeader(http.StatusNoContent)
							} else {
								_, _ = io.WriteString(w, `{"success":true}`)
							}
							return
						}
						w.Header().Set("Location", location)
						w.WriteHeader(status)
						_, _ = io.WriteString(w, secret+"-response-body")
					}))
					defer source.Close()
					switch shape {
					case "off-origin":
						location = target.URL + "/" + secret + "?token=" + token + "#" + secret
					case "same-origin":
						location = source.URL + "/" + secret + "?token=" + token
					case "relative":
						location = "/" + secret + "?token=" + token
					case "userinfo":
						location = strings.Replace(target.URL, "http://", "http://"+secret+":"+token+"@", 1)
					case "malformed":
						location = "http://[" + secret + "?token=" + token
					}
					client, err := newAgentHTTPClient("", false, "")
					if err != nil {
						t.Fatal(err)
					}
					defer client.CloseIdleConnections()
					var logs bytes.Buffer
					a := &Agent{
						cfg:     Config{APIToken: token, AgentID: "synthetic-saved-id", CommandAuthorityProfile: CommandAuthorityMonitoringOnly},
						agentID: "synthetic-saved-id", logger: zerolog.New(&logs),
						httpClient: client, trimmedPulseURL: source.URL,
						reportBuffer: utils.New[agentshost.Report](4),
					}
					ctx := context.Background()
					report := agentshost.Report{Agent: agentshost.AgentInfo{ID: a.agentID}}
					var call func() error
					switch entry {
					case "primary":
						call = func() error { return a.sendReport(ctx, report) }
					case "observer":
						call = func() error { return a.sendReportToDestination(ctx, report, source.URL, token, client, false) }
					case "enrollment":
						call = func() error { _, err := a.enroll(ctx); return err }
					case "runner-cancel":
						call = func() error {
							return CancelPendingActionRunnerCredential(ctx, ActionRunnerCredentialLifecycleConfig{PulseURL: source.URL, APIToken: token})
						}
					case "runner-revoke":
						call = func() error {
							return RevokeActionRunnerCredential(ctx, ActionRunnerCredentialLifecycleConfig{PulseURL: source.URL, APIToken: token}, a.agentID, "synthetic-host")
						}
					case "runner-activate":
						c := &CommandClient{actionRunnerOnly: true, pulseURL: source.URL, apiToken: token, agentID: a.agentID}
						call = func() error { return c.activateActionRunnerCredential(ctx) }
					}
					err = call()
					if err == nil {
						t.Fatal("redirect was accepted")
					}
					assertPrivateRedirectDiagnostic(t, err.Error(), token, secret)
					if sourceHits.Load() != 1 || targetHits.Load() != 0 {
						t.Fatal("redirect sent a second request")
					}
					if entry == "primary" {
						_ = a.deliverPrimaryReport(ctx, report)
						if a.reportBuffer.Len() != 1 {
							t.Fatal("redirect changed primary buffering")
						}
					} else if entry == "observer" {
						observer := &observerReporter{target: ObserverTarget{Name: "synthetic-observer", PulseURL: source.URL, APIToken: token}, httpClient: client, reportBuffer: utils.New[agentshost.Report](4)}
						a.deliverObserverReport(ctx, observer, report)
						if observer.reportBuffer.Len() != 1 || !a.reportBuffer.IsEmpty() {
							t.Fatal("redirect changed destination-isolated buffering")
						}
					}
					if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), token) {
						t.Error("report buffering log disclosed redirect data")
					}
					if entry != "enrollment" {
						recovered.Store(true)
						if err := call(); err != nil {
							t.Fatal("same-credential ordinary request did not recover")
						}
					}
					if a.cfg.APIToken != token || a.agentID != "synthetic-saved-id" || a.cfg.EnableCommands || a.cfg.CommandAuthorityProfile != CommandAuthorityMonitoringOnly || targetHits.Load() != 0 {
						t.Fatal("redirect changed saved authority or reached its destination")
					}
				})
			}
		}
	}
}

func assertPrivateRedirectDiagnostic(t *testing.T, diagnostic string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(diagnostic, secret) {
			t.Error("redirect diagnostic disclosed untrusted Location data")
			return
		}
	}
	if !strings.Contains(diagnostic, "redirect") || !strings.Contains(diagnostic, "final Pulse URL") {
		t.Error("redirect diagnostic lost safe recovery guidance")
	}
}
