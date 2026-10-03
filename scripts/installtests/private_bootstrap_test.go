package installtests

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/configapi"
)

// Exercise the real install.sh metadata parser with the actual current server
// envelope and coherent older-server envelopes. This does not install a server
// or execute an artifact's command string. Credentials enter Python via FD3.
func TestRootInstallerPrivateBootstrapMetadata(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	marker := `3<<<"$setup_response" <<'PY'` + "\n"
	_, body, ok := strings.Cut(string(source), marker)
	if !ok {
		t.Fatal("setup parser must receive response on private FD3")
	}
	parser, _, ok := strings.Cut(body, "\nPY\n")
	if !ok {
		t.Fatal("setup parser end missing")
	}
	const pulseURL = "http://10.1.2.3:7655"
	const host = "https://pve.example:8006"
	const token = "0123456789abcdef0123456789abcdef"
	for _, backup := range []bool{false, true} {
		for _, transport := range []string{"modern", "legacy", "mixed"} {
			t.Run(transport+"_backup_"+strconv.FormatBool(backup), func(t *testing.T) {
				artifact := configapi.BuildSetupScriptInstallArtifact(pulseURL, "pve", host, pulseURL, backup, token, time.Now().Add(time.Hour).Unix())
				if transport != "modern" {
					download, err := url.Parse(artifact.URL)
					if err != nil {
						t.Fatal(err)
					}
					query := download.Query()
					query.Set("setup_token", token)
					download.RawQuery = query.Encode()
					artifact.DownloadURL = download.String()
					if transport == "legacy" {
						command := `curl '` + artifact.URL + `' | { if [ "$(id -u)" -eq 0 ]; then env PULSE_SETUP_TOKEN='` + token + `' bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN='` + token + `' bash; fi; }`
						artifact.Command = command
						artifact.CommandWithEnv = command
						artifact.CommandWithoutEnv = strings.ReplaceAll(command, "env PULSE_SETUP_TOKEN='"+token+"' ", "")
					}
				}
				data, err := json.Marshal(artifact)
				if err != nil {
					t.Fatal(err)
				}
				input := filepath.Join(t.TempDir(), "response")
				if err := os.WriteFile(input, data, 0600); err != nil {
					t.Fatal(err)
				}
				file, err := os.Open(input)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				cmd := exec.Command("python3", "-", pulseURL, host, strconv.FormatBool(backup))
				cmd.Stdin = strings.NewReader(parser)
				cmd.ExtraFiles = []*os.File{file}
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("parser failed: %v", err)
				}
				fields := strings.Split(strings.TrimSuffix(string(out), "\n"), "\t")
				if len(fields) != 12 {
					t.Fatal("unexpected metadata fields")
				}
				if transport == "mixed" {
					if fields[6] != "" || fields[7] != "" || fields[8] != "" {
						t.Fatal("mixed transport accepted")
					}
					return
				}
				if fields[0] != token || fields[1] != "pve" || fields[2] != host || fields[3] != artifact.URL || fields[4] != artifact.DownloadURL || fields[6] != artifact.Command || fields[7] != artifact.CommandWithEnv || fields[8] != artifact.CommandWithoutEnv || fields[11] != "live" {
					t.Fatal("root installer discarded valid bootstrap metadata or token")
				}
			})
		}
	}
}
