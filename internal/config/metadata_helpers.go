package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// metadataFileLoader provides a generic way to load metadata from disk
type metadataFileLoader interface {
	LoadFromDisk() error
}

// loadMetadataFromFile reads metadata from a JSON file and unmarshals it into the provided map.
// The map pointer must be passed as a generic interface{} to allow runtime assignment.
func loadMetadataFromFile[T any](
	fs FileSystem,
	dataPath string,
	fileName string,
	maxFileSize int64,
	metadataMap *map[string]*T,
	logMsg string,
) error {
	filePath := filepath.Join(dataPath, fileName)

	log.Debug().Str("path", filePath).Msg("Loading " + logMsg + " from disk")

	data, err := readLimitedRegularFileFS(fs, filePath, maxFileSize)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("path", filePath).Msg(logMsg + " file does not exist yet")
			return nil
		}
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	if err := json.Unmarshal(data, metadataMap); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	log.Info().Int("count", len(*metadataMap)).Msg("Loaded " + logMsg)
	return nil
}

// saveMetadataToFile writes metadata to a JSON file using atomic write (write to temp, then rename).
func saveMetadataToFile[T any](
	fs FileSystem,
	dataPath string,
	fileName string,
	metadataMap map[string]*T,
	logMsg string,
) error {
	filePath := filepath.Join(dataPath, fileName)

	log.Debug().Str("path", filePath).Msg("Saving " + logMsg + " to disk")

	data, err := json.Marshal(metadataMap)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Restrict metadata persistence to owner-only access.
	if err := fs.MkdirAll(dataPath, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Write to temp file first for atomic operation
	tempFile := filePath + ".tmp"
	if err := fs.WriteFile(tempFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	// Rename temp file to actual file (atomic on most systems)
	if err := fs.Rename(tempFile, filePath); err != nil {
		return fmt.Errorf("failed to rename metadata file: %w", err)
	}

	log.Debug().Str("path", filePath).Int("entries", len(metadataMap)).Msg(logMsg + " saved successfully")

	return nil
}
