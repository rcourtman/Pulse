package proxmox

import (
	"context"
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
)

type guestStatusObservation struct {
	status     string
	cpu        float64
	mem        uint64
	observedAt time.Time
	counters   IOCounterPresence
	meminfo    *VMMemInfo
}

func guestStatusTestReader(t *testing.T, endpoint, kind, route string) func(context.Context) (*guestStatusObservation, error) {
	t.Helper()
	var client interface {
		GetVMStatus(context.Context, string, int) (*VMStatus, error)
		GetContainerStatus(context.Context, string, int) (*Container, error)
	}
	cfg := ClientConfig{Host: endpoint, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 30 * time.Second}
	if kind == "cluster" {
		cluster := NewClusterClient("guest-status-fixture", cfg, []string{endpoint}, nil)
		client = cluster
		t.Cleanup(func() {
			for _, c := range cluster.clients {
				c.httpClient.CloseIdleConnections()
			}
		})
	} else {
		single, err := NewClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
		client = single
		t.Cleanup(single.httpClient.CloseIdleConnections)
	}
	return func(ctx context.Context) (*guestStatusObservation, error) {
		if route == "qemu" {
			value, err := client.GetVMStatus(ctx, "node", 105)
			if value == nil {
				return nil, err
			}
			return &guestStatusObservation{value.Status, value.CPU, value.Mem, value.ObservedAt, value.IOCounters, value.MemInfo}, err
		}
		value, err := client.GetContainerStatus(ctx, "node", 105)
		if value == nil {
			return nil, err
		}
		return &guestStatusObservation{value.Status, value.CPU, value.Mem, value.ObservedAt, value.IOCounters, nil}, err
	}
}

// The old readers accepted these prefixes/envelopes and stamped them as fresh.
// Exercise the public single and cluster methods, not just a JSON helper.
func TestGuestDetailedStatusRejectsIncompleteEvidence(t *testing.T) {
	const healthy = `{"data":{"status":"running","cpu":0.25,"mem":400,"diskread":0,"netin":42}}`
	tests := []struct {
		name string
		body string
	}{
		{"missing-data", `{}`},
		{"null-data", `{"data":null}`},
		{"scalar-data", `{"data":42}`},
		{"array-data", `{"data":[]}`},
		{"empty-data", `{"data":{}}`},
		{"missing-status", `{"data":{"cpu":0.25,"mem":400}}`},
		{"null-status", `{"data":{"status":null}}`},
		{"empty-status", `{"data":{"status":" \t"}}`},
		{"duplicate-data", `{"data":{"status":"stopped","lock":"backup"},"data":{"status":"running"}}`},
		{"escaped-duplicate-data", `{"data":{"status":"stopped"},"\u0064ata":{"status":"running"}}`},
		{"case-data", `{"data":{"status":"stopped"},"DATA":{"status":"running"}}`},
		{"competing-errors", `{"data":{"status":"running"},"errors":{"private-proxy-detail":"not-an-observation"}}`},
		{"case-errors", `{"data":{"status":"running"},"ERRORS":{"private-proxy-detail":"not-an-observation"}}`},
		{"duplicate-status", `{"data":{"status":"stopped","status":"running"}}`},
		{"case-status", `{"data":{"status":"stopped","STATUS":"running"}}`},
		{"case-lock", `{"data":{"status":"running","lock":"backup","LOCK":""}}`},
		{"duplicate-mem", `{"data":{"status":"running","mem":900,"mem":0}}`},
		{"case-counter", `{"data":{"status":"running","diskread":900,"DISKREAD":0}}`},
		{"case-balloon", `{"data":{"status":"running","ballooninfo":{},"BALLOONINFO":null}}`},
		{"invalid-utf8", "{\"data\":{\"status\":\"running\",\"name\":\"\xff\"}}"},
		{"trailing-json", healthy + `{"data":{"status":"stopped"}}`},
		{"trailing-garbage", healthy + `private-proxy-detail`},
		{"short-content-length", healthy},
		{"chunked-over-limit", healthy},
	}
	for _, kind := range []string{"single", "cluster"} {
		for _, route := range []string{"qemu", "lxc"} {
			for _, tc := range tests {
				t.Run(kind+"/"+route+"/"+tc.name, func(t *testing.T) {
					var corrected atomic.Bool
					var statusCalls, otherCalls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						if strings.HasSuffix(r.URL.Path, "/nodes") {
							fmt.Fprint(w, `{"data":[]}`)
							return
						}
						if !strings.HasSuffix(r.URL.Path, "/"+route+"/105/status/current") {
							otherCalls.Add(1)
							http.NotFound(w, r)
							return
						}
						statusCalls.Add(1)
						if corrected.Load() {
							fmt.Fprint(w, healthy)
							return
						}
						if tc.name == "short-content-length" {
							w.Header().Set("Content-Length", "1000")
						}
						fmt.Fprint(w, tc.body)
						if tc.name == "chunked-over-limit" {
							w.(http.Flusher).Flush()
							fmt.Fprint(w, strings.Repeat(" ", int(maxResponseBodyBytes)))
						}
					}))
					defer server.Close()
					read := guestStatusTestReader(t, server.URL+"/status-proof/"+tc.name, kind, route)
					value, err := read(context.Background())
					if err == nil || value != nil {
						t.Errorf("invalid detailed status became a fresh observation: %+v / %v", value, err)
					}
					if err != nil && strings.Contains(err.Error(), "private-proxy-detail") {
						t.Error("malformed success copied private response contents into diagnostics")
					}
					corrected.Store(true)
					before := time.Now()
					value, err = read(context.Background())
					if err != nil || value == nil || value.status != "running" || value.cpu != .25 || value.mem != 400 || value.observedAt.Before(before) || value.observedAt.After(time.Now()) {
						t.Fatalf("ordinary complete status did not recover: %+v / %v", value, err)
					}
					if !value.counters.Explicit || !value.counters.DiskRead || !value.counters.NetworkIn || value.counters.DiskWrite || value.counters.NetworkOut || statusCalls.Load() != 2 || otherCalls.Load() != 0 {
						t.Fatal("recovery invented counters, repeated reads or sent a guest/config request")
					}
				})
			}
		}
	}
}

