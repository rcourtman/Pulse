package proxyfixture

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

// A fallback can dial again as soon as the prior response has been consumed,
// before the prior pipe goroutine has published its observation. Force reverse
// response completion to verify that observations still follow dial order.
func TestPipeObservationsFollowDialOrder(t *testing.T) {
	transport := &http.Transport{}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var release sync.Once
	unblock := func() { release.Do(func() { close(releaseFirst) }) }
	t.Cleanup(unblock)
	observed := Pipe(t, transport, nil, func(r *http.Request, _ []byte) (int, string) {
		if r.URL.Path == "/first" {
			close(firstStarted)
			<-releaseFirst
		}
		return http.StatusOK, r.URL.Path
	})
	read := func(path string) error {
		response, err := client.Get("http://fixture.invalid" + path)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- read("/first") }()
	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first request did not reach fixture")
	}
	if err := read("/second"); err != nil {
		t.Fatal(err)
	}

	// The first response is deliberately unfinished. No completion from the
	// second request may be attributed to the first request's slot.
	next := make(chan Observation, 1)
	go func() { next <- observed() }()
	var first Observation
	select {
	case first = <-next:
		t.Error("later response overtook an unfinished earlier observation")
		unblock()
	case <-time.After(50 * time.Millisecond):
		unblock()
		select {
		case first = <-next:
		case <-time.After(5 * time.Second):
			t.Fatal("first observation did not complete")
		}
	}
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	second := observed()
	for i, got := range []Observation{first, second} {
		want := []string{"/first", "/second"}[i]
		if got.Err != nil {
			t.Errorf("observation %d: %v", i, got.Err)
		}
		if got.URI != want {
			t.Errorf("observation %d URI=%q, want %q", i, got.URI, want)
		}
	}
}

// Reserving a slot must not discard a failed parse or response write. Client
// policy tests rely on wire failures as well as on complete HTTP observations.
func TestPipeRetainsWireErrors(t *testing.T) {
	for _, tc := range []struct{ name, request, wantURI string }{
		{"response-write", "GET /closed HTTP/1.1\r\nHost: fixture.invalid\r\n\r\n", "/closed"},
		{"request-parse", "not-an-http-request\r\n\r\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &http.Transport{}
			observed := Pipe(t, transport, nil, func(*http.Request, []byte) (int, string) {
				return http.StatusOK, "fixture response"
			})
			connection, err := transport.DialContext(context.Background(), "tcp", "fixture.invalid:80")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(connection, tc.request); err != nil {
				t.Fatal(err)
			}
			if err := connection.Close(); err != nil {
				t.Fatal(err)
			}
			got := observed()
			if got.Err == nil {
				t.Error("wire failure was lost")
			}
			if got.URI != tc.wantURI {
				t.Errorf("URI=%q, want %q", got.URI, tc.wantURI)
			}
		})
	}
}
