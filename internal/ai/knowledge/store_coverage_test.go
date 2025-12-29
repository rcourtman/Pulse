package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
)

func TestNewStore_FilePathError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "knowledge-file")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(path)

	store, err := NewStore(path)
	if err == nil || store != nil {
		t.Fatal("expected error when dataDir is a file")
	}
}

func TestNewStore_CryptoInitFailure(t *testing.T) {
	tmpDir := t.TempDir()
	orig := newCryptoManagerAt
	newCryptoManagerAt = func(string) (*crypto.CryptoManager, error) {
		return nil, os.ErrPermission
	}
	defer func() {
		newCryptoManagerAt = orig
	}()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("expected store creation to succeed: %v", err)
	}
	if store.crypto != nil {
		t.Error("expected crypto manager to be nil after init failure")
	}
}

func TestGuestFilePath_Extensions(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		cryptoMgr, err := crypto.NewCryptoManagerAt(tmpDir)
		if err != nil {
			t.Fatalf("create crypto manager: %v", err)
		}
		store.crypto = cryptoMgr
	}

	encPath := store.guestFilePath("guest-1")
	if !strings.HasSuffix(encPath, ".enc") {
		t.Errorf("expected encrypted path, got %q", encPath)
	}

	store.crypto = nil
	jsonPath := store.guestFilePath("guest-1")
	if !strings.HasSuffix(jsonPath, ".json") {
		t.Errorf("expected json path, got %q", jsonPath)
	}
}

func TestGetKnowledge_LegacyFallback(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		cryptoMgr, err := crypto.NewCryptoManagerAt(tmpDir)
		if err != nil {
			t.Fatalf("create crypto manager: %v", err)
		}
		store.crypto = cryptoMgr
	}

	guestID := "legacy-guest"
	knowledge := GuestKnowledge{
		GuestID:   guestID,
		GuestName: "Legacy",
		Notes: []Note{
			{ID: "note-1", Category: "service", Title: "svc", Content: "value"},
		},
	}
	data, err := json.Marshal(knowledge)
	if err != nil {
		t.Fatalf("marshal knowledge: %v", err)
	}
	legacyPath := filepath.Join(store.dataDir, filepath.Base(guestID)+".json")
	if err := os.WriteFile(legacyPath, data, 0600); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	loaded, err := store.GetKnowledge(guestID)
	if err != nil {
		t.Fatalf("GetKnowledge failed: %v", err)
	}
	if loaded.GuestName != "Legacy" {
		t.Errorf("expected legacy guest name, got %q", loaded.GuestName)
	}
}

func TestGetKnowledge_DecryptFallback(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		cryptoMgr, err := crypto.NewCryptoManagerAt(tmpDir)
		if err != nil {
			t.Fatalf("create crypto manager: %v", err)
		}
		store.crypto = cryptoMgr
	}

	guestID := "plain-enc"
	knowledge := GuestKnowledge{
		GuestID: guestID,
		Notes: []Note{
			{ID: "note-1", Category: "service", Title: "svc", Content: "value"},
		},
	}
	data, err := json.Marshal(knowledge)
	if err != nil {
		t.Fatalf("marshal knowledge: %v", err)
	}
	if err := os.WriteFile(store.guestFilePath(guestID), data, 0600); err != nil {
		t.Fatalf("write plain enc file: %v", err)
	}

	loaded, err := store.GetKnowledge(guestID)
	if err != nil {
		t.Fatalf("GetKnowledge failed: %v", err)
	}
	if len(loaded.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(loaded.Notes))
	}
}

func TestGetKnowledge_DecryptSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		t.Skip("crypto manager unavailable")
	}

	guestID := "encrypted-guest"
	knowledge := GuestKnowledge{
		GuestID: guestID,
		Notes: []Note{
			{ID: "note-1", Category: "service", Title: "svc", Content: "value"},
		},
	}
	plain, err := json.Marshal(knowledge)
	if err != nil {
		t.Fatalf("marshal knowledge: %v", err)
	}
	encrypted, err := store.crypto.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt knowledge: %v", err)
	}
	if err := os.WriteFile(store.guestFilePath(guestID), encrypted, 0600); err != nil {
		t.Fatalf("write enc file: %v", err)
	}

	loaded, err := store.GetKnowledge(guestID)
	if err != nil {
		t.Fatalf("GetKnowledge failed: %v", err)
	}
	if len(loaded.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(loaded.Notes))
	}
}

func TestGetKnowledge_DecryptParseError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		t.Skip("crypto manager unavailable")
	}

	guestID := "bad-enc-json"
	encrypted, err := store.crypto.Encrypt([]byte("{bad"))
	if err != nil {
		t.Fatalf("encrypt data: %v", err)
	}
	if err := os.WriteFile(store.guestFilePath(guestID), encrypted, 0600); err != nil {
		t.Fatalf("write enc file: %v", err)
	}

	if _, err := store.GetKnowledge(guestID); err == nil {
		t.Fatal("expected parse error after decrypt")
	}
}

