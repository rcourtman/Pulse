package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/declarativeconfig"
)

func newDeclarativeTestEnv(baseURL, token string, extra map[string]string) func(string) string {
	env := map[string]string{
		"PULSE_API_URL":   baseURL,
		"PULSE_API_TOKEN": token,
	}
	for k, v := range extra {
		env[k] = v
	}
	return func(key string) string {
		return env[key]
	}
}

func assertNoDisallowedCopyPunctuation(t *testing.T, label, text string) {
	t.Helper()
	if strings.Contains(text, "—") {
		t.Errorf("%s contains disallowed em-dash: %q", label, text)
	}
	if strings.Contains(text, ";") {
		t.Errorf("%s contains disallowed semicolon: %q", label, text)
	}
}

func TestConfigPullYAMLJSONAndOutputFile(t *testing.T) {
	const yamlBody = "apiVersion: pulse.monitoring/v1\nkind: MonitoringConfig\n"
	const jsonBody = `{"apiVersion":"pulse.monitoring/v1","kind":"MonitoringConfig"}`

	var gotFormats []string
	var gotSurface string
	var gotToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/config/declarative" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		gotSurface = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
		gotToken = r.Header.Get(agentcapabilities.AgentAPITokenHeader)
		format := r.URL.Query().Get("format")
		gotFormats = append(gotFormats, format)
		if format == "json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, jsonBody)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = io.WriteString(w, yamlBody)
	}))
	defer server.Close()

	runPull := func(args ...string) (string, string, error) {
		var stdout, stderr bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_secret_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(append([]string{"config", "pull"}, args...))
		err := cmd.ExecuteContext(context.Background())
		return stdout.String(), stderr.String(), err
	}

	t.Run("default yaml stdout", func(t *testing.T) {
		stdout, stderr, err := runPull()
		if err != nil {
			t.Fatalf("config pull failed: %v", err)
		}
		if stdout != yamlBody {
			t.Fatalf("stdout = %q, want %q", stdout, yamlBody)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		if gotSurface != agentcapabilities.AgentSurfacePulseCLI {
			t.Fatalf("X-Pulse-Agent-Surface = %q, want %q", gotSurface, agentcapabilities.AgentSurfacePulseCLI)
		}
		if gotToken != "tok_secret_123" {
			t.Fatalf("X-API-Token = %q, want tok_secret_123", gotToken)
		}
	})

	t.Run("json via --format json", func(t *testing.T) {
		stdout, _, err := runPull("--format", "json")
		if err != nil {
			t.Fatalf("config pull --format json failed: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("stdout not valid json: %v\n%s", err, stdout)
		}
		if doc["apiVersion"] != declarativeconfig.APIVersion {
			t.Fatalf("apiVersion = %v, want %s", doc["apiVersion"], declarativeconfig.APIVersion)
		}
	})

	t.Run("json via global --json flag", func(t *testing.T) {
		stdout, _, err := runPull("--json")
		if err != nil {
			t.Fatalf("config pull --json failed: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("stdout not valid json: %v\n%s", err, stdout)
		}
		if doc["kind"] != declarativeconfig.Kind {
			t.Fatalf("kind = %v, want %s", doc["kind"], declarativeconfig.Kind)
		}
	})

	t.Run("output file with 0600 permissions", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "monitoring.yaml")
		stdout, stderr, err := runPull("--output", outPath)
		if err != nil {
			t.Fatalf("config pull --output failed: %v", err)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout when --output is used, got %q", stdout)
		}
		if !strings.Contains(stderr, "Wrote declarative configuration to "+outPath) {
			t.Fatalf("stderr missing confirmation: %q", stderr)
		}
		assertNoDisallowedCopyPunctuation(t, "pull stderr", stderr)

		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read output file: %v", err)
		}
		if string(data) != yamlBody {
			t.Fatalf("output file = %q, want %q", string(data), yamlBody)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(outPath)
			if err != nil {
				t.Fatalf("stat output file: %v", err)
			}
			if perms := info.Mode().Perm(); perms != 0o600 {
				t.Fatalf("output file permissions = %04o, want 0600", perms)
			}
		}
	})

	t.Run("invalid format rejected", func(t *testing.T) {
		_, _, err := runPull("--format", "toml")
		if err == nil || !strings.Contains(err.Error(), "must be yaml or json") {
			t.Fatalf("expected format error, got %v", err)
		}
		assertNoDisallowedCopyPunctuation(t, "invalid format error", err.Error())
	})

	if len(gotFormats) != 4 || gotFormats[0] != "yaml" || gotFormats[1] != "json" || gotFormats[2] != "json" || gotFormats[3] != "yaml" {
		t.Fatalf("observed format query values = %v", gotFormats)
	}
}

