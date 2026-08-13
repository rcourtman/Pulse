package dockeragent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// registryCredential is one registry login resolved from the host's Docker or
// Podman credential store. IdentityToken marks OAuth identity-token logins
// (for example Azure ACR), whose secret must be exchanged through a
// refresh-token grant instead of Basic auth.
type registryCredential struct {
	Username      string
	Secret        string
	IdentityToken bool
}

// authorizationHeader renders the credential as a Basic Authorization value.
func (c *registryCredential) authorizationHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.Username+":"+c.Secret))
}

// registryCredentialSource resolves pull credentials for a registry host.
// Implementations must be safe for concurrent use. Resolved credentials are
// only ever presented to the registry (or its token endpoint) itself; they
// must never be reported to the Pulse server or appear in check errors.
type registryCredentialSource interface {
	Lookup(ctx context.Context, registry string) *registryCredential
}

const (
	// credentialCacheTTL bounds how long a resolved (or missing) credential is
	// reused before the config files and helpers are consulted again. Short
	// enough to pick up rotated logins, long enough that one full container
	// sweep does not exec a credential helper per image.
	credentialCacheTTL = 5 * time.Minute
	// credentialHelperTimeout caps a docker-credential-<helper> execution.
	credentialHelperTimeout = 10 * time.Second
	// maxCredentialHelperOutputBytes caps helper stdout accepted by the agent.
	maxCredentialHelperOutputBytes = 1 * 1024 * 1024
	// dockerHubConfigKey is the legacy index URL Docker stores Hub logins under.
	dockerHubConfigKey = "https://index.docker.io/v1/"
)

// credentialHelperNamePattern restricts which helper suffixes may be executed.
// The helper name comes from config.json; anything outside this set (path
// separators, shell metacharacters) is rejected rather than exec'd.
var credentialHelperNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// errCredentialsNotFound reports a helper miss ("credentials not found"),
// which is an anonymous fallthrough rather than an error surface.
var errCredentialsNotFound = errors.New("credentials not found")

// dockerConfigCredentials reads the host's Docker/Podman credential store:
// config.json "auths" entries plus credsStore/credHelpers helper binaries,
// exactly the sources `docker pull` consults. Lookups are cached briefly so a
// container sweep does not re-read files or re-exec helpers per image.
type dockerConfigCredentials struct {
	logger zerolog.Logger

	mu    sync.Mutex
	cache map[string]credentialCacheEntry

	// Seams for tests.
	getenv    func(string) string
	homeDir   func() (string, error)
	readFile  func(string) ([]byte, error)
	runHelper func(ctx context.Context, helper, serverURL string) ([]byte, error)
	now       func() time.Time
}

type credentialCacheEntry struct {
	cred      *registryCredential
	expiresAt time.Time
}

// newDockerConfigCredentials creates the default host credential source.
func newDockerConfigCredentials(logger zerolog.Logger) *dockerConfigCredentials {
	c := &dockerConfigCredentials{
		logger:   logger,
		cache:    map[string]credentialCacheEntry{},
		getenv:   os.Getenv,
		homeDir:  os.UserHomeDir,
		readFile: os.ReadFile,
		now:      time.Now,
	}
	c.runHelper = c.execCredentialHelper
	return c
}

// Lookup resolves credentials for a registry host, consulting the cache
// first. A nil return means no usable credential; the caller proceeds
// anonymously as before.
func (c *dockerConfigCredentials) Lookup(ctx context.Context, registry string) *registryCredential {
	host := normalizeRegistryHost(registry)
	if host == "" {
		return nil
	}

	c.mu.Lock()
	if entry, ok := c.cache[host]; ok && c.now().Before(entry.expiresAt) {
		c.mu.Unlock()
		return entry.cred
	}
	c.mu.Unlock()

	cred := c.resolve(ctx, host)

	c.mu.Lock()
	c.cache[host] = credentialCacheEntry{cred: cred, expiresAt: c.now().Add(credentialCacheTTL)}
	c.mu.Unlock()

	return cred
}

// resolve walks the candidate config files in precedence order and returns
// the first credential any of them yields for the host.
func (c *dockerConfigCredentials) resolve(ctx context.Context, host string) *registryCredential {
	for _, path := range c.configFilePaths() {
		data, err := c.readFile(path)
		if err != nil {
			continue
		}
		cred, err := c.credentialFromConfig(ctx, data, host)
		if err != nil {
			c.logger.Debug().Str("path", path).Err(err).Msg("Failed to resolve registry credentials from Docker config")
			continue
		}
		if cred != nil {
			c.logger.Debug().Str("registry", host).Str("path", path).Msg("Using host registry credentials for update check")
			return cred
		}
	}
	return nil
}

// configFilePaths lists candidate credential files in precedence order:
// the Podman override, the Docker config (env override then home), and the
// Podman runtime auth file.
func (c *dockerConfigCredentials) configFilePaths() []string {
	var paths []string
	if authFile := strings.TrimSpace(c.getenv("REGISTRY_AUTH_FILE")); authFile != "" {
		paths = append(paths, authFile)
	}
	if dir := strings.TrimSpace(c.getenv("DOCKER_CONFIG")); dir != "" {
		paths = append(paths, filepath.Join(dir, "config.json"))
	}
	if home, err := c.homeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".docker", "config.json"))
	}
	if runtimeDir := strings.TrimSpace(c.getenv("XDG_RUNTIME_DIR")); runtimeDir != "" {
		paths = append(paths, filepath.Join(runtimeDir, "containers", "auth.json"))
	}
	return paths
}

