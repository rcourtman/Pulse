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
)

type fakePrivilegedUpdate struct {
	root       string
	events     []string
	activation agenthelper.UpdateResult
}

func (f *fakePrivilegedUpdate) CreateQuarantinedArtifact() (string, *os.File, func(), error) {
	f.events = append(f.events, "quarantine")
	dir := filepath.Join(f.root, "pulse-agent-0123456789abcdef0123456789abcdef")
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", nil, func() {}, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "pulse-agent"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	return filepath.Base(dir), file, func() { _ = os.RemoveAll(dir) }, err
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
	f.activation = agenthelper.UpdateResult{Action: "activated", ActivationID: "pulse-agent-0123456789abcdef0123456789abcdef:0123456789abcdef", ActiveSHA256: digest, RollbackSHA256: strings.Repeat("a", 64)}
	return f.activation, nil
}

func (f *fakePrivilegedUpdate) Rollback(_ context.Context, activation agenthelper.UpdateResult) (agenthelper.UpdateResult, error) {
	f.events = append(f.events, "rollback")
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
	u := New(Config{PrivilegedUpdate: helper, Disabled: true})
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
	want := []string{"quarantine", "self-test", "signature:signed-update", "stage", "activate", "rollback"}
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
	u := New(Config{PrivilegedUpdate: helper, Disabled: true})
	u.selfTestFn = func(context.Context, string) error { return nil }

	err := u.performPrivilegedUpdate(context.Background(), "/usr/local/bin/pulse-agent", bytes.NewReader(binary), int64(len(binary)), strings.Repeat("0", 64), "signed-update")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("performPrivilegedUpdate error = %v", err)
	}
	if !reflect.DeepEqual(helper.events, []string{"quarantine"}) {
		t.Fatalf("helper events = %#v, want quarantine only", helper.events)
	}
}
