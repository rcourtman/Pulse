package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestBearerAPITokenScopesDenyReadWriteAndExecRoutes(t *testing.T) {
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, "bearer-read-token-123.12345678", []string{config.ScopeMonitoringRead}, nil),
		newTokenRecord(t, "bearer-write-token-123.12345678", []string{config.ScopeSettingsRead}, nil),
		newTokenRecord(t, "bearer-exec-token-123.12345678", []string{config.ScopeAIChat}, nil),
	)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		token         string
		wantScopeHint string
	}{
		{
			name:          "read route missing settings read",
			method:        http.MethodGet,
			path:          "/api/security/tokens",
			token:         "bearer-read-token-123.12345678",
			wantScopeHint: config.ScopeSettingsRead,
		},
		{
			name:          "write route missing settings write",
			method:        http.MethodPost,
			path:          "/api/security/tokens",
			body:          `{"name":"test","scopes":["monitoring:read"]}`,
			token:         "bearer-write-token-123.12345678",
			wantScopeHint: config.ScopeSettingsWrite,
		},
		{
			name:          "exec route missing ai execute",
			method:        http.MethodPost,
			path:          "/api/ai/execute",
			body:          `{}`,
			token:         "bearer-exec-token-123.12345678",
			wantScopeHint: config.ScopeAIExecute,
		},
	}

	client := &http.Client{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, server.URL+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Authorization", "Bearer "+tc.token)

			res, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			if res.StatusCode != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", res.StatusCode, string(body))
			}
			if !strings.Contains(string(body), tc.wantScopeHint) {
				t.Fatalf("expected response to mention %q, got %q", tc.wantScopeHint, string(body))
			}
		})
	}
}
