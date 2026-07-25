package audit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- crypto doubles for the NewSigner error arms ---

// failingEncryptCrypto fails both Encrypt and Decrypt; used to hit the
// "failed to encrypt audit signing key" arm on the generate-new-key path.
type failingEncryptCrypto struct{}

func (failingEncryptCrypto) Encrypt([]byte) ([]byte, error) {
	return nil, errors.New("encrypt unavailable")
}

func (failingEncryptCrypto) Decrypt([]byte) ([]byte, error) {
	return nil, errors.New("decrypt unavailable")
}

// decryptOnlyCrypto mirrors taggedMockCryptoManager's Decrypt (requires the
// "enc:" prefix) so a plaintext key file triggers migration, but its Encrypt
// always fails. This is the only way to reach the
// "failed to encrypt migrated audit signing key" arm.
type decryptOnlyCrypto struct{}

func (decryptOnlyCrypto) Encrypt([]byte) ([]byte, error) {
	return nil, errors.New("encrypt unavailable during migration")
}

func (decryptOnlyCrypto) Decrypt(b []byte) ([]byte, error) {
	if len(b) < 4 || string(b[:4]) != "enc:" {
		return nil, os.ErrInvalid
	}
	return append([]byte(nil), b[4:]...), nil
}

// --- NewSigner: uncovered error arms of the load-and-generate state machine ---

// TestBranchcov0724pmNewSigner_LoadKeyError hits the raw `return nil, err`
// arm by feeding a key file that neither decrypts nor matches the legacy
// 32-/64-byte plaintext shapes.
func TestBranchcov0724pmNewSigner_LoadKeyError(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, ".audit-signing.key")
	// 12 bytes: not decryptable by taggedMockCryptoManager (no "enc:" prefix),
	// not 32 bytes, not 64 bytes -> loadAuditSigningKey returns an error.
	if err := os.WriteFile(keyPath, []byte("garbage_key"), 0o600); err != nil {
		t.Fatalf("seed key file: %v", err)
	}

	signer, err := NewSigner(dir, taggedMockCryptoManager{})
	if err == nil {
		t.Fatalf("expected load error, got signer=%v", signer)
	}
	if !strings.Contains(err.Error(), "failed to decrypt audit signing key") {
		t.Fatalf("unexpected error text: %v", err)
	}
	if signer != nil {
		t.Fatalf("expected nil signer on error, got %v", signer)
	}
}

// TestBranchcov0724pmNewSigner_ShortDecryptedKey hits the
// "invalid audit signing key length" arm: the file decrypts successfully but
// yields fewer than 32 bytes.
func TestBranchcov0724pmNewSigner_ShortDecryptedKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, ".audit-signing.key")
	// "enc:" + "short" decrypts (via taggedMockCryptoManager) to a 5-byte key.
	if err := os.WriteFile(keyPath, []byte("enc:short"), 0o600); err != nil {
		t.Fatalf("seed short key file: %v", err)
	}

	signer, err := NewSigner(dir, taggedMockCryptoManager{})
	if err == nil {
		t.Fatalf("expected length error, got signer=%v", signer)
	}
	if !strings.Contains(err.Error(), "invalid audit signing key length") {
		t.Fatalf("unexpected error text: %v", err)
	}
	if !strings.Contains(err.Error(), "got 5") {
		t.Fatalf("error should report decoded length 5: %v", err)
	}
}

// TestBranchcov0724pmNewSigner_MigrationEncryptError reaches the
// "failed to encrypt migrated audit signing key" arm: a plaintext 32-byte key
// triggers migration, but Encrypt then fails.
func TestBranchcov0724pmNewSigner_MigrationEncryptError(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, ".audit-signing.key")
	plaintext := []byte("0123456789abcdef0123456789abcdef") // exactly 32 bytes
	if err := os.WriteFile(keyPath, plaintext, 0o600); err != nil {
		t.Fatalf("seed plaintext key: %v", err)
	}

	_, err := NewSigner(dir, decryptOnlyCrypto{})
	if err == nil {
		t.Fatal("expected migration encrypt error")
	}
	if !strings.Contains(err.Error(), "failed to encrypt migrated audit signing key") {
		t.Fatalf("unexpected error text: %v", err)
	}

	// The original plaintext file must be untouched when encryption fails
	// (migration must not corrupt the source on the failure path).
	got, readErr := os.ReadFile(keyPath)
	if readErr != nil {
		t.Fatalf("re-read key: %v", readErr)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("plaintext key corrupted on encrypt-failure path: %q", got)
	}
}

