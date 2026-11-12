//go:build unix

package agenthelper_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
)

// This is a framed synthetic peer, not a root helper, container daemon or real
// updater. The first accepted operation deliberately finishes after its caller
// has canceled. Closing a local wait must never be mistaken for remote rollback.
func helperContextPeer(t *testing.T, expectedOperation string, version int) (string, <-chan agenthelper.Request, func(), <-chan struct{}) {
	t.Helper()
	// Keep the Unix socket well below sockaddr_un's platform-specific path limit.
	dir, err := os.MkdirTemp("", "pulse-helper-context-")
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "helper.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		_ = os.RemoveAll(dir)
		t.Fatal(err)
	}
	release := make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	requests := make(chan agenthelper.Request, 3)
	firstFinished, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for index := 0; index < 2; index++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				var header [4]byte
				if _, err := io.ReadFull(conn, header[:]); err != nil {
					t.Errorf("read request header: %v", err)
					return
				}
				length := binary.BigEndian.Uint32(header[:])
				if length == 0 || length > agenthelper.MaxRequestBytes {
					t.Errorf("invalid request size %d", length)
					return
				}
				payload := make([]byte, length)
				if _, err := io.ReadFull(conn, payload); err != nil {
					t.Errorf("read request: %v", err)
					return
				}
				var request agenthelper.Request
				if err := json.Unmarshal(payload, &request); err != nil {
					t.Errorf("decode request: %v", err)
					return
				}
				requests <- request
				if index == 0 {
					<-release
					close(firstFinished)
					if request.Operation != expectedOperation || request.OperationVersion != version {
						t.Errorf("unexpected consuming request: %+v", request)
					}
				} else if request.Operation != agenthelper.OperationHealth {
					t.Errorf("client retried an abandoned operation instead of the independent health control: %s", request.Operation)
				}
				response := agenthelper.Response{
					ProtocolVersion: agenthelper.ProtocolVersion, RequestID: request.RequestID,
					Operation: request.Operation, OperationVersion: request.OperationVersion,
					Success: true, Result: json.RawMessage(`{"status":"ok","protocolVersion":1}`),
				}
				encoded, err := json.Marshal(response)
				if err != nil {
					t.Error(err)
					return
				}
				framed := make([]byte, 4+len(encoded))
				binary.BigEndian.PutUint32(framed[:4], uint32(len(encoded)))
				copy(framed[4:], encoded)
				_, err = io.Copy(conn, bytes.NewReader(framed))
				if index == 1 && err != nil {
					t.Errorf("write health control: %v", err)
				}
			}()
		}
	}()
	t.Cleanup(func() {
		finish()
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(4 * time.Second):
			t.Error("synthetic helper peer did not stop")
		}
		_ = os.RemoveAll(dir)
	})
	return socket, requests, finish, firstFinished
}

func TestHelperConsumersCancelWithoutReissue(t *testing.T) {
	for _, test := range []struct {
		name, operation string
		version         int
	}{
		{"health", agenthelper.OperationHealth, 1},
		{"smart", agenthelper.OperationSMARTSnapshot, 1},
		{"proxmox", agenthelper.OperationProxmoxLXCFilesystems, 2},
		{"containers", agenthelper.OperationContainerInventory, 1},
		{"stage", agenthelper.OperationAgentUpdateStage, 1},
		{"activate", agenthelper.OperationAgentUpdateActivate, 1},
		{"commit", agenthelper.OperationAgentUpdateCommit, 1},
		{"rollback", agenthelper.OperationAgentUpdateRollback, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			socket, requests, finish, remoteFinished := helperContextPeer(t, test.operation, test.version)
			telemetry, err := hostagent.NewPrivilegeHelperTelemetry(socket)
			if err != nil {
				t.Fatal(err)
			}
			containers, err := dockeragent.NewPrivilegeHelperContainerInventory(socket)
			if err != nil {
				t.Fatal(err)
			}
			updater, err := agentupdate.NewPrivilegeHelperUpdate(socket)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var callErr error
				switch test.name {
				case "health":
					callErr = telemetry.Health(ctx)
				case "smart":
					_, callErr = telemetry.SMARTSnapshot(ctx)
				case "proxmox":
					_, callErr = telemetry.ProxmoxLXCFilesystems(ctx)
				case "containers":
					_, callErr = containers.Inventory(ctx)
				case "stage":
					_, callErr = updater.Stage(ctx, "synthetic-artifact", "synthetic-digest", "test-version")
				case "activate":
					_, callErr = updater.Activate(ctx, "synthetic-artifact", "synthetic-digest", "test-version")
				case "commit":
					_, callErr = updater.Commit(ctx, agenthelper.UpdateResult{ActivationID: "synthetic-activation"})
				case "rollback":
					_, callErr = updater.Rollback(ctx, agenthelper.UpdateResult{ActivationID: "synthetic-activation"})
				}
				done <- callErr
			}()
			var admitted agenthelper.Request
			select {
			case admitted = <-requests:
			case <-time.After(time.Second):
				t.Fatal("consumer did not reach the framed Unix peer")
			}
			if admitted.ProtocolVersion != agenthelper.ProtocolVersion || admitted.RequestID == "" || admitted.DeadlineMillis <= 0 || admitted.DeadlineMillis > 30000 {
				t.Fatalf("invalid consuming envelope: %+v", admitted)
			}
			started := time.Now()
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("consumer lost cancellation identity: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("consumer kept waiting for a 30-second helper deadline")
			}
			t.Logf("%s returned cancellation in %s; one request admitted", test.operation, time.Since(started))
			select {
			case <-remoteFinished:
				t.Fatal("fixture remote completion preceded deliberate release")
			default:
			}
			finish()
			select {
			case <-remoteFinished: // Remote completion remains possible, not canceled or rolled back by the client.
			case <-time.After(time.Second):
				t.Fatal("synthetic remote operation did not finish independently")
			}
			controlCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
			defer stop()
			if err := telemetry.Health(controlCtx); err != nil {
				t.Fatalf("independent health control after cancellation: %v", err)
			}
			select {
			case control := <-requests:
				if control.Operation != agenthelper.OperationHealth || control.RequestID == admitted.RequestID {
					t.Fatalf("not an independent health control: %+v", control)
				}
			case <-time.After(time.Second):
				t.Fatal("missing independent health request")
			}
			select {
			case extra := <-requests:
				t.Fatalf("unexpected automatic helper request: %+v", extra)
			default:
			}
		})
	}
}
