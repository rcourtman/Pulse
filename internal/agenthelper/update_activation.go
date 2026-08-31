package agenthelper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

const maxUpdateArtifactBytes = 100 * 1024 * 1024

const (
	defaultUpdateRollbackWindow = 2 * time.Minute
	minUpdateRollbackWindow     = 5 * time.Second
	maxUpdateRollbackWindow     = 10 * time.Minute
	initialRollbackRetryDelay   = 5 * time.Second
	maxRollbackRetryDelay       = time.Minute
)

type UpdateRecoveryFailure struct {
	ActivationID string
	Error        string
	RetryIn      time.Duration
}

type UpdateActivateRequest struct {
	ArtifactID string `json:"artifactId"`
	SHA256     string `json:"sha256"`
	Version    string `json:"version"`
}

type UpdateStageRequest struct {
	ArtifactID string `json:"artifactId"`
	SHA256     string `json:"sha256"`
	Version    string `json:"version"`
}

type UpdateStageResult struct {
	Action     string    `json:"action"`
	ArtifactID string    `json:"artifactId"`
	SHA256     string    `json:"sha256"`
	DurableAt  time.Time `json:"durableAt"`
}

type UpdateRollbackRequest struct {
	ActivationID   string `json:"activationId"`
	CurrentSHA256  string `json:"currentSha256"`
	RollbackSHA256 string `json:"rollbackSha256"`
}

type UpdateCommitRequest struct {
	ActivationID  string `json:"activationId"`
	CurrentSHA256 string `json:"currentSha256"`
}

type UpdateResult struct {
	Action           string    `json:"action"`
	ActivationID     string    `json:"activationId"`
	ActiveSHA256     string    `json:"activeSha256"`
	RollbackSHA256   string    `json:"rollbackSha256"`
	RollbackDeadline time.Time `json:"rollbackDeadline,omitempty"`
	DurableAt        time.Time `json:"durableAt"`
}

type UpdateActivatorConfig struct {
	QuarantineDir           string
	StagingDir              string
	TargetPath              string
	StatePath               string
	VerifySignature         func([]byte, string) error
	InspectVersion          func(context.Context, string) (string, error)
	ValidateCommitter       func(context.Context, string) error
	ValidateOwner           func(*os.File) error
	ValidateQuarantineOwner func(*os.File) error
	Now                     func() time.Time
	RollbackWindow          time.Duration
	ScheduleRollback        func(time.Duration, func()) func()
	ReportRecoveryFailure   func(UpdateRecoveryFailure)
}

type updateActivator struct {
	mu                      sync.Mutex
	stagingDir              string
	quarantineDir           string
	targetPath              string
	rollbackPath            string
	statePath               string
	verifySignature         func([]byte, string) error
	inspectVersion          func(context.Context, string) (string, error)
	validateCommitter       func(context.Context, string) error
	validateOwner           func(*os.File) error
	validateQuarantineOwner func(*os.File) error
	now                     func() time.Time
	rollbackWindow          time.Duration
	scheduleRollback        func(time.Duration, func()) func()
	reportRecoveryFailure   func(UpdateRecoveryFailure)
	cancelRollback          func()
	rollbackRetryDelay      time.Duration
	rollbackTimerGeneration uint64
}

type durableUpdateState struct {
	Action           string    `json:"action"`
	ActivationID     string    `json:"activationId"`
	ActiveSHA256     string    `json:"activeSha256"`
	RollbackSHA256   string    `json:"rollbackSha256"`
	RollbackDeadline time.Time `json:"rollbackDeadline,omitempty"`
	UpdatedAt        time.Time `json:"updatedAt"`
	ActivatorPID     int32     `json:"activatorPid,omitempty"`
}

