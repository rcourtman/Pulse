package main

import (
	"encoding/base64"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
	"github.com/stretchr/testify/assert"
)

func TestVersionCmd(t *testing.T) {
	oldVersion := Version
	oldBuildTime := BuildTime
	oldGitCommit := GitCommit
	defer func() {
		Version = oldVersion
		BuildTime = oldBuildTime
		GitCommit = oldGitCommit
	}()

	Version = "1.2.3"
	BuildTime = "2023-01-01"
	GitCommit = "abcdef"

	output := captureOutput(func() {
		cmd := newProgram(newTestCLIEnv(), newTestCLIProcess()).RootCommand()
		cmd.SetArgs([]string{"version"})
		_ = cmd.Execute()
	})

	assert.Contains(t, output, "Pulse 1.2.3")
	assert.Contains(t, output, "Built: 2023-01-01")
	assert.Contains(t, output, "Commit: abcdef")

	BuildTime = "unknown"
	GitCommit = "unknown"
	output = captureOutput(func() {
		cmd := newProgram(newTestCLIEnv(), newTestCLIProcess()).RootCommand()
		cmd.SetArgs([]string{"version"})
		_ = cmd.Execute()
	})
	assert.Contains(t, output, "Pulse 1.2.3")
	assert.NotContains(t, output, "Built:")
	assert.NotContains(t, output, "Commit:")
}

func TestNormalizeImportPayload(t *testing.T) {
	_, err := server.NormalizeImportPayload([]byte("  "))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration payload is empty")

	s, err := server.NormalizeImportPayload([]byte(" ISE= "))
	assert.NoError(t, err)
	assert.Equal(t, "ISE=", s)

	s, err = server.NormalizeImportPayload([]byte(" dGVzdA== "))
	assert.NoError(t, err)
	assert.Equal(t, "test", s)

	s, err = server.NormalizeImportPayload([]byte("!!"))
	assert.NoError(t, err)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("!!")), s)
}
