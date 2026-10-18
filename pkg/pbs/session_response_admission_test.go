package pbs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type sessionAdmissionReadError struct{ io.Reader }

func (r sessionAdmissionReadError) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

// Synthetic response credentials never come from an installed service. A 200
// alone must not replace a session or refresh its expiry/generation.
func TestClientSessionResponseAdmission(t *testing.T) {
	const good = `{"data":{"ticket":"fixture-ticket:+/=_-","CSRFPreventionToken":"fixture-csrf:+/=_-","username":"fixture@pam","cap":{"VM.Audit":1}}}`
	tests := []struct {
		name, body       string
		valid, readError bool
	}{
		{"complete-session", good, true, false},
		{"complete-whitespace", " \n" + good + "\t", true, false},
		{"missing-data", `{}`, false, false},
		{"null-data", `{"data":null}`, false, false},
		{"empty-data", `{"data":{}}`, false, false},
		{"missing-ticket", `{"data":{"CSRFPreventionToken":"fixture-sensitive-csrf"}}`, false, false},
		{"null-ticket", `{"data":{"ticket":null,"CSRFPreventionToken":"fixture-sensitive-csrf"}}`, false, false},
		{"blank-ticket", `{"data":{"ticket":" ","CSRFPreventionToken":"fixture-sensitive-csrf"}}`, false, false},
		{"missing-csrf", `{"data":{"ticket":"fixture-sensitive-ticket"}}`, false, false},
		{"null-csrf", `{"data":{"ticket":"fixture-sensitive-ticket","CSRFPreventionToken":null}}`, false, false},
		{"empty-csrf", `{"data":{"ticket":"fixture-sensitive-ticket","CSRFPreventionToken":""}}`, false, false},
		{"wrong-type-csrf", `{"data":{"ticket":"fixture-sensitive-ticket","CSRFPreventionToken":42}}`, false, false},
		{"duplicate-data", `{"data":null,"data":{"ticket":"fixture-sensitive-ticket","CSRFPreventionToken":"fixture-sensitive-csrf"}}`, false, false},
		{"duplicate-ticket", `{"data":{"ticket":"","ticket":"fixture-sensitive-ticket","CSRFPreventionToken":"fixture-sensitive-csrf"}}`, false, false},
		{"duplicate-csrf", `{"data":{"ticket":"fixture-sensitive-ticket","CSRFPreventionToken":"","CSRFPreventionToken":"fixture-sensitive-csrf"}}`, false, false},
		{"escaped-duplicate", `{"data":{"ticket":"","ti\u0063ket":"fixture-sensitive-ticket","CSRFPreventionToken":"fixture-sensitive-csrf"}}`, false, false},
		{"noncanonical-data", strings.Replace(good, `"data"`, `"Data"`, 1), false, false},
		{"noncanonical-ticket", strings.Replace(good, `"ticket"`, `"Ticket"`, 1), false, false},
		{"noncanonical-csrf", strings.Replace(good, `"CSRFPreventionToken"`, `"csrfpreventiontoken"`, 1), false, false},
		{"conflicting-case", strings.Replace(good, `"ticket":`, `"Ticket":null,"ticket":`, 1), false, false},
		{"trailing-object", good + ` {"data":null}`, false, false},
		{"trailing-invalid", good + ` not-json`, false, false},
		{"unfinished-body", good + ` `, false, true},
		{"oversized-tail", good + strings.Repeat(" ", int(maxResponseBodyBytes)), false, false},
		{"broken-utf8", strings.Replace(good, "fixture-ticket", "fixture-"+string([]byte{0xff}), 1), false, false},
		{"cookie-injection", strings.Replace(good, "fixture-ticket:+/=_-", `fixture-sensitive-ticket;other=authority`, 1), false, false},
		{"header-injection", strings.Replace(good, "fixture-csrf:+/=_-", `fixture-sensitive-csrf\r\nAuthorization: other`, 1), false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{auth: auth{user: "fixture", realm: "pam", ticket: "retained-ticket", csrfToken: "retained-csrf", expiresAt: time.Now().Add(-time.Hour)}}

			before := c.auth
			var body io.Reader = strings.NewReader(tc.body)
			if tc.readError {
				body = sessionAdmissionReadError{body}
			}
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(body), ContentLength: -1}
			err := c.handleAuthResponse(context.Background(), resp)
			resp.Body.Close()
			if !tc.valid {
				if err == nil {
					t.Error("unusable or incomplete 200 admitted as a session")
				}
				if !reflect.DeepEqual(c.auth, before) {
					t.Error("rejected success replaced retained ticket/CSRF/expiry")
				}

				if shouldFallbackToForm(err) {
					t.Error("invalid success authorised another login encoding")
				}
				if err != nil && (strings.Contains(err.Error(), "fixture-sensitive") || strings.Contains(err.Error(), "fixture-ticket") || strings.Contains(err.Error(), "fixture-csrf")) {
					t.Error("response credentials leaked into validation error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.auth.ticket != "fixture-ticket:+/=_-" || c.auth.csrfToken != "fixture-csrf:+/=_-" || time.Until(c.auth.expiresAt) < 119*time.Minute || time.Until(c.auth.expiresAt) > 2*time.Hour {
				t.Error("complete session or normal lifetime changed")
			}

		})
	}
}

