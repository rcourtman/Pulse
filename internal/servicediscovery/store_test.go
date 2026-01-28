package servicediscovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
)

type fakeCrypto struct{}

func (fakeCrypto) Encrypt(plaintext []byte) ([]byte, error) {
	out := make([]byte, len(plaintext))
	for i := range plaintext {
		out[i] = plaintext[len(plaintext)-1-i]
	}
	return out, nil
}

func (fakeCrypto) Decrypt(ciphertext []byte) ([]byte, error) {
	return fakeCrypto{}.Encrypt(ciphertext)
}

type errorCrypto struct{}

func (errorCrypto) Encrypt(plaintext []byte) ([]byte, error) {
	return nil, os.ErrInvalid
}

func (errorCrypto) Decrypt(ciphertext []byte) ([]byte, error) {
	return nil, os.ErrInvalid
}

func TestStore_SaveGetListAndNotes(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	d1 := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "host1", "nginx"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "nginx",
		HostID:       "host1",
		ServiceName:  "Nginx",
	}
	if err := store.Save(d1); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	got, err := store.Get(d1.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got == nil || got.ServiceName != "Nginx" {
		t.Fatalf("unexpected discovery: %#v", got)
	}
	if !store.Exists(d1.ID) {
		t.Fatalf("expected discovery to exist")
	}

	if err := store.UpdateNotes(d1.ID, "notes", map[string]string{"token": "abc"}); err != nil {
		t.Fatalf("UpdateNotes error: %v", err)
	}
	updated, err := store.Get(d1.ID)
	if err != nil {
		t.Fatalf("Get updated error: %v", err)
	}
	if updated.UserNotes != "notes" || updated.UserSecrets["token"] != "abc" {
		t.Fatalf("notes not updated: %#v", updated)
	}

	d2 := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeVM, "node1", "101"),
		ResourceType: ResourceTypeVM,
		ResourceID:   "101",
		HostID:       "node1",
		ServiceName:  "VM",
	}
	if err := store.Save(d2); err != nil {
		t.Fatalf("Save d2 error: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 discoveries, got %d", len(list))
	}

	byType, err := store.ListByType(ResourceTypeVM)
	if err != nil {
		t.Fatalf("ListByType error: %v", err)
	}
	if len(byType) != 1 || byType[0].ID != d2.ID {
		t.Fatalf("unexpected ListByType: %#v", byType)
	}

	byHost, err := store.ListByHost("host1")
	if err != nil {
		t.Fatalf("ListByHost error: %v", err)
	}
	if len(byHost) != 1 || byHost[0].ID != d1.ID {
		t.Fatalf("unexpected ListByHost: %#v", byHost)
	}

	summary := updated.ToSummary()
	if summary.ID != d1.ID || !summary.HasUserNotes {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	if err := store.Delete(d1.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if store.Exists(d1.ID) {
		t.Fatalf("expected discovery to be deleted")
	}
}

func TestStore_CryptoRoundTripAndPaths(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = fakeCrypto{}

	id := "docker:host1:app/name"
	d := &ResourceDiscovery{
		ID:           id,
		ResourceType: ResourceTypeDocker,
		ResourceID:   "app/name",
		HostID:       "host1",
		ServiceName:  "App",
	}
	if err := store.Save(d); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	path := store.getFilePath(id)
	base := filepath.Base(path)
	if strings.Contains(base, ":") || strings.Contains(base, "/") {
		t.Fatalf("expected sanitized base filename, got %s", base)
	}

	loaded, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if loaded == nil || loaded.ServiceName != "App" {
		t.Fatalf("unexpected discovery: %#v", loaded)
	}

	store.ClearCache()
	if _, err := store.Get(id); err != nil {
		t.Fatalf("Get with decrypt error: %v", err)
	}
	list, err := store.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List with decrypt error: %v len=%d", err, len(list))
	}
}

func TestStore_NeedsRefreshAndGetMultiple(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	if !store.NeedsRefresh("missing", time.Minute) {
		t.Fatalf("expected missing discovery to need refresh")
	}

	d := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeHost, "host1", "host1"),
		ResourceType: ResourceTypeHost,
		ResourceID:   "host1",
		HostID:       "host1",
		ServiceName:  "Host",
	}
	if err := store.Save(d); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	path := store.getFilePath(d.ID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var saved ResourceDiscovery
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	saved.UpdatedAt = time.Now().Add(-2 * time.Hour)
	data, err = json.Marshal(&saved)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	store.ClearCache()
	if !store.NeedsRefresh(d.ID, time.Minute) {
		t.Fatalf("expected old discovery to need refresh")
	}

	ids := []string{d.ID, "missing"}
	multi, err := store.GetMultiple(ids)
	if err != nil {
		t.Fatalf("GetMultiple error: %v", err)
	}
	if len(multi) != 1 || multi[0].ID != d.ID {
		t.Fatalf("unexpected GetMultiple: %#v", multi)
	}
}

