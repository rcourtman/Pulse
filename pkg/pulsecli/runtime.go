package pulsecli

import (
	"os"

	"golang.org/x/term"
)

type Runtime struct {
	Config    ConfigRuntime
	Bootstrap BootstrapRuntime
	Mock      MockRuntime
}

type ConfigRuntime struct {
	ExportFile   *string
	ImportFile   *string
	Passphrase   *string
	ForceImport  *bool
	ReadPassword func(int) ([]byte, error)
}

type BootstrapRuntime struct {
	Exit func(int)
}

type MockRuntime struct {
	Exit          func(int)
	DefaultEnvDir func() string
	Stat          func(string) (os.FileInfo, error)
}

func configReadPassword(runtime *Runtime, fd int) ([]byte, error) {
	if runtime != nil && runtime.Config.ReadPassword != nil {
		return runtime.Config.ReadPassword(fd)
	}
	return term.ReadPassword(fd)
}

func bootstrapExit(runtime *Runtime, code int) {
	if runtime != nil && runtime.Bootstrap.Exit != nil {
		runtime.Bootstrap.Exit(code)
		return
	}
	os.Exit(code)
}

func mockExit(runtime *Runtime, code int) {
	if runtime != nil && runtime.Mock.Exit != nil {
		runtime.Mock.Exit(code)
		return
	}
	os.Exit(code)
}

func mockDefaultEnvDir(runtime *Runtime) string {
	if runtime != nil && runtime.Mock.DefaultEnvDir != nil {
		return runtime.Mock.DefaultEnvDir()
	}
	return "/opt/pulse"
}

func mockStat(runtime *Runtime, path string) (os.FileInfo, error) {
	if runtime != nil && runtime.Mock.Stat != nil {
		return runtime.Mock.Stat(path)
	}
	return os.Stat(path)
}
