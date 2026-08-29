package agenthelper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testUpdateActivator(t *testing.T, verifier func([]byte, string) error) (UpdateProvider, string, string, string) {
	t.Helper()
	root := t.TempDir()
	quarantine := filepath.Join(root, "quarantine")
	staging := filepath.Join(root, "staging")
	targetDir := filepath.Join(root, "bin")
	stateDir := filepath.Join(root, "state")
	for _, path := range []string{quarantine, staging, targetDir, stateDir} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(targetDir, "pulse-agent")
	if err := os.WriteFile(target, testELF("old-signed-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	provider, err := NewUpdateActivator(UpdateActivatorConfig{
		QuarantineDir: quarantine, StagingDir: staging, TargetPath: target, StatePath: filepath.Join(stateDir, "activation.json"),
		VerifySignature:         verifier,
		ValidateOwner:           func(*os.File) error { return nil },
		ValidateQuarantineOwner: func(*os.File) error { return nil },
		Now:                     func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	return provider, quarantine, target, filepath.Join(stateDir, "activation.json")
}

func testELF(body string) []byte {
	return append([]byte{0x7f, 'E', 'L', 'F'}, []byte(body)...)
}

func stageUpdate(t *testing.T, staging, identity string, binary []byte) string {
	t.Helper()
	dir := filepath.Join(staging, identity)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pulse-agent"), binary, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pulse-agent.sig"), []byte("valid-signature"), 0o600); err != nil {
		t.Fatal(err)
	}
	return sha256Hex(binary)
}

func promoteUpdate(t *testing.T, provider UpdateProvider, artifactID, digest string) {
	t.Helper()
	result, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: artifactID, SHA256: digest})
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if result.Action != "staged" || result.ArtifactID != artifactID || result.SHA256 != digest {
		t.Fatalf("stage result = %#v", result)
	}
}

func TestUpdateActivationAndIdentityBoundRollbackAreDurable(t *testing.T) {
	provider, staging, target, state := testUpdateActivator(t, func(data []byte, signature string) error {
		if string(data) != string(testELF("new-signed-binary")) || signature != "valid-signature" {
			return errors.New("signature mismatch")
		}
		return nil
	})
	newDigest := stageUpdate(t, staging, "release-1", testELF("new-signed-binary"))
	promoteUpdate(t, provider, "release-1", newDigest)
	oldDigest := sha256Hex(testELF("old-signed-binary"))
	activated, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-1", SHA256: newDigest})
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if activated.ActiveSHA256 != newDigest || activated.RollbackSHA256 != oldDigest || activated.ActivationID == "" {
		t.Fatalf("activation result = %#v", activated)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("new-signed-binary")) {
		t.Fatalf("installed binary = %q", installed)
	}
	if info, err := os.Stat(state); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("durable state mode=%v err=%v", info, err)
	}
	retried, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-1", SHA256: newDigest})
	if err != nil || retried != activated {
		t.Fatalf("idempotent activation retry = %#v err=%v, want %#v", retried, err, activated)
	}
	rolledBack, err := provider.Rollback(context.Background(), UpdateRollbackRequest{
		ActivationID: activated.ActivationID, CurrentSHA256: newDigest, RollbackSHA256: oldDigest,
	})
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolledBack.ActiveSHA256 != oldDigest || rolledBack.RollbackSHA256 != newDigest {
		t.Fatalf("rollback result = %#v", rolledBack)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("old-signed-binary")) {
		t.Fatalf("rolled-back binary = %q", installed)
	}
	if _, err := provider.Rollback(context.Background(), UpdateRollbackRequest{
		ActivationID: rolledBack.ActivationID, CurrentSHA256: rolledBack.ActiveSHA256, RollbackSHA256: rolledBack.RollbackSHA256,
	}); err == nil {
		t.Fatal("completed rollback replay reactivated the rejected binary")
	}
}