func NewUpdateActivator(config UpdateActivatorConfig) (UpdateProvider, error) {
	if config.VerifySignature == nil || config.InspectVersion == nil || config.ValidateCommitter == nil || config.ValidateOwner == nil || config.ValidateQuarantineOwner == nil {
		return nil, errors.New("signature, artifact-version, committer, root-ownership, and quarantine-ownership validators are required")
	}
	for _, path := range []string{config.QuarantineDir, config.StagingDir, config.TargetPath, config.StatePath} {
		if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return nil, errors.New("update helper paths must be clean and absolute")
		}
	}
	if config.QuarantineDir == config.StagingDir || strings.HasPrefix(config.QuarantineDir, config.StagingDir+string(os.PathSeparator)) || strings.HasPrefix(config.StagingDir, config.QuarantineDir+string(os.PathSeparator)) {
		return nil, errors.New("update quarantine and staging must be separate")
	}
	if filepath.Dir(config.TargetPath) == config.StagingDir || strings.HasPrefix(config.TargetPath, config.StagingDir+string(os.PathSeparator)) {
		return nil, errors.New("update target must be outside staging")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	rollbackWindow := config.RollbackWindow
	if rollbackWindow == 0 {
		rollbackWindow = defaultUpdateRollbackWindow
	}
	if rollbackWindow < minUpdateRollbackWindow || rollbackWindow > maxUpdateRollbackWindow {
		return nil, errors.New("update rollback window must be between 5 seconds and 10 minutes")
	}
	scheduleRollback := config.ScheduleRollback
	if scheduleRollback == nil {
		scheduleRollback = func(delay time.Duration, callback func()) func() {
			timer := time.AfterFunc(delay, callback)
			return func() { timer.Stop() }
		}
	}
	reportRecoveryFailure := config.ReportRecoveryFailure
	if reportRecoveryFailure == nil {
		reportRecoveryFailure = func(UpdateRecoveryFailure) {}
	}
	activator := &updateActivator{
		quarantineDir: config.QuarantineDir, stagingDir: config.StagingDir, targetPath: config.TargetPath,
		rollbackPath: config.TargetPath + ".last-known-good", statePath: config.StatePath,
		verifySignature: config.VerifySignature, inspectVersion: config.InspectVersion, validateCommitter: config.ValidateCommitter, validateOwner: config.ValidateOwner,
		validateQuarantineOwner: config.ValidateQuarantineOwner, now: now,
		rollbackWindow: rollbackWindow, scheduleRollback: scheduleRollback, reportRecoveryFailure: reportRecoveryFailure,
	}
	if err := activator.recoverUncommittedLocked(); err != nil {
		return nil, err
	}
	return activator, nil
}

