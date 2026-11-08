package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Model only the pre-7.50 option-spelling boundary. The shipped producers still
// use real curl for TLS, redirects, proxy/CA environment and deadline behavior;
// this does not claim qualification with an actual legacy curl executable.
func legacyCurlCLIEnvironment(t *testing.T, root string) []string {
	t.Helper()
	realCurl, err := exec.LookPath("curl")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "legacy-curl-bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	shim := `#!/bin/sh
set -eu
[ "${1:-}" = -q ] || { echo legacy-curl-requires-first-option-q >&2; exit 2; }
for option in "$@"; do
    [ "$option" != --disable ] || { echo legacy-curl-unknown-option >&2; exit 2; }
done
exec "$PULSE_TEST_REAL_CURL" "$@"
`
	writeTestFile(t, filepath.Join(bin, "curl"), []byte(shim), 0700)
	return []string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"PULSE_TEST_REAL_CURL=" + realCurl,
	}
}

// Keep the whole shipped call inventory visible, including both wrapper array
// assignments. The executable cases above and their consumers exercise these
// options; the inventory prevents a new call or merged -q option slipping by.
func TestUnixBootstrapEveryCurlCallDisablesDefaultConfigFirst(t *testing.T) {
	call := regexp.MustCompile(`(?:^[ \t]*(?:if[ \t]+(?:![ \t]+)?)?|\$\(|--[ \t]+)curl[ \t]+(.+)`)
	for name, want := range map[string]int{
		"install-container-agent.sh": 1,
		"install-mcp.sh":             1,
		"pulse-auto-update.sh":       2,
	} {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(repoFile("scripts", name))
			if err != nil {
				t.Fatal(err)
			}
			source, count := string(content), 0
			for _, line := range strings.Split(source, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.Contains(line, "command -v curl") {
					continue
				}
				match := call.FindStringSubmatch(line)
				if match == nil {
					continue
				}
				count++
				if strings.HasPrefix(match[1], "-q ") {
					continue
				}
				if name == "install-container-agent.sh" && strings.HasPrefix(match[1], `"${curl_flags[@]}"`) &&
					strings.Contains(source, "curl_flags=(-q -fsSL)") && strings.Contains(source, "curl_flags=(-q -kfsSL)") {
					continue
				}
				t.Errorf("curl must ignore default config with standalone first-option -q: %s", line)
			}
			if count != want {
				t.Errorf("curl inventory incomplete: got %d want %d", count, want)
			}
		})
	}
}
