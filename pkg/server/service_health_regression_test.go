package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

// Override only Addr, retaining the real listener for serving. This models an
// IPv6 wildcard reported on an install where IPv6 loopback is unavailable.
type serviceHealthAddrListener struct {
	net.Listener
	addr net.Addr
}

func (l serviceHealthAddrListener) Addr() net.Addr { return l.addr }

type serviceHealthRoundTripper func(*http.Request) (*http.Response, error)

func (f serviceHealthRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func healthyServiceHealthResponse(r *http.Request) (*http.Response, error) {
	body := "app()"
	switch r.URL.Path {
	case "/api/health":
		body = `{"status":"healthy"}`
	case "/":
		body = `<html><script src="/assets/app.js"></script></html>`
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
}

func TestServiceHealthWildcardFamiliesAndExplicitAddresses(t *testing.T) {
	for _, tc := range []struct {
		name string
		addr net.Addr
		tls  bool
		want []string
	}{
		{"IPv6 wildcard", &net.TCPAddr{IP: net.IPv6unspecified, Port: 7655}, false, []string{"http://127.0.0.1:7655", "http://[::1]:7655"}},
		{"unspecified family", &net.TCPAddr{Port: 7655}, false, []string{"http://127.0.0.1:7655", "http://[::1]:7655"}},
		{"IPv4 wildcard", &net.TCPAddr{IP: net.IPv4zero, Port: 7655}, false, []string{"http://127.0.0.1:7655"}},
		{"explicit IPv4", &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 7655}, false, []string{"http://192.0.2.1:7655"}},
		{"explicit IPv6", &net.TCPAddr{IP: net.IPv6loopback, Port: 7655}, true, []string{"https://[::1]:7655"}},
		{"explicit scoped IPv6", &net.TCPAddr{IP: net.ParseIP("fe80::1"), Zone: "eth0", Port: 7655}, true, []string{"https://[fe80::1%25eth0]:7655"}},
		{"not TCP", &net.UnixAddr{Name: "/tmp/unused", Net: "unix"}, false, nil},
		{"no port", &net.TCPAddr{IP: net.IPv4zero}, false, nil},
		{"invalid port", &net.TCPAddr{IP: net.IPv4zero, Port: 65536}, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := localServiceHealthBaseURLs(serviceHealthAddrListener{addr: tc.addr}, tc.tls)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("targets = %#v, want %#v", got, tc.want)
			}
		})
	}
	if got := localServiceHealthBaseURLs(nil, false); got != nil {
		t.Fatalf("nil listener targets = %#v", got)
	}
}

func TestServiceHealthIPv4SurvivesUnavailableIPv6(t *testing.T) {
	for _, failure := range []error{errors.New("IPv6 disabled"), errors.New("IPv6 unreachable"), context.DeadlineExceeded} {
		client := &http.Client{Transport: serviceHealthRoundTripper(func(r *http.Request) (*http.Response, error) {
			if r.URL.Hostname() == "::1" {
				return nil, failure
			}
			if r.URL.Hostname() != "127.0.0.1" {
				t.Fatalf("non-loopback request: %s", r.URL)
			}
			return healthyServiceHealthResponse(r)
		})}
		// Red control: the former IPv6-only target cannot observe this healthy install.
		old := serviceHealthProbe([]string{"http://[::1]:7655"}, client, time.Second, 0)()
		if old.Healthy || !old.Observed {
			t.Fatalf("IPv6-only control = %#v", old)
		}
		targets := localServiceHealthBaseURLs(serviceHealthAddrListener{addr: &net.TCPAddr{IP: net.IPv6unspecified, Port: 7655}}, false)
		got := serviceHealthProbe(targets, client, time.Second, 0)()
		if !got.Observed || !got.Healthy || got.FailureCategory != "" {
			t.Fatalf("wildcard with available IPv4 = %#v", got)
		}
	}
}

func TestServiceHealthWildcardOnRealIPv4AndIPv6OnlyListeners(t *testing.T) {
	for _, network := range []string{"tcp4", "tcp6"} {
		t.Run(network, func(t *testing.T) {
			address := "127.0.0.1:0"
			if network == "tcp6" {
				address = "[::1]:0"
			}
			listener, err := net.Listen(network, address)
			if err != nil {
				t.Fatalf("required %s loopback listen: %v", network, err)
			}
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response, _ := healthyServiceHealthResponse(r)
				defer response.Body.Close()
				_, _ = io.Copy(w, response.Body)
			}), ReadHeaderTimeout: time.Second}
			done := make(chan struct{})
			go func() { defer close(done); _ = server.Serve(listener) }()
			t.Cleanup(func() { _ = server.Close(); <-done })
			bound := listener.Addr().(*net.TCPAddr)
			wildcard := serviceHealthAddrListener{Listener: listener, addr: &net.TCPAddr{IP: net.IPv6unspecified, Port: bound.Port}}
			got := newServiceHealthProbe(wildcard, false)()
			if !got.Observed || !got.Healthy || got.FailureCategory != "" {
				t.Fatalf("real %s server through wildcard = %#v", network, got)
			}
		})
	}
}

