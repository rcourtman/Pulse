package proxmox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReplicationLifecycleInventoryEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{{"empty", `{"data":[]}`, true}, {"missing", `{}`, false}, {"null", `{"data":null}`, false}, {"wrong-shape", `{"data":{}}`, false}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
			defer server.Close()
			client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "reader@pve!manual", TokenValue: "synthetic"})
			if err != nil {
				t.Fatal(err)
			}
			jobs, err := client.GetReplicationStatus(context.Background())
			if tc.valid {
				if err != nil || jobs == nil || len(jobs) != 0 {
					t.Fatalf("valid explicit empty: jobs=%+v err=%v", jobs, err)
				}
			} else if err == nil {
				t.Fatalf("invalid envelope accepted as complete empty: %+v", jobs)
			}
		})
	}
}
func TestReplicationLifecycleHTTPCancellationStopsEnrichment(t *testing.T) {
	started := make(chan struct{})
	var statusReads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/cluster/replication" {
			fmt.Fprint(w, `{"data":[{"id":"100-0","source":"node1"},{"id":"101-0","source":"node1"}]}`)
			return
		}
		if statusReads.Add(1) == 1 {
			close(started)
		}
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "reader@pve!manual", TokenValue: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var jobs []ReplicationJob
	var readErr error
	go func() { defer close(done); jobs, readErr = client.GetReplicationStatus(ctx) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("status read not reached")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("client did not terminate")
	}
	if !errors.Is(readErr, context.Canceled) || jobs != nil {
		t.Fatalf("canceled read returned publishable config: jobs=%+v err=%v", jobs, readErr)
	}
	if statusReads.Load() != 1 {
		t.Fatalf("cancellation attempted more status reads: %d", statusReads.Load())
	}
}

type replicationEOFBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *replicationEOFBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err == io.EOF {
		b.cancel()
	}
	return n, err
}

type replicationTransport func(*http.Request) (*http.Response, error)

func (f replicationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReplicationLifecycleCancellationAfterFinalBody(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := NewClient(ClientConfig{Host: "http://example.invalid:8006", TokenName: "reader@pve!manual", TokenValue: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient.Transport = replicationTransport(func(r *http.Request) (*http.Response, error) {
		body := io.NopCloser(strings.NewReader(`{"data":[{"id":"100-0","source":"node1"}]}`))
		if r.URL.Path != "/api2/json/cluster/replication" {
			body = &replicationEOFBody{io.NopCloser(strings.NewReader(`{"data":{"last_sync":1791330000,"fail_count":0}}`)), cancel}
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body, Request: r}, nil
	})
	jobs, err := client.GetReplicationStatus(ctx)
	if !errors.Is(err, context.Canceled) || jobs != nil {
		t.Fatalf("final-body cancellation returned fresh success: jobs=%+v err=%v", jobs, err)
	}
}
