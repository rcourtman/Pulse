package providers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGenerateOAuthSession(t *testing.T) {
	s, err := GenerateOAuthSession("http://localhost/callback")
	if err != nil {
		t.Fatalf("GenerateOAuthSession: %v", err)
	}
	if s.RedirectURI != "http://localhost/callback" {
		t.Fatalf("RedirectURI = %q", s.RedirectURI)
	}
	if s.State == "" || s.CodeVerifier == "" {
		t.Fatalf("expected non-empty state and verifier: %+v", s)
	}
	if strings.Contains(s.State, "=") || strings.Contains(s.CodeVerifier, "=") {
		t.Fatalf("expected raw base64url encoding (no '='): %+v", s)
	}
	if time.Since(s.CreatedAt) > 2*time.Second {
		t.Fatalf("CreatedAt too old: %v", s.CreatedAt)
	}
}

func TestGetAuthorizationURL_IncludesExpectedParamsAndChallenge(t *testing.T) {
	session := &OAuthSession{
		State:        "state_123",
		CodeVerifier: "verifier_456",
	}

	u, err := url.Parse(GetAuthorizationURL(session))
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	q := u.Query()
	if q.Get("code") != "true" {
		t.Fatalf("code = %q", q.Get("code"))
	}
	if q.Get("client_id") != claudeCodeClientID {
		t.Fatalf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("response_type = %q", q.Get("response_type"))
	}
	if q.Get("redirect_uri") != "https://console.anthropic.com/oauth/code/callback" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if q.Get("scope") != oauthScopes {
		t.Fatalf("scope = %q", q.Get("scope"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("code_challenge_method = %q", q.Get("code_challenge_method"))
	}
	if q.Get("state") != "state_123" {
		t.Fatalf("state = %q", q.Get("state"))
	}

	h := sha256.Sum256([]byte(session.CodeVerifier))
	wantChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	if q.Get("code_challenge") != wantChallenge {
		t.Fatalf("code_challenge = %q, want %q", q.Get("code_challenge"), wantChallenge)
	}
}

func TestExchangeCodeForTokens_Success(t *testing.T) {
	oldTokenURL := oauthTokenURL
	oldClient := oauthHTTPClient
	t.Cleanup(func() {
		oauthTokenURL = oldTokenURL
		oauthHTTPClient = oldClient
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/oauth/token" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["grant_type"] != "authorization_code" {
			t.Fatalf("grant_type = %v", payload["grant_type"])
		}
		if payload["code"] != "code_abc" {
			t.Fatalf("code = %v", payload["code"])
		}
		if payload["client_id"] != claudeCodeClientID {
			t.Fatalf("client_id = %v", payload["client_id"])
		}
		if payload["code_verifier"] != "verifier" {
			t.Fatalf("code_verifier = %v", payload["code_verifier"])
		}
		if payload["state"] != "state" {
			t.Fatalf("state = %v", payload["state"])
		}

		_ = json.NewEncoder(w).Encode(OAuthTokens{
			AccessToken:  "access_1",
			RefreshToken: "refresh_1",
			TokenType:    "bearer",
			ExpiresIn:    3600,
			Scope:        oauthScopes,
		})
	}))
	defer server.Close()

	oauthTokenURL = server.URL + "/v1/oauth/token"
	oauthHTTPClient = server.Client()

	now := time.Now()
	tokens, err := ExchangeCodeForTokens(context.Background(), "code_abc", &OAuthSession{
		State:        "state",
		CodeVerifier: "verifier",
	})
	if err != nil {
		t.Fatalf("ExchangeCodeForTokens: %v", err)
	}
	if tokens.AccessToken != "access_1" || tokens.RefreshToken != "refresh_1" {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
	if tokens.ExpiresAt.Before(now) || tokens.ExpiresAt.After(now.Add(2*time.Hour)) {
		t.Fatalf("ExpiresAt = %v, now = %v", tokens.ExpiresAt, now)
	}
}

func TestExchangeCodeForTokens_ErrorStatusIncludesBody(t *testing.T) {
	oldTokenURL := oauthTokenURL
	oldClient := oauthHTTPClient
	t.Cleanup(func() {
		oauthTokenURL = oldTokenURL
		oauthHTTPClient = oldClient
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	oauthTokenURL = server.URL
	oauthHTTPClient = server.Client()

	_, err := ExchangeCodeForTokens(context.Background(), "code", &OAuthSession{State: "s", CodeVerifier: "v"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 400") || !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAPIKeyFromOAuth_SuccessAndEmptyKey(t *testing.T) {
	oldAPIKeyURL := oauthAPIKeyURL
	oldClient := oauthHTTPClient
	t.Cleanup(func() {
		oauthAPIKeyURL = oldAPIKeyURL
		oauthHTTPClient = oldClient
	})

	var call int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if call == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{"raw_key": "sk-ant-123"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"raw_key": ""})
	}))
	defer server.Close()

	oauthAPIKeyURL = server.URL
	oauthHTTPClient = server.Client()

	key, err := CreateAPIKeyFromOAuth(context.Background(), "access")
	if err != nil {
		t.Fatalf("CreateAPIKeyFromOAuth: %v", err)
	}
	if key != "sk-ant-123" {
		t.Fatalf("key = %q", key)
	}

	_, err = CreateAPIKeyFromOAuth(context.Background(), "access")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRefreshAccessToken_KeepsOriginalRefreshTokenWhenOmitted(t *testing.T) {
	oldTokenURL := oauthTokenURL
	oldClient := oauthHTTPClient
	t.Cleanup(func() {
		oauthTokenURL = oldTokenURL
		oauthHTTPClient = oldClient
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["grant_type"] != "refresh_token" {
			t.Fatalf("grant_type = %v", payload["grant_type"])
		}
		if payload["refresh_token"] != "refresh_old" {
			t.Fatalf("refresh_token = %v", payload["refresh_token"])
		}
		_ = json.NewEncoder(w).Encode(OAuthTokens{
			AccessToken: "access_new",
			// RefreshToken intentionally omitted
			TokenType: "bearer",
			ExpiresIn: 3600,
		})
	}))
	defer server.Close()

	oauthTokenURL = server.URL
	oauthHTTPClient = server.Client()

	tokens, err := RefreshAccessToken(context.Background(), "refresh_old")
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if tokens.AccessToken != "access_new" || tokens.RefreshToken != "refresh_old" {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
}

func TestAnthropicOAuthClient_forceRefreshToken_UpdatesAndCallsCallback(t *testing.T) {
	oldTokenURL := oauthTokenURL
	oldClient := oauthHTTPClient
	t.Cleanup(func() {
		oauthTokenURL = oldTokenURL
		oauthHTTPClient = oldClient
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(OAuthTokens{
			AccessToken:  "access_new",
			RefreshToken: "refresh_new",
			TokenType:    "bearer",
			ExpiresIn:    3600,
		})
	}))
	defer server.Close()

	oauthTokenURL = server.URL
	oauthHTTPClient = server.Client()

	client := NewAnthropicOAuthClient("access_old", "refresh_old", time.Now().Add(-time.Minute), "claude-3", 0)

	var cbTokens *OAuthTokens
	client.SetTokenRefreshCallback(func(tokens *OAuthTokens) { cbTokens = tokens })

	if err := client.forceRefreshToken(context.Background()); err != nil {
		t.Fatalf("forceRefreshToken: %v", err)
	}
	if client.accessToken != "access_new" || client.refreshToken != "refresh_new" {
		t.Fatalf("unexpected client tokens: access=%q refresh=%q", client.accessToken, client.refreshToken)
	}
	if cbTokens == nil || cbTokens.AccessToken != "access_new" {
		t.Fatalf("expected callback with new tokens, got: %+v", cbTokens)
	}
}

func TestAnthropicOAuthClient_Chat_RefreshesOn401AndRetriesImmediately(t *testing.T) {
	oldTokenURL := oauthTokenURL
	oldClient := oauthHTTPClient
	t.Cleanup(func() {
		oauthTokenURL = oldTokenURL
		oauthHTTPClient = oldClient
	})

	var messageCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth/token":
			_ = json.NewEncoder(w).Encode(OAuthTokens{
				AccessToken:  "access_new",
				RefreshToken: "refresh_new",
				TokenType:    "bearer",
				ExpiresIn:    3600,
			})
		case "/v1/messages":
			if r.URL.Query().Get("beta") != "true" {
				t.Fatalf("expected beta=true query, got %q", r.URL.RawQuery)
			}
			if r.Header.Get("anthropic-version") != anthropicAPIVersion {
				t.Fatalf("anthropic-version = %q", r.Header.Get("anthropic-version"))
			}
			if r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
				t.Fatalf("anthropic-beta = %q", r.Header.Get("anthropic-beta"))
			}
			if r.Header.Get("x-app") != "cli" {
				t.Fatalf("x-app = %q", r.Header.Get("x-app"))
			}

			messageCalls++
			if messageCalls == 1 {
				if r.Header.Get("Authorization") != "Bearer access_old" {
					t.Fatalf("Authorization (first) = %q", r.Header.Get("Authorization"))
				}
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"message":"unauthorized"}}`))
				return
			}
			if r.Header.Get("Authorization") != "Bearer access_new" {
				t.Fatalf("Authorization (retry) = %q", r.Header.Get("Authorization"))
			}

			var got anthropicRequest
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if got.Model != "claude-3" {
				t.Fatalf("Model = %q", got.Model)
			}
			if len(got.Messages) != 1 || got.Messages[0].Role != "user" || got.Messages[0].Content != "Hi" {
				t.Fatalf("unexpected messages: %+v", got.Messages)
			}

			_ = json.NewEncoder(w).Encode(anthropicResponse{
				ID:         "msg_123",
				Type:       "message",
				Role:       "assistant",
				Model:      "claude-3",
				StopReason: "end_turn",
				Content:    []anthropicContent{{Type: "text", Text: "ok"}},
				Usage:      anthropicUsage{InputTokens: 1, OutputTokens: 2},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oauthTokenURL = server.URL + "/v1/oauth/token"
	oauthHTTPClient = server.Client()

	client := NewAnthropicOAuthClientWithBaseURL(
		"access_old",
		"refresh_old",
		time.Now().Add(10*time.Minute), // valid token, so refresh is driven by 401
		"claude-3",
		server.URL+"/v1/messages?beta=true",
		0,
	)
	client.client = server.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	out, err := client.Chat(ctx, ChatRequest{Messages: []Message{{Role: "user", Content: "Hi"}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if out.Content != "ok" || out.InputTokens != 1 || out.OutputTokens != 2 {
		t.Fatalf("unexpected response: %+v", out)
	}
	if messageCalls != 2 {
		t.Fatalf("messageCalls = %d, want 2", messageCalls)
	}
}

func TestAnthropicOAuthClient_ListModels_UsesConfiguredHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "claude-3", "display_name": "Claude 3", "created_at": "2024-01-01T00:00:00Z"},
			},
		})
	}))
	defer server.Close()

	client := NewAnthropicOAuthClientWithBaseURL(
		"access",
		"refresh",
		time.Now().Add(10*time.Minute),
		"claude-3",
		server.URL+"/v1/messages?beta=true",
		0,
	)
	client.client = server.Client()

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].ID != "claude-3" || models[0].Name != "Claude 3" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

