package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")
	data := []byte("hello world")

	err := atomicWriteFile(file, data, 0644)
	if err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	read, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(read) != "hello world" {
		t.Errorf("content mismatch: got %s", read)
	}
}

func TestValidateAllowedNodesFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid dict",
			content: `
allowed_nodes:
  - node1
  - node2
`,
			wantErr: false,
		},
		{
			name: "valid list",
			content: `
- node1
- node2
`,
			wantErr: false,
		},
		{
			name:    "empty file",
			content: "",
			wantErr: false,
		},
		{
			name:    "empty dict",
			content: "allowed_nodes: []",
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			content: ": invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(tmpDir, tt.name+".yaml")
			err := os.WriteFile(file, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			err = validateAllowedNodesFile(file)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAllowedNodesFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetAllowedNodes(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "allowed_nodes.yaml")

	// Initial set
	err := setAllowedNodes(file, []string{"node1"}, false)
	if err != nil {
		t.Fatalf("setAllowedNodes failed: %v", err)
	}

	checkContent := func(expected []string) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		// Simple check for presence
		content := string(data)
		for _, e := range expected {
			if !strings.Contains(content, e) {
				t.Errorf("expected %s in %s", e, content)
			}
		}
	}

	checkContent([]string{"node1"})

	// Merge
	err = setAllowedNodes(file, []string{"node2"}, false)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	checkContent([]string{"node1", "node2"})

	// Replace
	err = setAllowedNodes(file, []string{"node3"}, true)
	if err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	// node1, node2 should be gone
	// Actual verification
	data, _ := os.ReadFile(file)
	nodesInFile := extractNodesFromYAML(data)
	if len(nodesInFile) != 1 || nodesInFile[0] != "node3" {
		t.Errorf("expected [node3], got %v", nodesInFile)
	}
}

func TestUpdateConfigMap(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "config.yaml")

	os.WriteFile(file, []byte("key: value\n"), 0644)

	err := updateConfigMap(file, func(m map[string]interface{}) error {
		m["new_key"] = "new_value"
		return nil
	})
	if err != nil {
		t.Fatalf("updateConfigMap failed: %v", err)
	}

	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "new_key: new_value") {
		t.Errorf("update failed, content: %s", string(data))
	}
}

func TestMigrateInlineToFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	nodesFile := filepath.Join(tmpDir, "allowed_nodes.yaml")

	// Scenario 1: No inline nodes, migration not needed
	os.WriteFile(configFile, []byte("allowed_nodes_file: "+nodesFile+"\n"), 0644)
	migrated, err := migrateInlineToFile(configFile, nodesFile)
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if migrated {
		t.Error("expected migrated=false")
	}

	// Scenario 2: Inline nodes present
	configContent := `
allowed_nodes:
  - inline1
  - inline2
`
	os.WriteFile(configFile, []byte(configContent), 0644)

	migrated, err = migrateInlineToFile(configFile, nodesFile)
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if !migrated {
		t.Error("expected migrated=true")
	}

	// Check config file has no allowed_nodes and has allowed_nodes_file
	data, _ := os.ReadFile(configFile)
	strData := string(data)
	if strings.Contains(strData, "allowed_nodes:") && !strings.Contains(strData, "allowed_nodes_file:") {
		t.Error("config not updated correctly")
	}

	// Check nodes file
	nodesData, _ := os.ReadFile(nodesFile)
	nodes := extractNodesFromYAML(nodesData)
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}
