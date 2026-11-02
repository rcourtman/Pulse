package installtests

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
)

func ciAdmissionCaseNames(output []byte) ([]string, error) {
	var names []string
	if err := json.Unmarshal(output, &names); err != nil {
		return nil, fmt.Errorf("invalid CI admission inventory: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("empty CI admission inventory")
	}
	valid := regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*\.test_[A-Za-z_0-9]+$`)
	seen := make(map[string]bool)
	for _, name := range names {
		if !valid.MatchString(name) || seen[name] {
			return nil, fmt.Errorf("invalid or repeated CI admission check %q", name)
		}
		seen[name] = true
	}
	return names, nil
}

func TestCIAdmissionInventoryRetainsEveryDiscoveredCheck(t *testing.T) {
	for _, output := range []string{
		``, `null`, `[]`, `{}`, `[1]`, `["Case.test_one"] trailing`,
		`["Case.test_one", "Case.test_one"]`, `["Case.test_one", ""]`,
		`["Case.test_one", "Other"]`, `["Case.test_one", "Case.test_two --skip"]`,
		`["Case.test_one", "Case.test_two\nCase.test_three"]`,
	} {
		t.Run(output, func(t *testing.T) {
			if _, err := ciAdmissionCaseNames([]byte(output)); err == nil {
				t.Fatal("unsafe or incomplete inventory was accepted")
			}
		})
	}
	output := `["FutureCase.test_last", "Case.test_first", "Case.test_added"]`
	names, err := ciAdmissionCaseNames([]byte(output))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 || names[0] != "FutureCase.test_last" || names[1] != "Case.test_first" || names[2] != "Case.test_added" {
		t.Fatalf("discovered checks were omitted or reordered: %v", names)
	}
}
