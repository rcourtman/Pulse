package agentbinaries

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type HostAgentBinary struct {
	Platform  string
	Arch      string
	Filenames []string
}

var requiredHostAgentBinaries = []HostAgentBinary{
	{Platform: "linux", Arch: "amd64", Filenames: []string{"pulse-host-agent-linux-amd64"}},
	{Platform: "linux", Arch: "arm64", Filenames: []string{"pulse-host-agent-linux-arm64"}},
	{Platform: "linux", Arch: "armv7", Filenames: []string{"pulse-host-agent-linux-armv7"}},
	{Platform: "linux", Arch: "armv6", Filenames: []string{"pulse-host-agent-linux-armv6"}},
	{Platform: "linux", Arch: "386", Filenames: []string{"pulse-host-agent-linux-386"}},
	{Platform: "darwin", Arch: "amd64", Filenames: []string{"pulse-host-agent-darwin-amd64"}},
	{Platform: "darwin", Arch: "arm64", Filenames: []string{"pulse-host-agent-darwin-arm64"}},
	{
		Platform:  "windows",
		Arch:      "amd64",
		Filenames: []string{"pulse-host-agent-windows-amd64", "pulse-host-agent-windows-amd64.exe"},
	},
	{
		Platform:  "windows",
		Arch:      "arm64",
		Filenames: []string{"pulse-host-agent-windows-arm64", "pulse-host-agent-windows-arm64.exe"},
	},
	{
		Platform:  "windows",
		Arch:      "386",
		Filenames: []string{"pulse-host-agent-windows-386", "pulse-host-agent-windows-386.exe"},
	},
}

var downloadMu sync.Mutex

var (
	httpClient            = &http.Client{Timeout: 2 * time.Minute}
	downloadURLForVersion = func(version string) string {
		return fmt.Sprintf("https://github.com/rcourtman/Pulse/releases/download/%[1]s/pulse-%[1]s.tar.gz", version)
	}
	checksumURLForVersion = func(version string) string {
		return downloadURLForVersion(version) + ".sha256"
	}
	downloadAndInstallHostAgentBinariesFn = DownloadAndInstallHostAgentBinaries
	findMissingHostAgentBinariesFn        = findMissingHostAgentBinaries
	mkdirAllFn                            = os.MkdirAll
	createTempFn                          = os.CreateTemp
	removeFn                              = os.Remove
	openFileFn                            = os.Open
	openFileModeFn                        = os.OpenFile
	renameFn                              = os.Rename
	symlinkFn                             = os.Symlink
	copyFn                                = io.Copy
	chmodFileFn                           = func(f *os.File, mode os.FileMode) error { return f.Chmod(mode) }
	closeFileFn                           = func(f *os.File) error { return f.Close() }
)

// httpGetWithErrorContext performs an HTTP GET and returns a detailed error if the status is not 200 OK.
func httpGetWithErrorContext(url string) (*http.Response, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download from %s: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d downloading %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
	}

	return resp, nil
}

// HostAgentSearchPaths returns the directories to search for host agent binaries.
func HostAgentSearchPaths() []string {
	primary := strings.TrimSpace(os.Getenv("PULSE_BIN_DIR"))
	if primary == "" {
		primary = "/opt/pulse/bin"
	}

	dirs := []string{primary, "./bin", "."}
	seen := make(map[string]struct{}, len(dirs))
	result := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		clean := filepath.Clean(dir)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}

