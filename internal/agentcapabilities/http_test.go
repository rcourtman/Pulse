package agentcapabilities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// These are two independent loopback servers, not real Pulse installations.
// A redirect must not widen the destination of either the token or write body.
func TestAgentHTTPRedirectsRejectOtherOrigins(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			t.Run(fmt.Sprintf("%d/%s", status, method), func(t *testing.T) {
				var destinationRequests, tokenRequests, bodyRequests atomic.Int32
				destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					destinationRequests.Add(1)
					if r.Header.Get(AgentAPITokenHeader) != "" {
						tokenRequests.Add(1)
					}
					body, _ := io.ReadAll(r.Body)
					if len(body) > 0 {
						bodyRequests.Add(1)
					}
					_, _ = io.WriteString(w, `{"unexpected":true}`)
				}))
				defer destination.Close()
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, destination.URL+"/elsewhere?private=redirect-query-marker", status)
				}))
				defer origin.Close()

				args := map[string]any{}
				if method == http.MethodPut {
					args["body"] = map[string]any{"note": "private-write-marker"}
				}
				response, err := CallCapabilityHTTP(context.Background(), origin.Client(), origin.URL, "synthetic-private-token", Capability{Method: method, Path: "/api/example"}, args)
				if err == nil {
					t.Errorf("off-origin redirect returned success: status %d", response.StatusCode)
				} else if strings.Contains(err.Error(), "redirect-query-marker") {
					t.Error("redirect rejection exposed the destination query")
				}
				if got := destinationRequests.Load(); got != 0 {
					t.Errorf("off-origin destination received %d requests (%d with token, %d with body)", got, tokenRequests.Load(), bodyRequests.Load())
				}
			})
		}
	}
}

type redirectTestTransport func(*http.Request) (*http.Response, error)

