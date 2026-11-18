package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type accountPortalStep struct {
	Name        string            `yaml:"name"`
	Uses        string            `yaml:"uses"`
	Run         string            `yaml:"run"`
	Dir         string            `yaml:"working-directory"`
	Env         map[string]string `yaml:"env"`
	With        map[string]string `yaml:"with"`
	Conditional any               `yaml:"if"`
	Tolerant    any               `yaml:"continue-on-error"`
}

type accountPortalJob struct {
	Name        string              `yaml:"name"`
	RunsOn      string              `yaml:"runs-on"`
	Timeout     int                 `yaml:"timeout-minutes"`
	Steps       []accountPortalStep `yaml:"steps"`
	Conditional any                 `yaml:"if"`
	Tolerant    any                 `yaml:"continue-on-error"`
}

type accountPortalWorkflowSpec struct {
	On          map[string]yaml.Node        `yaml:"on"`
	Permissions map[string]string           `yaml:"permissions"`
	Concurrency map[string]string           `yaml:"concurrency"`
	Jobs        map[string]accountPortalJob `yaml:"jobs"`
}

func readAccountPortalWorkflow(t *testing.T) accountPortalWorkflowSpec {
	t.Helper()
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..")
	content, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "account-portal.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow accountPortalWorkflowSpec
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(workflow.Permissions, map[string]string{"contents": "read"}) || len(workflow.Jobs) != 2 || len(workflow.On) != 3 {
		t.Fatal("portal CI must stay read-only, with separate PR/main and scheduled jobs")
	}
	return workflow
}

func accountPortalWorkflow(t *testing.T) ([]accountPortalStep, string) {
	t.Helper()
	workflow := readAccountPortalWorkflow(t)
	for _, event := range []string{"push", "pull_request"} {
		var trigger struct {
			Branches []string `yaml:"branches"`
			Paths    []string `yaml:"paths"`
		}
		item := workflow.On[event]
		if err := item.Decode(&trigger); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(trigger.Branches, []string{"main"}) || !reflect.DeepEqual(trigger.Paths, []string{
			"internal/cloudcp/portal/**", "scripts/exclusive-lock.mjs",
			"scripts/installtests/account_portal_workflow_test.go", ".github/workflows/account-portal.yml",
		}) {
			t.Fatalf("%s must admit the whole portal, shared build helper and CI controls", event)
		}
	}
	job := workflow.Jobs["portal-frontend"]
	if job.Name != "Account portal typecheck, tests and embed" || job.RunsOn != "ubuntu-24.04" || job.Timeout != 10 ||
		job.Conditional != "${{ github.event_name != 'schedule' }}" || job.Tolerant != nil {
		t.Fatal("missing bounded, non-optional portal PR/main job")
	}
	if len(job.Steps) != 4 {
		t.Fatal("portal job must check exact source, set up Node and validate the whole module")
	}
	for _, step := range job.Steps {
		if step.Conditional != nil || step.Tolerant != nil || strings.Contains(step.Run, "secrets.") {
			t.Fatalf("portal check cannot be optional, tolerant or credentialed: %s", step.Name)
		}
	}
	checkout, identity, node, proof := job.Steps[0], job.Steps[1], job.Steps[2], job.Steps[3]
	if checkout.Uses != "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1" ||
		!reflect.DeepEqual(checkout.With, map[string]string{"ref": "${{ github.sha }}", "persist-credentials": "false"}) ||
		identity.Env["EXPECTED_SOURCE_SHA"] != "${{ github.sha }}" || identity.Run != `test "$(git rev-parse HEAD)" = "$EXPECTED_SOURCE_SHA"` {
		t.Fatal("portal proof must execute the credential-free exact checkout")
	}
	if node.Uses != "actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1" ||
		!reflect.DeepEqual(node.With, map[string]string{"node-version": "24", "cache": "npm", "cache-dependency-path": "internal/cloudcp/portal/frontend/package-lock.json"}) {
		t.Fatal("portal proof must use its own locked npm module")
	}
	if proof.Dir != "internal/cloudcp/portal/frontend" || proof.Name != "Validate complete portal module and committed embed" {
		t.Fatal("portal proof must not substitute main-frontend or focused tests")
	}
	return job.Steps, proof.Run
}

