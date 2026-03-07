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

type Env struct {
	ExportFile        string
	ImportFile        string
	Passphrase        string
	ForceImport       bool
	Exit              func(int)
	ReadPassword      func(int) ([]byte, error)
	MockEnvDefaultDir string
	MockEnvStat       func(string) (os.FileInfo, error)
}

func NewEnv() *Env {
	return &Env{
		Exit:              os.Exit,
		ReadPassword:      term.ReadPassword,
		MockEnvDefaultDir: "/opt/pulse",
		MockEnvStat:       os.Stat,
	}
}

func (env *Env) ConfigDeps() *ConfigDeps {
	if env == nil {
		env = NewEnv()
	}
	return NewConfigDeps(
		&env.ExportFile,
		&env.ImportFile,
		&env.Passphrase,
		&env.ForceImport,
		env.ReadPassword,
	)
}

func (env *Env) BootstrapDeps() *BootstrapDeps {
	if env == nil {
		env = NewEnv()
	}
	return NewBootstrapDeps(env.Exit)
}

func (env *Env) MockDeps() *MockDeps {
	if env == nil {
		env = NewEnv()
	}
	return NewMockDeps(
		env.Exit,
		func() string {
			return env.MockEnvDefaultDir
		},
		env.MockEnvStat,
	)
}

func (env *Env) CommandDeps() CommandDeps {
	return CommandDeps{
		Config:    env.ConfigDeps(),
		Bootstrap: env.BootstrapDeps(),
		Mock:      env.MockDeps(),
	}
}

func (env *Env) ResetFlags() {
	ResetFlags(env.ConfigDeps())
}

func NewConfigDeps(exportFile, importFile, passphrase *string, forceImport *bool, readPassword func(int) ([]byte, error)) *ConfigDeps {
	return &ConfigDeps{
		ExportFile:   exportFile,
		ImportFile:   importFile,
		Passphrase:   passphrase,
		ForceImport:  forceImport,
		ReadPassword: readPassword,
	}
}

func NewBootstrapDeps(exit func(int)) *BootstrapDeps {
	return &BootstrapDeps{Exit: exit}
}

func NewMockDeps(exit func(int), defaultEnvDir func() string, stat func(string) (os.FileInfo, error)) *MockDeps {
	return &MockDeps{
		Exit:          exit,
		DefaultEnvDir: defaultEnvDir,
		Stat:          stat,
	}
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
