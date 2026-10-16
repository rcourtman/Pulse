package monitoring

import (
	"errors"
	"testing"
)

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

func TestShouldReuseCachedPBSBackups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "404 datastore missing should not reuse cache",
			err:  errors.New("API error 404: datastore 'archive' does not exist"),
			want: false,
		},
		{
			name: "400 namespace missing should not reuse cache",
			err:  errors.New("API error 400: namespace '/old' not found"),
			want: false,
		},
		{
			name: "400 invalid backup group should not reuse cache",
			err:  errors.New("API error 400: invalid backup group"),
			want: false,
		},
		{
			name: "500 server error should reuse cache",
			err:  errors.New("API error 500: internal server error"),
			want: true,
		},
		{
			name: "timeout should reuse cache",
			err:  errors.New("Get \"https://pbs/api2/json\": context deadline exceeded"),
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldReuseCachedPBSBackups(tt.err)
			if got != tt.want {
				t.Fatalf("shouldReuseCachedPBSBackups() = %v, want %v", got, tt.want)
			}
		})
	}
}
