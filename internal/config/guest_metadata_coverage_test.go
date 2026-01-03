package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuestMetadataStore_SaveErrors(t *testing.T) {
	tempDir := t.TempDir()

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, mkdirError: errors.New("mkdir error")}
	store := NewGuestMetadataStore(tempDir, mfs)

	// Trigger save via Set
	err := store.Set("id1", &GuestMetadata{LastKnownName: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create data directory")
}

func TestGuestMetadataStore_LoadErrors(t *testing.T) {
	tempDir := t.TempDir()

	mfs := &mockFSError{FileSystem: defaultFileSystem{}}
	store := NewGuestMetadataStore(tempDir, mfs)

	mfs.readError = errors.New("read error")
	err := store.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}
