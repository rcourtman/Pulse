package installtests

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type nativeAgentFrontendStep struct {
	Name             string
	If               string
	Uses             string
	With             map[string]string
	WorkingDirectory string `yaml:"working-directory"`
	Run              string
}

func validateNativeAgentFrontendTriggers(content []byte) error {
	var workflow struct {
		On map[string]struct{ Paths []string }
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return err
	}
	for _, event := range []string{"push", "pull_request"} {
		paths := workflow.On[event].Paths
		for _, required := range []string{"frontend-modern/package.json", "frontend-modern/package-lock.json"} {
			found := false
			for _, pattern := range paths {
				if strings.HasPrefix(pattern, "!") {
					return fmt.Errorf("%s must not exclude native frontend inputs", event)
				}
				found = found || pattern == required
			}
			if !found {
				return fmt.Errorf("%s must admit native frontend graph changes in %s", event, required)
			}
		}
	}
	return nil
}

func TestNativeAgentWorkflowAdmitsFrontendGraphChanges(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNativeAgentFrontendTriggers(content); err != nil {
		t.Fatal(err)
	}
	// Decode each event, rather than accepting a matching path elsewhere in
	// the file. Both PR qualification and containing-main checks need it.
	for _, event := range []string{"push", "pull_request"} {
		for _, required := range []string{"frontend-modern/package.json", "frontend-modern/package-lock.json"} {
			t.Run(event+" missing "+required, func(t *testing.T) {
				var workflow map[string]interface{}
				if err := yaml.Unmarshal(content, &workflow); err != nil {
					t.Fatal(err)
				}
				trigger := workflow["on"].(map[string]interface{})[event].(map[string]interface{})
				paths := trigger["paths"].([]interface{})
				var changed []interface{}
				for _, pattern := range paths {
					if pattern != required {
						changed = append(changed, pattern)
					}
				}
				if len(changed) != len(paths)-1 {
					t.Fatal("control must remove exactly one original input")
				}
				trigger["paths"] = changed
				mutant, err := yaml.Marshal(workflow)
				if err != nil {
					t.Fatal(err)
				}
				if err := validateNativeAgentFrontendTriggers(mutant); err == nil {
					t.Fatal("accepted frontend graph change without native verification")
				}
			})
		}
		t.Run(event+" excluded frontend graph", func(t *testing.T) {
			var workflow map[string]interface{}
			if err := yaml.Unmarshal(content, &workflow); err != nil {
				t.Fatal(err)
			}
			trigger := workflow["on"].(map[string]interface{})[event].(map[string]interface{})
			trigger["paths"] = append(trigger["paths"].([]interface{}), "!frontend-modern/**")
			mutant, err := yaml.Marshal(workflow)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateNativeAgentFrontendTriggers(mutant); err == nil {
				t.Fatal("accepted a later filter excluding the native frontend graph")
			}
		})
	}
}

func validateNativeAgentFrontendPreparation(steps []nativeAgentFrontendStep) error {
	indices := map[string]int{}
	for i, step := range steps {
		indices[step.Name] = i
	}
	names := []string{
		"Set up Node.js for native runtime test assets",
		"Install frontend test dependencies",
		"Build real frontend assets for API-linked native tests",
		"Test native agent runtime",
	}
	previous := -1
	for _, name := range names {
		i, ok := indices[name]
		if !ok || i <= previous || steps[i].If != "" {
			return fmt.Errorf("%s must cover every native platform, in preparation order", name)
		}
		previous = i
	}
	node, dependencies, build := steps[indices[names[0]]], steps[indices[names[1]]], steps[indices[names[2]]]
	if node.Uses != "actions/setup-node@820762786026740c76f36085b0efc47a31fe5020" ||
		node.With["node-version"] != "24" || node.With["cache"] != "npm" ||
		node.With["cache-dependency-path"] != "frontend-modern/package-lock.json" {
		return fmt.Errorf("native frontend preparation must use the pinned Node tool and locked cache")
	}
	if dependencies.WorkingDirectory != "frontend-modern" || strings.TrimSpace(dependencies.Run) != "npm ci" ||
		build.WorkingDirectory != "frontend-modern" || strings.TrimSpace(build.Run) != "npm run build" {
		return fmt.Errorf("native tests need the locked real frontend build, not placeholder assets")
	}
	return nil
}

func TestNativeAgentWorkflowBuildsRealFrontendBeforeRuntime(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct{ Steps []nativeAgentFrontendStep }
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["native-agent"].Steps
	if err := validateNativeAgentFrontendPreparation(steps); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []string{"missing build", "Windows-only preparation", "late build", "stub build"} {
		t.Run(scenario, func(t *testing.T) {
			changed := append([]nativeAgentFrontendStep(nil), steps...)
			buildIndex, runtimeIndex := -1, -1
			for i, step := range changed {
				switch step.Name {
				case "Build real frontend assets for API-linked native tests":
					buildIndex = i
				case "Test native agent runtime":
					runtimeIndex = i
				}
			}
			switch scenario {
			case "missing build":
				changed = append(changed[:buildIndex], changed[buildIndex+1:]...)
			case "Windows-only preparation":
				changed[buildIndex].If = "${{ !matrix.unix }}"
			case "late build":
				changed[buildIndex], changed[runtimeIndex] = changed[runtimeIndex], changed[buildIndex]
			case "stub build":
				changed[buildIndex].Run = "mkdir -p ../internal/api/frontend-modern/dist && touch ../internal/api/frontend-modern/dist/index.html"
			}
			if err := validateNativeAgentFrontendPreparation(changed); err == nil {
				t.Fatalf("accepted %s instead of real all-platform preparation", scenario)
			}
		})
	}
}