// TestBranchcov0724pmNewSigner_MigrationWriteError reaches the
// "failed to rewrite audit signing key" arm by making an otherwise-migratable
// key file unwritable. Skipped when running as root (root bypasses file mode).
func TestBranchcov0724pmNewSigner_MigrationWriteError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("migration WriteFile failure cannot be triggered as root")
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, ".audit-signing.key")
	// 32-byte plaintext -> migration triggers with the (succeeding)
	// taggedMockCryptoManager.Encrypt; the subsequent rewrite hits the
	// read-only file and fails.
	if err := os.WriteFile(keyPath, []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatalf("seed plaintext key: %v", err)
	}
	if err := os.Chmod(keyPath, 0o444); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}

	_, err := NewSigner(dir, taggedMockCryptoManager{})
	if err == nil {
		t.Fatal("expected migration rewrite error")
	}
	if !strings.Contains(err.Error(), "failed to rewrite audit signing key") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestBranchcov0724pmNewSigner_GenerateEncryptError reaches the
// "failed to encrypt audit signing key" arm on the generate-new-key path
// (no existing key file present).
func TestBranchcov0724pmNewSigner_GenerateEncryptError(t *testing.T) {
	dir := t.TempDir()
	// No key file present -> generate path; failingEncryptCrypto.Encrypt fails.
	_, err := NewSigner(dir, failingEncryptCrypto{})
	if err == nil {
		t.Fatal("expected generate encrypt error")
	}
	if !strings.Contains(err.Error(), "failed to encrypt audit signing key") {
		t.Fatalf("unexpected error text: %v", err)
	}

	// On this failure path no key file should have been written.
	if _, statErr := os.Stat(filepath.Join(dir, ".audit-signing.key")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no key file after encrypt failure, stat err=%v", statErr)
	}
}

