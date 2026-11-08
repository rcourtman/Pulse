package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
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
