//go:build unix

package installtests

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestLifecycleAgentFailureDiagnostics(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode string
		want []string
	}{
		{"early-exit", []string{"pulse-agent exited before fixture report: exit status 23", "diagnostic_signal= cleanup_kill=false wait_error=exit status 23", "fixture output complete"}},
		{"report-timeout", []string{"timed out waiting for fixture report", "diagnostic_signal=SIGQUIT cleanup_kill=false wait_error=exit status 2", "SIGQUIT: quit", "TestLifecycleAgentProcessControl"}},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestLifecycleAgentDiagnosticControl$", "-test.v")
			cmd.Env = append(os.Environ(), "PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL="+tc.mode)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("fixture failure was replaced: err=%v context=%v\n%s", err, ctx.Err(), output)
			}
			for _, want := range append(tc.want, "pulse-agent failure diagnostics:", "fixture stdout before wait", "fixture stderr before wait", "output_truncated=false") {
				if !strings.Contains(string(output), want) {
					t.Fatalf("lost failure diagnostic %q:\n%s", want, output)
				}
			}
			for _, token := range []string{"bootstrap-one", "runtime-one", "bootstrap-two", "runtime-two"} {
				if strings.Contains(string(output), token) {
					t.Fatal("fixture credential reached failure output")
				}
			}
			if strings.Count(string(output), "[redacted fixture token]") != 4 || strings.Count(string(output), "pulse-agent failure diagnostics:") != 1 {
				t.Fatalf("lost redaction or duplicated child wait/output:\n%s", output)
			}
		})
	}
}

// A child test lets the parent inspect testing.T's actual Fatal/Cleanup output.
// These are ordinary fixture exits/timeouts and an explicitly requested Go
// runtime diagnostic, not a reproduced agent crash or historical CI cause.
func TestLifecycleAgentDiagnosticControl(t *testing.T) {
	mode := os.Getenv("PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL")
	if mode == "" {
		return
	}
	var timeout time.Duration
	switch mode {
	case "early-exit":
		timeout = 3 * time.Second
	case "report-timeout":
		timeout = 200 * time.Millisecond
	default:
		t.Fatalf("unknown fixture mode %q", mode)
	}
	process := startLifecycleAgent(t, lifecycleDiagnosticExecutable(t), "http://fixture.invalid", t.TempDir(), false)
	waitForLifecycleState(t, timeout, "fixture report", func(agentLifecycleSnapshot) bool { return false }, &agentLifecycleControlPlane{}, process)
	t.Fatal("failure control was accepted")
}

func lifecycleDiagnosticExecutable(t *testing.T) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quoted := "'" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "'"
	path := filepath.Join(t.TempDir(), "fixture-agent")
	script := "#!/bin/sh\nexec " + quoted + " -test.run=^TestLifecycleAgentProcessControl$\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLifecycleAgentProcessControl(t *testing.T) {
	switch os.Getenv("PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL") {
	case "":
		return
	case "graceful":
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM)
		fmt.Println("fixture ready")
		<-stop
		signal.Stop(stop)
		fmt.Println("fixture stopped")
		os.Exit(0)
	case "early-exit", "report-timeout":
		fmt.Println("fixture stdout before wait bootstrap-one runtime-one")
		fmt.Fprintln(os.Stderr, "fixture stderr before wait bootstrap-two runtime-two")
		if os.Getenv("PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL") == "early-exit" {
			fmt.Println(strings.Repeat("drained output\n", 4000))
			fmt.Println("fixture output complete")
			os.Exit(23)
		}
		for {
			time.Sleep(time.Hour)
		}
	default:
		t.Fatal("unknown fixture process mode")
	}
}

func TestLifecycleAgentStopDrainsOutputOnce(t *testing.T) {
	t.Setenv("PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL", "graceful")
	process := startLifecycleAgent(t, lifecycleDiagnosticExecutable(t), "http://fixture.invalid", t.TempDir(), false)
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(process.logs.String(), "fixture ready") {
		if time.Now().After(deadline) {
			t.Fatal("fixture did not start")
		}
		time.Sleep(time.Millisecond)
	}
	output := process.stop(t)
	if !strings.Contains(output, "fixture stopped") || strings.Count(output, "fixture ready") != 1 {
		t.Fatalf("stop returned before output was drained: %q", output)
	}
	if again := process.stop(t); again != output {
		t.Fatalf("repeated stop changed the drained output: %q != %q", again, output)
	}
}

func TestLifecycleAgentOutputConcurrentRead(t *testing.T) {
	output := &lifecycleAgentOutput{}
	var writers sync.WaitGroup
	for i := 0; i < 4; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for j := 0; j < 100; j++ {
				_, _ = output.Write([]byte("fixture line\n"))
				_ = output.String()
			}
		}()
	}
	writers.Wait()
	if got := strings.Count(output.String(), "fixture line\n"); got != 400 {
		t.Fatalf("output lost concurrent writes: lines=%d", got)
	}
}

