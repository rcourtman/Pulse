package config

// Code standards tests: these act as linter rules that run in CI.
// They scan source files for anti-patterns in the persistence layer.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoPersistenceBoilerplate ensures new Save*/Load* methods on ConfigPersistence
// use the saveJSON/loadSlice generic helpers instead of reimplementing the
// lock→marshal→encrypt→write boilerplate. Methods that need domain-specific logic
// are allowlisted.
func TestNoPersistenceBoilerplate(t *testing.T) {
	// Methods with domain-specific logic that legitimately can't use the generics.
	allowedMethods := map[string]bool{
		"SaveAlertConfig":           true, // 400+ lines of validation
		"SaveAPITokens":             true, // backup + sanitize
		"SaveNodesConfig":           true, // wipe guard, multi-type
		"SaveNodesConfigAllowEmpty": true, // wipe guard variant
		"SaveSystemSettings":        true, // .env file update
		"SaveOIDCConfig":            true, // clears EnvOverrides
		"SaveSSOConfig":             true, // nil guard, clone, clear overrides
		"SaveAppriseConfig":         true, // normalize config
		"SaveOrganization":          true, // uses fs.WriteFile directly
		"SaveEmailConfig":           true, // domain-specific logging
		"SaveWebhooks":              true, // domain-specific logging with count
		"SaveAIConfig":              true, // complex AI-specific logic
		"SaveAIFindings":            true, // versioned envelope
		"SaveAIChatSessions":        true, // versioned envelope
		"SaveAIChatSession":         true, // delegates to SaveAIChatSessions
		"SaveAIUsageHistory":        true, // versioned envelope
		"SavePatrolRunHistory":      true, // versioned envelope
	}

	data, err := os.ReadFile("persistence.go")
	if err != nil {
		t.Fatalf("failed to read persistence.go: %v", err)
	}
	content := string(data)

	// Find all Save* method definitions on ConfigPersistence
	saveMethodRe := regexp.MustCompile(`func \(c \*ConfigPersistence\) (Save\w+)\(`)
	matches := saveMethodRe.FindAllStringSubmatch(content, -1)

	for _, m := range matches {
		methodName := m[1]
		if allowedMethods[methodName] {
			continue
		}

		// Find the method body: from the func declaration to the next func declaration or EOF
		funcStart := strings.Index(content, m[0])
		if funcStart < 0 {
			continue
		}

		// Find the closing brace by looking for the next top-level func
		rest := content[funcStart+len(m[0]):]
		nextFunc := regexp.MustCompile(`\nfunc `).FindStringIndex(rest)
		var methodBody string
		if nextFunc != nil {
			methodBody = rest[:nextFunc[0]]
		} else {
			methodBody = rest
		}

		// Check if this method uses saveJSON (the generic helper)
		if !strings.Contains(methodBody, "saveJSON(") && !strings.Contains(methodBody, "saveJSON[") {
			// Method doesn't use the generic — check if it has the boilerplate pattern
			if strings.Contains(methodBody, "json.Marshal") || strings.Contains(methodBody, "json.MarshalIndent") {
				line := 1 + strings.Count(content[:funcStart], "\n")
				t.Errorf("persistence.go:%d: %s() contains JSON marshal boilerplate — use saveJSON() generic helper or add to allowlist in code_standards_test.go", line, methodName)
			}
		}
	}
}

// TestNoPersistenceLoadBoilerplate does the same check for Load* methods.
func TestNoPersistenceLoadBoilerplate(t *testing.T) {
	// Methods with domain-specific logic that legitimately can't use generic loaders.
	allowedMethods := map[string]bool{
		"LoadAlertConfig":      true, // complex validation
		"LoadAPITokens":        true, // ensureScopes post-processing
		"LoadNodesConfig":      true, // multi-type
		"LoadSystemSettings":   true, // complex with defaults
		"LoadOIDCConfig":       true, // complex with defaults
		"LoadSSOConfig":        true, // legacy migration
		"LoadAppriseConfig":    true, // complex with defaults
		"LoadEmailConfig":      true, // complex with defaults
		"LoadWebhooks":         true, // legacy migration
		"LoadOrganization":     true, // different error handling
		"LoadAIConfig":         true, // complex migration logic
		"LoadAIFindings":       true, // versioned envelope
		"LoadAIChatSessions":   true, // versioned envelope
		"LoadAIUsageHistory":   true, // versioned envelope
		"LoadPatrolRunHistory": true, // versioned envelope
		"LoadGuestMetadata":    true, // metadata store
		"LoadDockerMetadata":   true, // metadata store
		"LoadHostMetadata":     true, // metadata store
	}

	data, err := os.ReadFile("persistence.go")
	if err != nil {
		t.Fatalf("failed to read persistence.go: %v", err)
	}
	content := string(data)

	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	// Also scan any additional persistence files
	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, "persistence") && filepath.Ext(name) == ".go" && !strings.HasSuffix(name, "_test.go") && name != "persistence.go" {
			extra, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}
			content += "\n" + string(extra)
		}
	}

	loadMethodRe := regexp.MustCompile(`func \(c \*ConfigPersistence\) (Load\w+)\(`)
	matches := loadMethodRe.FindAllStringSubmatch(content, -1)

	for _, m := range matches {
		methodName := m[1]
		if allowedMethods[methodName] {
			continue
		}

		funcStart := strings.Index(content, m[0])
		if funcStart < 0 {
			continue
		}

		rest := content[funcStart+len(m[0]):]
		nextFunc := regexp.MustCompile(`\nfunc `).FindStringIndex(rest)
		var methodBody string
		if nextFunc != nil {
			methodBody = rest[:nextFunc[0]]
		} else {
			methodBody = rest
		}

		if !strings.Contains(methodBody, "loadSlice[") &&
			!strings.Contains(methodBody, "loadSlice(") &&
			!strings.Contains(methodBody, "loadJSON[") &&
			!strings.Contains(methodBody, "loadJSON(") {
			if strings.Contains(methodBody, "json.Unmarshal") {
				line := 1 + strings.Count(content[:funcStart], "\n")
				t.Errorf("persistence.go:%d: %s() contains JSON unmarshal boilerplate — use loadSlice()/loadJSON() generic helper or add to allowlist in code_standards_test.go", line, methodName)
			}
		}
	}
}
