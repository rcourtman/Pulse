package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type hostAgentBinary struct {
	platform  string
	arch      string
	filenames []string
}

var requiredHostAgentBinaries = []hostAgentBinary{
	{platform: "linux", arch: "amd64", filenames: []string{"pulse-host-agent-linux-amd64"}},
	{platform: "linux", arch: "arm64", filenames: []string{"pulse-host-agent-linux-arm64"}},
	{platform: "linux", arch: "armv7", filenames: []string{"pulse-host-agent-linux-armv7"}},
	{platform: "darwin", arch: "amd64", filenames: []string{"pulse-host-agent-darwin-amd64"}},
	{platform: "darwin", arch: "arm64", filenames: []string{"pulse-host-agent-darwin-arm64"}},
	{
		platform:  "windows",
		arch:      "amd64",
		filenames: []string{"pulse-host-agent-windows-amd64", "pulse-host-agent-windows-amd64.exe"},
	},
	{
		platform:  "windows",
		arch:      "arm64",
		filenames: []string{"pulse-host-agent-windows-arm64", "pulse-host-agent-windows-arm64.exe"},
	},
	{
		platform:  "windows",
		arch:      "386",
		filenames: []string{"pulse-host-agent-windows-386", "pulse-host-agent-windows-386.exe"},
	},
}

func validateAgentBinaries() {
	binDirs := hostAgentSearchPaths()
	missing := findMissingHostAgentBinaries(binDirs)
	if len(missing) == 0 {
		log.Info().Msg("All host agent binaries available for download")
		return
	}

	missingPlatforms := make([]string, 0, len(missing))
	for key := range missing {
		missingPlatforms = append(missingPlatforms, key)
	}
	sort.Strings(missingPlatforms)

	log.Warn().
		Strs("missing_platforms", missingPlatforms).
		Msg("Host agent binaries missing - attempting to download bundle from GitHub release")

	if err := downloadAndInstallHostAgentBinaries(binDirs[0]); err != nil {
		log.Error().
			Err(err).
			Str("target_dir", binDirs[0]).
			Strs("missing_platforms", missingPlatforms).
			Msg("Failed to automatically install host agent binaries; install script downloads will fail")
		return
	}

	remaining := findMissingHostAgentBinaries(binDirs)
	if len(remaining) == 0 {
		log.Info().Msg("Host agent binaries restored from GitHub release bundle")
		return
	}

	stillMissing := make([]string, 0, len(remaining))
	for key := range remaining {
		stillMissing = append(stillMissing, key)
	}
	sort.Strings(stillMissing)
	log.Warn().
		Strs("missing_platforms", stillMissing).
		Msg("Host agent binaries still missing after automatic restoration attempt")
}

func hostAgentSearchPaths() []string {
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

func findMissingHostAgentBinaries(binDirs []string) map[string]hostAgentBinary {
	missing := make(map[string]hostAgentBinary)
	for _, binary := range requiredHostAgentBinaries {
		if !hostAgentBinaryExists(binDirs, binary.filenames) {
			key := fmt.Sprintf("%s-%s", binary.platform, binary.arch)
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

func downloadAndInstallHostAgentBinaries(targetDir string) error {
	version := strings.TrimSpace(Version)
	if version == "" || strings.EqualFold(version, "dev") {
		return fmt.Errorf("cannot download host agent bundle for non-release version %q", version)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to ensure bin directory %s: %w", targetDir, err)
	}

	url := fmt.Sprintf("https://github.com/rcourtman/Pulse/releases/download/%[1]s/pulse-%[1]s.tar.gz", version)
	tempFile, err := os.CreateTemp("", "pulse-host-agent-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temporary archive file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download host agent bundle from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("unexpected status %d downloading %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
	}

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save host agent bundle: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary bundle file: %w", err)
	}

	if err := extractHostAgentBinaries(tempFile.Name(), targetDir); err != nil {
		return err
	}

	return nil
}

func extractHostAgentBinaries(archivePath, targetDir string) error {
	file, err := os.Open(archivePath)
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

		if header == nil {
			continue
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && header.Typeflag != tar.TypeSymlink {
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
		case tar.TypeReg, tar.TypeRegA:
			if err := writeHostAgentFile(destPath, tr, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			symlinks = append(symlinks, pendingLink{
				path:   destPath,
				target: header.Linkname,
			})
		}
	}

	for _, link := range symlinks {
		if err := os.Remove(link.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to replace existing symlink %s: %w", link.path, err)
		}
		if err := os.Symlink(link.target, link.path); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", destination, err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(destination), "pulse-host-agent-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for %s: %w", destination, err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to extract %s: %w", destination, err)
	}

	if err := tmpFile.Chmod(normalizeExecutableMode(mode)); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to set permissions on %s: %w", destination, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to finalize %s: %w", destination, err)
	}

	if err := os.Rename(tmpFile.Name(), destination); err != nil {
		return fmt.Errorf("failed to install %s: %w", destination, err)
	}

	return nil
}

func copyHostAgentFile(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open %s for fallback copy: %w", source, err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("failed to prepare directory for %s: %w", destination, err)
	}

	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create fallback copy %s: %w", destination, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
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