func TestConfigDiffChangesNoChangesEnvResolutionAndMissingEnv(t *testing.T) {
	const resolvedSecret = "super-secret-smtp-password-xyz"

	var receivedReq map[string]any
	var planCalls int
	planResp := declarativeconfig.PlanResult{
		APIVersion: declarativeconfig.APIVersion,
		Kind:       declarativeconfig.Kind,
		PlanHash:   "sha256:plan123",
		Prune:      true,
		Summary: declarativeconfig.PlanSummary{
			Creates:    1,
			Updates:    1,
			Deletes:    0,
			Unchanged:  2,
			HasChanges: true,
		},
		Changes: []declarativeconfig.ConfigChange{
			{
				Domain:  "notifications",
				Action:  "update",
				Key:     "email",
				Summary: "Update email notification settings",
			},
			{
				Domain:  "availabilityTargets",
				Action:  "create",
				Key:     "api-health",
				Summary: "Create availability target api-health",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		planCalls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/config/declarative/plan" {
			http.Error(w, "unexpected route", http.StatusNotFound)
			return
		}
		if r.Header.Get(agentcapabilities.AgentSurfaceHeader) != agentcapabilities.AgentSurfacePulseCLI {
			t.Errorf("missing pulse_cli surface header")
		}
		body, _ := io.ReadAll(r.Body)
		receivedReq = nil
		_ = json.Unmarshal(body, &receivedReq)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(planResp)
	}))
	defer server.Close()

	docYAML := `apiVersion: pulse.monitoring/v1
kind: MonitoringConfig
notifications:
  email:
    enabled: true
    server: smtp.example.com
    port: 587
    password:
      secretRef: env:SMTP_PASS
`
	filePath := filepath.Join(t.TempDir(), "monitoring.yaml")
	if err := os.WriteFile(filePath, []byte(docYAML), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Run("changes human table and local env secret resolution", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", map[string]string{"SMTP_PASS": resolvedSecret}),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"config", "diff", "-f", filePath, "--prune"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("config diff failed: %v", err)
		}

		out := stdout.String()
		for _, want := range []string{"ACTION", "DOMAIN", "KEY", "SUMMARY", "api-health", "Summary: 1 create, 1 update, 0 delete, 2 unchanged", "Plan hash: sha256:plan123"} {
			if !strings.Contains(out, want) {
				t.Errorf("diff output missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, resolvedSecret) || strings.Contains(stderr.String(), resolvedSecret) {
			t.Fatalf("resolved secret leaked into CLI output")
		}
		assertNoDisallowedCopyPunctuation(t, "diff human output", out)

		if prune, _ := receivedReq["prune"].(bool); !prune {
			t.Fatalf("expected prune=true in request body, got %#v", receivedReq)
		}
		docMap, _ := receivedReq["document"].(map[string]any)
		notifs, _ := docMap["notifications"].(map[string]any)
		email, _ := notifs["email"].(map[string]any)
		pw, _ := email["password"].(map[string]any)
		if pw["value"] != resolvedSecret {
			t.Fatalf("expected resolved secret value in request payload, got %#v", pw)
		}
		if ref, _ := pw["secretRef"].(string); ref != "" {
			t.Fatalf("expected secretRef cleared after env resolution, got %q", ref)
		}
	})

	t.Run("json mode emits raw PlanResult", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", map[string]string{"SMTP_PASS": resolvedSecret}),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "diff", "-f", filePath, "--json"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("config diff --json failed: %v", err)
		}
		var parsed declarativeconfig.PlanResult
		if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
			t.Fatalf("invalid json output: %v\n%s", err, stdout.String())
		}
		if parsed.PlanHash != "sha256:plan123" || len(parsed.Changes) != 2 {
			t.Fatalf("unexpected parsed PlanResult: %+v", parsed)
		}
	})

	t.Run("no changes detected via stdin", func(t *testing.T) {
		planResp = declarativeconfig.PlanResult{
			APIVersion: declarativeconfig.APIVersion,
			Kind:       declarativeconfig.Kind,
			PlanHash:   "sha256:nochange",
			Summary:    declarativeconfig.PlanSummary{Unchanged: 3, HasChanges: false},
			Changes:    []declarativeconfig.ConfigChange{},
		}
		var stdout bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", map[string]string{"SMTP_PASS": resolvedSecret}),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetIn(strings.NewReader(docYAML))
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "diff", "-f", "-"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("config diff -f - failed: %v", err)
		}
		if !strings.Contains(stdout.String(), "No changes detected.") {
			t.Fatalf("expected no-changes message, got %q", stdout.String())
		}
		assertNoDisallowedCopyPunctuation(t, "diff no-changes output", stdout.String())
	})

	t.Run("missing env var fails fast locally before HTTP", func(t *testing.T) {
		callsBefore := planCalls
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "diff", "-f", filePath})
		err := cmd.ExecuteContext(context.Background())
		if err == nil || !strings.Contains(err.Error(), "SMTP_PASS") {
			t.Fatalf("expected missing env var error mentioning SMTP_PASS, got %v", err)
		}
		if planCalls != callsBefore {
			t.Fatalf("expected zero HTTP calls on missing env var, got %d", planCalls-callsBefore)
		}
		assertNoDisallowedCopyPunctuation(t, "missing env error", err.Error())
	})
}

