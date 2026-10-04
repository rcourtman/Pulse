package installtests

import (
	"context"
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

func trueNASAPIBlocks(doc string) ([]string, error) {
	_, section, found := strings.Cut(doc, "### Testing and adding a connection (API)\n")
	if !found {
		return nil, fmt.Errorf("connection API guidance is missing")
	}
	section, _, _ = strings.Cut(section, "\n## Troubleshooting")
	matches := regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(section, -1)
	if len(matches) != 3 {
		return nil, fmt.Errorf("want separate preparation/test/save blocks, got %d", len(matches))
	}
	blocks := make([]string, 3)
	for i, match := range matches {
		blocks[i] = match[1]
	}
	return blocks, nil
}

func trueNASAPISafety(blocks []string) error {
	for _, required := range []string{"set -eu", "umask 077", `[ -L "$HOME/.config" ]`,
		`[ -L "$config_dir" ]`, `[ -L "$connection_file" ]`, `[ ! -f "$connection_file" ]`,
		`chmod 700 "$config_dir"`, `chmod 600 "$connection_file"`, `vi "$connection_file"`} {
		if !strings.Contains(blocks[0], required) {
			return fmt.Errorf("unsafe file preparation: missing %s", required)
		}
	}
	for i, block := range blocks[1:] {
		for _, required := range []string{"set -eu", "umask 077", `result_file=$(mktemp "$HOME/.config/pulse/truenas-`,
			"status=$(curl --disable --fail-with-body --silent --show-error", "--connect-timeout 5 --max-time 20",
			"--proto '=http' --noproxy 127.0.0.1", `--header "@$HOME/.config/pulse/api-header"`,
			`--data-binary "@$HOME/.config/pulse/truenas-connection.json"`,
			`--output "$result_file" --write-out '%{http_code}'`,
			fmt.Sprintf(`[ "$status" = %d ]`, 200+i)} {
			if !strings.Contains(block, required) {
				return fmt.Errorf("unsafe request %d: missing %s", i+1, required)
			}
		}
		for _, forbidden := range []string{"--insecure", "--location", "--trace", "--verbose", "--config", "--retry", "X-API-Token:", "Authorization:"} {
			if strings.Contains(block, forbidden) {
				return fmt.Errorf("unsafe request: %s", forbidden)
			}
		}
		if strings.Count(block, "curl ") != 1 || strings.Contains(block, " | ") {
			return fmt.Errorf("each copied request must retain its own result and make only one POST")
		}
	}
	return nil
}

func trueNASAPIGuidance(t *testing.T) []string {
	t.Helper()
	doc, err := os.ReadFile(repoFile("docs", "TRUENAS.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := trueNASAPIBlocks(string(doc))
	if err != nil {
		t.Fatal(err)
	}
	if err := trueNASAPISafety(blocks); err != nil {
		t.Fatal(err)
	}
	mirror, err := os.ReadFile(repoFile("frontend-modern", "public", "docs", "TRUENAS.md"))
	if err != nil || string(mirror) != string(doc) {
		t.Fatal("shipped TrueNAS guide must be identical to the verified recipe")
	}
	return blocks
}

func runTrueNASRecipe(t *testing.T, recipe, home, bin string, extraEnv ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", recipe)
	cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+bin+":"+os.Getenv("PATH"),
		"http_proxy=", "HTTP_PROXY=", "https_proxy=", "HTTPS_PROXY=", "ALL_PROXY=", "all_proxy=", "NO_PROXY=", "no_proxy=")
	cmd.Env = append(cmd.Env, extraEnv...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("copied recipe exceeded its bounded test deadline")
	}
	if err == nil {
		return string(output), 0
	}
	failure, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatal(err)
	}
	return string(output), failure.ExitCode()
}

func TestTrueNASAPIGuidancePreparationIsPrivateAndFailClosed(t *testing.T) {
	for _, mode := range []string{"new", "existing", "config symlink", "directory symlink", "file symlink", "non-regular", "mkdir failure", "chmod failure"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			bin := filepath.Join(home, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			edited := filepath.Join(home, "editor-started")
			if err := os.WriteFile(filepath.Join(bin, "vi"), []byte("#!/bin/sh\nprintf 'started' > \"$EDITOR_STARTED\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(home, ".config", "pulse")
			file := filepath.Join(dir, "truenas-connection.json")
			if mode == "existing" || mode == "file symlink" || mode == "non-regular" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			target := filepath.Join(home, "untouched")
			if strings.Contains(mode, "symlink") {
				link := file
				if mode == "file symlink" {
					if err := os.WriteFile(target, []byte("private-existing-key"), 0644); err != nil {
						t.Fatal(err)
					}
				} else {
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
				if err := os.WriteFile(file, []byte("private-existing-key"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "non-regular" {
				if err := os.Mkdir(file, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasSuffix(mode, " failure") {
				if err := os.WriteFile(filepath.Join(bin, strings.TrimSuffix(mode, " failure")), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			output, exit := runTrueNASRecipe(t, trueNASAPIGuidance(t)[0], home, bin, "EDITOR_STARTED="+edited)
			_, editorErr := os.Stat(edited)
			if mode != "new" && mode != "existing" {
				if exit == 0 || !os.IsNotExist(editorErr) {
					t.Fatal("unsafe or failed preparation reached the editor")
				}
				if strings.Contains(mode, "symlink") {
					info, err := os.Stat(target)
					if err != nil || info.Mode().Perm() != map[bool]os.FileMode{true: 0644, false: 0755}[mode == "file symlink"] {
						t.Fatal("refused path modified its target permissions")
					}
				}
			} else {
				if exit != 0 || editorErr != nil {
					t.Fatalf("safe preparation failed: exit %d, %s", exit, output)
				}
				for path, permissions := range map[string]os.FileMode{dir: 0700, file: 0600} {
					info, err := os.Stat(path)
					if err != nil || info.Mode().Perm() != permissions {
						t.Fatal("credential path has incorrect permissions")
					}
				}
				content, err := os.ReadFile(file)
				if err != nil || (mode == "existing" && string(content) != "private-existing-key") {
					t.Fatal("preparation did not preserve the existing credential file")
				}
			}
			if strings.Contains(output, "private-existing-key") {
				t.Fatal("preparation exposed private contents")
			}
		})
	}
}

func TestTrueNASAPIGuidanceRejectsUnsafeRecipesAndExactParent(t *testing.T) {
	blocks := trueNASAPIGuidance(t)
	for name, edit := range map[string][2]string{
		"config": {"curl --disable ", "curl "}, "body": {`--output "$result_file" `, ""},
		"deadline": {"--max-time 20 ", ""}, "retry": {"curl --disable ", "curl --disable --retry 2 "},
		"redirect":     {"curl --disable ", "curl --disable --location "},
		"inline token": {`"@$HOME/.config/pulse/api-header"`, `'X-API-Token: synthetic-secret'`},
		"fail-open":    {"set -eu", ""}, "wrong save status": {`[ "$status" = 201 ]`, `[ "$status" = 200 ]`},
	} {
		t.Run(name, func(t *testing.T) {
			changed := make([]string, len(blocks))
			for i, block := range blocks {
				changed[i] = strings.ReplaceAll(block, edit[0], edit[1])
			}
			if trueNASAPISafety(changed) == nil {
				t.Fatal("unsafe mutation was accepted")
			}
		})
	}
	if inputs := os.Getenv("PULSE_PROOF_INPUTS"); inputs != "" {
		parent, err := os.ReadFile(filepath.Join(inputs, "truenas-parent.md"))
		if err != nil {
			t.Fatal(err)
		}
		old, err := trueNASAPIBlocks(string(parent))
		if err != nil {
			t.Fatal(err)
		}
		if err := trueNASAPISafety(old); err == nil {
			t.Fatal("unchanged exact parent guide passed new safety controls")
		} else {
			t.Logf("exact parent rejected: %s", err)
		}
	}
}

// Synthetic peers exercise the actual copied commands and deliberately echo
// both fixture secrets. This is credential/transport safety, not native
// TrueNAS collection, Pulse authorisation or a customer's setup acceptance.
func TestTrueNASAPIGuidanceCopiedRequestsKeepSecretsPrivate(t *testing.T) {
	for _, scenario := range []string{"success", "HTTP error", "redirect", "wrong success status", "trusted HTTPS", "untrusted HTTPS"} {
		for index, recipe := range trueNASAPIGuidance(t)[1:] {
			t.Run(fmt.Sprintf("%s/%d", scenario, index+1), func(t *testing.T) {
				realCurl, err := exec.LookPath("curl")
				if err != nil {
					t.Fatal("curl is required to verify copied TrueNAS guidance")
				}
				home := t.TempDir()
				bin := filepath.Join(home, "bin")
				dir := filepath.Join(home, ".config", "pulse")
				for _, path := range []string{bin, dir} {
					if err := os.MkdirAll(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
				const token = "synthetic-pulse-secret"
				const key = "synthetic-truenas-secret"
				const body = `{"host":"https://synthetic.invalid","apiKey":"` + key + `"}`
				for file, content := range map[string]string{"api-header": "X-API-Token: " + token, "truenas-connection.json": body} {
					if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				capture := filepath.Join(home, "argv")
				trace := filepath.Join(home, "must-not-trace")
				wrapper := "#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$TN_ARGV\"\nexec \"$TN_CURL\" \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(wrapper), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, ".curlrc"), []byte("insecure\nlocation\ntrace = "+trace+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				requests := make(chan string, 4)
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					payload, _ := io.ReadAll(r.Body)
					requests <- strings.Join([]string{r.Method, r.URL.Path, r.Header.Get("X-API-Token"), r.Header.Get("Content-Type"), string(payload)}, "\n")
					code := 200 + index
					switch scenario {
					case "HTTP error":
						code = 400
					case "redirect":
						w.Header().Set("Location", "/must-not-follow")
						code = 307
					case "wrong success status":
						code = 201 - index
					}
					w.WriteHeader(code)
					fmt.Fprint(w, token+" "+key+" private-response")
				}))
				server.Config.ErrorLog = log.New(io.Discard, "", 0)
				if strings.Contains(scenario, "HTTPS") {
					server.StartTLS()
					recipe = strings.Replace(recipe, "--proto '=http'", "--proto '=https'", 1)
					if scenario == "trusted HTTPS" {
						ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
						if err := os.WriteFile(filepath.Join(home, "ca.pem"), ca, 0600); err != nil {
							t.Fatal(err)
						}
						recipe = strings.Replace(recipe, "curl --disable ", `curl --disable --cacert "$HOME/ca.pem" `, 1)
					}
				} else {
					server.Start()
				}
				t.Cleanup(server.Close)
				recipe = strings.Replace(recipe, "http://127.0.0.1:7655", server.URL, 1)
				output, exit := runTrueNASRecipe(t, recipe, home, bin, "TN_ARGV="+capture, "TN_CURL="+realCurl)
				wantExit := 0
				if scenario == "HTTP error" {
					wantExit = 22
				} else if scenario == "untrusted HTTPS" {
					wantExit = 60
				} else if scenario == "redirect" || scenario == "wrong success status" {
					wantExit = 1
				}
				if exit != wantExit {
					t.Fatalf("copied request exit %d, want %d: %s", exit, wantExit, output)
				}
				argv, err := os.ReadFile(capture)
				if err != nil {
					t.Fatal(err)
				}
				for _, secret := range []string{token, key, "private-response"} {
					if strings.Contains(string(argv), secret) || strings.Contains(output, secret) {
						t.Fatal("copied request exposed a fixture credential or response")
					}
				}
				if _, err := os.Stat(trace); !os.IsNotExist(err) {
					t.Fatal("curl configuration enabled tracing")
				}
				responses, err := filepath.Glob(filepath.Join(dir, "truenas-*.??????"))
				if err != nil || len(responses) != 1 {
					t.Fatal("request must retain one new private response file")
				}
				info, err := os.Stat(responses[0])
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatal("response file is not private")
				}
				if scenario == "untrusted HTTPS" {
					if len(requests) != 0 {
						t.Fatal("untrusted TLS peer received credentials")
					}
				} else {
					if len(requests) != 1 {
						t.Fatal("request was missing, redirected or retried")
					}
					path := "/api/truenas/connections"
					if index == 0 {
						path += "/test"
					}
					if got := <-requests; got != strings.Join([]string{"POST", path, token, "application/json", body}, "\n") {
						t.Fatal("copied POST differs from the documented endpoint, header or JSON file")
					}
					response, err := os.ReadFile(responses[0])
					if err != nil || string(response) != token+" "+key+" private-response" {
						t.Fatal("complete sensitive response was not retained locally")
					}
				}
			})
		}
	}
}