// Stage promotes exactly one signed collector-downloaded artifact from the
// fixed quarantine into the helper's fixed root-owned staging tree. The
// request carries identity only: no path, command, URL, or copy destination is
// caller-controlled.
func (u *updateActivator) Stage(ctx context.Context, request UpdateStageRequest) (UpdateStageResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	request.Version = strings.TrimSpace(request.Version)
	if !validArtifactID(request.ArtifactID) || !validSHA256(request.SHA256) || !validArtifactVersion(request.Version) {
		return UpdateStageResult{}, invalidArtifact("artifact identity or digest is invalid")
	}
	if err := u.validateDirectoryWith(u.quarantineDir, u.validateQuarantineOwner); err != nil {
		return UpdateStageResult{}, invalidArtifact("update quarantine root is invalid")
	}
	sourceDir := filepath.Join(u.quarantineDir, request.ArtifactID)
	if err := u.validateDirectoryWith(sourceDir, u.validateQuarantineOwner); err != nil {
		return UpdateStageResult{}, invalidArtifact("quarantined artifact directory is invalid")
	}
	artifact, err := u.readOwnedBoundedWith(filepath.Join(sourceDir, "pulse-agent"), maxUpdateArtifactBytes, u.validateQuarantineOwner)
	if err != nil {
		return UpdateStageResult{}, invalidArtifact("quarantined artifact is unavailable or unsafe")
	}
	if len(artifact) < 4 || artifact[0] != 0x7f || string(artifact[1:4]) != "ELF" {
		return UpdateStageResult{}, invalidArtifact("quarantined artifact is not a Linux executable")
	}
	signature, err := u.readOwnedBoundedWith(filepath.Join(sourceDir, "pulse-agent.sig"), 4096, u.validateQuarantineOwner)
	if err != nil {
		return UpdateStageResult{}, invalidArtifact("quarantined signature is unavailable or unsafe")
	}
	actual := sha256Hex(artifact)
	if !strings.EqualFold(actual, request.SHA256) {
		return UpdateStageResult{}, invalidArtifact("quarantined artifact digest does not match request")
	}
	if err := u.verifySignature(artifact, strings.TrimSpace(string(signature))); err != nil {
		return UpdateStageResult{}, invalidArtifact("quarantined artifact signature is invalid")
	}
	if err := ctx.Err(); err != nil {
		return UpdateStageResult{}, &ProviderError{Code: ErrorDeadlineExceeded, Message: "update staging deadline exceeded", Retryable: true}
	}
	if err := u.validateDirectory(u.stagingDir); err != nil {
		return UpdateStageResult{}, invalidArtifact("update staging root is invalid")
	}
	destination := filepath.Join(u.stagingDir, request.ArtifactID)
	if err := u.installStagedArtifact(destination, artifact, signature); err != nil {
		return UpdateStageResult{}, &ProviderError{Code: ErrorInternal, Message: "promote quarantined artifact into root staging"}
	}
	removeInvalidStaging := func(message string) (UpdateStageResult, error) {
		_ = u.removeStagedArtifact(request.ArtifactID)
		return UpdateStageResult{}, invalidArtifact(message)
	}
	candidateVersion, err := u.inspectVersion(ctx, filepath.Join(destination, "pulse-agent"))
	if err != nil || strings.TrimSpace(candidateVersion) != request.Version {
		return removeInvalidStaging("signed artifact is not the requested pulse-agent version")
	}
	currentVersion, err := u.inspectVersion(ctx, u.targetPath)
	if err != nil || !validArtifactVersion(strings.TrimSpace(currentVersion)) {
		return removeInvalidStaging("installed pulse-agent version is unavailable")
	}
	if utils.CompareVersions(utils.NormalizeVersion(request.Version), utils.NormalizeVersion(strings.TrimSpace(currentVersion))) <= 0 {
		return removeInvalidStaging("signed artifact does not advance the installed pulse-agent version")
	}
	return UpdateStageResult{Action: "staged", ArtifactID: request.ArtifactID, SHA256: actual, DurableAt: u.now().UTC()}, nil
}

func validArtifactVersion(version string) bool {
	return version != "" && version != "dev" && len(version) <= 128 && !strings.ContainsAny(version, "\x00\r\n")
}

