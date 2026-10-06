package proxmox

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestGuestAgentErrorReasonKeepsWireAndCommandEvidence(t *testing.T) {
	const path = "/nodes/node/qemu/105/agent/get-fsinfo"
	cases := []struct {
		name, body string
		status     int
		want       string
		rejected   bool
	}{
		{"stopped", `{"message":"QEMU guest agent is not running"}`, 500, "agent-not-running", true},
		{"stopped-same-target", "VM 105 qmp command 'guest-get-fsinfo' failed - QEMU guest agent is not running", 500, "agent-not-running", true},
		{"unsupported", "unsupported command: guest-get-fsinfo", 500, "agent-error", true},
		{"missing-command", "The command guest-get-fsinfo has not been found", 500, "agent-error", true},
		{"unexplained", "provider-private-detail", 500, "agent-error", false},
		{"other-guest", "VM 106 qmp command 'guest-get-fsinfo' failed - QEMU guest agent is not running", 500, "agent-error", false},
		{"other-command", "unsupported command: guest-get-osinfo", 500, "agent-error", false},
		{"conflicting", `{"message":"QEMU guest agent is not running","errors":{"message":"upstream unavailable"}}`, 500, "agent-error", false},
		{"proxy-quotes-stopped", "QEMU guest agent is not running", 502, "agent-error", false},
		{"bad-request-quotes-status", "API error 403: permission denied", 400, "agent-error", false},
		{"unauthorised-quotes-stopped", "API error 500: QEMU guest agent is not running", 401, "permission-denied", false},
		{"forbidden", "provider-private-detail", 403, "permission-denied", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason := guestAgentTerminalFailureReason(path, []byte(tc.body))
			err := &apiResponseError{
				statusCode: tc.status, guestCommandRejected: tc.status == http.StatusInternalServerError && reason != "",
				guestFailureReason: reason, cause: errors.New(tc.body),
			}
			if err.guestCommandRejected != tc.rejected {
				t.Fatalf("terminal rejection = %t, want %t", err.guestCommandRejected, tc.rejected)
			}
			if got := GuestAgentErrorReason(fmt.Errorf("wrapped: %w", err)); got != tc.want {
				t.Errorf("fixed reason = %q, want %q", got, tc.want)
			}
			if err.Error() != tc.body || !errors.Is(err, err.cause) {
				t.Fatal("observed error text/cause was changed")
			}
		})
	}
	for _, err := range []error{nil, errors.New("API error 500: QEMU guest agent is not running")} {
		if got := GuestAgentErrorReason(err); got != "" {
			t.Errorf("untyped error acquired wire evidence: %q", got)
		}
	}
	deferred := &guestAgentDeferredError{reason: "agent-cooldown"}
	if got := GuestAgentErrorReason(fmt.Errorf("wrapped: %w", deferred)); got != "agent-cooldown" {
		t.Errorf("uncertainty was reclassified: %q", got)
	}
}
