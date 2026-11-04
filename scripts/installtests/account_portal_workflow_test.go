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

func accountPortalWorkflow(t *testing.T) ([]accountPortalStep, string) {
	t.Helper()
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..")
	content, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "account-portal.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On map[string]struct {
			Branches []string `yaml:"branches"`
			Paths    []string `yaml:"paths"`
		} `yaml:"on"`
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			Name        string              `yaml:"name"`
			RunsOn      string              `yaml:"runs-on"`
			Timeout     int                 `yaml:"timeout-minutes"`
			Steps       []accountPortalStep `yaml:"steps"`
			Conditional any                 `yaml:"if"`
			Tolerant    any                 `yaml:"continue-on-error"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(workflow.Permissions, map[string]string{"contents": "read"}) || len(workflow.Jobs) != 1 || len(workflow.On) != 2 {
		t.Fatal("portal CI must remain a read-only PR/main check")
	}
	for _, event := range []string{"push", "pull_request"} {
		trigger := workflow.On[event]
		if !reflect.DeepEqual(trigger.Branches, []string{"main"}) || !reflect.DeepEqual(trigger.Paths, []string{
			"internal/cloudcp/portal/**", "scripts/exclusive-lock.mjs",
			"scripts/installtests/account_portal_workflow_test.go", ".github/workflows/account-portal.yml",
		}) {
			t.Fatalf("%s must admit the whole portal, shared build helper and CI controls", event)
		}
	}
	job := workflow.Jobs["portal-frontend"]
	if job.Name != "Account portal typecheck, tests and embed" || job.RunsOn != "ubuntu-24.04" || job.Timeout != 10 || job.Conditional != nil || job.Tolerant != nil {
		t.Fatal("missing bounded, non-optional portal job")
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