func (u *updateActivator) Activate(ctx context.Context, request UpdateActivateRequest) (UpdateResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	request.Version = strings.TrimSpace(request.Version)
	if !validArtifactID(request.ArtifactID) || !validSHA256(request.SHA256) || !validArtifactVersion(request.Version) {
		return UpdateResult{}, invalidArtifact("artifact identity or digest is invalid")
	}
	if err := u.validateDirectory(u.stagingDir); err != nil {
		return UpdateResult{}, invalidArtifact("update staging root is invalid")
	}
	artifactDir := filepath.Join(u.stagingDir, request.ArtifactID)
	if err := u.validateDirectory(artifactDir); err != nil {
		return UpdateResult{}, invalidArtifact("staged artifact directory is invalid")
	}
	artifact, err := u.readOwnedBounded(filepath.Join(artifactDir, "pulse-agent"), maxUpdateArtifactBytes)
	if err != nil {
		return UpdateResult{}, invalidArtifact("staged artifact is unavailable or unsafe")
	}
	if len(artifact) < 4 || artifact[0] != 0x7f || string(artifact[1:4]) != "ELF" {
		return UpdateResult{}, invalidArtifact("staged artifact is not a Linux executable")
	}
	signatureBytes, err := u.readOwnedBounded(filepath.Join(artifactDir, "pulse-agent.sig"), 4096)
	if err != nil {
		return UpdateResult{}, invalidArtifact("staged signature is unavailable or unsafe")
	}
	actual := sha256Hex(artifact)
	if !strings.EqualFold(actual, request.SHA256) {
		return UpdateResult{}, invalidArtifact("staged artifact digest does not match request")
	}
	if err := u.verifySignature(artifact, strings.TrimSpace(string(signatureBytes))); err != nil {
		return UpdateResult{}, invalidArtifact("staged artifact signature is invalid")
	}
	candidateVersion, err := u.inspectVersion(ctx, filepath.Join(artifactDir, "pulse-agent"))
	if err != nil || strings.TrimSpace(candidateVersion) != request.Version {
		return UpdateResult{}, invalidArtifact("staged artifact is not the requested pulse-agent version")
	}
	activationID := request.ArtifactID + ":" + actual[:16]
	if state, err := u.readState(); err == nil {
		if state.Action == "preparing" || state.Action == "pending" {
			if state.ActivationID != activationID {
				return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "another agent update is still awaiting commit or rollback"}
			}
			if state.Action == "preparing" {
				if err := u.recoverStateLocked(state); err != nil {
					return UpdateResult{}, err
				}
				return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "interrupted activation was rolled back and cannot be replayed"}
			}
			if !strings.EqualFold(state.ActiveSHA256, actual) {
				return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "artifact identity has already completed a different update transition"}
			}
			if err := u.validateInstalledState(state); err != nil {
				return UpdateResult{}, err
			}
			u.schedulePendingRollbackLocked(state)
			return updateResultFromState(state), nil
		}
		if state.ActivationID == activationID {
			return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "artifact identity has already completed its update transition"}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "durable update state is invalid"}
	}
	if err := ctx.Err(); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorDeadlineExceeded, Message: "update activation deadline exceeded", Retryable: true}
	}
	current, err := u.readOwnedBounded(u.targetPath, maxUpdateArtifactBytes)
	if err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "installed agent binary is unavailable or unsafe"}
	}
	currentVersion, err := u.inspectVersion(ctx, u.targetPath)
	if err != nil || !validArtifactVersion(strings.TrimSpace(currentVersion)) ||
		utils.CompareVersions(utils.NormalizeVersion(request.Version), utils.NormalizeVersion(strings.TrimSpace(currentVersion))) <= 0 {
		return UpdateResult{}, invalidArtifact("staged artifact does not advance the installed pulse-agent version")
	}
	rollbackDigest := sha256Hex(current)
	deadline := u.now().UTC().Add(u.rollbackWindow)
	peer, _ := PeerFromContext(ctx)
	preparing := durableUpdateState{
		Action: "preparing", ActivationID: activationID, ActiveSHA256: actual,
		RollbackSHA256: rollbackDigest, RollbackDeadline: deadline, UpdatedAt: u.now().UTC(), ActivatorPID: peer.PID,
	}
	if err := u.writeDurableState(preparing); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "persist activation intent"}
	}
	if err := u.installDurably(artifact, current); err != nil {
		_ = u.recoverStateLocked(preparing)
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "activate staged agent binary"}
	}
	pending := durableUpdateState{
		Action: "pending", ActivationID: activationID,
		ActiveSHA256: actual, RollbackSHA256: rollbackDigest,
		RollbackDeadline: deadline, UpdatedAt: u.now().UTC(), ActivatorPID: peer.PID,
	}
	if err := u.writeDurableState(pending); err != nil {
		if recoveryErr := u.recoverStateLocked(preparing); recoveryErr != nil {
			return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "persist pending activation and restore last-known-good binary"}
		}
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "persist pending activation"}
	}
	u.schedulePendingRollbackLocked(pending)
	return updateResultFromState(pending), nil
}