func TestGetKnowledge_DoubleCheckCache(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	guestID := "cached-guest"
	expected := &GuestKnowledge{GuestID: guestID, Notes: []Note{{ID: "note-1"}}}

	orig := beforeKnowledgeWriteLock
	beforeKnowledgeWriteLock = func() {
		store.mu.Lock()
		store.cache[guestID] = expected
		store.mu.Unlock()
	}
	defer func() {
		beforeKnowledgeWriteLock = orig
	}()

	loaded, err := store.GetKnowledge(guestID)
	if err != nil {
		t.Fatalf("GetKnowledge failed: %v", err)
	}
	if loaded != expected {
		t.Error("expected cached knowledge to be returned from double-check")
	}
}

func TestGetKnowledge_LegacyReadError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		t.Skip("crypto manager unavailable")
	}

	guestID := "legacy-error"
	legacyPath := filepath.Join(store.dataDir, filepath.Base(guestID)+".json")
	if err := os.Mkdir(legacyPath, 0700); err != nil {
		t.Fatalf("create legacy dir: %v", err)
	}

	if _, err := store.GetKnowledge(guestID); err == nil {
		t.Fatal("expected legacy read error")
	}
}

func TestGetKnowledge_DecryptError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		cryptoMgr, err := crypto.NewCryptoManagerAt(tmpDir)
		if err != nil {
			t.Fatalf("create crypto manager: %v", err)
		}
		store.crypto = cryptoMgr
	}

	guestID := "bad-enc"
	if err := os.WriteFile(store.guestFilePath(guestID), []byte("not-json"), 0600); err != nil {
		t.Fatalf("write enc file: %v", err)
	}

	if _, err := store.GetKnowledge(guestID); err == nil {
		t.Fatal("expected decrypt error")
	}
}

func TestGetKnowledge_ParseError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	guestID := "bad-json"
	if err := os.WriteFile(store.guestFilePath(guestID), []byte("{bad"), 0600); err != nil {
		t.Fatalf("write json file: %v", err)
	}

	if _, err := store.GetKnowledge(guestID); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGetKnowledge_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	guestID := "dir-guest"
	path := store.guestFilePath(guestID)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatalf("create dir at file path: %v", err)
	}

	if _, err := store.GetKnowledge(guestID); err == nil {
		t.Fatal("expected read error")
	}
}

func TestSaveNote_LoadExistingInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	guestID := "existing-guest"
	if err := os.WriteFile(store.guestFilePath(guestID), []byte("{bad"), 0600); err != nil {
		t.Fatalf("write invalid file: %v", err)
	}

	if err := store.SaveNote(guestID, "Guest", "vm", "service", "Web", "nginx"); err != nil {
		t.Fatalf("SaveNote failed: %v", err)
	}
	knowledge, err := store.GetKnowledge(guestID)
	if err != nil {
		t.Fatalf("GetKnowledge failed: %v", err)
	}
	if len(knowledge.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(knowledge.Notes))
	}
}

func TestSaveNote_LoadEncryptedExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		t.Skip("crypto manager unavailable")
	}

	guestID := "encrypted-existing"
	existing := GuestKnowledge{
		GuestID:   guestID,
		GuestName: "Guest",
		GuestType: "vm",
		Notes: []Note{
			{ID: "note-1", Category: "service", Title: "Web", Content: "apache"},
		},
	}
	plain, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal knowledge: %v", err)
	}
	encrypted, err := store.crypto.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt knowledge: %v", err)
	}
	if err := os.WriteFile(store.guestFilePath(guestID), encrypted, 0600); err != nil {
		t.Fatalf("write enc file: %v", err)
	}

	if err := store.SaveNote(guestID, "", "", "service", "Web", "nginx"); err != nil {
		t.Fatalf("SaveNote failed: %v", err)
	}
	knowledge, err := store.GetKnowledge(guestID)
	if err != nil {
		t.Fatalf("GetKnowledge failed: %v", err)
	}
	if len(knowledge.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(knowledge.Notes))
	}
	if knowledge.Notes[0].Content != "nginx" {
		t.Errorf("expected updated content, got %q", knowledge.Notes[0].Content)
	}
}

func TestDeleteNote_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	if err := store.SaveNote("guest", "Guest", "vm", "service", "Web", "nginx"); err != nil {
		t.Fatalf("SaveNote failed: %v", err)
	}
	if err := store.DeleteNote("guest", "missing"); err == nil {
		t.Fatal("expected note not found error")
	}
}

