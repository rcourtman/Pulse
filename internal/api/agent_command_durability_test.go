package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/stretchr/testify/require"
)

// Couple the real saved-authority verifier to the real agent transport. No
// shell command is executed: the synthetic peer only reads/signature-checks a
// command frame and supplies a fixed result.
func TestCommandApprovalDurabilityAtAgentDispatch(t *testing.T) {
	for _, failPublication := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved_before_frame", true: "failed_save_no_frame"}[failPublication], func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "store")
			require.NoError(t, os.Mkdir(dir, 0700))
			req := commandAuthorizationApproval("fixture-durable")
			store := installCommandAuthorizationStoreAt(t, dir, req)
			original, err := os.ReadFile(filepath.Join(dir, "ai_approvals.json"))
			require.NoError(t, err)

			const token = "synthetic-durability-token"
			server := agentexec.NewServerWithAdmissionValidator(func(raw, id, hostname string) (agentexec.AgentAdmission, bool) {
				return agentexec.AgentAdmission{OrganizationID: "org-1", AgentID: id, Hostname: hostname}, raw == token && id == "agent-1"
			}, func(agentexec.AgentAdmission) bool { return true })
			server.SetCommandAuthorizationVerifier(verifyAndConsumeCommandAuthorization)
			ts := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
			t.Cleanup(ts.Close)
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), http.Header{"Origin": []string{ts.URL}})
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })
			write := func(kind agentexec.MessageType, payload any) {
				t.Helper()
				msg, err := agentexec.NewMessage(kind, "", payload)
				require.NoError(t, err)
				require.NoError(t, conn.SetWriteDeadline(time.Now().Add(3*time.Second)))
				require.NoError(t, conn.WriteJSON(msg))
			}
			read := func() agentexec.Message {
				t.Helper()
				require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
				var msg agentexec.Message
				require.NoError(t, conn.ReadJSON(&msg))
				return msg
			}
			write(agentexec.MsgTypeAgentRegister, agentexec.AgentRegisterPayload{
				AgentID: "agent-1", Hostname: "fixture-host", Version: "fixture", Platform: "linux", Token: token,
			})
			require.Equal(t, agentexec.MsgTypeRegistered, read().Type)

			savedDir := dir
			if failPublication {
				savedDir = filepath.Join(parent, "preserved")
				require.NoError(t, os.Rename(dir, savedDir))
				require.NoError(t, os.WriteFile(dir, []byte("synthetic unavailable storage"), 0600))
				t.Cleanup(func() {
					require.NoError(t, os.Remove(dir))
					require.NoError(t, os.Rename(savedDir, dir))
				})
			}
			payload := agentexec.ExecuteCommandPayload{
				RequestID: "fixture-first", Command: req.Command, ApprovalID: req.ID,
				TargetType: req.TargetType, TargetID: req.TargetID, Timeout: 1,
			}
			payload.BindCommandAuthorization("org-1", "action-1")
			ctx, cancel := context.WithTimeout(agentexec.WithOrganizationID(context.Background(), "org-1"), 3*time.Second)
			defer cancel()
			type returned struct {
				result *agentexec.CommandResultPayload
				err    error
			}
			done := make(chan returned, 1)
			go func() { result, err := server.ExecuteCommand(ctx, "agent-1", payload); done <- returned{result, err} }()
			if failPublication {
				out := <-done
				require.Nil(t, out.result)
				require.ErrorContains(t, out.err, "approval consumption could not be saved")
				require.NotContains(t, out.err.Error(), parent)
				kept, err := os.ReadFile(filepath.Join(savedDir, "ai_approvals.json"))
				require.NoError(t, err)
				require.Equal(t, original, kept)
				local, found := store.GetApproval(req.ID)
				require.True(t, found)
				require.True(t, local.Consumed)
			} else {
				msg := read()
				require.Equal(t, agentexec.MsgTypeExecuteCmd, msg.Type)
				var delivered agentexec.ExecuteCommandPayload
				require.NoError(t, json.Unmarshal(msg.Payload, &delivered))
				require.NoError(t, agentexec.VerifyCommandApprovalGrant(token, "agent-1", delivered, time.Now()))
				// At the first command frame, a fresh store must already see the
				// spent decision, without Flush or a debounce wait.
				reloaded, err := approval.NewStore(approval.StoreConfig{DataDir: dir})
				require.NoError(t, err)
				saved, found := reloaded.GetApproval(req.ID)
				require.True(t, found)
				require.True(t, saved.Consumed)
				write(agentexec.MsgTypeCommandResult, agentexec.CommandResultPayload{
					RequestID: delivered.RequestID, Success: true, Stdout: "synthetic result", ExitCode: 0,
				})
				out := <-done
				require.NoError(t, out.err)
				require.NotNil(t, out.result)
				require.True(t, out.result.Success)
				approval.SetStore(reloaded)
				payload.RequestID = "fixture-after-reload"
				result, err := server.ExecuteCommand(ctx, "agent-1", payload)
				require.Nil(t, result)
				require.ErrorContains(t, err, "already been consumed")
			}
			// ExecuteCommand has returned. This connected peer must receive no
			// command on failed publication, or on replay after the real reload.
			require.Len(t, server.GetConnectedAgentsForOrganization("org-1"), 1)
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
			var unexpected agentexec.Message
			err = conn.ReadJSON(&unexpected)
			var timeout net.Error
			require.ErrorAs(t, err, &timeout)
			require.True(t, timeout.Timeout(), "no-frame proof needs a live socket read timeout, not an unrelated close")
		})
	}
}
