package pulsecli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// These are the registered commands, not a second implementation of their
// HTTP requests. All credentials and action contents are synthetic.
type cliRedirectCommand struct {
	name   string
	args   []string
	method string
	path   string
}

func cliRedirectCommands() []cliRedirectCommand {
	return []cliRedirectCommand{
		{"fleet", []string{"fleet", "connections"}, http.MethodGet, "/api/connections"},
		{"plan", []string{"actions", "plan", "--request-id", "synthetic-request", "--resource-id", "vm:42", "--capability", "restart", "--reason", "synthetic-private-action"}, http.MethodPost, "/api/actions/plan"},
		{"decide", []string{"actions", "decide", "--action-id", "synthetic-action", "--outcome", "approved", "--reason", "synthetic-private-action"}, http.MethodPost, "/api/actions/synthetic-action/decision"},
		{"execute", []string{"actions", "execute", "--action-id", "synthetic-action", "--reason", "synthetic-private-action"}, http.MethodPost, "/api/actions/synthetic-action/execute"},
		{"capabilities", []string{"actions", "capabilities", "--resource-id", "vm:42"}, http.MethodGet, "/api/resources/vm:42/facets"},
		{"audit", []string{"actions", "audit"}, http.MethodGet, "/api/audit/actions"},
		{"events", []string{"actions", "events", "--action-id", "synthetic-action"}, http.MethodGet, "/api/audit/actions/synthetic-action/events"},
	}
}

func executeCLIRedirectCommand(t *testing.T, command cliRedirectCommand, baseURL string, client HTTPDoer) (string, string, error) {
	t.Helper()
	getenv := func(key string) string {
		switch key {
		case "PULSE_API_URL":
			return baseURL
		case "PULSE_API_TOKEN":
			return "synthetic-cli-token"
		default:
			return ""
		}
	}
	root := NewRootCommand(CommandSpec{Use: "pulse"}, RuntimeSpec{}, CommandDeps{
		Actions: &ActionsDeps{HTTPClient: client, Getenv: getenv},
		Fleet:   &FleetDeps{HTTPClient: client, Getenv: getenv},
	})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(command.args)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := root.ExecuteContext(ctx)
	return stdout.String(), stderr.String(), err
}

type cliRedirectRequest struct {
	path   string
	method string
	authOK bool
	body   string
}

func recordCLIRedirectRequest(r *http.Request) cliRedirectRequest {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	return cliRedirectRequest{
		path: r.URL.Path, method: r.Method,
		authOK: r.Header.Get("Authorization") == "Bearer synthetic-cli-token",
		body:   string(body),
	}
}

func TestCLIAPICommandsKeepRedirectsWithinOrigin(t *testing.T) {
	for _, injected := range []bool{false, true} {
		for _, command := range cliRedirectCommands() {
			for _, status := range []int{301, 302, 303, 307, 308} {
				t.Run(fmt.Sprintf("injected-%t/%s/%d", injected, command.name, status), func(t *testing.T) {
					foreign := make(chan cliRedirectRequest, 2)
					sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						foreign <- recordCLIRedirectRequest(r)
						w.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(w, `{}`)
					}))
					defer sink.Close()
					sourceRequests := make(chan cliRedirectRequest, 2)
					source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						sourceRequests <- recordCLIRedirectRequest(r)
						http.Redirect(w, r, sink.URL+"/synthetic-private-destination?value=synthetic-private-query", status)
					}))
					defer source.Close()
					var client HTTPDoer
					if injected {
						client = source.Client()
					}
					stdout, stderr, err := executeCLIRedirectCommand(t, command, source.URL, client)
					if len(sourceRequests) != 1 {
						t.Fatalf("source requests = %d, want one", len(sourceRequests))
					}
					original := <-sourceRequests
					if original.path != command.path || original.method != command.method || !original.authOK {
						t.Fatalf("original request path=%s method=%s bearer=%t", original.path, original.method, original.authOK)
					}
					if command.method == http.MethodPost && !strings.Contains(original.body, "synthetic-private-action") {
						t.Fatal("action request did not exercise the replayable body")
					}
					foreignCount := len(foreign)
					foreignBearer, replayedBody := false, false
					for len(foreign) > 0 {
						request := <-foreign
						foreignBearer = foreignBearer || request.authOK
						replayedBody = replayedBody || strings.Contains(request.body, "synthetic-private-action")
					}
					// Counts/booleans make the pre-repair failure useful evidence
					// without printing bearer values or request bodies.
					t.Logf("foreign requests=%d bearer=%t replayed-body=%t", foreignCount, foreignBearer, replayedBody)
					if err == nil || !strings.Contains(err.Error(), "set the final Pulse base URL") || foreignCount != 0 {
						t.Errorf("redirect must refuse before a foreign request; err=%v", err)
					}
					combined := stdout + stderr + fmt.Sprint(err)
					for _, private := range []string{"synthetic-cli-token", "synthetic-private-destination", "synthetic-private-query", "synthetic-private-action"} {
						if strings.Contains(combined, private) {
							t.Errorf("command output disclosed %s", private)
						}
					}
					if stdout != "" {
						t.Error("refused redirect printed a successful API result")
					}
				})
			}
		}
	}
}

func TestCLIAPICommandsPreserveSameOriginTLSRedirects(t *testing.T) {
	for _, command := range cliRedirectCommands() {
		for _, status := range []int{307, 308} {
			t.Run(fmt.Sprintf("%s/%d", command.name, status), func(t *testing.T) {
				requests := make(chan cliRedirectRequest, 3)
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests <- recordCLIRedirectRequest(r)
					if r.URL.Path == command.path {
						http.Redirect(w, r, "/proxy"+command.path, status)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					body := `{}`
					if command.name == "fleet" {
						body = `{"connections":[]}`
					}
					_, _ = io.WriteString(w, body)
				}))
				defer server.Close()
				client := server.Client() // Keep this caller's actual TLS trust.
				client.Timeout = 3 * time.Second
				stdout, _, err := executeCLIRedirectCommand(t, command, server.URL, client)
				if err != nil {
					t.Fatalf("same-origin TLS command failed: %v", err)
				}
				if stdout == "" || len(requests) != 2 {
					t.Fatalf("output=%t requests=%d, want JSON and two requests", stdout != "", len(requests))
				}
				original, redirected := <-requests, <-requests
				if redirected.path != "/proxy"+command.path || redirected.method != command.method || !original.authOK || !redirected.authOK || redirected.body != original.body {
					t.Fatalf("same-origin request changed: path=%s method=%s original-bearer=%t redirected-bearer=%t equal-body=%t", redirected.path, redirected.method, original.authOK, redirected.authOK, redirected.body == original.body)
				}
				if client.CheckRedirect != nil || client.Timeout != 3*time.Second {
					t.Fatal("caller HTTP client policy was mutated")
				}
			})
		}
	}
}
