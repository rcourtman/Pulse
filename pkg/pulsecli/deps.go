package pulsecli

import (
	"os"

	"golang.org/x/term"
)

type ConfigDeps struct {
	ExportFile   *string
	ImportFile   *string
	Passphrase   *string
	ForceImport  *bool
	ReadPassword func(int) ([]byte, error)
}

type BootstrapDeps struct {
	Exit func(int)
}

type MockDeps struct {
	Exit          func(int)
	DefaultEnvDir func() string
	Stat          func(string) (os.FileInfo, error)
}

func configReadPassword(config *ConfigDeps, fd int) ([]byte, error) {
	if config != nil && config.ReadPassword != nil {
		return config.ReadPassword(fd)
	}
	return term.ReadPassword(fd)
}

func bootstrapExit(bootstrap *BootstrapDeps, code int) {
	if bootstrap != nil && bootstrap.Exit != nil {
		bootstrap.Exit(code)
		return
	}
	os.Exit(code)
}

func mockExit(mock *MockDeps, code int) {
	if mock != nil && mock.Exit != nil {
		mock.Exit(code)
		return
	}
	os.Exit(code)
}

func mockDefaultEnvDir(mock *MockDeps) string {
	if mock != nil && mock.DefaultEnvDir != nil {
		return mock.DefaultEnvDir()
	}
	return "/opt/pulse"
}

func mockStat(mock *MockDeps, path string) (os.FileInfo, error) {
	if mock != nil && mock.Stat != nil {
		return mock.Stat(path)
	}
	return os.Stat(path)
}
