package monitoring

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

// KnownHostsManager exposes operations for ensuring SSH host keys exist locally.
type KnownHostsManager interface {
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
	// ResetFailures drops all recorded keyscan failure backoff so the next
	// Ensure call retries immediately.
	ResetFailures()
}

type knownHostsManager struct {
	path           string
	cache          map[string]struct{}
	failures       map[string]*keyscanFailure
	mu             sync.Mutex
	keyscanFn      keyscanFunc
	keyscanTimeout time.Duration
}

// keyscanFailure remembers a failed ssh-keyscan so poll cycles don't re-exec
// it every pass against a host that keeps refusing (#1638). Backoff doubles
// per consecutive failure and any success clears the entry.
type keyscanFailure struct {
	retryAt time.Time
	backoff time.Duration
	err     error
}

type keyscanFunc func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error)

const (
	defaultKeyscanTimeout        = 5 * time.Second
	keyscanFailureInitialBackoff = 30 * time.Second
	keyscanFailureMaxBackoff     = 15 * time.Minute
)

// sshBackoffNow is the clock the SSH-related backoff maps in this package read.
// Tests replace it to exercise expiry and compounding without sleeping.
var sshBackoffNow = time.Now

// sshBackoffDecayed reports whether a recorded failure is stale enough that the
// next failure should restart from the floor instead of compounding. Backoff
// that only ever doubles keeps a host pinned at the ceiling after a long quiet
// period, so an entry whose retry deadline passed more than one backoff window
// ago is treated as fresh (#1638).
func sshBackoffDecayed(now, retryAt time.Time, backoff time.Duration) bool {
	if backoff <= 0 {
		return true
	}
	return now.After(retryAt.Add(backoff))
}

// nextSSHBackoff returns the backoff to record for a failure, given whatever
// was recorded for the same target before.
func nextSSHBackoff(now time.Time, previousRetryAt time.Time, previousBackoff, initial, ceiling time.Duration, escalate bool) time.Duration {
	if !escalate || previousBackoff <= 0 || sshBackoffDecayed(now, previousRetryAt, previousBackoff) {
		return initial
	}
	backoff := previousBackoff * 2
	if backoff > ceiling {
		backoff = ceiling
	}
	return backoff
}

var (
	mkdirAllFn       = os.MkdirAll
	statFn           = os.Stat
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
	// ErrKeyscanSuppressed signals that no ssh-keyscan was executed because the
	// previous failure's backoff window has not expired yet. Callers use it to
	// tell "the host refused us" apart from "we did not ask", so they do not
	// escalate their own backoff for work that never ran.
	ErrKeyscanSuppressed = errors.New("knownhosts: ssh-keyscan suppressed by backoff")
	// ErrHostKeyChanged signals that a host key already exists with a different fingerprint.
	ErrHostKeyChanged = errors.New("knownhosts: host key changed")
)