type dockerConfigFile struct {
	Auths       map[string]dockerConfigAuth `json:"auths"`
	CredsStore  string                      `json:"credsStore"`
	CredHelpers map[string]string           `json:"credHelpers"`
}

type dockerConfigAuth struct {
	Auth          string `json:"auth"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	IdentityToken string `json:"identitytoken"`
}

// credentialFromConfig resolves the host's credential from one parsed config
// file, honoring Docker's precedence: per-registry credHelpers, then the
// global credsStore, then the static auths entry.
func (c *dockerConfigCredentials) credentialFromConfig(ctx context.Context, data []byte, host string) (*registryCredential, error) {
	var cfg dockerConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if helper := configHelperForHost(cfg.CredHelpers, host); helper != "" {
		return c.credentialFromHelper(ctx, helper, host)
	}
	if cfg.CredsStore != "" {
		return c.credentialFromHelper(ctx, cfg.CredsStore, host)
	}

	entry, ok := configAuthForHost(cfg.Auths, host)
	if !ok {
		return nil, nil
	}
	username, secret := entry.Username, entry.Password
	if entry.Auth != "" {
		decoded, err := decodeBase64Auth(entry.Auth)
		if err != nil {
			return nil, fmt.Errorf("decode auth entry: %w", err)
		}
		user, pass, found := strings.Cut(decoded, ":")
		if !found {
			return nil, fmt.Errorf("malformed auth entry")
		}
		username, secret = user, pass
	}
	if entry.IdentityToken != "" {
		return &registryCredential{Username: username, Secret: entry.IdentityToken, IdentityToken: true}, nil
	}
	if username == "" || secret == "" {
		return nil, nil
	}
	return &registryCredential{Username: username, Secret: secret}, nil
}

// decodeBase64Auth accepts both padded and unpadded base64 auth entries.
func decodeBase64Auth(auth string) (string, error) {
	if decoded, err := base64.StdEncoding.DecodeString(auth); err == nil {
		return string(decoded), nil
	}
	decoded, err := base64.RawStdEncoding.DecodeString(auth)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// credentialFromHelper resolves credentials through a docker-credential
// helper binary, the same protocol the Docker CLI uses.
func (c *dockerConfigCredentials) credentialFromHelper(ctx context.Context, helper, host string) (*registryCredential, error) {
	if !credentialHelperNamePattern.MatchString(helper) {
		return nil, fmt.Errorf("invalid credential helper name %q", helper)
	}

	serverURL := host
	if host == "index.docker.io" {
		serverURL = dockerHubConfigKey
	}

	output, err := c.runHelper(ctx, helper, serverURL)
	if err != nil {
		if errors.Is(err, errCredentialsNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var resp struct {
		Username string `json:"Username"`
		Secret   string `json:"Secret"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("decode helper response: %w", err)
	}
	if resp.Secret == "" {
		return nil, nil
	}
	if resp.Username == "<token>" {
		return &registryCredential{Username: resp.Username, Secret: resp.Secret, IdentityToken: true}, nil
	}
	return &registryCredential{Username: resp.Username, Secret: resp.Secret}, nil
}

// execCredentialHelper runs docker-credential-<helper> get with the server
// URL on stdin. Helper stderr is deliberately kept out of returned errors so
// no helper output can ever reach a reported check error.
func (c *dockerConfigCredentials) execCredentialHelper(ctx context.Context, helper, serverURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, credentialHelperTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker-credential-"+helper, "get")
	cmd.Stdin = strings.NewReader(serverURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		combined := strings.ToLower(stdout.String() + " " + stderr.String())
		if strings.Contains(combined, "credentials not found") {
			return nil, errCredentialsNotFound
		}
		return nil, fmt.Errorf("credential helper %q: %v", helper, err)
	}
	if stdout.Len() > maxCredentialHelperOutputBytes {
		return nil, fmt.Errorf("credential helper %q: output too large", helper)
	}
	return stdout.Bytes(), nil
}

// normalizeRegistryHost canonicalizes a registry reference for credential
// lookups: scheme and path are stripped and the Docker Hub aliases collapse
// onto a single key so config entries stored under the legacy index URL match
// checks against registry-1.docker.io.
func normalizeRegistryHost(registry string) string {
	host := strings.ToLower(strings.TrimSpace(registry))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	switch host {
	case "registry-1.docker.io", "index.docker.io", "docker.io", "registry.docker.io":
		return "index.docker.io"
	}
	return host
}

func configAuthForHost(auths map[string]dockerConfigAuth, host string) (dockerConfigAuth, bool) {
	for key, entry := range auths {
		if normalizeRegistryHost(key) == host {
			return entry, true
		}
	}
	return dockerConfigAuth{}, false
}

func configHelperForHost(helpers map[string]string, host string) string {
	for key, helper := range helpers {
		if normalizeRegistryHost(key) == host {
			return helper
		}
	}
	return ""
}
