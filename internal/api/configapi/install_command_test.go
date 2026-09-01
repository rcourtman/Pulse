package configapi

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildProxmoxAgentInstallCommandExecutesTrustedRootAndSudoTokenBootstrap(t *testing.T) {
	if testing.Short() {
		t.Skip("executes the generated POSIX shell command")
	}

	fixtureDir := t.TempDir()
	binDir := filepath.Join(fixtureDir, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	installerPath := filepath.Join(fixtureDir, "installer.sh")
	capturePath := filepath.Join(fixtureDir, "captured-token")
	writeExecutable(t, installerPath, `#!/usr/bin/env bash
set -e
token_file=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --token-file) token_file="$2"; shift 2 ;;
    *) shift ;;
  esac
done
[ -n "$token_file" ]
[ "$(stat -c %a "$token_file" 2>/dev/null || stat -f %Lp "$token_file")" = "600" ]
parent_dir=$(dirname "$token_file")
[ "$(stat -c %a "$parent_dir" 2>/dev/null || stat -f %Lp "$parent_dir")" = "700" ]
[ "$(stat -c %u "$token_file" 2>/dev/null || stat -f %u "$token_file")" = "$(stat -c %u "$parent_dir" 2>/dev/null || stat -f %u "$parent_dir")" ]
cat "$token_file" > "$FAKE_CAPTURE"
`)
	writeExecutable(t, filepath.Join(binDir, "curl"), `#!/bin/sh
cat "$FAKE_INSTALLER"
`)
	writeExecutable(t, filepath.Join(binDir, "sudo"), `#!/bin/sh
exec "$@"
`)
	writeExecutable(t, filepath.Join(binDir, "id"), `#!/bin/sh
if [ "$1" = "-u" ]; then
  printf '%s\n' "$FAKE_ID_UID"
else
  exec /usr/bin/id "$@"
fi
`)

	command := BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{
		BaseURL:            "https://pulse.example",
		Token:              "token-123",
		InstallType:        "pve",
		IncludeInstallType: true,
	})
	for _, fakeUID := range []string{"0", "1000"} {
		t.Run("uid_"+fakeUID, func(t *testing.T) {
			if err := os.Remove(capturePath); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-c", command)
			cmd.Env = append(os.Environ(),
				"PATH="+binDir+":"+os.Getenv("PATH"),
				"FAKE_CAPTURE="+capturePath,
				"FAKE_ID_UID="+fakeUID,
				"FAKE_INSTALLER="+installerPath,
			)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("generated command failed: %v\n%s\ncommand:\n%s", err, output, command)
			}
			got, err := os.ReadFile(capturePath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "token-123" {
				t.Fatalf("captured token = %q, want token-123", got)
			}
		})
	}
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}
