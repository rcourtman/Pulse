package agentupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

type fakePrivilegedUpdate struct {
	root        string
	events      []string
	activation  agenthelper.UpdateResult
	rollbackErr error
}

func (f *fakePrivilegedUpdate) CreateQuarantinedArtifact() (string, *os.File, func() error, error) {
	f.events = append(f.events, "quarantine")
	dir := filepath.Join(f.root, "pulse-agent-0123456789abcdef0123456789abcdef")
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", nil, func() error { return nil }, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "pulse-agent"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	return filepath.Base(dir), file, func() error {
		f.events = append(f.events, "cleanup")
		return os.RemoveAll(dir)
	}, err
}

func (f *fakePrivilegedUpdate) WriteQuarantinedSignature(_ string, signature string) error {
	f.events = append(f.events, "signature:"+signature)
	return nil
}

func (f *fakePrivilegedUpdate) Stage(_ context.Context, artifactID, digest string) (agenthelper.UpdateStageResult, error) {
	f.events = append(f.events, "stage")
	return agenthelper.UpdateStageResult{Action: "staged", ArtifactID: artifactID, SHA256: digest, DurableAt: time.Now()}, nil
}

func (f *fakePrivilegedUpdate) Activate(_ context.Context, _ string, digest string) (agenthelper.UpdateResult, error) {
	f.events = append(f.events, "activate")
	f.activation = agenthelper.UpdateResult{Action: "pending", ActivationID: "pulse-agent-0123456789abcdef0123456789abcdef:0123456789abcdef", ActiveSHA256: digest, RollbackSHA256: strings.Repeat("a", 64), RollbackDeadline: time.Now().Add(time.Minute)}
	return f.activation, nil
}

func (f *fakePrivilegedUpdate) Commit(_ context.Context, activation agenthelper.UpdateResult) (agenthelper.UpdateResult, error) {
	f.events = append(f.events, "commit")
	return agenthelper.UpdateResult{Action: "committed", ActivationID: activation.ActivationID, ActiveSHA256: activation.ActiveSHA256, RollbackSHA256: activation.RollbackSHA256}, nil
}

func (f *fakePrivilegedUpdate) Rollback(_ context.Context, activation agenthelper.UpdateResult) (agenthelper.UpdateResult, error) {
	f.events = append(f.events, "rollback")
	if f.rollbackErr != nil {
		return agenthelper.UpdateResult{}, f.rollbackErr
	}
	if activation != f.activation {
		return agenthelper.UpdateResult{}, errors.New("wrong activation identity")
	}
	return agenthelper.UpdateResult{Action: "rolled_back", ActivationID: activation.ActivationID, ActiveSHA256: activation.RollbackSHA256, RollbackSHA256: activation.ActiveSHA256}, nil
}

func TestPrivilegedUpdateStagesActivatesAndRollsBackRestartFailure(t *testing.T) {
	originalGOOS := runtimeGOOS
	originalRestart := restartProcessFn
	t.Cleanup(func() {
		runtimeGOOS = originalGOOS
		restartProcessFn = originalRestart
	})
	runtimeGOOS = goOSLinux
	restartProcessFn = func(string) error { return errors.New("exec refused") }

	binary := append([]byte{0x7f, 'E', 'L', 'F'}, bytes.Repeat([]byte("x"), 128)...)
	sum := sha256.Sum256(binary)
	digest := hex.EncodeToString(sum[:])
	helper := &fakePrivilegedUpdate{root: t.TempDir()}
	if err := securityutil.HardenPrivatePath(helper.root, 0o700); err != nil {
		t.Fatal(err)
	}
	u := New(Config{PrivilegedUpdate: helper, Disabled: true, StateDir: helper.root, CurrentVersion: "1.0.0"})
	u.selfTestFn = func(_ context.Context, path string) error {
		helper.events = append(helper.events, "self-test")
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, binary) {
			t.Fatalf("self-test bytes mismatch: err=%v", err)
		}
		return nil
	}

	err := u.performPrivilegedUpdate(context.Background(), "/usr/local/bin/pulse-agent", bytes.NewReader(binary), int64(len(binary)), digest, "signed-update")
	if err == nil || !strings.Contains(err.Error(), "update rolled back") {
		t.Fatalf("performPrivilegedUpdate error = %v", err)
	}
	want := []string{"quarantine", "self-test", "signature:signed-update", "stage", "activate", "cleanup", "rollback"}
	if !reflect.DeepEqual(helper.events, want) {
		t.Fatalf("helper events = %#v, want %#v", helper.events, want)
	}
}

