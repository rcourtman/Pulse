package agentupdate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

const privilegedUpdateQuarantineDir = "/var/lib/pulse-agent/update-quarantine"
const pendingPrivilegedUpdateFile = ".pulse-agent-update-pending.json"

// PrivilegedUpdate is the updater's closed view of helper-backed activation.
// Implementations cannot select a target path, staging path, command, or URL.
type PrivilegedUpdate interface {
	CreateQuarantinedArtifact() (artifactID string, file *os.File, cleanup func() error, err error)
	WriteQuarantinedSignature(artifactID, signature string) error
	Stage(context.Context, string, string) (agenthelper.UpdateStageResult, error)
	Activate(context.Context, string, string) (agenthelper.UpdateResult, error)
	Commit(context.Context, agenthelper.UpdateResult) (agenthelper.UpdateResult, error)
	Rollback(context.Context, agenthelper.UpdateResult) (agenthelper.UpdateResult, error)
}

type PendingPrivilegedUpdate struct {
	Activation      agenthelper.UpdateResult `json:"activation"`
	PreviousVersion string                   `json:"previousVersion"`
}

type privilegeHelperUpdate struct {
	client        *agenthelper.Client
	quarantineDir string
}

func NewPrivilegeHelperUpdate(socketPath string) (PrivilegedUpdate, error) {
	client, err := agenthelper.NewClient(agenthelper.ClientConfig{SocketPath: socketPath, MaxDeadline: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	return &privilegeHelperUpdate{client: client, quarantineDir: privilegedUpdateQuarantineDir}, nil
}

func (p *privilegeHelperUpdate) CreateQuarantinedArtifact() (string, *os.File, func() error, error) {
	if err := validateCollectorQuarantineRoot(p.quarantineDir); err != nil {
		return "", nil, func() error { return nil }, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", nil, func() error { return nil }, fmt.Errorf("create update artifact identity: %w", err)
	}
	artifactID := "pulse-agent-" + hex.EncodeToString(nonce[:])
	artifactDir := filepath.Join(p.quarantineDir, artifactID)
	if err := os.Mkdir(artifactDir, 0o700); err != nil {
		return "", nil, func() error { return nil }, fmt.Errorf("create quarantined artifact directory: %w", err)
	}
	cleanup := func() error {
		var cleanupErr error
		for _, name := range []string{"pulse-agent", "pulse-agent.sig"} {
			if err := os.Remove(filepath.Join(artifactDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErr = errors.Join(cleanupErr, err)
			}
		}
		if err := os.Remove(artifactDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		if err := syncUpdateDirectory(p.quarantineDir); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		return cleanupErr
	}
	file, err := os.OpenFile(filepath.Join(artifactDir, "pulse-agent"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		_ = cleanup()
		return "", nil, func() error { return nil }, fmt.Errorf("create quarantined agent binary: %w", err)
	}
	return artifactID, file, cleanup, nil
}

func (p *privilegeHelperUpdate) WriteQuarantinedSignature(artifactID, signature string) error {
	if !validPrivilegedArtifactID(artifactID) {
		return errors.New("invalid quarantined artifact identity")
	}
	signature = strings.TrimSpace(signature)
	if signature == "" || len(signature) > 4096 {
		return errors.New("invalid quarantined update signature")
	}
	path := filepath.Join(p.quarantineDir, artifactID, "pulse-agent.sig")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create quarantined update signature: %w", err)
	}
	if _, err := file.WriteString(signature + "\n"); err != nil {
		_ = file.Close()
		return fmt.Errorf("write quarantined update signature: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync quarantined update signature: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close quarantined update signature: %w", err)
	}
	return syncUpdateDirectory(filepath.Dir(path))
}

func (p *privilegeHelperUpdate) Stage(ctx context.Context, artifactID, digest string) (agenthelper.UpdateStageResult, error) {
	var result agenthelper.UpdateStageResult
	_, err := p.client.Call(ctx, agenthelper.OperationAgentUpdateStage, agenthelper.OperationVersion1, 30*time.Second, agenthelper.UpdateStageRequest{ArtifactID: artifactID, SHA256: digest}, &result)
	return result, err
}

func (p *privilegeHelperUpdate) Activate(ctx context.Context, artifactID, digest string) (agenthelper.UpdateResult, error) {
	var result agenthelper.UpdateResult
	_, err := p.client.Call(ctx, agenthelper.OperationAgentUpdateActivate, agenthelper.OperationVersion1, 30*time.Second, agenthelper.UpdateActivateRequest{ArtifactID: artifactID, SHA256: digest}, &result)
	return result, err
}

func (p *privilegeHelperUpdate) Commit(ctx context.Context, activation agenthelper.UpdateResult) (agenthelper.UpdateResult, error) {
	var result agenthelper.UpdateResult
	_, err := p.client.Call(ctx, agenthelper.OperationAgentUpdateCommit, agenthelper.OperationVersion1, 30*time.Second, agenthelper.UpdateCommitRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256,
	}, &result)
	return result, err
}

func (p *privilegeHelperUpdate) Rollback(ctx context.Context, activation agenthelper.UpdateResult) (agenthelper.UpdateResult, error) {
	var result agenthelper.UpdateResult
	_, err := p.client.Call(ctx, agenthelper.OperationAgentUpdateRollback, agenthelper.OperationVersion1, 30*time.Second, agenthelper.UpdateRollbackRequest{
		ActivationID: activation.ActivationID, CurrentSHA256: activation.ActiveSHA256, RollbackSHA256: activation.RollbackSHA256,
	}, &result)
	return result, err
}

func validateCollectorQuarantineRoot(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("collector update quarantine path is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect collector update quarantine: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return errors.New("collector update quarantine must be a real 0700 directory")
	}
	if !collectorQuarantineOwnedByCurrentUID(info) {
		return errors.New("collector update quarantine must be owned by the collector UID")
	}
	return nil
}

func validPrivilegedArtifactID(value string) bool {
	if !strings.HasPrefix(value, "pulse-agent-") || len(value) != len("pulse-agent-")+32 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "pulse-agent-"))
	return err == nil
}

func PersistPendingPrivilegedUpdate(stateDir, previousVersion string, activation agenthelper.UpdateResult) error {
	path, err := pendingPrivilegedUpdatePath(stateDir)
	if err != nil {
		return err
	}
	if err := securityutil.HardenPrivatePath(stateDir, 0o700); err != nil {
		return fmt.Errorf("harden pending update state directory: %w", err)
	}
	if err := validatePendingUpdateStateDir(stateDir); err != nil {
		return err
	}
	previousVersion = strings.TrimSpace(previousVersion)
	if previousVersion == "" || len(previousVersion) > 128 || strings.ContainsAny(previousVersion, "\x00\r\n") {
		return errors.New("previous update version is invalid")
	}
	if !validPendingActivation(activation) {
		return errors.New("pending helper activation is invalid")
	}
	data, err := json.Marshal(PendingPrivilegedUpdate{Activation: activation, PreviousVersion: previousVersion})
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(stateDir, ".pulse-agent-update-pending-*")
	if err != nil {
		return fmt.Errorf("create pending update handoff: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := securityutil.HardenPrivatePath(tempPath, 0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := replacePendingUpdateFile(tempPath, path); err != nil {
		return err
	}
	return syncUpdateDirectory(stateDir)
}

func LoadPendingPrivilegedUpdate(stateDir string) (*PendingPrivilegedUpdate, error) {
	path, err := pendingPrivilegedUpdatePath(stateDir)
	if err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := validatePendingUpdateStateDir(stateDir); err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, errors.New("pending update handoff is not a private regular file")
	}
	if err := securityutil.ValidatePrivatePath(path, before); err != nil {
		return nil, fmt.Errorf("pending update handoff is not private: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, errors.New("pending update handoff identity changed")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var pending PendingPrivilegedUpdate
	if err := decoder.Decode(&pending); err != nil {
		return nil, fmt.Errorf("decode pending update handoff: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("decode pending update handoff trailing data: %w", err)
		}
		return nil, errors.New("pending update handoff contains trailing JSON")
	}
	if !validPendingActivation(pending.Activation) || strings.TrimSpace(pending.PreviousVersion) == "" {
		return nil, errors.New("pending update handoff is invalid")
	}
	return &pending, nil
}

func ClearPendingPrivilegedUpdate(stateDir string) error {
	path, err := pendingPrivilegedUpdatePath(stateDir)
	if err != nil {
		return err
	}
	if err := validatePendingUpdateStateDir(stateDir); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncUpdateDirectory(stateDir)
}

func pendingPrivilegedUpdatePath(stateDir string) (string, error) {
	if stateDir == "" || !filepath.IsAbs(stateDir) || filepath.Clean(stateDir) != stateDir {
		return "", errors.New("pending update state directory must be clean and absolute")
	}
	return filepath.Join(stateDir, pendingPrivilegedUpdateFile), nil
}

func validatePendingUpdateStateDir(stateDir string) error {
	info, err := os.Lstat(stateDir)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("pending update state directory must be a private real directory")
	}
	if err := securityutil.ValidatePrivatePath(stateDir, info); err != nil {
		return fmt.Errorf("pending update state directory must be private: %w", err)
	}
	return nil
}

func validPendingActivation(activation agenthelper.UpdateResult) bool {
	parts := strings.Split(activation.ActivationID, ":")
	if activation.Action != "pending" || len(parts) != 2 || !validPrivilegedArtifactID(parts[0]) || len(parts[1]) != 16 {
		return false
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return false
	}
	for _, digest := range []string{activation.ActiveSHA256, activation.RollbackSHA256} {
		if len(digest) != 64 {
			return false
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return false
		}
	}
	return !activation.RollbackDeadline.IsZero()
}