func TestAccountPortalDailyAdvisoryWatch(t *testing.T) {
	workflow := readAccountPortalWorkflow(t)
	schedule := workflow.On["schedule"]
	var cron []map[string]string
	if err := schedule.Decode(&cron); err != nil || !reflect.DeepEqual(cron, []map[string]string{{"cron": "43 5 * * *"}}) {
		t.Fatalf("the unchanged graph needs one daily read: %v %v", cron, err)
	}
	if !reflect.DeepEqual(workflow.Concurrency, map[string]string{
		"group":              "account-portal-${{ github.event_name == 'schedule' && 'advisory-watch' || github.ref }}",
		"cancel-in-progress": "${{ github.event_name == 'pull_request' }}",
	}) {
		t.Fatal("scheduled observation must not replace a pending build")
	}
	job := workflow.Jobs["portal-advisories"]
	if job.Name != "Account portal dependency advisory watch" || job.RunsOn != "ubuntu-24.04" || job.Timeout != 10 ||
		job.Conditional != "${{ github.event_name == 'schedule' }}" || job.Tolerant != nil || len(job.Steps) != 3 {
		t.Fatal("missing bounded, mandatory scheduled account audit")
	}
	checkout, node, audit := job.Steps[0], job.Steps[1], job.Steps[2]
	if checkout.Uses != "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1" ||
		!reflect.DeepEqual(checkout.With, map[string]string{"ref": "${{ github.sha }}", "persist-credentials": "false"}) ||
		node.Uses != "actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1" ||
		!reflect.DeepEqual(node.With, map[string]string{"node-version": "24", "package-manager-cache": "false"}) {
		t.Fatal("the daily audit must use the exact source and a cache-free, credential-free Node setup")
	}
	for _, step := range job.Steps {
		if step.Conditional != nil || step.Tolerant != nil {
			t.Fatalf("step must not read as optional: %s", step.Name)
		}
	}
	if audit.Uses != "" || audit.Dir != "" || !reflect.DeepEqual(audit.Env, map[string]string{
		"EXPECTED_SOURCE_REF":      "${{ github.ref }}",
		"EXPECTED_SOURCE_SHA":      "${{ github.sha }}",
		"NPM_AUDIT_REQUIRE_RESULT": "true",
	}) {
		t.Fatal("audit must require a strict result at main's bound source")
	}

	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct {
		state, wantMessage string
		shows              int
		npm, pass          bool
	}{
		{"clean", "no vulnerabilities", 2, true, true},
		{"finding", "audit finding ", 2, true, false},
		{"unavailable", "A fresh strict result is required", 2, true, false},
		{"wrong-ref", "requires the exact scheduled main source", 0, false, false},
		{"wrong-sha", "requires the exact scheduled main source", 0, false, false},
		{"bad-sha", "requires the exact scheduled main source", 0, false, false},
		{"missing-lock", "fixture lock missing", 2, false, false},
	} {
		t.Run(tc.state, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			writeAccountPortalFixture(t, filepath.Join(dir, "git"), `#!/bin/bash
set -euo pipefail
if [[ "$*" == 'rev-parse HEAD' ]]; then
  if [[ "$FAULT" == wrong-sha ]]; then printf '%040d\n' 0; else echo "$EXACT_SHA"; fi
  exit 0
fi
test "$#" -eq 2
test "$1" = show
printf 'git %s\n' "$*" >> "$CALLS"
case "$2" in
  "$EXACT_SHA:internal/cloudcp/portal/frontend/package.json") echo '{"name":"account-fixture"}' ;;
  "$EXACT_SHA:internal/cloudcp/portal/frontend/package-lock.json")
    if [[ "$FAULT" == missing-lock ]]; then echo 'fixture lock missing' >&2; exit 17; fi
    echo '{"name":"account-fixture","lockfileVersion":3}'
    ;;
  *) exit 18 ;;
esac
`, 0700)
			writeAccountPortalFixture(t, filepath.Join(dir, "npm"), `#!/bin/bash
set -euo pipefail
printf 'npm %s\n' "$*" >> "$CALLS"
[[ "$PWD" == "$RUNNER_TEMP"/account-portal-audit.* ]]
test "$(cat package.json)" = '{"name":"account-fixture"}'
test "$(cat package-lock.json)" = '{"name":"account-fixture","lockfileVersion":3}'
test ! -e node_modules
test ! -e .npmrc
case "$FAULT" in
  clean) echo '{"metadata":{"vulnerabilities":{"total":0,"critical":0,"high":0,"moderate":0,"low":0,"info":0}}}' ;;
  finding) echo '{"vulnerabilities":{"fixture-dependency":{"name":"fixture-dependency","severity":"low"}}}'; exit 1 ;;
  unavailable) echo '{}'; exit 1 ;;
  *) exit 19 ;;
esac
`, 0700)
			ref, expected := "refs/heads/main", sha
			if tc.state == "wrong-ref" {
				ref = "refs/heads/not-main"
			}
			if tc.state == "bad-sha" {
				expected = ""
			}
			cmd := exec.Command("bash", "-c", audit.Run)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"),
				"RUNNER_TEMP="+dir, "GITHUB_WORKSPACE="+root, "FAULT="+tc.state, "CALLS="+calls,
				"EXACT_SHA="+sha, "EXPECTED_SOURCE_REF="+ref, "EXPECTED_SOURCE_SHA="+expected,
				"NPM_AUDIT_CMD="+filepath.Join(dir, "npm"), "NPM_AUDIT_ATTEMPTS=1",
				"NPM_AUDIT_REQUIRE_RESULT="+audit.Env["NPM_AUDIT_REQUIRE_RESULT"])
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.pass || !strings.Contains(string(out), tc.wantMessage) ||
				strings.Contains(string(out), "AUDITED_MAIN_SHA="+sha) != tc.npm {
				t.Fatalf("wrong audit verdict: %v\n%s", err, out)
			}
			want := []string{
				"git show " + sha + ":internal/cloudcp/portal/frontend/package.json\n",
				"git show " + sha + ":internal/cloudcp/portal/frontend/package-lock.json\n",
			}[:tc.shows]
			if tc.npm {
				want = append(want, "npm audit --json --package-lock-only\n")
			}
			got, readErr := os.ReadFile(calls)
			if len(want) == 0 && os.IsNotExist(readErr) {
				return
			}
			if readErr != nil || string(got) != strings.Join(want, "") {
				t.Fatalf("wrong actual source/npm operations: %v\n%s\nwant %v", readErr, got, want)
			}
		})
	}
}

