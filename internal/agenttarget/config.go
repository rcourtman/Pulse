package agenttarget

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

const ConfigVersion = 1

const maxConfigBytes = 256 * 1024
const maxObservers = 16
const maxTokenBytes = 64 * 1024

// Observer is a report-only Pulse destination. The primary Pulse URL remains
// the sole authority for remote configuration, commands, and agent updates.
type Observer struct {
	Name               string
	URL                string
	Token              string
	InsecureSkipVerify bool
	AllowPlaintextHTTP bool
	CACertPath         string
	ServerFingerprint  string
	ProvisionProxmox   bool
	ID                 string
}

type observerFile struct {
	Version   int                  `json:"version"`
	Observers []observerFileTarget `json:"observers"`
}

type observerFileTarget struct {
	Name               string `json:"name"`
	URL                string `json:"url"`
	TokenFile          string `json:"tokenFile"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
	AllowPlaintextHTTP bool   `json:"allowPlaintextHTTP,omitempty"`
	CACertPath         string `json:"caCertPath,omitempty"`
	ServerFingerprint  string `json:"serverFingerprint,omitempty"`
	ProvisionProxmox   *bool  `json:"provisionProxmox,omitempty"`
}

// LoadObservers validates and resolves an owner-private observer configuration.
// Raw tokens are never accepted in the JSON document; each destination must
// reference a separately permissioned token file.
func LoadObservers(path string, primaryURL string) ([]Observer, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		return nil, errors.New("observer config path must be absolute")
	}
	if err := requirePrivateRegularFile(path, "observer config", maxConfigBytes); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open observer config: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(io.LimitReader(f, maxConfigBytes+1))
	dec.DisallowUnknownFields()
	var parsed observerFile
	if err := dec.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode observer config: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return nil, err
	}
	if parsed.Version != ConfigVersion {
		return nil, fmt.Errorf("observer config version %d is unsupported; expected %d", parsed.Version, ConfigVersion)
	}
	if len(parsed.Observers) > maxObservers {
		return nil, fmt.Errorf("observer config has %d destinations; maximum is %d", len(parsed.Observers), maxObservers)
	}

	normalizedPrimary := ""
	if strings.TrimSpace(primaryURL) != "" {
		primary, err := NormalizePulseURL(primaryURL, true, true)
		if err != nil {
			return nil, fmt.Errorf("normalize primary Pulse URL: %w", err)
		}
		normalizedPrimary = primary
	}

	result := make([]Observer, 0, len(parsed.Observers))
	seenNames := make(map[string]struct{}, len(parsed.Observers))
	seenURLs := make(map[string]struct{}, len(parsed.Observers))
	for index, item := range parsed.Observers {
		observer, err := resolveObserver(item)
		if err != nil {
			return nil, fmt.Errorf("observer %d: %w", index+1, err)
		}
		nameKey := strings.ToLower(observer.Name)
		if _, exists := seenNames[nameKey]; exists {
			return nil, fmt.Errorf("observer %d: duplicate name %q", index+1, observer.Name)
		}
		if _, exists := seenURLs[observer.URL]; exists {
			return nil, fmt.Errorf("observer %d: duplicate URL", index+1)
		}
		if normalizedPrimary != "" && observer.URL == normalizedPrimary {
			return nil, fmt.Errorf("observer %d: URL duplicates the primary Pulse destination", index+1)
		}
		seenNames[nameKey] = struct{}{}
		seenURLs[observer.URL] = struct{}{}
		result = append(result, observer)
	}
	return result, nil
}

func resolveObserver(item observerFileTarget) (Observer, error) {
	name := strings.TrimSpace(item.Name)
	if err := validateName(name); err != nil {
		return Observer{}, err
	}
	url, err := NormalizePulseURL(item.URL, false, item.AllowPlaintextHTTP)
	if err != nil {
		return Observer{}, fmt.Errorf("invalid URL: %w", err)
	}

	tokenPath := strings.TrimSpace(item.TokenFile)
	if !filepath.IsAbs(tokenPath) {
		return Observer{}, errors.New("tokenFile must be an absolute path")
	}
	if err := requirePrivateRegularFile(tokenPath, "observer token", maxTokenBytes); err != nil {
		return Observer{}, err
	}
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return Observer{}, fmt.Errorf("read observer token: %w", err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return Observer{}, errors.New("observer token file is empty")
	}
	if strings.IndexFunc(token, unicode.IsSpace) >= 0 {
		return Observer{}, errors.New("observer token contains whitespace")
	}

	provisionProxmox := true
	if item.ProvisionProxmox != nil {
		provisionProxmox = *item.ProvisionProxmox
	}
	return Observer{
		Name:               name,
		URL:                url,
		Token:              token,
		InsecureSkipVerify: item.InsecureSkipVerify,
		AllowPlaintextHTTP: item.AllowPlaintextHTTP,
		CACertPath:         strings.TrimSpace(item.CACertPath),
		ServerFingerprint:  strings.TrimSpace(item.ServerFingerprint),
		ProvisionProxmox:   provisionProxmox,
		ID:                 stableID(url),
	}, nil
}

// NormalizePulseURL applies the transport policy for one report destination.
// Authoritative primary destinations retain the established self-hosted
// local-network HTTP allowance. Report-only observers require an explicit
// per-destination opt-in for every non-loopback plaintext URL, even when the
// process-wide primary override is enabled.
func NormalizePulseURL(raw string, authoritative bool, allowPlaintext bool) (string, error) {
	if !authoritative && !allowPlaintext {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err == nil && parsed.Hostname() != "" && strings.EqualFold(parsed.Scheme, "http") &&
			!securityutil.IsLoopbackHost(parsed.Hostname()) {
			return "", fmt.Errorf("Pulse URL %q must use https unless the observer explicitly allows plaintext HTTP", raw)
		}
	}
	parsed, err := securityutil.NormalizePulseHTTPBaseURLWithOptions(raw, securityutil.PulseURLValidationOptions{
		AllowLocalNetworkHTTP:      authoritative,
		AllowOperatorPlaintextHTTP: allowPlaintext,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 64 {
		return errors.New("name must be 64 characters or fewer")
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("name %q contains unsupported characters", name)
	}
	return nil
}

func requirePrivateRegularFile(path string, label string, maxBytes int64) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat %s file: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%s file must be a regular file, not a symlink", label)
	}
	if info.Size() > maxBytes {
		return fmt.Errorf("%s file exceeds the %d-byte size limit", label, maxBytes)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s file permissions must not grant group or other access", label)
	}
	return nil
}

func ensureJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode observer config trailer: %w", err)
	}
	return errors.New("observer config contains multiple JSON values")
}

func stableID(url string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(url)))
	return hex.EncodeToString(sum[:8])
}
