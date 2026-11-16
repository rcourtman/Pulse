//go:build unix

package installtests

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func waitForLifecycleOutput(t *testing.T, proc *runningLifecycleAgent, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(proc.logs.String(), marker) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture child did not write its readiness marker: %s", proc.diagnostics())
}

// Observe the real testing.T failure/cleanup path in a separate test process.
// These intentionally failing children must never turn into passing controls.
func TestLifecycleAgentFailureEvidence(t *testing.T) {
	if mode := os.Getenv("PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL"); mode != "" {
		script := `printf 'fixture stdout bootstrap-one runtime-one\n'; printf 'fixture stderr bootstrap-two runtime-two\n' >&2; `
		switch mode {
		case "exit-zero", "shutdown-after-exit":
			script += "exit 0"
		case "exit-seven":
			script += "exit 7"
		case "timeout":
			script += "exec sleep 60" // No descendant holds the fixture output pipes.
		default:
			t.Fatalf("unknown diagnostic control %q", mode)
		}
		proc := startLifecycleProcess(t, exec.Command("bash", "-c", script))
		waitForLifecycleOutput(t, proc, "fixture stderr")
		if mode != "timeout" {
			select {
			case <-proc.done:
			case <-time.After(5 * time.Second):
				t.Fatal("fixture child exit was not observed")
			}
		}
		if mode == "shutdown-after-exit" {
			proc.stop(t)
		} else {
			waitForLifecycleState(t, 100*time.Millisecond, "controlled report",
				func(s agentLifecycleSnapshot) bool { return s.reportCount > 0 },
				&agentLifecycleControlPlane{}, proc)
		}
		t.Fatal("failure control unexpectedly returned")
		return
	}

	for _, tc := range []struct {
		mode, failure, result string
	}{
		{"exit-zero", "pulse-agent exited before controlled report: exit status 0", "process result: exit status 0"},
		{"exit-seven", "pulse-agent exited before controlled report: exit status 7", "process result: exit status 7"},
		{"shutdown-after-exit", "pulse-agent exited before requested shutdown: exit status 0", "process result: exit status 0"},
		{"timeout", "timed out waiting for controlled report", "process result: signal: killed"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLifecycleAgentFailureEvidence$", "-test.v", "-test.timeout=15s")
			cmd.Env = append(os.Environ(), "PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL="+tc.mode)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("control did not finish with the expected test failure: context=%v err=%v\n%s", ctx.Err(), err, output)
			}
			text := string(output)
			for _, want := range []string{tc.failure, tc.result, "pulse-agent failure evidence (owned leader/output only):",
				"fixture stdout [fixture credential] [fixture credential]", "fixture stderr [fixture credential] [fixture credential]"} {
				if !strings.Contains(text, want) {
					t.Errorf("control lost %q:\n%s", want, text)
				}
			}
			for _, secret := range []string{"bootstrap-one", "runtime-one", "bootstrap-two", "runtime-two"} {
				if strings.Contains(text, secret) {
					t.Errorf("failure diagnostic disclosed a synthetic credential")
				}
			}
			if strings.Contains(text, "WARNING: DATA RACE") {
				t.Fatalf("control raced while retaining its output:\n%s", text)
			}
		})
	}
}

func TestLifecycleAgentStopDrainsOutputOnce(t *testing.T) {
	proc := startLifecycleProcess(t, exec.Command("bash", "-c", `
trap 'printf "shutdown stdout\n"; printf "shutdown stderr\n" >&2; exit 0' TERM
printf 'fixture ready bootstrap-one\n'
while :; do sleep 0.05; done
`))
	waitForLifecycleOutput(t, proc, "fixture ready")
	output := proc.stop(t)
	for _, want := range []string{"shutdown stdout", "shutdown stderr", "bootstrap-one"} {
		if !strings.Contains(output, want) {
			t.Errorf("stop lost raw %q; credential assertions must see unredacted output: %q", want, output)
		}
	}
	if got := proc.stop(t); got != output {
		t.Fatalf("second stop did not retain the first drained result: %q", got)
	}
	if result, exited := proc.result(); !exited || result != "exit status 0" {
		t.Fatalf("shutdown exit was not retained: %q, exited=%t", result, exited)
	}
}

func TestLifecycleAgentConcurrentOutputSnapshots(t *testing.T) {
	var logs lifecycleAgentOutput
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 100 {
				_, _ = logs.Write([]byte("fixture output\n"))
				_ = logs.String()
			}
		})
	}
	workers.Wait()
	if got := strings.Count(logs.String(), "fixture output\n"); got != 400 {
		t.Fatalf("snapshots lost child output: got %d lines, want 400", got)
	}
}

func TestLifecycleAgentDiagnosticOutputRedactsBeforeBounding(t *testing.T) {
	input := "bootstrap-one runtime-one bootstrap-two runtime-two\n" + strings.Repeat("x", 300*1024)
	output := lifecycleDiagnosticOutput(input)
	if !strings.Contains(output, "output_truncated=true omitted_bytes=") || len(output) > 257*1024 ||
		strings.Count(output, "[fixture credential]") != 4 {
		t.Fatal("diagnostic lost its redactions or explicit output bound")
	}
	for _, token := range []string{"bootstrap-one", "runtime-one", "bootstrap-two", "runtime-two"} {
		if strings.Contains(output, token) {
			t.Fatal("bounded diagnostic contains a fixture credential")
		}
	}
	// Bounding raw bytes first would leave a partial credential at the edge.
	const limit = 256 * 1024
	prefix := strings.Repeat("x", limit-5)
	if got := lifecycleDiagnosticOutput(prefix + "bootstrap-one"); got !=
		"output_truncated=true omitted_bytes=15\n"+prefix+"[fixt" {
		t.Fatal("diagnostic did not redact the credential crossing its output boundary")
	}
}

