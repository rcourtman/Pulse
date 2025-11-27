//go:build !windows

package main

import (
	"github.com/rs/zerolog"
)

// runAsWindowsService is a no-op on non-Windows platforms
func runAsWindowsService(_ Config, _ zerolog.Logger) error {
	return nil
}