// TestBranchcov0724pmNewSigner_MkdirAllError reaches the
// "failed to create directory for audit signing key" arm by pointing dataDir
// at a regular file, so MkdirAll(parent) fails with "not a directory".
func TestBranchcov0724pmNewSigner_MkdirAllError(t *testing.T) {
	// dataDir is a regular file: ReadFile(file/.audit-signing.key) fails
	// (ENOTDIR) -> skip load -> generate -> MkdirAll(file) fails.
	file, err := os.CreateTemp("", "audit-data-is-file")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(file.Name())
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	_, err = NewSigner(file.Name(), taggedMockCryptoManager{})
	if err == nil {
		t.Fatal("expected mkdir error")
	}
	if !strings.Contains(err.Error(), "failed to create directory for audit signing key") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestBranchcov0724pmNewSigner_GenerateWriteError reaches the
// "failed to save audit signing key" arm by pre-creating the key path as a
// directory, so the final WriteFile fails with EISDIR.
func TestBranchcov0724pmNewSigner_GenerateWriteError(t *testing.T) {
	dir := t.TempDir()
	// Pre-create the destination path as a directory. ReadFile on a directory
	// fails ("is a directory") -> skip load -> generate -> MkdirAll(dir) ok ->
	// WriteFile(dir) fails with EISDIR.
	keyPath := filepath.Join(dir, ".audit-signing.key")
	if err := os.Mkdir(keyPath, 0o700); err != nil {
		t.Fatalf("seed key-as-dir: %v", err)
	}

	_, err := NewSigner(dir, taggedMockCryptoManager{})
	if err == nil {
		t.Fatal("expected save error")
	}
	if !strings.Contains(err.Error(), "failed to save audit signing key") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestBranchcov0724pmNewSigner_NilCryptoYieldsDisabledSigner covers the
// cryptoMgr==nil fast path explicitly and asserts the resulting signer signs
// nothing ("empty key") and verifies nothing.
func TestBranchcov0724pmNewSigner_NilCryptoYieldsDisabledSigner(t *testing.T) {
	signer, err := NewSigner(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewSigner(nil crypto): %v", err)
	}
	if signer == nil {
		t.Fatal("expected non-nil disabled signer")
	}
	if signer.SigningEnabled() {
		t.Fatal("signing must be disabled with nil crypto manager")
	}
	if signer.ExportKey() != "" {
		t.Fatalf("ExportKey should be empty for disabled signer, got %q", signer.ExportKey())
	}
	event := Event{ID: "x", Timestamp: time.Now(), EventType: "e"}
	if sig := signer.Sign(event); sig != "" {
		t.Fatalf("disabled signer produced signature %q", sig)
	}
	// A disabled signer must reject verification regardless of signature.
	if signer.Verify(Event{ID: "x", Signature: "deadbeef"}) {
		t.Fatal("disabled signer must not verify any event")
	}
}

// TestBranchcov0724pmNewSigner_WrongKeyDoesNotCrossVerify pins the "wrong key"
// behaviour: a signature produced under one key must not verify under another.
func TestBranchcov0724pmNewSigner_WrongKeyDoesNotCrossVerify(t *testing.T) {
	a, err := NewSignerWithKey([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewSignerWithKey A: %v", err)
	}
	b, err := NewSignerWithKey([]byte("fedcba9876543210fedcba9876543210"))
	if err != nil {
		t.Fatalf("NewSignerWithKey B: %v", err)
	}
	event := Event{
		ID: "cross", Timestamp: time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC),
		EventType: "login", User: "u", IP: "1.2.3.4", Path: "/", Success: true,
	}
	event.Signature = a.Sign(event)
	if !a.Verify(event) {
		t.Fatal("signer A should verify its own signature")
	}
	if b.Verify(event) {
		t.Fatal("signer B must reject A's signature (wrong key)")
	}
}

// --- async_logger.go: VerifySignature (75%) and IsPersistentAuditLogger (66.7%) ---

// TestBranchcov0724pmAsyncLogger_VerifySignatureArms covers the two arms the
// existing async test misses: a nil receiver and a backend that does not
// implement VerifySignature (ConsoleLogger). It also re-confirms the happy
// delegation path with a real signed event and a tampered payload.
func TestBranchcov0724pmAsyncLogger_VerifySignatureArms(t *testing.T) {
	// Arm 1: nil receiver must return false without panicking.
	var nilLogger *AsyncLogger
	if nilLogger.VerifySignature(Event{}) {
		t.Fatal("nil AsyncLogger.VerifySignature must return false")
	}

	// Arm 2: backend without a VerifySignature method (ConsoleLogger) ->
	// type assertion fails -> ok==false -> returns false.
	console := NewAsyncLogger(NewConsoleLogger(), AsyncLoggerConfig{BufferSize: 4})
	defer console.Close()
	if console.VerifySignature(Event{ID: "x", Signature: "sig"}) {
		t.Fatal("ConsoleLogger-backed AsyncLogger must report false (no verifier)")
	}

	// Arm 3 (happy path, for the prose: "valid signature"): a genuinely signed
	// event verifies true through the async wrapper; a tampered payload false.
	backend, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   t.TempDir(),
		CryptoMgr: newMockCryptoManager(),
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	defer backend.Close()

	signer, err := NewSigner(t.TempDir(), newMockCryptoManager())
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	ts := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	event := Event{
		ID: "verify-good", Timestamp: ts, EventType: "login",
		User: "admin", IP: "10.0.0.1", Path: "/login", Success: true, Details: "ok",
	}
	event.Signature = signer.Sign(event)
	if err := backend.Record(event); err != nil {
		t.Fatalf("record event: %v", err)
	}

	async := NewAsyncLogger(backend, AsyncLoggerConfig{BufferSize: 8})
	defer async.Close()
	stored, _, qerr := async.QueryPage(QueryFilter{ID: event.ID, Limit: 1})
	if qerr != nil || len(stored) != 1 {
		t.Fatalf("QueryPage: qerr=%v len=%d", qerr, len(stored))
	}
	if !async.VerifySignature(stored[0]) {
		t.Fatal("async wrapper must verify a validly signed stored event")
	}
	// Tamper the payload: same signature, changed content -> must fail.
	tampered := stored[0]
	tampered.Details = "tampered"
	if async.VerifySignature(tampered) {
		t.Fatal("async wrapper must reject a tampered payload")
	}
}

// TestBranchcov0724pmAsyncLogger_IsPersistentAuditLoggerArms covers both
// verdicts plus the nil receiver, which the existing suite never exercises.
func TestBranchcov0724pmAsyncLogger_IsPersistentAuditLoggerArms(t *testing.T) {
	// Nil receiver -> false.
	var nilLogger *AsyncLogger
	if nilLogger.IsPersistentAuditLogger() {
		t.Fatal("nil AsyncLogger.IsPersistentAuditLogger must return false")
	}

	// Non-persistent backend (ConsoleLogger) -> false via delegation.
	console := NewAsyncLogger(NewConsoleLogger(), AsyncLoggerConfig{BufferSize: 2})
	defer console.Close()
	if console.IsPersistentAuditLogger() {
		t.Fatal("console-backed async logger must not be persistent")
	}

	// Persistent backend (SQLiteLogger) -> true.
	backend, err := NewSQLiteLogger(SQLiteLoggerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	defer backend.Close()
	persistent := NewAsyncLogger(backend, AsyncLoggerConfig{BufferSize: 2})
	defer persistent.Close()
	if !persistent.IsPersistentAuditLogger() {
		t.Fatal("sqlite-backed async logger must be persistent")
	}
}

// --- audit.go: IsPersistentLogger (50%) ---

// stubBareLogger satisfies Logger but deliberately does NOT implement
// persistentAuditLogger, so IsPersistentLogger falls through to the
// isConsole branch.
type stubBareLogger struct{}

func (stubBareLogger) Record(Event) error                 { return nil }
func (stubBareLogger) Query(QueryFilter) ([]Event, error) { return nil, nil }
func (stubBareLogger) Count(QueryFilter) (int, error)     { return 0, nil }
func (stubBareLogger) GetWebhookURLs() []string           { return nil }
func (stubBareLogger) UpdateWebhookURLs([]string) error   { return nil }
func (stubBareLogger) Close() error                       { return nil }

// TestBranchcov0724pmIsPersistentLogger_AllArms covers the nil fast path and
// the fall-through isConsole branch (both verdicts).
func TestBranchcov0724pmIsPersistentLogger_AllArms(t *testing.T) {
	// Arm 1: nil logger -> false.
	if IsPersistentLogger(nil) {
		t.Fatal("IsPersistentLogger(nil) must be false")
	}

	// Arm 2: a Logger that does not declare persistence and is NOT a
	// *ConsoleLogger falls through to `return !isConsole` == true. This is the
	// fallback's "treat unknown backends as persistent" contract for enterprise
	// implementations.
	if !IsPersistentLogger(stubBareLogger{}) {
		t.Fatal("IsPersistentLogger should treat a non-console bare Logger as persistent")
	}

	// Arm 3: ConsoleLogger declares persistence explicitly and reports false.
	if IsPersistentLogger(NewConsoleLogger()) {
		t.Fatal("IsPersistentLogger(ConsoleLogger) must be false")
	}

	// Arm 4: SQLiteLogger declares persistence and reports true.
	backend, err := NewSQLiteLogger(SQLiteLoggerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	defer backend.Close()
	if !IsPersistentLogger(backend) {
		t.Fatal("IsPersistentLogger(SQLiteLogger) must be true")
	}
}
