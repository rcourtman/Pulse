package authstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	DefaultProfileName = "default"

	BackendKeychain      = "keychain"
	BackendSecretService = "secret-service"
	BackendDPAPI         = "dpapi"
	BackendFile          = "file"

	profilesFileName    = "profiles.json"
	credentialsFileName = "credentials.json"
	keychainServiceName = "pulse-cli"
)

var (
	ErrProfileNotFound     = errors.New("profile not found")
	ErrSecretNotFound      = errors.New("secret not found")
	ErrKeychainUnavailable = errors.New("os keychain unavailable")
)

// Profile stores non-secret connection metadata for one Pulse instance.
// Raw API tokens are never written to profiles.json.
type Profile struct {
	URL          string    `json:"url"`
	TokenBackend string    `json:"tokenBackend"`
	Scope        string    `json:"scope,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ProfilesFile is the JSON representation persisted at profiles.json.
type ProfilesFile struct {
	DefaultProfile string             `json:"defaultProfile"`
	Profiles       map[string]Profile `json:"profiles"`
}

// SecretStore abstracts OS keychain and file-based secret persistence so unit
// tests can run deterministically without touching the host keychain.
type SecretStore interface {
	Backend() string
	Available() bool
	Set(profile, token string) error
	Get(profile string) (string, error)
	Delete(profile string) error
}

// StoreOptions configures a Store instance. Zero values use platform defaults.
type StoreOptions struct {
	ConfigDir string
	Keychain  SecretStore
	FileStore SecretStore
	Now       func() time.Time
}

// Store manages multi-profile CLI configuration and credential storage.
type Store struct {
	configDir string
	keychain  SecretStore
	fileStore SecretStore
	now       func() time.Time
}

// DefaultConfigDir returns the user-local directory for Pulse CLI config files.
// Precedence: PULSE_CONFIG_DIR > XDG_CONFIG_HOME/pulse > platform default
// (%APPDATA%\pulse on Windows; ~/.config/pulse on Unix).
func DefaultConfigDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("PULSE_CONFIG_DIR")); override != "" {
		return filepath.Clean(override), nil
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(filepath.Clean(xdg), "pulse"), nil
	}
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config dir: %w", err)
		}
		return filepath.Join(dir, "pulse"), nil
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".config", "pulse"), nil
	}
	dir, cfgErr := os.UserConfigDir()
	if cfgErr != nil {
		return "", fmt.Errorf("resolve user config dir: %w", cfgErr)
	}
	return filepath.Join(dir, "pulse"), nil
}

// NormalizeProfileName validates and normalizes a CLI profile name.
// Empty input resolves to DefaultProfileName ("default").
func NormalizeProfileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultProfileName, nil
	}
	if len(name) > 64 {
		return "", fmt.Errorf("invalid profile name %q: must be at most 64 characters", name)
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		isAlphaNum := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		isAllowedPunct := ch == '-' || ch == '_' || ch == '.'
		if i == 0 && !isAlphaNum {
			return "", fmt.Errorf("invalid profile name %q: must start with a letter or digit", name)
		}
		if !isAlphaNum && !isAllowedPunct {
			return "", fmt.Errorf("invalid profile name %q: allowed characters are letters, digits, '.', '-', '_'", name)
		}
	}
	return name, nil
}

// NormalizeURL validates and normalizes a Pulse server base URL.
func NormalizeURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("pulse URL is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid pulse URL %q: %w", rawURL, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("invalid pulse URL %q: scheme must be http or https", rawURL)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("invalid pulse URL %q: host is required", rawURL)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("invalid pulse URL: embedded userinfo is not allowed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid pulse URL %q: query parameters and fragments are not allowed", rawURL)
	}
	parsed.Scheme = scheme
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

// NewDefaultStore constructs a Store using default platform directories and
// keychain/file backends.
func NewDefaultStore() (*Store, error) {
	return NewStore(StoreOptions{})
}

// NewStore constructs a Store with optional overrides for tests or custom
// directories.
func NewStore(opts StoreOptions) (*Store, error) {
	dir := strings.TrimSpace(opts.ConfigDir)
	if dir == "" {
		var err error
		dir, err = DefaultConfigDir()
		if err != nil {
			return nil, err
		}
	}
	dir = filepath.Clean(dir)

	fileStore := opts.FileStore
	if fileStore == nil {
		fileStore = NewFileSecretStore(filepath.Join(dir, credentialsFileName))
	}
	keychain := opts.Keychain
	if keychain == nil {
		keychain = defaultKeychainStore(dir)
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	return &Store{
		configDir: dir,
		keychain:  keychain,
		fileStore: fileStore,
		now:       nowFn,
	}, nil
}

// ConfigDir returns the directory containing profiles.json and fallback files.
func (s *Store) ConfigDir() string {
	return s.configDir
}

// ProfilesPath returns the full path to profiles.json.
func (s *Store) ProfilesPath() string {
	return filepath.Join(s.configDir, profilesFileName)
}

// CredentialsPath returns the full path to the 0600 fallback credentials file.
func (s *Store) CredentialsPath() string {
	return filepath.Join(s.configDir, credentialsFileName)
}

// LoadProfiles reads profiles.json, returning an empty initialized ProfilesFile
// if no config file exists yet.
func (s *Store) LoadProfiles() (*ProfilesFile, error) {
	data, err := os.ReadFile(s.ProfilesPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ProfilesFile{Profiles: map[string]Profile{}}, nil
		}
		return nil, fmt.Errorf("read profiles config %s: %w", s.ProfilesPath(), err)
	}
	var pf ProfilesFile
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &pf); err != nil {
			return nil, fmt.Errorf("parse profiles config %s: %w", s.ProfilesPath(), err)
		}
	}
	if pf.Profiles == nil {
		pf.Profiles = map[string]Profile{}
	}
	return &pf, nil
}

// SaveProfile stores the token in the OS keychain (falling back to 0600
// credentials.json when no keychain is available) and writes non-secret profile
// metadata to profiles.json with 0600 permissions.
func (s *Store) SaveProfile(name, rawURL, token, scope string) (Profile, error) {
	resolvedName, err := NormalizeProfileName(name)
	if err != nil {
		return Profile{}, err
	}
	cleanURL, err := NormalizeURL(rawURL)
	if err != nil {
		return Profile{}, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Profile{}, fmt.Errorf("API token must not be empty")
	}

	backend := ""
	if s.keychain != nil && s.keychain.Available() {
		if err := s.keychain.Set(resolvedName, token); err == nil {
			backend = s.keychain.Backend()
			// Remove any stale fallback file token when keychain succeeds.
			if s.fileStore != nil {
				_ = s.fileStore.Delete(resolvedName)
			}
		}
	}
	if backend == "" {
		if s.fileStore == nil || !s.fileStore.Available() {
			return Profile{}, fmt.Errorf("no credential storage backend available")
		}
		if err := s.fileStore.Set(resolvedName, token); err != nil {
			return Profile{}, fmt.Errorf("store token in credentials file: %w", err)
		}
		backend = s.fileStore.Backend()
	}

	pf, err := s.LoadProfiles()
	if err != nil {
		return Profile{}, err
	}
	profile := Profile{
		URL:          cleanURL,
		TokenBackend: backend,
		Scope:        strings.TrimSpace(scope),
		UpdatedAt:    s.now().UTC(),
	}
	pf.Profiles[resolvedName] = profile
	if strings.TrimSpace(pf.DefaultProfile) == "" {
		pf.DefaultProfile = resolvedName
	}

	if err := s.saveProfilesFile(pf); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

// GetProfile resolves a named profile (or the default profile when name is
// empty) from profiles.json without retrieving its secret token.
func (s *Store) GetProfile(name string) (string, Profile, error) {
	pf, err := s.LoadProfiles()
	if err != nil {
		return "", Profile{}, err
	}
	resolved, err := resolveProfileLookupName(pf, name)
	if err != nil {
		return "", Profile{}, err
	}
	profile, ok := pf.Profiles[resolved]
	if !ok {
		return "", Profile{}, fmt.Errorf("%w: %s", ErrProfileNotFound, resolved)
	}
	return resolved, profile, nil
}

// GetToken resolves a named (or default) profile and fetches its stored token
// from the backend recorded in profiles.json.
func (s *Store) GetToken(name string) (string, Profile, string, error) {
	resolved, profile, err := s.GetProfile(name)
	if err != nil {
		return "", Profile{}, "", err
	}

	var token string
	switch profile.TokenBackend {
	case BackendFile:
		if s.fileStore == nil {
			return "", Profile{}, "", ErrSecretNotFound
		}
		token, err = s.fileStore.Get(resolved)
	case "":
		if s.keychain != nil && s.keychain.Available() {
			token, err = s.keychain.Get(resolved)
		} else {
			err = ErrKeychainUnavailable
		}
		if err != nil && s.fileStore != nil {
			if fileToken, fileErr := s.fileStore.Get(resolved); fileErr == nil {
				token = fileToken
				err = nil
			}
		}
	default:
		if s.keychain == nil {
			return "", Profile{}, "", fmt.Errorf("%w (backend %s)", ErrKeychainUnavailable, profile.TokenBackend)
		}
		token, err = s.keychain.Get(resolved)
	}
	if err != nil {
		return resolved, profile, "", fmt.Errorf("load token for profile %q (%s): %w", resolved, profile.TokenBackend, err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return resolved, profile, "", fmt.Errorf("%w: profile %q", ErrSecretNotFound, resolved)
	}
	return resolved, profile, token, nil
}

// DeleteProfile removes a profile's stored token from both keychain and file
// fallback stores and deletes its entry from profiles.json.
func (s *Store) DeleteProfile(name string) error {
	pf, err := s.LoadProfiles()
	if err != nil {
		return err
	}
	resolved, err := resolveProfileLookupName(pf, name)
	if err != nil {
		return err
	}
	if _, ok := pf.Profiles[resolved]; !ok {
		return fmt.Errorf("%w: %s", ErrProfileNotFound, resolved)
	}

	if s.keychain != nil && s.keychain.Available() {
		_ = s.keychain.Delete(resolved)
	}
	if s.fileStore != nil {
		_ = s.fileStore.Delete(resolved)
	}

	delete(pf.Profiles, resolved)
	if pf.DefaultProfile == resolved {
		pf.DefaultProfile = pickNextDefaultProfile(pf.Profiles)
	}
	return s.saveProfilesFile(pf)
}

// SetDefaultProfile changes the default profile in profiles.json.
func (s *Store) SetDefaultProfile(name string) error {
	resolved, err := NormalizeProfileName(name)
	if err != nil {
		return err
	}
	pf, err := s.LoadProfiles()
	if err != nil {
		return err
	}
	if _, ok := pf.Profiles[resolved]; !ok {
		return fmt.Errorf("%w: %s", ErrProfileNotFound, resolved)
	}
	pf.DefaultProfile = resolved
	return s.saveProfilesFile(pf)
}

// ListProfiles returns the configured default profile name and a detached copy
// of all saved profiles.
func (s *Store) ListProfiles() (string, map[string]Profile, error) {
	pf, err := s.LoadProfiles()
	if err != nil {
		return "", nil, err
	}
	out := make(map[string]Profile, len(pf.Profiles))
	for k, v := range pf.Profiles {
		out[k] = v
	}
	return pf.DefaultProfile, out, nil
}

func (s *Store) saveProfilesFile(pf *ProfilesFile) error {
	payload, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profiles config: %w", err)
	}
	payload = append(payload, '\n')
	return writeFileAtomic(s.ProfilesPath(), payload, 0o600)
}

func resolveProfileLookupName(pf *ProfilesFile, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		if pf != nil && strings.TrimSpace(pf.DefaultProfile) != "" {
			return strings.TrimSpace(pf.DefaultProfile), nil
		}
		return DefaultProfileName, nil
	}
	return NormalizeProfileName(name)
}

func pickNextDefaultProfile(profiles map[string]Profile) string {
	if len(profiles) == 0 {
		return ""
	}
	if _, ok := profiles[DefaultProfileName]; ok {
		return DefaultProfileName
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names[0]
}

// FileSecretStore stores per-profile API tokens in a 0600 JSON file as an
// explicit fallback for headless/SSH hosts without an OS keychain.
type FileSecretStore struct {
	path string
}

type credentialsFilePayload struct {
	Version int               `json:"version"`
	Tokens  map[string]string `json:"tokens"`
}

// NewFileSecretStore creates a file-backed SecretStore at path.
func NewFileSecretStore(path string) *FileSecretStore {
	return &FileSecretStore{path: filepath.Clean(path)}
}

func (f *FileSecretStore) Backend() string {
	return BackendFile
}

func (f *FileSecretStore) Available() bool {
	return f != nil && strings.TrimSpace(f.path) != ""
}

func (f *FileSecretStore) Set(profile, token string) error {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token must not be empty")
	}
	creds, err := f.load()
	if err != nil {
		return err
	}
	creds.Version = 1
	creds.Tokens[name] = token
	return f.save(creds)
}

func (f *FileSecretStore) Get(profile string) (string, error) {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return "", err
	}
	creds, err := f.load()
	if err != nil {
		return "", err
	}
	token, ok := creds.Tokens[name]
	if !ok || strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
	}
	return token, nil
}

func (f *FileSecretStore) Delete(profile string) error {
	name, err := NormalizeProfileName(profile)
	if err != nil {
		return err
	}
	creds, err := f.load()
	if err != nil {
		return err
	}
	if _, ok := creds.Tokens[name]; !ok {
		return nil
	}
	delete(creds.Tokens, name)
	if len(creds.Tokens) == 0 {
		if err := os.Remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove empty credentials file %s: %w", f.path, err)
		}
		return nil
	}
	return f.save(creds)
}

func (f *FileSecretStore) load() (*credentialsFilePayload, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &credentialsFilePayload{Version: 1, Tokens: map[string]string{}}, nil
		}
		return nil, fmt.Errorf("read credentials file %s: %w", f.path, err)
	}
	var payload credentialsFilePayload
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, fmt.Errorf("parse credentials file %s: %w", f.path, err)
		}
	}
	if payload.Tokens == nil {
		payload.Tokens = map[string]string{}
	}
	if payload.Version == 0 {
		payload.Version = 1
	}
	return &payload, nil
}

func (f *FileSecretStore) save(creds *credentialsFilePayload) error {
	payload, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials file: %w", err)
	}
	payload = append(payload, '\n')
	return writeFileAtomic(f.path, payload, 0o600)
}

func keychainAccountName(profile string) string {
	return "profile:" + profile
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".pulse-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file %s: %w", tmpPath, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	cleanup = false
	return nil
}