func TestServiceHealthStalledFamilyCannotStarveIPv6(t *testing.T) {
	var hosts []string
	client := &http.Client{Transport: serviceHealthRoundTripper(func(r *http.Request) (*http.Response, error) {
		hosts = append(hosts, r.URL.Hostname())
		if r.URL.Hostname() == "127.0.0.1" {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		if r.Context().Err() != nil {
			t.Fatalf("IPv6 has no reserved budget: %v", r.Context().Err())
		}
		return healthyServiceHealthResponse(r)
	})}
	got := serviceHealthProbe([]string{"http://127.0.0.1:7655", "http://[::1]:7655"}, client, 200*time.Millisecond, 0)()
	if !got.Healthy || !reflect.DeepEqual(hosts, []string{"127.0.0.1", "::1", "::1", "::1"}) {
		t.Fatalf("observation = %#v, requested hosts = %#v", got, hosts)
	}
}

func TestServiceHealthSlowStartupRetriesOnceAndRecovers(t *testing.T) {
	var apiCalls int
	var firstDeadline time.Time
	client := &http.Client{Transport: serviceHealthRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/health" {
			apiCalls++
			deadline, ok := r.Context().Deadline()
			if !ok {
				t.Fatal("unbounded probe")
			}
			if apiCalls == 1 {
				firstDeadline = deadline
				<-r.Context().Done()
				return nil, r.Context().Err()
			}
			if !deadline.After(firstDeadline) || r.Context().Err() != nil {
				t.Fatal("retry reused exhausted startup budget")
			}
		}
		return healthyServiceHealthResponse(r)
	})}
	got := serviceHealthProbe([]string{"http://127.0.0.1:7655"}, client, 50*time.Millisecond, time.Millisecond)()
	if !got.Healthy || apiCalls != 2 {
		t.Fatalf("observation = %#v, API calls = %d", got, apiCalls)
	}
}

func TestServiceHealthFinalTimeoutAtEveryStageIsBounded(t *testing.T) {
	for _, stage := range []string{"/api/health", "/", "/assets/app.js"} {
		t.Run(stage, func(t *testing.T) {
			apiCalls, timeouts := 0, 0
			client := &http.Client{Transport: serviceHealthRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/api/health" {
					apiCalls++
				}
				if r.URL.Path == stage {
					timeouts++
					if _, ok := r.Context().Deadline(); !ok {
						t.Fatal("unbounded request")
					}
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				return healthyServiceHealthResponse(r)
			})}
			got := serviceHealthProbe([]string{"http://127.0.0.1:7655"}, client, 20*time.Millisecond, 0)()
			if !got.Observed || got.Healthy || got.FailureCategory != telemetry.ServiceHealthFailureTimeout || apiCalls != 2 || timeouts != 2 {
				t.Fatalf("observation = %#v, API calls/timeouts = %d/%d", got, apiCalls, timeouts)
			}
		})
	}
}