func TestUpdateActivationRejectsInvalidSignatureAndExecutable(t *testing.T) {
	provider, staging, _, _ := testUpdateActivator(t, func([]byte, string) error { return errors.New("untrusted") })
	digest := stageUpdate(t, staging, "bad-signature", testELF("binary"))
	if _, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: "bad-signature", SHA256: digest}); err == nil {
		t.Fatal("invalid artifact signature accepted")
	}

	provider, staging, _, _ = testUpdateActivator(t, func([]byte, string) error { return nil })
	digest = stageUpdate(t, staging, "not-elf", []byte("signed but not executable"))
	if _, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: "not-elf", SHA256: digest}); err == nil {
		t.Fatal("non-ELF artifact accepted")
	}
}

func TestUpdateActivationRejectsTraversalSymlinksAndDigestMismatch(t *testing.T) {
	provider, staging, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	validDigest := stageUpdate(t, staging, "release-1", testELF("new"))
	for _, request := range []UpdateActivateRequest{
		{ArtifactID: "../release-1", SHA256: validDigest},
		{ArtifactID: "release/1", SHA256: validDigest},
		{ArtifactID: "release-1", SHA256: strings.Repeat("0", 64)},
	} {
		if _, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: request.ArtifactID, SHA256: request.SHA256}); err == nil {
			t.Fatalf("unsafe activation accepted: %#v", request)
		}
	}
	outside := filepath.Join(filepath.Dir(staging), "outside")
	if err := os.WriteFile(outside, []byte("attacker"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkDir := filepath.Join(staging, "symlink-release")
	if err := os.Mkdir(symlinkDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(symlinkDir, "pulse-agent")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(symlinkDir, "pulse-agent.sig"), []byte("valid-signature"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Stage(context.Background(), UpdateStageRequest{ArtifactID: "symlink-release", SHA256: sha256Hex([]byte("attacker"))}); err == nil {
		t.Fatal("symlink artifact accepted")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}
	promoteUpdate(t, provider, "release-1", validDigest)
	if _, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-1", SHA256: validDigest}); err == nil {
		t.Fatal("symlink install target accepted")
	}
}

func TestUpdateActivationUsesOpenedBytesAcrossStagingSwap(t *testing.T) {
	var stagingBinary, target string
	provider, staging, resolvedTarget, _ := testUpdateActivator(t, func(data []byte, _ string) error {
		if err := os.Remove(stagingBinary); err != nil {
			return err
		}
		if err := os.Symlink(target, stagingBinary); err != nil {
			return err
		}
		if string(data) != string(testELF("verified-bytes")) {
			return errors.New("unexpected verified data")
		}
		return nil
	})
	target = resolvedTarget
	digest := stageUpdate(t, staging, "release-swap", testELF("verified-bytes"))
	stagingBinary = filepath.Join(staging, "release-swap", "pulse-agent")
	promoteUpdate(t, provider, "release-swap", digest)
	if _, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-swap", SHA256: digest}); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if installed, _ := os.ReadFile(target); string(installed) != string(testELF("verified-bytes")) {
		t.Fatalf("symlink swap changed activated bytes: %q", installed)
	}
}

func TestUpdateRollbackRejectsWrongIdentityAndChangedBinary(t *testing.T) {
	provider, staging, target, _ := testUpdateActivator(t, func([]byte, string) error { return nil })
	digest := stageUpdate(t, staging, "release-1", testELF("new"))
	promoteUpdate(t, provider, "release-1", digest)
	result, err := provider.Activate(context.Background(), UpdateActivateRequest{ArtifactID: "release-1", SHA256: digest})
	if err != nil {
		t.Fatal(err)
	}
	bad := UpdateRollbackRequest{ActivationID: "release-2:" + result.ActivationID[len(result.ActivationID)-16:], CurrentSHA256: result.ActiveSHA256, RollbackSHA256: result.RollbackSHA256}
	if _, err := provider.Rollback(context.Background(), bad); err == nil {
		t.Fatal("wrong rollback activation identity accepted")
	}
	if err := os.WriteFile(target, []byte("changed-after-activation"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Rollback(context.Background(), UpdateRollbackRequest{ActivationID: result.ActivationID, CurrentSHA256: result.ActiveSHA256, RollbackSHA256: result.RollbackSHA256}); err == nil {
		t.Fatal("rollback accepted changed active binary")
	}
}
