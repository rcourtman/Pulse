//go:build !windows

package installtests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// Run the complete shipped entrypoint, real curl and real OpenSSL. Only platform
// discovery and service mutations are replaced: a preflight must not reach them.
type pinPreflight struct {
	t                  *testing.T
	root, bin, openssl string
	state              map[string]string
}

func newPinPreflight(t *testing.T) *pinPreflight {
	t.Helper()
	openssl, err := exec.LookPath("openssl")
	if err != nil {
		t.Fatal("native Unix pin checks require openssl: ", err)
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Fatal("native Unix pin checks require curl: ", err)
	}
	f := &pinPreflight{t: t, root: t.TempDir(), openssl: openssl,
		state: map[string]string{"agent-id": "unchanged-agent\n", "runtime.token": strings.Repeat("a", 64),
			"connection.env": "# existing recovery material\n"}}
	f.bin = filepath.Join(f.root, "bin")
	if err := os.Mkdir(f.bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.root, "tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.root, "state"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, value := range f.state {
		if err := os.WriteFile(filepath.Join(f.root, "state", name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f.writeTool("uname", "case \"$1\" in -s) echo Darwin;; -m) echo x86_64;; *) exit 97;; esac\n")
	f.writeTool("openssl", "if [ \"$1\" = s_client ]; then printf '%s' \"$$\" > \"$FIXTURE/probe-pid\"; fi\nexec "+strconv.Quote(openssl)+" \"$@\"\n")
	f.writeTool("systemctl", "[ \"$*\" = 'is-active --quiet pulse-agent' ] && exit 3\nprintf mutation >> \"$FIXTURE/mutations\"\nexit 97\n")
	for _, tool := range []string{"sudo", "launchctl", "service", "rc-service", "install", "chown"} {
		f.writeTool(tool, "printf mutation >> \"$FIXTURE/mutations\"\nexit 97\n")
	}
	return f
}

func (f *pinPreflight) writeTool(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.bin, name), []byte("#!/bin/sh\n"+body), 0700); err != nil {
		f.t.Fatal(err)
	}
}

func (f *pinPreflight) command(url, pin string, extra ...string) *exec.Cmd {
	args := []string{repoFile("scripts", "install.sh"), "--url", url,
		"--preflight-only", "--output", "json", "--non-interactive",
		"--disable-docker", "--disable-kubernetes", "--disable-proxmox",
		"--state-dir", filepath.Join(f.root, "state")}
	if pin != "" {
		args = append(args, "--server-fingerprint", pin)
	}
	args = append(args, extra...)
	cmd := exec.Command("bash", args...)
	cmd.Env = []string{"PATH=" + f.bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + f.root, "TMPDIR=" + filepath.Join(f.root, "tmp"), "FIXTURE=" + f.root,
		"LC_ALL=C", "TERM=dumb"}
	return cmd
}

func (f *pinPreflight) preserved() {
	f.t.Helper()
	if _, err := os.Stat(filepath.Join(f.root, "mutations")); !os.IsNotExist(err) {
		f.t.Fatal("preflight reached a service or installed-file mutation")
	}
	files, err := os.ReadDir(filepath.Join(f.root, "state"))
	if err != nil || len(files) != len(f.state) {
		f.t.Fatal("preflight changed the recovery inventory: ", files, err)
	}
	for name, want := range f.state {
		path := filepath.Join(f.root, "state", name)
		got, err := os.ReadFile(path)
		info, statErr := os.Stat(path)
		if err != nil || string(got) != want || statErr != nil || info.Mode().Perm() != 0600 {
			f.t.Fatalf("recovery material changed: %s", name)
		}
	}
	files, err = os.ReadDir(filepath.Join(f.root, "tmp"))
	if err != nil || len(files) != 0 {
		f.t.Fatalf("certificate-probe temporary files survived: %v %v", files, err)
	}
	if raw, err := os.ReadFile(filepath.Join(f.root, "probe-pid")); err == nil {
		pid, err := strconv.Atoi(string(raw))
		if err != nil || syscall.Kill(pid, 0) != syscall.ESRCH {
			f.t.Fatalf("certificate-probe child not reaped: %q %v", raw, err)
		}
	}
}

func pinExit(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

// Closing the peer at the outer test deadline is fixture cleanup, never a
// successful installer deadline. It also lets the old unbounded code terminate
// so an adverse baseline cannot leave a TLS child or fixture connection behind.
func silentPinPeer(t *testing.T) (string, <-chan struct{}, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan struct{})
	finished := make(chan struct{})
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(finished)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		<-stop
	}()
	closePeer := func() {
		once.Do(func() { close(stop); listener.Close(); <-finished })
	}
	t.Cleanup(closePeer)
	return "https://" + listener.Addr().String(), accepted, closePeer
}

