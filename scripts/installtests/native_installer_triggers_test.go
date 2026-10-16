package installtests

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The Unix native step runs the root/server installer fixtures as well as
// scripts/install.sh's agent fixtures. Linux-only Build and Test coverage does
// not supply the macOS verdict for these source and fixture inputs, including
// local update completion and both half-removed-install recovery paths.
var nativeServerUpgradeInputs = []string{
	"install.sh",
	"scripts/pulse-auto-update.sh",
	"scripts/tests/test_auto_update_completion.py",
	"scripts/tests/test_auto_update_service_state.py",
	"scripts/tests/test_auto_update_systemd.py",
	"scripts/tests/test_server_service_discovery.py",
	"scripts/tests/test_server_installer_start.py",
	"scripts/tests/test_server_update_repair.py",
}

func validateNativeServerUpgradeTriggers(content []byte) error {
	var workflow struct {
		On map[string]struct{ Paths []string }
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return err
	}
	for _, event := range []string{"push", "pull_request"} {
		paths := workflow.On[event].Paths
		for _, pattern := range paths {
			if strings.HasPrefix(pattern, "!") {
				return fmt.Errorf("%s must not exclude native installer inputs", event)
			}
		}
		for _, required := range nativeServerUpgradeInputs {
			found := false
			for _, pattern := range paths {
				found = found || pattern == required
			}
			if !found {
				return fmt.Errorf("%s must admit native server upgrade input %s", event, required)
			}
		}
	}
	return nil
}

func TestNativeInstallerWorkflowAdmitsServerUpgradeSource(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNativeServerUpgradeTriggers(content); err != nil {
		t.Fatal(err)
	}
	// Decode and change one event at a time. A matching input in the other
	// event, or an unrelated job string, cannot conceal a missing admission.
	for _, event := range []string{"push", "pull_request"} {
		for _, required := range nativeServerUpgradeInputs {
			for _, exclusion := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/excluded=%t", event, required, exclusion), func(t *testing.T) {
					var workflow map[string]interface{}
					if err := yaml.Unmarshal(content, &workflow); err != nil {
						t.Fatal(err)
					}
					trigger := workflow["on"].(map[string]interface{})[event].(map[string]interface{})
					paths := trigger["paths"].([]interface{})
					if exclusion {
						trigger["paths"] = append(paths, "!"+required)
					} else {
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
					}
					mutant, err := yaml.Marshal(workflow)
					if err != nil {
						t.Fatal(err)
					}
					if err := validateNativeServerUpgradeTriggers(mutant); err == nil {
						t.Fatal("accepted upgrade input without native verification")
					}
				})
			}
		}
	}
}
