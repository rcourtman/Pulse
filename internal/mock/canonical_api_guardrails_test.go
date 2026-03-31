package mock

import (
	"os"
	"strings"
	"testing"
)

func TestMockPackageDoesNotReintroduceLegacySnapshotExports(t *testing.T) {
	t.Helper()

	files := []string{
		"generator.go",
		"alert_history.go",
		"integration.go",
		"platform_fixtures.go",
		"recovery_points.go",
	}
	forbiddenSnippets := []string{
		"func GenerateMockData(",
		"func GenerateAlertHistory(",
		"func GetMockState(",
		"func GetMockRecoveryPoints(",
		"func GetPlatformFixtures(",
		"func DefaultPlatformFixtures(",
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		source := string(data)
		for _, snippet := range forbiddenSnippets {
			if strings.Contains(source, snippet) {
				t.Fatalf("%s must not reintroduce legacy mock export %q", path, snippet)
			}
		}
	}
}
