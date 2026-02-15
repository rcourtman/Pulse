package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func (r *Router) handleDownloadUnifiedInstallScript(w http.ResponseWriter, req *http.Request) {
	handleDownloadInstallScriptCommon(w, req, "/opt/pulse/scripts/install.sh", filepath.Join(r.projectRoot, "scripts", "install.sh"), "install.sh", "text/x-shellscript", r.proxyInstallScriptFromGitHub)
}

func (r *Router) handleDownloadUnifiedInstallScriptPS(w http.ResponseWriter, req *http.Request) {
	handleDownloadInstallScriptCommon(w, req, "/opt/pulse/scripts/install.ps1", filepath.Join(r.projectRoot, "scripts", "install.ps1"), "install.ps1", "text/plain", r.proxyInstallScriptFromGitHub)
}

type proxyFunc func(http.ResponseWriter, *http.Request, string)

func handleDownloadInstallScriptCommon(w http.ResponseWriter, req *http.Request, prodPath, fallbackPath, scriptName, contentType string, fallbackProxy proxyFunc) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	scriptPath := prodPath
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptPath = fallbackPath
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			log.Info().Msgf("Local %s not found, proxying from GitHub releases", scriptName)
			fallbackProxy(w, req, scriptName)
			return
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+scriptName+"\"")
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
	case "freebsd-amd64":
		return "freebsd-amd64"
	case "freebsd-arm64":
		return "freebsd-arm64"
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

	// Validate architecture if provided
	if archParam != "" && normalizeUnifiedAgentArch(archParam) == "" {
		http.Error(w, "Invalid architecture specified", http.StatusBadRequest)
		return
	}

	searchPaths := make([]string, 0, 6)

	// If a specific architecture is requested, only look for that architecture
	// Do NOT fall back to generic binary - that could serve the wrong architecture
	normalized := normalizeUnifiedAgentArch(archParam)
	if normalized != "" {
		searchPaths = append(searchPaths,
			filepath.Join(pulseBinDir(), "pulse-agent-"+normalized),
			filepath.Join("/opt/pulse", "pulse-agent-"+normalized),
			filepath.Join("/app", "pulse-agent-"+normalized),
			filepath.Join(r.projectRoot, "bin", "pulse-agent-"+normalized),
		)
	} else {
		// No specific architecture requested - allow fallback to generic binary
		searchPaths = append(searchPaths,
			filepath.Join(pulseBinDir(), "pulse-agent"),
			"/opt/pulse/pulse-agent",
			filepath.Join("/app", "pulse-agent"),
			filepath.Join(r.projectRoot, "bin", "pulse-agent"),
		)
	}

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
		defer file.Close()

		w.Header().Set("X-Checksum-Sha256", checksum)
		http.ServeContent(w, req, filepath.Base(candidate), info.ModTime(), file)
		return
	}

	// Fallback: proxy from GitHub releases for the binary
	// This handles LXC/barebone installations that don't have agent binaries locally.
	// We proxy instead of redirecting because agents require the X-Checksum-Sha256 header,
	// which GitHub doesn't provide.
	if normalized != "" {
		r.proxyAgentBinaryFromGitHub(w, req, normalized)
		return
	}

	// No architecture specified and no local binary - can't redirect without knowing arch
	http.Error(w, "Agent binary not found. Specify ?arch=linux-amd64 (or your architecture)", http.StatusNotFound)
}

// proxyAgentBinaryFromGitHub downloads an agent binary from GitHub releases and serves
// it to the requesting agent with the X-Checksum-Sha256 header. This is used when the
// binary isn't available locally (e.g., LXC/bare-metal installations updated via web UI).
// We must proxy instead of redirecting because the agent requires the checksum header
// for security verification, and GitHub doesn't provide it.
func (r *Router) proxyAgentBinaryFromGitHub(w http.ResponseWriter, req *http.Request, normalized string) {
	binaryName := "pulse-agent-" + normalized
	if strings.HasPrefix(normalized, "windows-") {
		binaryName += ".exe"
	}
	githubURL := "https://github.com/rcourtman/Pulse/releases/latest/download/" + binaryName

	log.Info().Str("arch", normalized).Str("url", githubURL).Msg("Local agent binary not found, proxying from GitHub releases")

	client := r.installScriptClient
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Minute,
		}
	}

	resp, err := client.Get(githubURL)
	if err != nil {
		log.Error().Err(err).Str("url", githubURL).Msg("Failed to fetch agent binary from GitHub")
		http.Error(w, "Failed to fetch agent binary", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("url", githubURL).Msg("GitHub returned non-200 status for agent binary")
		http.Error(w, "Agent binary not found on GitHub", http.StatusNotFound)
		return
	}

	// Read the binary with size limit (100 MB)
	const maxAgentBinarySize = 100 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxAgentBinarySize+1)

	hasher := sha256.New()
	content, err := io.ReadAll(io.TeeReader(limitedReader, hasher))
	if err != nil {
		log.Error().Err(err).Msg("Failed to read agent binary from GitHub")
		http.Error(w, "Failed to read agent binary", http.StatusInternalServerError)
		return
	}
	if int64(len(content)) > maxAgentBinarySize {
		log.Error().Int64("size", int64(len(content))).Msg("Agent binary from GitHub exceeds size limit")
		http.Error(w, "Agent binary too large", http.StatusInternalServerError)
		return
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	w.Header().Set("X-Checksum-Sha256", checksum)
	w.Header().Set("X-Served-From", "github-proxy")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(content)
}

// proxyInstallScriptFromGitHub fetches an install script from GitHub releases
// This is used as a fallback when scripts aren't available locally (e.g., LXC updates)
func (r *Router) proxyInstallScriptFromGitHub(w http.ResponseWriter, req *http.Request, scriptName string) {
	// Use raw.githubusercontent.com to fetch from main branch
	githubURL := "https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/" + scriptName

	client := r.installScriptClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	resp, err := client.Get(githubURL)
	if err != nil {
		log.Error().Err(err).Str("url", githubURL).Msg("Failed to fetch install script from GitHub")
		http.Error(w, "Failed to fetch install script", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("url", githubURL).Msg("GitHub returned non-200 status for install script")
		http.Error(w, "Install script not found", http.StatusNotFound)
		return
	}

	// Read the script content
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read install script from GitHub")
		http.Error(w, "Failed to read install script", http.StatusInternalServerError)
		return
	}

	// Determine content type based on script extension
	contentType := "text/x-shellscript"
	if strings.HasSuffix(scriptName, ".ps1") {
		contentType = "text/plain"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+scriptName+"\"")
	w.Header().Set("X-Served-From", "github-fallback")
	w.Write(content)
}
