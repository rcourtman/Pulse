package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"golang.org/x/oauth2"
)

func TestOIDCServiceMatches(t *testing.T) {
	svc := &OIDCService{
		snapshot: oidcSnapshot{
			issuer:       "https://issuer.example.com",
			clientID:     "client-id",
			clientSecret: "client-secret",
			redirectURL:  "https://pulse.example.com/callback",
			scopes:       []string{"openid", "email"},
			caBundle:     "",
			caBundleHash: "",
		},
	}

	cfg := &config.OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://issuer.example.com",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://pulse.example.com/callback",
		Scopes:       []string{"openid", "email"},
		CABundle:     "",
	}

	if !svc.Matches(cfg) {
		t.Fatalf("expected config to match")
	}

	cfg.ClientID = "other"
	if svc.Matches(cfg) {
		t.Fatalf("expected config mismatch")
	}
}

func TestOIDCServiceStateEntryAndConsume(t *testing.T) {
	svc := &OIDCService{stateStore: newOIDCStateStore()}

	state, entry, err := svc.newStateEntry("", "/return")
	if err != nil {
		t.Fatalf("newStateEntry error: %v", err)
	}
	if state == "" || entry == nil {
		t.Fatalf("expected state and entry")
	}
	if entry.ReturnTo != "/return" {
		t.Fatalf("expected returnTo /return, got %q", entry.ReturnTo)
	}

	consumed, ok := svc.consumeState(state)
	if !ok || consumed == nil {
		t.Fatalf("expected to consume state")
	}
	if _, ok := svc.consumeState(state); ok {
		t.Fatalf("expected state to be removed")
	}
}

func TestOIDCServiceAuthCodeURLIncludesPKCE(t *testing.T) {
	svc := &OIDCService{
		oauth2Cfg: &oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://issuer.example.com/auth"}, ClientID: "client"},
	}
	entry := &oidcStateEntry{Nonce: "nonce", CodeChallenge: "challenge"}
	url := svc.authCodeURL("state", entry)
	if url == "" {
		t.Fatalf("expected auth url")
	}
	if !strings.Contains(url, "code_challenge=challenge") {
		t.Fatalf("expected code_challenge in url, got %q", url)
	}
}

