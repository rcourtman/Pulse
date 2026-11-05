// Package agentidentity owns the published pulse-agent executable contract.
package agentidentity

import (
	"bufio"
	"bytes"
	"debug/buildinfo"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
)

const Package = "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent"
const maxIdentitySize = 256

// ReleaseIdentity is stamped by scripts/release_ldflags.sh. Keep this value
// reachable from main even in stripped builds. It is independent of dependency
// literals and the optional (and absent with -trimpath) linker command metadata.
var ReleaseIdentity string

//go:embed version-pattern.txt
var releaseVersionPattern string

var versionPattern = regexp.MustCompile(strings.TrimSpace(releaseVersionPattern))
var stampPattern = regexp.MustCompile(`\[\[pulse-agent-release:v1:(v[0-9A-Za-z.+-]+)\]\]`)

// Version preserves prerelease identity, while dropping build provenance.
func Version(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 128 || (len(raw) == 128 && !strings.HasPrefix(raw, "v")) || !versionPattern.MatchString(raw) {
		return "", fmt.Errorf("invalid agent release version %q", raw)
	}
	raw = strings.SplitN(raw, "+", 2)[0]
	return "v" + strings.TrimPrefix(raw, "v"), nil
}

// ParseReleaseIdentity validates a complete bounded release stamp.
func ParseReleaseIdentity(stamp string) (string, error) {
	if len(stamp) > maxIdentitySize {
		return "", errors.New("oversized executable release identity")
	}
	matches := stampPattern.FindStringSubmatch(stamp)
	if len(matches) != 2 || matches[0] != stamp {
		return "", errors.New("invalid executable release stamp")
	}
	return Version(matches[1])
}

// StampedVersion retains and checks the release stamp. Unstamped developer
// binaries may still print their development version.
func StampedVersion(fallback string) (string, error) {
	if ReleaseIdentity == "" {
		return fallback, nil
	}
	stamped, err := ParseReleaseIdentity(ReleaseIdentity)
	if err != nil {
		return "", err
	}
	actual, err := Version(fallback)
	if err != nil || actual != stamped {
		return "", errors.New("executable version conflicts with release stamp")
	}
	return fallback, nil
}

func RuntimeReleaseIdentity(version string) (string, error) {
	if ReleaseIdentity == "" {
		return "", errors.New("executable has no embedded release identity")
	}
	if _, err := StampedVersion(version); err != nil {
		return "", err
	}
	return ReleaseIdentity, nil
}

func settings(info *debug.BuildInfo) (goos, goarch, goarm string) {
	for _, setting := range info.Settings {
		switch setting.Key {
		case "GOOS":
			goos = setting.Value
		case "GOARCH":
			goarch = setting.Value
		case "GOARM":
			goarm = setting.Value
		}
	}
	return
}

// Target derives the published target from compiler settings, never the host CPU
// or uname. An ARMv6 executable stays ARMv6 on an ARMv7-capable host.
func Target(info *debug.BuildInfo) (string, error) {
	if info == nil {
		return "", errors.New("missing Go build metadata")
	}
	goos, goarch, goarm := settings(info)
	return TargetForSettings(goos, goarch, goarm)
}

func TargetForSettings(goos, goarch, goarm string) (string, error) {
	if goarch == "arm" {
		parts := strings.Split(goarm, ",")
		level := parts[0]
		if len(parts) > 2 || (len(parts) == 2 && parts[1] != "softfloat" && parts[1] != "hardfloat") {
			return "", fmt.Errorf("unsupported GOARM %s", goarm)
		}
		if goos != "linux" || (level != "6" && level != "7") {
			return "", fmt.Errorf("unsupported agent target %s/%s GOARM=%s", goos, goarch, goarm)
		}
		return "linux-armv" + level, nil
	}
	target := goos + "-" + goarch
	switch target {
	case "linux-amd64", "linux-arm64", "linux-386", "darwin-amd64", "darwin-arm64", "windows-amd64", "windows-arm64", "windows-386", "freebsd-amd64", "freebsd-arm64":
		return target, nil
	}
	return "", fmt.Errorf("unsupported agent target %s", target)
}

func CurrentTarget() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", errors.New("missing running executable Go build metadata")
	}
	return Target(info)
}

// Validate checks the main package and published target using real executable
// metadata, then reads the bounded explicit release stamp. Duplicate identical
// stamps are valid because non-trimpath linker metadata can repeat -X values.
func Validate(path, expectedVersion, expectedTarget string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return ValidateReader(file, expectedVersion, expectedTarget)
}

