package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		{
			name:     "host continuity",
			fileName: "host_continuity.json",
			maxBytes: maxHostContinuityFileSizeBytes,
			load: func(dir string) error {
				return NewHostContinuityStore(dir, nil).Load()
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
		{
			name:     "host continuity",
			fileName: "host_continuity.json",
			load: func(dir string) error {
				return NewHostContinuityStore(dir, nil).Load()
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

// A store that saved more than its own Load accepts would start empty after
// the next restart and replace the file at its next write.
func TestMetadataStores_SaveRefusesWhatLoadWouldReject(t *testing.T) {
	// One record that leaves about 1 KiB below maxBytes once saved.
	nearly := func(maxBytes int64) []string {
		return []string{string(bytes.Repeat([]byte("x"), int(maxBytes)-1024))}
	}
	big := []string{string(bytes.Repeat([]byte("x"), 4096))}

	t.Run("guest", func(t *testing.T) {
		dir := t.TempDir()
		store := NewGuestMetadataStore(dir, nil)
		require.NoError(t, store.ReplaceAll(map[string]*GuestMetadata{"kept": {Notes: nearly(maxGuestMetadataFileSizeBytes)}}))

		require.ErrorIs(t, store.Set("refused", &GuestMetadata{Notes: big}), ErrMetadataFileTooLarge)
		require.Nil(t, store.Get("refused"))
		require.ErrorIs(t, store.Set("kept", &GuestMetadata{Notes: append(nearly(maxGuestMetadataFileSizeBytes), big...)}), ErrMetadataFileTooLarge)
		require.Len(t, store.Get("kept").Notes, 1)
		require.ErrorIs(t, store.ReplaceAll(map[string]*GuestMetadata{"refused": {Notes: nearly(2 * maxGuestMetadataFileSizeBytes)}}), ErrMetadataFileTooLarge)
		require.NotNil(t, store.Get("kept"))
		require.NoError(t, store.Set("small", &GuestMetadata{Description: "still fits"}))

		// Async admission and RememberIdentity are held to the same limit and
		// roll back memory so later saves and Close() keep working.
		require.ErrorIs(t, store.SetAsync("refused", &GuestMetadata{Notes: big}), ErrMetadataFileTooLarge)
		require.Nil(t, store.Get("refused"))
		require.True(t, store.WaitForPendingWrites(time.Minute))
		require.NoError(t, store.Close(5*time.Second))

		reloaded := NewGuestMetadataStore(dir, nil)
		require.NoError(t, reloaded.Load())
		require.NotNil(t, reloaded.Get("kept"))
		require.NotNil(t, reloaded.Get("small"))
		require.Nil(t, reloaded.Get("refused"))
	})

	t.Run("docker", func(t *testing.T) {
		dir := t.TempDir()
		store := NewDockerMetadataStore(dir, nil)
		require.NoError(t, store.ReplaceAll(map[string]*DockerMetadata{"kept": {Notes: nearly(maxDockerMetadataFileSizeBytes)}}))
		require.NoError(t, store.SetHostMetadata("host", &DockerHostMetadata{CustomDisplayName: "kept"}))

		require.ErrorIs(t, store.Set("refused", &DockerMetadata{Notes: big}), ErrMetadataFileTooLarge)
		require.Nil(t, store.Get("refused"))
		require.ErrorIs(t, store.SetHostMetadata("refused", &DockerHostMetadata{Notes: big}), ErrMetadataFileTooLarge)
		require.Nil(t, store.GetHostMetadata("refused"))
		require.ErrorIs(t, store.SetHostMetadata("host", &DockerHostMetadata{Notes: big}), ErrMetadataFileTooLarge)
		require.Equal(t, "kept", store.GetHostMetadata("host").CustomDisplayName)
		require.ErrorIs(t, store.ReplaceAll(map[string]*DockerMetadata{"refused": {Notes: nearly(2 * maxDockerMetadataFileSizeBytes)}}), ErrMetadataFileTooLarge)
		require.NotNil(t, store.Get("kept"))
		require.NoError(t, store.Set("small", &DockerMetadata{Description: "still fits"}))

		reloaded := NewDockerMetadataStore(dir, nil)
		require.NoError(t, reloaded.Load())
		require.NotNil(t, reloaded.Get("kept"))
		require.NotNil(t, reloaded.Get("small"))
		require.NotNil(t, reloaded.GetHostMetadata("host"))
	})

	t.Run("host", func(t *testing.T) {
		dir := t.TempDir()
		store := NewHostMetadataStore(dir, nil)
		off := false

		// Set, which every signed-in user reaches, stops short of the file
		// limit, so the command policy of a host can still be saved.
		require.NoError(t, store.Set("kept", &HostMetadata{Notes: nearly(maxHostAnnotationsFileSizeBytes)}))
		require.ErrorIs(t, store.Set("refused", &HostMetadata{Notes: big}), ErrMetadataFileTooLarge)
		require.Nil(t, store.Get("refused"))
		require.NoError(t, store.SetCommandsEnabled("host", &off))
		require.ErrorIs(t, store.Set("host", &HostMetadata{Notes: big}), ErrMetadataFileTooLarge)
		require.Empty(t, store.Get("host").Notes)
		require.False(t, *store.Get("host").CommandsEnabled)
		require.NoError(t, store.Delete("absent"))

		// Nothing is written above what Load accepts.
		require.ErrorIs(t, store.ReplaceAll(map[string]*HostMetadata{"refused": {Notes: nearly(2 * maxHostMetadataFileSizeBytes)}}), ErrMetadataFileTooLarge)
		require.NotNil(t, store.Get("kept"))
		require.NoError(t, store.ReplaceAll(map[string]*HostMetadata{
			"kept": {Notes: nearly(maxHostMetadataFileSizeBytes)},
			"host": {CommandsEnabled: &off},
		}))
		refused := string(bytes.Repeat([]byte("h"), 2048))
		require.ErrorIs(t, store.SetCommandsEnabled(refused, &off), ErrMetadataFileTooLarge)
		require.Nil(t, store.Get(refused))

		reloaded := NewHostMetadataStore(dir, nil)
		require.NoError(t, reloaded.Load())
		require.NotNil(t, reloaded.Get("kept"))
		require.Nil(t, reloaded.Get("refused"))
		policy := reloaded.Get("host")
		require.NotNil(t, policy)
		require.NotNil(t, policy.CommandsEnabled)
		require.False(t, *policy.CommandsEnabled)
	})

	t.Run("host continuity journal", func(t *testing.T) {
		dir := t.TempDir()
		store := NewHostContinuityStore(dir, nil)
		require.NoError(t, store.Upsert(HostContinuityEntry{HostID: "kept", Hostname: "kept.example"}))

		oversized := string(bytes.Repeat([]byte("h"), int(maxHostContinuityFileSizeBytes)+1024))
		require.ErrorIs(t, store.ObserveReport(HostContinuityEntry{HostID: "refused", Hostname: oversized}), ErrMetadataFileTooLarge)
		_, exists := store.Get("refused")
		require.False(t, exists)
		require.NoError(t, store.Close())

		reloaded := NewHostContinuityStore(dir, nil)
		require.NoError(t, reloaded.LoadError())
		_, kept := reloaded.Get("kept")
		require.True(t, kept)
	})

	t.Run("docker report order journal", func(t *testing.T) {
		dir := t.TempDir()
		store := NewDockerReportOrderStore(dir, nil)
		require.NoError(t, store.Upsert(DockerReportOrderEntry{HostID: "kept", StreamID: "stream-1", Sequence: 1}))

		oversized := string(bytes.Repeat([]byte("d"), maxDockerReportOrderFileSizeBytes+1024))
		require.ErrorIs(t, store.Upsert(DockerReportOrderEntry{HostID: "refused", StreamID: oversized, Sequence: 1}), ErrMetadataFileTooLarge)
		_, exists := store.Get("refused")
		require.False(t, exists)

		reloaded := NewDockerReportOrderStore(dir, nil)
		require.NoError(t, reloaded.LoadError())
		_, kept := reloaded.Get("kept")
		require.True(t, kept)
	})
}
