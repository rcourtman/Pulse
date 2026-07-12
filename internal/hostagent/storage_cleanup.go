package hostagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

const (
	storageCleanupCacheTTL        = 30 * time.Minute
	storageCleanupMaxCacheEntries = 100000
	aptPackageCachePath           = "/var/cache/apt/archives"
)

type storageCleanupManager struct {
	platform string
	now      func() time.Time
	cacheTTL time.Duration
	run      func(context.Context, []string, string, ...string) packageUpdateCommandResult
	lookPath func(string) (string, error)
	scan     func() (agentexec.HostStorageCleanupSnapshot, error)
	lease    *packageManagerLease

	mu     sync.Mutex
	cached *agentexec.HostStorageCleanupSnapshot
}

func newStorageCleanupManager(platform string, lease *packageManagerLease) *storageCleanupManager {
	return &storageCleanupManager{
		platform: strings.ToLower(strings.TrimSpace(platform)),
		now:      time.Now,
		cacheTTL: storageCleanupCacheTTL,
		run:      runPackageUpdateCommand,
		lookPath: packageUpdateLookPath,
		scan:     scanAPTPackageCache,
		lease:    lease,
	}
}

func (m *storageCleanupManager) Snapshot(ctx context.Context, force bool) agentexec.HostStorageCleanupSnapshot {
	if m == nil {
		return agentexec.HostStorageCleanupSnapshot{}
	}
	release, err := m.lease.acquire(ctx)
	if err != nil {
		return agentexec.HostStorageCleanupSnapshot{CheckedAt: m.currentTime(), Error: "package cache inspection canceled"}
	}
	defer release()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked(force)
}

func (m *storageCleanupManager) snapshotLocked(force bool) agentexec.HostStorageCleanupSnapshot {
	now := m.currentTime()
	if !force && m.cached != nil && now.Sub(m.cached.CheckedAt) < m.cacheTTL {
		return *m.cached
	}

	snapshot := agentexec.HostStorageCleanupSnapshot{CheckedAt: now}
	if !supportsAPTPlatform(m.platform) {
		m.storeSnapshot(snapshot)
		return snapshot
	}
	if _, err := m.lookPath("apt-get"); err != nil {
		m.storeSnapshot(snapshot)
		return snapshot
	}

	snapshot.Supported = true
	snapshot.Provider = "apt-package-cache"
	scanned, err := m.scan()
	if err != nil {
		snapshot.Error = "package cache inspection failed"
		m.storeSnapshot(snapshot)
		return snapshot
	}
	snapshot.Fingerprint = scanned.Fingerprint
	snapshot.ReclaimableBytes = scanned.ReclaimableBytes
	m.storeSnapshot(snapshot)
	return snapshot
}

func (m *storageCleanupManager) Apply(ctx context.Context, req agentexec.HostStorageCleanupPayload) (result agentexec.HostStorageCleanupResultPayload) {
	startedAt := time.Now()
	result = agentexec.HostStorageCleanupResultPayload{
		RequestID: strings.TrimSpace(req.RequestID), ActionID: strings.TrimSpace(req.ActionID),
		ExecutionPhase: agentexec.HostStorageCleanupPhasePreflight,
		Verification:   agentexec.HostStorageCleanupVerificationInconclusive,
	}
	defer func() { result.Duration = time.Since(startedAt).Milliseconds() }()

	if strings.TrimSpace(req.Operation) != agentexec.HostStorageCleanupOperationPackageCache {
		result.Error = "unsupported typed host storage cleanup operation"
		return result
	}
	release, err := m.lease.acquire(ctx)
	if err != nil {
		result.Error = "package manager lease unavailable"
		return result
	}
	defer release()

	m.mu.Lock()
	defer m.mu.Unlock()

	before := m.snapshotLocked(true)
	result.Before = before
	if !before.Supported {
		result.After = before
		result.Error = "host package-cache cleanup is not supported by this agent"
		return result
	}
	if before.Error != "" {
		result.After = before
		result.Error = "package cache cleanup preflight failed"
		return result
	}
	if before.Fingerprint != strings.TrimSpace(req.ExpectedFingerprint) {
		result.After = before
		result.Error = "package cache inventory changed; replan required"
		return result
	}
	if before.ReclaimableBytes <= 0 {
		result.After = before
		result.Error = "package cache is already empty; replan required"
		return result
	}

	result.ExecutionPhase = agentexec.HostStorageCleanupPhaseClean
	result.MutationStarted = true
	clean := m.run(ctx, []string{"DEBIAN_FRONTEND=noninteractive"}, "apt-get", "clean")
	if clean.err != nil || clean.exitCode != 0 {
		result.After = m.snapshotLocked(true)
		result.Verification = agentexec.HostStorageCleanupVerificationFailed
		result.Error = "package cache cleanup failed"
		return result
	}

	result.Success = true
	result.ExecutionPhase = agentexec.HostStorageCleanupPhaseVerify
	after := m.snapshotLocked(true)
	result.After = after
	if after.Error != "" {
		result.Error = "package cache cleanup completed but verification was inconclusive"
		return result
	}
	if before.ReclaimableBytes > after.ReclaimableBytes {
		result.ReclaimedBytes = before.ReclaimableBytes - after.ReclaimableBytes
	}
	if result.ReclaimedBytes <= 0 {
		result.Verification = agentexec.HostStorageCleanupVerificationFailed
		result.Error = "package cache cleanup completed but no space was reclaimed"
		return result
	}
	result.Verification = agentexec.HostStorageCleanupVerificationVerified
	result.ExecutionPhase = agentexec.HostStorageCleanupPhaseComplete
	return result
}

func scanAPTPackageCache() (agentexec.HostStorageCleanupSnapshot, error) {
	return scanAPTPackageCacheAt(aptPackageCachePath)
}

func scanAPTPackageCacheAt(cachePath string) (agentexec.HostStorageCleanupSnapshot, error) {
	hasher := sha256.New()
	entries := 0
	var total int64
	err := filepath.WalkDir(cachePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".deb") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() < 0 {
			return nil
		}
		entries++
		if entries > storageCleanupMaxCacheEntries {
			return fmt.Errorf("package cache entry count exceeds bounded limit")
		}
		if info.Size() > agentexec.HostStorageCleanupMaxReportedBytes-total {
			return fmt.Errorf("package cache exceeds bounded size")
		}
		rel, err := filepath.Rel(cachePath, path)
		if err != nil {
			return err
		}
		total += info.Size()
		_, _ = fmt.Fprintf(hasher, "%s\x00%d\x00%d\n", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return agentexec.HostStorageCleanupSnapshot{}, err
	}
	return agentexec.HostStorageCleanupSnapshot{
		Fingerprint:      "sha256:" + hex.EncodeToString(hasher.Sum(nil)),
		ReclaimableBytes: total,
	}, nil
}

func (m *storageCleanupManager) currentTime() time.Time {
	if m != nil && m.now != nil {
		return m.now().UTC()
	}
	return time.Now().UTC()
}

func (m *storageCleanupManager) storeSnapshot(snapshot agentexec.HostStorageCleanupSnapshot) {
	copy := snapshot
	m.cached = &copy
}
