package dockeragent

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// credentialSourceFunc adapts a function to registryCredentialSource.
type credentialSourceFunc func(ctx context.Context, registry string) *registryCredential

func (f credentialSourceFunc) Lookup(ctx context.Context, registry string) *registryCredential {
	return f(ctx, registry)
}

func staticCredentials(registry, username, secret string) credentialSourceFunc {
	return func(_ context.Context, got string) *registryCredential {
		if got != registry {
			return nil
		}
		return &registryCredential{Username: username, Secret: secret}
	}
}

func basicAuthValue(username, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+secret))
}

func newTestCredentialStore(files map[string]string, env map[string]string) *dockerConfigCredentials {
	cleanFiles := make(map[string]string, len(files))
	for path, content := range files {
		cleanFiles[filepath.Clean(path)] = content
	}

	c := &dockerConfigCredentials{
		logger: zerolog.Nop(),
		cache:  map[string]credentialCacheEntry{},
		getenv: func(key string) string { return env[key] },
		homeDir: func() (string, error) {
			if home, ok := env["HOME"]; ok {
				return home, nil
			}
			return "", errors.New("no home")
		},
		readFile: func(path string) ([]byte, error) {
			if content, ok := cleanFiles[filepath.Clean(path)]; ok {
				return []byte(content), nil
			}
			return nil, os.ErrNotExist
		},
		now: time.Now,
	}
	c.runHelper = func(_ context.Context, _, _ string) ([]byte, error) {
		return nil, errCredentialsNotFound
	}
	return c
}

func TestNormalizeRegistryHost(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"registry-1.docker.io", "index.docker.io"},
		{"index.docker.io", "index.docker.io"},
		{"docker.io", "index.docker.io"},
		{"registry.docker.io", "index.docker.io"},
		{"https://index.docker.io/v1/", "index.docker.io"},
		{"http://registry.example.com", "registry.example.com"},
		{"Registry.Example.COM:5000/some/path", "registry.example.com:5000"},
		{"ghcr.io", "ghcr.io"},
		{"  quay.io  ", "quay.io"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := normalizeRegistryHost(tt.in); got != tt.want {
			t.Errorf("normalizeRegistryHost(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDockerConfigCredentials_StaticAuths(t *testing.T) {
	auth := base64.StdEncoding.EncodeToString([]byte("user:pa:ss"))
	store := newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": fmt.Sprintf(`{"auths":{"registry.example.com":{"auth":%q}}}`, auth),
	}, map[string]string{"HOME": "/home/agent"})

	cred := store.Lookup(context.Background(), "registry.example.com")
	if cred == nil {
		t.Fatal("Expected credential, got nil")
	}
	// Passwords may contain colons; only the first splits user from secret.
	if cred.Username != "user" || cred.Secret != "pa:ss" || cred.IdentityToken {
		t.Fatalf("Unexpected credential: %+v", cred)
	}

	if got := store.Lookup(context.Background(), "other.example.com"); got != nil {
		t.Fatalf("Expected nil for unknown registry, got %+v", got)
	}
}

func TestDockerConfigCredentials_DockerHubAlias(t *testing.T) {
	auth := base64.StdEncoding.EncodeToString([]byte("hubuser:hubpass"))
	store := newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": fmt.Sprintf(`{"auths":{"https://index.docker.io/v1/":{"auth":%q}}}`, auth),
	}, map[string]string{"HOME": "/home/agent"})

	cred := store.Lookup(context.Background(), "registry-1.docker.io")
	if cred == nil || cred.Username != "hubuser" || cred.Secret != "hubpass" {
		t.Fatalf("Expected hub credential via legacy index key, got %+v", cred)
	}
}