func TestLifecycleRequestObservationTiming(t *testing.T) {
	start := time.Unix(1_000, 0)
	for _, tc := range []struct {
		name, want         string
		enrollment, report time.Time
	}{
		{name: "no requests", want: "no-report-attempt"},
		{name: "enrolled without report", enrollment: start, want: "no-report-attempt"},
		{name: "report without enrollment", report: start, want: "no-successful-enrollment"},
		{name: "report before enrollment", enrollment: start.Add(time.Second), report: start, want: "before-successful-enrollment"},
		{name: "report after enrollment", enrollment: start, report: start.Add(250 * time.Millisecond), want: "250ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &agentLifecycleControlPlane{
				enrolledAt: tc.enrollment, firstReportAt: tc.report,
				lastReportID: "private-fixture-report-id", lastReportToken: "private-fixture-token",
			}
			got := state.snapshot()
			if got.firstReport != tc.want || !strings.Contains(got.diagnostic(), "first_report_after_first_enrollment="+tc.want) {
				t.Fatalf("ambiguous first report timing: %s, want %s", got.diagnostic(), tc.want)
			}
			if strings.Contains(got.diagnostic(), "private-fixture") {
				t.Fatal("timing diagnostic contains a reported identity or token")
			}
		})
	}
}

func TestLifecycleRequestFailureEvidence(t *testing.T) {
	if mode := os.Getenv("PULSE_LIFECYCLE_REQUEST_CONTROL"); mode != "" {
		proc := startLifecycleProcess(t, exec.Command("bash", "-c", "printf 'fixture ready\\n'; exec sleep 60"))
		waitForLifecycleOutput(t, proc, "fixture ready")
		state := &agentLifecycleControlPlane{online: true}
		state.setCredentials("bootstrap-one", "runtime-one", "private-fixture-canonical-id")
		enroll := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", strings.NewReader(`{"commandsEnabled":false}`))
		enroll.Header.Set("X-API-Token", "bootstrap-one")
		state.serveHTTP(httptest.NewRecorder(), enroll)
		var body bytes.Buffer
		gz := gzip.NewWriter(&body)
		_, _ = gz.Write([]byte(`{"agent":{"id":"private-fixture-report-id"}}`))
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		report := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report?secret=private-fixture-query", &body)
		switch mode {
		case "rejected":
			report.Header.Set("X-API-Token", "private-fixture-wrong-token")
		case "unexpected-id":
			report.Header.Set("X-API-Token", "runtime-one")
		default:
			t.Fatalf("unknown request control %q", mode)
		}
		state.serveHTTP(httptest.NewRecorder(), report)
		waitForLifecycleState(t, 100*time.Millisecond, "canonical report", func(s agentLifecycleSnapshot) bool {
			return s.reportCount > 0 && s.lastReportID == "agent-one"
		}, state, proc)
		t.Fatal("request failure control unexpectedly returned")
		return
	}
	for _, tc := range []struct{ mode, counts, identity string }{
		{"rejected", "reports=0 commands=false requests=[enrollment/200=1,enrollment/started=1,report/401=1,report/started=1]", "missing"},
		{"unexpected-id", "reports=1 commands=false requests=[enrollment/200=1,enrollment/started=1,report/200=1,report/started=1]", "other"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLifecycleRequestFailureEvidence$", "-test.v", "-test.timeout=15s")
			cmd.Env = append(os.Environ(), "PULSE_LIFECYCLE_REQUEST_CONTROL="+tc.mode)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("request control did not retain the expected failure: context=%v err=%v\n%s", ctx.Err(), err, output)
			}
			for _, want := range []string{"timed out waiting for canonical report; enrollments=1", tc.counts,
				"first_report_after_first_enrollment=", "report_id=" + tc.identity, "process result: signal: killed"} {
				if !strings.Contains(string(output), want) {
					t.Errorf("request failure lost %q:\n%s", want, output)
				}
			}
			for _, private := range []string{"private-fixture", "bootstrap-one", "runtime-one", "WARNING: DATA RACE"} {
				if strings.Contains(string(output), private) {
					t.Errorf("request failure disclosed private fixture data or raced:\n%s", output)
				}
			}
		})
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
	if got := state.snapshot(); got.reportCount != 0 || got.firstReport != "no-successful-enrollment" || got.reportIdentity != "missing" ||
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
		got.lastReportToken != "runtime-one" || got.firstReport != "before-successful-enrollment" ||
		got.reportIdentity != "canonical" || !strings.Contains(got.diagnostic(), "report_id=canonical") {
		t.Fatalf("fixture changed admission or lost observations: %+v", got)
	}
	for _, private := range []string{"bootstrap-one", "runtime-one", "private-fixture"} {
		if strings.Contains(got.diagnostic(), private) {
			t.Fatal("request diagnostic includes a request secret/path/identifier")
		}
	}
	// A prior report is no longer canonical after credential/identity rotation;
	// the timing must still identify its lifetime-first sample explicitly.
	state.setCredentials("bootstrap-two", "runtime-two", "agent-two")
	rotated := state.snapshot()
	if rotated.reportIdentity != "other" || rotated.firstReport != got.firstReport ||
		!strings.Contains(rotated.diagnostic(), "first_report_after_first_enrollment="+got.firstReport) ||
		!strings.Contains(rotated.diagnostic(), "report_id=other") {
		t.Fatal("rotation lost report identity classification or mislabeled lifetime timing")
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
