package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileTools_Registry(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})
	tools := exec.registry.ListTools(InvocationPolicy{ControlLevel: ControlLevelControlled})

	found := false
	for _, tool := range tools {
		if tool.Name == "pulse_file_edit" {
			found = true
			break
		}
	}
	assert.False(t, found, "retired model file mutation tool must not be registered")
}
