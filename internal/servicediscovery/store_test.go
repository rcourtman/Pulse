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

func TestStore_NewStoreRejectsBlankDataDir(t *testing.T) {
	if _, err := NewStore(" \t\n "); err == nil {
		t.Fatalf("expected error for blank data dir")
	}
}

func TestStore_NewStoreTrimsDataDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore("  " + dir + "  ")
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	want := filepath.Join(dir, "discovery")
	if store.dataDir != want {
		t.Fatalf("store.dataDir = %q, want %q", store.dataDir, want)
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

func TestStore_SaveAndGet_ReturnsDefensiveCopies(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	discovery := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "host1", "web"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		ServiceName:  "Web",
		Facts: []DiscoveryFact{
			{Key: "service", Value: "nginx"},
		},
		ConfigPaths: []string{"/etc/nginx"},
		UserSecrets: map[string]string{"token": "abc"},
		RawCommandOutput: map[string]string{
			"ps": "nginx",
		},
	}
	if err := store.Save(discovery); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Mutate caller-owned value after Save; cache/state must remain unchanged.
	discovery.ServiceName = "Mutated"
	discovery.Facts[0].Value = "changed"
	discovery.ConfigPaths[0] = "/tmp"
	discovery.UserSecrets["token"] = "mutated"
	discovery.RawCommandOutput["ps"] = "changed"

	got1, err := store.Get(discovery.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got1 == nil {
		t.Fatalf("expected discovery, got nil")
	}
	if got1.ServiceName != "Web" {
		t.Fatalf("expected cached service name to remain Web, got %q", got1.ServiceName)
	}
	if got1.Facts[0].Value != "nginx" {
		t.Fatalf("expected cached fact value nginx, got %q", got1.Facts[0].Value)
	}
	if got1.ConfigPaths[0] != "/etc/nginx" {
		t.Fatalf("expected cached config path /etc/nginx, got %q", got1.ConfigPaths[0])
	}
	if got1.UserSecrets["token"] != "abc" {
		t.Fatalf("expected cached secret token abc, got %q", got1.UserSecrets["token"])
	}
	if got1.RawCommandOutput["ps"] != "nginx" {
		t.Fatalf("expected cached raw output nginx, got %q", got1.RawCommandOutput["ps"])
	}

	// Mutate returned value from Get; internal cache must remain unchanged.
	got1.ServiceName = "ChangedByCaller"
	got1.Facts[0].Value = "bad"
	got1.ConfigPaths[0] = "/bad"
	got1.UserSecrets["token"] = "bad"
	got1.RawCommandOutput["ps"] = "bad"

	got2, err := store.Get(discovery.ID)
	if err != nil {
		t.Fatalf("second Get error: %v", err)
	}
	if got2.ServiceName != "Web" || got2.Facts[0].Value != "nginx" || got2.ConfigPaths[0] != "/etc/nginx" || got2.UserSecrets["token"] != "abc" || got2.RawCommandOutput["ps"] != "nginx" {
		t.Fatalf("expected second Get to be isolated from caller mutations, got %#v", got2)
	}
}

func TestStore_Fingerprints_ReturnDefensiveCopies(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	fp := &ContainerFingerprint{
		ResourceID: "docker:host1:web",
		HostID:     "host1",
		Hash:       "abc123",
		Ports:      []string{"80/tcp"},
		MountPaths: []string{"/config"},
		EnvKeys:    []string{"FOO"},
	}
	if err := store.SaveFingerprint(fp); err != nil {
		t.Fatalf("SaveFingerprint error: %v", err)
	}

	// Mutate caller-owned fingerprint after SaveFingerprint.
	fp.Hash = "mutated"
	fp.Ports[0] = "443/tcp"
	fp.MountPaths[0] = "/tmp"
	fp.EnvKeys[0] = "BAR"

	got1, err := store.GetFingerprint("docker:host1:web")
	if err != nil {
		t.Fatalf("GetFingerprint error: %v", err)
	}
	if got1 == nil {
		t.Fatalf("expected fingerprint, got nil")
	}
	if got1.Hash != "abc123" || got1.Ports[0] != "80/tcp" || got1.MountPaths[0] != "/config" || got1.EnvKeys[0] != "FOO" {
		t.Fatalf("expected stored fingerprint to be isolated from caller mutations, got %#v", got1)
	}

	// Mutate returned fingerprint; store should still return original data.
	got1.Hash = "changed-by-caller"
	got1.Ports[0] = "9999/tcp"
	got1.MountPaths[0] = "/bad"
	got1.EnvKeys[0] = "BAD"

	got2, err := store.GetFingerprint("docker:host1:web")
	if err != nil {
		t.Fatalf("second GetFingerprint error: %v", err)
	}
	if got2.Hash != "abc123" || got2.Ports[0] != "80/tcp" || got2.MountPaths[0] != "/config" || got2.EnvKeys[0] != "FOO" {
		t.Fatalf("expected second GetFingerprint to be isolated from caller mutations, got %#v", got2)
	}

	all := store.GetAllFingerprints()
	all["docker:host1:web"].Hash = "changed-from-map"
	got3, err := store.GetFingerprint("docker:host1:web")
	if err != nil {
		t.Fatalf("third GetFingerprint error: %v", err)
	}
	if got3.Hash != "abc123" {
		t.Fatalf("expected GetAllFingerprints map to be isolated copy, got hash %q", got3.Hash)
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

func TestStore_GetChangedResources(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	// Save fingerprints for different resource types, using the same key
	// format that collectFingerprints uses: "type:host:id"
	dockerFP := &ContainerFingerprint{
		ResourceID: "docker:host1:nginx",
		HostID:     "host1",
		Hash:       "aaa111",
	}
	lxcFP := &ContainerFingerprint{
		ResourceID: "lxc:node1:101",
		HostID:     "node1",
		Hash:       "bbb222",
	}
	vmFP := &ContainerFingerprint{
		ResourceID: "vm:node1:200",
		HostID:     "node1",
		Hash:       "ccc333",
	}
	for _, fp := range []*ContainerFingerprint{dockerFP, lxcFP, vmFP} {
		if err := store.SaveFingerprint(fp); err != nil {
			t.Fatalf("SaveFingerprint error: %v", err)
		}
	}

	// No discoveries exist yet — all three should be reported as changed.
	changed, err := store.GetChangedResources()
	if err != nil {
		t.Fatalf("GetChangedResources error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed (no discoveries yet), got %d: %v", len(changed), changed)
	}

	// Save discoveries with matching fingerprint hashes.
	for _, d := range []*ResourceDiscovery{
		{ID: "docker:host1:nginx", ResourceType: ResourceTypeDocker, HostID: "host1", ResourceID: "nginx", Fingerprint: "aaa111"},
		{ID: "lxc:node1:101", ResourceType: ResourceTypeSystemContainer, HostID: "node1", ResourceID: "101", Fingerprint: "bbb222"},
		{ID: "vm:node1:200", ResourceType: ResourceTypeVM, HostID: "node1", ResourceID: "200", Fingerprint: "ccc333"},
	} {
		if err := store.Save(d); err != nil {
			t.Fatalf("Save error: %v", err)
		}
	}

	// All fingerprints match their discoveries — nothing should be changed.
	changed, err = store.GetChangedResources()
	if err != nil {
		t.Fatalf("GetChangedResources error: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("expected 0 changed (all match), got %d: %v", len(changed), changed)
	}

	// Update the LXC fingerprint to simulate a change.
	lxcFP.Hash = "bbb222_changed"
	if err := store.SaveFingerprint(lxcFP); err != nil {
		t.Fatalf("SaveFingerprint error: %v", err)
	}

	changed, err = store.GetChangedResources()
	if err != nil {
		t.Fatalf("GetChangedResources error: %v", err)
	}
	if len(changed) != 1 {
		t.Fatalf("expected 1 changed (LXC only), got %d: %v", len(changed), changed)
	}
	if changed[0] != "lxc:node1:101" {
		t.Fatalf("expected changed resource to be lxc:node1:101, got %s", changed[0])
	}
}

func TestStore_GetStaleResourcesUsesLastUpdatedTimestamp(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	id := MakeResourceID(ResourceTypeDocker, "host1", "web")
	if err := store.Save(&ResourceDiscovery{
		ID:           id,
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
	}); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	path := store.getFilePath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var saved ResourceDiscovery
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// First discovery happened long ago, but the record was updated recently.
	saved.DiscoveredAt = time.Now().Add(-48 * time.Hour)
	saved.UpdatedAt = time.Now().Add(-10 * time.Minute)
	data, err = json.Marshal(&saved)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	stale, err := store.GetStaleResources(time.Hour)
	if err != nil {
		t.Fatalf("GetStaleResources error: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("expected no stale discoveries when UpdatedAt is recent, got %v", stale)
	}

	// Once last update is old, it should be considered stale.
	saved.UpdatedAt = time.Now().Add(-48 * time.Hour)
	data, err = json.Marshal(&saved)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	stale, err = store.GetStaleResources(time.Hour)
	if err != nil {
		t.Fatalf("GetStaleResources error: %v", err)
	}
	if len(stale) != 1 || stale[0] != id {
		t.Fatalf("expected stale discovery %q, got %v", id, stale)
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

func TestStore_FingerprintAccessorsAndCleanup(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	origLimit := maxDiscoveryFileReadBytes
	maxDiscoveryFileReadBytes = 64
	t.Cleanup(func() {
		maxDiscoveryFileReadBytes = origLimit
	})

	id := MakeResourceID(ResourceTypeDocker, "host1", "oversized")
	if err := os.WriteFile(store.getFilePath(id), []byte(strings.Repeat("x", 128)), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if _, err := store.Get(id); err == nil || !strings.Contains(err.Error(), "exceeds max size") {
		t.Fatalf("expected max size error, got: %v", err)
	}
}

func TestStore_GetRejectsNonRegularDiscoveryFile(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	id := MakeResourceID(ResourceTypeDocker, "host1", "dir")
	if err := os.MkdirAll(store.getFilePath(id), 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	if _, err := store.Get(id); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected non-regular file error, got: %v", err)
	}
}

func TestStore_ListSkipsOversizedAndSymlinkDiscoveries(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	origLimit := maxDiscoveryFileReadBytes
	maxDiscoveryFileReadBytes = 4096
	t.Cleanup(func() {
		maxDiscoveryFileReadBytes = origLimit
	})

	valid := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "host1", "valid"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "valid",
		HostID:       "host1",
		ServiceName:  "Valid",
	}
	if err := store.Save(valid); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	oversizedPath := filepath.Join(store.dataDir, "oversized.enc")
	if err := os.WriteFile(oversizedPath, []byte(strings.Repeat("x", 8192)), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	symlinkPath := filepath.Join(store.dataDir, "symlink.enc")
	if err := os.Symlink(store.getFilePath(valid.ID), symlinkPath); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 1 || list[0].ID != valid.ID {
		t.Fatalf("expected only valid discovery, got: %#v", list)
	}
}

func TestStore_LoadFingerprintsSkipsOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	fingerprintDir := filepath.Join(dir, "discovery", "fingerprints")
	if err := os.MkdirAll(fingerprintDir, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	origLimit := maxFingerprintFileReadBytes
	maxFingerprintFileReadBytes = 256
	t.Cleanup(func() {
		maxFingerprintFileReadBytes = origLimit
	})

	valid := &ContainerFingerprint{
		ResourceID: "docker:host1:nginx",
		HostID:     "host1",
		Hash:       "abc123",
	}
	validData, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fingerprintDir, "valid.json"), validData, 0600); err != nil {
		t.Fatalf("WriteFile valid fingerprint error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fingerprintDir, "oversized.json"), []byte(strings.Repeat("x", 512)), 0600); err != nil {
		t.Fatalf("WriteFile oversized fingerprint error: %v", err)
	}

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	store.dataDir = file

	if ids := store.ListDiscoveryIDs(); ids != nil {
		t.Fatalf("expected nil IDs when dataDir is unreadable, got %v", ids)
	}
}

func TestStore_GetStaleResources(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	old := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "host1", "old"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "old",
		HostID:       "host1",
		DiscoveredAt: time.Now().Add(-2 * time.Hour),
	}
	fresh := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "host1", "fresh"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "fresh",
		HostID:       "host1",
		DiscoveredAt: time.Now().Add(-5 * time.Minute),
	}
	for _, d := range []*ResourceDiscovery{old, fresh} {
		if err := store.Save(d); err != nil {
			t.Fatalf("Save error: %v", err)
		}
	}

	// Save() sets UpdatedAt to time.Now(), so overwrite the on-disk files
	// (which List() reads) to simulate genuinely stale/fresh entries.
	for _, d := range []*ResourceDiscovery{old, fresh} {
		switch d.ResourceID {
		case "old":
			d.UpdatedAt = time.Now().Add(-2 * time.Hour)
		case "fresh":
			d.UpdatedAt = time.Now().Add(-5 * time.Minute)
		}
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if err := os.WriteFile(store.getFilePath(d.ID), data, 0600); err != nil {
			t.Fatalf("write error: %v", err)
		}
	}

	stale, err := store.GetStaleResources(time.Hour)
	if err != nil {
		t.Fatalf("GetStaleResources error: %v", err)
	}
	if len(stale) != 1 || stale[0] != old.ID {
		t.Fatalf("expected stale IDs [%q], got %v", old.ID, stale)
	}

	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	store.dataDir = file

	if _, err := store.GetStaleResources(time.Hour); err == nil {
		t.Fatalf("expected GetStaleResources to return list error")
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
