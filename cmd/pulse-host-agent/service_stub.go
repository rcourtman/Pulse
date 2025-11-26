//go:build !windows
// +build !windows

package main

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rs/zerolog"
)

// runAsWindowsService is a no-op on non-Windows platforms
func runAsWindowsService(cfg hostagent.Config, logger zerolog.Logger) error {
	return nil
}

// runServiceDebug is a no-op on non-Windows platforms
func runServiceDebug(cfg hostagent.Config, logger zerolog.Logger) error {
	return nil
}
