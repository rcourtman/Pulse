package proxmox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// End the owner while HTTP response bytes are consumed, rather than relying on
// the transport and scheduler to choose a particular cancellation race.
type constructionCancellationBody struct {
	io.Reader
	cancel context.CancelFunc
	closed *bool
}

func (r constructionCancellationBody) Read(p []byte) (int, error) {
	r.cancel()
	return r.Reader.Read(p)
}
func (r constructionCancellationBody) Close() error { *r.closed = true; return nil }

func testClientCancelledAuthResponseOwnsErrorAndSession(t *testing.T) {
	const good = `{"data":{"ticket":"recovered-ticket","CSRFPreventionToken":"recovered-csrf"}}`
	for _, encoding := range []string{"json", "form"} {
		for _, outcome := range []struct {
			name, body string
			status     int
			readError  bool
		}{
			{"invalid-success", `{}`, http.StatusOK, false},
			{"complete-success", good, http.StatusOK, false},
			{"encoding-refusal", `{}`, http.StatusUnsupportedMediaType, false},
			{"credential-refusal", `{}`, http.StatusForbidden, false},
			{"body-read-failure", good, http.StatusOK, true},
		} {
			t.Run(encoding+"/"+outcome.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls, reads, recovering, closed := 0, 0, false, false
				c := &Client{baseURL: "https://pve.fixture.invalid/api2/json",
					config: ClientConfig{Password: "fixture-password"},
					auth: auth{user: "fixture", realm: "pam", ticket: "retained-ticket",
						csrfToken: "retained-csrf", expiresAt: time.Now().Add(-time.Hour)},
					authGeneration: 3,
				}
				c.httpClient = &http.Client{Timeout: 5 * time.Second, Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path != "/api2/json/access/ticket" {
						reads++
						cookie, err := r.Cookie("PVEAuthCookie")
						if !recovering || err != nil || cookie.Value != "recovered-ticket" {
							t.Error("ordinary read used an ended or incoherent session")
						}
						return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"node":"fixture-node","status":"online"}]}`)), ContentLength: -1}, nil
					}
					calls++
					if encoding == "form" && r.Header.Get("Content-Type") == "application/json" {
						return &http.Response{StatusCode: http.StatusUnsupportedMediaType, Body: io.NopCloser(strings.NewReader(`{}`)), ContentLength: -1}, nil
					}
					if recovering {
						return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(good)), ContentLength: -1}, nil
					}
					var body io.Reader = strings.NewReader(outcome.body)
					if outcome.readError {
						body = sessionAdmissionReadError{body}
					}
					return &http.Response{StatusCode: outcome.status, Body: constructionCancellationBody{Reader: body, cancel: cancel, closed: &closed}, ContentLength: -1}, nil
				})}
				before, generation := c.authSnapshot()
				err := c.authenticate(ctx)
				after, nextGeneration := c.authSnapshot()
				if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(after, before) || nextGeneration != generation {
					t.Errorf("ended response lost caller or changed retained session: error=%v session_changed=%t generation_changed=%t", err, !reflect.DeepEqual(after, before), nextGeneration != generation)
				}
				want := 1
				if encoding == "form" {
					want = 2
				}
				if calls != want || reads != 0 || c.authInFlight != nil || !closed {
					t.Errorf("ended exchange replayed, read, leaked owner or left body open: calls=%d reads=%d pending=%t closed=%t", calls, reads, c.authInFlight != nil, closed)
				}
				recovering = true
				if err := c.authenticate(context.Background()); err != nil {
					t.Fatal(err)
				}
				recovered, recoveredGeneration := c.authSnapshot()
				if recovered.ticket != "recovered-ticket" || recovered.csrfToken != "recovered-csrf" || recoveredGeneration != generation+1 || calls != 2*want {
					t.Errorf("later authentication did not recover exactly once: generation=%d calls=%d", recoveredGeneration, calls)
				}
				nodes, err := c.GetNodes(context.Background())
				if err != nil || len(nodes) != 1 || nodes[0].Node != "fixture-node" || reads != 1 || calls != 2*want {
					t.Errorf("ordinary recovered read lost session or replayed: error=%v nodes=%d calls=%d reads=%d", err, len(nodes), calls, reads)
				}
			})
		}
	}
}
