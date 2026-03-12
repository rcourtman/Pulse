package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

const (
	canonicalUnifiedAgentReportPath = "/api/agents/agent/report"
	legacyUnifiedAgentReportPath    = "/api/agents/host/report"
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

	invalidCandidates := make([]string, 0, len(searchPaths))

	for _, candidate := range searchPaths {
		if candidate == "" {
			continue
		}

		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}

		if err := validateUnifiedAgentBinary(candidate); err != nil {
			log.Warn().Err(err).Str("path", candidate).Msg("Skipping incompatible local unified agent binary")
			invalidCandidates = append(invalidCandidates, fmt.Sprintf("%s (%v)", candidate, err))
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

	if len(invalidCandidates) > 0 {
		log.Warn().Strs("paths", invalidCandidates).Msg("Ignoring stale local unified agent binaries")
	}

	// In dev mode, never fall through to GitHub releases — the released binary
	// would lack current fixes. Return a clear 404 with build instructions.
	if r.serverVersion == "dev" {
		reason := fmt.Sprintf("Agent binary not found for %q in dev mode.", normalized)
		if len(invalidCandidates) > 0 {
			reason = fmt.Sprintf("Local agent binary for %q is stale or incompatible in dev mode:\n  %s",
				normalized,
				strings.Join(invalidCandidates, "\n  "),
			)
		}
		http.Error(w, reason+"\nBuild with:\n  GOOS=linux GOARCH=amd64 go build -o bin/pulse-agent-linux-amd64 ./cmd/pulse-agent", http.StatusNotFound)
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
	if len(invalidCandidates) > 0 {
		http.Error(w, "Local agent binary is stale or incompatible. Specify ?arch=linux-amd64 (or your architecture) after rebuilding the local agent artifact.", http.StatusNotFound)
		return
	}
	http.Error(w, "Agent binary not found. Specify ?arch=linux-amd64 (or your architecture)", http.StatusNotFound)
}

func validateUnifiedAgentBinary(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read binary: %w", err)
	}
	if bytes.Contains(content, []byte(legacyUnifiedAgentReportPath)) {
		return fmt.Errorf("references deprecated host endpoint %s", legacyUnifiedAgentReportPath)
	}
	if !bytes.Contains(content, []byte(canonicalUnifiedAgentReportPath)) {
		return fmt.Errorf("missing canonical host endpoint %s", canonicalUnifiedAgentReportPath)
	}
	return nil
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

	if resp.StatusCode == http.StatusOK {
		content, checksum, readErr := readBinaryWithChecksum(resp.Body)
		if readErr != nil {
			log.Error().Err(readErr).Msg("Failed to read agent binary from GitHub")
			http.Error(w, "Failed to read agent binary", http.StatusInternalServerError)
			return
		}
		serveProxiedAgentBinary(w, content, checksum, "github-proxy")
		return
	}

	if resp.StatusCode != http.StatusNotFound {
		log.Error().Int("status", resp.StatusCode).Str("url", githubURL).Msg("GitHub returned non-200 status for agent binary")
		http.Error(w, "Agent binary not found on GitHub", http.StatusNotFound)
		return
	}

	archiveContent, checksum, archiveErr := r.fetchAgentBinaryFromReleaseArchive(client, normalized)
	if archiveErr != nil {
		log.Error().Err(archiveErr).Str("arch", normalized).Msg("Failed archive fallback for agent binary")
		http.Error(w, "Agent binary not found on GitHub", http.StatusNotFound)
		return
	}
	serveProxiedAgentBinary(w, archiveContent, checksum, "github-proxy-archive")
}

const maxAgentBinarySize = 100 * 1024 * 1024

func readBinaryWithChecksum(body io.Reader) ([]byte, string, error) {
	limitedReader := io.LimitReader(body, maxAgentBinarySize+1)
	hasher := sha256.New()
	content, err := io.ReadAll(io.TeeReader(limitedReader, hasher))
	if err != nil {
		return nil, "", err
	}
	if int64(len(content)) > maxAgentBinarySize {
		return nil, "", fmt.Errorf("binary exceeds size limit")
	}
	return content, hex.EncodeToString(hasher.Sum(nil)), nil
}

func serveProxiedAgentBinary(w http.ResponseWriter, content []byte, checksum, servedFrom string) {
	w.Header().Set("X-Checksum-Sha256", checksum)
	w.Header().Set("X-Served-From", servedFrom)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(content)
}

func (r *Router) fetchAgentBinaryFromReleaseArchive(client *http.Client, normalized string) ([]byte, string, error) {
	tag, err := fetchLatestReleaseTag(client)
	if err != nil {
		return nil, "", err
	}
	version := strings.TrimPrefix(tag, "v")

	archiveName := fmt.Sprintf("pulse-agent-v%s-%s.tar.gz", version, normalized)
	entryName := "pulse-agent-" + normalized
	isWindows := strings.HasPrefix(normalized, "windows-")
	if isWindows {
		archiveName = fmt.Sprintf("pulse-agent-v%s-%s.zip", version, normalized)
		entryName += ".exe"
	}

	archiveURL := fmt.Sprintf("https://github.com/rcourtman/Pulse/releases/download/%s/%s", tag, archiveName)
	resp, err := client.Get(archiveURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch release archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("release archive returned status %d", resp.StatusCode)
	}

	archiveReader := io.LimitReader(resp.Body, maxAgentBinarySize+1)
	archiveBytes, err := io.ReadAll(archiveReader)
	if err != nil {
		return nil, "", fmt.Errorf("failed reading release archive: %w", err)
	}
	if int64(len(archiveBytes)) > maxAgentBinarySize {
		return nil, "", fmt.Errorf("release archive exceeded size limit")
	}

	var binary []byte
	if isWindows {
		binary, err = extractFromZip(archiveBytes, entryName)
	} else {
		binary, err = extractFromTarGz(archiveBytes, entryName)
	}
	if err != nil {
		return nil, "", err
	}
	if int64(len(binary)) > maxAgentBinarySize {
		return nil, "", fmt.Errorf("extracted binary exceeded size limit")
	}
	sum := sha256.Sum256(binary)
	return binary, hex.EncodeToString(sum[:]), nil
}

func fetchLatestReleaseTag(client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/rcourtman/Pulse/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pulse-agent-download-proxy")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest release lookup returned status %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("failed decoding latest release payload: %w", err)
	}
	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", fmt.Errorf("latest release payload missing tag_name")
	}
	return tag, nil
}