// Expired ordinary reads all share the same invalid success, retain their old
// tuple and dispatch no API work. A distinct later read can accept a complete
// session without resetting credentials, clocks, identities or safety fences.
func TestClientSessionInvalidSuccessRecovery(t *testing.T) {
	const readers = 8
	var logins, reads, forms atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path == "/api2/json/access/ticket" {
			if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
				forms.Add(1)
			}
			n := logins.Add(1)
			if n == 2 {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return
				}
				fmt.Fprint(w, `{"data":{"ticket":"fixture-sensitive-ticket"}}`)
				return
			}
			fmt.Fprintf(w, `{"data":{"ticket":"session-%d","CSRFPreventionToken":"csrf-%d"}}`, n, n)
			return
		}
		reads.Add(1)
		cookie, err := r.Cookie("PBSAuthCookie")
		if err != nil || cookie.Value != "session-3" || (r.Method != http.MethodGet && r.Header.Get("CSRFPreventionToken") != "csrf-3") {
			t.Error("ordinary recovered read lost ticket/CSRF coherence")
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, `{"data":{"cpu":0.25,"memory":{"total":1024,"used":256},"uptime":60}}`)
	}))
	defer server.Close()
	c, err := NewClient(ClientConfig{Host: server.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: func() string { pin := sha256.Sum256(server.Certificate().Raw); return hex.EncodeToString(pin[:]) }(), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer c.httpClient.CloseIdleConnections()
	c.authMu.Lock()
	c.auth.expiresAt = time.Now().Add(-time.Hour)
	before := c.auth
	c.authMu.Unlock()
	done := make(chan error, readers)
	go func() { _, err := c.GetNodeStatus(ctx); done <- err }()
	pbsSessionAwait(t, entered)
	for i := 1; i < readers; i++ {
		joined := &pbsSessionJoinContext{Context: ctx, joined: make(chan struct{})}
		go func() { _, err := c.GetNodeStatus(joined); done <- err }()
		pbsSessionAwait(t, joined.joined)
	}
	close(release)
	for i := 0; i < readers; i++ {
		if err := <-done; err == nil {
			t.Error("joined expired read advanced past unusable login success")
		} else if strings.Contains(err.Error(), "fixture-sensitive") {
			t.Error("failed renewal error contains session bytes")
		}
	}
	c.authMu.Lock()
	unchanged := reflect.DeepEqual(c.auth, before)
	c.authMu.Unlock()
	if !unchanged || logins.Load() != 2 || reads.Load() != 0 || forms.Load() != 0 {
		t.Errorf("invalid success changed session or dispatched work: same=%t auth/API/form=%d/%d/%d", unchanged, logins.Load(), reads.Load(), forms.Load())
	}
	if _, err := c.GetNodeStatus(ctx); err != nil {
		t.Fatal(err)
	}
	resp, err := c.request(ctx, http.MethodPost, "/synthetic-read", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 3 || reads.Load() != 2 || forms.Load() != 0 {
		t.Errorf("later ordinary recovery auth/API/form=%d/%d/%d", logins.Load(), reads.Load(), forms.Load())
	}
	t.Log("eight joined failures, zero API/form dispatches, retained tuple; one later login plus coherent ordinary GET/POST recovery")
}
