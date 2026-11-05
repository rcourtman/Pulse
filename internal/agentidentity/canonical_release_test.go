package agentidentity

import (
	"bytes"
	"debug/buildinfo"
	"os"
	"path/filepath"
	"testing"
)

// The worker supplies actual cmd/pulse-agent builds made by the canonical
// release recipe. Lightweight real-format fixtures exercise the same parser in
// ordinary unit runs; this opt-in proof also covers the full dependency graph.
func TestCanonicalReleaseExecutables(t *testing.T) {
	root := os.Getenv("PULSE_AGENT_RELEASE_TEST_DIR")
	if root == "" {
		t.Skip("requires canonical cross-build worker packet")
	}
	for _, target := range []string{"linux-amd64", "windows-amd64", "darwin-arm64"} {
		t.Run(target, func(t *testing.T) {
			path := filepath.Join(root, target)
			if err := Validate(path, "v6.2.1", target); err != nil {
				t.Fatal(err)
			}
			if err := Validate(path, "v6.2.10", target); err == nil {
				t.Fatal("accepted wrong exact version")
			}
			if err := Validate(path, "v6.2.1", "linux-armv6"); err == nil {
				t.Fatal("accepted wrong executable target")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// This real dependency version appears in Go build metadata. Its presence
			// must never satisfy the advertised main-program identity.
			info, err := buildinfo.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			dependencyVersion := ""
			for _, dependency := range info.Deps {
				if version, err := Version(dependency.Version); err == nil && version != "v6.2.1" && bytes.Contains(data, []byte(dependency.Version)) {
					dependencyVersion = dependency.Version
					break
				}
			}
			if dependencyVersion == "" {
				t.Fatal("dependency literal control missing")
			}
			if err := Validate(path, dependencyVersion, target); err == nil {
				t.Fatal("accepted dependency version as main-program version")
			}
			for _, version := range []string{"v6.2.1", "v6.2.0"} {
				stamped := append(append([]byte(nil), data...), []byte("[[pulse-agent-release:v1:"+version+"]]")...)
				err := ValidateReader(bytes.NewReader(stamped), "v6.2.1", target)
				if (err == nil) != (version == "v6.2.1") {
					t.Fatalf("duplicate/conflict %s: %v", version, err)
				}
			}
		})
	}
	if err := Validate(filepath.Join(root, "longer-stable"), "v6.2.1", "linux-amd64"); err == nil {
		t.Fatal("accepted 6.2.10 as 6.2.1")
	}
}