func TestLifecycleAgentDiagnosticOutputRedactsBeforeBounding(t *testing.T) {
	input := "bootstrap-one runtime-one bootstrap-two runtime-two\n" + strings.Repeat("x", 300*1024)
	output := lifecycleDiagnosticOutput(input)
	if !strings.Contains(output, "output_truncated=true omitted_bytes=") || len(output) > 257*1024 ||
		strings.Count(output, "[redacted fixture token]") != 4 {
		t.Fatal("diagnostic lost its redactions or explicit output bound")
	}
	for _, token := range []string{"bootstrap-one", "runtime-one", "bootstrap-two", "runtime-two"} {
		if strings.Contains(output, token) {
			t.Fatal("bounded diagnostic contains a fixture credential")
		}
	}
}

func TestLifecycleRequestObservationsKeepRejectedAttempts(t *testing.T) {
	state := &agentLifecycleControlPlane{online: true}
	state.setCredentials("bootstrap-one", "runtime-one", "agent-one")
	request := func(path, token string, body []byte, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("X-API-Token", token)
		reply := httptest.NewRecorder()
		state.serveHTTP(reply, req)
		if reply.Code != want {
			t.Fatalf("fixture status=%d, want %d", reply.Code, want)
		}
	}
	request("/api/agents/agent/report", "wrong-fixture-token", nil, http.StatusUnauthorized)
	if got := state.snapshot(); got.reportCount != 0 || got.firstReport != "no-successful-enrollment" ||
		got.requests != "report/401=1,report/started=1" {
		t.Fatalf("rejected report became no attempt: %+v", got)
	}
	request("/api/agents/agent/enroll", "wrong-fixture-token", nil, http.StatusUnauthorized)
	request("/api/agents/agent/enroll", "bootstrap-one", []byte("not JSON"), http.StatusBadRequest)
	request("/api/agents/agent/enroll", "bootstrap-one", []byte(`{"commandsEnabled":false}`), http.StatusOK)
	request("/api/agents/agent/lookup", "", nil, http.StatusNotFound)
	request("/api/agents/agent/private-fixture-id/config", "", nil, http.StatusOK)
	request("/private-fixture-path?secret=private-fixture-query", "", nil, http.StatusNotFound)
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, _ = gz.Write([]byte(`{"agent":{"id":"agent-one"}}`))
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	request("/api/agents/agent/report", "runtime-one", compressed.Bytes(), http.StatusOK)
	state.setOnline(false)
	request("/api/agents/agent/report", "runtime-one", nil, http.StatusServiceUnavailable)
	got := state.snapshot()
	want := "config/200=1,config/started=1,enrollment/200=1,enrollment/400=1,enrollment/401=1,enrollment/started=3,lookup/404=1,lookup/started=1,other/404=1,other/started=1,report/200=1,report/401=1,report/503=1,report/started=3"
	if got.requests != want || got.enrollmentCount != 1 || got.reportCount != 1 || got.lastReportID != "agent-one" ||
		got.lastReportToken != "runtime-one" || got.firstReport == "no-report-attempt" || got.firstReport == "no-successful-enrollment" {
		t.Fatalf("fixture changed admission or lost observations: %+v", got)
	}
	for _, private := range []string{"bootstrap-one", "runtime-one", "private-fixture"} {
		if strings.Contains(got.diagnostic(), private) {
			t.Fatal("request diagnostic includes a request secret/path/identifier")
		}
	}
}

func TestLifecycleRequestObservationsAreConcurrent(t *testing.T) {
	state := &agentLifecycleControlPlane{online: true}
	state.setCredentials("bootstrap-one", "runtime-one", "agent-one")
	var requests sync.WaitGroup
	for i := 0; i < 8; i++ {
		requests.Add(1)
		go func() {
			defer requests.Done()
			for j := 0; j < 10; j++ {
				req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", nil)
				state.serveHTTP(httptest.NewRecorder(), req)
				_ = state.snapshot()
			}
		}()
	}
	requests.Wait()
	if got := state.snapshot(); got.reportCount != 0 || got.requests != "report/401=80,report/started=80" {
		t.Fatalf("concurrent request observations lost or accepted a report: %+v", got)
	}
}

func TestLifecycleRequestObservationsKeepIncompleteRequests(t *testing.T) {
	state := &agentLifecycleControlPlane{online: true}
	state.setCredentials("bootstrap-one", "runtime-one", "agent-one")
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", reader)
	req.Header.Set("X-API-Token", "bootstrap-one")
	reply := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		state.serveHTTP(reply, req)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		got := state.snapshot()
		if got.requests != "" {
			if got.requests != "enrollment/started=1" || got.enrollmentCount != 0 {
				t.Fatalf("incomplete body became a response/enrollment: %+v", got)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture did not receive request")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := io.WriteString(writer, `{"commandsEnabled":false}`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("fixture did not finish response")
	}
	if got := state.snapshot(); reply.Code != http.StatusOK || got.enrollmentCount != 1 ||
		got.requests != "enrollment/200=1,enrollment/started=1" || got.firstReport != "no-report-attempt" {
		t.Fatalf("completed request lost response or fabricated report timing: %+v", got)
	}
}
