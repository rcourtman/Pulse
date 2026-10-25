package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// commitImport includes guest metadata in the same file transaction as settings
// and credentials. Keep the store's writer and mutation locks through commit so
// an older background snapshot cannot overwrite the imported file, and readers
// cannot observe imported metadata before its configuration has committed.
func (s *GuestMetadataStore) commitImport(tx *importTransaction, metadata map[string]*GuestMetadata) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrGuestMetadataStoreClosed
	}

	working := make(map[string]*GuestMetadata, len(metadata))
	for id, meta := range metadata {
		if meta == nil {
			continue
		}
		clone := cloneGuestMetadata(meta)
		clone.ID = id
		if clone.Tags == nil {
			clone.Tags = []string{}
		}
		working[id] = clone
	}
	data, err := json.Marshal(working)
	if err != nil {
		return fmt.Errorf("serialize imported guest metadata: %w", err)
	}
	if err := tx.StageFile(filepath.Join(s.dataPath, "guest_metadata.json"), data, 0600); err != nil {
		return fmt.Errorf("stage imported guest metadata: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Publish only after every file committed. A pending writer will now see
	// that this revision is durable instead of replaying a pre-import snapshot.
	s.metadata = working
	s.revision++
	s.attemptedRevision = s.revision
	s.lastWriteErr = nil
	return nil
}
