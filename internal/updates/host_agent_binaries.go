package updates

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

const (
	hostAgentDownloadAttempts    = 3
	hostAgentRetryInitialBackoff = 100 * time.Millisecond
	hostAgentHTTPErrorBodyLimit  = 2048
	hostAgentChecksumBodyLimit   = 1024
)

var (
	maxHostAgentBinarySize = int64(256 * 1024 * 1024)
	maxHostAgentBundleSize = int64(512 * 1024 * 1024)
	httpClient             = &http.Client{Timeout: 2 * time.Minute}
	downloadURLForVersion  = func(version string) string {
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
	renameFn                              = os.Rename
	symlinkFn                             = os.Symlink
	copyFn                                = io.Copy
	chmodFileFn                           = func(f *os.File, mode os.FileMode) error { return f.Chmod(mode) }
	closeFileFn                           = func(f *os.File) error { return f.Close() }
	retrySleepFn                          = time.Sleep
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

	log.Info().Msg("host agent binaries restored from GitHub release bundle")
	return nil
}

// DownloadAndInstallHostAgentBinaries fetches the universal host agent bundle for the given version and installs it.
func DownloadAndInstallHostAgentBinaries(version string, targetDir string) (retErr error) {
	normalizedVersion := normalizeVersionTag(version)
	if normalizedVersion == "" || strings.EqualFold(normalizedVersion, "vdev") {
		return fmt.Errorf("cannot download host agent bundle for non-release version %q", version)
	}

	if err := mkdirAllFn(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to ensure bin directory %s: %w", targetDir, err)
	}

	bundleURL := downloadURLForVersion(normalizedVersion)
	tempFile, err := createTempFn("", "pulse-host-agent-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temporary archive file: %w", err)
	}
	tempFileNeedsClose := true
	defer func() {
		if tempFileNeedsClose {
			_ = closeFileFn(tempFile)
		}
		_ = removeFn(tempFile.Name())
	}()

	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download host agent bundle from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("unexpected status %d downloading %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
	}

	if resp.ContentLength > maxHostAgentBundleSize {
		return fmt.Errorf("host agent bundle exceeds max size %d bytes", maxHostAgentBundleSize)
	}

	limitedReader := io.LimitReader(resp.Body, maxHostAgentBundleSize+1)
	written, err := io.Copy(tempFile, limitedReader)
	if err != nil {
		return fmt.Errorf("failed to save host agent bundle: %w", err)
	}
	if written > maxHostAgentBundleSize {
		return fmt.Errorf("host agent bundle exceeds max size %d bytes", maxHostAgentBundleSize)
	}

	if err := closeFileFn(tempFile); err != nil {
		return fmt.Errorf("failed to close temporary bundle file: %w", err)
	}
	tempFileNeedsClose = false

	checksumURL := checksumURLForVersion(normalizedVersion)
	if err := verifyHostAgentBundleChecksum(tempFile.Name(), bundleURL, checksumURL); err != nil {
		return fmt.Errorf("agentbinaries.DownloadAndInstallHostAgentBinaries: %w", err)
	}

	if err := extractHostAgentBinaries(tempFile.Name(), targetDir); err != nil {
		return err
	}

	return nil
}

func downloadHostAgentBundle(url string, tempFile *os.File) error {
	backoff := hostAgentRetryInitialBackoff

	for attempt := 1; attempt <= hostAgentDownloadAttempts; attempt++ {
		if err := tempFile.Truncate(0); err != nil {
			return fmt.Errorf("failed to reset temporary bundle file: %w", err)
		}
		if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek temporary bundle file: %w", err)
		}

		resp, err := httpClient.Get(url)
		if err != nil {
			if attempt == hostAgentDownloadAttempts {
				return fmt.Errorf("failed to download host agent bundle from %s: %w", url, err)
			}
			retrySleepFn(backoff)
			backoff *= 2
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, hostAgentHTTPErrorBodyLimit))
			resp.Body.Close()
			err := fmt.Errorf("unexpected status %d downloading %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
			if !isRetryableHTTPStatus(resp.StatusCode) || attempt == hostAgentDownloadAttempts {
				return err
			}
			retrySleepFn(backoff)
			backoff *= 2
			continue
		}

		if _, err := copyFn(tempFile, resp.Body); err != nil {
			resp.Body.Close()
			if attempt == hostAgentDownloadAttempts {
				return fmt.Errorf("failed to save host agent bundle: %w", err)
			}
			retrySleepFn(backoff)
			backoff *= 2
			continue
		}

		if err := resp.Body.Close(); err != nil {
			if attempt == hostAgentDownloadAttempts {
				return fmt.Errorf("failed to finalize host agent bundle download: %w", err)
			}
			retrySleepFn(backoff)
			backoff *= 2
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to download host agent bundle from %s", url)
}

