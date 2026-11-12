package hostagent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Enrollment runs on Windows too; do not borrow the !windows Proxmox adapter.
type enrollmentRoundTripFunc func(*http.Request) (*http.Response, error)

func (f enrollmentRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Keep both public pre-module enrollment and the legacy loop on the real
// filesystem/HTTP implementation. No report or command client is run here.
func responseTestEnrollment(t *testing.T, entry, url, dir string, ctx context.Context, logs *bytes.Buffer) (EnrollmentResult, error) {
	t.Helper()
	collector := &enrollmentStateCollector{defaultCollector: &defaultCollector{}}
	logger := zerolog.New(logs)
	if entry == "pre-module" {
		return Enroll(ctx, EnrollmentConfig{PulseURL: url, APIToken: "synthetic-bootstrap", StateDir: dir, Collector: collector, Logger: &logger})
	}
	a := &Agent{
		cfg: Config{APIToken: "synthetic-bootstrap", AgentID: "synthetic-original"}, agentID: "synthetic-original",
		stateDir: dir, collector: collector, logger: logger, httpClient: &http.Client{Timeout: time.Second}, trimmedPulseURL: url,
	}
	err := a.runEnrollmentLoop(ctx)
	if err != nil {
		if a.cfg.APIToken != "synthetic-bootstrap" || a.cfg.AgentID != "synthetic-original" || a.agentID != "synthetic-original" {
			t.Error("failed enrollment released replacement authority in memory")
		}
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{APIToken: a.cfg.APIToken, AgentID: a.cfg.AgentID}, nil
}

func TestEnrollmentUnconfirmedSuccessIsNotRetried(t *testing.T) {
	valid := `{"runtimeToken":"synthetic-runtime","agentId":"synthetic-new"}`
	for _, entry := range []string{"pre-module", "legacy-loop"} {
		for _, tc := range []struct {
			name, body, want string
			incomplete       bool
		}{
			{name: "malformed", body: `{"runtimeToken":"synthetic-private-response",`, want: "invalid JSON"},
			{name: "empty", want: "invalid JSON"},
			{name: "multiple-values", body: valid + `{}`, want: "invalid JSON"},
			{name: "token-type", body: `{"runtimeToken":{"synthetic-private-response":true}}`, want: "invalid JSON"},
			{name: "identity-type", body: `{"runtimeToken":"synthetic-private-response","agentId":true}`, want: "invalid JSON"},
			{name: "missing-token", body: `{"agentId":"synthetic-new"}`, want: "nonblank runtime token"},
			{name: "null", body: `null`, want: "nonblank runtime token"},
			{name: "blank-token", body: `{"runtimeToken":" \t\r\n "}`, want: "nonblank runtime token"},
			{name: "oversized-valid-prefix", body: valid + strings.Repeat(" ", maxEnrollResponseBytes), want: "size limit"},
			{name: "incomplete-valid-prefix", body: valid, incomplete: true, want: "incomplete response body"},
		} {
			t.Run(entry+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				files := map[string]string{"agent-id": "synthetic-original", "saved-ca.pem": "synthetic-saved-trust"}
				for name, data := range files {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				var posts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost || r.URL.Path != enrollEndpoint || r.Header.Get("X-API-Token") != "synthetic-bootstrap" {
						t.Error("enrollment route or bootstrap credential changed")
					}
					if posts.Add(1) != 1 {
						http.Error(w, "synthetic-private-response", http.StatusConflict)
						return
					}
					if tc.incomplete {
						w.Header().Set("Content-Length", strconv.Itoa(len(tc.body)+100))
						w.Header().Set("Connection", "close")
					}
					_, _ = io.WriteString(w, tc.body)
				}))
				defer server.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				var logs bytes.Buffer
				result, err := responseTestEnrollment(t, entry, server.URL, dir, ctx, &logs)
				var responseErr *enrollResponseError
				if !errors.As(err, &responseErr) || !strings.Contains(err.Error(), tc.want) ||
					!strings.Contains(err.Error(), "bootstrap token may already be consumed") || !strings.Contains(err.Error(), "refusing to retry enrollment") {
					t.Fatalf("invalid success did not return safe terminal uncertainty: %v", err)
				}
				if posts.Load() != 1 || result != (EnrollmentResult{}) {
					t.Fatalf("invalid success retried or returned authority: posts=%d result=%+v", posts.Load(), result)
				}
				for _, marker := range []string{"synthetic-private-response", "synthetic-bootstrap", "synthetic-runtime", dir} {
					if strings.Contains(err.Error()+logs.String(), marker) {
						t.Error("enrollment response diagnostic disclosed private input")
					}
				}
				if _, err := os.Lstat(filepath.Join(dir, runtimeTokenFile)); !errors.Is(err, os.ErrNotExist) {
					t.Error("unconfirmed response created a runtime credential")
				}
				for name, data := range files {
					path := filepath.Join(dir, name)
					got, err := os.ReadFile(path)
					info, statErr := os.Stat(path)
					if err != nil || statErr != nil || string(got) != data || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
						t.Error("unconfirmed response modified saved identity or trust")
					}
				}
			})
		}
	}
}

