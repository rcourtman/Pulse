//go:build !windows

package installtests

import (
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Extract the actual copied commands, including the old guide for an exact
// parent control. These tests contact only synthetic peers in the proof VM.
func pbsAPIBlocks(doc string) ([4]string, error) {
	var blocks [4]string
	for _, match := range regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(doc, -1) {
		block := match[1]
		index := -1
		switch {
		case strings.Contains(block, "pbs-agent-token") && strings.Contains(block, "vi "):
			index = 0
		case strings.Contains(block, "pbs-header") && strings.Contains(block, "vi "):
			index = 1
		case strings.Contains(block, "pbs-agent-install.sh") && strings.Contains(block, "curl "):
			index = 2
		case strings.Contains(block, "https://pbs.example.com:8007/api2/json/admin/datastore"):
			index = 3
		}
		if index >= 0 {
			if blocks[index] != "" {
				return blocks, fmt.Errorf("duplicate PBS recipe %d", index)
			}
			blocks[index] = block
		}
	}
	for i, block := range blocks {
		if block == "" {
			return blocks, fmt.Errorf("missing PBS recipe %d", i)
		}
	}
	return blocks, nil
}

func pbsAPISafety(blocks [4]string) error {
	for i, block := range blocks {
		required := []string{"set -eu", "umask 077", `[ -L "$HOME/.config" ]`, `[ -L "$config_dir" ]`, `chmod 700 "$config_dir"`}
		if i < 2 {
			required = append(required, `[ -L "$credential_file" ]`, `[ ! -f "$credential_file" ]`, `chmod 600 "$credential_file"`, `vi "$credential_file"`)
		} else {
			required = append(required, "curl --disable ", "--silent --show-error --proto '=https'", "--connect-timeout 5",
				`--write-out '%{http_code}'`, `printf 'HTTP %s\n' "$status"`, `[ "$curl_exit" -eq 0 ] || exit "$curl_exit"`, `[ "$status" = 200 ]`)
			if i == 2 {
				required = append(required, "--fail ", "--max-time 60", `download_file=$(mktemp "$config_dir/pbs-agent-download.XXXXXX")`,
					`[ -e "$installer_file" ] || [ -L "$installer_file" ]`, `--output "$download_file"`, `mv -n "$download_file" "$installer_file"`, `[ ! -e "$download_file" ]`)
			} else {
				required = append(required, "--fail-with-body ", "--max-time 15", `[ -L "$credential_file" ]`, `[ ! -f "$credential_file" ]`,
					`chmod 600 "$credential_file"`, `result_file=$(mktemp "$config_dir/pbs-response.XXXXXX")`, `--header "@$credential_file"`, `--output "$result_file"`)
			}
			for _, forbidden := range []string{"--insecure", "--location", "--config", "--trace", "--verbose", "--retry", "Authorization:", " | "} {
				if strings.Contains(block, forbidden) {
					return fmt.Errorf("PBS recipe %d contains %s", i, forbidden)
				}
			}
			if strings.Count(block, "curl ") != 1 {
				return fmt.Errorf("PBS recipe %d must make one request", i)
			}
		}
		for _, text := range required {
			if !strings.Contains(block, text) {
				return fmt.Errorf("PBS recipe %d lacks %s", i, text)
			}
		}
	}
	return nil
}

func pbsAPIGuidance(t *testing.T) [4]string {
	t.Helper()
	doc, err := os.ReadFile(repoFile("docs", "PBS.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := pbsAPIBlocks(string(doc))
	if err != nil {
		t.Fatal(err)
	}
	if err := pbsAPISafety(blocks); err != nil {
		t.Fatal(err)
	}
	mirror, err := os.ReadFile(repoFile("frontend-modern", "public", "docs", "PBS.md"))
	if err != nil || string(mirror) != string(doc) {
		t.Fatal("shipped PBS guide differs from the verified recipes")
	}
	return blocks
}

func runPBSRecipe(t *testing.T, recipe, home, bin string, extraEnv ...string) (string, int) {
	t.Helper()
	requestLimit := 15 * time.Second
	if strings.Contains(recipe, "--max-time 60") {
		requestLimit = 60 * time.Second
	}
	result, err := observeGuidanceRecipe(t, recipe, home, bin, guidanceRecipeLimits(requestLimit), extraEnv...)
	if err != nil {
		t.Fatalf("PBS recipe observation failed: %v; %s", err, result.summary())
	}
	return result.output, result.exit
}

func TestPBSAPIGuidancePreparationIsPrivateAndFailClosed(t *testing.T) {
	for index, filename := range []string{"pbs-agent-token", "pbs-header"} {
		for _, mode := range []string{"new", "existing", "config symlink", "directory symlink", "file symlink", "dangling symlink", "non-regular", "mkdir failure", "chmod failure", "touch failure", "editor failure"} {
			t.Run(filename+"/"+mode, func(t *testing.T) {
				home := t.TempDir()
				bin := filepath.Join(home, "bin")
				if err := os.Mkdir(bin, 0700); err != nil {
					t.Fatal(err)
				}
				edited := filepath.Join(home, "editor-started")
				editor := "#!/bin/sh\nprintf started > \"$EDITOR_STARTED\"\n"
				if mode == "editor failure" {
					editor += "exit 1\n"
				}
				if err := os.WriteFile(filepath.Join(bin, "vi"), []byte(editor), 0700); err != nil {
					t.Fatal(err)
				}
				dir := filepath.Join(home, ".config", "pulse")
				file := filepath.Join(dir, filename)
				if mode == "existing" || strings.Contains(mode, "file symlink") || mode == "dangling symlink" || mode == "non-regular" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
				}
				target := filepath.Join(home, "untouched")
				if strings.Contains(mode, "symlink") {
					link := file
					switch mode {
					case "file symlink":
						if err := os.WriteFile(target, []byte("synthetic-existing-secret"), 0644); err != nil {
							t.Fatal(err)
						}
					case "config symlink", "directory symlink":
						if err := os.Mkdir(target, 0755); err != nil {
							t.Fatal(err)
						}
						link = filepath.Join(home, ".config")
						if mode == "directory symlink" {
							if err := os.Mkdir(link, 0755); err != nil {
								t.Fatal(err)
							}
							link = dir
						}
					}
					if err := os.Symlink(target, link); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "existing" {
					if err := os.WriteFile(file, []byte("synthetic-existing-secret"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "non-regular" {
					if err := os.Mkdir(file, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if strings.HasSuffix(mode, " failure") && mode != "editor failure" {
					if err := os.WriteFile(filepath.Join(bin, strings.TrimSuffix(mode, " failure")), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
						t.Fatal(err)
					}
				}
				output, exit := runPBSRecipe(t, pbsAPIGuidance(t)[index], home, bin, "EDITOR_STARTED="+edited)
				_, editorErr := os.Stat(edited)
				if mode != "new" && mode != "existing" {
					if exit == 0 || (mode != "editor failure" && !os.IsNotExist(editorErr)) {
						t.Fatal("failed or unsafe preparation was accepted or reached the editor")
					}
					if strings.Contains(mode, "symlink") && mode != "dangling symlink" {
						info, err := os.Stat(target)
						wantMode := os.FileMode(0755)
						if mode == "file symlink" {
							wantMode = 0644
							content, err := os.ReadFile(target)
							if err != nil || string(content) != "synthetic-existing-secret" {
								t.Fatal("refused credential path modified its target")
							}
						}
						if err != nil || info.Mode().Perm() != wantMode {
							t.Fatal("refused path modified target permissions")
						}
					}
				} else {
					if exit != 0 || editorErr != nil {
						t.Fatalf("safe preparation failed: exit %d, %s", exit, output)
					}
					for path, permissions := range map[string]os.FileMode{dir: 0700, file: 0600} {
						info, err := os.Stat(path)
						if err != nil || info.Mode().Perm() != permissions {
							t.Fatal("credential path is not private")
						}
					}
					content, err := os.ReadFile(file)
					if err != nil || (mode == "existing" && string(content) != "synthetic-existing-secret") {
						t.Fatal("existing credential was not preserved")
					}
				}
				if strings.Contains(output, "synthetic-existing-secret") {
					t.Fatal("preparation disclosed a credential")
				}
			})
		}
	}
}

func TestPBSAPIGuidanceRejectsUnsafeRecipesAndExactParent(t *testing.T) {
	blocks := pbsAPIGuidance(t)
	for name, edit := range map[string][2]string{
		"curl config": {"curl --disable ", "curl "}, "body disclosure": {`--output "$result_file" `, ""},
		"deadline": {"--max-time 15 ", ""}, "retry": {"curl --disable ", "curl --disable --retry 2 "},
		"redirect": {"curl --disable ", "curl --disable --location "}, "TLS bypass": {"curl --disable ", "curl --disable --insecure "},
		"inline token": {`"@$credential_file"`, `'Authorization: synthetic-inline-secret'`},
		"fail open":    {"set -eu", ""}, "symlink": {`[ -L "$credential_file" ]`, "false"},
		"installer overwrite": {`mv -n "$download_file"`, `mv "$download_file"`},
	} {
		t.Run(name, func(t *testing.T) {
			changed := blocks
			for i, block := range changed {
				changed[i] = strings.ReplaceAll(block, edit[0], edit[1])
			}
			if pbsAPISafety(changed) == nil {
				t.Fatal("unsafe mutation was accepted")
			}
		})
	}
	if inputs := os.Getenv("PULSE_PROOF_INPUTS"); inputs != "" {
		parent, err := os.ReadFile(filepath.Join(inputs, "pbs-parent.md"))
		if os.IsNotExist(err) {
			return // Unrelated proof inputs do not require a PBS parent.
		}
		if err != nil {
			t.Fatal(err)
		}
		old, err := pbsAPIBlocks(string(parent))
		if err != nil {
			t.Fatal(err)
		}
		if err := pbsAPISafety(old); err == nil {
			t.Fatal("unchanged exact parent passed the new safety controls")
		} else {
			t.Logf("exact parent rejected: %s", err)
		}
	}
}

// Actual curl sees hostile local defaults and peers that echo a fixture token
// and identifying data. This proves copied transport/file safety, not native
// PBS permissions, installed collection, backup recovery or agent installation.
func TestPBSAPIGuidanceCopiedRequestsKeepSecretsPrivate(t *testing.T) {
	blocks := pbsAPIGuidance(t)
	for index, recipe := range blocks[2:] {
		for _, scenario := range []string{"success", "401", "403", "redirect", "wrong success status", "untrusted HTTPS", "truncated installer", "existing installer", "symlinked header"} {
			if (index == 0 && scenario == "symlinked header") || (index == 1 && strings.Contains(scenario, "installer")) {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s", index, scenario), func(t *testing.T) {
				recipe := recipe
				realCurl, err := exec.LookPath("curl")
				if err != nil {
					t.Fatal("curl is required to verify copied PBS guidance")
				}
				home := t.TempDir()
				bin := filepath.Join(home, "bin")
				dir := filepath.Join(home, ".config", "pulse")
				for _, path := range []string{bin, dir} {
					if err := os.MkdirAll(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
				const token = "PBSAPIToken=pulse-monitor@pbs!pulse-token:synthetic-pbs-secret"
				const body = "synthetic-pbs-secret private-datastore-name"
				header := filepath.Join(dir, "pbs-header")
				if scenario == "symlinked header" {
					target := filepath.Join(home, "outside-header")
					if err := os.WriteFile(target, []byte("Authorization: "+token+"\n"), 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, header); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(header, []byte("Authorization: "+token+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				installer := filepath.Join(dir, "pbs-agent-install.sh")
				if scenario == "existing installer" {
					if err := os.WriteFile(installer, []byte("original-installer"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				capture := filepath.Join(home, "argv")
				trace := filepath.Join(home, "must-not-trace")
				wrapper := "#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$PBS_ARGV\"\nexec \"$PBS_CURL\" \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(wrapper), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, ".curlrc"), []byte("insecure\nlocation\ntrace = "+trace+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				requests := make(chan string, 4)
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests <- strings.Join([]string{r.Method, r.URL.Path, r.Header.Get("Authorization")}, "\n")
					code := 200
					switch scenario {
					case "401":
						code = 401
					case "403":
						code = 403
					case "redirect":
						w.Header().Set("Location", "/must-not-follow")
						code = 307
					case "wrong success status":
						code = 206
					case "truncated installer":
						w.Header().Set("Content-Length", "1024")
					}
					w.WriteHeader(code)
					fmt.Fprint(w, body)
				}))
				server.Config.ErrorLog = log.New(io.Discard, "", 0)
				server.StartTLS()
				t.Cleanup(server.Close)
				originalURL := "https://pulse.example.com"
				if index == 1 {
					originalURL = "https://pbs.example.com:8007"
				}
				recipe = strings.Replace(recipe, originalURL, server.URL, 1)
				if scenario != "untrusted HTTPS" {
					ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
					if err := os.WriteFile(filepath.Join(home, "ca.pem"), ca, 0600); err != nil {
						t.Fatal(err)
					}
					recipe = strings.Replace(recipe, "curl --disable ", `curl --disable --cacert "$HOME/ca.pem" `, 1)
				}
				if index == 0 && scenario == "wrong success status" {
					// The old 25-second watchdog expired during this controlled
					// preparation, before curl. Keep the real 60-second download
					// limit and verify that HTTP 206 still cannot become an installer.
					recipe = "sleep 26\n" + recipe
				}
				output, exit := runPBSRecipe(t, recipe, home, bin, "PBS_ARGV="+capture, "PBS_CURL="+realCurl)
				wantExit := 0
				switch scenario {
				case "401", "403":
					wantExit = 22
				case "untrusted HTTPS":
					wantExit = 60
				case "truncated installer":
					wantExit = 18
				case "redirect", "wrong success status", "existing installer", "symlinked header":
					wantExit = 1
				}
				if exit != wantExit {
					t.Fatalf("copied request exit %d, want %d: %s", exit, wantExit, output)
				}
				if strings.Contains(output, "synthetic-pbs-secret") || strings.Contains(output, "private-datastore-name") {
					t.Fatal("copied request disclosed a credential or response")
				}
				if _, err := os.Stat(trace); !os.IsNotExist(err) {
					t.Fatal("curl configuration enabled tracing")
				}
				if scenario == "existing installer" || scenario == "symlinked header" {
					if _, err := os.Stat(capture); !os.IsNotExist(err) || len(requests) != 0 {
						t.Fatal("unsafe local path reached curl")
					}
					path, expected := installer, "original-installer"
					if scenario == "symlinked header" {
						path, expected = filepath.Join(home, "outside-header"), "Authorization: "+token+"\n"
					}
					content, err := os.ReadFile(path)
					if err != nil || string(content) != expected {
						t.Fatal("refused local path modified existing content")
					}
					return
				}
				argv, err := os.ReadFile(capture)
				if err != nil || strings.Contains(string(argv), "synthetic-pbs-secret") || !strings.HasPrefix(string(argv), "--disable\x00") {
					t.Fatal("curl arguments disclosed credentials or did not disable local configuration first")
				}
				if !strings.Contains(output, "HTTP ") {
					t.Fatal("request failed to expose its safe HTTP result")
				}
				if scenario == "untrusted HTTPS" {
					if len(requests) != 0 {
						t.Fatal("untrusted TLS peer received a request")
					}
				} else {
					if len(requests) != 1 {
						t.Fatal("request was missing, redirected or retried")
					}
					path, auth := "/install.sh", ""
					if index == 1 {
						path, auth = "/api2/json/admin/datastore", token
					}
					if got := <-requests; got != strings.Join([]string{"GET", path, auth}, "\n") {
						t.Fatal("copied request used the wrong method, endpoint or credential")
					}
				}
				if index == 0 {
					content, err := os.ReadFile(installer)
					if scenario == "success" {
						info, statErr := os.Stat(installer)
						if err != nil || string(content) != body || statErr != nil || info.Mode().Perm() != 0600 {
							t.Fatal("successful installer was not saved completely and privately")
						}
					} else if !os.IsNotExist(err) {
						t.Fatal("failed or redirected download became an executable installer path")
					}
				} else {
					responses, err := filepath.Glob(filepath.Join(dir, "pbs-response.??????"))
					if err != nil || len(responses) != 1 {
						t.Fatal("request did not retain one new private response")
					}
					info, err := os.Stat(responses[0])
					if err != nil || info.Mode().Perm() != 0600 {
						t.Fatal("response file is not private")
					}
					if scenario != "untrusted HTTPS" {
						content, err := os.ReadFile(responses[0])
						if err != nil || string(content) != body {
							t.Fatal("complete sensitive response was not retained locally")
						}
					}
				}
			})
		}
	}
}