func ValidateReader(r io.ReaderAt, expectedVersion, expectedTarget string) error {
	var excess [1]byte
	if n, _ := r.ReadAt(excess[:], 100*1024*1024); n != 0 {
		return errors.New("executable exceeds 100 MiB limit")
	}
	info, err := buildinfo.Read(r)
	if err != nil {
		return fmt.Errorf("read executable build metadata: %w", err)
	}
	if info.Path != Package {
		return fmt.Errorf("unexpected executable package %q", info.Path)
	}
	target, err := Target(info)
	if err != nil {
		return err
	}
	if expectedTarget != "" && target != expectedTarget {
		return fmt.Errorf("executable target %s does not match %s", target, expectedTarget)
	}
	identity, err := readIdentity(io.NewSectionReader(r, 0, 100*1024*1024+1))
	if err != nil {
		return err
	}
	if err := validateRecordedStamp(info, identity); err != nil {
		return err
	}
	if expectedVersion != "" {
		expected, err := Version(expectedVersion)
		if err != nil {
			return err
		}
		if identity != expected {
			return fmt.Errorf("stale build: executable version %s does not match %s", identity, expected)
		}
	}
	return nil
}

// Keep the literal prefix escaped in program constants: the scanner itself
// must not introduce an unterminated release marker into every executable.
var identityPrefix = []byte(strings.ReplaceAll(`\[\[pulse-agent-release:`, `\[`, `[`))

func readIdentity(r io.Reader) (string, error) {
	reader := bufio.NewReaderSize(r, 64*1024)
	pending := []byte{}
	identity := ""
	for {
		chunk := make([]byte, 64*1024)
		n, readErr := reader.Read(chunk)
		window := append(pending, chunk[:n]...)
		pending = nil
		cursor := 0
		for cursor < len(window) {
			offset := bytes.Index(window[cursor:], identityPrefix)
			if offset < 0 {
				keep := len(identityPrefix) - 1
				if keep > len(window)-cursor {
					keep = len(window) - cursor
				}
				pending = append([]byte(nil), window[len(window)-keep:]...)
				break
			}
			start := cursor + offset
			closing := bytes.Index(window[start+len(identityPrefix):], []byte("]]"))
			if closing < 0 {
				if len(window)-start > maxIdentitySize {
					return "", errors.New("oversized executable release identity")
				}
				pending = append([]byte(nil), window[start:]...)
				break
			}
			end := start + len(identityPrefix) + closing + 2
			stamp := string(window[start:end])
			if _, err := ParseReleaseIdentity(stamp); err != nil {
				return "", err
			}
			if identity != "" && identity != stamp {
				return "", errors.New("conflicting executable release identities")
			}
			identity = stamp
			cursor = end
		}
		if readErr == io.EOF {
			if bytes.HasPrefix(pending, identityPrefix) {
				return "", errors.New("unterminated executable release identity")
			}
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	if identity == "" {
		return "", errors.New("missing bounded executable release identity")
	}
	return ParseReleaseIdentity(identity)
}

// Recorded linker metadata is optional, but must corroborate the canonical
// stamp assignment when it contains a release marker. This catches accidental
// wrong-symbol flags. It does not authenticate source or replace signed assets
// and the updater's runtime probe.
func validateRecordedStamp(info *debug.BuildInfo, identity string) error {
	for _, setting := range info.Settings {
		if setting.Key != "-ldflags" || !strings.Contains(setting.Value, string(identityPrefix)) {
			continue
		}
		fields := strings.Fields(setting.Value)
		for index := 0; index < len(fields); index++ {
			value := ""
			if fields[index] == "-X" && index+1 < len(fields) {
				index++
				value = fields[index]
			} else if strings.HasPrefix(fields[index], "-X=") {
				value = strings.TrimPrefix(fields[index], "-X=")
			}
			value = strings.Trim(value, "\"'")
			const assignment = "github.com/rcourtman/pulse-go-rewrite/internal/agentidentity.ReleaseIdentity="
			if strings.HasPrefix(value, assignment) {
				stamped, err := ParseReleaseIdentity(strings.TrimPrefix(value, assignment))
				if err == nil && stamped == identity {
					return nil
				}
			}
		}
		return errors.New("recorded linker flags do not stamp the canonical release identity")
	}
	return nil
}
