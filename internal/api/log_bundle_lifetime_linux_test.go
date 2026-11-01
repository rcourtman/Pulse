package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// A real FIFO holds ordinary log generation after full Router admission but
// before ZIP bytes reach the transport. Only synthetic data and temp stores.
func TestSupportBundleNativeWithdrawal(t *testing.T) {
	for _, transport := range []string{"header", "bearer", "session"} {
		t.Run(transport, func(t *testing.T) {
			router, cfg, server := newLogStreamRouter(t)
			fifo := filepath.Join(t.TempDir(), "held.log")
			if err := syscall.Mkfifo(fifo, 0600); err != nil {
				t.Fatal(err)
			}
			config.Mu.Lock()
			cfg.LogFile = fifo
			config.Mu.Unlock()
			raw := logStreamToken
			if transport == "session" {
				raw = "synthetic-held-bundle-session"
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", cfg.AuthUser)
				t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
			}
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/logs/download", nil)
			switch transport {
			case "bearer":
				req.Header.Set("Authorization", "Bearer "+raw)
			case "session":
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
			default:
				req.Header.Set("X-API-Token", raw)
			}
			type result struct {
				response *http.Response
				err      error
			}
			done := make(chan result, 1)
			go func() { response, err := server.Client().Do(req); done <- result{response, err} }()
			// O_NONBLOCK avoids a test goroutine stuck in open if admission fails.
			var pipe *os.File
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				fd, err := syscall.Open(fifo, syscall.O_WRONLY|syscall.O_NONBLOCK, 0600)
				if err == nil {
					pipe = os.NewFile(uintptr(fd), fifo)
					break
				}
				if err != syscall.ENXIO {
					t.Fatal(err)
				}
				time.Sleep(time.Millisecond)
			}
			if pipe == nil {
				unblock, _ := os.OpenFile(fifo, os.O_RDWR, 0600)
				if unblock != nil {
					unblock.Close()
				}
				t.Fatal("download did not reach held log generation")
			}
			defer pipe.Close()
			// Handler has captured/opened the FIFO. Further requests use a normal log.
			controlFile := filepath.Join(t.TempDir(), "control.log")
			const marker = "synthetic confidential log after withdrawal"
			if err := os.WriteFile(controlFile, []byte(marker), 0600); err != nil {
				t.Fatal(err)
			}
			config.Mu.Lock()
			cfg.LogFile = controlFile
			config.Mu.Unlock()
			if transport == "session" {
				logoutReadSession(t, server.URL, raw)
			} else {
				id := cfg.APITokens[0].ID
				deletion, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/security/tokens/"+id, nil)
				deletion.Header.Set("X-API-Token", logStreamControl)
				response, err := server.Client().Do(deletion)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				persisted, err := router.persistence.LoadAPITokens()
				if response.StatusCode != http.StatusNoContent || err != nil || len(persisted) != 1 || persisted[0].ID == id {
					t.Fatal("real DELETE did not persist withdrawal")
				}
			}
			// A fresh request already refuses the same credential.
			fresh := req.Clone(ctx)
			response, err := server.Client().Do(fresh)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("fresh admission = %d", response.StatusCode)
			}
			if _, err := pipe.Write([]byte(marker)); err != nil {
				t.Fatal(err)
			}
			pipe.Close()
			var completed result
			select {
			case completed = <-done:
			case <-ctx.Done():
				t.Fatal("held download failed to finish")
			}
			if completed.err != nil {
				t.Fatal(completed.err)
			}
			defer completed.response.Body.Close()
			body, err := io.ReadAll(completed.response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if completed.response.StatusCode != http.StatusUnauthorized {
				// Decode to distinguish a private-log disclosure from a generic status
				// mismatch. No original/customer material enters this fixture or output.
				disclosed := bytes.Contains(readSupportBundle(t, body)["pulse.log"], []byte(marker))
				t.Errorf("held download = %d after persisted withdrawal (private log disclosed=%v); want generic 401", completed.response.StatusCode, disclosed)
			} else if string(body) != "Authentication required\n" || completed.response.Header.Get("Content-Disposition") != "" || completed.response.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
				t.Error("withdrawn response retained private ZIP headers/body")
			}
			control, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/logs/download", nil)
			control.Header.Set("X-API-Token", logStreamControl)
			independent, err := server.Client().Do(control)
			if err != nil {
				t.Fatal(err)
			}
			defer independent.Body.Close()
			independentBody, err := io.ReadAll(independent.Body)
			if err != nil {
				t.Fatal(err)
			}
			if independent.StatusCode != http.StatusOK || independent.Header.Get("Content-Type") != "application/zip" || !bytes.Equal(readSupportBundle(t, independentBody)["pulse.log"], []byte(marker)) {
				t.Fatal("independent settings reader lost original log evidence")
			}
		})
	}
}
