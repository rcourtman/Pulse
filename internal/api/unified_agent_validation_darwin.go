package api

import (
	"os"
	"syscall"
)

func agentArtifactChangeTime(info os.FileInfo) (agentArtifactChangeStamp, bool) {
	value, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return agentArtifactChangeStamp{}, false
	}
	return agentArtifactChangeStamp{seconds: int64(value.Ctimespec.Sec), nanoseconds: int64(value.Ctimespec.Nsec)}, true
}
