package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGuestAgentDisabledCommandErrorReason(t *testing.T) {
	const fileReadPath = "/nodes/node/qemu/105/agent/file-read?file=%2Fproc%2Fmeminfo"
	const fsInfoPath = "/nodes/node/qemu/105/agent/get-fsinfo"
	const osInfoPath = "/nodes/node/qemu/105/agent/get-osinfo"

	cases := []struct {
		name     string
		path     string
		status   int
		body     string
		rejected bool
		disabled bool
		reason   string
	}{
		{
			name:     "file-open-disabled-plain",
			path:     fileReadPath,
			status:   http.StatusInternalServerError,
			body:     "VM 105 qmp command 'guest-file-open' failed - The command guest-file-open has been disabled for this instance",
			rejected: true,
			disabled: true,
			reason:   "agent-error",
		},
		{
			name:     "file-open-disabled-json",
			path:     fileReadPath,
			status:   http.StatusInternalServerError,
			body:     `{"data":null,"message":"VM 105 qmp command 'guest-file-open' failed - The command guest-file-open has been disabled for this instance\n"}`,
			rejected: true,
			disabled: true,
			reason:   "agent-error",
		},
		{
			name:     "file-read-disabled-json",
			path:     fileReadPath,
			status:   http.StatusInternalServerError,
			body:     `{"data":null,"message":"VM 105 qmp command 'guest-file-read' failed - The command guest-file-read has been disabled for this instance"}`,
			rejected: true,
			disabled: true,
			reason:   "agent-error",
		},
		{
			name:     "file-open-agent-not-running",
			path:     fileReadPath,
			status:   http.StatusInternalServerError,
			body:     `{"data":null,"message":"VM 105 qmp command 'guest-file-open' failed - QEMU guest agent is not running"}`,
			rejected: true,
			disabled: false,
			reason:   "agent-not-running",
		},
		{
			name:     "osinfo-disabled",
			path:     osInfoPath,
			status:   http.StatusInternalServerError,
			body:     `{"data":null,"message":"VM 105 qmp command 'guest-get-osinfo' failed - The command guest-get-osinfo has been disabled for this instance"}`,
			rejected: true,
			disabled: true,
			reason:   "agent-error",
		},
		{
			name:     "frozen-guest-stays-uncertain",
			path:     fileReadPath,
			status:   http.StatusInternalServerError,
			body:     `{"data":null,"message":"VM 105 qmp command 'guest-file-open' failed - Command guest-file-open has been disabled: the command is not allowed while guest is frozen"}`,
			rejected: false,
			disabled: false,
			reason:   "agent-error",
		},
		{
			name:     "wrong-vmid-stays-uncertain",
			path:     fileReadPath,
			status:   http.StatusInternalServerError,
			body:     `{"data":null,"message":"VM 106 qmp command 'guest-file-open' failed - The command guest-file-open has been disabled for this instance"}`,
			rejected: false,
			disabled: false,
			reason:   "agent-error",
		},
		{
			name:     "wrong-endpoint-stays-uncertain",
			path:     fsInfoPath,
			status:   http.StatusInternalServerError,
			body:     `{"data":null,"message":"VM 105 qmp command 'guest-file-open' failed - The command guest-file-open has been disabled for this instance"}`,
			rejected: false,
			disabled: false,
			reason:   "agent-error",
		},
		{
			name:     "gateway-502-quoting-disabled-stays-uncertain",
			path:     fileReadPath,
			status:   http.StatusBadGateway,
			body:     `{"data":null,"message":"VM 105 qmp command 'guest-file-open' failed - The command guest-file-open has been disabled for this instance"}`,
			rejected: false,
			disabled: false,
			reason:   "agent-error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason := guestAgentTerminalFailureReason(tc.path, []byte(tc.body))
			err := &apiResponseError{
				statusCode:           tc.status,
				guestCommandRejected: tc.status == http.StatusInternalServerError && reason != "",
				guestFailureReason:   reason,
				cause:                errors.New(tc.body),
			}
			wrapped := fmt.Errorf("wrapped: %w", err)
			if err.guestCommandRejected != tc.rejected {
				t.Fatalf("guestCommandRejected = %t, want %t (reason=%q)", err.guestCommandRejected, tc.rejected, reason)
			}
			if got := IsGuestAgentCommandDisabled(wrapped); got != tc.disabled {
				t.Fatalf("IsGuestAgentCommandDisabled = %t, want %t", got, tc.disabled)
			}
			if got := GuestAgentErrorReason(wrapped); got != tc.reason {
				t.Fatalf("GuestAgentErrorReason = %q, want %q", got, tc.reason)
			}
		})
	}
}