func TestStore_ErrorsAndListSkips(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	if err := store.Save(&ResourceDiscovery{}); err == nil {
		t.Fatalf("expected error for empty ID")
	}

	store.crypto = errorCrypto{}
	if err := store.Save(&ResourceDiscovery{ID: "bad"}); err == nil {
		t.Fatalf("expected encrypt error")
	}

	store.crypto = nil
	if _, err := store.Get("missing"); err != nil {
		t.Fatalf("unexpected missing error: %v", err)
	}

	d := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "host1", "web"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		ServiceName:  "Web",
		UserSecrets:  map[string]string{"token": "abc"},
	}
	if err := store.Save(d); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Corrupt file to force unmarshal error during List.
	badPath := filepath.Join(store.dataDir, "bad.enc")
	if err := os.WriteFile(badPath, []byte("{bad"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.dataDir, "note.txt"), []byte("skip"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.dataDir, "skip.enc.tmp"), []byte("skip"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(store.dataDir, "dir"), 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	unreadable := filepath.Join(store.dataDir, "unreadable.enc")
	if err := os.WriteFile(unreadable, []byte("nope"), 0000); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 discovery, got %d", len(list))
	}

	store.crypto = errorCrypto{}
	list, err = store.List()
	if err != nil {
		t.Fatalf("List with crypto error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected crypto errors to skip entries")
	}

	store.crypto = errorCrypto{}
	store.ClearCache()
	if _, err := store.Get(d.ID); err == nil {
		t.Fatalf("expected decrypt error")
	}

	store.crypto = nil
	if got, err := store.GetByResource(ResourceTypeDocker, "host1", "web"); err != nil || got == nil {
		t.Fatalf("GetByResource error: %v", err)
	}

	if err := store.UpdateNotes(d.ID, "notes-only", nil); err != nil {
		t.Fatalf("UpdateNotes error: %v", err)
	}
	updated, err := store.Get(d.ID)
	if err != nil || updated.UserSecrets == nil {
		t.Fatalf("expected secrets to be preserved: %#v err=%v", updated, err)
	}

	store.crypto = errorCrypto{}
	store.ClearCache()
	if err := store.UpdateNotes(d.ID, "notes", nil); err == nil {
		t.Fatalf("expected update notes error with crypto failure")
	}
	if got, err := store.GetMultiple([]string{d.ID}); err != nil || len(got) != 0 {
		t.Fatalf("expected GetMultiple to skip errors")
	}

	if err := store.UpdateNotes("missing", "notes", nil); err == nil {
		t.Fatalf("expected error for missing discovery")
	}

	if err := store.Delete("missing"); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
}

func TestStore_NewStoreError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if _, err := NewStore(file); err == nil {
		t.Fatalf("expected error for file data dir")
	}
}

func TestStore_NewStoreCryptoFailure(t *testing.T) {
	orig := newCryptoManagerAt
	newCryptoManagerAt = func(dataDir string) (*crypto.CryptoManager, error) {
		manager, err := crypto.NewCryptoManagerAt(dataDir)
		if err != nil {
			return nil, err
		}
		return manager, os.ErrInvalid
	}
	t.Cleanup(func() {
		newCryptoManagerAt = orig
	})

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	if store.crypto == nil {
		t.Fatalf("expected crypto manager despite init warning")
	}
}

func TestStore_SaveMarshalError(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	orig := marshalDiscovery
	marshalDiscovery = func(any) ([]byte, error) {
		return nil, os.ErrInvalid
	}
	t.Cleanup(func() {
		marshalDiscovery = orig
	})

	if err := store.Save(&ResourceDiscovery{ID: "marshal"}); err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestStore_SaveAndGetErrors(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	id := MakeResourceID(ResourceTypeDocker, "host1", "web")
	filePath := store.getFilePath(id)
	if err := os.MkdirAll(filePath, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	if err := store.Save(&ResourceDiscovery{ID: id}); err == nil {
		t.Fatalf("expected rename error")
	}

	tmpFile := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(tmpFile, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	store.dataDir = tmpFile
	if err := store.Save(&ResourceDiscovery{ID: "bad"}); err == nil {
		t.Fatalf("expected write error")
	}

	store.dataDir = t.TempDir()
	store.crypto = nil
	badPath := store.getFilePath("bad")
	if err := os.WriteFile(badPath, []byte("{bad"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if _, err := store.Get("bad"); err == nil {
		t.Fatalf("expected unmarshal error")
	}
}

func TestStore_ListErrors(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	store.dataDir = filepath.Join(t.TempDir(), "missing")
	list, err := store.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("expected empty list for missing dir")
	}

	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	store.dataDir = file
	if _, err := store.List(); err == nil {
		t.Fatalf("expected list error for file path")
	}
	if _, err := store.ListByType(ResourceTypeDocker); err == nil {
		t.Fatalf("expected list by type error")
	}
	if _, err := store.ListByHost("host1"); err == nil {
		t.Fatalf("expected list by host error")
	}
}

func TestStore_DeleteError(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	id := MakeResourceID(ResourceTypeDocker, "host1", "dir")
	filePath := store.getFilePath(id)
	if err := os.MkdirAll(filePath, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	nested := filepath.Join(filePath, "nested")
	if err := os.WriteFile(nested, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if err := store.Delete(id); err == nil {
		t.Fatalf("expected delete error for non-empty dir")
	}
}
