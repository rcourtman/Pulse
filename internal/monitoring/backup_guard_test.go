package monitoring

import "testing"

func TestShouldPreserveBackups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		nodeCount          int
		hadSuccessfulNode  bool
		storagesWithBackup int
		contentSuccess     int
		want               bool
	}{
		{
			name:               "no successful nodes with nodes present",
			nodeCount:          2,
			hadSuccessfulNode:  false,
			storagesWithBackup: 0,
			contentSuccess:     0,
			want:               true,
		},
		{
			name:               "no nodes skips preservation",
			nodeCount:          0,
			hadSuccessfulNode:  false,
			storagesWithBackup: 0,
			contentSuccess:     0,
			want:               false,
		},
		{
			name:               "storages present but no content success",
			nodeCount:          3,
			hadSuccessfulNode:  true,
			storagesWithBackup: 5,
			contentSuccess:     0,
			want:               true,
		},
		{
			name:               "storages present with successes",
			nodeCount:          3,
			hadSuccessfulNode:  true,
			storagesWithBackup: 5,
			contentSuccess:     2,
			want:               false,
		},
		{
			name:               "no storages and no successes but had success elsewhere",
			nodeCount:          1,
			hadSuccessfulNode:  true,
			storagesWithBackup: 0,
			contentSuccess:     0,
			want:               false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldPreserveBackups(tt.nodeCount, tt.hadSuccessfulNode, tt.storagesWithBackup, tt.contentSuccess)
			if got != tt.want {
				t.Fatalf("shouldPreserveBackups() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldPreservePBSBackups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		datastoreCount   int
		datastoreFetches int
		want             bool
	}{
		{
			name:             "all datastores failed",
			datastoreCount:   3,
			datastoreFetches: 0,
			want:             true,
		},
		{
			name:             "no datastores skips preservation",
			datastoreCount:   0,
			datastoreFetches: 0,
			want:             false,
		},
		{
			name:             "some datastores succeeded",
			datastoreCount:   3,
			datastoreFetches: 2,
			want:             false,
		},
		{
			name:             "all datastores succeeded",
			datastoreCount:   3,
			datastoreFetches: 3,
			want:             false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldPreservePBSBackups(tt.datastoreCount, tt.datastoreFetches)
			if got != tt.want {
				t.Fatalf("shouldPreservePBSBackups() = %v, want %v", got, tt.want)
			}
		})
	}
}
