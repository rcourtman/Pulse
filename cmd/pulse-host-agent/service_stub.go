//go:build !windows

package main

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rs/zerolog"
)

// runAsWindowsService is a no-op on non-Windows platforms
func runAsWindowsService(_ hostagent.Config, _ zerolog.Logger) error {
	return nil
}