func TestEnrollmentStatusBodiesStayOutOfDiagnostics(t *testing.T) {
	for _, entry := range []string{"pre-module", "legacy-loop"} {
		for _, status := range []int{400, 401, 403, 404, 409, 408, 429, 500, 502, 503} {
			t.Run(fmt.Sprintf("%s/%d", entry, status), func(t *testing.T) {
				oldInitial, oldMax := enrollInitialDelay, enrollMaxDelay
				enrollInitialDelay, enrollMaxDelay = time.Millisecond, time.Millisecond
				t.Cleanup(func() { enrollInitialDelay, enrollMaxDelay = oldInitial, oldMax })
				retryable := status == 408 || status == 429 || status >= 500
				var posts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if posts.Add(1) == 1 || !retryable {
						w.WriteHeader(status)
						_, _ = io.WriteString(w, "synthetic-private-response synthetic-bootstrap synthetic-private-path")
						return
					}
					_, _ = io.WriteString(w, `{"runtimeToken":"synthetic-runtime","agentId":"synthetic-new","futureField":true}`)
				}))
				defer server.Close()
				dir := t.TempDir()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				var logs bytes.Buffer
				result, err := responseTestEnrollment(t, entry, server.URL, dir, ctx, &logs)
				wantPosts := int32(1)
				if retryable {
					wantPosts = 2
					if err != nil || result.APIToken != "synthetic-runtime" || result.AgentID != "synthetic-new" {
						t.Fatalf("transient failure did not retain existing retry behavior: %v", err)
					}
				} else if status == 403 {
					if err != nil || result.APIToken != "synthetic-bootstrap" {
						t.Fatalf("manual-token 403 fallback changed: %v", err)
					}
				} else {
					var statusErr *enrollStatusError
					if !errors.As(err, &statusErr) || statusErr.StatusCode != status || !strings.Contains(err.Error(), strconv.Itoa(status)) {
						t.Fatalf("permanent refusal lost numeric status: %v", err)
					}
				}
				if posts.Load() != wantPosts {
					t.Fatalf("status classification changed: posts=%d, want=%d", posts.Load(), wantPosts)
				}
				for _, marker := range []string{"synthetic-private-response", "synthetic-bootstrap", "synthetic-private-path"} {
					if strings.Contains(fmt.Sprint(err)+logs.String(), marker) {
						t.Error("upstream status body reached diagnostics")
					}
				}
				if !retryable {
					if _, err := os.Stat(filepath.Join(dir, runtimeTokenFile)); !errors.Is(err, os.ErrNotExist) {
						t.Error("status-only response published a credential")
					}
				} else if got, err := os.ReadFile(filepath.Join(dir, runtimeTokenFile)); err != nil || string(got) != "synthetic-runtime" {
					t.Error("valid response following a transient status was not saved")
				}
			})
		}
	}
}

func TestEnrollmentResponseSizeBoundaryAndOptionalIdentity(t *testing.T) {
	for _, entry := range []string{"pre-module", "legacy-loop"} {
		for _, size := range []int{maxEnrollResponseBytes - 1, maxEnrollResponseBytes} {
			t.Run(fmt.Sprintf("%s/%d", entry, size), func(t *testing.T) {
				body := `{"runtimeToken":"synthetic-runtime","futureField":true}`
				body += strings.Repeat(" ", size-len(body))
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) }))
				defer server.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				var logs bytes.Buffer
				result, err := responseTestEnrollment(t, entry, server.URL, t.TempDir(), ctx, &logs)
				if err != nil || result.APIToken != "synthetic-runtime" {
					t.Fatalf("complete bounded response with optional fields rejected: %v", err)
				}
			})
		}
	}
}

type enrollmentReadFailure struct {
	data   *strings.Reader
	closed bool
	cancel context.CancelFunc
}

func (r *enrollmentReadFailure) Read(p []byte) (int, error) {
	if r.data.Len() > 0 {
		return r.data.Read(p)
	}
	if r.cancel != nil {
		r.cancel()
	}
	return 0, errors.New("synthetic-private-response synthetic-private-read-error")
}

func (r *enrollmentReadFailure) Close() error { r.closed = true; return nil }

func TestEnrollmentResponseReadFailureIsPrivateAndClosesBody(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled-%t", canceled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			body := &enrollmentReadFailure{data: strings.NewReader(`{"runtimeToken":"synthetic-runtime"}`)}
			if canceled {
				body.cancel = cancel
			}
			calls := 0
			var logs bytes.Buffer
			a := &Agent{cfg: Config{APIToken: "synthetic-bootstrap"}, collector: &defaultCollector{}, stateDir: t.TempDir(), logger: zerolog.New(&logs), trimmedPulseURL: "https://synthetic.invalid",
				httpClient: &http.Client{Transport: enrollmentRoundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
				})}}
			err := a.runEnrollmentLoop(ctx)
			if err == nil || !strings.Contains(err.Error(), "incomplete response body") || !body.closed || calls != 1 {
				t.Fatalf("read failure lost its terminal/cleanup facts: err=%v calls=%d closed=%t", err, calls, body.closed)
			}
			if canceled && !errors.Is(err, context.Canceled) {
				t.Error("body-read cancellation lost context identity")
			}
			if a.cfg.APIToken != "synthetic-bootstrap" {
				t.Error("read failure released a partial runtime credential")
			}
			if strings.Contains(err.Error()+logs.String(), "synthetic-") {
				t.Error("read failure copied private data/error details into diagnostics")
			}
		})
	}
}
