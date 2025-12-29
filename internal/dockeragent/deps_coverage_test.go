package dockeragent

import (
	"context"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestDefaultDeps(t *testing.T) {
	swap(t, &goArch, "other")
	_ = determineSelfUpdateArch()

	if rc, err := openProcUptime(); err == nil {
		_ = rc.Close()
	}

	if cli, err := newDockerClientFn(); err == nil {
		_ = cli.Close()
	}

	agent := &Agent{logger: zerolog.Nop()}
	_ = selfUpdateFunc(agent, context.Background())
}

func TestUnameMachineError(t *testing.T) {
	prev := os.Getenv("PATH")
	_ = os.Setenv("PATH", "")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", prev)
	})

	_, _ = unameMachine()
}
