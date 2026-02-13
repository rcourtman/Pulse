package knownhosts

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
	// EnsureWithPort guarantees that the host key for the provided host:port exists
	// in the managed known_hosts file.
	EnsureWithPort(ctx context.Context, host string, port int) error
	// EnsureWithEntries installs provided host key entries for the given host/port.
	EnsureWithEntries(ctx context.Context, host string, port int, entries [][]byte) error
	// Path returns the absolute path to the managed known_hosts file.
	Path() string
}

type manager struct {
	path           string
	cache          map[string]struct{}
	mu             sync.Mutex
	keyscanFn      keyscanFunc
	keyscanTimeout time.Duration
}

type keyscanFunc func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error)

const (
	defaultKeyscanTimeout = 5 * time.Second
)

var (
	mkdirAllFn       = os.MkdirAll
	statFn           = os.Stat
	chmodFn          = os.Chmod
	openFileFn       = os.OpenFile
	openFn           = os.Open
	appendOpenFileFn = func(path string) (io.WriteCloser, error) {
		return openFileFn(path, os.O_APPEND|os.O_WRONLY, 0o600)
	}
	keyscanCmdRunner = func(ctx context.Context, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "ssh-keyscan", args...)
		return cmd.CombinedOutput()
	}

	// ErrNoHostKeys is returned when ssh-keyscan yields no usable entries.
	ErrNoHostKeys = errors.New("knownhosts: no host keys discovered")
	// ErrHostKeyChanged signals that a host key already exists with a different fingerprint.
	ErrHostKeyChanged = errors.New("knownhosts: host key changed")
)

var (
	defaultMkdirAllFn       = mkdirAllFn
	defaultStatFn           = statFn
	defaultChmodFn          = chmodFn
	defaultOpenFileFn       = openFileFn
	defaultOpenFn           = openFn
	defaultAppendOpenFileFn = appendOpenFileFn
	defaultKeyscanCmdRunner = keyscanCmdRunner
)

// HostKeyChangeError describes a detected host key mismatch.
type HostKeyChangeError struct {
	Host     string
	Existing string
	Provided string
}

func (e *HostKeyChangeError) Error() string {
	return fmt.Sprintf("knownhosts: host key for %s changed", e.Host)
}

func (e *HostKeyChangeError) Unwrap() error {
	return ErrHostKeyChanged
}

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

// Ensure implements Manager.Ensure (uses default port 22).
func (m *manager) Ensure(ctx context.Context, host string) error {
	return m.EnsureWithPort(ctx, host, 22)
}

// EnsureWithPort implements Manager.EnsureWithPort.
func (m *manager) EnsureWithPort(ctx context.Context, host string, port int) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("knownhosts: missing host")
	}
	if port <= 0 {
		port = 22 // Default to standard SSH port
	}

	hostSpec := host
	if port != 22 {
		hostSpec = fmt.Sprintf("[%s]:%d", host, port)
	}

	cacheKey := fmt.Sprintf("%s:%d", host, port)
	m.mu.Lock()
	_, cached := m.cache[cacheKey]
	m.mu.Unlock()
	if cached {
		return nil
	}

	keyData, err := m.keyscanFn(ctx, host, port, m.keyscanTimeout)
	if err != nil {
		return fmt.Errorf("knownhosts: ssh-keyscan failed for %s:%d: %w", host, port, err)
	}

	entries := sanitizeKeyscanOutput(hostSpec, keyData)
	if len(entries) == 0 {
		return fmt.Errorf("%w for %s:%d", ErrNoHostKeys, host, port)
	}

	return m.EnsureWithEntries(ctx, host, port, entries)
}

// EnsureWithEntries installs the provided host key entries for host:port.
func (m *manager) EnsureWithEntries(ctx context.Context, host string, port int, entries [][]byte) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("knownhosts: missing host")
	}
	if port <= 0 {
		port = 22
	}
	if len(entries) == 0 {
		return fmt.Errorf("knownhosts: no host key entries provided for %s", host)
	}

	cacheKey := fmt.Sprintf("%s:%d", host, port)
	hostSpec := host
	if port != 22 {
		hostSpec = fmt.Sprintf("[%s]:%d", host, port)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureKnownHostsFile(); err != nil {
		return err
	}

	var toAppend [][]byte
	for _, entry := range entries {
		normalized, keyType, err := normalizeHostEntry(hostSpec, entry)
		if err != nil {
			return err
		}

		existing, err := findHostKeyLine(m.path, hostSpec, keyType)
		if err != nil {
			return err
		}

		if existing != "" {
			if existing != string(normalized) {
				return &HostKeyChangeError{
					Host:     hostSpec,
					Existing: existing,
					Provided: string(normalized),
				}
			}
			continue
		}

		toAppend = append(toAppend, normalized)
	}

	if len(toAppend) > 0 {
		if err := appendHostKey(m.path, toAppend); err != nil {
			return err
		}
	}

	m.cache[cacheKey] = struct{}{}
	return nil
}

