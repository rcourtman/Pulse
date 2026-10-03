package audit_test

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

// These controls exercise the recovery guide's key/evidence distinctions with
// the real signer and encryption manager. They use only synthetic, private
// temporary directories: no SQLite store, installed instance or external call.
func TestSignerRecoveryGuidance(t *testing.T) {
	dataDir := t.TempDir()
	writeEncryptionKey := func(dir string, value byte) *crypto.CryptoManager {
		t.Helper()
		key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{value}, 32))
		if err := os.WriteFile(filepath.Join(dir, ".encryption.key"), []byte(key), 0o600); err != nil {
			t.Fatalf("write synthetic encryption key: %v", err)
		}
		manager, err := crypto.NewCryptoManagerAt(dir)
		if err != nil {
			t.Fatalf("load synthetic encryption manager: %v", err)
		}
		return manager
	}
	manager := writeEncryptionKey(dataDir, 'A')
	auditDir := filepath.Join(dataDir, "audit")
	signer, err := audit.NewSigner(auditDir, manager)
	if err != nil {
		t.Fatalf("create original signer: %v", err)
	}
	event := audit.Event{
		ID:        "synthetic-recovery-evidence",
		Timestamp: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		EventType: "login",
		User:      "synthetic-user",
		Path:      "/api/login",
		Success:   true,
		Details:   "synthetic recovery control",
	}
	event.Signature = signer.Sign(event)
	if !signer.Verify(event) {
		t.Fatal("original event must verify before recovery comparisons")
	}
	storedKey, err := os.ReadFile(filepath.Join(auditDir, ".audit-signing.key"))
	if err != nil {
		t.Fatalf("read synthetic encrypted key: %v", err)
	}

	t.Run("reload preserves verification without regeneration", func(t *testing.T) {
		reloaded, err := audit.NewSigner(auditDir, manager)
		if err != nil || !reloaded.Verify(event) {
			t.Fatalf("reload failed to verify unchanged event: %v", err)
		}
		currentKey, err := os.ReadFile(filepath.Join(auditDir, ".audit-signing.key"))
		if err != nil || !bytes.Equal(storedKey, currentKey) {
			t.Fatal("reload must not replace the stored signing key")
		}
	})

	t.Run("new signing key fails unchanged historical event", func(t *testing.T) {
		freshSigner, err := audit.NewSigner(filepath.Join(t.TempDir(), "audit"), manager)
		if err != nil {
			t.Fatalf("create unrelated signer: %v", err)
		}
		if freshSigner.Verify(event) {
			t.Fatal("a new key must not verify an unchanged old event")
		}
		if !signer.Verify(event) {
			t.Fatal("failed check with another key must not alter the original evidence")
		}
	})

	t.Run("signing key alone cannot restore with different encryption key", func(t *testing.T) {
		restoreDir := t.TempDir()
		otherManager := writeEncryptionKey(restoreDir, 'B')
		restoreAuditDir := filepath.Join(restoreDir, "audit")
		if err := os.MkdirAll(restoreAuditDir, 0o700); err != nil {
			t.Fatalf("create isolated restore directory: %v", err)
		}
		keyPath := filepath.Join(restoreAuditDir, ".audit-signing.key")
		if err := os.WriteFile(keyPath, storedKey, 0o600); err != nil {
			t.Fatalf("copy synthetic encrypted key: %v", err)
		}
		if _, err := audit.NewSigner(restoreAuditDir, otherManager); err == nil {
			t.Fatal("wrong encryption key must not silently recover the signer")
		}
		after, err := os.ReadFile(keyPath)
		if err != nil || !bytes.Equal(after, storedKey) {
			t.Fatal("a failed restore must preserve the supplied encrypted key")
		}
	})

	t.Run("matching key pair verifies in isolated restore", func(t *testing.T) {
		restoreDir := t.TempDir()
		restoredManager := writeEncryptionKey(restoreDir, 'A')
		restoreAuditDir := filepath.Join(restoreDir, "audit")
		if err := os.MkdirAll(restoreAuditDir, 0o700); err != nil {
			t.Fatalf("create isolated restore directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(restoreAuditDir, ".audit-signing.key"), storedKey, 0o600); err != nil {
			t.Fatalf("copy synthetic encrypted key: %v", err)
		}
		restoredSigner, err := audit.NewSigner(restoreAuditDir, restoredManager)
		if err != nil || !restoredSigner.Verify(event) {
			t.Fatalf("matching isolated key pair failed to verify old event: %v", err)
		}
		if !signer.Verify(event) {
			t.Fatal("isolated restore must leave the original verifier usable")
		}
	})

	t.Run("unsigned is not retrospectively authenticated", func(t *testing.T) {
		unsigned := event
		unsigned.Signature = ""
		if signer.Verify(unsigned) {
			t.Fatal("working signer must not authenticate an unsigned old event")
		}
	})

	t.Run("altered data and unsupported format remain failures", func(t *testing.T) {
		altered := event
		altered.Details = "altered synthetic evidence"
		if signer.Verify(altered) {
			t.Fatal("changed signed content must still fail verification")
		}
		unsupported := event
		unsupported.Signature = "v99:" + event.Signature
		if signer.Verify(unsupported) {
			t.Fatal("unsupported signature format must not be accepted")
		}
		if !signer.Verify(event) {
			t.Fatal("negative comparisons must preserve the original evidence")
		}
	})
}
