package dockeragent

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
)

var (
	connectRuntimeFn   = connectRuntime
	hostmetricsCollect = hostmetrics.Collect
	newTickerFn        = time.NewTicker
	newTimerFn         = time.NewTimer
	randomDurationFn   = randomDuration
	nowFn              = time.Now
	sleepFn            = time.Sleep
	buildRuntimeCandidatesFn = buildRuntimeCandidates
	tryRuntimeCandidateFn    = tryRuntimeCandidate
	osExecutableFn     = os.Executable
	osCreateTempFn     = os.CreateTemp
	osRenameFn         = os.Rename
	osChmodFn          = os.Chmod
	osRemoveFn         = os.Remove
	osReadFileFn       = os.ReadFile
	osWriteFileFn      = os.WriteFile
	osStatFn           = os.Stat
	syscallExecFn      = syscall.Exec
	goArch             = runtime.GOARCH
	unameMachine       = func() (string, error) {
		out, err := exec.Command("uname", "-m").Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	machineIDPaths = []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}
	unraidVersionPath = "/etc/unraid-version"
	unraidPersistPath = "/boot/config/plugins/pulse-docker-agent/pulse-docker-agent"
	openProcUptime    = func() (io.ReadCloser, error) {
		return os.Open("/proc/uptime")
	}
	newDockerClientFn = func(opts ...client.Opt) (dockerClient, error) {
		return client.NewClientWithOpts(opts...)
	}
	selfUpdateFunc = func(a *Agent, ctx context.Context) error {
		return a.selfUpdate(ctx)
	}
)
