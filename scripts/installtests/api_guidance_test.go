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

// Execute the actual reference recipes against synthetic peers only. In
// particular, action examples must not acquire an automatic retry or emit a
// private response just because a POST failed or its response was incomplete.
func pulseAPIRecipes(doc string) (preparation, helper string, calls []string, err error) {
	for _, match := range regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(doc, -1) {
		block := match[1]
		switch {
		case strings.Contains(block, "api-header") && strings.Contains(block, "vi "):
			preparation = block
		case strings.HasPrefix(block, "pulse_api() ("):
			helper = block
		case strings.HasPrefix(block, "pulse_api "):
			calls = append(calls, block)
		case strings.Contains(block, "curl "):
			return "", "", nil, fmt.Errorf("request bypasses the private-response helper")
		}
	}
	if preparation == "" || helper == "" || len(calls) != 8 {
		err = fmt.Errorf("missing credential/helper recipe or request examples (got %d)", len(calls))
	}
	return
}

func pulseAPIReference(t *testing.T) (string, string, []string) {
	t.Helper()
	doc, err := os.ReadFile(repoFile("docs", "API.md"))
	if err != nil {
		t.Fatal(err)
	}
	mirror, err := os.ReadFile(repoFile("frontend-modern", "public", "docs", "API.md"))
	if err != nil || string(doc) != string(mirror) {
		t.Fatal("shipped API reference differs from the tested recipes")
	}
	prep, helper, calls, err := pulseAPIRecipes(string(doc))
	if err != nil {
		t.Fatal(err)
	}
	return prep, helper, calls
}

func runPulseAPIRecipe(t *testing.T, script, home, bin string, extraEnv ...string) (string, int) {
	t.Helper()
	result, err := observeGuidanceRecipe(t, script, home, bin, guidanceRecipeLimits(20*time.Second), extraEnv...)
	if err != nil {
		t.Fatalf("API recipe observation failed: %v; %s", err, result.summary())
	}
	return result.output, result.exit
}

