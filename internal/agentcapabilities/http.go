package agentcapabilities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

const (
	AgentCapabilitiesPath           = "/api/agent/capabilities"
	AgentEventsPath                 = "/api/agent/events"
	AgentWorkflowPromptActivityPath = "/api/agent/workflow-prompt-activity"
	AgentAPITokenHeader             = "X-API-Token"
	AgentSurfaceHeader              = "X-Pulse-Agent-Surface"
	AgentSurfacePulseMCP            = "pulse_mcp"

	maxManifestResponseBodyBytes   int64 = 1 << 20  // 1 MiB
	maxCapabilityResponseBodyBytes int64 = 16 << 20 // 16 MiB
)

// HTTPDoer is the shared minimum interface for agent-surface HTTP clients.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// HTTPCallResponse is the raw response from invoking a manifest capability.
type HTTPCallResponse struct {
	Method     string
	Path       string
	StatusCode int
	Header     http.Header
	Body       []byte
}

// OK reports whether the upstream Pulse response was a 2xx.
func (r HTTPCallResponse) OK() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// StableErrorEnvelope decodes the response body as the shared agent error
// envelope when the upstream used that shape.
func (r HTTPCallResponse) StableErrorEnvelope() (ErrorEnvelope, bool) {
	return DecodeErrorEnvelope(r.Body)
}

// FailureError returns the stable branchable error when a response is non-2xx.
func (r HTTPCallResponse) FailureError() error {
	if r.OK() {
		return nil
	}
	method := strings.TrimSpace(r.Method)
	if method == "" {
		method = "HTTP"
	}
	path := strings.TrimSpace(r.Path)
	if path == "" {
		path = "agent capability"
	}
	if env, ok := r.StableErrorEnvelope(); ok {
		return fmt.Errorf("%s %s: %d %s (%s)", method, path, r.StatusCode, env.Error, env.Message)
	}
	body := strings.TrimSpace(string(r.Body))
	if body == "" {
		return fmt.Errorf("%s %s: %d", method, path, r.StatusCode)
	}
	return fmt.Errorf("%s %s: %d %s", method, path, r.StatusCode, body)
}

// AgentURL joins a Pulse base URL and an agent-surface path.
func AgentURL(baseURL, path string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("base URL is required")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("agent path is required")
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return "", fmt.Errorf("agent path must be relative: %s", path)
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	fullURL := baseURL + path
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return "", fmt.Errorf("parse agent URL %q: %w", fullURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("agent base URL must include scheme and host: %s", baseURL)
	}
	return parsed.String(), nil
}

