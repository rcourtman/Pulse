package knownhosts

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager exposes operations for ensuring SSH host keys exist locally.
type Manager interface {
	// Ensure guarantees that the host key for the provided host exists in the
	// managed known_hosts file.
	Ensure(ctx context.Context, host string) error
	// Path returns the absolute path to the managed known_hosts file.
	Path() string
}

type manager struct {
	path            string
	cache           map[string]struct{}
	mu              sync.Mutex
	keyscanFn       keyscanFunc
	keyscanTimeout  time.Duration
}

type keyscanFunc func(ctx context.Context, host string, timeout time.Duration) ([]byte, error)

const (
	defaultKeyscanTimeout = 5 * time.Second
)

var (
	// ErrNoHostKeys is returned when ssh-keyscan yields no usable entries.
	ErrNoHostKeys = errors.New("knownhosts: no host keys discovered")
)

// Option allows customizing Manager construction.
type Option func(*manager)

// WithTimeout overrides the ssh-keyscan timeout (defaults to 5 seconds).
func WithTimeout(d time.Duration) Option {
	return func(m *manager) {
		if d > 0 {
			m.keyscanTimeout = d
		}
	}
}

// WithKeyscanFunc overrides the function used to fetch host keys (mainly for tests).
func WithKeyscanFunc(fn keyscanFunc) Option {
	return func(m *manager) {
		if fn != nil {
			m.keyscanFn = fn
		}
	}
}

// NewManager returns a Manager writing to the supplied known_hosts path.
func NewManager(path string, opts ...Option) (Manager, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("knownhosts: empty path")
	}

	m := &manager{
		path:           path,
		cache:          make(map[string]struct{}),
		keyscanFn:      defaultKeyscan,
		keyscanTimeout: defaultKeyscanTimeout,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// Ensure implements Manager.Ensure.
func (m *manager) Ensure(ctx context.Context, host string) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("knownhosts: missing host")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.cache[host]; ok {
		return nil
	}

	if err := m.ensureKnownHostsFile(); err != nil {
		return err
	}

	exists, err := hostKeyExists(m.path, host)
	if err != nil {
		return err
	}
	if exists {
		m.cache[host] = struct{}{}
		return nil
	}

	keyData, err := m.keyscanFn(ctx, host, m.keyscanTimeout)
	if err != nil {
		return fmt.Errorf("knownhosts: ssh-keyscan failed for %s: %w", host, err)
	}

	entries := sanitizeKeyscanOutput(host, keyData)
	if len(entries) == 0 {
		return fmt.Errorf("%w for %s", ErrNoHostKeys, host)
	}

	if err := appendHostKey(m.path, entries); err != nil {
		return err
	}

	m.cache[host] = struct{}{}
	return nil
}

// Path implements Manager.Path.
func (m *manager) Path() string {
	return m.path
}

func (m *manager) ensureKnownHostsFile() error {
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("knownhosts: mkdir %s: %w", dir, err)
	}

	if _, err := os.Stat(m.path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	f, err := os.OpenFile(m.path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("knownhosts: create %s: %w", m.path, err)
	}
	return f.Close()
}

func hostKeyExists(path, host string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if hostLineMatches(host, scanner.Text()) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func appendHostKey(path string, entries [][]byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("knownhosts: open %s: %w", path, err)
	}
	defer f.Close()

	for _, entry := range entries {
		if len(entry) == 0 {
			continue
		}
		if _, err := f.Write(append(entry, '\n')); err != nil {
			return fmt.Errorf("knownhosts: write entry: %w", err)
		}
	}
	return nil
}

func sanitizeKeyscanOutput(host string, raw []byte) [][]byte {
	var entries [][]byte

	lines := bytes.Split(raw, []byte{'\n'})
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if hostLineMatches(host, string(line)) {
			entries = append(entries, line)
		}
	}

	return entries
}

func hostLineMatches(host, line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	if strings.HasPrefix(trimmed, "|") {
		return false // hashed entry; we only manage clear-text hosts
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}

	return hostFieldMatches(host, fields[0])
}

func hostFieldMatches(host, field string) bool {
	for _, part := range strings.Split(field, ",") {
		for _, candidate := range hostCandidates(part) {
			if strings.EqualFold(candidate, host) {
				return true
			}
		}
	}
	return false
}

func hostCandidates(part string) []string {
	part = strings.TrimSpace(part)
	if part == "" {
		return nil
	}

	if strings.HasPrefix(part, "[") {
		if idx := strings.Index(part, "]"); idx != -1 {
			host := part[1:idx]
			candidates := []string{part}
			if host != "" {
				candidates = append(candidates, host)
			}
			return candidates
		}
	}

	candidates := []string{part}
	if strings.Count(part, ":") == 1 {
		if idx := strings.Index(part, ":"); idx > 0 {
			candidates = append(candidates, part[:idx])
		}
	}

	return candidates
}

func defaultKeyscan(ctx context.Context, host string, timeout time.Duration) ([]byte, error) {
	seconds := int(timeout.Round(time.Second) / time.Second)
	if seconds <= 0 {
		seconds = int(defaultKeyscanTimeout / time.Second)
	}

	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(scanCtx, "ssh-keyscan", "-T", strconv.Itoa(seconds), host)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
