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
)

// Exercise the actual copied recipes without hardware access, remote SSH or
// privileged cleanup. Network controls use only synthetic guest-local TLS.
func temperatureRecipes(t *testing.T) (local, ssh, download string) {
	t.Helper()
	for _, name := range []string{"TEMPERATURE_MONITORING.md", "TROUBLESHOOTING.md"} {
		doc, err := os.ReadFile(repoFile("docs", name))
		if err != nil {
			t.Fatal(err)
		}
		mirror, err := os.ReadFile(repoFile("frontend-modern", "public", "docs", name))
		if err != nil || string(doc) != string(mirror) {
			t.Fatalf("%s shipped mirror differs from the tested guide", name)
		}
		if name == "TROUBLESHOOTING.md" {
			section := strings.Split(strings.Split(string(doc), "#### Temperature data missing\n")[1], "\n#### ")[0]
			if !strings.Contains(section, "TEMPERATURE_MONITORING.md#check-existing-linux-readings-safely") ||
				strings.Contains(section, "sensors-detect") {
				t.Fatal("troubleshooting must lead to the passive check, not hardware detection")
			}
			continue
		}
		for _, match := range regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(string(doc), -1) {
			block := match[1]
			if strings.Contains(block, "sensors-detect") || strings.Contains(block, "modprobe") {
				t.Fatal("copied diagnostic recipe performs hardware detection or driver loading")
			}
			switch {
			case strings.Contains(block, "5s sensors -j"):
				local = block
			case strings.Contains(block, "ssh -T "):
				ssh = block
			case strings.Contains(block, "curl "):
				download = block
			}
		}
	}
	if local == "" || ssh == "" || download == "" {
		t.Fatal("temperature guide lacks a bounded local, restricted SSH or private download recipe")
	}
	return
}

