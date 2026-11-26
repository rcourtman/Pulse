package pmg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestNewClientWithContextHonoursCancellationAndTrust(t *testing.T) {
	var logins, versionCalls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	requestEnded := make(chan struct{})
	var enteredOnce, releaseOnce, endedOnce sync.Once

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/access/ticket" {
			attempt := logins.Add(1)
			if attempt == 1 {
				enteredOnce.Do(func() { close(entered) })
				select {
				case <-r.Context().Done():
					endedOnce.Do(func() { close(requestEnded) })
					return
				case <-release:
				}
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
			return
		}
		if r.URL.Path == "/api2/json/version" {
			versionCalls.Add(1)
			if !strings.Contains(r.Header.Get("Cookie"), "PMGAuthCookie=fixture-ticket") {
				http.Error(w, "missing auth cookie", http.StatusForbidden)
				return
			}
			fmt.Fprint(w, `{"data":{"version":"8.3.1","release":"1"}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(release) })

	pin := sha256.Sum256(server.Certificate().Raw)
	cfg := ClientConfig{
		Host:        server.URL,
		User:        "fixture@pmg",
		Password:    "fixture-password",
		VerifySSL:   true,
		Fingerprint: hex.EncodeToString(pin[:]),
		Timeout:     5 * time.Second,
	}

	// 1. Pre-cancelled context fails immediately without sending a login request.
	preCancelled, preCancel := context.WithCancel(context.Background())
	preCancel()
	if client, err := NewClientWithContext(preCancelled, cfg); client != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled constructor returned client=%v err=%v", client, err)
	}
	if logins.Load() != 0 {
		t.Fatalf("pre-cancelled constructor still sent %d login requests", logins.Load())
	}

	// 2. Cancelling an in-flight password login unblocks construction and ends the request.
	inflightCtx, inflightCancel := context.WithCancel(context.Background())
	type result struct {
		client *Client
		err    error
	}
	done := make(chan result, 1)
	go func() {
		c, err := NewClientWithContext(inflightCtx, cfg)
		done <- result{c, err}
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("constructor did not reach PMG password login")
	}
	inflightCancel()

	select {
	case res := <-done:
		if res.client != nil || !errors.Is(res.err, context.Canceled) {
			t.Fatalf("cancelled constructor returned client=%v err=%v", res.client, res.err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled constructor did not return promptly")
	}
	select {
	case <-requestEnded:
	case <-time.After(time.Second):
		t.Fatal("cancelled constructor left HTTP login request active")
	}

	// 3. A subsequent construction succeeds and its client stays usable after its construction context ends.
	releaseOnce.Do(func() { close(release) })
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 5*time.Second)
	client, err := NewClientWithContext(buildCtx, cfg)
	if err != nil || client == nil {
		buildCancel()
		t.Fatalf("valid constructor failed: client=%v err=%v", client, err)
	}
	buildCancel()

	version, err := client.GetVersion(context.Background())
	if err != nil || version == nil || version.Version != "8.3.1" || versionCalls.Load() != 1 {
		t.Fatalf("retained client failed after construction context ended: version=%+v err=%v calls=%d", version, err, versionCalls.Load())
	}

	// 4. A mismatched TLS pin refuses before sending a password-bearing HTTP request.
	before := logins.Load()
	badPin := cfg
	badPin.Fingerprint = strings.Repeat("0", 64)
	if refused, err := NewClientWithContext(context.Background(), badPin); refused != nil || err == nil || logins.Load() != before {
		t.Fatalf("wrong TLS pin was not refused before login: client=%v err=%v logins=%d (before=%d)", refused, err, logins.Load(), before)
	}
}

func TestNewClientWithContextDoesNotLogInvalidURLCredentials(t *testing.T) {
	var buf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf)
	defer func() { log.Logger = origLogger }()

	client, err := NewClientWithContext(context.Background(), ClientConfig{
		Host: "http://admin:super-secret-pmg-pass@pmg.local:8006",
		User: "admin@pmg",
	})
	if client != nil || err == nil {
		t.Fatalf("expected invalid credential-bearing URL to be refused, got client=%v err=%v", client, err)
	}
	if strings.Contains(buf.String(), "super-secret-pmg-pass") {
		t.Fatalf("constructor logged secret from refused HTTP URL: %s", buf.String())
	}
}
