//go:build linux

package hostagent

import (
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// A mock Collector does not cover the independently composed package managers.
// The command identity fixture must never inspect its test host's packages or
// cache before it can exercise a report acknowledgement. Trap both the Linux
// report-only inspector and the older manager lookup without running a tool.
func TestCommandIdentityHarnessDoesNotInspectTheHost(t *testing.T) {
	oldInspector, oldLookup := packageInspectionResolveAPT, packageUpdateLookPath
	var inspections, lookups atomic.Int32
	packageInspectionResolveAPT = func() (string, error) {
		inspections.Add(1)
		return "", os.ErrNotExist
	}
	packageUpdateLookPath = func(string) (string, error) {
		lookups.Add(1)
		return "", os.ErrNotExist
	}
	t.Cleanup(func() {
		packageInspectionResolveAPT, packageUpdateLookPath = oldInspector, oldLookup
	})

	h := newCommandIdentityHarness(t, "configured-id", time.Hour, func(_ int, w http.ResponseWriter) {
		writeAck(w, `{"success":true,"agentId":"server-resolved","commandAgentId":"server-resolved"}`)
	})
	h.run(t)
	h.expectStart(t, "server-resolved")
	if got := inspections.Load(); got != 0 {
		t.Errorf("identity fixture reached the host's package inspector %d times", got)
	}
	if got := lookups.Load(); got != 0 {
		t.Errorf("identity fixture looked up a host package tool %d times", got)
	}
}
