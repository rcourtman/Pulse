package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestIsInherentlySharedStorageType(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		want        bool
	}{
		{"rbd is shared", "rbd", true},
		{"RBD uppercase is shared", "RBD", true},
		{"cephfs is shared", "cephfs", true},
		{"pbs is shared", "pbs", true},
		{"PBS uppercase is shared", "PBS", true},
		{"glusterfs is shared", "glusterfs", true},
		{"nfs is shared", "nfs", true},
		{"NFS uppercase is shared", "NFS", true},
		{"cifs is shared", "cifs", true},
		{"smb is shared", "smb", true},
		{"iscsi is shared", "iscsi", true},
		{"iscsidirect is shared", "iscsidirect", true},
		{"dir is not shared", "dir", false},
		{"lvm is not shared", "lvm", false},
		{"lvmthin is not shared", "lvmthin", false},
		{"zfspool is not shared", "zfspool", false},
		{"zfs is not shared", "zfs", false},
		{"local is not shared", "local", false},
		{"empty string is not shared", "", false},
		{"whitespace is not shared", "  ", false},
		{"rbd with spaces is shared", "  rbd  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInherentlySharedStorageType(tt.storageType); got != tt.want {
				t.Errorf("isInherentlySharedStorageType(%q) = %v, want %v", tt.storageType, got, tt.want)
			}
		})
	}
}

func TestStorageContentQueryable(t *testing.T) {
	tests := []struct {
		name    string
		storage proxmox.Storage
		want    bool
	}{
		{
			name: "disabled storage skipped",
			storage: proxmox.Storage{
				Storage: "local",
				Enabled: 0,
				Active:  1,
			},
			want: false,
		},
		{
			name: "active storage allowed",
			storage: proxmox.Storage{
				Storage: "local-zfs",
				Enabled: 1,
				Active:  1,
			},
			want: true,
		},
		{
			name: "pbs backup storage allowed even when inactive",
			storage: proxmox.Storage{
				Storage: "pbs-datastore",
				Type:    "pbs",
				Content: "backup",
				Enabled: 1,
				Active:  0,
			},
			want: true,
		},
		{
			name: "non-backup inactive storage skipped",
			storage: proxmox.Storage{
				Storage: "nfs-images",
				Content: "images",
				Enabled: 1,
				Active:  0,
			},
			want: false,
		},
		{
			name: "non-pbs inactive backup storage allowed (shared storage)",
			storage: proxmox.Storage{
				Storage: "backup-dir",
				Type:    "dir",
				Content: "backup",
				Enabled: 1,
				Active:  0,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := storageContentQueryable(tt.storage); got != tt.want {
				t.Fatalf("storageContentQueryable() = %v, want %v", got, tt.want)
			}
		})
	}
}