func verifyHostAgentBundleChecksum(bundlePath, bundleURL, checksumURL string) error {
	checksum, filename, err := downloadHostAgentChecksum(checksumURL)
	if err != nil {
		return err
	}

	expectedName := fileNameFromURL(bundleURL)
	if filename != "" && expectedName != "" && filename != expectedName {
		return fmt.Errorf("checksum file does not match bundle name (got %q, expected %q)", filename, expectedName)
	}

	actual, err := hashFileSHA256(bundlePath)
	if err != nil {
		return err
	}

	if !strings.EqualFold(actual, checksum) {
		return fmt.Errorf("host agent bundle checksum mismatch for %s (expected: %s, actual: %s)", bundlePath, checksum, actual)
	}

	return nil
}

func downloadHostAgentChecksum(checksumURL string) (string, string, error) {
	backoff := hostAgentRetryInitialBackoff

	for attempt := 1; attempt <= hostAgentDownloadAttempts; attempt++ {
		resp, err := httpClient.Get(checksumURL)
		if err != nil {
			if attempt == hostAgentDownloadAttempts {
				return "", "", fmt.Errorf("failed to download checksum from %s: %w", checksumURL, err)
			}
			retrySleepFn(backoff)
			backoff *= 2
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, hostAgentHTTPErrorBodyLimit))
			resp.Body.Close()
			err := fmt.Errorf("unexpected status %d downloading checksum %s: %s", resp.StatusCode, checksumURL, strings.TrimSpace(string(body)))
			if !isRetryableHTTPStatus(resp.StatusCode) || attempt == hostAgentDownloadAttempts {
				return "", "", err
			}
			retrySleepFn(backoff)
			backoff *= 2
			continue
		}

		payload, err := io.ReadAll(io.LimitReader(resp.Body, hostAgentChecksumBodyLimit))
		resp.Body.Close()
		if err != nil {
			if attempt == hostAgentDownloadAttempts {
				return "", "", fmt.Errorf("failed to read checksum file: %w", err)
			}
			retrySleepFn(backoff)
			backoff *= 2
			continue
		}

		checksum, filename, err := parseHostAgentChecksumPayload(payload)
		if err != nil {
			return "", "", err
		}

		return checksum, filename, nil
	}

	return "", "", fmt.Errorf("failed to download checksum from %s", checksumURL)
}