func (u *updateActivator) Commit(ctx context.Context, request UpdateCommitRequest) (UpdateResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !validActivationID(request.ActivationID) || !validSHA256(request.CurrentSHA256) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "commit identity is invalid"}
	}
	state, err := u.readState()
	if err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "commit identity does not match a durable activation"}
	}
	if state.Action == "committed" && state.ActivationID == request.ActivationID && strings.EqualFold(state.ActiveSHA256, request.CurrentSHA256) {
		return updateResultFromState(state), nil
	}
	if state.Action != "pending" || state.ActivationID != request.ActivationID || !strings.EqualFold(state.ActiveSHA256, request.CurrentSHA256) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "commit identity does not match the pending activation"}
	}
	if !u.now().UTC().Before(state.RollbackDeadline) {
		if err := u.recoverStateLocked(state); err != nil {
			return UpdateResult{}, err
		}
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "activation deadline expired and the update was rolled back"}
	}
	if err := ctx.Err(); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorDeadlineExceeded, Message: "update commit deadline exceeded", Retryable: true}
	}
	if peer, ok := PeerFromContext(ctx); state.ActivatorPID > 0 && (!ok || peer.PID != state.ActivatorPID) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "commit must come from the activated collector process"}
	}
	if err := u.validateCommitter(ctx, state.ActiveSHA256); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "commit caller is not the activated collector binary"}
	}
	if err := u.validateInstalledState(state); err != nil {
		return UpdateResult{}, err
	}
	if err := u.removeStagedArtifact(artifactIDFromActivation(state.ActivationID)); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "remove committed update staging"}
	}
	pending := state
	state.Action = "committed"
	state.RollbackDeadline = time.Time{}
	state.UpdatedAt = u.now().UTC()
	if err := u.writeDurableState(state); err != nil {
		if recoveryErr := u.recoverStateLocked(pending); recoveryErr != nil {
			return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "persist update commit and restore last-known-good binary"}
		}
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "persist update commit"}
	}
	u.cancelPendingRollbackLocked()
	return updateResultFromState(state), nil
}

func (u *updateActivator) Rollback(ctx context.Context, request UpdateRollbackRequest) (UpdateResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !validActivationID(request.ActivationID) || !validSHA256(request.CurrentSHA256) || !validSHA256(request.RollbackSHA256) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "rollback identity is invalid"}
	}
	state, err := u.readState()
	if err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "rollback identity does not match a durable activation"}
	}
	if state.Action == "rolled_back" && state.ActivationID == request.ActivationID &&
		strings.EqualFold(state.RollbackSHA256, request.CurrentSHA256) && strings.EqualFold(state.ActiveSHA256, request.RollbackSHA256) {
		return updateResultFromState(state), nil
	}
	if (state.Action != "pending" && state.Action != "preparing") || state.ActivationID != request.ActivationID ||
		!strings.EqualFold(state.ActiveSHA256, request.CurrentSHA256) ||
		!strings.EqualFold(state.RollbackSHA256, request.RollbackSHA256) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "rollback identity does not match the pending activation"}
	}
	if err := ctx.Err(); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorDeadlineExceeded, Message: "update rollback deadline exceeded", Retryable: true}
	}
	if err := u.recoverStateLocked(state); err != nil {
		return UpdateResult{}, err
	}
	rolledBack, err := u.readState()
	if err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "read durable rollback result"}
	}
	return updateResultFromState(rolledBack), nil
}

func (u *updateActivator) recoverUncommittedLocked() error {
	state, err := u.readState()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &ProviderError{Code: ErrorStateConflict, Message: "durable update state is invalid"}
	}
	if state.Action != "preparing" && state.Action != "pending" {
		return nil
	}
	return u.recoverStateLocked(state)
}

