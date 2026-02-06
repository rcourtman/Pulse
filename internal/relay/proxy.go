package relay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	// maxProxyBodySize is the maximum request/response body size before truncation.
	// Must fit inside a 64KB relay frame after base64 encoding (~33% expansion) and
	// JSON wrapper overhead (~500 bytes). 47KB * 4/3 ≈ 62.7KB + overhead ≈ 63.2KB < 64KB.
	maxProxyBodySize = 47 * 1024 // 47KB

	// proxyRequestTimeout is the per-request timeout for proxied HTTP calls.
	proxyRequestTimeout = 30 * time.Second
)

// ProxyRequest is the JSON payload inside a DATA frame from the app to the instance.
type ProxyRequest struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"` // base64-encoded
}

// ProxyResponse is the JSON payload inside a DATA frame from the instance to the app.
type ProxyResponse struct {
	ID      string            `json:"id"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"` // base64-encoded
}

// HTTPProxy proxies DATA frame payloads to the local Pulse API.
type HTTPProxy struct {
	localAddr string
	client    *http.Client
	logger    zerolog.Logger
}

// NewHTTPProxy creates a proxy that forwards requests to the given local address.
func NewHTTPProxy(localAddr string, logger zerolog.Logger) *HTTPProxy {
	return &HTTPProxy{
		localAddr: localAddr,
		client: &http.Client{
			Timeout: proxyRequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
	}
}

// HandleRequest processes a DATA frame payload as an HTTP request and returns the response payload.
// The apiToken is the validated token from the channel's CHANNEL_OPEN, injected as X-API-Token.
func (p *HTTPProxy) HandleRequest(payload []byte, apiToken string) ([]byte, error) {
	var req ProxyRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return p.errorResponse("", http.StatusBadRequest, "invalid request payload"), nil
	}

	if req.ID == "" || req.Method == "" || req.Path == "" {
		return p.errorResponse(req.ID, http.StatusBadRequest, "missing required fields (id, method, path)"), nil
	}

	// Ensure path starts with /
	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}

	// Decode body if present
	var bodyReader io.Reader
	if req.Body != "" {
		bodyBytes, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return p.errorResponse(req.ID, http.StatusBadRequest, "invalid base64 body"), nil
		}
		if len(bodyBytes) > maxProxyBodySize {
			return p.errorResponse(req.ID, http.StatusRequestEntityTooLarge, "request body exceeds 47KB limit"), nil
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	url := fmt.Sprintf("http://%s%s", p.localAddr, req.Path)
	httpReq, err := http.NewRequest(req.Method, url, bodyReader)
	if err != nil {
		return p.errorResponse(req.ID, http.StatusInternalServerError, "failed to create request"), nil
	}

	// Allowlist: only forward safe, content-describing headers.
	// Everything else is stripped to prevent auth-context leakage
	// (X-Proxy-Secret, X-Forwarded-*, Forwarded, Cookie, Authorization, etc.)
	for k, v := range req.Headers {
		if allowedProxyHeader(k) {
			httpReq.Header.Set(k, v)
		}
	}

	// Inject the API token for Pulse auth middleware
	httpReq.Header.Set("X-API-Token", apiToken)

	p.logger.Debug().
		Str("request_id", req.ID).
		Str("method", req.Method).
		Str("path", req.Path).
		Msg("Proxying relay request to local API")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		p.logger.Warn().Err(err).Str("request_id", req.ID).Msg("Local API request failed")
		return p.errorResponse(req.ID, http.StatusBadGateway, "local API request failed"), nil
	}
	defer resp.Body.Close()

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, maxProxyBodySize+1)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return p.errorResponse(req.ID, http.StatusBadGateway, "failed to read response body"), nil
	}

	if len(respBody) > maxProxyBodySize {
		return p.errorResponse(req.ID, http.StatusRequestEntityTooLarge, "response body exceeds 47KB limit"), nil
	}

	// Build response headers (pick relevant ones)
	respHeaders := make(map[string]string)
	for _, key := range []string{"Content-Type", "X-Request-Id", "Cache-Control"} {
		if v := resp.Header.Get(key); v != "" {
			respHeaders[key] = v
		}
	}

	proxyResp := ProxyResponse{
		ID:      req.ID,
		Status:  resp.StatusCode,
		Headers: respHeaders,
	}
	if len(respBody) > 0 {
		proxyResp.Body = base64.StdEncoding.EncodeToString(respBody)
	}

	data, err := json.Marshal(proxyResp)
	if err != nil {
		return p.errorResponse(req.ID, http.StatusInternalServerError, "failed to marshal response"), nil
	}
	return data, nil
}

// allowedProxyHeaders is the set of headers that may be forwarded from relay
// requests to the local Pulse API. All other headers are stripped to prevent
// auth-context leakage (X-Proxy-Secret, X-Forwarded-*, etc.).
var allowedProxyHeaders = map[string]bool{
	"accept":            true,
	"accept-encoding":   true,
	"accept-language":   true,
	"content-type":      true,
	"content-length":    true,
	"if-match":          true,
	"if-none-match":     true,
	"if-modified-since": true,
}

func allowedProxyHeader(name string) bool {
	return allowedProxyHeaders[strings.ToLower(name)]
}

func (p *HTTPProxy) errorResponse(requestID string, status int, message string) []byte {
	resp := ProxyResponse{
		ID:     requestID,
		Status: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
	body, _ := json.Marshal(map[string]string{"error": message})
	resp.Body = base64.StdEncoding.EncodeToString(body)
	data, _ := json.Marshal(resp)
	return data
}