func TestPulseAPIGuidancePreparationIsPrivateAndFailClosed(t *testing.T) {
	prep, _, _ := pulseAPIReference(t)
	for _, mode := range []string{"new", "existing", "config symlink", "directory symlink", "file symlink", "dangling symlink", "non-regular", "mkdir failure", "chmod failure", "touch failure", "editor failure"} {
		t.Run(mode, func(t *testing.T) {
			home, bin := t.TempDir(), t.TempDir()
			dir := filepath.Join(home, ".config", "pulse")
			file := filepath.Join(dir, "api-header")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if mode == "new" || mode == "mkdir failure" {
				if err := os.Remove(dir); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "existing" {
				if err := os.WriteFile(file, []byte("synthetic-existing-header"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "non-regular" {
				if err := os.Mkdir(file, 0700); err != nil {
					t.Fatal(err)
				}
			}
			target := filepath.Join(home, "unchanged")
			if strings.Contains(mode, "symlink") {
				link := file
				if mode == "config symlink" || mode == "directory symlink" {
					link = dir
					if mode == "config symlink" {
						link = filepath.Join(home, ".config")
					}
					if err := os.Remove(dir); err != nil {
						t.Fatal(err)
					}
					if mode == "config symlink" {
						if err := os.Remove(link); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.Mkdir(target, 0755); err != nil {
						t.Fatal(err)
					}
				} else if mode == "file symlink" {
					if err := os.WriteFile(target, []byte("synthetic-existing-header"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
			}
			edited := filepath.Join(home, "editor-started")
			editor := "#!/bin/sh\nprintf started > \"$API_EDITED\"\n"
			if mode == "editor failure" {
				editor += "exit 7\n"
			}
			if err := os.WriteFile(filepath.Join(bin, "vi"), []byte(editor), 0700); err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(mode, " failure") && mode != "editor failure" {
				name := strings.TrimSuffix(mode, " failure")
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			output, exit := runPulseAPIRecipe(t, prep, home, bin, "API_EDITED="+edited)
			_, editorErr := os.Stat(edited)
			if mode == "new" || mode == "existing" {
				if exit != 0 || editorErr != nil {
					t.Fatalf("safe preparation failed: exit %d", exit)
				}
				for path, perm := range map[string]os.FileMode{dir: 0700, file: 0600} {
					info, err := os.Stat(path)
					if err != nil || info.Mode().Perm() != perm {
						t.Fatal("credential paths are not private")
					}
				}
				if mode == "existing" {
					data, err := os.ReadFile(file)
					if err != nil || string(data) != "synthetic-existing-header" {
						t.Fatal("preparation did not preserve the existing header")
					}
				}
			} else if exit == 0 || (mode != "editor failure" && !os.IsNotExist(editorErr)) {
				t.Fatal("unsafe or failed preparation reached the editor or succeeded")
			}
			if mode == "file symlink" {
				data, err := os.ReadFile(target)
				info, statErr := os.Stat(target)
				if err != nil || statErr != nil || string(data) != "synthetic-existing-header" || info.Mode().Perm() != 0644 {
					t.Fatal("preparation modified the symlink target")
				}
			}
			if mode == "config symlink" || mode == "directory symlink" {
				info, err := os.Stat(target)
				if err != nil || info.Mode().Perm() != 0755 {
					t.Fatal("preparation changed the target directory")
				}
			}
			if strings.Contains(output, "synthetic-existing-header") {
				t.Fatal("preparation disclosed a header")
			}
		})
	}
}

func TestPulseAPIGuidanceHelperStopsBeforeCurlOnUnsafeFilesOrPreparation(t *testing.T) {
	_, helper, _ := pulseAPIReference(t)
	for _, mode := range []string{"missing", "file symlink", "config symlink", "directory symlink", "non-regular", "chmod failure", "mktemp failure", "unsupported method", "full URL", "extra argument"} {
		t.Run(mode, func(t *testing.T) {
			home, bin := t.TempDir(), t.TempDir()
			dir := filepath.Join(home, ".config", "pulse")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, "api-header")
			if mode != "missing" && mode != "file symlink" && mode != "non-regular" {
				if err := os.WriteFile(file, []byte("synthetic-header"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "file symlink" {
				if err := os.Symlink(filepath.Join(home, "absent"), file); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "config symlink" || mode == "directory symlink" {
				link := dir
				if mode == "config symlink" {
					link = filepath.Join(home, ".config")
				}
				if err := os.Rename(link, link+"-retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(link+"-retained", link); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "non-regular" {
				if err := os.Mkdir(file, 0700); err != nil {
					t.Fatal(err)
				}
			}
			called := filepath.Join(home, "curl-called")
			if err := os.WriteFile(filepath.Join(bin, "curl"), []byte("#!/bin/sh\nprintf started > \"$API_CALLED\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(mode, " failure") {
				if err := os.WriteFile(filepath.Join(bin, strings.TrimSuffix(mode, " failure")), []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			call := "pulse_api GET /api/state/summary"
			switch mode {
			case "unsupported method":
				call = "pulse_api DELETE /api/actions/act_example"
			case "full URL":
				call = "pulse_api GET https://not-this-instance.invalid/api/state"
			case "extra argument":
				call += " --location"
			}
			// Bash disables implicit errexit inside functions used as conditions.
			// The documented helper must still stop on chmod/mktemp failure.
			script := helper + "\nif " + call + "; then exit 0; else exit $?; fi\n"
			_, exit := runPulseAPIRecipe(t, script, home, bin, "API_CALLED="+called)
			_, err := os.Stat(called)
			if exit == 0 || !os.IsNotExist(err) {
				t.Fatal("invalid input or failed preparation reached curl")
			}
		})
	}
}

type apiRecipeRequest struct{ method, uri, token, contentType, body string }

func TestPulseAPIGuidanceCopiedRequestsKeepResponsesPrivateAndOneShot(t *testing.T) {
	_, helper, calls := pulseAPIReference(t)
	paths := []string{"/api/state/summary", "/api/connections", "/api/agent/resource-capabilities/vm%3A42", "/api/actions/plan", "/api/actions/act_.../decision", "/api/actions/act_.../execute", "/api/audit/actions?resourceId=vm%3A42&limit=10", "/api/audit/actions/act_.../events"}
	for index, call := range calls {
		for _, scenario := range []string{"success", "401", "403", "redirect", "partial", "untrusted HTTPS", "trusted HTTPS"} {
			t.Run(fmt.Sprintf("request-%d/%s", index, scenario), func(t *testing.T) {
				home, bin := t.TempDir(), t.TempDir()
				dir := filepath.Join(home, ".config", "pulse")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				const token = "synthetic-fixture-secret"
				const response = `{"token":"synthetic-fixture-secret","resource":"private-fixture-name","audit":"private-fixture-event"}`
				if err := os.WriteFile(filepath.Join(dir, "api-header"), []byte("X-API-Token: "+token+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
				retained := filepath.Join(dir, "api-response.retained")
				if err := os.WriteFile(retained, []byte("previous-result"), 0600); err != nil {
					t.Fatal(err)
				}
				capture, trace := filepath.Join(home, "argv"), filepath.Join(home, "trace")
				realCurl, err := exec.LookPath("curl")
				if err != nil {
					t.Fatal("curl is required to verify copied API requests")
				}
				wrapper := "#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$API_ARGV\"\nexec \"$API_CURL\" \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(wrapper), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, ".curlrc"), []byte("insecure\nlocation\ntrace = "+trace+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				requests := make(chan apiRecipeRequest, 4)
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					requests <- apiRecipeRequest{r.Method, r.URL.RequestURI(), r.Header.Get("X-API-Token"), r.Header.Get("Content-Type"), string(body)}
					switch scenario {
					case "401":
						w.WriteHeader(http.StatusUnauthorized)
					case "403":
						w.WriteHeader(http.StatusForbidden)
					case "redirect":
						w.Header().Set("Location", "/should-not-receive-header")
						w.WriteHeader(http.StatusFound)
					case "partial":
						w.Header().Set("Content-Length", fmt.Sprint(len(response)+100))
					}
					_, _ = io.WriteString(w, response)
				})
				var server *httptest.Server
				recipe := helper
				if strings.Contains(scenario, "HTTPS") {
					server = httptest.NewUnstartedServer(handler)
					server.Config.ErrorLog = log.New(io.Discard, "", 0)
					server.StartTLS()
					recipe = strings.Replace(recipe, "--proto '=http' --noproxy 127.0.0.1", "--proto '=https'", 1)
					if scenario == "trusted HTTPS" {
						ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
						if err := os.WriteFile(filepath.Join(home, "ca.pem"), ca, 0600); err != nil {
							t.Fatal(err)
						}
						recipe = strings.Replace(recipe, "curl --disable ", `curl --disable --cacert "$HOME/ca.pem" `, 1)
					}
				} else {
					server = httptest.NewServer(handler)
				}
				t.Cleanup(server.Close)
				recipe = strings.Replace(recipe, "http://127.0.0.1:7655", server.URL, 1)
				extra := []string{"API_ARGV=" + capture, "API_CURL=" + realCurl}
				if !strings.Contains(scenario, "HTTPS") {
					// The local recipe must not send credentials to an ambient proxy.
					extra = append(extra, "http_proxy=http://127.0.0.1:1", "ALL_PROXY=http://127.0.0.1:1")
				}
				output, exit := runPulseAPIRecipe(t, recipe+"\n"+call, home, bin, extra...)
				wantExit := map[string]int{"success": 0, "401": 22, "403": 22, "redirect": 1, "partial": 18, "untrusted HTTPS": 60, "trusted HTTPS": 0}[scenario]
				if exit != wantExit {
					t.Fatalf("request exit %d, want %d", exit, wantExit)
				}
				argv, err := os.ReadFile(capture)
				if err != nil || !strings.HasPrefix(string(argv), "--disable\x00") {
					t.Fatal("curl did not ignore configuration first")
				}
				for _, sensitive := range []string{token, "private-fixture-name", "private-fixture-event"} {
					if strings.Contains(output, sensitive) || strings.Contains(string(argv), sensitive) {
						t.Fatal("request printed private credentials or response data")
					}
				}
				if _, err := os.Stat(trace); !os.IsNotExist(err) {
					t.Fatal("local curl configuration enabled tracing")
				}
				responses, err := filepath.Glob(filepath.Join(dir, "api-response.??????"))
				if err != nil || len(responses) != 1 || !strings.Contains(output, responses[0]) {
					t.Fatal("response was not saved to one new, reported private file")
				}
				for path, perm := range map[string]os.FileMode{dir: 0700, filepath.Join(dir, "api-header"): 0600, responses[0]: 0600} {
					info, err := os.Stat(path)
					if err != nil || info.Mode().Perm() != perm {
						t.Fatal("request paths are not private")
					}
				}
				if data, err := os.ReadFile(retained); err != nil || string(data) != "previous-result" {
					t.Fatal("request replaced an earlier response")
				}
				if scenario == "untrusted HTTPS" {
					if len(requests) != 0 {
						t.Fatal("untrusted TLS peer received credentials")
					}
				} else {
					if len(requests) != 1 {
						t.Fatal("request was missing, redirected or retried")
					}
					got := <-requests
					method := "GET"
					contentType, body := "", ""
					if index >= 3 && index <= 5 {
						method, contentType = "POST", "application/json"
						_, body, _ = strings.Cut(call, "<<'JSON'\n")
						body = strings.TrimSuffix(body, "JSON\n")
					}
					if got != (apiRecipeRequest{method, paths[index], token, contentType, body}) {
						t.Fatal("request method, endpoint, private header or JSON body differs from the example")
					}
					if data, err := os.ReadFile(responses[0]); err != nil || string(data) != response {
						t.Fatal("HTTP response, including failure or partial body, was not retained privately")
					}
				}
			})
		}
	}
}

func TestPulseAPIGuidanceTimedOutPOSTIsNotRepeated(t *testing.T) {
	_, helper, calls := pulseAPIReference(t)
	home, bin := t.TempDir(), t.TempDir()
	dir := filepath.Join(home, ".config", "pulse")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api-header"), []byte("X-API-Token: synthetic-timeout-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		requests <- struct{}{}
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "private-partial-response")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	if !strings.Contains(helper, "--connect-timeout 5 --max-time 20") {
		t.Fatal("documented request deadlines are missing")
	}
	// Use the actual documented twenty-second limit, not a test-only shortened
	// deadline. An execution endpoint can have acted even when this read fails.
	helper = strings.Replace(helper, "http://127.0.0.1:7655", server.URL, 1)
	// A deliberate preparation delay exceeds the old whole-shell comparison
	// once added to the actual twenty-second request. It must not shorten curl's
	// limit or be mistaken for request time.
	result, err := observeGuidanceRecipe(t, "sleep 12\n"+helper+"\n"+calls[5], home, bin, guidanceRecipeLimits(20*time.Second))
	if err != nil {
		t.Fatalf("timed-out POST observation failed: %v; %s", err, result.summary())
	}
	t.Log(result.summary())
	output, exit := result.output, result.exit
	if exit != 28 || result.request > 25*time.Second || result.curlStarts != 1 || result.curlEnds != 1 || len(requests) != 1 {
		t.Fatal("POST did not stop at its request deadline or was retried")
	}
	if strings.Contains(output, "private-partial-response") || strings.Contains(output, "synthetic-timeout-secret") {
		t.Fatal("timed-out POST disclosed a private body or credential")
	}
	responses, err := filepath.Glob(filepath.Join(dir, "api-response.??????"))
	if err != nil || len(responses) != 1 {
		t.Fatal("timed-out POST did not retain its response")
	}
	if data, err := os.ReadFile(responses[0]); err != nil || string(data) != "private-partial-response" {
		t.Fatal("partial response was not retained for private reconciliation")
	}
}

func TestPulseAPIGuidanceExactParentPrintsPrivateResponse(t *testing.T) {
	inputs := os.Getenv("PULSE_PROOF_INPUTS")
	if inputs == "" {
		t.Skip("exact-parent input is provided by the candidate source proof")
	}
	old, err := os.ReadFile(filepath.Join(inputs, "api-parent.md"))
	if os.IsNotExist(err) {
		t.Skip("unrelated proof input does not supply the exact parent")
	}
	if err != nil {
		t.Fatal(err)
	}
	var request string
	for _, match := range regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(string(old), -1) {
		if strings.Contains(match[1], "/api/state/summary") && strings.Contains(match[1], "curl ") {
			request = match[1]
			break
		}
	}
	if request == "" {
		t.Fatal("exact old reference summary command is missing")
	}
	home, bin := t.TempDir(), t.TempDir()
	dir := filepath.Join(home, ".config", "pulse")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api-header"), []byte("X-API-Token: synthetic-parent-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	const response = "synthetic-parent-secret private-parent-resource"
	requests := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		if r.Header.Get("X-API-Token") != "synthetic-parent-secret" {
			w.WriteHeader(http.StatusUnauthorized)
		}
		_, _ = io.WriteString(w, response)
	}))
	defer server.Close()
	request = strings.Replace(request, "http://127.0.0.1:7655", server.URL, 1)
	output, exit := runPulseAPIRecipe(t, request, home, bin)
	if exit != 0 || len(requests) != 1 || !strings.Contains(output, response) {
		t.Fatal("exact parent did not reproduce the private-response console exposure")
	}
}

func TestPulseAPIGuidanceRejectsExactOldReference(t *testing.T) {
	inputs := os.Getenv("PULSE_PROOF_INPUTS")
	if inputs == "" {
		t.Skip("exact-parent input is provided by the candidate source proof")
	}
	old, err := os.ReadFile(filepath.Join(inputs, "api-parent.md"))
	if os.IsNotExist(err) {
		t.Skip("unrelated proof input does not supply the exact parent")
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := pulseAPIRecipes(string(old)); err == nil {
		t.Fatal("old direct, unbounded console-response examples were accepted")
	}
}
