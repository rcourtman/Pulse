package unifiedresources

import (
	"testing"
)

// --- storage_consumers.go helpers ---

// --- proxmoxStorageNameFromDevice ---

func TestProxmoxStorageNameFromDevice(t *testing.T) {
	tests := []struct {
		device   string
		expected string
	}{
		{"local-lvm:vm-100-disk-0", "local-lvm"},
		{"ceph-storage:vm-200-disk-1", "ceph-storage"},
		{"nocolon", ""},
		{"", ""},
		{"  ", ""},
		{":leadingcolon", ""},
	}
	for _, tt := range tests {
		got := proxmoxStorageNameFromDevice(tt.device)
		if got != tt.expected {
			t.Errorf("proxmoxStorageNameFromDevice(%q) = %q, want %q", tt.device, got, tt.expected)
		}
	}
}

// --- normalizeStorageLookupName ---

func TestNormalizeStorageLookupName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Local-LVM", "local-lvm"},
		{"  /nfs-share/ ", "nfs-share"},
		{"", ""},
		{"  ", ""},
		{"already-lower", "already-lower"},
	}
	for _, tt := range tests {
		got := normalizeStorageLookupName(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeStorageLookupName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- normalizeStoragePath ---

func TestNormalizeStoragePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/mnt/pve/storage", "/mnt/pve/storage"},
		{" /mnt/pve/storage/ ", "/mnt/pve/storage"},
		{"", ""},
		{"  ", ""},
		{".", ""},
		{"/mnt/../mnt/pve", "/mnt/pve"},
	}
	for _, tt := range tests {
		got := normalizeStoragePath(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeStoragePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- storagePathMatchesDevice ---

func TestStoragePathMatchesDevice(t *testing.T) {
	tests := []struct {
		storagePath string
		devicePath  string
		expected    bool
	}{
		{"/mnt/pve/storage", "/mnt/pve/storage", true},
		{"/mnt/pve/storage", "/mnt/pve/storage/images/vm-100-disk-0", true},
		{"/mnt/pve/storage", "/mnt/pve/storagex", false},
		{"/mnt/pve/storage", "/mnt/pve/other", false},
		{"", "/mnt/pve/storage", false},
		{"/mnt/pve/storage", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got := storagePathMatchesDevice(tt.storagePath, tt.devicePath)
		if got != tt.expected {
			t.Errorf("storagePathMatchesDevice(%q, %q) = %v, want %v", tt.storagePath, tt.devicePath, got, tt.expected)
		}
	}
}

// --- normalizeStorageNodeName ---

func TestNormalizeStorageNodeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Node1", "node1"},
		{"  PVE-Host.local ", "pve-host"},
		{"pve-host.local", "pve-host"},
		{"already-clean", "already-clean"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeStorageNodeName(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeStorageNodeName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- storageSupportsNode ---

func TestStorageSupportsNode_EmptyGuestNode(t *testing.T) {
	resource := &Resource{
		Storage: &StorageMeta{},
		Proxmox: &ProxmoxData{},
	}
	// Empty guest node means any storage matches.
	if !storageSupportsNode(resource, "") {
		t.Error("empty guest node should match any storage")
	}
}

func TestStorageSupportsNode_MatchByNodeName(t *testing.T) {
	resource := &Resource{
		Storage: &StorageMeta{},
		Proxmox: &ProxmoxData{NodeName: "pve-node1"},
	}
	if !storageSupportsNode(resource, "PVE-Node1") {
		t.Error("should match storage by proxmox node name (case-insensitive)")
	}
}

func TestStorageSupportsNode_MatchByNodesList(t *testing.T) {
	resource := &Resource{
		Storage: &StorageMeta{Nodes: []string{"node1", "node2", "node3"}},
		Proxmox: &ProxmoxData{NodeName: "other-node"},
	}
	if !storageSupportsNode(resource, "Node2") {
		t.Error("should match when guest node is in storage's node list")
	}
}

func TestStorageSupportsNode_SharedStorageMatchesAny(t *testing.T) {
	resource := &Resource{
		Storage: &StorageMeta{Shared: true},
		Proxmox: &ProxmoxData{NodeName: "node-a"},
	}
	if !storageSupportsNode(resource, "different-node") {
		t.Error("shared storage should match any node")
	}
}

func TestStorageSupportsNode_NoMatch(t *testing.T) {
	resource := &Resource{
		Storage: &StorageMeta{Nodes: []string{"node1"}},
		Proxmox: &ProxmoxData{NodeName: "node-a"},
	}
	if storageSupportsNode(resource, "unrelated-node") {
		t.Error("should not match unrelated node on non-shared storage")
	}
}

func TestStorageSupportsNode_NilResource(t *testing.T) {
	if storageSupportsNode(nil, "node1") {
		t.Error("nil resource should not support any node")
	}
}

func TestStorageSupportsNode_DotLocalSuffix(t *testing.T) {
	resource := &Resource{
		Storage: &StorageMeta{},
		Proxmox: &ProxmoxData{NodeName: "pve-host.local"},
	}
	if !storageSupportsNode(resource, "pve-host") {
		t.Error("should match with .local suffix stripped")
	}
}

// --- isStorageConsumerResource ---

func TestIsStorageConsumerResource_VM(t *testing.T) {
	resource := &Resource{Type: ResourceTypeVM, Proxmox: &ProxmoxData{}}
	if !isStorageConsumerResource(resource) {
		t.Error("VM should be a storage consumer")
	}
}

func TestIsStorageConsumerResource_Container(t *testing.T) {
	resource := &Resource{Type: ResourceTypeSystemContainer, Proxmox: &ProxmoxData{}}
	if !isStorageConsumerResource(resource) {
		t.Error("system container should be a storage consumer")
	}
}

func TestIsStorageConsumerResource_Storage(t *testing.T) {
	resource := &Resource{Type: ResourceTypeStorage, Proxmox: &ProxmoxData{}}
	if isStorageConsumerResource(resource) {
		t.Error("storage resource should not be a storage consumer")
	}
}

func TestIsStorageConsumerResource_NoProxmox(t *testing.T) {
	resource := &Resource{Type: ResourceTypeVM}
	if isStorageConsumerResource(resource) {
		t.Error("VM without proxmox data should not be a storage consumer")
	}
}

func TestIsStorageConsumerResource_Nil(t *testing.T) {
	if isStorageConsumerResource(nil) {
		t.Error("nil should not be a storage consumer")
	}
}

// --- pbsNamespaceMatchesInstance ---

func TestPbsNamespaceMatchesInstance(t *testing.T) {
	tests := []struct {
		namespace string
		instance  string
		expected  bool
	}{
		{"pbs-server", "pbs-server", true},
		{"PBS-Server", "pbs-server", true},
		{"server", "pbs-server", true}, // suffix match
		{"pbs-server", "server", true}, // reverse suffix match
		{"other", "pbs-server", false},
		{"", "pbs-server", false},
		{"pbs-server", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got := pbsNamespaceMatchesInstance(tt.namespace, tt.instance)
		if got != tt.expected {
			t.Errorf("pbsNamespaceMatchesInstance(%q, %q) = %v, want %v", tt.namespace, tt.instance, got, tt.expected)
		}
	}
}

// --- filterPBSBackupCandidatesByType ---

func TestFilterPBSBackupCandidatesByType_VM(t *testing.T) {
	vm := &Resource{ID: "vm-1", Type: ResourceTypeVM}
	ct := &Resource{ID: "ct-1", Type: ResourceTypeSystemContainer}
	candidates := []*Resource{vm, ct}

	filtered := filterPBSBackupCandidatesByType(candidates, "vm")
	if len(filtered) != 1 || filtered[0].ID != "vm-1" {
		t.Errorf("expected only VM, got %d results", len(filtered))
	}
}

func TestFilterPBSBackupCandidatesByType_Container(t *testing.T) {
	vm := &Resource{ID: "vm-1", Type: ResourceTypeVM}
	ct := &Resource{ID: "ct-1", Type: ResourceTypeSystemContainer}
	candidates := []*Resource{vm, ct}

	filtered := filterPBSBackupCandidatesByType(candidates, "ct")
	if len(filtered) != 1 || filtered[0].ID != "ct-1" {
		t.Errorf("expected only container, got %d results", len(filtered))
	}
}

func TestFilterPBSBackupCandidatesByType_EmptyType(t *testing.T) {
	candidates := []*Resource{
		{ID: "vm-1", Type: ResourceTypeVM},
		{ID: "ct-1", Type: ResourceTypeSystemContainer},
	}
	filtered := filterPBSBackupCandidatesByType(candidates, "")
	if len(filtered) != 2 {
		t.Errorf("empty type should return all candidates, got %d", len(filtered))
	}
}

func TestFilterPBSBackupCandidatesByType_CaseInsensitive(t *testing.T) {
	vm := &Resource{ID: "vm-1", Type: ResourceTypeVM}
	filtered := filterPBSBackupCandidatesByType([]*Resource{vm}, " VM ")
	if len(filtered) != 1 {
		t.Errorf("should be case-insensitive, got %d results", len(filtered))
	}
}