func TestGuestDetailedStatusRequiresHTTP200(t *testing.T) {
	for _, route := range []string{"qemu", "lxc"} {
		for _, code := range []int{http.StatusCreated, http.StatusPartialContent} {
			t.Run(fmt.Sprintf("%s/%d", route, code), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(code)
					fmt.Fprint(w, `{"data":{"status":"running"}}`)
				}))
				defer server.Close()
				value, err := guestStatusTestReader(t, server.URL, "single", route)(context.Background())
				if err == nil || value != nil {
					t.Fatalf("HTTP %d was a detailed status observation: %+v / %v", code, value, err)
				}
			})
		}
	}
}

func TestGuestDetailedStatusWaitsForBodyAndRetainsDeadline(t *testing.T) {
	for _, route := range []string{"qemu", "lxc"} {
		for _, complete := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/complete=%t", route, complete), func(t *testing.T) {
				entered, finish := make(chan struct{}), make(chan struct{})
				var release sync.Once
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, `{"data":{"status":"running","cpu":0}}`)
					w.(http.Flusher).Flush()
					close(entered)
					select {
					case <-finish:
						fmt.Fprint(w, " \n")
					case <-r.Context().Done():
					}
				}))
				defer server.Close()
				defer release.Do(func() { close(finish) })
				read := guestStatusTestReader(t, server.URL, "single", route)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				type result struct {
					value *guestStatusObservation
					err   error
				}
				done := make(chan result, 1)
				go func() { value, err := read(ctx); done <- result{value, err} }()
				<-entered
				select {
				case got := <-done:
					t.Fatalf("status published before body completion: %+v / %v", got.value, got.err)
				case <-time.After(25 * time.Millisecond):
				}
				if complete {
					completedAt := time.Now()
					release.Do(func() { close(finish) })
					got := <-done
					if got.err != nil || got.value == nil || got.value.cpu != 0 || got.value.observedAt.Before(completedAt) {
						t.Fatalf("complete body did not own observation time: %+v / %v", got.value, got.err)
					}
				} else {
					cancel()
					got := <-done
					if got.value != nil || !errors.Is(got.err, context.Canceled) {
						t.Fatalf("caller cancellation became a fresh observation: %+v / %v", got.value, got.err)
					}
				}
			})
		}
	}
}

func TestGuestDetailedStatusPreservesSparseZeroAndAdditiveFields(t *testing.T) {
	for _, route := range []string{"qemu", "lxc"} {
		for _, body := range []string{
			`{"data":{"status":"stopped"}}`,
			`{"data":{"status":"provider-future-state"},"provider-metadata":{"value":1}}`,
			`{"data":{"\u0073tatus":"running","cpu":0,"mem":0,"diskread":0,"agent":{"enabled":1,"available":0},"meminfo":{"total":1000,"available":0},"future-metadata":{"STATUS":"not-the-status"}},"errors":null}`,
		} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			read := guestStatusTestReader(t, server.URL, "single", route)
			value, err := read(context.Background())
			server.Close()
			if err != nil || value == nil || value.observedAt.IsZero() {
				t.Fatalf("valid sparse/additive status rejected: %+v / %v", value, err)
			}
			if strings.Contains(body, `"available":0`) && (value.mem != 0 || !value.counters.Explicit || !value.counters.DiskRead || value.counters.DiskWrite || route == "qemu" && (value.meminfo == nil || !value.meminfo.HasAvailable() || value.meminfo.Available != 0)) {
				t.Fatal("measured zero or omitted-field presence was lost")
			}
		}
	}
}

type guestStatusCloseFailure struct{ io.Reader }

func (guestStatusCloseFailure) Close() error { return errors.New("fixture-close-failure") }

func TestGuestDetailedStatusRejectsCloseFailure(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusOK, Body: guestStatusCloseFailure{strings.NewReader(`{"data":{"status":"running"}}`)}}
	if value, err := readGuestStatus[VMStatus](resp); value != nil || err == nil {
		t.Fatal("body close failure became a completed observation")
	}
}
