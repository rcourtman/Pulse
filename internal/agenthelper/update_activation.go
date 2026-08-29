package agenthelper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxUpdateArtifactBytes = 100 * 1024 * 1024

type UpdateActivateRequest struct {
	ArtifactID string `json:"artifactId"`
	SHA256     string `json:"sha256"`
}

type UpdateStageRequest struct {
	ArtifactID string `json:"artifactId"`
	SHA256     string `json:"sha256"`
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

type UpdateResult struct {
	Action         string    `json:"action"`
	ActivationID   string    `json:"activationId"`
	ActiveSHA256   string    `json:"activeSha256"`
	RollbackSHA256 string    `json:"rollbackSha256"`
	DurableAt      time.Time `json:"durableAt"`
}

type UpdateActivatorConfig struct {
	QuarantineDir           string
	StagingDir              string
	TargetPath              string
	StatePath               string
	VerifySignature         func([]byte, string) error
	ValidateOwner           func(*os.File) error
	ValidateQuarantineOwner func(*os.File) error
	Now                     func() time.Time
}

type updateActivator struct {
	mu                      sync.Mutex
	stagingDir              string
	quarantineDir           string
	targetPath              string
	rollbackPath            string
	statePath               string
	verifySignature         func([]byte, string) error
	validateOwner           func(*os.File) error
	validateQuarantineOwner func(*os.File) error
	now                     func() time.Time
}

type durableUpdateState struct {
	Action         string    `json:"action"`
	ActivationID   string    `json:"activationId"`
	ActiveSHA256   string    `json:"activeSha256"`
	RollbackSHA256 string    `json:"rollbackSha256"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func NewUpdateActivator(config UpdateActivatorConfig) (UpdateProvider, error) {
	if config.VerifySignature == nil || config.ValidateOwner == nil || config.ValidateQuarantineOwner == nil {
		return nil, errors.New("signature, root-ownership, and quarantine-ownership validators are required")
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
	return &updateActivator{
		quarantineDir: config.QuarantineDir, stagingDir: config.StagingDir, targetPath: config.TargetPath,
		rollbackPath: config.TargetPath + ".last-known-good", statePath: config.StatePath,
		verifySignature: config.VerifySignature, validateOwner: config.ValidateOwner,
		validateQuarantineOwner: config.ValidateQuarantineOwner, now: now,
	}, nil
}

// Stage promotes exactly one signed collector-downloaded artifact from the
// fixed quarantine into the helper's fixed root-owned staging tree. The
// request carries identity only: no path, command, URL, or copy destination is
// caller-controlled.
func (u *updateActivator) Stage(ctx context.Context, request UpdateStageRequest) (UpdateStageResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !validArtifactID(request.ArtifactID) || !validSHA256(request.SHA256) {
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
	return UpdateStageResult{Action: "staged", ArtifactID: request.ArtifactID, SHA256: actual, DurableAt: u.now().UTC()}, nil
}

func (u *updateActivator) Activate(ctx context.Context, request UpdateActivateRequest) (UpdateResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !validArtifactID(request.ArtifactID) || !validSHA256(request.SHA256) {
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
	activationID := request.ArtifactID + ":" + actual[:16]
	if state, err := u.readState(); err == nil && state.ActivationID == activationID {
		if state.Action != "activated" || !strings.EqualFold(state.ActiveSHA256, actual) {
			return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "artifact identity has already completed a different update transition"}
		}
		current, currentErr := u.readOwnedBounded(u.targetPath, maxUpdateArtifactBytes)
		rollback, rollbackErr := u.readOwnedBounded(u.rollbackPath, maxUpdateArtifactBytes)
		if currentErr != nil || rollbackErr != nil || !strings.EqualFold(sha256Hex(current), state.ActiveSHA256) || !strings.EqualFold(sha256Hex(rollback), state.RollbackSHA256) {
			return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "durable activation no longer matches installed binaries"}
		}
		return UpdateResult{Action: state.Action, ActivationID: state.ActivationID, ActiveSHA256: state.ActiveSHA256, RollbackSHA256: state.RollbackSHA256, DurableAt: state.UpdatedAt}, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "durable update state is invalid"}
	}
	if err := ctx.Err(); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorDeadlineExceeded, Message: "update activation deadline exceeded", Retryable: true}
	}
	current, err := u.readOwnedBounded(u.targetPath, maxUpdateArtifactBytes)
	if err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "installed agent binary is unavailable or unsafe"}
	}
	rollbackDigest := sha256Hex(current)
	if err := u.installDurably(artifact, current); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "activate staged agent binary"}
	}
	result := UpdateResult{
		Action: "activated", ActivationID: activationID,
		ActiveSHA256: actual, RollbackSHA256: rollbackDigest, DurableAt: u.now().UTC(),
	}
	if err := u.writeState(result); err != nil {
		// Restore the pre-activation binary if the durable receipt cannot
		// commit. A failed operation must not leave an unjournaled activation.
		_ = u.installDurably(current, artifact)
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "persist activation result"}
	}
	return result, nil
}

func (u *updateActivator) Rollback(ctx context.Context, request UpdateRollbackRequest) (UpdateResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !validActivationID(request.ActivationID) || !validSHA256(request.CurrentSHA256) || !validSHA256(request.RollbackSHA256) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "rollback identity is invalid"}
	}
	state, err := u.readState()
	if err != nil || state.Action != "activated" || state.ActivationID != request.ActivationID ||
		!strings.EqualFold(state.ActiveSHA256, request.CurrentSHA256) ||
		!strings.EqualFold(state.RollbackSHA256, request.RollbackSHA256) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "rollback identity does not match the durable activation"}
	}
	current, err := u.readOwnedBounded(u.targetPath, maxUpdateArtifactBytes)
	if err != nil || !strings.EqualFold(sha256Hex(current), state.ActiveSHA256) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "installed agent identity changed after activation"}
	}
	rollback, err := u.readOwnedBounded(u.rollbackPath, maxUpdateArtifactBytes)
	if err != nil || !strings.EqualFold(sha256Hex(rollback), state.RollbackSHA256) {
		return UpdateResult{}, &ProviderError{Code: ErrorStateConflict, Message: "last-known-good identity changed after activation"}
	}
	if err := ctx.Err(); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorDeadlineExceeded, Message: "update rollback deadline exceeded", Retryable: true}
	}
	if err := u.installDurably(rollback, current); err != nil {
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "restore last-known-good agent binary"}
	}
	result := UpdateResult{
		Action: "rolled_back", ActivationID: state.ActivationID,
		ActiveSHA256: state.RollbackSHA256, RollbackSHA256: state.ActiveSHA256, DurableAt: u.now().UTC(),
	}
	if err := u.writeState(result); err != nil {
		_ = u.installDurably(current, rollback)
		return UpdateResult{}, &ProviderError{Code: ErrorInternal, Message: "persist rollback result"}
	}
	return result, nil
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
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, errors.New("unsafe file type")
	}
	file, err := openFileNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err := validate(file); err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || !after.Mode().IsRegular() || after.Mode().Perm()&0o022 != 0 {
		return nil, errors.New("file identity or ownership changed")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("file exceeds bounded size")
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
	binaryTemp, err := writeSyncedTemp(tempDir, ".pulse-agent-*", artifact, 0o600)
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

func (u *updateActivator) writeState(result UpdateResult) error {
	state := durableUpdateState{Action: result.Action, ActivationID: result.ActivationID, ActiveSHA256: result.ActiveSHA256, RollbackSHA256: result.RollbackSHA256, UpdatedAt: result.DurableAt}
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
