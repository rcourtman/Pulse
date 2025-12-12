package api

import (
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

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	scriptPath := "/opt/pulse/scripts/install.sh"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// Fallback to project root (dev environment)
		scriptPath = filepath.Join(r.projectRoot, "scripts", "install.sh")
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			log.Error().Str("path", scriptPath).Msg("Unified install script not found")
			http.Error(w, "Install script not found", http.StatusNotFound)
			return
		}
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

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	scriptPath := "/opt/pulse/scripts/install.ps1"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// Fallback to project root (dev environment)
		scriptPath = filepath.Join(r.projectRoot, "scripts", "install.ps1")
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			log.Error().Str("path", scriptPath).Msg("Unified PowerShell install script not found")
			http.Error(w, "Install script not found", http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "inline; filename=\"install.ps1\"")
	http.ServeFile(w, req, scriptPath)
}

// normalizeUnifiedAgentArch normalizes architecture strings for the unified agent.
func normalizeUnifiedAgentArch(arch string) string {
	arch = strings.ToLower(strings.TrimSpace(arch))
	switch arch {
	case "linux-amd64", "amd64", "x86_64":
		return "linux-amd64"
	case "linux-arm64", "arm64", "aarch64":
		return "linux-arm64"
	case "linux-armv7", "armv7", "armv7l", "armhf":
		return "linux-armv7"
	case "linux-armv6", "armv6":
		return "linux-armv6"
	case "linux-386", "386", "i386", "i686":
		return "linux-386"
	case "darwin-amd64", "macos-amd64":
		return "darwin-amd64"
	case "darwin-arm64", "macos-arm64":
		return "darwin-arm64"
	case "windows-amd64":
		return "windows-amd64"
	case "windows-arm64":
		return "windows-arm64"
	case "windows-386":
		return "windows-386"
	default:
		return ""
	}
}

// handleDownloadUnifiedAgent serves the pulse-agent binary
func (r *Router) handleDownloadUnifiedAgent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	archParam := strings.TrimSpace(req.URL.Query().Get("arch"))
	searchPaths := make([]string, 0, 6)

	if normalized := normalizeUnifiedAgentArch(archParam); normalized != "" {
		searchPaths = append(searchPaths,
			filepath.Join(pulseBinDir(), "pulse-agent-"+normalized),
			filepath.Join("/opt/pulse", "pulse-agent-"+normalized),
			filepath.Join("/app", "pulse-agent-"+normalized),
			filepath.Join(r.projectRoot, "bin", "pulse-agent-"+normalized),
		)
	}

	// Default locations (host architecture)
	searchPaths = append(searchPaths,
		filepath.Join(pulseBinDir(), "pulse-agent"),
		"/opt/pulse/pulse-agent",
		filepath.Join("/app", "pulse-agent"),
		filepath.Join(r.projectRoot, "bin", "pulse-agent"),
	)

	for _, candidate := range searchPaths {
		if candidate == "" {
			continue
		}

		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}

		checksum, err := r.cachedSHA256(candidate, info)
		if err != nil {
			log.Error().Err(err).Str("path", candidate).Msg("Failed to compute unified agent checksum")
			continue
		}

		file, err := os.Open(candidate)
		if err != nil {
			log.Error().Err(err).Str("path", candidate).Msg("Failed to open unified agent binary for download")
			continue
		}

		w.Header().Set("X-Checksum-Sha256", checksum)
		http.ServeContent(w, req, filepath.Base(candidate), info.ModTime(), file)
		file.Close()
		return
	}

	http.Error(w, "Agent binary not found", http.StatusNotFound)
}
