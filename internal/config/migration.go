package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// MigrationStatus tracks the status of a data migration.
type MigrationStatus struct {
	Version     string    `json:"version"`
	MigratedAt  time.Time `json:"migratedAt"`
	SourceFiles []string  `json:"sourceFiles"`
	TargetDir   string    `json:"targetDir"`
}

// filesToMigrate lists files that should be moved to the default org directory.
// NOTE: This migration is currently dormant - RunMigrationIfNeeded is not called anywhere.
// If enabled in the future, .encryption.key should be added to this list FIRST.
var filesToMigrate = []string{
	"nodes.enc",
	"system.json",
	"alerts.json",
	"notifications.json",
	"audit.db",
}

// MigrateToMultiTenant migrates existing data to the multi-tenant directory structure.
// It moves files from the root data directory to /orgs/default/ and creates symlinks
// for backward compatibility.
//
// Migration structure:
//
//	/data/
//	  orgs/
//	    default/
//	      nodes.enc       <- moved from /data/nodes.enc
//	      system.json     <- moved from /data/system.json
//	      alerts.json     <- moved from /data/alerts.json
//	  nodes.enc           <- symlink to orgs/default/nodes.enc
//	  system.json         <- symlink to orgs/default/system.json
//	  alerts.json         <- symlink to orgs/default/alerts.json
func MigrateToMultiTenant(dataDir string) error {
	if dataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	// Check if migration is needed
	defaultOrgDir := filepath.Join(dataDir, "orgs", "default")
	migrationMarker := filepath.Join(defaultOrgDir, ".migrated")

	// If migration marker exists, migration has already been completed
	if _, err := os.Stat(migrationMarker); err == nil {
		log.Debug().Str("data_dir", dataDir).Msg("Multi-tenant migration already completed")
		return nil
	}

	log.Info().Str("data_dir", dataDir).Msg("Starting multi-tenant data migration")

	// Create the default org directory
	if err := os.MkdirAll(defaultOrgDir, 0755); err != nil {
		return fmt.Errorf("failed to create default org directory: %w", err)
	}

	migratedFiles := []string{}
	skippedFiles := []string{}

	// Migrate each file
	for _, filename := range filesToMigrate {
		srcPath := filepath.Join(dataDir, filename)
		dstPath := filepath.Join(defaultOrgDir, filename)

		// Check if source file exists
		srcInfo, err := os.Lstat(srcPath)
		if os.IsNotExist(err) {
			log.Debug().Str("file", filename).Msg("File does not exist, skipping")
			skippedFiles = append(skippedFiles, filename)
			continue
		}
		if err != nil {
			log.Warn().Err(err).Str("file", filename).Msg("Error checking source file, skipping")
			skippedFiles = append(skippedFiles, filename)
			continue
		}

		// Skip if source is already a symlink (already migrated)
		if srcInfo.Mode()&os.ModeSymlink != 0 {
			log.Debug().Str("file", filename).Msg("File is already a symlink, skipping")
			skippedFiles = append(skippedFiles, filename)
			continue
		}

		// Check if destination already exists
		if _, err := os.Stat(dstPath); err == nil {
			log.Warn().Str("file", filename).Msg("Destination already exists, skipping")
			skippedFiles = append(skippedFiles, filename)
			continue
		}

		// Move the file
		if err := os.Rename(srcPath, dstPath); err != nil {
			// If rename fails (cross-device), try copy + delete
			if err := copyFile(srcPath, dstPath); err != nil {
				log.Error().Err(err).Str("file", filename).Msg("Failed to migrate file")
				continue
			}
			if err := os.Remove(srcPath); err != nil {
				log.Warn().Err(err).Str("file", filename).Msg("Failed to remove original file after copy")
			}
		}

		log.Info().Str("file", filename).Msg("Migrated file to default org directory")
		migratedFiles = append(migratedFiles, filename)

		// Create symlink for backward compatibility
		relPath, _ := filepath.Rel(dataDir, dstPath)
		if err := os.Symlink(relPath, srcPath); err != nil {
			log.Warn().Err(err).Str("file", filename).Msg("Failed to create symlink for backward compatibility")
		} else {
			log.Debug().Str("file", filename).Str("target", relPath).Msg("Created symlink for backward compatibility")
		}
	}

	// Create migration marker
	markerContent := fmt.Sprintf("migrated_at=%s\nversion=1.0\nmigrated_files=%d\nskipped_files=%d\n",
		time.Now().Format(time.RFC3339), len(migratedFiles), len(skippedFiles))
	if err := os.WriteFile(migrationMarker, []byte(markerContent), 0644); err != nil {
		log.Warn().Err(err).Msg("Failed to write migration marker")
	}

	log.Info().
		Int("migrated", len(migratedFiles)).
		Int("skipped", len(skippedFiles)).
		Str("data_dir", dataDir).
		Msg("Multi-tenant data migration completed")

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Get original file permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}

// IsMigrationNeeded checks if multi-tenant migration is needed.
func IsMigrationNeeded(dataDir string) bool {
	if dataDir == "" {
		return false
	}

	defaultOrgDir := filepath.Join(dataDir, "orgs", "default")
	migrationMarker := filepath.Join(defaultOrgDir, ".migrated")

	// If marker exists, migration is not needed
	if _, err := os.Stat(migrationMarker); err == nil {
		return false
	}

	// Check if any files to migrate exist
	for _, filename := range filesToMigrate {
		srcPath := filepath.Join(dataDir, filename)
		info, err := os.Lstat(srcPath)
		if err != nil {
			continue
		}
		// File exists and is not a symlink - migration needed
		if info.Mode()&os.ModeSymlink == 0 {
			return true
		}
	}

	return false
}

// RunMigrationIfNeeded checks if migration is needed and runs it.
func RunMigrationIfNeeded(dataDir string) error {
	if !IsMigrationNeeded(dataDir) {
		return nil
	}
	return MigrateToMultiTenant(dataDir)
}