func (u *updateActivator) recoverStateLocked(state durableUpdateState) error {
	if state.Action != "preparing" && state.Action != "pending" {
		return &ProviderError{Code: ErrorStateConflict, Message: "update state is not recoverable"}
	}
	current, err := u.readOwnedBounded(u.targetPath, maxUpdateArtifactBytes)
	if err != nil {
		return &ProviderError{Code: ErrorStateConflict, Message: "installed agent binary is unavailable during recovery"}
	}
	currentDigest := sha256Hex(current)
	if strings.EqualFold(currentDigest, state.ActiveSHA256) {
		rollback, rollbackErr := u.readOwnedBounded(u.rollbackPath, maxUpdateArtifactBytes)
		if rollbackErr != nil || !strings.EqualFold(sha256Hex(rollback), state.RollbackSHA256) {
			return &ProviderError{Code: ErrorStateConflict, Message: "last-known-good binary is unavailable during recovery"}
		}
		if err := u.installDurably(rollback, current); err != nil {
			return &ProviderError{Code: ErrorInternal, Message: "restore last-known-good agent binary"}
		}
	} else if !strings.EqualFold(currentDigest, state.RollbackSHA256) {
		return &ProviderError{Code: ErrorStateConflict, Message: "installed agent identity changed during pending activation"}
	}

	rolledBack := durableUpdateState{
		Action: "rolled_back", ActivationID: state.ActivationID,
		ActiveSHA256: state.RollbackSHA256, RollbackSHA256: state.ActiveSHA256,
		UpdatedAt: u.now().UTC(),
	}
	// Keep the durable state recoverable until the staged payload has also
	// been removed. A crash anywhere before the terminal state write will
	// replay this same bounded rollback and cleanup on helper startup.
	if err := u.removeStagedArtifact(artifactIDFromActivation(state.ActivationID)); err != nil {
		return &ProviderError{Code: ErrorInternal, Message: "remove rolled-back update staging"}
	}
	if err := u.writeDurableState(rolledBack); err != nil {
		return &ProviderError{Code: ErrorInternal, Message: "persist rollback result"}
	}
	u.cancelPendingRollbackLocked()
	return nil
}

func (u *updateActivator) validateInstalledState(state durableUpdateState) error {
	current, currentErr := u.readOwnedBounded(u.targetPath, maxUpdateArtifactBytes)
	rollback, rollbackErr := u.readOwnedBounded(u.rollbackPath, maxUpdateArtifactBytes)
	if currentErr != nil || rollbackErr != nil ||
		!strings.EqualFold(sha256Hex(current), state.ActiveSHA256) ||
		!strings.EqualFold(sha256Hex(rollback), state.RollbackSHA256) {
		return &ProviderError{Code: ErrorStateConflict, Message: "durable activation no longer matches installed binaries"}
	}
	return nil
}

func (u *updateActivator) schedulePendingRollbackLocked(state durableUpdateState) {
	u.cancelPendingRollbackLocked()
	delay := state.RollbackDeadline.Sub(u.now().UTC())
	if delay < 0 {
		delay = 0
	}
	u.schedulePendingRollbackAttemptLocked(state, delay)
}

func (u *updateActivator) schedulePendingRollbackAttemptLocked(state durableUpdateState, delay time.Duration) {
	u.rollbackTimerGeneration++
	generation := u.rollbackTimerGeneration
	u.cancelRollback = u.scheduleRollback(delay, func() {
		u.mu.Lock()
		failure := u.runPendingRollbackAttemptLocked(state, generation)
		u.mu.Unlock()
		if failure != nil {
			u.reportRecoveryFailure(*failure)
		}
	})
}

func (u *updateActivator) runPendingRollbackAttemptLocked(state durableUpdateState, generation uint64) *UpdateRecoveryFailure {
	if generation != u.rollbackTimerGeneration {
		return nil
	}
	// Consume this generation before touching durable state. A stopped timer
	// whose callback was already queued cannot clear or replace a newer timer.
	u.rollbackTimerGeneration++
	u.cancelRollback = nil
	current, err := u.readState()
	if err != nil {
		failure := u.schedulePendingRollbackRetryLocked(state, fmt.Errorf("read durable update state: %w", err))
		return &failure
	}
	if current.Action != "pending" || current.ActivationID != state.ActivationID {
		u.rollbackRetryDelay = 0
		return nil
	}
	if err := u.recoverStateLocked(current); err != nil {
		failure := u.schedulePendingRollbackRetryLocked(current, err)
		return &failure
	}
	return nil
}