func TestGuestAgentFileReadDisabledDoesNotTriggerCooldownAndPreservesBackupLock(t *testing.T) {
	var locked atomic.Bool
	var agentCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			if locked.Load() {
				fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
			} else {
				fmt.Fprint(w, `{"data":{}}`)
			}
		case strings.HasSuffix(r.URL.Path, "/agent/file-read"):
			agentCalls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"data":null,"message":"VM 105 qmp command 'guest-file-open' failed - The command guest-file-open has been disabled for this instance\n"}`)
		case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
			agentCalls.Add(1)
			fmt.Fprint(w, `{"data":{"result":[{"name":"vda1","mountpoint":"/","type":"ext4","total-bytes":1000000,"used-bytes":400000}]}}`)
		case strings.HasSuffix(r.URL.Path, "/agent/get-osinfo"):
			agentCalls.Add(1)
			fmt.Fprint(w, `{"data":{"result":{"id":"debian","name":"Debian GNU/Linux","version-id":"12"}}}`)
		case strings.HasSuffix(r.URL.Path, "/agent/info"):
			agentCalls.Add(1)
			fmt.Fprint(w, `{"data":{"result":{"version":"10.1.0","supported_commands":[{"name":"guest-ping","enabled":true,"success-response":true},{"name":"guest-get-fsinfo","enabled":true,"success-response":true},{"name":"guest-file-open","enabled":false,"success-response":true},{"name":"guest-file-read","enabled":false,"success-response":true}]}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := backupTestClient(t, guestAgentFixtureEndpoint(t, server.URL))

	// 1. Disabled guest-file-open HTTP 500 must be a terminal rejection, not ErrGuestAgentDeferred.
	if _, err := c.GetVMMemoryAvailabilityFromAgent(context.Background(), "node", 105); err == nil {
		t.Fatal("expected error from disabled file-read")
	} else {
		if errors.Is(err, ErrGuestAgentDeferred) {
			t.Fatalf("disabled guest-file-open triggered deferral/cooldown: %v", err)
		}
		if !IsGuestAgentCommandDisabled(err) {
			t.Fatalf("expected IsGuestAgentCommandDisabled(err)=true, got false: %v", err)
		}
		if reason := GuestAgentErrorReason(err); reason != "agent-error" {
			t.Fatalf("GuestAgentErrorReason = %q, want agent-error", reason)
		}
	}

	// 2. Subsequent QGA reads on the same VM must succeed immediately (no 60s agent-cooldown).
	fs, err := c.GetVMFSInfo(context.Background(), "node", 105)
	if err != nil || len(fs) != 1 || fs[0].Mountpoint != "/" {
		t.Fatalf("GetVMFSInfo after disabled file-read = %+v, %v", fs, err)
	}
	osInfo, err := c.GetVMAgentInfo(context.Background(), "node", 105)
	if err != nil || osInfo == nil {
		t.Fatalf("GetVMAgentInfo after disabled file-read = %+v, %v", osInfo, err)
	}
	summary, err := c.GetVMAgentInfoSummary(context.Background(), "node", 105)
	if err != nil {
		t.Fatalf("GetVMAgentInfoSummary failed: %v", err)
	}
	if summary.Version != "10.1.0" || !summary.HasSupportedCommands || !summary.FileReadDisabled {
		t.Fatalf("GetVMAgentInfoSummary = %+v, want version 10.1.0, HasSupportedCommands=true, FileReadDisabled=true", summary)
	}

	// 3. Backup lock must still block GetVMAgentInfoSummary and file-read before any QGA call.
	beforeLocked := agentCalls.Load()
	locked.Store(true)
	if _, err := c.GetVMAgentInfoSummary(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "vm-locked" {
		t.Fatalf("locked GetVMAgentInfoSummary err = %v, want vm-locked", err)
	}
	if _, err := c.GetVMMemoryAvailabilityFromAgent(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "vm-locked" {
		t.Fatalf("locked GetVMMemoryAvailabilityFromAgent err = %v, want vm-locked", err)
	}
	if agentCalls.Load() != beforeLocked {
		t.Fatalf("agent calls increased while VM was backup-locked: %d -> %d", beforeLocked, agentCalls.Load())
	}
}

func TestParseVMAgentInfoResultBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want VMAgentInfo
	}{
		{
			name: "enabled-file-commands",
			raw:  `{"version":"10.1.0","supported_commands":[{"name":"guest-file-open","enabled":true},{"name":"guest-file-read","enabled":true}]}`,
			want: VMAgentInfo{Version: "10.1.0", HasSupportedCommands: true, FileReadDisabled: false},
		},
		{
			name: "only-file-read-disabled",
			raw:  `{"version":"10.1.0","supported_commands":[{"name":"guest-file-open","enabled":true},{"name":"guest-file-read","enabled":false}]}`,
			want: VMAgentInfo{Version: "10.1.0", HasSupportedCommands: true, FileReadDisabled: true},
		},
		{
			name: "hyphenated-key",
			raw:  `{"qemu-ga":{"version":"9.2.0"},"supported-commands":[{"name":"guest-file-open","enabled":false}]}`,
			want: VMAgentInfo{Version: "9.2.0", HasSupportedCommands: true, FileReadDisabled: true},
		},
		{
			name: "omitted-supported-commands",
			raw:  `{"version":"10.1.0"}`,
			want: VMAgentInfo{Version: "10.1.0", HasSupportedCommands: false, FileReadDisabled: false},
		},
		{
			name: "null-supported-commands",
			raw:  `{"version":"10.1.0","supported_commands":null}`,
			want: VMAgentInfo{Version: "10.1.0", HasSupportedCommands: false, FileReadDisabled: false},
		},
		{
			name: "empty-supported-commands",
			raw:  `{"version":"10.1.0","supported_commands":[]}`,
			want: VMAgentInfo{Version: "10.1.0", HasSupportedCommands: true, FileReadDisabled: false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseVMAgentInfoResult([]byte(tc.raw)); got != tc.want {
				t.Fatalf("parseVMAgentInfoResult = %+v, want %+v", got, tc.want)
			}
		})
	}
}