func TestConfigApplyFlowsDriftProGateAndSafety(t *testing.T) {
	docYAML := "apiVersion: pulse.monitoring/v1\nkind: MonitoringConfig\n"
	filePath := filepath.Join(t.TempDir(), "monitoring.yaml")
	if err := os.WriteFile(filePath, []byte(docYAML), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Run("apply with --plan-hash skips plan and sends planHash directly", func(t *testing.T) {
		var planCalled, applyCalled bool
		var gotApplyBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/config/declarative/plan":
				planCalled = true
				http.Error(w, "should not plan", http.StatusInternalServerError)
			case "/api/config/declarative/apply":
				applyCalled = true
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &gotApplyBody)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(declarativeConfigApplyResponse{
					Applied:  true,
					PlanHash: "sha256:reviewed",
					Summary:  declarativeconfig.PlanSummary{Creates: 1, HasChanges: true},
					Changes: []declarativeconfig.ConfigChange{
						{Domain: "availabilityTargets", Action: "create", Key: "web-1", Summary: "Create availability target web-1"},
					},
				})
			}
		}))
		defer server.Close()

		var stdout bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "apply", "-f", filePath, "--plan-hash", "sha256:reviewed"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("config apply --plan-hash failed: %v", err)
		}
		if planCalled || !applyCalled {
			t.Fatalf("planCalled=%t applyCalled=%t, want false/true", planCalled, applyCalled)
		}
		if gotApplyBody["planHash"] != "sha256:reviewed" {
			t.Fatalf("apply planHash = %v, want sha256:reviewed", gotApplyBody["planHash"])
		}
		if !strings.Contains(stdout.String(), "Applied declarative configuration: 1 created, 0 updated, 0 deleted, 0 unchanged") {
			t.Fatalf("unexpected apply stdout:\n%s", stdout.String())
		}
		assertNoDisallowedCopyPunctuation(t, "apply stdout", stdout.String())
	})

	t.Run("apply with --yes plans first and applies with returned planHash", func(t *testing.T) {
		var planCalled, applyCalled bool
		var gotPlanHash string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/config/declarative/plan":
				planCalled = true
				_ = json.NewEncoder(w).Encode(declarativeconfig.PlanResult{
					APIVersion: declarativeconfig.APIVersion,
					Kind:       declarativeconfig.Kind,
					PlanHash:   "sha256:fromplan",
					Summary:    declarativeconfig.PlanSummary{Updates: 1, HasChanges: true},
					Changes:    []declarativeconfig.ConfigChange{{Domain: "alerts", Action: "update", Key: "config", Summary: "Update alert configuration"}},
				})
			case "/api/config/declarative/apply":
				applyCalled = true
				var req map[string]any
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotPlanHash, _ = req["planHash"].(string)
				_ = json.NewEncoder(w).Encode(declarativeConfigApplyResponse{
					Applied:  true,
					PlanHash: gotPlanHash,
					Summary:  declarativeconfig.PlanSummary{Updates: 1, HasChanges: true},
				})
			}
		}))
		defer server.Close()

		var stdout bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "apply", "-f", filePath, "--yes"})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("config apply --yes failed: %v", err)
		}
		if !planCalled || !applyCalled || gotPlanHash != "sha256:fromplan" {
			t.Fatalf("planCalled=%t applyCalled=%t gotPlanHash=%q", planCalled, applyCalled, gotPlanHash)
		}
	})

	t.Run("apply exits cleanly when plan has no changes", func(t *testing.T) {
		applyCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/config/declarative/apply" {
				applyCalled = true
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(declarativeconfig.PlanResult{
				APIVersion: declarativeconfig.APIVersion,
				Kind:       declarativeconfig.Kind,
				PlanHash:   "sha256:clean",
				Summary:    declarativeconfig.PlanSummary{Unchanged: 2, HasChanges: false},
			})
		}))
		defer server.Close()

		var stdout bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "apply", "-f", filePath})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("config apply no-op failed: %v", err)
		}
		if applyCalled {
			t.Fatal("apply endpoint must not be called when plan has no changes")
		}
		if !strings.Contains(stdout.String(), "No changes to apply.") {
			t.Fatalf("expected no-changes message, got %q", stdout.String())
		}
	})

	t.Run("non-TTY refuses without --plan-hash or --yes", func(t *testing.T) {
		applyCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/config/declarative/apply" {
				applyCalled = true
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(declarativeconfig.PlanResult{
				APIVersion: declarativeconfig.APIVersion,
				Kind:       declarativeconfig.Kind,
				PlanHash:   "sha256:needsconfirm",
				Summary:    declarativeconfig.PlanSummary{Creates: 1, HasChanges: true},
				Changes:    []declarativeconfig.ConfigChange{{Domain: "operatorStates", Action: "create", Key: "vm:101", Summary: "Create operator state for vm:101"}},
			})
		}))
		defer server.Close()

		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetIn(strings.NewReader(""))
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "apply", "-f", filePath})
		err := cmd.ExecuteContext(context.Background())
		if err == nil {
			t.Fatal("expected non-TTY refusal error")
		}
		if applyCalled {
			t.Fatal("apply must not run when non-TTY confirmation is missing")
		}
		if !strings.Contains(err.Error(), "--plan-hash sha256:needsconfirm") || !strings.Contains(err.Error(), "--yes") {
			t.Fatalf("unexpected non-TTY error: %v", err)
		}
		assertNoDisallowedCopyPunctuation(t, "non-TTY error", err.Error())
	})

	t.Run("interactive TTY prompt confirms and applies", func(t *testing.T) {
		prev := declarativeConfigStdinIsTerminal
		declarativeConfigStdinIsTerminal = func(io.Reader) bool { return true }
		t.Cleanup(func() { declarativeConfigStdinIsTerminal = prev })

		applyCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/config/declarative/plan" {
				_ = json.NewEncoder(w).Encode(declarativeconfig.PlanResult{
					APIVersion: declarativeconfig.APIVersion,
					Kind:       declarativeconfig.Kind,
					PlanHash:   "sha256:tty1",
					Summary:    declarativeconfig.PlanSummary{Creates: 1, HasChanges: true},
					Changes:    []declarativeconfig.ConfigChange{{Domain: "operatorStates", Action: "create", Key: "vm:101", Summary: "Create operator state for vm:101"}},
				})
				return
			}
			applyCalled = true
			_ = json.NewEncoder(w).Encode(declarativeConfigApplyResponse{
				Applied:  true,
				PlanHash: "sha256:tty1",
				Summary:  declarativeconfig.PlanSummary{Creates: 1, HasChanges: true},
			})
		}))
		defer server.Close()

		var stdout bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetIn(strings.NewReader("y\n"))
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "apply", "-f", filePath})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("interactive TTY apply failed: %v", err)
		}
		if !applyCalled {
			t.Fatal("expected apply endpoint to be called after y confirmation")
		}
		if !strings.Contains(stdout.String(), "Apply these changes? [y/N]: ") {
			t.Fatalf("expected interactive prompt in output:\n%s", stdout.String())
		}
		assertNoDisallowedCopyPunctuation(t, "interactive prompt output", stdout.String())
	})

	t.Run("409 drift error formatted cleanly", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(agentcapabilities.NewErrorEnvelope(
				agentcapabilities.AgentErrCodeDeclarativeConfigPlanDrift,
				"configuration changed since planning",
				map[string]string{"expectedPlanHash": "sha256:fresh999"},
			))
		}))
		defer server.Close()

		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "apply", "-f", filePath, "--plan-hash", "sha256:stale"})
		err := cmd.ExecuteContext(context.Background())
		if err == nil {
			t.Fatal("expected 409 drift error")
		}
		if !strings.Contains(err.Error(), "declarative_config_plan_drift") || !strings.Contains(err.Error(), "sha256:fresh999") {
			t.Fatalf("unexpected drift error: %v", err)
		}
		assertNoDisallowedCopyPunctuation(t, "409 drift error", err.Error())
	})

	t.Run("402 Pro gate error formatted cleanly", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = io.WriteString(w, `{"error":"license_required","message":"Remote probe agents require Pulse Pro","feature":"external_probe","upgrade_url":"https://pulserelay.pro/pricing"}`)
		}))
		defer server.Close()

		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote: &RemoteDeps{
				HTTPClient: server.Client(),
				Getenv:     newDeclarativeTestEnv(server.URL, "tok_123", nil),
				IsTerminal: func(io.Writer) bool { return true },
			},
		})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"config", "apply", "-f", filePath, "--plan-hash", "sha256:procheck"})
		err := cmd.ExecuteContext(context.Background())
		if err == nil {
			t.Fatal("expected 402 Pro gate error")
		}
		if !strings.Contains(err.Error(), "Pulse Pro") || !strings.Contains(err.Error(), "external_probe") {
			t.Fatalf("unexpected 402 error: %v", err)
		}
		assertNoDisallowedCopyPunctuation(t, "402 Pro gate error", err.Error())
	})
}