func (u *updateActivator) schedulePendingRollbackRetryLocked(state durableUpdateState, recoveryErr error) UpdateRecoveryFailure {
	delay := initialRollbackRetryDelay
	if u.rollbackRetryDelay > 0 {
		delay = u.rollbackRetryDelay * 2
		if delay > maxRollbackRetryDelay {
			delay = maxRollbackRetryDelay
		}
	}
	u.rollbackRetryDelay = delay
	failure := UpdateRecoveryFailure{
		ActivationID: state.ActivationID,
		Error:        recoveryErr.Error(),
		RetryIn:      delay,
	}
	u.schedulePendingRollbackAttemptLocked(state, delay)
	return failure
}

func (u *updateActivator) cancelPendingRollbackLocked() {
	u.rollbackTimerGeneration++
	if u.cancelRollback != nil {
		u.cancelRollback()
		u.cancelRollback = nil
	}
	u.rollbackRetryDelay = 0
}

func updateResultFromState(state durableUpdateState) UpdateResult {
	return UpdateResult{
		Action: state.Action, ActivationID: state.ActivationID,
		ActiveSHA256: state.ActiveSHA256, RollbackSHA256: state.RollbackSHA256,
		RollbackDeadline: state.RollbackDeadline, DurableAt: state.UpdatedAt,
	}
}

