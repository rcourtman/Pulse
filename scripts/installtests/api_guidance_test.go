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
type pulseAPIRequestContract struct {
	name, firstLine, method, uri, input string
}

// Match each copied request to its own transport contract, not its position in
// the guide. New examples must gain a fixture rather than disabling every
// existing check with an unexplained recipe-count mismatch.
func pulseAPIRequestContracts() []pulseAPIRequestContract {
	return []pulseAPIRequestContract{
		{"summary", "pulse_api GET /api/state/summary", "GET", "/api/state/summary", ""},
		{"connections", "pulse_api GET /api/connections", "GET", "/api/connections", ""},
		{"capabilities", "pulse_api GET /api/agent/resource-capabilities/vm%3A42", "GET", "/api/agent/resource-capabilities/vm%3A42", ""},
		{"plan", "pulse_api POST /api/actions/plan <<'JSON'", "POST", "/api/actions/plan", "heredoc"},
		{"decision", "pulse_api POST /api/actions/act_.../decision <<'JSON'", "POST", "/api/actions/act_.../decision", "heredoc"},
		{"execute", "pulse_api POST /api/actions/act_.../execute <<'JSON'", "POST", "/api/actions/act_.../execute", "heredoc"},
		{"audit", "pulse_api GET '/api/audit/actions?resourceId=vm%3A42&limit=10'", "GET", "/api/audit/actions?resourceId=vm%3A42&limit=10", ""},
		{"events", "pulse_api GET /api/audit/actions/act_.../events", "GET", "/api/audit/actions/act_.../events", ""},
		{"add-node", `pulse_api POST /api/config/nodes < "$node_request_file"`, "POST", "/api/config/nodes", "file"},
	}
}

type pulseAPIRecipe struct {
	contract pulseAPIRequestContract
	command  string
}

func pulseAPIRecipes(doc string) (preparation, helper string, calls []pulseAPIRecipe, err error) {
	contracts := make(map[string]pulseAPIRequestContract)
	for _, contract := range pulseAPIRequestContracts() {
		contracts[contract.firstLine] = contract
	}
	seen := make(map[string]bool)
	for _, match := range regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(doc, -1) {
		block := match[1]
		switch {
		case strings.Contains(block, "api-header") && strings.Contains(block, "vi "):
			if preparation != "" {
				return "", "", nil, fmt.Errorf("duplicate credential preparation recipe")
			}
			preparation = block
		case strings.HasPrefix(block, "pulse_api() ("):
			if helper != "" {
				return "", "", nil, fmt.Errorf("duplicate private-response helper")
			}
			helper = block
		case strings.HasPrefix(block, "pulse_api "):
			firstLine, _, _ := strings.Cut(block, "\n")
			contract, ok := contracts[firstLine]
			if !ok {
				return "", "", nil, fmt.Errorf("request example lacks a transport fixture: %s", firstLine)
			}
			if seen[firstLine] {
				return "", "", nil, fmt.Errorf("duplicate request example: %s", contract.name)
			}
			seen[firstLine] = true
			calls = append(calls, pulseAPIRecipe{contract, block})
		case strings.Contains(block, "pulse_api "):
			return "", "", nil, fmt.Errorf("request example must start with the private-response helper")
		case strings.Contains(block, "curl "):
			return "", "", nil, fmt.Errorf("request bypasses the private-response helper")
		}
	}
	if preparation == "" || helper == "" {
		return "", "", nil, fmt.Errorf("missing credential preparation or private-response helper")
	}
	for _, contract := range pulseAPIRequestContracts() {
		if !seen[contract.firstLine] {
			return "", "", nil, fmt.Errorf("missing request example: %s", contract.name)
		}
	}
	return
}

func pulseAPIReferenceDocument(t *testing.T) string {
	t.Helper()
	doc, err := os.ReadFile(repoFile("docs", "API.md"))
	if err != nil {
		t.Fatal(err)
	}
	mirror, err := os.ReadFile(repoFile("frontend-modern", "public", "docs", "API.md"))
	if err != nil || string(doc) != string(mirror) {
		t.Fatal("shipped API reference differs from the tested recipes")
	}
	return string(doc)
}

func pulseAPIReference(t *testing.T) (string, string, []pulseAPIRecipe) {
	t.Helper()
	prep, helper, calls, err := pulseAPIRecipes(pulseAPIReferenceDocument(t))
	if err != nil {
		t.Fatal(err)
	}
	return prep, helper, calls
}