// NewAgentHTTPRequest builds an agent-surface HTTP request with the shared API
// token header and JSON content-type convention.
func NewAgentHTTPRequest(ctx context.Context, method, baseURL, path, token string, body io.Reader) (*http.Request, error) {
	fullURL, err := AgentURL(baseURL, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set(AgentAPITokenHeader, token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// FetchManifest pulls the public agent capabilities manifest from a Pulse
// instance using the shared discovery path.
func FetchManifest(ctx context.Context, client HTTPDoer, baseURL string) (*Manifest, error) {
	req, err := NewAgentHTTPRequest(ctx, http.MethodGet, baseURL, AgentCapabilitiesPath, "", nil)
	if err != nil {
		return nil, err
	}
	resp, err := DoAgentHTTPRequest(client, req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", AgentCapabilitiesPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", AgentCapabilitiesPath, resp.StatusCode)
	}
	body, err := readBoundedResponseBody(resp, maxManifestResponseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("GET %s response: %w", AgentCapabilitiesPath, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &manifest, nil
}

// BuildCapabilityHTTPRequest projects a manifest capability invocation into an
// authenticated Pulse HTTP request.
func BuildCapabilityHTTPRequest(ctx context.Context, baseURL, token string, cap Capability, args map[string]any) (*http.Request, ProjectedCall, error) {
	projected, err := ProjectCapabilityCall(cap, args)
	if err != nil {
		return nil, ProjectedCall{}, err
	}
	var body io.Reader
	if projected.HasBody {
		body = bytes.NewReader(projected.Body)
	}
	req, err := NewAgentHTTPRequest(ctx, cap.Method, baseURL, projected.Path, token, body)
	if err != nil {
		return nil, ProjectedCall{}, err
	}
	// Attach forwarded query parameters (GET/DELETE filter arguments). The
	// projection layer owns which arguments become query params; here we only
	// encode them onto the request URL.
	if len(projected.Query) > 0 {
		req.URL.RawQuery = projected.Query.Encode()
	}
	return req, projected, nil
}

// CallCapabilityHTTP executes a manifest capability through the shared
// discovery/projection/auth header path. Callers own the surface-specific
// wrapping of the raw response.
func CallCapabilityHTTP(ctx context.Context, client HTTPDoer, baseURL, token string, cap Capability, args map[string]any) (HTTPCallResponse, error) {
	req, projected, err := BuildCapabilityHTTPRequest(ctx, baseURL, token, cap, args)
	if err != nil {
		return HTTPCallResponse{}, err
	}
	resp, err := DoAgentHTTPRequest(client, req)
	if err != nil {
		return HTTPCallResponse{}, fmt.Errorf("%s %s: %w", cap.Method, projected.Path, err)
	}
	defer resp.Body.Close()
	body, err := readBoundedResponseBody(resp, maxCapabilityResponseBodyBytes)
	if err != nil {
		return HTTPCallResponse{}, fmt.Errorf("read %s %s response: %w", cap.Method, projected.Path, err)
	}
	return HTTPCallResponse{
		Method:     cap.Method,
		Path:       projected.Path,
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       body,
	}, nil
}

func readBoundedResponseBody(resp *http.Response, limit int64) ([]byte, error) {
	if err := securityutil.LimitResponseBody(resp, limit); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

// CallCapabilityHTTPByName resolves and executes a named manifest capability
// through the shared discovery/projection/auth header path.
func CallCapabilityHTTPByName(ctx context.Context, client HTTPDoer, baseURL, token string, capabilities []Capability, name string, args map[string]any) (HTTPCallResponse, error) {
	cap, err := ResolveCapability(capabilities, name)
	if err != nil {
		return HTTPCallResponse{}, err
	}
	return CallCapabilityHTTP(ctx, client, baseURL, token, cap, args)
}

// CallRequestResponseCapabilityHTTPByName resolves and executes a named
// manifest capability only when it is eligible for request/response external
// tool adapters. Streaming capabilities are consumed through the SSE helpers.
func CallRequestResponseCapabilityHTTPByName(ctx context.Context, client HTTPDoer, baseURL, token string, capabilities []Capability, name string, args map[string]any) (HTTPCallResponse, error) {
	cap, err := ResolveRequestResponseCapability(capabilities, name)
	if err != nil {
		return HTTPCallResponse{}, err
	}
	return CallCapabilityHTTP(ctx, client, baseURL, token, cap, args)
}

// CallRequestResponseCapabilityHTTPBodyByName resolves and executes a named
// request/response manifest capability, requiring a 2xx upstream response
// before returning the raw body. Reference clients and probes use this when
// they want normal HTTP failure handling through the shared stable error
// envelope instead of MCP's isError result projection.
func CallRequestResponseCapabilityHTTPBodyByName(ctx context.Context, client HTTPDoer, baseURL, token string, capabilities []Capability, name string, args map[string]any) ([]byte, error) {
	resp, err := CallRequestResponseCapabilityHTTPByName(ctx, client, baseURL, token, capabilities, name, args)
	if err != nil {
		return nil, err
	}
	if err := resp.FailureError(); err != nil {
		return nil, err
	}
	return append([]byte(nil), resp.Body...), nil
}

var errAgentRedirectOrigin = errors.New("agent HTTP redirect left the configured origin; set the final Pulse base URL")

// DoAgentHTTPRequest keeps standard HTTP clients within the original request's
// origin. Go's default redirect rules do not protect our custom API-token
// header or replayable request bodies. Copy the client so callers retain their
// transport, timeout, cookie jar and stricter redirect policy without mutation.
// Custom HTTPDoers own their transport policy (including any redirect handling).
func DoAgentHTTPRequest(client HTTPDoer, req *http.Request) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	standard, ok := client.(*http.Client)
	if !ok {
		return client.Do(req)
	}
	guarded := *standard
	previousRedirectPolicy := guarded.CheckRedirect
	guarded.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) == 0 || !sameAgentHTTPOrigin(via[0].URL, next.URL) {
			return errAgentRedirectOrigin
		}
		if previousRedirectPolicy != nil {
			if err := previousRedirectPolicy(next, via); err != nil {
				return err
			}
			// A caller's callback may edit the proposed request.
			if !sameAgentHTTPOrigin(via[0].URL, next.URL) {
				return errAgentRedirectOrigin
			}
		} else if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	resp, err := guarded.Do(req)
	if errors.Is(err, errAgentRedirectOrigin) {
		// net/http wraps redirect errors with the destination URL. Do not expose
		// its userinfo, path or query through MCP results or adapter logs.
		return nil, errAgentRedirectOrigin
	}
	return resp, err
}

func sameAgentHTTPOrigin(from, to *url.URL) bool {
	if from == nil || to == nil || from.Hostname() == "" || to.Hostname() == "" || from.Opaque != "" || to.Opaque != "" {
		return false
	}
	if !strings.EqualFold(from.Scheme, to.Scheme) || !strings.EqualFold(from.Hostname(), to.Hostname()) {
		return false
	}
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if strings.EqualFold(u.Scheme, "https") {
			return "443"
		}
		return "80"
	}
	return (strings.EqualFold(from.Scheme, "http") || strings.EqualFold(from.Scheme, "https")) &&
		port(from) == port(to) && from.User.String() == to.User.String()
}
