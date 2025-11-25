package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// handleDownloadUnifiedInstallScript serves the universal install.sh script
func (r *Router) handleDownloadUnifiedInstallScript(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scriptPath := filepath.Join(r.config.AppRoot, "scripts", "install.sh")

	// Check if file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Error().Str("path", scriptPath).Msg("Unified install script not found")
		http.Error(w, "Install script not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/x-shellscript")
	w.Header().Set("Content-Disposition", "inline; filename=\"install.sh\"")
	http.ServeFile(w, req, scriptPath)
}

// handleDownloadUnifiedInstallScriptPS serves the universal install.ps1 script
func (r *Router) handleDownloadUnifiedInstallScriptPS(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scriptPath := filepath.Join(r.config.AppRoot, "scripts", "install.ps1")

	// Check if file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Error().Str("path", scriptPath).Msg("Unified PowerShell install script not found")
		http.Error(w, "Install script not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "inline; filename=\"install.ps1\"")
	http.ServeFile(w, req, scriptPath)
}

// handleDownloadUnifiedAgent serves the pulse-agent binary
func (r *Router) handleDownloadUnifiedAgent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For now, we only have the locally built binary.
	// In production, this would look up the correct binary for the requested OS/Arch.
	// Query params: ?os=linux&arch=amd64

	osName := req.URL.Query().Get("os")
	arch := req.URL.Query().Get("arch")

	if osName == "" {
		osName = "linux" // Default
	}
	if arch == "" {
		arch = "amd64" // Default
	}

	// Normalize OS
	osName = strings.ToLower(osName)
	if osName == "darwin" {
		osName = "macos"
	}

	// In dev mode, we just serve the binary we built in the root
	// In prod, we'd look in a dist folder
	binaryName := "pulse-agent"
	if osName == "windows" {
		binaryName = "pulse-agent.exe"
	}

	// Try to find the binary
	// 1. Check root (dev)
	binaryPath := filepath.Join(r.config.AppRoot, binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// 2. Check dist folder (prod/build)
		binaryPath = filepath.Join(r.config.AppRoot, "dist", fmt.Sprintf("%s-%s", osName, arch), binaryName)
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			// Fallback for dev: just serve the root binary regardless of requested OS/Arch
			// This allows testing the flow even if cross-compilation hasn't happened
			binaryPath = filepath.Join(r.config.AppRoot, "pulse-agent")
			if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
				log.Error().Str("path", binaryPath).Msg("Unified agent binary not found")
				http.Error(w, "Agent binary not found", http.StatusNotFound)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", binaryName))
	http.ServeFile(w, req, binaryPath)
}
