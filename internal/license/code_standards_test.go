package license

// Code standards tests: these act as linter rules that run in CI.
// They scan source files for hardcoded grace period values.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestNoHardcodedGracePeriod ensures no file in the license package hardcodes
// the 7-day grace period duration instead of using DefaultGracePeriod.
func TestNoHardcodedGracePeriod(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read license directory: %v", err)
	}

	// Match "7 * 24 * time.Hour" or "168 * time.Hour" (7*24=168)
	hardcoded := regexp.MustCompile(`(?:7\s*\*\s*24\s*\*\s*time\.Hour|168\s*\*\s*time\.Hour)`)

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		content := string(data)

		matches := hardcoded.FindAllStringIndex(content, -1)
		for _, m := range matches {
			line := 1 + strings.Count(content[:m[0]], "\n")
			// Get the full line containing the match
			lineStart := strings.LastIndex(content[:m[0]], "\n") + 1
			lineEnd := strings.Index(content[m[0]:], "\n")
			var fullLine string
			if lineEnd >= 0 {
				fullLine = content[lineStart : m[0]+lineEnd]
			} else {
				fullLine = content[lineStart:]
			}
			// Allow the constant definition itself
			if strings.Contains(fullLine, "DefaultGracePeriod") {
				continue
			}
			t.Errorf("%s:%d: hardcoded grace period duration — use DefaultGracePeriod constant instead", name, line)
		}
	}
}
