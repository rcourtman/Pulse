package main

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
)

func TestRunUnconfirmedEnrollmentResponseStopsAuthenticatedConsumers(t *testing.T) {
	valid := `{"runtimeToken":"synthetic-runtime","agentId":"synthetic-new"}`
	for _, tc := range []struct {
		name, body string
		incomplete bool
	}{
		{name: "malformed", body: `{"runtimeToken":"synthetic-private-response",`},
		{name: "missing-token", body: `{"agentId":"synthetic-new"}`},
		{name: "blank-token", body: `{"runtimeToken":" \t "}`},
		{name: "incomplete-valid-prefix", body: valid, incomplete: true},
		{name: "oversized-valid-prefix", body: valid + strings.Repeat(" ", 64*1024)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "agent-id"), []byte("synthetic-original\n"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			evidence := spyIdentityStartup(t, cancel)
			enrollAgent = hostagent.Enroll
			oldGate := enrollBeforeModuleStartup
			enrollBeforeModuleStartup = true
			t.Cleanup(func() { enrollBeforeModuleStartup = oldGate })
			var posts, downstream atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/agents/agent/enroll" {
					downstream.Add(1)
					http.NotFound(w, r)
					return
				}
				if posts.Add(1) != 1 || r.Header.Get("X-API-Token") != "synthetic-bootstrap" {
					t.Error("unconfirmed exchange was retried or changed bootstrap authority")
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
			ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			caPath := filepath.Join(dir, "saved-ca.pem")
			if err := os.WriteFile(caPath, ca, 0600); err != nil {
				t.Fatal(err)
			}
			err := run(ctx, []string{"--url", server.URL, "--token", "synthetic-bootstrap", "--enroll", "--state-dir", dir,
				"--agent-id", "synthetic-original", "--cacert", caPath, "--enable-host", "--enable-docker", "--enable-kubernetes", "--health-addr", ""}, func(string) string { return "" })
			if err == nil || !strings.Contains(err.Error(), "cannot confirm enrollment response (HTTP 200)") ||
				!strings.Contains(err.Error(), "bootstrap token may already be consumed") {
				t.Fatalf("unconfirmed response did not stop unified startup: %v", err)
			}
			for _, marker := range []string{dir, "synthetic-private-response", "synthetic-bootstrap", "synthetic-runtime"} {
				if strings.Contains(err.Error(), marker) {
					t.Error("startup response error disclosed private input")
				}
			}
			if posts.Load() != 1 || downstream.Load() != 0 || len(evidence.calls) != 0 {
				t.Fatalf("unconfirmed response reached startup consumers: posts=%d requests=%d constructors=%v", posts.Load(), downstream.Load(), evidence.calls)
			}
			if _, err := os.Stat(filepath.Join(dir, "runtime.token")); !os.IsNotExist(err) {
				t.Error("unconfirmed response published a runtime credential")
			}
			if got, err := os.ReadFile(filepath.Join(dir, "agent-id")); err != nil || string(got) != "synthetic-original\n" {
				t.Error("unconfirmed response replaced saved identity")
			}
			if got, err := os.ReadFile(caPath); err != nil || string(got) != string(ca) {
				t.Error("unconfirmed response changed saved TLS trust")
			}
		})
	}
}