func TestTemperatureGuidanceKeepsExistingSafetyAndSourceMeaning(t *testing.T) {
	local, ssh, download := temperatureRecipes(t)
	for _, recipe := range []string{local, ssh, download} {
		for _, required := range []string{"set -eu", "umask 077"} {
			if !strings.Contains(recipe, required) {
				t.Fatalf("copied recipe lacks %q", required)
			}
		}
	}
	for _, recipe := range []string{local, ssh} {
		for _, required := range []string{"timeout --signal=TERM --kill-after=2s", "mktemp -d", "|| check_exit=$?", `exit "$check_exit"`} {
			if !strings.Contains(recipe, required) {
				t.Fatalf("reading recipe lacks %q", required)
			}
		}
	}
	for _, required := range []string{"BatchMode=yes", "IdentitiesOnly=yes", "StrictHostKeyChecking=yes", "ConnectionAttempts=1", "/usr/local/sbin/pulse-sensors"} {
		if !strings.Contains(ssh, required) {
			t.Fatalf("SSH recipe lacks %q", required)
		}
	}
	for _, forbidden := range []string{"StrictHostKeyChecking=no", "accept-new", "UserKnownHostsFile=/dev/null", "sensors-detect", "thermal_zone0/temp"} {
		if strings.Contains(ssh, forbidden) {
			t.Fatalf("SSH recipe weakens trust or bypasses the forced wrapper: %q", forbidden)
		}
	}
	for _, required := range []string{"curl --disable --fail ", "--proto '=https'", "--connect-timeout 5 --max-time 60", `[ "$status" = 200 ]`, `mv -n "$download_file" "$helper_file"`, `[ ! -e "$download_file" ]`} {
		if !strings.Contains(download, required) {
			t.Fatalf("helper download lacks %q", required)
		}
	}
	for _, forbidden := range []string{"--insecure", "--location", "--retry", " | ", "bash "} {
		if strings.Contains(download, forbidden) {
			t.Fatalf("download bypasses verification or runs its response: %q", forbidden)
		}
	}
	doc, err := os.ReadFile(repoFile("docs", "TEMPERATURE_MONITORING.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"thermal_zone0` is not necessarily the CPU", "millidegrees", "not physical-disk SMART", "--token-file", "--local-only", "--ssh-known-hosts", "--remove-proxmox-access", "not a signed release asset", "not an empty reading"} {
		if !strings.Contains(string(doc), text) {
			t.Fatalf("temperature guide loses existing scope or source distinction %q", text)
		}
	}
}

func TestTemperatureGuidanceReadingCommandsArePrivateAndBounded(t *testing.T) {
	local, ssh, _ := temperatureRecipes(t)
	for index, recipe := range []string{local, ssh} {
		for _, scenario := range []string{"success", "partial failure", "hung command", "temporary directory failure", "missing timeout"} {
			t.Run(fmt.Sprintf("%d/%s", index, scenario), func(t *testing.T) {
				home, bin, tmp := t.TempDir(), t.TempDir(), t.TempDir()
				tool := "sensors"
				outName, errName := "sensors.json", "sensors.err"
				if index == 1 {
					tool, outName, errName = "ssh", "wrapper.json", "ssh.err"
				}
				capture := filepath.Join(home, "argv")
				const out = "synthetic-private-sensor-disk-identity"
				const diagnostic = "synthetic-private-tool-error"
				stub := "#!/bin/sh\nprintf 'call\\n' >> \"$TEMP_ARGV\"\nprintf '%s\\n' \"$@\" >> \"$TEMP_ARGV\"\nprintf '" + out + "'\nprintf '" + diagnostic + "' >&2\n"
				wantExit := 0
				switch scenario {
				case "partial failure":
					wantExit = 2
					stub += "exit 2\n"
				case "hung command":
					// The real GNU timeout must enforce its TERM/KILL bound; the
					// stub replaces itself, so no orphan child is manufactured.
					stub += "trap '' TERM\nexec sleep 40\n"
					wantExit = 137
				case "temporary directory failure":
					wantExit = 42
					if err := os.WriteFile(filepath.Join(bin, "mktemp"), []byte("#!/bin/sh\nexit 42\n"), 0700); err != nil {
						t.Fatal(err)
					}
				case "missing timeout":
					wantExit = 1
				}
				if err := os.WriteFile(filepath.Join(bin, tool), []byte(stub), 0700); err != nil {
					t.Fatal(err)
				}
				env := []string{"TEMP_ARGV=" + capture, "TMPDIR=" + tmp}
				if scenario == "missing timeout" {
					env = append(env, "PATH="+bin)
				}
				output, exit := runPBSRecipe(t, recipe, home, bin, env...)
				if exit != wantExit {
					t.Fatalf("reading recipe exit %d, want %d: %s", exit, wantExit, output)
				}
				if strings.Contains(output, out) || strings.Contains(output, diagnostic) {
					t.Fatal("reading recipe disclosed private output")
				}
				argv, err := os.ReadFile(capture)
				if scenario == "missing timeout" || scenario == "temporary directory failure" {
					if !os.IsNotExist(err) {
						t.Fatal("failed preparation reached the reading tool")
					}
					return
				}
				if err != nil || strings.Count(string(argv), "call\n") != 1 {
					t.Fatal("reading tool was not called exactly once")
				}
				if index == 0 && string(argv) != "call\n-j\n" {
					t.Fatal("local check did not perform only the existing sensors JSON read")
				}
				if index == 1 {
					want := "call\n-T\n-i\n/path/to/pulse-key\n-o\nBatchMode=yes\n-o\nIdentitiesOnly=yes\n-o\nStrictHostKeyChecking=yes\n-o\nConnectTimeout=5\n-o\nConnectionAttempts=1\n-o\nServerAliveInterval=5\n-o\nServerAliveCountMax=1\nroot@node\n/usr/local/sbin/pulse-sensors\n"
					if string(argv) != want {
						t.Fatal("SSH read changed the key, trust, bounded transport or forced wrapper arguments")
					}
				}
				dirs, err := filepath.Glob(filepath.Join(tmp, "pulse-temperature*"))
				if err != nil || len(dirs) != 1 {
					t.Fatal("reading did not create one private result directory")
				}
				for name, want := range map[string]string{outName: out, errName: diagnostic} {
					path := filepath.Join(dirs[0], name)
					content, err := os.ReadFile(path)
					info, statErr := os.Stat(path)
					if err != nil || string(content) != want || statErr != nil || info.Mode().Perm() != 0600 {
						t.Fatal("reading output was not retained completely and privately")
					}
				}
				info, err := os.Stat(dirs[0])
				if err != nil || info.Mode().Perm() != 0700 {
					t.Fatal("result directory is not owner-only")
				}
			})
		}
	}
}

func TestTemperatureGuidanceCleanupDownloadPreservesFilesAndTrust(t *testing.T) {
	_, _, original := temperatureRecipes(t)
	for _, scenario := range []string{"success", "401", "403", "redirect", "wrong success status", "untrusted HTTPS", "truncated response", "existing helper", "helper symlink", "dangling helper", "helper directory", "configuration symlink", "pulse directory symlink", "mkdir failure", "chmod failure"} {
		t.Run(scenario, func(t *testing.T) {
			realCurl, err := exec.LookPath("curl")
			if err != nil {
				t.Fatal("curl is required to verify copied download guidance")
			}
			home, bin := t.TempDir(), t.TempDir()
			dir := filepath.Join(home, ".config", "pulse")
			helper := filepath.Join(dir, "sensor-proxy-uninstall.sh")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			const preserved = "original-helper-or-link-target"
			target := filepath.Join(home, "untouched")
			if err := os.WriteFile(target, []byte(preserved), 0600); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "existing helper":
				if err := os.WriteFile(helper, []byte(preserved), 0600); err != nil {
					t.Fatal(err)
				}
			case "helper symlink", "dangling helper":
				linkTarget := target
				if scenario == "dangling helper" {
					linkTarget += "-missing"
				}
				if err := os.Symlink(linkTarget, helper); err != nil {
					t.Fatal(err)
				}
			case "helper directory":
				if err := os.Mkdir(helper, 0700); err != nil {
					t.Fatal(err)
				}
			case "configuration symlink", "pulse directory symlink":
				// Empty fixture directories only; never remove source or data.
				if err := os.Remove(dir); err != nil {
					t.Fatal(err)
				}
				link := dir
				if scenario == "configuration symlink" {
					link = filepath.Dir(dir)
					if err := os.Remove(link); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(home, link); err != nil {
					t.Fatal(err)
				}
			case "mkdir failure", "chmod failure":
				tool := strings.Fields(scenario)[0]
				if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\nexit 42\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			trace, argv := filepath.Join(home, "must-not-trace"), filepath.Join(home, "argv")
			if err := os.WriteFile(filepath.Join(home, ".curlrc"), []byte("insecure\nlocation\ntrace = "+trace+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			stub := "#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$TEMP_ARGV\"\nexec \"$TEMP_CURL\" \"$@\"\n"
			if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(stub), 0700); err != nil {
				t.Fatal(err)
			}
			requests := make(chan string, 4)
			const body = "synthetic-private-response-not-a-cleanup-command"
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Method + " " + r.URL.Path
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
				case "truncated response":
					w.Header().Set("Content-Length", "1024")
				}
				w.WriteHeader(code)
				fmt.Fprint(w, body)
			}))
			server.Config.ErrorLog = log.New(io.Discard, "", 0)
			server.StartTLS()
			t.Cleanup(server.Close)
			recipe := strings.Replace(original, "https://raw.githubusercontent.com", server.URL, 1)
			if scenario != "untrusted HTTPS" {
				ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
				if err := os.WriteFile(filepath.Join(home, "ca.pem"), ca, 0600); err != nil {
					t.Fatal(err)
				}
				recipe = strings.Replace(recipe, "curl --disable ", `curl --disable --cacert "$HOME/ca.pem" `, 1)
			}
			output, exit := runPBSRecipe(t, recipe, home, bin, "TEMP_ARGV="+argv, "TEMP_CURL="+realCurl)
			wantExit := 1
			switch scenario {
			case "success":
				wantExit = 0
			case "401", "403":
				wantExit = 22
			case "untrusted HTTPS":
				wantExit = 60
			case "truncated response":
				wantExit = 18
			case "mkdir failure", "chmod failure":
				wantExit = 42
			}
			if exit != wantExit || strings.Contains(output, body) {
				t.Fatalf("download exit %d, want %d, or response was disclosed: %s", exit, wantExit, output)
			}
			if _, err := os.Stat(trace); !os.IsNotExist(err) {
				t.Fatal("ambient curl configuration enabled tracing")
			}
			content, err := os.ReadFile(target)
			if err != nil || string(content) != preserved {
				t.Fatal("preparation overwrote a link target")
			}
			args, argErr := os.ReadFile(argv)
			localFailure := strings.Contains(scenario, "helper") || strings.Contains(scenario, "symlink") || strings.HasSuffix(scenario, "failure")
			if localFailure {
				if len(requests) != 0 || !os.IsNotExist(argErr) {
					t.Fatal("unsafe preparation reached the downloader")
				}
				if scenario == "existing helper" {
					content, err := os.ReadFile(helper)
					if err != nil || string(content) != preserved {
						t.Fatal("refused download overwrote an existing helper")
					}
				}
				return
			}
			if argErr != nil || !strings.HasPrefix(string(args), "--disable\x00") {
				t.Fatal("curl did not ignore configuration before other options")
			}
			if scenario == "untrusted HTTPS" {
				if len(requests) != 0 {
					t.Fatal("untrusted peer received a request")
				}
			} else if len(requests) != 1 || <-requests != "GET /rcourtman/Pulse/main/scripts/uninstall-sensor-proxy.sh" {
				t.Fatal("download was missing, redirected, retried or used the wrong path")
			}
			content, err = os.ReadFile(helper)
			if scenario == "success" {
				info, statErr := os.Stat(helper)
				if err != nil || string(content) != body || statErr != nil || info.Mode().Perm() != 0600 {
					t.Fatal("complete helper was not promoted privately")
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("failed, redirected or partial download occupied the runnable helper path")
			}
		})
	}
}