var (
	defaultMkdirAllFn       = mkdirAllFn
	defaultStatFn           = statFn
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

// KnownHostsOption allows customizing KnownHostsManager construction.
type KnownHostsOption func(*knownHostsManager)

// WithTimeout overrides the ssh-keyscan timeout (defaults to 5 seconds).
func WithTimeout(d time.Duration) KnownHostsOption {
	return func(m *knownHostsManager) {
		if d > 0 {
			m.keyscanTimeout = d
		}
	}
}

// WithKeyscanFunc overrides the function used to fetch host keys (mainly for tests).
func WithKeyscanFunc(fn keyscanFunc) KnownHostsOption {
	return func(m *knownHostsManager) {
		if fn != nil {
			m.keyscanFn = fn
		}
	}
}

// NewKnownHostsManager returns a KnownHostsManager writing to the supplied known_hosts path.
func NewKnownHostsManager(path string, opts ...KnownHostsOption) (KnownHostsManager, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("knownhosts: empty path")
	}

	m := &knownHostsManager{
		path:           path,
		cache:          make(map[string]struct{}),
		failures:       make(map[string]*keyscanFailure),
		keyscanFn:      defaultKeyscan,
		keyscanTimeout: defaultKeyscanTimeout,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// Ensure implements KnownHostsManager.Ensure (uses default port 22).
func (m *knownHostsManager) Ensure(ctx context.Context, host string) error {
	return m.EnsureWithPort(ctx, host, 22)
}

// EnsureWithPort implements KnownHostsManager.EnsureWithPort.
func (m *knownHostsManager) EnsureWithPort(ctx context.Context, host string, port int) error {
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
	if cached {
		m.mu.Unlock()
		return nil
	}
	if failure := m.failures[cacheKey]; failure != nil && sshBackoffNow().Before(failure.retryAt) {
		err := failure.err
		m.mu.Unlock()
		return fmt.Errorf("%w for %s:%d until it expires: %w", ErrKeyscanSuppressed, host, port, err)
	}
	m.mu.Unlock()

	keyData, err := m.keyscanFn(ctx, host, port, m.keyscanTimeout)
	if err != nil {
		wrapped := fmt.Errorf("knownhosts: ssh-keyscan failed for %s:%d: %w", host, port, err)
		m.recordKeyscanFailure(cacheKey, wrapped)
		return wrapped
	}

	entries := sanitizeKeyscanOutput(hostSpec, keyData)
	if len(entries) == 0 {
		wrapped := fmt.Errorf("%w for %s:%d", ErrNoHostKeys, host, port)
		m.recordKeyscanFailure(cacheKey, wrapped)
		return wrapped
	}

	return m.EnsureWithEntries(ctx, host, port, entries)
}

func (m *knownHostsManager) recordKeyscanFailure(cacheKey string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failures == nil {
		m.failures = make(map[string]*keyscanFailure)
	}

	// A keyscan that ran out of time says nothing about the host refusing us,
	// so hold such a failure at the floor rather than compounding it.
	escalate := !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)

	now := sshBackoffNow()
	var previousRetryAt time.Time
	var previousBackoff time.Duration
	if existing := m.failures[cacheKey]; existing != nil {
		previousRetryAt = existing.retryAt
		previousBackoff = existing.backoff
	}
	backoff := nextSSHBackoff(now, previousRetryAt, previousBackoff, keyscanFailureInitialBackoff, keyscanFailureMaxBackoff, escalate)

	m.failures[cacheKey] = &keyscanFailure{
		retryAt: now.Add(backoff),
		backoff: backoff,
		err:     err,
	}
}

// ResetFailures implements KnownHostsManager.ResetFailures.
func (m *knownHostsManager) ResetFailures() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures = make(map[string]*keyscanFailure)
}

// EnsureWithEntries installs the provided host key entries for host:port.
func (m *knownHostsManager) EnsureWithEntries(ctx context.Context, host string, port int, entries [][]byte) error {
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
	delete(m.failures, cacheKey)
	return nil
}

// Path implements KnownHostsManager.Path.
func (m *knownHostsManager) Path() string {
	return m.path
}

func (m *knownHostsManager) ensureKnownHostsFile() error {
	dir := filepath.Dir(m.path)
	if err := mkdirAllFn(dir, 0o700); err != nil {
		return fmt.Errorf("knownhosts: mkdir %s: %w", dir, err)
	}

	if _, err := statFn(m.path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	f, err := openFileFn(m.path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("knownhosts: create %s: %w", m.path, err)
	}
	return f.Close()
}

func appendHostKey(path string, entries [][]byte) error {
	f, err := appendOpenFileFn(path)
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

func findHostKeyLine(path, host, keyType string) (string, error) {
	f, err := openFn(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

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
		return "", err
	}
	return "", nil
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
		// Surface our own timeout as a context error so the backoff can tell it
		// apart from a host actively refusing the scan.
		if ctxErr := scanCtx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("%w (output: %s)", ctxErr, strings.TrimSpace(string(output)))
		}
		return nil, fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
