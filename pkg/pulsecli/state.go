package pulsecli

import (
	"os"

	"golang.org/x/term"
)

type State struct {
	ExportFile        *string
	ImportFile        *string
	Passphrase        *string
	ForceImport       *bool
	ExitFunc          *func(int)
	ReadPassword      *func(int) ([]byte, error)
	MockEnvDefaultDir *string
	MockEnvStat       *func(string) (os.FileInfo, error)
}

func stateExit(state *State, code int) {
	if state != nil && state.ExitFunc != nil && *state.ExitFunc != nil {
		(*state.ExitFunc)(code)
		return
	}
	os.Exit(code)
}

func stateReadPassword(state *State, fd int) ([]byte, error) {
	if state != nil && state.ReadPassword != nil && *state.ReadPassword != nil {
		return (*state.ReadPassword)(fd)
	}
	return term.ReadPassword(fd)
}

func stateMockEnvDefaultDir(state *State) string {
	if state != nil && state.MockEnvDefaultDir != nil && *state.MockEnvDefaultDir != "" {
		return *state.MockEnvDefaultDir
	}
	return "/opt/pulse"
}

func stateMockStat(state *State, path string) (os.FileInfo, error) {
	if state != nil && state.MockEnvStat != nil && *state.MockEnvStat != nil {
		return (*state.MockEnvStat)(path)
	}
	return os.Stat(path)
}