func (f redirectTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAgentHTTPRedirectOriginPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
		allowed        bool
	}{
		{"relative", "https://pulse.example", "/accepted", true},
		{"same-origin", "https://pulse.example", "https://pulse.example/accepted", true},
		{"host-case", "https://pulse.example", "https://PULSE.example/accepted", true},
		{"default-https-port", "https://pulse.example", "https://pulse.example:443/accepted", true},
		{"default-http-port", "http://pulse.example:80", "http://pulse.example/accepted", true},
		{"same-nondefault-port", "https://pulse.example:9443", "https://pulse.example:9443/accepted", true},
		{"other-host", "https://pulse.example", "https://elsewhere.example/accepted", false},
		{"subdomain", "https://pulse.example", "https://child.pulse.example/accepted", false},
		{"other-port", "https://pulse.example", "https://pulse.example:9443/accepted", false},
		{"downgrade", "https://pulse.example", "http://pulse.example/accepted", false},
		{"upgrade", "http://pulse.example", "https://pulse.example/accepted", false},
		{"trailing-dot", "https://pulse.example", "https://pulse.example./accepted", false},
		{"new-userinfo", "https://pulse.example", "https://name:redirect-password-marker@pulse.example/accepted", false},
		{"retained-userinfo", "https://name:original@pulse.example", "/accepted", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			client := &http.Client{Transport: redirectTestTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return &http.Response{StatusCode: http.StatusTemporaryRedirect, Header: http.Header{"Location": {tc.to}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"version":"v1"}`)), Request: r}, nil
			})}
			_, err := FetchManifest(context.Background(), client, tc.from)
			if (err == nil) != tc.allowed {
				t.Errorf("redirect allowed = %v, want %v", err == nil, tc.allowed)
			}
			wantRequests := 1
			if tc.allowed {
				wantRequests = 2
			}
			if requests != wantRequests {
				t.Errorf("transport requests = %d, want %d", requests, wantRequests)
			}
			if err != nil && strings.Contains(err.Error(), "redirect-password-marker") {
				t.Error("redirect rejection exposed userinfo")
			}
		})
	}
}

func TestAgentHTTPRedirectsPreserveClientPolicy(t *testing.T) {
	for _, stop := range []error{errors.New("caller redirect refusal"), http.ErrUseLastResponse} {
		t.Run(stop.Error(), func(t *testing.T) {
			var calls atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/accepted", http.StatusTemporaryRedirect)
			}))
			defer origin.Close()
			client := origin.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { calls.Add(1); return stop }
			_, err := FetchManifest(context.Background(), client, origin.URL)
			if err == nil || (stop != http.ErrUseLastResponse && !errors.Is(err, stop)) {
				t.Fatalf("caller policy error was lost: %v", err)
			}
			if calls.Load() != 1 {
				t.Fatalf("caller policy calls = %d, want 1", calls.Load())
			}
			// The guard must not replace policy on a client it does not own.
			if got := client.CheckRedirect(nil, nil); got != stop {
				t.Fatal("caller client was mutated")
			}
		})
	}
}

func TestAgentHTTPRedirectsRetainDefaultLimit(t *testing.T) {
	var requests atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/loop", http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	_, err := FetchManifest(context.Background(), origin.Client(), origin.URL)
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("default redirect limit lost: %v", err)
	}
	if requests.Load() != 10 {
		t.Fatalf("redirect requests = %d, want 10", requests.Load())
	}
}

func TestAgentHTTPRedirectCallbackCannotWidenOrigin(t *testing.T) {
	var foreignRequests, callbackCalls atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignRequests.Add(1) }))
	defer foreign.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/accepted", http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	client := origin.Client()
	client.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		callbackCalls.Add(1)
		next.URL, _ = url.Parse(foreign.URL + "/elsewhere")
		return nil
	}
	_, err := FetchManifest(context.Background(), client, origin.URL)
	if !errors.Is(err, errAgentRedirectOrigin) || callbackCalls.Load() != 1 || foreignRequests.Load() != 0 {
		t.Fatalf("callback escaped boundary: err=%v callbacks=%d foreign=%d", err, callbackCalls.Load(), foreignRequests.Load())
	}
}

func TestAgentHTTPRedirectsPreserveTLSAndCancellation(t *testing.T) {
	var foreignRequests atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignRequests.Add(1) }))
	defer foreign.Close()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/accepted":
			_, _ = io.WriteString(w, `{"version":"v1"}`)
		case "/downgrade":
			http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
		default:
			http.Redirect(w, r, "/accepted", http.StatusTemporaryRedirect)
		}
	}))
	defer origin.Close()
	client := origin.Client() // Includes this fixture's trusted TLS certificate.
	if _, err := FetchManifest(context.Background(), client, origin.URL); err != nil {
		t.Fatalf("configured TLS transport lost: %v", err)
	}
	response, err := CallCapabilityHTTP(context.Background(), client, origin.URL, "synthetic-token", Capability{Method: http.MethodGet, Path: "/downgrade"}, nil)
	if !errors.Is(err, errAgentRedirectOrigin) || response.StatusCode != 0 || foreignRequests.Load() != 0 {
		t.Fatalf("TLS downgrade escaped boundary: err=%v status=%d foreign=%d", err, response.StatusCode, foreignRequests.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FetchManifest(ctx, client, origin.URL); !errors.Is(err, context.Canceled) {
		t.Fatalf("request cancellation lost: %v", err)
	}
	if client.CheckRedirect != nil {
		t.Fatal("caller TLS client was mutated")
	}
}

func TestFetchManifestUsesSharedDiscoveryPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != AgentCapabilitiesPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, AgentCapabilitiesPath)
		}
		if token := r.Header.Get(AgentAPITokenHeader); token != "" {
			t.Fatalf("manifest discovery must not send API token, got %q", token)
		}
		_ = json.NewEncoder(w).Encode(Manifest{
			Version: "v1",
			Capabilities: []Capability{{
				Name:   "get_fleet_context",
				Method: http.MethodGet,
				Path:   "/api/agent/fleet-context",
			}},
		})
	}))
	defer server.Close()

	manifest, err := FetchManifest(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	if manifest.Version != "v1" || len(manifest.Capabilities) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestFetchManifestRejectsDeclaredOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(maxManifestResponseBodyBytes+1))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := FetchManifest(context.Background(), server.Client(), server.URL)
	assertResponseSizeLimitError(t, err, maxManifestResponseBodyBytes)
}

func TestFetchManifestRejectsUndeclaredOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeUndeclaredOversizedResponse(w, maxManifestResponseBodyBytes)
	}))
	defer server.Close()

	_, err := FetchManifest(context.Background(), server.Client(), server.URL)
	assertResponseSizeLimitError(t, err, maxManifestResponseBodyBytes)
}

func TestBuildCapabilityHTTPRequestProjectsPathBodyAndAuth(t *testing.T) {
	cap := Capability{
		Name:   SetOperatorStateCapabilityName,
		Method: http.MethodPost,
		Path:   "/api/agent/operator-state/{resourceId}",
	}
	req, projected, err := BuildCapabilityHTTPRequest(context.Background(), "http://pulse.local/", "test-token", cap, map[string]any{
		ResourceIDArgumentName: "vm:101/console",
		"note":                 "maintenance",
	})
	if err != nil {
		t.Fatalf("BuildCapabilityHTTPRequest: %v", err)
	}

	if req.Method != http.MethodPost {
		t.Fatalf("method = %q, want POST", req.Method)
	}
	if req.URL.String() != "http://pulse.local/api/agent/operator-state/vm%3A101%2Fconsole" {
		t.Fatalf("url = %q", req.URL.String())
	}
	if got := req.Header.Get(AgentAPITokenHeader); got != "test-token" {
		t.Fatalf("%s = %q, want test-token", AgentAPITokenHeader, got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if projected.Path != "/api/agent/operator-state/vm%3A101%2Fconsole" || !projected.HasBody {
		t.Fatalf("projected = %+v", projected)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(body), "resourceId") {
		t.Fatalf("body must not duplicate path args, got %s", body)
	}
	if !strings.Contains(string(body), `"note":"maintenance"`) {
		t.Fatalf("body = %s", body)
	}
}

func TestBuildCapabilityHTTPRequestForwardsGETFiltersAsQuery(t *testing.T) {
	cap := Capability{Name: FleetContextCapabilityName, Method: http.MethodGet, Path: FleetContextCapabilityPath}
	for _, tc := range []struct {
		name  string
		value any
		want  string
	}{
		{"boolean_true", true, "true"},
		{"boolean_false", false, "false"},
		{"legacy_string", "true", "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, projected, err := BuildCapabilityHTTPRequest(context.Background(), "http://pulse.local/", "fixture-token", cap, map[string]any{
				"hasFindings":       tc.value,
				"technology":        "qemu & lxc",
				ApprovalArgumentKey: "private-approval",
			})
			if err != nil {
				t.Fatal(err)
			}
			if req.Method != http.MethodGet || req.Body != nil || projected.HasBody {
				t.Fatalf("GET must retain its method and no body: %s %+v", req.Method, projected)
			}
			wantQuery := "hasFindings=" + tc.want + "&technology=qemu+%26+lxc"
			if req.URL.RawQuery != wantQuery || projected.Query.Encode() != wantQuery {
				t.Errorf("encoded query = %q, projection = %q, want %q", req.URL.RawQuery, projected.Query.Encode(), wantQuery)
			}
			if req.URL.Query().Get("technology") != "qemu & lxc" {
				t.Error("query encoding changed the string filter")
			}
			if req.Header.Get(AgentAPITokenHeader) != "fixture-token" {
				t.Error("filter projection lost authentication header")
			}
		})
	}
}

func TestBuildCapabilityHTTPRequestGETWithoutArgsHasNoQuery(t *testing.T) {
	// A GET with no filter arguments must not synthesize a query string —
	// backward compatibility for the resources/list adapter and any caller
	// that wants the unfiltered result.
	cap := Capability{
		Name:   FleetContextCapabilityName,
		Method: http.MethodGet,
		Path:   FleetContextCapabilityPath,
	}
	req, projected, err := BuildCapabilityHTTPRequest(context.Background(), "http://pulse.local/", "token", cap, map[string]any{})
	if err != nil {
		t.Fatalf("BuildCapabilityHTTPRequest: %v", err)
	}
	if projected.Query != nil {
		t.Fatalf("projected.Query = %+v, want nil for argless GET", projected.Query)
	}
	if req.URL.RawQuery != "" {
		t.Fatalf("RawQuery = %q, want empty", req.URL.RawQuery)
	}
}

func TestCallCapabilityHTTPExecutesManifestProjection(t *testing.T) {
	var got struct {
		Method      string
		Path        string
		Token       string
		ContentType string
		Body        string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.EscapedPath()
		got.Token = r.Header.Get(AgentAPITokenHeader)
		got.ContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		got.Body = string(body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	resp, err := CallCapabilityHTTP(context.Background(), server.Client(), server.URL, "shared-token", Capability{
		Name:   PlanActionCapabilityName,
		Method: http.MethodPost,
		Path:   "/api/actions/{actionId}/plan",
	}, map[string]any{
		ActionIDArgumentName: "act:123",
		"body": map[string]any{
			"reason": "operator requested",
		},
	})
	if err != nil {
		t.Fatalf("CallCapabilityHTTP: %v", err)
	}
	if !resp.OK() || resp.StatusCode != http.StatusAccepted || string(resp.Body) != `{"ok":true}` {
		t.Fatalf("response = %+v body=%s", resp, resp.Body)
	}
	if got.Method != http.MethodPost || got.Path != "/api/actions/act%3A123/plan" {
		t.Fatalf("request method/path = %s %s", got.Method, got.Path)
	}
	if got.Token != "shared-token" {
		t.Fatalf("token = %q", got.Token)
	}
	if got.ContentType != "application/json" {
		t.Fatalf("content type = %q", got.ContentType)
	}
	if got.Body != `{"reason":"operator requested"}` {
		t.Fatalf("body = %s", got.Body)
	}
}

func TestCallCapabilityHTTPRejectsDeclaredOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(maxCapabilityResponseBodyBytes+1))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := CallCapabilityHTTP(context.Background(), server.Client(), server.URL, "token", Capability{
		Name:   FleetContextCapabilityName,
		Method: http.MethodGet,
		Path:   FleetContextCapabilityPath,
	}, nil)
	assertResponseSizeLimitError(t, err, maxCapabilityResponseBodyBytes)
}

func TestCallCapabilityHTTPRejectsUndeclaredOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeUndeclaredOversizedResponse(w, maxCapabilityResponseBodyBytes)
	}))
	defer server.Close()

	_, err := CallCapabilityHTTP(context.Background(), server.Client(), server.URL, "token", Capability{
		Name:   FleetContextCapabilityName,
		Method: http.MethodGet,
		Path:   FleetContextCapabilityPath,
	}, nil)
	assertResponseSizeLimitError(t, err, maxCapabilityResponseBodyBytes)
}

func assertResponseSizeLimitError(t *testing.T, err error, limit int64) {
	t.Helper()
	want := fmt.Sprintf("response body exceeds %d bytes", limit)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func writeUndeclaredOversizedResponse(w http.ResponseWriter, limit int64) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	prefix := []byte(`{"padding":"`)
	if _, err := w.Write(prefix); err != nil {
		return
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	chunk := strings.Repeat("x", 32*1024)
	remaining := limit + 1 - int64(len(prefix))
	for remaining > 0 {
		writeSize := int64(len(chunk))
		if writeSize > remaining {
			writeSize = remaining
		}
		if _, err := io.WriteString(w, chunk[:int(writeSize)]); err != nil {
			return
		}
		remaining -= writeSize
	}
}

func TestCallCapabilityHTTPByNameResolvesAndExecutesManifestProjection(t *testing.T) {
	var got struct {
		Method string
		Path   string
		Token  string
		Body   string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.EscapedPath()
		got.Token = r.Header.Get(AgentAPITokenHeader)
		body, _ := io.ReadAll(r.Body)
		got.Body = string(body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	resp, err := CallCapabilityHTTPByName(context.Background(), server.Client(), server.URL, "shared-token", []Capability{{
		Name:   ResourceContextCapabilityName,
		Method: http.MethodPost,
		Path:   "/api/agent/resource-context/{resourceId}/note",
	}}, ResourceContextCapabilityName, map[string]any{
		"resourceId": "vm:101/console",
		"note":       "focus first",
	})
	if err != nil {
		t.Fatalf("CallCapabilityHTTPByName: %v", err)
	}
	if !resp.OK() || resp.StatusCode != http.StatusAccepted || string(resp.Body) != `{"ok":true}` {
		t.Fatalf("response = %+v body=%s", resp, resp.Body)
	}
	if got.Method != http.MethodPost || got.Path != "/api/agent/resource-context/vm%3A101%2Fconsole/note" {
		t.Fatalf("request method/path = %s %s", got.Method, got.Path)
	}
	if got.Token != "shared-token" {
		t.Fatalf("token = %q", got.Token)
	}
	if strings.Contains(got.Body, "resourceId") || got.Body != `{"note":"focus first"}` {
		t.Fatalf("body = %s", got.Body)
	}
}

func TestCallCapabilityHTTPByNameReturnsTypedLookupError(t *testing.T) {
	_, err := CallCapabilityHTTPByName(context.Background(), nil, "http://pulse.local", "token", []Capability{{
		Name:   FleetContextCapabilityName,
		Method: http.MethodGet,
		Path:   "/api/agent/fleet-context",
	}}, "missing_capability", nil)
	if err == nil {
		t.Fatal("expected missing capability error")
	}
	var lookupErr CapabilityLookupError
	if !errors.As(err, &lookupErr) {
		t.Fatalf("error type = %T %v, want CapabilityLookupError", err, err)
	}
	if lookupErr.Name != "missing_capability" || err.Error() != "unknown capability: missing_capability" {
		t.Fatalf("lookup error = %#v %q", lookupErr, err.Error())
	}
}

func TestCallRequestResponseCapabilityHTTPByNameRejectsStreamingCapabilities(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Fatalf("streaming capability must not be invoked through request/response HTTP helper")
	}))
	defer server.Close()

	_, err := CallRequestResponseCapabilityHTTPByName(context.Background(), server.Client(), server.URL, "shared-token", []Capability{{
		Name:   EventSubscriptionCapabilityName,
		Method: http.MethodGet,
		Path:   AgentEventsPath,
	}}, EventSubscriptionCapabilityName, nil)
	if err == nil {
		t.Fatal("expected streaming capability to be rejected")
	}
	var lookupErr CapabilityLookupError
	if !errors.As(err, &lookupErr) || lookupErr.Name != EventSubscriptionCapabilityName {
		t.Fatalf("error = %T %[1]v, want CapabilityLookupError for %s", err, EventSubscriptionCapabilityName)
	}
	if called {
		t.Fatal("streaming capability reached upstream HTTP server")
	}
}

func TestCallRequestResponseCapabilityHTTPBodyByNameReturnsSuccessfulBody(t *testing.T) {
	var got struct {
		Method string
		Path   string
		Token  string
		Body   string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.EscapedPath()
		got.Token = r.Header.Get(AgentAPITokenHeader)
		body, _ := io.ReadAll(r.Body)
		got.Body = string(body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"resource":"ok"}`))
	}))
	defer server.Close()

	body, err := CallRequestResponseCapabilityHTTPBodyByName(
		context.Background(),
		server.Client(),
		server.URL,
		"body-token",
		[]Capability{{
			Name:   ResourceContextCapabilityName,
			Method: http.MethodPost,
			Path:   "/api/agent/resource-context/{resourceId}/focus",
		}},
		ResourceContextCapabilityName,
		map[string]any{
			"resourceId": "vm:101/console",
			"reason":     "critical finding",
		},
	)
	if err != nil {
		t.Fatalf("CallRequestResponseCapabilityHTTPBodyByName: %v", err)
	}
	if string(body) != `{"resource":"ok"}` {
		t.Fatalf("body = %s", body)
	}
	if got.Method != http.MethodPost || got.Path != "/api/agent/resource-context/vm%3A101%2Fconsole/focus" {
		t.Fatalf("request method/path = %s %s", got.Method, got.Path)
	}
	if got.Token != "body-token" {
		t.Fatalf("token = %q", got.Token)
	}
	if strings.Contains(got.Body, "resourceId") || got.Body != `{"reason":"critical finding"}` {
		t.Fatalf("body = %s", got.Body)
	}
}

func TestCallRequestResponseCapabilityHTTPBodyByNameReturnsStableFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"scope_required","message":"monitoring:read is required"}`))
	}))
	defer server.Close()

	_, err := CallRequestResponseCapabilityHTTPBodyByName(
		context.Background(),
		server.Client(),
		server.URL,
		"body-token",
		[]Capability{{
			Name:   FleetContextCapabilityName,
			Method: http.MethodGet,
			Path:   "/api/agent/fleet-context",
		}},
		FleetContextCapabilityName,
		nil,
	)
	if err == nil {
		t.Fatal("expected stable non-2xx error")
	}
	if got := err.Error(); got != "GET /api/agent/fleet-context: 403 scope_required (monitoring:read is required)" {
		t.Fatalf("error = %q", got)
	}
}

func TestHTTPCallResponseFailureErrorUsesStableEnvelope(t *testing.T) {
	resp := HTTPCallResponse{
		Method:     http.MethodGet,
		Path:       "/api/agent/resource-context/vm%3A101",
		StatusCode: http.StatusForbidden,
		Body:       []byte(`{"error":"scope_required","message":"monitoring:read is required"}`),
	}
	if resp.OK() {
		t.Fatal("forbidden response must not be OK")
	}
	if got := resp.FailureError().Error(); got != "GET /api/agent/resource-context/vm%3A101: 403 scope_required (monitoring:read is required)" {
		t.Fatalf("failure error = %q", got)
	}
}
