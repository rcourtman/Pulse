package hostagent

import (
	"context"
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

func TestHostUpdateStoreFailurePrecedesPackageManagerMutation(t *testing.T) {
	manager := newPackageUpdateManager("linux", newPackageManagerLease())
	calls := 0
	manager.run = func(context.Context, []string, string, ...string) packageUpdateCommandResult {
		calls++
		return packageUpdateCommandResult{}
	}
	client := &CommandClient{agentID: "agent-1", packageUpdates: manager, operationReceiptErr: errors.New("disk full"), logger: zerolog.Nop()}
	req := agentexec.HostUpdatePayload{RequestID: "a.dispatch.1", ActionID: "a", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := agentexec.BindHostUpdatePayload(&req); err != nil {
		t.Fatal(err)
	}
	client.handleHostUpdate(context.Background(), nil, req)
	if calls != 0 {
		t.Fatalf("package manager calls=%d", calls)
	}
}
