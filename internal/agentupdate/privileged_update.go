package agentupdate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
)

const privilegedUpdateQuarantineDir = "/var/lib/pulse-agent/update-quarantine"

// PrivilegedUpdate is the updater's closed view of helper-backed activation.
// Implementations cannot select a target path, staging path, command, or URL.
type PrivilegedUpdate interface {
	CreateQuarantinedArtifact() (artifactID string, file *os.File, cleanup func(), err error)
	WriteQuarantinedSignature(artifactID, signature string) error
	Stage(context.Context, string, string) (agenthelper.UpdateStageResult, error)
	Activate(context.Context, string, string) (agenthelper.UpdateResult, error)
	Rollback(context.Context, agenthelper.UpdateResult) (agenthelper.UpdateResult, error)
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

func (p *privilegeHelperUpdate) CreateQuarantinedArtifact() (string, *os.File, func(), error) {
	if err := validateCollectorQuarantineRoot(p.quarantineDir); err != nil {
		return "", nil, func() {}, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", nil, func() {}, fmt.Errorf("create update artifact identity: %w", err)
	}
	artifactID := "pulse-agent-" + hex.EncodeToString(nonce[:])
	artifactDir := filepath.Join(p.quarantineDir, artifactID)
	if err := os.Mkdir(artifactDir, 0o700); err != nil {
		return "", nil, func() {}, fmt.Errorf("create quarantined artifact directory: %w", err)
	}
	cleanup := func() {
		_ = os.Remove(filepath.Join(artifactDir, "pulse-agent"))
		_ = os.Remove(filepath.Join(artifactDir, "pulse-agent.sig"))
		_ = os.Remove(artifactDir)
	}
	file, err := os.OpenFile(filepath.Join(artifactDir, "pulse-agent"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		cleanup()
		return "", nil, func() {}, fmt.Errorf("create quarantined agent binary: %w", err)
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

func syncUpdateDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
