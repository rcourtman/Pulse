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

// Exercise the copied recipes against a synthetic TLS peer, not a customer's
// management API. This proves curl transport/credential handling, not Pulse's
// tenant authorisation or an installed provider deployment.
func mspValidationRecipes(t *testing.T) []string {
	t.Helper()
	doc, err := os.ReadFile(repoFile("docs", "MSP.md"))
	if err != nil {
		t.Fatal(err)
	}
	const heading = "### Validation checklist (run after setup, repeat after network changes)\n"
	_, section, found := strings.Cut(string(doc), heading)
	if !found {
		t.Fatal("MSP validation checklist is missing")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	blocks := regexp.MustCompile("(?s)```bash\n(.*?)```").FindAllStringSubmatch(section, -1)
	var recipes []string
	for _, block := range blocks {
		// A continuation is one command; test each command separately so the
		// second public probe cannot mask the first probe's failed exit.
		joined := regexp.MustCompile(`\\\n[ \t]*`).ReplaceAllString(block[1], " ")
		for _, line := range strings.Split(joined, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if err := mspRecipeSafety(line); err != nil {
				t.Fatalf("unsafe copied MSP recipe: %v", err)
			}
			recipes = append(recipes, line)
		}
	}
	if len(recipes) != 5 {
		t.Fatalf("want five HTTP checks, got %d", len(recipes))
	}
	return recipes
}

func mspRecipeSafety(recipe string) error {
	for _, required := range []string{
		"curl --disable ", "--silent --show-error", "--proto '=https'",
		"--connect-timeout 5", "--max-time 10", "--output /dev/null",
		"--write-out '%{http_code}\\n'", "https://",
	} {
		if !strings.Contains(recipe, required) {
			return fmt.Errorf("missing %s", required)
		}
	}
	for _, forbidden := range []string{
		"--insecure", "--location", "--config", "--trace", "--verbose",
		"--fail", "X-API-Token:", "Authorization:", "--data", "--request",
	} {
		if strings.Contains(recipe, forbidden) {
			return fmt.Errorf("forbidden %s", forbidden)
		}
	}
	if regexp.MustCompile(`(?:^|\s)-[A-Za-z]`).MatchString(recipe) ||
		strings.ContainsAny(recipe, "|;`") || strings.Contains(recipe, "$(") {
		return fmt.Errorf("unexpected short flag, pipeline or shell expansion")
	}
	if strings.Contains(recipe, "pulse.internal") &&
		!strings.Contains(recipe, `--header "@$HOME/.config/pulse/`) {
		return fmt.Errorf("authenticated request lacks private header file")
	}
	return nil
}

func TestMSPDiagnosticGuidanceRejectsUnsafeRecipes(t *testing.T) {
	recipe := mspValidationRecipes(t)[2]
	for name, unsafe := range map[string]string{
		"curl config":  strings.Replace(recipe, "--disable ", "", 1),
		"TLS bypass":   strings.Replace(recipe, "--disable ", "--disable --insecure ", 1),
		"short bypass": strings.Replace(recipe, "--disable ", "--disable -k ", 1),
		"redirect":     strings.Replace(recipe, "--disable ", "--disable --location ", 1),
		"trace":        strings.Replace(recipe, "--disable ", "--disable --trace /tmp/trace ", 1),
		"inline token": strings.Replace(recipe, `"@$HOME/.config/pulse/agent-header"`,
			`"X-API-Token: synthetic-inline-secret"`, 1),
		"missing deadline":      strings.Replace(recipe, "--max-time 10 ", "", 1),
		"missing connect bound": strings.Replace(recipe, "--connect-timeout 5 ", "", 1),
		"body disclosure":       strings.Replace(recipe, "--output /dev/null ", "", 1),
		"HTTP":                  strings.Replace(recipe, "https://", "http://", 1),
		"pipe hides exit":       recipe + " | cat",
	} {
		t.Run(name, func(t *testing.T) {
			if mspRecipeSafety(unsafe) == nil {
				t.Fatal("unsafe mutation was accepted")
			}
		})
	}
}

type mspProbeRequest struct {
	method, path, token, org string
}

type mspProbeLab struct {
	home, ca, capture, trace, curl string
	server                         *httptest.Server
	requests                       chan mspProbeRequest
}

func newMSPProbeLab(t *testing.T, redirect bool) *mspProbeLab {
	t.Helper()
	realCurl, err := exec.LookPath("curl")
	if err != nil {
		t.Fatal("curl is required to verify copied MSP diagnostics")
	}
	lab := &mspProbeLab{home: t.TempDir(), curl: realCurl, requests: make(chan mspProbeRequest, 8)}
	lab.ca = filepath.Join(lab.home, "trusted-ca.pem")
	lab.capture = filepath.Join(lab.home, "argv")
	lab.trace = filepath.Join(lab.home, "forbidden-trace")
	lab.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lab.requests <- mspProbeRequest{r.Method, r.URL.Path, r.Header.Get("X-API-Token"), r.Header.Get("X-Pulse-Org-ID")}
		if redirect {
			http.Redirect(w, r, lab.server.URL+"/must-not-follow", http.StatusFound)
			return
		}
		code := http.StatusForbidden
		if r.URL.Path == "/" || r.URL.Path == "/api/login" {
			code = http.StatusNotFound
		}
		w.WriteHeader(code)
		// Even an upstream body echoing credentials must be discarded.
		fmt.Fprint(w, "synthetic-agent-secret synthetic-org-secret private-body")
	}))
	lab.server.Config.ErrorLog = log.New(io.Discard, "", 0)
	lab.server.StartTLS()
	t.Cleanup(lab.server.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: lab.server.Certificate().Raw})
	if err := os.WriteFile(lab.ca, cert, 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(lab.home, ".config", "pulse")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{"agent-header": "synthetic-agent-secret", "org-a-header": "synthetic-org-secret"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("X-API-Token: "+token+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// --disable must ignore settings that would disclose headers, bypass TLS
	// or follow redirects, even on a user's otherwise configured curl.
	if err := os.WriteFile(filepath.Join(lab.home, ".curlrc"), []byte("insecure\nlocation\ntrace = "+lab.trace+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(lab.home, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	wrapper := "#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$MSP_ARGV\"\nexec \"$MSP_CURL\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	return lab
}

func (lab *mspProbeLab) run(t *testing.T, recipe string, trustCA bool) (string, int) {
	t.Helper()
	// Replace only documented endpoint placeholders; the CA option is the
	// guide's explicit private-CA alternative. Credentials never enter argv.
	recipe = strings.ReplaceAll(recipe, "https://agents.example.com:7656", lab.server.URL)
	recipe = strings.ReplaceAll(recipe, "https://pulse.internal:7655", lab.server.URL)
	if trustCA {
		recipe = strings.Replace(recipe, "curl --disable ", `curl --disable --cacert "$MSP_CA" `, 1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", "-c", recipe)
	command.Env = append(os.Environ(), "HOME="+lab.home, "PATH="+filepath.Join(lab.home, "bin")+":"+os.Getenv("PATH"),
		"MSP_ARGV="+lab.capture, "MSP_CURL="+lab.curl, "MSP_CA="+lab.ca,
		"https_proxy=", "HTTPS_PROXY=", "http_proxy=", "HTTP_PROXY=", "ALL_PROXY=", "all_proxy=")
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("copied recipe exceeded test deadline: %v", ctx.Err())
	}
	exit := 0
	if err != nil {
		failure, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		exit = failure.ExitCode()
	}
	argv, err := os.ReadFile(lab.capture)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"synthetic-agent-secret", "synthetic-org-secret", "private-body"} {
		if strings.Contains(string(argv), secret) || strings.Contains(string(output), secret) {
			t.Fatal("copied recipe disclosed synthetic private data")
		}
	}
	if _, err := os.Stat(lab.trace); !os.IsNotExist(err) {
		t.Fatal("curl local configuration enabled a trace")
	}
	return string(output), exit
}

func TestMSPDiagnosticGuidanceCopiedChecksUsePrivateHeadersAndTrustedTLS(t *testing.T) {
	lab := newMSPProbeLab(t, false)
	want := []mspProbeRequest{
		{"GET", "/", "", ""}, {"GET", "/api/login", "", ""},
		{"GET", "/api/notifications/webhooks", "synthetic-agent-secret", ""},
		{"GET", "/api/alerts/active", "synthetic-org-secret", "org-b"},
		{"GET", "/api/alerts/active", "synthetic-org-secret", "default"},
	}
	for i, recipe := range mspValidationRecipes(t) {
		t.Run(fmt.Sprint(i+1), func(t *testing.T) {
			output, exit := lab.run(t, recipe, true)
			expected := "403\n"
			if i < 2 {
				expected = "404\n"
			}
			if exit != 0 || output != expected {
				t.Fatalf("curl result = %q, exit %d; want %q, exit 0", output, exit, expected)
			}
			select {
			case got := <-lab.requests:
				if got != want[i] {
					t.Fatal("request method, path or private header did not match the documented check")
				}
			default:
				t.Fatal("copied check sent no request")
			}
		})
	}
}

func TestMSPDiagnosticGuidanceUntrustedTLSRemainsFailure(t *testing.T) {
	lab := newMSPProbeLab(t, false)
	output, exit := lab.run(t, mspValidationRecipes(t)[2], false)
	if exit != 60 || !strings.HasSuffix(output, "000\n") {
		t.Fatalf("want certificate failure/000, got exit %d, %q", exit, output)
	}
	if len(lab.requests) != 0 {
		t.Fatal("untrusted peer received an authenticated HTTP request")
	}
}

func TestMSPDiagnosticGuidanceDoesNotFollowRedirects(t *testing.T) {
	lab := newMSPProbeLab(t, true)
	output, exit := lab.run(t, mspValidationRecipes(t)[2], true)
	if exit != 0 || output != "302\n" || len(lab.requests) != 1 {
		t.Fatalf("redirect must remain an unexpected status, not a followed check: %q, exit %d", output, exit)
	}
}
