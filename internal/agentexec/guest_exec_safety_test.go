package agentexec

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func testVMGuestExecCapabilityGate(t *testing.T) {
	for _, tc := range []struct {
		version  int
		platform string
		admitted bool
	}{
		{0, "linux", false}, {1, "linux", true}, {2, "linux", false}, {1, "windows", false},
	} {
		t.Run(fmt.Sprintf("%s/version-%d", tc.platform, tc.version), func(t *testing.T) {
			s := NewServer(allowAllTestTokens)
			ts := newWSServer(t, s)
			defer ts.Close()
			conn, _, err := dialAgentExecWebSocket(t, ts.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{AgentID: "node-agent", Hostname: "node", Version: "future-version-is-not-capability", Platform: tc.platform, Token: "fixture", GuestExecGuardVersion: tc.version}))
			if !wsReadRegisteredPayload(t, conn).Success {
				t.Fatal("registration rejected")
			}
			cmd := ExecuteCommandPayload{RequestID: "scan", Command: "echo inspected", TargetType: "vm", TargetID: "105", Trusted: true, Timeout: 2}
			if !tc.admitted {
				if result, err := s.ExecuteCommand(context.Background(), "node-agent", cmd); result != nil || err == nil || !IsGuestExecDeferred(err.Error()) {
					t.Fatalf("legacy target dispatched: result=%#v err=%v", result, err)
				}
				if result, err := s.ReadFile(context.Background(), "node-agent", ReadFilePayload{RequestID: "read", Path: "/etc/os-release", TargetType: "vm", TargetID: "105"}); result != nil || err == nil || !IsGuestExecDeferred(err.Error()) {
					t.Fatalf("legacy file target dispatched: result=%#v err=%v", result, err)
				}
				// The first message must be the pong, not an unsafe VM dispatch.
				wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentPing, "", nil))
				if msg := wsReadRawMessage(t, conn); msg.Type != MsgTypePong {
					t.Fatalf("unsafe message queued: %s", msg.Type)
				}
				return
			}
			type outcome struct {
				result *CommandResultPayload
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				result, err := s.ExecuteCommand(context.Background(), "node-agent", cmd)
				done <- outcome{result, err}
			}()
			msg, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
			if err != nil || msg.Type != MsgTypeExecuteCmd || msg.Payload == nil {
				t.Fatalf("dispatch=%#v err=%v", msg, err)
			}
			var payload ExecuteCommandPayload
			if err := json.Unmarshal(*msg.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.TargetID != "105" || payload.TargetType != "vm" || payload.Command != cmd.Command || !payload.Trusted {
				t.Fatalf("target changed: %#v", payload)
			}
			wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeCommandResult, payload.RequestID, CommandResultPayload{RequestID: payload.RequestID, Success: false, ExitCode: -1, Error: GuestExecDeferred(GuestExecVMLocked).Error()}))
			got := <-done
			if got.err != nil || got.result == nil || got.result.Success || !IsGuestExecDeferred(got.result.Error) {
				t.Fatalf("deferral lost: %#v %v", got.result, got.err)
			}
		})
	}
}