func extractFromTarGz(archive []byte, entryName string) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("failed to open tar.gz: %w", err)
	}
	defer gzReader.Close()

	tr := tar.NewReader(gzReader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed reading tar entry: %w", err)
		}
		if filepath.Base(header.Name) != entryName {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(tr, maxAgentBinarySize+1))
		if err != nil {
			return nil, fmt.Errorf("failed reading binary from tar.gz: %w", err)
		}
		if int64(len(content)) > maxAgentBinarySize {
			return nil, fmt.Errorf("binary from tar.gz exceeded size limit")
		}
		return content, nil
	}
	return nil, fmt.Errorf("binary %q not found in tar.gz", entryName)
}

func extractFromZip(archive []byte, entryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	for _, file := range zr.File {
		if filepath.Base(file.Name) != entryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed opening binary in zip: %w", err)
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, maxAgentBinarySize+1))
		rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed reading binary from zip: %w", readErr)
		}
		if int64(len(content)) > maxAgentBinarySize {
			return nil, fmt.Errorf("binary from zip exceeded size limit")
		}
		return content, nil
	}
	return nil, fmt.Errorf("binary %q not found in zip", entryName)
}

func (r *Router) installScriptReleaseAssetURL(scriptName string) (string, error) {
	rawVersion := strings.TrimSpace(r.serverVersion)
	if rawVersion == "" {
		return "", fmt.Errorf("server version is unavailable")
	}
	if strings.EqualFold(rawVersion, "dev") {
		return "", fmt.Errorf("development builds must serve local install scripts")
	}

	version, err := updates.ParseVersion(rawVersion)
	if err != nil {
		return "", fmt.Errorf("server version %q is not a published release version", rawVersion)
	}
	if version.Build != "" {
		return "", fmt.Errorf("server version %q includes build metadata and cannot map to a release asset", rawVersion)
	}

	return fmt.Sprintf(
		"https://github.com/rcourtman/Pulse/releases/download/v%s/%s",
		version.String(),
		scriptName,
	), nil
}

// proxyInstallScriptFromGitHub fetches an install script from the exact GitHub
// release asset that matches the running server version. This is used as a
// fallback when scripts aren't available locally (for example in LXC updates).
func (r *Router) proxyInstallScriptFromGitHub(w http.ResponseWriter, req *http.Request, scriptName string) {
	githubURL, err := r.installScriptReleaseAssetURL(scriptName)
	if err != nil {
		log.Error().Err(err).Str("server_version", strings.TrimSpace(r.serverVersion)).Str("script", scriptName).Msg("Install script fallback unavailable for current server build")
		http.Error(w, "Install script unavailable for current server build", http.StatusServiceUnavailable)
		return
	}

	client := r.installScriptClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, githubURL, nil)
	if err != nil {
		log.Error().Err(err).Str("url", githubURL).Msg("Failed to create install script fallback request")
		http.Error(w, "Failed to fetch install script", http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(proxyReq)
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
	if req.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Write(content)
}