func TestServiceHealthDoesNotHideGenuineFailuresOnAnotherFamily(t *testing.T) {
	for _, tc := range []struct {
		name, path, body, want string
		status                 int
		err                    error
	}{
		{name: "connectivity", path: "/api/health", err: errors.New("refused"), want: telemetry.ServiceHealthFailureAPIConnectivity},
		{name: "API status", path: "/api/health", status: 503, body: `{"status":"unhealthy"}`, want: telemetry.ServiceHealthFailureAPIStatus},
		{name: "API invalid JSON", path: "/api/health", status: 200, body: "invalid", want: telemetry.ServiceHealthFailureAPIStatus},
		{name: "API unhealthy body", path: "/api/health", status: 200, body: `{"status":"unhealthy"}`, want: telemetry.ServiceHealthFailureAPIStatus},
		{name: "API oversized body", path: "/api/health", status: 200, body: strings.Repeat("x", serviceHealthBodyLimit+1), want: telemetry.ServiceHealthFailureAPIStatus},
		{name: "UI status", path: "/", status: 404, want: telemetry.ServiceHealthFailureUIStatus},
		{name: "UI invalid HTML", path: "/", status: 200, body: "not HTML", want: telemetry.ServiceHealthFailureUIStatus},
		{name: "UI oversized body", path: "/", status: 200, body: strings.Repeat("x", serviceHealthBodyLimit+1), want: telemetry.ServiceHealthFailureUIStatus},
		{name: "missing asset references", path: "/", status: 200, body: "<html></html>", want: telemetry.ServiceHealthFailureFrontendAssets},
		{name: "asset status", path: "/assets/app.js", status: 404, want: telemetry.ServiceHealthFailureFrontendAssets},
		{name: "asset empty", path: "/assets/app.js", status: 200, want: telemetry.ServiceHealthFailureFrontendAssets},
		{name: "asset oversized", path: "/assets/app.js", status: 200, body: strings.Repeat("x", serviceHealthBodyLimit+1), want: telemetry.ServiceHealthFailureFrontendAssets},
	} {
		t.Run(tc.name, func(t *testing.T) {
			apiCalls, ipv6Calls := 0, 0
			client := &http.Client{Transport: serviceHealthRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/api/health" {
					apiCalls++
				}
				if r.URL.Hostname() == "::1" {
					ipv6Calls++
					if tc.err == nil {
						t.Fatal("response failure switched listener family")
					}
				}
				if r.URL.Path == tc.path {
					if tc.err != nil {
						return nil, tc.err
					}
					return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
				}
				return healthyServiceHealthResponse(r)
			})}
			got := serviceHealthProbe([]string{"http://127.0.0.1:7655", "http://[::1]:7655"}, client, time.Second, 0)()
			wantAPICalls := 2
			if tc.err != nil {
				wantAPICalls = 4
			}
			if !got.Observed || got.Healthy || got.FailureCategory != tc.want || apiCalls != wantAPICalls {
				t.Fatalf("observation = %#v, API calls = %d, IPv6 calls = %d", got, apiCalls, ipv6Calls)
			}
		})
	}
}

func TestServiceHealthHTTPSAndRedirectRefusal(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, _ := healthyServiceHealthResponse(r)
		defer response.Body.Close()
		_, _ = io.Copy(w, response.Body)
	}))
	t.Cleanup(server.Close)
	if got := newServiceHealthProbe(server.Listener, true)(); !got.Healthy {
		t.Fatalf("HTTPS service = %#v", got)
	}

	var redirected atomic.Int32
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	t.Cleanup(external.Close)
	for _, stage := range []string{"/api/health", "/", "/assets/app.js"} {
		t.Run(stage, func(t *testing.T) {
			local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == stage {
					http.Redirect(w, r, external.URL, http.StatusFound)
					return
				}
				response, _ := healthyServiceHealthResponse(r)
				defer response.Body.Close()
				_, _ = io.Copy(w, response.Body)
			}))
			t.Cleanup(local.Close)
			got := newServiceHealthProbe(local.Listener, false)()
			if got.Healthy || redirected.Load() != 0 {
				t.Fatalf("redirect followed/accepted: %#v, external requests = %d", got, redirected.Load())
			}
		})
	}
}

func TestServiceHealthConcurrentObservations(t *testing.T) {
	probe := serviceHealthProbe([]string{"http://127.0.0.1:7655"}, &http.Client{Transport: serviceHealthRoundTripper(healthyServiceHealthResponse)}, time.Second, 0)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := probe(); !got.Healthy {
				t.Errorf("concurrent observation = %#v", got)
			}
		}()
	}
	wg.Wait()
}

func TestFrontendAssetPathsEnforceExistingBounds(t *testing.T) {
	var index strings.Builder
	for i := 0; i < serviceHealthAssetLimit+10; i++ {
		fmt.Fprintf(&index, `<script src="/assets/%d.js"></script>`, i)
	}
	if paths := frontendAssetPaths([]byte(index.String())); len(paths) != serviceHealthAssetLimit {
		t.Fatalf("asset count = %d, want %d", len(paths), serviceHealthAssetLimit)
	}
	if paths := frontendAssetPaths([]byte(`<script src="/assets/a.js"></script><script src="/assets/a.js"></script><script src="https://outside.example/x.js"></script><script src="//outside.example/x.js"></script>`)); !reflect.DeepEqual(paths, []string{"/assets/a.js"}) {
		t.Fatalf("duplicate/external paths escaped: %#v", paths)
	}
}