func TestPrivilegedUpdateFailsClosedBeforeActivation(t *testing.T) {
	originalGOOS := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = originalGOOS })
	runtimeGOOS = goOSLinux
	binary := append([]byte{0x7f, 'E', 'L', 'F'}, []byte("update")...)
	helper := &fakePrivilegedUpdate{root: t.TempDir()}
	if err := securityutil.HardenPrivatePath(helper.root, 0o700); err != nil {
		t.Fatal(err)
	}
	u := New(Config{PrivilegedUpdate: helper, Disabled: true, StateDir: helper.root, CurrentVersion: "1.0.0"})
	u.selfTestFn = func(context.Context, string) error { return nil }

	err := u.performPrivilegedUpdate(context.Background(), "/usr/local/bin/pulse-agent", bytes.NewReader(binary), int64(len(binary)), strings.Repeat("0", 64), "signed-update")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("performPrivilegedUpdate error = %v", err)
	}
	if !reflect.DeepEqual(helper.events, []string{"quarantine", "cleanup"}) {
		t.Fatalf("helper events = %#v, want quarantine cleanup", helper.events)
	}
}

func TestPendingPrivilegedUpdateHandoffIsDurableAndStrict(t *testing.T) {
	stateDir := t.TempDir()
	if err := securityutil.HardenPrivatePath(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	activation := agenthelper.UpdateResult{
		Action:           "pending",
		ActivationID:     "pulse-agent-0123456789abcdef0123456789abcdef:0123456789abcdef",
		ActiveSHA256:     strings.Repeat("a", 64),
		RollbackSHA256:   strings.Repeat("b", 64),
		RollbackDeadline: time.Now().Add(time.Minute).UTC(),
		DurableAt:        time.Now().UTC(),
	}
	if err := PersistPendingPrivilegedUpdate(stateDir, " 1.0.0 ", activation); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, pendingPrivilegedUpdateFile)
	if info, err := os.Lstat(path); err != nil {
		t.Fatalf("inspect handoff: %v", err)
	} else if err := securityutil.ValidatePrivatePath(path, info); err != nil {
		t.Fatalf("handoff is not private: %v", err)
	}
	loaded, err := LoadPendingPrivilegedUpdate(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.PreviousVersion != "1.0.0" || loaded.Activation != activation {
		t.Fatalf("loaded handoff = %#v", loaded)
	}
	if err := ClearPendingPrivilegedUpdate(stateDir); err != nil {
		t.Fatal(err)
	}
	if loaded, err := LoadPendingPrivilegedUpdate(stateDir); err != nil || loaded != nil {
		t.Fatalf("cleared handoff = %#v, %v", loaded, err)
	}
}

func TestPendingPrivilegedUpdateHandoffRejectsUnsafeState(t *testing.T) {
	stateDir := t.TempDir()
	if err := securityutil.HardenPrivatePath(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, pendingPrivilegedUpdateFile)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte(`{"activation":{},"previousVersion":"1.0.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPendingPrivilegedUpdate(stateDir); err == nil {
		t.Fatal("symlinked pending handoff accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	unsafe := `{"activation":{"action":"pending","activationId":"pulse-agent-0123456789abcdef0123456789abcdef:0123456789abcdef","activeSha256":"` + strings.Repeat("a", 64) + `","rollbackSha256":"` + strings.Repeat("b", 64) + `","rollbackDeadline":"2030-01-01T00:00:00Z"},"previousVersion":"1.0.0","unexpected":true}`
	if err := os.WriteFile(path, []byte(unsafe), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPendingPrivilegedUpdate(stateDir); err == nil {
		t.Fatal("unknown handoff field accepted")
	}
}
