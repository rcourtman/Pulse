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
	ExportFile  string
	ImportFile  string
	Passphrase  string
	ForceImport bool
}

type ProcessIO struct {
	Exit         func(int)
	ReadPassword func(int) ([]byte, error)
}

type MockFS struct {
	DefaultEnvDir string
	Stat          func(string) (os.FileInfo, error)
}

func NewEnv() *Env {
	return &Env{}
}

func NewProcessIO() ProcessIO {
	return ProcessIO{
		Exit:         os.Exit,
		ReadPassword: term.ReadPassword,
	}
}

func NewMockFS() MockFS {
	return MockFS{
		DefaultEnvDir: "/opt/pulse",
		Stat:          os.Stat,
	}
}

func (process ProcessIO) readPassword() func(int) ([]byte, error) {
	if process.ReadPassword != nil {
		return process.ReadPassword
	}
	return term.ReadPassword
}

func (process ProcessIO) exit() func(int) {
	if process.Exit != nil {
		return process.Exit
	}
	return os.Exit
}

func (mockFS MockFS) defaultEnvDir() func() string {
	if mockFS.DefaultEnvDir != "" {
		return func() string {
			return mockFS.DefaultEnvDir
		}
	}
	return func() string {
		return "/opt/pulse"
	}
}

func (mockFS MockFS) stat() func(string) (os.FileInfo, error) {
	if mockFS.Stat != nil {
		return mockFS.Stat
	}
	return os.Stat
}

func (env *Env) ConfigDeps(process ProcessIO) *ConfigDeps {
	if env == nil {
		env = NewEnv()
	}
	return NewConfigDeps(
		&env.ExportFile,
		&env.ImportFile,
		&env.Passphrase,
		&env.ForceImport,
		process.readPassword(),
	)
}

func BuildBootstrapDeps(process ProcessIO) *BootstrapDeps {
	return NewBootstrapDeps(process.exit())
}

func BuildMockDeps(process ProcessIO, mockFS MockFS) *MockDeps {
	return NewMockDeps(
		process.exit(),
		mockFS.defaultEnvDir(),
		mockFS.stat(),
	)
}

func (env *Env) CommandDeps(process ProcessIO, mockFS MockFS) CommandDeps {
	return CommandDeps{
		Config:    env.ConfigDeps(process),
		Bootstrap: BuildBootstrapDeps(process),
		Mock:      BuildMockDeps(process, mockFS),
	}
}

func (env *Env) ResetFlags() {
	ResetFlags(env.ConfigDeps(ProcessIO{}))
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
