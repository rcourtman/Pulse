package config

import (
	"path/filepath"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func resolveSharedInstallationDataDir(configDir string) (string, error) {
	normalizedConfigDir, err := securityutil.NormalizeStorageDir(ResolveRuntimeDataDir(configDir))
	if err != nil {
		return "", err
	}

	orgsDir := filepath.Dir(normalizedConfigDir)
	if filepath.Base(orgsDir) != "orgs" {
		return normalizedConfigDir, nil
	}

	orgID := filepath.Base(normalizedConfigDir)
	if !isValidOrgID(orgID) {
		return normalizedConfigDir, nil
	}

	return filepath.Dir(orgsDir), nil
}