func TestAccountPortalWorkflowCoversCompleteModule(t *testing.T) {
	steps, proof := accountPortalWorkflow(t)
	// These are the actual workflow commands, without test filters, offline
	// substitutions or a build that silently commits corrected served assets.
	want := []string{
		"set -euo pipefail",
		`sha256sum ../dist/portal_app.js ../dist/portal_app.css ../dist/build_manifest.json > "$RUNNER_TEMP/account-portal-assets.sha256"`,
		"npm ci --ignore-scripts --no-audit --no-fund",
		"npm run typecheck",
		"npm test -- --maxWorkers=2",
		"npm run build",
		`sha256sum --check "$RUNNER_TEMP/account-portal-assets.sha256"`,
		"git diff --exit-code -- ../dist",
	}
	if !reflect.DeepEqual(strings.Split(strings.TrimSpace(proof), "\n"), want) {
		t.Fatal("full module commands and immutable embed checks changed")
	}
	for _, tc := range []struct{ actual, expected string }{{"exact", "exact"}, {"other", "exact"}} {
		dir := t.TempDir()
		writeAccountPortalFixture(t, filepath.Join(dir, "git"), "#!/bin/sh\nprintf '%s\\n' \"$ACTUAL_SHA\"\n", 0700)
		cmd := exec.Command("bash", "-euo", "pipefail", "-c", steps[1].Run)
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "ACTUAL_SHA="+tc.actual, "EXPECTED_SOURCE_SHA="+tc.expected)
		out, err := cmd.CombinedOutput()
		if (err == nil) != (tc.actual == tc.expected) {
			t.Fatalf("source identity %q/%q: %v\n%s", tc.actual, tc.expected, err, out)
		}
	}
}