// EnsureHostAgentBinaries verifies that all host agent binaries are present locally.
// If any are missing, it attempts to restore them from the matching GitHub release.
// The returned map contains any binaries that remain missing after the attempt.
func EnsureHostAgentBinaries(version string) map[string]HostAgentBinary {
	binDirs := HostAgentSearchPaths()
	missing := findMissingHostAgentBinariesFn(binDirs)
	if len(missing) == 0 {
		return nil
	}

	downloadMu.Lock()
	defer downloadMu.Unlock()

	// Re-check after acquiring the lock in case another goroutine restored them.
	missing = findMissingHostAgentBinariesFn(binDirs)
	if len(missing) == 0 {
		return nil
	}

	missingPlatforms := make([]string, 0, len(missing))
	for key := range missing {
		missingPlatforms = append(missingPlatforms, key)
	}
	sort.Strings(missingPlatforms)

	log.Warn().
		Strs("missing_platforms", missingPlatforms).
		Msg("Host agent binaries missing - attempting to download bundle from GitHub release")

	if err := downloadAndInstallHostAgentBinariesFn(version, binDirs[0]); err != nil {
		log.Error().
			Err(err).
			Str("target_dir", binDirs[0]).
			Strs("missing_platforms", missingPlatforms).
			Msg("Failed to automatically install host agent binaries; download endpoints will return 404s")
		return missing
	}

	if remaining := findMissingHostAgentBinariesFn(binDirs); len(remaining) > 0 {
		stillMissing := make([]string, 0, len(remaining))
		for key := range remaining {
			stillMissing = append(stillMissing, key)
		}
		sort.Strings(stillMissing)
		log.Warn().
			Strs("missing_platforms", stillMissing).
			Msg("Host agent binaries still missing after automatic restoration attempt")
		return remaining
	}

	log.Info().Msg("Host agent binaries restored from GitHub release bundle")
	return nil
}

// DownloadAndInstallHostAgentBinaries fetches the universal host agent bundle for the given version and installs it.
func DownloadAndInstallHostAgentBinaries(version string, targetDir string) error {
	normalizedVersion := normalizeVersionTag(version)
	if normalizedVersion == "" || strings.EqualFold(normalizedVersion, "vdev") {
		return fmt.Errorf("cannot download host agent bundle for non-release version %q", version)
	}

	if err := mkdirAllFn(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to ensure bin directory %s: %w", targetDir, err)
	}

	url := downloadURLForVersion(normalizedVersion)
	tempFile, err := createTempFn("", "pulse-host-agent-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temporary archive file: %w", err)
	}
	defer removeFn(tempFile.Name())

	resp, err := httpGetWithErrorContext(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save host agent bundle: %w", err)
	}

	if err := closeFileFn(tempFile); err != nil {
		return fmt.Errorf("failed to close temporary bundle file: %w", err)
	}

	checksumURL := checksumURLForVersion(normalizedVersion)
	if err := verifyHostAgentBundleChecksum(tempFile.Name(), url, checksumURL); err != nil {
		return fmt.Errorf("agentbinaries.DownloadAndInstallHostAgentBinaries: %w", err)
	}

	if err := extractHostAgentBinaries(tempFile.Name(), targetDir); err != nil {
		return fmt.Errorf("agentbinaries.DownloadAndInstallHostAgentBinaries: %w", err)
	}

	return nil
}

func verifyHostAgentBundleChecksum(bundlePath, bundleURL, checksumURL string) error {
	checksum, filename, err := downloadHostAgentChecksum(checksumURL)
	if err != nil {
		return fmt.Errorf("agentbinaries.verifyHostAgentBundleChecksum: %w", err)
	}

	expectedName := fileNameFromURL(bundleURL)
	if filename != "" && expectedName != "" && filename != expectedName {
		return fmt.Errorf("checksum file does not match bundle name (got %q, expected %q)", filename, expectedName)
	}

	actual, err := hashFileSHA256(bundlePath)
	if err != nil {
		return fmt.Errorf("agentbinaries.verifyHostAgentBundleChecksum: %w", err)
	}

	if !strings.EqualFold(actual, checksum) {
		return fmt.Errorf("host agent bundle checksum mismatch")
	}

	return nil
}

func downloadHostAgentChecksum(checksumURL string) (string, string, error) {
	resp, err := httpGetWithErrorContext(checksumURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", "", fmt.Errorf("failed to read checksum file: %w", err)
	}

	fields := strings.Fields(string(payload))
	if len(fields) == 0 {
		return "", "", fmt.Errorf("checksum file is empty")
	}

	checksum := strings.ToLower(strings.TrimSpace(fields[0]))
	if len(checksum) != 64 {
		return "", "", fmt.Errorf("checksum file has invalid hash")
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return "", "", fmt.Errorf("checksum file has invalid hash")
	}

	filename := ""
	if len(fields) > 1 {
		filename = path.Base(fields[1])
	}

	return checksum, filename, nil
}

func fileNameFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if base := path.Base(parsed.Path); base != "" && base != "." {
			return base
		}
	}
	base := path.Base(rawURL)
	if idx := strings.IndexAny(base, "?#"); idx != -1 {
		base = base[:idx]
	}
	return base
}