func TestInstallSHPinSilentHandshakeHasDeadline(t *testing.T) {
	f := newPinPreflight(t)
	url, _, closePeer := silentPinPeer(t)
	cmd := f.command(url, strings.Repeat("a", 64))
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
	defer cancel()
	go func() { <-ctx.Done(); closePeer() }()
	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	if ctx.Err() != nil {
		t.Fatalf("installer did not bound the silent TLS peer; fixture closed it after %s: exit %d\n%s", elapsed, pinExit(err), out)
	}
	if pinExit(err) != 14 || !strings.Contains(string(out), "Timed out verifying Pulse server certificate") || elapsed < 9*time.Second || elapsed > 13*time.Second {
		t.Fatalf("wrong pin timeout: elapsed %s exit %d\n%s", elapsed, pinExit(err), out)
	}
	f.preserved()
	t.Logf("real silent TLS peer: exit 14 after %s; child reaped, no mutations or temporary files", elapsed)
}

func TestInstallSHPinTrustAndPreflight(t *testing.T) {
	var reads atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		switch r.URL.Path {
		case "/api/health":
			io.WriteString(w, `{"status":"ok"}`)
		case "/download/pulse-agent":
			if r.Method != http.MethodHead || r.URL.Query().Get("arch") != "darwin-amd64" {
				t.Errorf("unexpected preflight download: %s %s", r.Method, r.URL)
			}
			w.Header().Set("X-Checksum-Sha256", strings.Repeat("a", 64))
		default:
			t.Errorf("unexpected request: %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	hash := sha256.Sum256(server.Certificate().Raw)
	pin := hex.EncodeToString(hash[:])
	for _, tc := range []struct {
		name, pin string
		extra     []string
		want      int
		reads     int32
		message   string
	}{
		{"exact", pin, nil, 0, 2, "Preflight checks passed"},
		{"normalized", strings.ToUpper(strings.Join(strings.Split(pin, ""), ":")), nil, 0, 2, "Preflight checks passed"},
		{"mismatch", strings.Repeat("0", 64), nil, 14, 0, "certificate fingerprint mismatch"},
		{"insecure_still_requires_pin", strings.Repeat("0", 64), []string{"--insecure"}, 14, 0, "certificate fingerprint mismatch"},
		{"malformed", "not-a-pin", nil, 14, 0, "Invalid --server-fingerprint"},
		{"no_pin_still_verifies_tls", "", nil, 14, 0, "Preflight checks failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPinPreflight(t)
			before := reads.Load()
			out, err := f.command(server.URL, tc.pin, tc.extra...).CombinedOutput()
			if pinExit(err) != tc.want || !strings.Contains(string(out), tc.message) || reads.Load()-before != tc.reads {
				t.Fatalf("wrong pin result: exit %d, requests %d\n%s", pinExit(err), reads.Load()-before, out)
			}
			if tc.want == 0 {
				var result map[string]any
				for _, line := range strings.Split(string(out), "\n") {
					if strings.HasPrefix(line, "{") {
						if err := json.Unmarshal([]byte(line), &result); err != nil {
							t.Fatal(err)
						}
					}
				}
				if result["code"] != "ok" || result["exitCode"] != float64(0) {
					t.Fatalf("preflight did not finish: %v", result)
				}
			}
			f.preserved()
		})
	}
}

func TestInstallSHPinInterruptedProbeIsReaped(t *testing.T) {
	for _, signal := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(fmt.Sprint(signal), func(t *testing.T) {
			f := newPinPreflight(t)
			url, accepted, closePeer := silentPinPeer(t)
			cmd := f.command(url, strings.Repeat("a", 64))
			var output strings.Builder
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-accepted:
			case <-time.After(5 * time.Second):
				cmd.Process.Kill()
				closePeer()
				cmd.Wait()
				t.Fatal("certificate probe did not connect")
			}
			if err := cmd.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				if pinExit(err) != 128+int(signal) {
					t.Fatalf("wrong interruption exit %d\n%s", pinExit(err), &output)
				}
			case <-time.After(3 * time.Second):
				cmd.Process.Kill()
				closePeer()
				<-done
				t.Fatal("interruption did not stop the certificate probe")
			}
			f.preserved()
		})
	}
}

func TestInstallSHPinUnreadableCertificateFailsClosed(t *testing.T) {
	for _, body := range []string{
		// A completed failed TLS command and a successful command without a
		// certificate must both become the normal input-free preflight error.
		"exit 37\n",
		"if [ \"$1\" = s_client ]; then printf 'not a certificate'; exit 0; fi\nexec \"$REAL_OPENSSL\" \"$@\"\n",
	} {
		f := newPinPreflight(t)
		f.writeTool("openssl", body)
		cmd := f.command("https://127.0.0.1:1", strings.Repeat("a", 64))
		cmd.Env = append(cmd.Env, "REAL_OPENSSL="+f.openssl)
		out, err := cmd.CombinedOutput()
		if pinExit(err) != 14 || !strings.Contains(string(out), "Could not read Pulse server certificate") || strings.Contains(string(out), "not a certificate") {
			t.Fatalf("unreadable certificate did not fail closed: exit %d\n%s", pinExit(err), out)
		}
		f.preserved()
	}
}
