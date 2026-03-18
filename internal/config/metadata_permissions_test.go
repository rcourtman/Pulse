package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadataStores_SaveWithOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission mode assertions are not stable on windows")
	}

	testCases := []struct {
		name       string
		fileName   string
		writeStore func(dir string) error
	}{
		{
			name:     "guest",
			fileName: "guest_metadata.json",
			writeStore: func(dir string) error {
				return NewGuestMetadataStore(dir, nil).Set("guest:101", &GuestMetadata{CustomURL: "https://guest.local"})
			},
		},
		{
			name:     "host",
			fileName: "host_metadata.json",
			writeStore: func(dir string) error {
				return NewHostMetadataStore(dir, nil).Set("host:101", &HostMetadata{CustomURL: "https://host.local"})
			},
		},
		{
			name:     "docker",
			fileName: "docker_metadata.json",
			writeStore: func(dir string) error {
				return NewDockerMetadataStore(dir, nil).Set("host:container:101", &DockerMetadata{CustomURL: "https://docker.local"})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := filepath.Join(t.TempDir(), "metadata")
			require.NoError(t, tc.writeStore(storeDir))

			dirInfo, err := os.Stat(storeDir)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

			fileInfo, err := os.Stat(filepath.Join(storeDir, tc.fileName))
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
		})
	}
}