func hashFileSHA256(path string) (string, error) {
	file, err := openFileFn(path)
	if err != nil {
		return "", fmt.Errorf("failed to open bundle for checksum: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash bundle: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func findMissingHostAgentBinaries(binDirs []string) map[string]HostAgentBinary {
	missing := make(map[string]HostAgentBinary)
	for _, binary := range requiredHostAgentBinaries {
		if !hostAgentBinaryExists(binDirs, binary.Filenames) {
			key := fmt.Sprintf("%s-%s", binary.Platform, binary.Arch)
			missing[key] = binary
		}
	}
	return missing
}

func hostAgentBinaryExists(binDirs, filenames []string) bool {
	for _, dir := range binDirs {
		for _, name := range filenames {
			path := filepath.Join(dir, name)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return true
			}
		}
	}
	return false
}

func normalizeVersionTag(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		return ""
	}
	v = strings.TrimPrefix(v, "v")
	return "v" + v
}

func extractHostAgentBinaries(archivePath, targetDir string) error {
	file, err := openFileFn(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open host agent bundle: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tr := tar.NewReader(gzReader)
	type pendingLink struct {
		path   string
		target string
	}
	var symlinks []pendingLink

	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read host agent bundle: %w", err)
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeSymlink {
			continue
		}

		if !strings.HasPrefix(header.Name, "bin/") {
			continue
		}

		base := path.Base(header.Name)
		if !strings.HasPrefix(base, "pulse-host-agent-") {
			continue
		}

		destPath := filepath.Join(targetDir, base)

		switch header.Typeflag {
		case tar.TypeReg:
			if err := writeHostAgentFile(destPath, tr, header.FileInfo().Mode()); err != nil {
				return fmt.Errorf("agentbinaries.extractHostAgentBinaries: %w", err)
			}
		case tar.TypeSymlink:
			symlinks = append(symlinks, pendingLink{
				path:   destPath,
				target: header.Linkname,
			})
		}
	}

	for _, link := range symlinks {
		if err := removeFn(link.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to replace existing symlink %s: %w", link.path, err)
		}
		if err := symlinkFn(link.target, link.path); err != nil {
			// Fallback: copy the referenced file if symlinks are not permitted
			source := filepath.Join(targetDir, link.target)
			if err := copyHostAgentFile(source, link.path); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s: %w", link.path, link.target, err)
			}
		}
	}

	return nil
}

func writeHostAgentFile(destination string, reader io.Reader, mode os.FileMode) error {
	if err := mkdirAllFn(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", destination, err)
	}

	tmpFile, err := createTempFn(filepath.Dir(destination), "pulse-host-agent-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for %s: %w", destination, err)
	}
	defer removeFn(tmpFile.Name())

	if _, err := copyFn(tmpFile, reader); err != nil {
		closeFileFn(tmpFile)
		return fmt.Errorf("failed to extract %s: %w", destination, err)
	}

	if err := chmodFileFn(tmpFile, normalizeExecutableMode(mode)); err != nil {
		closeFileFn(tmpFile)
		return fmt.Errorf("failed to set permissions on %s: %w", destination, err)
	}

	if err := closeFileFn(tmpFile); err != nil {
		return fmt.Errorf("failed to finalize %s: %w", destination, err)
	}

	if err := renameFn(tmpFile.Name(), destination); err != nil {
		return fmt.Errorf("failed to install %s: %w", destination, err)
	}

	return nil
}

func copyHostAgentFile(source, destination string) error {
	src, err := openFileFn(source)
	if err != nil {
		return fmt.Errorf("failed to open %s for fallback copy: %w", source, err)
	}
	defer src.Close()

	if err := mkdirAllFn(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("failed to prepare directory for %s: %w", destination, err)
	}

	dst, err := openFileModeFn(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create fallback copy %s: %w", destination, err)
	}
	defer dst.Close()

	if _, err := copyFn(dst, src); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", source, destination, err)
	}

	return nil
}

func normalizeExecutableMode(mode os.FileMode) os.FileMode {
	perms := mode.Perm()
	if perms&0o111 == 0 {
		perms |= 0o755
	}
	return (mode &^ os.ModePerm) | perms
}
