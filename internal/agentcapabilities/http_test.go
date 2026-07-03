package agentcapabilities

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	// A GET capability with non-path arguments must forward them as URL query
	// parameters (not a body, not dropped) so filter-style tools like
	// get_fleet_context can receive agent-supplied filters through the MCP
	// tools/call path.
	cap := Capability{
		Name:   FleetContextCapabilityName,
		Method: http.MethodGet,
		Path:   FleetContextCapabilityPath,
	}
	req, projected, err := BuildCapabilityHTTPRequest(context.Background(), "http://pulse.local/", "token", cap, map[string]any{
		"hasFindings": "true",
		"technology":  "docker",
	})
	if err != nil {
		t.Fatalf("BuildCapabilityHTTPRequest: %v", err)
	}
	if req.Method != http.MethodGet {
		t.Fatalf("method = %q, want GET", req.Method)
	}
	if projected.HasBody {
		t.Fatalf("GET projection must not produce a body; projected = %+v", projected)
	}
	if projected.Query == nil {
		t.Fatal("projected.Query must be populated for a GET with filter args")
	}
	if got := projected.Query.Get("hasFindings"); got != "true" {
		t.Errorf("query hasFindings = %q, want true", got)
	}
	if got := projected.Query.Get("technology"); got != "docker" {
		t.Errorf("query technology = %q, want docker", got)
	}
	// The query must land on the request URL encoded.
	if req.URL.RawQuery == "" {
		t.Fatalf("RawQuery empty; url = %q", req.URL.String())
	}
	gotFindings := req.URL.Query().Get("hasFindings")
	gotTech := req.URL.Query().Get("technology")
	if gotFindings != "true" || gotTech != "docker" {
		t.Errorf("url query = hasFindings=%q technology=%q", gotFindings, gotTech)
	}
	if req.Body != nil {
		t.Fatalf("GET request must not carry a body; got %+v", req.Body)
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