func parseHostAgentChecksumPayload(payload []byte) (string, string, error) {
	fields := strings.Fields(string(payload))
	if len(fields) == 0 {
		return "", "", fmt.Errorf("checksum file is empty")
	}

	checksum = strings.ToLower(strings.TrimSpace(fields[0]))
	if len(checksum) != 64 {
		return "", "", fmt.Errorf("checksum file has invalid hash (wrong length)")
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return "", "", fmt.Errorf("checksum file has invalid hash (bad hex): %w", err)
	}

	filename = ""
	if len(fields) > 1 {
		filename = strings.TrimPrefix(path.Base(fields[1]), "*")
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

func hashFileSHA256(path string) (hash string, retErr error) {
	file, err := openFileFn(path)
	if err != nil {
		return "", fmt.Errorf("failed to open bundle for checksum: %w", err)
	}
	defer closeFileWithContext(&retErr, file, "failed to close bundle after checksum")

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

func requiredHostAgentFilenameSet() map[string]struct{} {
	names := make(map[string]struct{})
	for _, binary := range requiredHostAgentBinaries {
		for _, name := range binary.Filenames {
			names[name] = struct{}{}
		}
	}
	return names
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

func extractHostAgentBinaries(archivePath, targetDir string) (retErr error) {
	file, err := openFileFn(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open host agent bundle: %w", err)
	}
	defer closeFileWithContext(&retErr, file, "failed to close host agent bundle")

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer closeCloserWithContext(&retErr, gzReader, "failed to close host agent gzip reader")

	tr := tar.NewReader(gzReader)
	allowedNames := requiredHostAgentFilenameSet()
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

		entryName := path.Clean(header.Name)
		if path.IsAbs(entryName) {
			continue
		}
		if !strings.HasPrefix(entryName, "bin/") {
			continue
		}
		if path.Dir(entryName) != "bin" {
			continue
		}

		base := path.Base(entryName)
		if _, ok := allowedNames[base]; !ok {
			continue
		}

		destPath := filepath.Join(targetDir, base)

		switch header.Typeflag {
		case tar.TypeReg:
			if header.Size < 0 {
				return fmt.Errorf("host agent entry %q has invalid size %d", base, header.Size)
			}
			if header.Size > maxHostAgentBinarySize {
				return fmt.Errorf("host agent entry %q exceeds max size %d bytes", base, maxHostAgentBinarySize)
			}
			if err := writeHostAgentFile(destPath, tr, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			target, err := normalizeHostAgentSymlinkTarget(header.Linkname)
			if err != nil {
				return fmt.Errorf("agentbinaries.extractHostAgentBinaries: %w", err)
			}
			symlinks = append(symlinks, pendingLink{
				path:   destPath,
				target: target,
			})
		}
	}

	for _, link := range symlinks {
		linkTarget, err := normalizeHostAgentSymlinkTarget(link.target, allowedNames)
		if err != nil {
			return fmt.Errorf("invalid host agent symlink target %q: %w", link.target, err)
		}
		link.target = linkTarget

		if _, err := os.Stat(filepath.Join(targetDir, link.target)); err != nil {
			return fmt.Errorf("host agent symlink target %q is unavailable: %w", link.target, err)
		}

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

func normalizeHostAgentSymlinkTarget(target string, allowedNames map[string]struct{}) (string, error) {
	clean := strings.TrimSpace(target)
	if clean == "" {
		return "", fmt.Errorf("target is empty")
	}
	clean = path.Clean(clean)
	if clean == "." || clean == ".." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("target must be a relative host agent filename")
	}
	if path.Base(clean) != clean {
		return "", fmt.Errorf("target must not contain directory traversal")
	}
	if strings.Contains(clean, `\`) {
		return "", fmt.Errorf("target must not contain path separators")
	}
	if _, ok := allowedNames[clean]; !ok {
		return "", fmt.Errorf("target must reference an expected host agent binary")
	}
	return clean, nil
}

func writeHostAgentFile(destination string, reader io.Reader, mode os.FileMode) error {
	if err := mkdirAllFn(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", destination, err)
	}

	tmpFile, err := createTempFn(filepath.Dir(destination), "pulse-host-agent-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for %s: %w", destination, err)
	}
	defer removeFileWithContext(&retErr, tmpFile.Name(), fmt.Sprintf("failed to remove temporary file for %s", destination))

	if _, err := copyFn(tmpFile, reader); err != nil {
		if closeErr := closeFileFn(tmpFile); closeErr != nil {
			return errors.Join(
				fmt.Errorf("failed to extract %s: %w", destination, err),
				fmt.Errorf("failed to close temporary file for %s: %w", destination, closeErr),
			)
		}
		return fmt.Errorf("failed to extract %s: %w", destination, err)
	}

	if err := chmodFileFn(tmpFile, normalizeExecutableMode(mode)); err != nil {
		if closeErr := closeFileFn(tmpFile); closeErr != nil {
			return errors.Join(
				fmt.Errorf("failed to set permissions on %s: %w", destination, err),
				fmt.Errorf("failed to close temporary file for %s: %w", destination, closeErr),
			)
		}
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

func copyHostAgentFile(source, destination string) (retErr error) {
	src, err := openFileFn(source)
	if err != nil {
		return fmt.Errorf("failed to open %s for fallback copy: %w", source, err)
	}
	defer closeFileWithContext(&retErr, src, fmt.Sprintf("failed to close source file %s", source))

	info, err := src.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %s for fallback copy: %w", source, err)
	}
	defer closeFileWithContext(&retErr, dst, fmt.Sprintf("failed to close fallback copy %s", destination))

	if _, err := copyFn(dst, src); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", source, destination, err)
	}

	return nil
}

func normalizeExecutableMode(_ os.FileMode) os.FileMode {
	return 0o755
}

func isRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return statusCode >= http.StatusInternalServerError && statusCode <= 599
	}
}
