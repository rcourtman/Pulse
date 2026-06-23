package agentcapabilities

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewToolBlockedErrorBuildsSharedEnvelope(t *testing.T) {
	resp := NewToolBlockedError(ErrCodeStrictResolution, "discover first", map[string]any{
		"resource_id": "vm:101",
	})

	if resp.OK {
		t.Fatal("blocked tool response must set ok=false")
	}
	if resp.Error == nil {
		t.Fatal("blocked tool response must include error")
	}
	if resp.Error.Code != ErrCodeStrictResolution {
		t.Fatalf("error code = %q, want %q", resp.Error.Code, ErrCodeStrictResolution)
	}
	if !resp.Error.Blocked || resp.Error.Failed || resp.Error.Retryable {
		t.Fatalf("unexpected error posture: %+v", resp.Error)
	}

	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		`"ok":false`,
		`"code":"STRICT_RESOLUTION"`,
		`"blocked":true`,
		`"resource_id":"vm:101"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("tool response missing %s: %s", want, text)
		}
	}
}

func TestToolResponseResultPreservesErrorState(t *testing.T) {
	blocked := NewToolBlockedError(ErrCodeActionNotAllowed, "not available here", nil)
	result := NewToolResponseResult(blocked)
	if !result.IsError {
		t.Fatal("blocked tool response must become isError=true")
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"code":"ACTION_NOT_ALLOWED"`) {
		t.Fatalf("blocked response not encoded in tool content: %+v", result)
	}

	ok := NewToolResponseResult(ToolResponse{OK: true, Data: map[string]any{"status": "ok"}})
	if ok.IsError {
		t.Fatal("ok tool response must become isError=false")
	}
	if len(ok.Content) != 1 || !strings.Contains(ok.Content[0].Text, `"status":"ok"`) {
		t.Fatalf("ok response not encoded in tool content: %+v", ok)
	}

}

func TestToolResultHasVerificationOK(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{
			name: "top level verification ok",
			text: `{"success":true,"verification":{"ok":true,"method":"exit_code"}}`,
			want: true,
		},
		{
			name: "leading text before json",
			text: `Action complete: {"verification":{"ok":true}}`,
			want: true,
		},
		{
			name: "verification false",
			text: `{"verification":{"ok":false}}`,
			want: false,
		},
		{
			name: "missing verification",
			text: `{"success":true}`,
			want: false,
		},
		{
			name: "malformed json",
			text: `Action complete: {"verification":{"ok":true}`,
			want: false,
		},
		{
			name: "non boolean ok",
			text: `{"verification":{"ok":"true"}}`,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToolResultHasVerificationOK(tc.text); got != tc.want {
				t.Fatalf("ToolResultHasVerificationOK(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestToolResultErrorCode(t *testing.T) {
	blocked := NewToolResponseResult(NewToolBlockedError(ErrCodeStrictResolution, "discover first", nil))
	blockedText := ToolResultText(blocked)
	fsmBlocked := NewToolResponseResult(NewToolBlockedError(ErrCodeFSMBlocked, "verify before writing again", nil))
	fsmBlockedText := ToolResultText(fsmBlocked)

	for _, tc := range []struct {
		name     string
		text     string
		wantCode string
		wantOK   bool
	}{
		{
			name:     "tool response error code",
			text:     blockedText,
			wantCode: ErrCodeStrictResolution,
			wantOK:   true,
		},
		{
			name:     "fsm blocked error code",
			text:     fsmBlockedText,
			wantCode: ErrCodeFSMBlocked,
			wantOK:   true,
		},
		{
			name:     "leading text before json",
			text:     `Blocked: {"ok":false,"error":{"code":"ROUTING_MISMATCH","message":"choose child"}}`,
			wantCode: ErrCodeRoutingMismatch,
			wantOK:   true,
		},
		{
			name:     "legacy top level error code",
			text:     `{"error_code":"EXECUTION_CONTEXT_UNAVAILABLE"}`,
			wantCode: ErrCodeExecutionContextUnavailable,
			wantOK:   true,
		},
		{
			name:   "missing error",
			text:   `{"ok":true}`,
			wantOK: false,
		},
		{
			name:   "malformed json",
			text:   `Blocked: {"error":{"code":"STRICT_RESOLUTION"}`,
			wantOK: false,
		},
		{
			name:   "non string code",
			text:   `{"error":{"code":42}}`,
			wantOK: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotCode, gotOK := ToolResultErrorCode(tc.text)
			if gotCode != tc.wantCode || gotOK != tc.wantOK {
				t.Fatalf("ToolResultErrorCode(%q) = %q, %v; want %q, %v", tc.text, gotCode, gotOK, tc.wantCode, tc.wantOK)
			}
			if gotOK && !ToolResultHasErrorCode(tc.text, tc.wantCode) {
				t.Fatalf("ToolResultHasErrorCode(%q, %q) = false, want true", tc.text, tc.wantCode)
			}
			if ToolResultHasErrorCode(tc.text, ErrCodeNoAgent) {
				t.Fatalf("ToolResultHasErrorCode(%q, %q) = true, want false", tc.text, ErrCodeNoAgent)
			}
		})
	}
}
