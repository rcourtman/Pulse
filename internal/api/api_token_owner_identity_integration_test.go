package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestOwnerBoundAPITokenInheritsBoundUserRBACPrincipal(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&mockAuthorizerFn{
		fn: func(ctx context.Context, action string, resource string) (bool, error) {
			return auth.GetUser(ctx) == "alice" && action == auth.ActionAdmin && resource == auth.ResourceUsers, nil
		},
	})
	defer auth.SetAuthorizer(prevAuthorizer)

	ownerBound := newTokenRecord(t, "owner-bound-token-123.12345678", []string{config.ScopeSettingsRead}, map[string]string{
		apiTokenMetadataOwnerUserID: "alice",
	})
	detached := newTokenRecord(t, "detached-token-123.12345678", []string{config.ScopeSettingsRead}, nil)

	cfg := newTestConfigWithTokens(t, ownerBound, detached)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	requestList := func(rawToken string) (int, string) {
		t.Helper()

		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/security/tokens", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+rawToken)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		return res.StatusCode, string(body)
	}

	status, body := requestList("owner-bound-token-123.12345678")
	if status != http.StatusOK {
		t.Fatalf("expected owner-bound token to pass RBAC route, got %d: %s", status, body)
	}

	var payload struct {
		Tokens []apiTokenDTO `json:"tokens"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode success payload: %v", err)
	}
	if len(payload.Tokens) != 2 {
		t.Fatalf("expected 2 tokens in list response, got %d", len(payload.Tokens))
	}

	status, body = requestList("detached-token-123.12345678")
	if status != http.StatusForbidden {
		t.Fatalf("expected detached token to fail RBAC route, got %d: %s", status, body)
	}
}