func TestConfigSubcommandRegistrationOnClientAndServerRoots(t *testing.T) {
	clientRoot := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{})
	for _, path := range [][]string{
		{"config"},
		{"config", "pull"},
		{"config", "diff"},
		{"config", "apply"},
	} {
		found, _, err := clientRoot.Find(path)
		if err != nil || found == nil || found == clientRoot {
			t.Fatalf("client root missing subcommand %v (found=%v err=%v)", path, found, err)
		}
	}
	for _, disallowed := range [][]string{
		{"config", "export"},
		{"config", "import"},
		{"config", "info"},
	} {
		found, rem, err := clientRoot.Find(disallowed)
		if err == nil && found != nil && found.Name() == disallowed[1] && len(rem) == 0 {
			t.Fatalf("client root should not expose server DR subcommand %v", disallowed)
		}
	}

	serverRoot := NewRootCommand(CommandSpec{Use: "pulse"}, RuntimeSpec{}, CommandDeps{})
	for _, path := range [][]string{
		{"config", "export"},
		{"config", "import"},
		{"config", "info"},
		{"config", "auto-import"},
		{"config", "pull"},
		{"config", "diff"},
		{"config", "apply"},
	} {
		found, _, err := serverRoot.Find(path)
		if err != nil || found == nil || found == serverRoot {
			t.Fatalf("server root missing subcommand %v (found=%v err=%v)", path, found, err)
		}
	}

	// Copy hygiene check across config and all declarative subcommand help output.
	for _, args := range [][]string{
		{"config", "--help"},
		{"config", "pull", "--help"},
		{"config", "diff", "--help"},
		{"config", "apply", "--help"},
	} {
		var buf bytes.Buffer
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{})
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs(args)
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("%v failed: %v", args, err)
		}
		assertNoDisallowedCopyPunctuation(t, strings.Join(args, " "), buf.String())
	}
}