func TestGetNotesByCategory_All(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	store.SaveNote("guest", "Guest", "vm", "service", "Web", "nginx")
	store.SaveNote("guest", "Guest", "vm", "config", "DB", "postgres")

	notes, err := store.GetNotesByCategory("guest", "")
	if err != nil {
		t.Fatalf("GetNotesByCategory failed: %v", err)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}

func TestGetNotesByCategory_Error(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	guestID := "bad-category"
	if err := os.WriteFile(store.guestFilePath(guestID), []byte("{bad"), 0600); err != nil {
		t.Fatalf("write invalid file: %v", err)
	}

	if _, err := store.GetNotesByCategory(guestID, "service"); err == nil {
		t.Fatal("expected GetNotesByCategory error")
	}
}

func TestFormatForContext_Error(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	guestID := "bad-context"
	if err := os.WriteFile(store.guestFilePath(guestID), []byte("{bad"), 0600); err != nil {
		t.Fatalf("write invalid file: %v", err)
	}

	if result := store.FormatForContext(guestID); result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestSaveToFile_MarshalError(t *testing.T) {
	store := &Store{
		dataDir: t.TempDir(),
		cache:   make(map[string]*GuestKnowledge),
	}

	badTime := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	knowledge := &GuestKnowledge{
		GuestID:   "guest",
		UpdatedAt: badTime,
		Notes: []Note{
			{
				ID:        "note-1",
				Category:  "service",
				Title:     "Web",
				Content:   "nginx",
				CreatedAt: badTime,
				UpdatedAt: badTime,
			},
		},
	}

	if err := store.saveToFile("guest", knowledge); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestSaveToFile_WriteError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "knowledge-data")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(path)

	store := &Store{
		dataDir: path,
		cache:   make(map[string]*GuestKnowledge),
	}
	knowledge := &GuestKnowledge{GuestID: "guest"}

	if err := store.saveToFile("guest", knowledge); err == nil {
		t.Fatal("expected write error")
	}
}

func TestSaveToFile_RemovesLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		t.Skip("crypto manager unavailable")
	}

	guestID := "legacy-remove"
	legacyPath := filepath.Join(store.dataDir, filepath.Base(guestID)+".json")
	if err := os.WriteFile(legacyPath, []byte(`{"guest_id":"legacy-remove","notes":[]}`), 0600); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	knowledge := &GuestKnowledge{GuestID: guestID}
	if err := store.saveToFile(guestID, knowledge); err != nil {
		t.Fatalf("saveToFile failed: %v", err)
	}
	if _, err := os.Stat(legacyPath); err == nil {
		t.Fatal("expected legacy file to be removed")
	}
}

func TestSaveToFile_EncryptError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if store.crypto == nil {
		t.Skip("crypto manager unavailable")
	}

	keyPath := filepath.Join(tmpDir, ".encryption.key")
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("remove key: %v", err)
	}

	knowledge := &GuestKnowledge{GuestID: "guest"}
	if err := store.saveToFile("guest", knowledge); err == nil {
		t.Fatal("expected encrypt error")
	}
}

func TestListGuests_ReadDirError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "knowledge-dir")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(path)

	store := &Store{
		dataDir: path,
		cache:   make(map[string]*GuestKnowledge),
	}

	if _, err := store.ListGuests(); err == nil {
		t.Fatal("expected read dir error")
	}
}

func TestFormatAllForContext_ListError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "knowledge-dir")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(path)

	store := &Store{
		dataDir: path,
		cache:   make(map[string]*GuestKnowledge),
	}

	if result := store.FormatAllForContext(); result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestFormatAllForContext_NoNotes(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	guestID := "empty-notes"
	knowledge := &GuestKnowledge{GuestID: guestID, Notes: []Note{}}
	data, err := json.Marshal(knowledge)
	if err != nil {
		t.Fatalf("marshal knowledge: %v", err)
	}
	if err := os.WriteFile(store.guestFilePath(guestID), data, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if result := store.FormatAllForContext(); result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestFormatAllForContext_NoTruncate(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	store.SaveNote("guest-1", "", "vm", "service", "Web", "nginx")
	store.SaveNote("guest-2", "GuestTwo", "vm", "config", "DB", "postgres")

	result := store.FormatAllForContext()
	if !strings.Contains(result, "notes across") {
		t.Errorf("expected non-truncated header, got %q", result)
	}
	if !strings.Contains(result, "guest-1") {
		t.Error("expected guest ID fallback when name is empty")
	}
}

func TestFormatAllForContext_CredentialMasking(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	store.SaveNote("guest-1", "Guest", "vm", "credential", "Root", "password1234")
	result := store.FormatAllForContext()
	if !strings.Contains(result, "pa****34") {
		t.Errorf("expected masked credential, got %q", result)
	}
}

func TestFormatAllForContext_NoGuests(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	if result := store.FormatAllForContext(); result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestFormatAllForContext_TooLargeFirstNote(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.crypto = nil

	largeContent := make([]byte, 9000)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	store.SaveNote("guest-1", "Guest", "vm", "service", "Big", string(largeContent))

	if result := store.FormatAllForContext(); result != "" {
		t.Errorf("expected empty result for oversized first note, got %q", result)
	}
}
