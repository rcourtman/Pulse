package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

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
