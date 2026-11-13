package models

import (
	"testing"
	"time"
)

// A copy of a container must not share its failure log lines with the
// original: the state snapshot is read while the monitor writes the next one.
func TestCloneDockerContainerCopiesTheFailureLog(t *testing.T) {
	original := DockerContainer{ID: "c1", State: "exited", ExitCode: 1, FailureLog: &DockerContainerFailureLog{
		Reason: "exited", Lines: []string{"Error: invalid DATABASE_URL", "exiting"}, CapturedAt: time.Unix(1794500000, 0).UTC(),
	}}
	clone := cloneDockerContainer(original)
	if clone.FailureLog == nil || clone.FailureLog == original.FailureLog || len(clone.FailureLog.Lines) != 2 {
		t.Fatalf("clone log = %+v", clone.FailureLog)
	}
	clone.FailureLog.Lines[0] = "changed"
	if original.FailureLog.Lines[0] != "Error: invalid DATABASE_URL" {
		t.Fatal("changing the copy changed the original")
	}
	var none *DockerContainerFailureLog
	if none.Clone() != nil {
		t.Fatal("a missing log cloned into something")
	}
}