func TestOIDCServiceExchangeCode(t *testing.T) {
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Form.Get("code_verifier") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"access","token_type":"Bearer","refresh_token":"refresh","expires_in":3600}`)
	}))
	defer server.Close()

	svc := &OIDCService{
		oauth2Cfg: &oauth2.Config{
			ClientID:     "client",
			ClientSecret: "secret",
			Endpoint:     oauth2.Endpoint{TokenURL: server.URL},
			RedirectURL:  "https://pulse.example.com/callback",
		},
		httpClient: server.Client(),
	}

	entry := &oidcStateEntry{CodeVerifier: "verifier"}
	token, err := svc.exchangeCode(context.Background(), "code", entry)
	if err != nil {
		t.Fatalf("exchangeCode error: %v", err)
	}
	if token.AccessToken != "access" {
		t.Fatalf("expected access token, got %q", token.AccessToken)
	}
}

func TestHashCABundle(t *testing.T) {
	if hash, err := hashCABundle(""); err != nil || hash != "" {
		t.Fatalf("expected empty hash, got %q err=%v", hash, err)
	}

	file := t.TempDir() + "/ca.pem"
	data := []byte("test-ca")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	hash, err := hashCABundle(file)
	if err != nil {
		t.Fatalf("hashCABundle error: %v", err)
	}
	sum := sha256.Sum256(data)
	if hash != fmt.Sprintf("%x", sum[:]) {
		t.Fatalf("unexpected hash %q", hash)
	}
}

func TestOIDCStateStoreCleanupAndConsume(t *testing.T) {
	store := &oidcStateStore{entries: make(map[string]*oidcStateEntry), stopCleanup: make(chan struct{})}
	store.Put("expired", &oidcStateEntry{ExpiresAt: time.Now().Add(-time.Minute)})
	store.Put("active", &oidcStateEntry{ExpiresAt: time.Now().Add(time.Minute)})

	store.cleanup()

	if _, ok := store.entries["expired"]; ok {
		t.Fatalf("expected expired entry to be cleaned")
	}

	entry, ok := store.Consume("active")
	if !ok || entry == nil {
		t.Fatalf("expected to consume active entry")
	}
	if _, ok := store.Consume("active"); ok {
		t.Fatalf("expected entry to be removed after consume")
	}
}

func TestOIDCStateStoreStop(t *testing.T) {
	store := &oidcStateStore{entries: make(map[string]*oidcStateEntry), stopCleanup: make(chan struct{})}
	store.Stop()
	select {
	case <-store.stopCleanup:
	default:
		t.Fatalf("expected stop channel to be closed")
	}
}

func TestGenerateRandomURLString(t *testing.T) {
	val, err := generateRandomURLString(16)
	if err != nil {
		t.Fatalf("generateRandomURLString error: %v", err)
	}
	if val == "" {
		t.Fatalf("expected non-empty value")
	}
}

func TestGeneratePKCEPair(t *testing.T) {
	verifier, challenge, err := generatePKCEPair()
	if err != nil {
		t.Fatalf("generatePKCEPair error: %v", err)
	}
	if verifier == "" || challenge == "" {
		t.Fatalf("expected verifier and challenge")
	}

	hash := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(hash[:])
	if challenge != expected {
		t.Fatalf("unexpected challenge %q", challenge)
	}
}

func TestOIDCServiceConsumeStateExpired(t *testing.T) {
	svc := &OIDCService{stateStore: newOIDCStateStore()}
	svc.stateStore.Put("expired", &oidcStateEntry{ExpiresAt: time.Now().Add(-time.Minute)})

	entry, ok := svc.consumeState("expired")
	if ok || entry != nil {
		t.Fatalf("expected expired entry to be rejected")
	}
}

func TestOIDCServiceContextWithHTTPClient(t *testing.T) {
	client := &http.Client{}
	svc := &OIDCService{httpClient: client}

	ctx := svc.contextWithHTTPClient(context.Background())
	if ctx == nil {
		t.Fatalf("expected context")
	}
}

func TestOIDCServiceRefreshToken_NoToken(t *testing.T) {
	svc := &OIDCService{}
	if _, err := svc.RefreshToken(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty refresh token")
	}
}

func TestOIDCServiceAuthCodeURL_NoPKCE(t *testing.T) {
	svc := &OIDCService{
		oauth2Cfg: &oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://issuer.example.com/auth"}, ClientID: "client"},
	}
	entry := &oidcStateEntry{Nonce: "nonce"}
	url := svc.authCodeURL("state", entry)
	if url == "" {
		t.Fatalf("expected auth url")
	}
	if strings.Contains(url, "code_challenge=") {
		t.Fatalf("did not expect code_challenge in url")
	}
}

func TestOIDCServiceExchangeCode_Error(t *testing.T) {
	svc := &OIDCService{
		oauth2Cfg: &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: "http://127.0.0.1:0"}, ClientID: "client", ClientSecret: "secret"},
	}
	entry := &oidcStateEntry{CodeVerifier: "verifier"}
	_, err := svc.exchangeCode(context.Background(), "code", entry)
	if err == nil {
		t.Fatalf("expected exchangeCode error")
	}
}

func TestOIDCStateStorePutConsumeExpired(t *testing.T) {
	store := &oidcStateStore{entries: make(map[string]*oidcStateEntry), stopCleanup: make(chan struct{})}
	store.Put("expired", &oidcStateEntry{ExpiresAt: time.Now().Add(-time.Second)})

	if entry, ok := store.Consume("expired"); ok || entry != nil {
		t.Fatalf("expected expired entry to be rejected")
	}
}

func TestOIDCServiceMatchesCABundleHash(t *testing.T) {
	file := t.TempDir() + "/ca.pem"
	data := []byte("test-ca")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	hash, err := hashCABundle(file)
	if err != nil {
		t.Fatalf("hashCABundle error: %v", err)
	}

	svc := &OIDCService{snapshot: oidcSnapshot{issuer: "iss", clientID: "id", clientSecret: "secret", redirectURL: "cb", scopes: []string{"openid"}, caBundle: file, caBundleHash: hash}}
	cfg := &config.OIDCConfig{Enabled: true, IssuerURL: "iss", ClientID: "id", ClientSecret: "secret", RedirectURL: "cb", Scopes: []string{"openid"}, CABundle: file}

	if !svc.Matches(cfg) {
		t.Fatalf("expected CABundle hash to match")
	}
}

func TestOIDCServiceAuthCodeURLMatchesNonce(t *testing.T) {
	svc := &OIDCService{
		oauth2Cfg: &oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://issuer.example.com/auth"}, ClientID: "client"},
	}
	entry := &oidcStateEntry{Nonce: "nonce"}
	url := svc.authCodeURL("state", entry)
	if !strings.Contains(url, "nonce=nonce") {
		t.Fatalf("expected nonce in url, got %q", url)
	}
}

func TestOIDCStateStoreConsumeUnknown(t *testing.T) {
	store := &oidcStateStore{entries: make(map[string]*oidcStateEntry), stopCleanup: make(chan struct{})}
	if entry, ok := store.Consume("missing"); ok || entry != nil {
		t.Fatalf("expected missing entry to return false")
	}
}

func TestOIDCServiceNewStateEntryFields(t *testing.T) {
	svc := &OIDCService{stateStore: newOIDCStateStore()}
	_, entry, err := svc.newStateEntry("", "/return")
	if err != nil {
		t.Fatalf("newStateEntry error: %v", err)
	}
	if entry.Nonce == "" || entry.CodeVerifier == "" || entry.CodeChallenge == "" {
		t.Fatalf("expected nonce and pkce fields")
	}
}

func TestOIDCServiceConsumeStateMissing(t *testing.T) {
	svc := &OIDCService{stateStore: newOIDCStateStore()}
	entry, ok := svc.consumeState("missing")
	if ok || entry != nil {
		t.Fatalf("expected missing state to return false")
	}
}

func TestOIDCServiceAuthCodeURLWithPKCEAndNonce(t *testing.T) {
	svc := &OIDCService{
		oauth2Cfg: &oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://issuer.example.com/auth"}, ClientID: "client"},
	}
	entry := &oidcStateEntry{Nonce: "nonce", CodeChallenge: "challenge"}
	url := svc.authCodeURL("state", entry)
	if !strings.Contains(url, "code_challenge=challenge") || !strings.Contains(url, "nonce=nonce") {
		t.Fatalf("unexpected auth url: %q", url)
	}
}

func TestOIDCServiceContextWithHTTPClientNil(t *testing.T) {
	svc := &OIDCService{}
	ctx := svc.contextWithHTTPClient(context.Background())
	if ctx == nil {
		t.Fatalf("expected context")
	}
}

func TestOIDCServiceRefreshTokenKeepsOldRefresh(t *testing.T) {
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"access","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()

	svc := &OIDCService{
		oauth2Cfg:  &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: server.URL}, ClientID: "client", ClientSecret: "secret"},
		httpClient: server.Client(),
	}

	result, err := svc.RefreshToken(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("RefreshToken error: %v", err)
	}
	if result.RefreshToken != "old-refresh" {
		t.Fatalf("expected old refresh token to be preserved")
	}
	if result.AccessToken == "" {
		t.Fatalf("expected access token")
	}
}

func TestOIDCServiceRefreshTokenReplacesRefresh(t *testing.T) {
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()

	svc := &OIDCService{
		oauth2Cfg:  &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: server.URL}, ClientID: "client", ClientSecret: "secret"},
		httpClient: server.Client(),
	}

	result, err := svc.RefreshToken(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("RefreshToken error: %v", err)
	}
	if result.RefreshToken != "new-refresh" {
		t.Fatalf("expected refresh token to be replaced")
	}
	if result.AccessToken == "" {
		t.Fatalf("expected access token")
	}
}

func TestOIDCServiceExchangeCodeMissingVerifier(t *testing.T) {
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "code_verifier") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"access","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()

	svc := &OIDCService{
		oauth2Cfg:  &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: server.URL}, ClientID: "client", ClientSecret: "secret"},
		httpClient: server.Client(),
	}

	entry := &oidcStateEntry{}
	_, err := svc.exchangeCode(context.Background(), "code", entry)
	if err != nil {
		t.Fatalf("exchangeCode error: %v", err)
	}
}

func TestOIDCServiceMatchesScopeMismatch(t *testing.T) {
	svc := &OIDCService{snapshot: oidcSnapshot{issuer: "iss", clientID: "id", clientSecret: "secret", redirectURL: "cb", scopes: []string{"openid"}}}
	cfg := &config.OIDCConfig{Enabled: true, IssuerURL: "iss", ClientID: "id", ClientSecret: "secret", RedirectURL: "cb", Scopes: []string{"openid", "email"}}

	if svc.Matches(cfg) {
		t.Fatalf("expected scope mismatch")
	}
}

func TestOIDCServiceMatchesNil(t *testing.T) {
	var svc *OIDCService
	if svc.Matches(&config.OIDCConfig{}) {
		t.Fatalf("expected Matches to be false for nil service")
	}
}

func TestOIDCServiceMatchesNilConfig(t *testing.T) {
	svc := &OIDCService{}
	if svc.Matches(nil) {
		t.Fatalf("expected Matches to be false for nil config")
	}
}

func TestGenerateRandomURLString_ErrorSize(t *testing.T) {
	if _, err := generateRandomURLString(0); err != nil {
		t.Fatalf("expected no error for size 0, got %v", err)
	}
}