func artifactIDFromActivation(activationID string) string {
	parts := strings.SplitN(activationID, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

func (u *updateActivator) removeStagedArtifact(artifactID string) error {
	if !validArtifactID(artifactID) {
		return errors.New("invalid staged artifact identity")
	}
	destination := filepath.Join(u.stagingDir, artifactID)
	if _, err := os.Lstat(destination); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := u.validateDirectory(destination); err != nil {
		return err
	}
	for _, name := range []string{"pulse-agent", "pulse-agent.sig"} {
		path := filepath.Join(destination, name)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("unsafe staged artifact cleanup target")
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	if err := os.Remove(destination); err != nil {
		return err
	}
	return syncDirectory(u.stagingDir)
}

func (u *updateActivator) validateDirectory(path string) error {
	return u.validateDirectoryWith(path, u.validateOwner)
}

func (u *updateActivator) validateDirectoryWith(path string, validate func(*os.File) error) error {
	file, err := openFileNoFollow(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := validate(file); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o022 != 0 {
		return errors.New("directory is not root-owned and immutable to non-root callers")
	}
	return nil
}

func (u *updateActivator) readOwnedBounded(path string, limit int64) ([]byte, error) {
	return u.readOwnedBoundedWith(path, limit, u.validateOwner)
}

func (u *updateActivator) readOwnedBoundedWith(path string, limit int64, validate func(*os.File) error) ([]byte, error) {
	file, err := openFileNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err := validate(file); err != nil {
		return nil, err
	}
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Mode().Perm()&0o022 != 0 || before.Size() < 0 || before.Size() > limit {
		return nil, errors.New("file descriptor is not a bounded private regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("file exceeds bounded size")
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || after.Mode().Perm()&0o022 != 0 || after.Size() != before.Size() || int64(len(data)) != after.Size() {
		return nil, errors.New("file descriptor changed while being read")
	}
	return data, nil
}

func (u *updateActivator) installStagedArtifact(destination string, artifact, signature []byte) error {
	if info, err := os.Lstat(destination); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("unsafe staged artifact destination")
		}
		if err := u.validateDirectory(destination); err != nil {
			return err
		}
		current, binaryErr := u.readOwnedBounded(filepath.Join(destination, "pulse-agent"), maxUpdateArtifactBytes)
		currentSignature, signatureErr := u.readOwnedBounded(filepath.Join(destination, "pulse-agent.sig"), 4096)
		if binaryErr == nil && signatureErr == nil && strings.EqualFold(sha256Hex(current), sha256Hex(artifact)) && string(currentSignature) == string(signature) {
			return nil
		}
		return errors.New("staged artifact identity conflict")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tempDir, err := os.MkdirTemp(u.stagingDir, ".pulse-agent-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	if err := os.Chmod(tempDir, 0o700); err != nil {
		return err
	}
	// Root-owned staging is executable only so the helper can perform the fixed
	// pulse-agent identity/version probe. The collector cannot modify this tree.
	binaryTemp, err := writeSyncedTemp(tempDir, ".pulse-agent-*", artifact, 0o700)
	if err != nil {
		return err
	}
	if err := os.Rename(binaryTemp, filepath.Join(tempDir, "pulse-agent")); err != nil {
		return err
	}
	signatureTemp, err := writeSyncedTemp(tempDir, ".pulse-agent.sig-*", signature, 0o600)
	if err != nil {
		return err
	}
	if err := os.Rename(signatureTemp, filepath.Join(tempDir, "pulse-agent.sig")); err != nil {
		return err
	}
	if err := syncDirectory(tempDir); err != nil {
		return err
	}
	if err := os.Rename(tempDir, destination); err != nil {
		return err
	}
	return syncDirectory(u.stagingDir)
}

func (u *updateActivator) installDurably(active, lastKnownGood []byte) error {
	targetDir := filepath.Dir(u.targetPath)
	if err := u.validateDirectory(targetDir); err != nil {
		return err
	}
	rollbackTemp, err := writeSyncedTemp(targetDir, ".pulse-agent-lkg-*", lastKnownGood, 0o755)
	if err != nil {
		return err
	}
	defer os.Remove(rollbackTemp)
	activeTemp, err := writeSyncedTemp(targetDir, ".pulse-agent-active-*", active, 0o755)
	if err != nil {
		return err
	}
	defer os.Remove(activeTemp)
	if err := os.Rename(rollbackTemp, u.rollbackPath); err != nil {
		return err
	}
	if err := syncDirectory(targetDir); err != nil {
		return err
	}
	if err := os.Rename(activeTemp, u.targetPath); err != nil {
		return err
	}
	return syncDirectory(targetDir)
}

func (u *updateActivator) writeDurableState(state durableUpdateState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	dir := filepath.Dir(u.statePath)
	if err := u.validateDirectory(dir); err != nil {
		return err
	}
	temp, err := writeSyncedTemp(dir, ".pulse-agent-update-state-*", data, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	if err := os.Rename(temp, u.statePath); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func (u *updateActivator) readState() (durableUpdateState, error) {
	data, err := u.readOwnedBounded(u.statePath, 16*1024)
	if err != nil {
		return durableUpdateState{}, err
	}
	var state durableUpdateState
	if err := decodeStrict(data, &state); err != nil {
		return durableUpdateState{}, err
	}
	return state, nil
}

func writeSyncedTemp(dir, pattern string, data []byte, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if err := file.Chmod(mode); err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validArtifactID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			continue
		}
		return false
	}
	return value != "." && value != ".." && !strings.Contains(value, "..")
}

func validActivationID(value string) bool {
	parts := strings.Split(value, ":")
	return len(parts) == 2 && validArtifactID(parts[0]) && len(parts[1]) == 16 && isLowerHex(parts[1])
}

func validSHA256(value string) bool { return len(value) == 64 && isLowerHex(strings.ToLower(value)) }

func isLowerHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func invalidArtifact(message string) error {
	return &ProviderError{Code: ErrorArtifactInvalid, Message: message}
}

var _ UpdateProvider = (*updateActivator)(nil)
