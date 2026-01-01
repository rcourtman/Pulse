package hostagent

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWrapCommand_TargetWrapping(t *testing.T) {
	tests := []struct {
		name    string
		payload executeCommandPayload
		want    string
	}{
		{
			name: "host command unchanged",
			payload: executeCommandPayload{
				Command:    "echo ok",
				TargetType: "host",
			},
			want: "echo ok",
		},
		{
			name: "container wraps with pct",
			payload: executeCommandPayload{
				Command:    "echo ok",
				TargetType: "container",
				TargetID:   "101",
			},
			want: "pct exec 101 -- echo ok",
		},
		{
			name: "vm wraps with qm guest exec",
			payload: executeCommandPayload{
				Command:    "echo ok",
				TargetType: "vm",
				TargetID:   "900",
			},
			want: "qm guest exec 900 -- echo ok",
		},
		{
			name: "missing target id does not wrap",
			payload: executeCommandPayload{
				Command:    "echo ok",
				TargetType: "container",
				TargetID:   "",
			},
			want: "echo ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wrapCommand(tt.payload); got != tt.want {
				t.Fatalf("wrapCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommandClient_executeCommand_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executeCommand uses different shell on windows")
	}

	c := &CommandClient{}
	result := c.executeCommand(context.Background(), executeCommandPayload{
		RequestID: "r1",
		Command:   "echo hello",
		Timeout:   5,
	})
	if !result.Success || result.ExitCode != 0 {
		t.Fatalf("expected success, got %#v", result)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Fatalf("stdout = %q, expected to contain %q", result.Stdout, "hello")
	}
}

func TestCommandClient_executeCommand_NonZeroExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executeCommand uses different shell on windows")
	}

	c := &CommandClient{}
	result := c.executeCommand(context.Background(), executeCommandPayload{
		RequestID: "r1",
		Command:   "echo err 1>&2; exit 3",
		Timeout:   5,
	})
	if result.Success {
		t.Fatalf("expected failure, got %#v", result)
	}
	if result.ExitCode != 3 {
		t.Fatalf("exit code = %d, want %d", result.ExitCode, 3)
	}
	if !strings.Contains(result.Stderr, "err") {
		t.Fatalf("stderr = %q, expected to contain %q", result.Stderr, "err")
	}
}

func TestCommandClient_executeCommand_TimeoutSetsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executeCommand uses different shell on windows")
	}

	c := &CommandClient{}
	start := time.Now()
	result := c.executeCommand(context.Background(), executeCommandPayload{
		RequestID: "r1",
		Command:   "sleep 2",
		Timeout:   1,
	})
	if time.Since(start) > 3*time.Second {
		t.Fatalf("timeout path took too long: %v", time.Since(start))
	}
	if result.Success {
		t.Fatalf("expected failure, got %#v", result)
	}
	if result.ExitCode != -1 {
		t.Fatalf("exit code = %d, want %d", result.ExitCode, -1)
	}
	if result.Error != "command timed out" {
		t.Fatalf("error = %q, want %q", result.Error, "command timed out")
	}
}

func TestCommandClient_executeCommand_TruncatesLargeOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executeCommand uses different shell on windows")
	}

	c := &CommandClient{}
	result := c.executeCommand(context.Background(), executeCommandPayload{
		RequestID: "r1",
		Command:   "head -c 1048580 /dev/zero | tr '\\0' 'a'",
		Timeout:   5,
	})
	if !result.Success {
		t.Fatalf("expected success, got %#v", result)
	}
	if !strings.Contains(result.Stdout, "(output truncated)") {
		t.Fatalf("expected truncation marker, got stdout len=%d", len(result.Stdout))
	}
	if len(result.Stdout) > 1024*1024+64 {
		t.Fatalf("stdout len=%d, expected <= %d", len(result.Stdout), 1024*1024+64)
	}
}
