package installtests

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type nativeHelperWorkflow struct {
	On   map[string]struct{ Paths []string }
	Jobs map[string]struct {
		Strategy struct {
			Matrix struct {
				Include []struct{ Runner string }
			}
		}
		Steps []struct {
			Name, If, Run   string
			ContinueOnError bool `yaml:"continue-on-error"`
		}
	}
}

var nativeHelperTriggerPaths = []string{
	"cmd/pulse-agent-helper/**",
	"internal/agenthelper/**",
	"internal/filesystemprobe/**",
	"internal/updatesignature/**",
	"pkg/agents/**",
}

func validateNativeHelperWorkflow(content []byte) error {
	var workflow nativeHelperWorkflow
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return err
	}
	for _, event := range []string{"push", "pull_request"} {
		paths := workflow.On[event].Paths
		for _, required := range nativeHelperTriggerPaths {
			found := false
			for _, pattern := range paths {
				found = found || pattern == required
			}
			if !found {
				return fmt.Errorf("%s does not admit helper changes in %s", event, required)
			}
		}
	}
	job := workflow.Jobs["native-agent"]
	for _, required := range []string{"ubuntu-24.04", "ubuntu-24.04-arm"} {
		found := false
		for _, platform := range job.Strategy.Matrix.Include {
			found = found || platform.Runner == required
		}
		if !found {
			return fmt.Errorf("missing native Linux helper platform %s", required)
		}
	}
	wanted := map[string][]string{
		"Test native Linux helper boundaries": {
			"go", "test", "-race", "-count=1", "-timeout", "5m",
			"./cmd/pulse-agent-helper", "./internal/agenthelper", "./internal/filesystemprobe",
			"./internal/updatesignature", "./pkg/agents/host",
		},
		"Build native Linux helper": {"go", "build", "-o", "pulse-agent-helper-native", "./cmd/pulse-agent-helper"},
	}
	for _, step := range job.Steps {
		argv, required := wanted[step.Name]
		if !required {
			continue
		}
		if step.If != "runner.os == 'Linux'" || step.ContinueOnError || !reflect.DeepEqual(strings.Fields(step.Run), argv) {
			return fmt.Errorf("%s must retain both Linux platforms, every package and its failing verdict", step.Name)
		}
		delete(wanted, step.Name)
	}
	if len(wanted) != 0 {
		return fmt.Errorf("missing native helper steps: %v", wanted)
	}
	return nil
}

func TestNativeHelperWorkflowCoversChangedSourceAndLinuxBoundaries(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNativeHelperWorkflow(content); err != nil {
		t.Fatal(err)
	}
	// A helper-only change must admit both PR and main verification. Check each
	// event separately: deleting only one occurrence must not be concealed by
	// the other event's matching filter.
	for _, required := range nativeHelperTriggerPaths {
		for _, occurrence := range []int{0, 1} {
			t.Run(fmt.Sprintf("missing trigger %s event %d", required, occurrence), func(t *testing.T) {
				pattern := "      - '" + required + "'\n"
				start := strings.Index(string(content), pattern)
				if occurrence == 1 && start >= 0 {
					next := strings.Index(string(content)[start+len(pattern):], pattern)
					if next < 0 {
						t.Fatal("missing second trigger in original workflow")
					}
					start += len(pattern) + next
				}
				if start < 0 {
					t.Fatal("missing trigger in original workflow")
				}
				changed := string(content[:start]) + string(content[start+len(pattern):])
				if err := validateNativeHelperWorkflow([]byte(changed)); err == nil {
					t.Fatal("accepted helper-only source without native verification")
				}
			})
		}
	}
	for _, scenario := range []struct{ name, old, new string }{
		{"missing x64", "runner: ubuntu-24.04\n", "runner: macos-15\n"},
		{"missing arm64", "runner: ubuntu-24.04-arm\n", "runner: macos-15\n"},
		{"not both Linux jobs", "if: runner.os == 'Linux'", "if: matrix.runner == 'ubuntu-24.04'"},
		{"tolerated helper failure", "name: Test native Linux helper boundaries", "name: Test native Linux helper boundaries\n        continue-on-error: true"},
		{"missing helper command tests", "          ./cmd/pulse-agent-helper\n", ""},
		{"missing protocol tests", "          ./internal/agenthelper\n", ""},
		{"missing filesystem fences", "          ./internal/filesystemprobe\n", ""},
		{"missing signature tests", "          ./internal/updatesignature\n", ""},
		{"missing host wire tests", "          ./pkg/agents/host\n", ""},
		{"cached verdict", "go test -race -count=1 -timeout 5m", "go test -race -timeout 5m"},
		{"missing race coverage", "go test -race -count=1 -timeout 5m", "go test -count=1 -timeout 5m"},
		{"hidden test failure", "          ./pkg/agents/host\n", "          ./pkg/agents/host || true\n"},
		{"missing build", "name: Build native Linux helper", "name: Omitted helper build"},
		{"hidden build failure", "run: go build -o pulse-agent-helper-native ./cmd/pulse-agent-helper", "run: go build -o pulse-agent-helper-native ./cmd/pulse-agent-helper || true"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			changed := strings.Replace(string(content), scenario.old, scenario.new, 1)
			if changed == string(content) {
				t.Fatal("control did not change the workflow")
			}
			if err := validateNativeHelperWorkflow([]byte(changed)); err == nil {
				t.Fatal("accepted missing or non-authoritative helper verification")
			}
		})
	}
}