func TestDockerConfigCredentials_PlaintextFieldsAndIdentityToken(t *testing.T) {
	store := newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": `{"auths":{
			"plain.example.com":{"username":"u","password":"p"},
			"acr.example.com":{"auth":"MDAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAwOg==","identitytoken":"refresh-token"}
		}}`,
	}, map[string]string{"HOME": "/home/agent"})

	cred := store.Lookup(context.Background(), "plain.example.com")
	if cred == nil || cred.Username != "u" || cred.Secret != "p" {
		t.Fatalf("Expected plaintext credential, got %+v", cred)
	}

	acr := store.Lookup(context.Background(), "acr.example.com")
	if acr == nil || !acr.IdentityToken || acr.Secret != "refresh-token" {
		t.Fatalf("Expected identity-token credential, got %+v", acr)
	}
}

func TestDockerConfigCredentials_UnpaddedBase64(t *testing.T) {
	auth := base64.RawStdEncoding.EncodeToString([]byte("user:pass"))
	store := newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": fmt.Sprintf(`{"auths":{"registry.example.com":{"auth":%q}}}`, auth),
	}, map[string]string{"HOME": "/home/agent"})

	cred := store.Lookup(context.Background(), "registry.example.com")
	if cred == nil || cred.Username != "user" || cred.Secret != "pass" {
		t.Fatalf("Expected credential from unpadded auth, got %+v", cred)
	}
}

func TestDockerConfigCredentials_CredHelpers(t *testing.T) {
	store := newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": `{"credsStore":"global","credHelpers":{"helper.example.com":"special"}}`,
	}, map[string]string{"HOME": "/home/agent"})

	var gotHelper, gotServerURL string
	store.runHelper = func(_ context.Context, helper, serverURL string) ([]byte, error) {
		gotHelper, gotServerURL = helper, serverURL
		return []byte(`{"Username":"helper-user","Secret":"helper-secret"}`), nil
	}

	cred := store.Lookup(context.Background(), "helper.example.com")
	if cred == nil || cred.Username != "helper-user" || cred.Secret != "helper-secret" {
		t.Fatalf("Expected helper credential, got %+v", cred)
	}
	if gotHelper != "special" {
		t.Fatalf("Expected per-registry credHelper to win over credsStore, got %q", gotHelper)
	}
	if gotServerURL != "helper.example.com" {
		t.Fatalf("Expected host server URL, got %q", gotServerURL)
	}
}

func TestDockerConfigCredentials_CredsStoreHubServerURL(t *testing.T) {
	store := newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": `{"credsStore":"osxkeychain"}`,
	}, map[string]string{"HOME": "/home/agent"})

	var gotServerURL string
	store.runHelper = func(_ context.Context, _, serverURL string) ([]byte, error) {
		gotServerURL = serverURL
		return []byte(`{"Username":"<token>","Secret":"identity"}`), nil
	}

	cred := store.Lookup(context.Background(), "registry-1.docker.io")
	if cred == nil || !cred.IdentityToken || cred.Secret != "identity" {
		t.Fatalf("Expected identity-token helper credential, got %+v", cred)
	}
	if gotServerURL != dockerHubConfigKey {
		t.Fatalf("Expected legacy hub server URL, got %q", gotServerURL)
	}
}

func TestDockerConfigCredentials_HelperMissAndFailure(t *testing.T) {
	store := newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": `{"credsStore":"missing"}`,
	}, map[string]string{"HOME": "/home/agent"})

	if cred := store.Lookup(context.Background(), "registry.example.com"); cred != nil {
		t.Fatalf("Expected nil on helper miss, got %+v", cred)
	}

	store = newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": `{"credsStore":"broken"}`,
	}, map[string]string{"HOME": "/home/agent"})
	store.runHelper = func(_ context.Context, _, _ string) ([]byte, error) {
		return nil, errors.New("boom")
	}
	if cred := store.Lookup(context.Background(), "registry.example.com"); cred != nil {
		t.Fatalf("Expected nil on helper failure, got %+v", cred)
	}
}

func TestDockerConfigCredentials_RejectsUnsafeHelperName(t *testing.T) {
	store := newTestCredentialStore(map[string]string{
		"/home/agent/.docker/config.json": `{"credHelpers":{"registry.example.com":"../evil"}}`,
	}, map[string]string{"HOME": "/home/agent"})

	helperCalled := false
	store.runHelper = func(_ context.Context, _, _ string) ([]byte, error) {
		helperCalled = true
		return []byte(`{"Username":"u","Secret":"s"}`), nil
	}

	if cred := store.Lookup(context.Background(), "registry.example.com"); cred != nil {
		t.Fatalf("Expected nil for unsafe helper name, got %+v", cred)
	}
	if helperCalled {
		t.Fatal("Helper with unsafe name must not be executed")
	}
}

func TestDockerConfigCredentials_FilePrecedence(t *testing.T) {
	authFileCred := base64.StdEncoding.EncodeToString([]byte("podman:override"))
	dockerCred := base64.StdEncoding.EncodeToString([]byte("docker:home"))
	store := newTestCredentialStore(map[string]string{
		"/etc/pulse/auth.json":            fmt.Sprintf(`{"auths":{"registry.example.com":{"auth":%q}}}`, authFileCred),
		"/home/agent/.docker/config.json": fmt.Sprintf(`{"auths":{"registry.example.com":{"auth":%q},"only-home.example.com":{"auth":%q}}}`, dockerCred, dockerCred),
	}, map[string]string{
		"HOME":               "/home/agent",
		"REGISTRY_AUTH_FILE": "/etc/pulse/auth.json",
	})

	cred := store.Lookup(context.Background(), "registry.example.com")
	if cred == nil || cred.Username != "podman" {
		t.Fatalf("Expected REGISTRY_AUTH_FILE to take precedence, got %+v", cred)
	}

	// A file earlier in precedence without the host must not shadow a later one.
	fallback := store.Lookup(context.Background(), "only-home.example.com")
	if fallback == nil || fallback.Username != "docker" {
		t.Fatalf("Expected fallthrough to the docker config, got %+v", fallback)
	}
}

func TestDockerConfigCredentials_PodmanRuntimeAuthFile(t *testing.T) {
	auth := base64.StdEncoding.EncodeToString([]byte("podman:runtime"))
	store := newTestCredentialStore(map[string]string{
		"/run/user/1000/containers/auth.json": fmt.Sprintf(`{"auths":{"registry.example.com":{"auth":%q}}}`, auth),
	}, map[string]string{
		"HOME":            "/home/agent",
		"XDG_RUNTIME_DIR": "/run/user/1000",
	})

	cred := store.Lookup(context.Background(), "registry.example.com")
	if cred == nil || cred.Username != "podman" || cred.Secret != "runtime" {
		t.Fatalf("Expected podman runtime auth.json credential, got %+v", cred)
	}
}

func TestDockerConfigCredentials_CacheAvoidsRepeatResolution(t *testing.T) {
	auth := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	reads := 0
	store := newTestCredentialStore(nil, map[string]string{"HOME": "/home/agent"})
	store.readFile = func(path string) ([]byte, error) {
		reads++
		if filepath.Clean(path) == filepath.Clean("/home/agent/.docker/config.json") {
			return []byte(fmt.Sprintf(`{"auths":{"registry.example.com":{"auth":%q}}}`, auth)), nil
		}
		return nil, os.ErrNotExist
	}
	current := time.Unix(1700000000, 0)
	store.now = func() time.Time { return current }

	for i := 0; i < 3; i++ {
		if cred := store.Lookup(context.Background(), "registry.example.com"); cred == nil {
			t.Fatal("Expected credential")
		}
	}
	if reads != 1 {
		t.Fatalf("Expected a single config read for cached lookups, got %d", reads)
	}

	current = current.Add(credentialCacheTTL + time.Second)
	if cred := store.Lookup(context.Background(), "registry.example.com"); cred == nil {
		t.Fatal("Expected credential after cache expiry")
	}
	if reads != 2 {
		t.Fatalf("Expected re-resolution after TTL, got %d reads", reads)
	}
}

func TestRegistryChecker_FetchDigest_BasicChallengeWithCredentials(t *testing.T) {
	headCalls := 0
	var retryAuth string
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				headCalls++
				if req.Header.Get("Authorization") == "" {
					return newStringResponse(http.StatusUnauthorized, map[string]string{
						"Www-Authenticate": `Basic realm="Registry"`,
					}, ""), nil
				}
				retryAuth = req.Header.Get("Authorization")
				return newStringResponse(http.StatusOK, map[string]string{
					"Docker-Content-Digest": "sha256:private",
				}, ""), nil
			}),
		},
		credentials: staticCredentials("registry.example.com", "user", "pass"),
	}

	digest, _, err := checker.fetchDigest(context.Background(), "registry.example.com", "app", "latest", "", "", "")
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if digest != "sha256:private" {
		t.Fatalf("Expected private digest, got %q", digest)
	}
	if headCalls != 2 {
		t.Fatalf("Expected 2 HEAD calls, got %d", headCalls)
	}
	if retryAuth != basicAuthValue("user", "pass") {
		t.Fatalf("Expected Basic retry authorization, got %q", retryAuth)
	}
}

func TestRegistryChecker_FetchDigest_BasicChallengeWithoutCredentialsStillFails(t *testing.T) {
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return newStringResponse(http.StatusUnauthorized, map[string]string{
					"Www-Authenticate": `Basic realm="Registry"`,
				}, ""), nil
			}),
		},
	}

	_, _, err := checker.fetchDigest(context.Background(), "registry.example.com", "app", "latest", "", "", "")
	if err == nil || err.Error() != "authentication required" {
		t.Fatalf("Expected authentication required, got %v", err)
	}
}

func TestRegistryChecker_FetchDigest_BearerChallengeWithCredentials(t *testing.T) {
	var tokenAuth string
	var retryAuth string
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "auth.example" {
					tokenAuth = req.Header.Get("Authorization")
					return newStringResponse(http.StatusOK, nil, `{"token":"private-token"}`), nil
				}
				if req.Header.Get("Authorization") == "" {
					return newStringResponse(http.StatusUnauthorized, map[string]string{
						"Www-Authenticate": `Bearer realm="https://auth.example/token",service="registry.example.com"`,
					}, ""), nil
				}
				retryAuth = req.Header.Get("Authorization")
				return newStringResponse(http.StatusOK, map[string]string{
					"Docker-Content-Digest": "sha256:private",
				}, ""), nil
			}),
		},
		credentials: staticCredentials("registry.example.com", "user", "pass"),
	}

	digest, _, err := checker.fetchDigest(context.Background(), "registry.example.com", "team/app", "latest", "", "", "")
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if digest != "sha256:private" {
		t.Fatalf("Expected private digest, got %q", digest)
	}
	if tokenAuth != basicAuthValue("user", "pass") {
		t.Fatalf("Expected Basic auth on token negotiation, got %q", tokenAuth)
	}
	if retryAuth != "Bearer private-token" {
		t.Fatalf("Expected negotiated bearer retry, got %q", retryAuth)
	}
}

func TestRegistryChecker_FetchDigest_DockerHubCredentials(t *testing.T) {
	var tokenAuth string
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "auth.docker.io" {
					tokenAuth = req.Header.Get("Authorization")
					return newStringResponse(http.StatusOK, nil, `{"token":"hub-token"}`), nil
				}
				if req.Header.Get("Authorization") != "Bearer hub-token" {
					return newStringResponse(http.StatusUnauthorized, nil, ""), nil
				}
				return newStringResponse(http.StatusOK, map[string]string{
					"Docker-Content-Digest": "sha256:hub",
				}, ""), nil
			}),
		},
		credentials: staticCredentials("registry-1.docker.io", "hubuser", "hubpass"),
	}

	digest, _, err := checker.fetchDigest(context.Background(), "registry-1.docker.io", "team/private", "latest", "", "", "")
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if digest != "sha256:hub" {
		t.Fatalf("Expected hub digest, got %q", digest)
	}
	if tokenAuth != basicAuthValue("hubuser", "hubpass") {
		t.Fatalf("Expected Basic auth on hub token request, got %q", tokenAuth)
	}
}

func TestRegistryChecker_FetchDigest_StaleCredentialsFallBackToAnonymous(t *testing.T) {
	tokenRequests := 0
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "auth.docker.io" {
					tokenRequests++
					if req.Header.Get("Authorization") != "" {
						// The stored login has been revoked.
						return newStringResponse(http.StatusUnauthorized, nil, ""), nil
					}
					return newStringResponse(http.StatusOK, nil, `{"token":"anon-token"}`), nil
				}
				if req.Header.Get("Authorization") != "Bearer anon-token" {
					return newStringResponse(http.StatusUnauthorized, nil, ""), nil
				}
				return newStringResponse(http.StatusOK, map[string]string{
					"Docker-Content-Digest": "sha256:public",
				}, ""), nil
			}),
		},
		credentials: staticCredentials("registry-1.docker.io", "stale", "stale"),
	}

	digest, _, err := checker.fetchDigest(context.Background(), "registry-1.docker.io", "library/nginx", "latest", "", "", "")
	if err != nil {
		t.Fatalf("Expected anonymous fallback to succeed, got %v", err)
	}
	if digest != "sha256:public" {
		t.Fatalf("Expected public digest, got %q", digest)
	}
	if tokenRequests != 2 {
		t.Fatalf("Expected credentialed then anonymous token requests, got %d", tokenRequests)
	}
}

func TestRegistryChecker_FetchDigest_CredentialedTokenNotRenegotiated(t *testing.T) {
	headCalls := 0
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "auth.docker.io" {
					return newStringResponse(http.StatusOK, nil, `{"token":"hub-token"}`), nil
				}
				headCalls++
				return newStringResponse(http.StatusUnauthorized, map[string]string{
					"Www-Authenticate": `Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`,
				}, ""), nil
			}),
		},
		credentials: staticCredentials("registry-1.docker.io", "user", "pass"),
	}

	_, _, err := checker.fetchDigest(context.Background(), "registry-1.docker.io", "team/private", "latest", "", "", "")
	if err == nil || err.Error() != "authentication required" {
		t.Fatalf("Expected authentication required, got %v", err)
	}
	if headCalls != 1 {
		t.Fatalf("Expected a single HEAD (no pointless renegotiation), got %d", headCalls)
	}
}

func TestRegistryChecker_FetchDigest_IdentityTokenExchange(t *testing.T) {
	var tokenForm string
	var tokenMethod string
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "auth.example" {
					tokenMethod = req.Method
					body, _ := readBodyWithLimit(req.Body, maxRegistryTokenBodyBytes)
					tokenForm = string(body)
					return newStringResponse(http.StatusOK, nil, `{"access_token":"exchanged"}`), nil
				}
				if req.Header.Get("Authorization") == "" {
					return newStringResponse(http.StatusUnauthorized, map[string]string{
						"Www-Authenticate": `Bearer realm="https://auth.example/token",service="acr.example.com"`,
					}, ""), nil
				}
				if req.Header.Get("Authorization") != "Bearer exchanged" {
					return newStringResponse(http.StatusUnauthorized, nil, ""), nil
				}
				return newStringResponse(http.StatusOK, map[string]string{
					"Docker-Content-Digest": "sha256:acr",
				}, ""), nil
			}),
		},
		credentials: credentialSourceFunc(func(_ context.Context, registry string) *registryCredential {
			if registry != "acr.example.com" {
				return nil
			}
			return &registryCredential{Username: "<token>", Secret: "refresh-secret", IdentityToken: true}
		}),
	}

	digest, _, err := checker.fetchDigest(context.Background(), "acr.example.com", "team/app", "latest", "", "", "")
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if digest != "sha256:acr" {
		t.Fatalf("Expected ACR digest, got %q", digest)
	}
	if tokenMethod != http.MethodPost {
		t.Fatalf("Expected POST refresh-token grant, got %s", tokenMethod)
	}
	for _, fragment := range []string{
		"grant_type=refresh_token",
		"refresh_token=refresh-secret",
		"service=acr.example.com",
		"scope=repository%3Ateam%2Fapp%3Apull",
	} {
		if !strings.Contains(tokenForm, fragment) {
			t.Fatalf("Token form missing %q: %q", fragment, tokenForm)
		}
	}
}

func TestRegistryChecker_FetchDigest_BasicAuthCarriesIntoManifestList(t *testing.T) {
	var listAuth string
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("Authorization") == "" {
					return newStringResponse(http.StatusUnauthorized, map[string]string{
						"Www-Authenticate": `Basic realm="Registry"`,
					}, ""), nil
				}
				if req.Method == http.MethodGet {
					listAuth = req.Header.Get("Authorization")
					return newStringResponse(http.StatusOK, nil,
						`{"manifests":[{"digest":"sha256:platform","platform":{"architecture":"amd64","os":"linux"}}]}`), nil
				}
				return newStringResponse(http.StatusOK, map[string]string{
					"Docker-Content-Digest": "sha256:index",
					"Content-Type":          "application/vnd.oci.image.index.v1+json",
				}, ""), nil
			}),
		},
		credentials: staticCredentials("registry.example.com", "user", "pass"),
	}

	digest, headDigest, err := checker.fetchDigest(context.Background(), "registry.example.com", "app", "latest", "amd64", "linux", "")
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}
	if digest != "sha256:platform" || headDigest != "sha256:index" {
		t.Fatalf("Expected resolved platform digest, got %q / %q", digest, headDigest)
	}
	if listAuth != basicAuthValue("user", "pass") {
		t.Fatalf("Expected Basic auth on manifest list fetch, got %q", listAuth)
	}
}