// Path implements Manager.Path.
func (m *manager) Path() string {
	return m.path
}

func (m *manager) ensureKnownHostsFile() error {
	dir := filepath.Dir(m.path)
	if err := mkdirAllFn(dir, 0o700); err != nil {
		return fmt.Errorf("knownhosts: mkdir %s: %w", dir, err)
	}
	if err := chmodFn(dir, 0o700); err != nil {
		return fmt.Errorf("knownhosts: chmod %s: %w", dir, err)
	}

	if _, err := statFn(m.path); err == nil {
		if err := chmodFn(m.path, 0o600); err != nil {
			return fmt.Errorf("knownhosts: chmod %s: %w", m.path, err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("knownhosts: stat %s: %w", m.path, err)
	}

	f, err := openFileFn(m.path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("knownhosts: create %s: %w", m.path, err)
	}
	if err := f.Close(); err != nil {
<<<<<<< HEAD
		return err
	}
	if err := chmodFn(m.path, 0o600); err != nil {
		return fmt.Errorf("knownhosts: chmod %s: %w", m.path, err)
=======
		return fmt.Errorf("knownhosts: close %s: %w", m.path, err)
>>>>>>> refactor/parallel-05-error-handling
	}
	return nil
}

func appendHostKey(path string, entries [][]byte) (retErr error) {
	f, err := appendOpenFileFn(path)
	if err != nil {
		return fmt.Errorf("knownhosts: open %s: %w", path, err)
	}
	defer func() {
		retErr = joinCloseError(retErr, fmt.Sprintf("knownhosts: close %s", path), f.Close())
	}()

	for _, entry := range entries {
		if len(entry) == 0 {
			continue
		}
		if _, err := f.Write(append(entry, '\n')); err != nil {
			return fmt.Errorf("knownhosts: write entry to %s: %w", path, err)
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

func normalizeHostEntry(host string, entry []byte) ([]byte, string, error) {
	trimmed := strings.TrimSpace(string(entry))
	fields := strings.Fields(trimmed)
	if len(fields) < 3 {
		return nil, "", fmt.Errorf("knownhosts: invalid host key entry for %s", host)
	}

	keyType := fields[1]
	keyData := fields[2]
	var comment string
	if len(fields) > 3 {
		comment = strings.Join(fields[3:], " ")
	}

	if comment != "" {
		return []byte(fmt.Sprintf("%s %s %s %s", host, keyType, keyData, comment)), keyType, nil
	}
	return []byte(fmt.Sprintf("%s %s %s", host, keyType, keyData)), keyType, nil
}

func findHostKeyLine(path, host, keyType string) (line string, retErr error) {
	f, err := openFn(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("knownhosts: open %s: %w", path, err)
	}
	defer func() {
		retErr = joinCloseError(retErr, fmt.Sprintf("knownhosts: close %s", path), f.Close())
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !hostLineMatches(host, line) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if keyType != "" && fields[1] != keyType {
			continue
		}
		return strings.TrimSpace(line), nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("knownhosts: scan %s: %w", path, err)
	}
	return "", nil
}

func joinCloseError(err error, op string, closeErr error) error {
	if closeErr == nil {
		return err
	}

	wrappedCloseErr := fmt.Errorf("%s: %w", op, closeErr)
	if err == nil {
		return wrappedCloseErr
	}

	return errors.Join(err, wrappedCloseErr)
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

func defaultKeyscan(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
	seconds := int(timeout.Round(time.Second) / time.Second)
	if seconds <= 0 {
		seconds = int(defaultKeyscanTimeout / time.Second)
	}
	if port <= 0 {
		port = 22
	}

	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{"-T", strconv.Itoa(seconds)}
	if port != 22 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	args = append(args, host)

	output, err := keyscanCmdRunner(scanCtx, args...)
	if err != nil {
		return nil, fmt.Errorf("ssh-keyscan command failed: %w", err)
	}
	return output, nil
}