func writeAccountPortalFixture(t *testing.T, name, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(name, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestAccountPortalWorkflowStopsOnFailureAndStaleEmbed(t *testing.T) {
	_, proof := accountPortalWorkflow(t)
	for _, fault := range []string{"none", "ci", "typecheck", "test", "build", "javascript", "css", "manifest", "missing-asset", "diff"} {
		t.Run(fault, func(t *testing.T) {
			dir := t.TempDir()
			for _, child := range []string{"bin", "frontend", "dist", "runner"} {
				if err := os.Mkdir(filepath.Join(dir, child), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"portal_app.js", "portal_app.css", "build_manifest.json"} {
				if fault != "missing-asset" || name != "build_manifest.json" {
					writeAccountPortalFixture(t, filepath.Join(dir, "dist", name), "committed "+name+"\n", 0600)
				}
			}
			writeAccountPortalFixture(t, filepath.Join(dir, "bin", "npm"), `#!/bin/bash
set -euo pipefail
printf '%s\n' "$*" >> "$CALLS"
command=$1
if [ "$1" = run ]; then command=$2; fi
if [ "$command" = "$FAULT" ]; then exit 23; fi
if [ "$command" = build ]; then
  case "$FAULT" in
    javascript) echo stale >> ../dist/portal_app.js ;;
    css) echo stale >> ../dist/portal_app.css ;;
    manifest) echo stale >> ../dist/build_manifest.json ;;
  esac
fi
`, 0700)
			writeAccountPortalFixture(t, filepath.Join(dir, "bin", "git"), `#!/bin/bash
set -euo pipefail
printf 'git %s\n' "$*" >> "$CALLS"
test "$FAULT" != diff
`, 0700)
			calls := filepath.Join(dir, "calls")
			cmd := exec.Command("bash", "-euo", "pipefail", "-c", proof)
			cmd.Dir = filepath.Join(dir, "frontend")
			cmd.Env = append(os.Environ(), "PATH="+filepath.Join(dir, "bin")+":"+os.Getenv("PATH"), "RUNNER_TEMP="+filepath.Join(dir, "runner"), "FAULT="+fault, "CALLS="+calls)
			out, err := cmd.CombinedOutput()
			if (err == nil) != (fault == "none") {
				t.Fatalf("fault %s: %v\n%s", fault, err, out)
			}
			actual, readErr := os.ReadFile(calls)
			want := []string{"ci --ignore-scripts --no-audit --no-fund", "run typecheck", "test -- --maxWorkers=2", "run build", "git diff --exit-code -- ../dist"}
			for i, phase := range []string{"ci", "typecheck", "test", "build"} {
				if fault == phase {
					want = want[:i+1]
					break
				}
			}
			if fault == "missing-asset" {
				if !os.IsNotExist(readErr) {
					t.Fatal("missing committed assets must fail before npm")
				}
				return
			}
			if fault == "javascript" || fault == "css" || fault == "manifest" {
				want = want[:4]
			}
			if readErr != nil || string(actual) != strings.Join(want, "\n")+"\n" {
				t.Fatalf("wrong actual command sequence: %v\n%s\nwant %v", readErr, actual, want)
			}
		})
	}
}