func TestPulseAPIGuidanceRecipeCatalogueIsCompleteAndOrderIndependent(t *testing.T) {
	doc := pulseAPIReferenceDocument(t)
	prep, helper, calls := pulseAPIReference(t)
	fence := func(block string) string { return "```bash\n" + block + "```\n" }
	reordered := fence(prep) + fence(helper)
	for i := len(calls) - 1; i >= 0; i-- {
		reordered += fence(calls[i].command)
	}
	cases := []struct{ name, doc, wantError string }{
		{"current", doc, ""},
		{"reordered", reordered, ""},
		{"missing preparation", strings.Replace(doc, prep, "", 1), "missing credential preparation"},
		{"missing helper", strings.Replace(doc, helper, "", 1), "missing credential preparation"},
		{"duplicate preparation", doc + fence(prep), "duplicate credential preparation"},
		{"duplicate helper", doc + fence(helper), "duplicate private-response helper"},
		{"unqualified request", doc + fence("pulse_api GET /api/unqualified\n"), "lacks a transport fixture"},
		{"hidden request", doc + fence("# Request\npulse_api GET /api/state/summary\n"), "must start with"},
		{"direct curl", doc + fence("curl --disable http://127.0.0.1:7655/api/state/summary\n"), "bypasses"},
		{"changed method", strings.Replace(doc, "pulse_api GET /api/state/summary\n", "pulse_api PUT /api/state/summary\n", 1), "lacks a transport fixture"},
		{"changed node input", strings.Replace(doc, `pulse_api POST /api/config/nodes < "$node_request_file"`, "pulse_api POST /api/config/nodes", 1), "lacks a transport fixture"},
	}
	for _, call := range calls {
		cases = append(cases,
			struct{ name, doc, wantError string }{"missing " + call.contract.name, strings.Replace(doc, call.command, "", 1), "missing request example"},
			struct{ name, doc, wantError string }{"duplicate " + call.contract.name, doc + fence(call.command), "duplicate request example"})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, got, err := pulseAPIRecipes(tc.doc)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("recipe catalogue error = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil || len(got) != len(pulseAPIRequestContracts()) {
				t.Fatalf("complete catalogue rejected: %v", err)
			}
			for _, call := range got {
				if !strings.HasPrefix(call.command, call.contract.firstLine+"\n") {
					t.Fatal("request was bound to another example's transport contract")
				}
			}
		})
	}
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
				// DELETE and PUT are supported by the existing RBAC recipes.
				// Keep the rejection control outside that reviewed method set.
				call = "pulse_api PATCH /api/actions/act_example"
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
	for _, call := range calls {
		for _, scenario := range []string{"success", "401", "403", "redirect", "partial", "untrusted HTTPS", "trusted HTTPS"} {
			t.Run(call.contract.name+"/"+scenario, func(t *testing.T) {
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
				contentType, body := "", ""
				var nodeInput string
				switch call.contract.input {
				case "heredoc":
					contentType = "application/json"
					_, body, _ = strings.Cut(call.command, "<<'JSON'\n")
					body = strings.TrimSuffix(body, "JSON\n")
				case "file":
					// Fixed, non-credential body bytes test the private-file
					// transport. The node guide's schema/trust specimen and
					// preparation are independently covered by test_node_api_docs.py.
					contentType, body = "application/json", "{\"fixture\":\"node-api-doc-fixture\"}\n"
					file, err := os.CreateTemp(home, "node-request-*.json")
					if err != nil {
						t.Fatal(err)
					}
					info, err := file.Stat()
					if err != nil || info.Mode().Perm() != 0600 {
						_ = file.Close()
						t.Fatal("node input must be private before writing")
					}
					_, writeErr := io.WriteString(file, body)
					closeErr := file.Close()
					if writeErr != nil || closeErr != nil {
						t.Fatal("could not prepare the synthetic node input")
					}
					nodeInput = file.Name()
					extra = append(extra, "node_request_file="+nodeInput)
				}
				if !strings.Contains(scenario, "HTTPS") {
					// The local recipe must not send credentials to an ambient proxy.
					extra = append(extra, "http_proxy=http://127.0.0.1:1", "ALL_PROXY=http://127.0.0.1:1")
				}
				output, exit := runPulseAPIRecipe(t, recipe+"\n"+call.command, home, bin, extra...)
				wantExit := map[string]int{"success": 0, "401": 22, "403": 22, "redirect": 1, "partial": 18, "untrusted HTTPS": 60, "trusted HTTPS": 0}[scenario]
				if exit != wantExit {
					t.Fatalf("request exit %d, want %d", exit, wantExit)
				}
				argv, err := os.ReadFile(capture)
				if err != nil || !strings.HasPrefix(string(argv), "--disable\x00") {
					t.Fatal("curl did not ignore configuration first")
				}
				for _, sensitive := range []string{token, "private-fixture-name", "private-fixture-event", "node-api-doc-fixture"} {
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
				if nodeInput != "" {
					data, readErr := os.ReadFile(nodeInput)
					info, statErr := os.Stat(nodeInput)
					if readErr != nil || statErr != nil || string(data) != body || info.Mode().Perm() != 0600 {
						t.Fatal("request changed its private node input")
					}
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
					if got != (apiRecipeRequest{call.contract.method, call.contract.uri, token, contentType, body}) {
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

func TestPulseAPIGuidanceMissingNodeInputStopsBeforeCurl(t *testing.T) {
	_, helper, calls := pulseAPIReference(t)
	home, bin := t.TempDir(), t.TempDir()
	dir := filepath.Join(home, ".config", "pulse")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api-header"), []byte("X-API-Token: synthetic-header\n"), 0600); err != nil {
		t.Fatal(err)
	}
	called := filepath.Join(home, "curl-called")
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte("#!/bin/sh\nprintf started > \"$API_CALLED\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if call.contract.name != "add-node" {
			continue
		}
		_, exit := runPulseAPIRecipe(t, helper+"\n"+call.command, home, bin,
			"node_request_file="+filepath.Join(home, "absent.json"), "API_CALLED="+called)
		if _, err := os.Stat(called); exit == 0 || !os.IsNotExist(err) {
			t.Fatal("missing private node input reached curl or succeeded")
		}
		responses, err := filepath.Glob(filepath.Join(dir, "api-response.*"))
		if err != nil || len(responses) != 0 {
			t.Fatal("missing node input entered the request helper")
		}
		return
	}
	t.Fatal("node request recipe is missing")
}

func TestPulseAPIGuidanceTimedOutPOSTIsNotRepeated(t *testing.T) {
	_, helper, calls := pulseAPIReference(t)
	var execute string
	for _, call := range calls {
		if call.contract.name == "execute" {
			execute = call.command
		}
	}
	if execute == "" {
		t.Fatal("execution request recipe is missing")
	}
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
	result, err := observeGuidanceRecipe(t, "sleep 12\n"+helper+"\n"+execute, home, bin, guidanceRecipeLimits(20*time.Second))
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
