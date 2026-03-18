package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadataStores_LoadRejectsOversizedFiles(t *testing.T) {
	testCases := []struct {
		name     string
		fileName string
		maxBytes int64
		load     func(dir string) error
	}{
		{
			name:     "guest",
			fileName: "guest_metadata.json",
			maxBytes: maxGuestMetadataFileSizeBytes,
			load: func(dir string) error {
				return NewGuestMetadataStore(dir, nil).Load()
			},
		},
		{
			name:     "host",
			fileName: "host_metadata.json",
			maxBytes: maxHostMetadataFileSizeBytes,
			load: func(dir string) error {
				return NewHostMetadataStore(dir, nil).Load()
			},
		},
		{
			name:     "docker",
			fileName: "docker_metadata.json",
			maxBytes: maxDockerMetadataFileSizeBytes,
			load: func(dir string) error {
				return NewDockerMetadataStore(dir, nil).Load()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, tc.fileName)
			payload := bytes.Repeat([]byte("x"), int(tc.maxBytes+1))
			require.NoError(t, os.WriteFile(filePath, payload, 0o600))

			err := tc.load(dir)
			require.Error(t, err)
			require.Contains(t, err.Error(), "exceeds max size")
		})
	}
}

func TestMetadataStores_LoadRejectsNonRegularFiles(t *testing.T) {
	testCases := []struct {
		name     string
		fileName string
		load     func(dir string) error
	}{
		{
			name:     "guest",
			fileName: "guest_metadata.json",
			load: func(dir string) error {
				return NewGuestMetadataStore(dir, nil).Load()
			},
		},
		{
			name:     "host",
			fileName: "host_metadata.json",
			load: func(dir string) error {
				return NewHostMetadataStore(dir, nil).Load()
			},
		},
		{
			name:     "docker",
			fileName: "docker_metadata.json",
			load: func(dir string) error {
				return NewDockerMetadataStore(dir, nil).Load()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, tc.fileName)
			require.NoError(t, os.Mkdir(filePath, 0o700))

			err := tc.load(dir)
			require.Error(t, err)
			require.Contains(t, err.Error(), "non-regular file")
		})
	}
}
