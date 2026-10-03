package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

func startServiceHealthListener(t *testing.T, listener net.Listener, tlsEnabled bool, handler http.Handler) {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	_ = server.Listener.Close()
	server.Listener = listener
	if tlsEnabled {
		server.StartTLS()
	} else {
		server.Start()
	}
	t.Cleanup(server.Close)
}

func serveHealthyServiceHealthFixture(w http.ResponseWriter, r *http.Request) {
	response, _ := healthyServiceHealthResponse(r)
	defer response.Body.Close()
	_, _ = io.Copy(w, response.Body)
}

// Adapted from PR2351's real HTTP/HTTPS wildcard regression. Keep main's
// bounded alternate-family retry, but only on sockets that own both families.
func TestServiceHealthProbeHandlesWildcardListeners(t *testing.T) {
	for _, test := range []struct {
		name, network, address, wantHost string
		wantTargets                      int
	}{
		{"ipv4", "tcp4", "0.0.0.0:0", "127.0.0.1", 1},
		{"dual-stack", "tcp", "[::]:0", "127.0.0.1", 2},
		{"ipv6-only", "tcp6", "[::]:0", "::1", 1},
	} {
		for _, tlsEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/tls=%v", test.name, tlsEnabled), func(t *testing.T) {
				listener, err := net.Listen(test.network, test.address)
				if err != nil {
					t.Fatalf("required %s listener: %v", test.name, err)
				}
				startServiceHealthListener(t, listener, tlsEnabled, http.HandlerFunc(serveHealthyServiceHealthFixture))
				baseURLs := localServiceHealthBaseURLs(listener, tlsEnabled)
				if len(baseURLs) != test.wantTargets {
					t.Fatalf("probe targets = %v, want %d owned families", baseURLs, test.wantTargets)
				}
				parsed, err := url.Parse(baseURLs[0])
				if err != nil || parsed.Hostname() != test.wantHost {
					t.Fatalf("probe targets = %v, want bound listener through %s", baseURLs, test.wantHost)
				}
				got := newServiceHealthProbe(listener, tlsEnabled)()
				if !got.Observed || !got.Healthy || got.FailureCategory != "" {
					t.Fatalf("reachable listener reported unhealthy: %#v", got)
				}
			})
		}
	}
}

func TestServiceHealthWildcardNeverObservesOtherSocketOnSamePort(t *testing.T) {
	for _, network := range []string{"tcp4", "tcp6"} {
		for _, tlsEnabled := range []bool{false, true} {
			for _, ownHealthy := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/tls=%v/healthy=%v", network, tlsEnabled, ownHealthy), func(t *testing.T) {
					ownAddress, otherNetwork, otherHost := "0.0.0.0:0", "tcp6", "::1"
					if network == "tcp6" {
						ownAddress, otherNetwork, otherHost = "[::]:0", "tcp4", "127.0.0.1"
					}
					listener, err := net.Listen(network, ownAddress)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = listener.Close() })
					port := listener.Addr().(*net.TCPAddr).Port
					other, err := net.Listen(otherNetwork, net.JoinHostPort(otherHost, fmt.Sprint(port)))
					if err != nil {
						t.Fatalf("required separate socket on same port: %v", err)
					}
					var otherRequests atomic.Int32
					startServiceHealthListener(t, other, tlsEnabled, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						otherRequests.Add(1)
						if ownHealthy {
							http.Error(w, "unrelated server unavailable", http.StatusServiceUnavailable)
							return
						}
						serveHealthyServiceHealthFixture(w, r)
					}))
					startServiceHealthListener(t, listener, tlsEnabled, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if !ownHealthy {
							http.Error(w, "own server unavailable", http.StatusServiceUnavailable)
							return
						}
						serveHealthyServiceHealthFixture(w, r)
					}))
					got := newServiceHealthProbe(listener, tlsEnabled)()
					wantCategory := ""
					if !ownHealthy {
						wantCategory = telemetry.ServiceHealthFailureAPIStatus
					}
					if !got.Observed || got.Healthy != ownHealthy || got.FailureCategory != wantCategory || otherRequests.Load() != 0 {
						t.Fatalf("observation=%#v want healthy=%v category=%q; unrelated requests=%d", got, ownHealthy, wantCategory, otherRequests.Load())
					}
				})
			}
		}
	}
}

type serviceHealthSocketErrorListener struct {
	serviceHealthAddrListener
	raw syscall.RawConn
	err error
}

func (l serviceHealthSocketErrorListener) SyscallConn() (syscall.RawConn, error) { return l.raw, l.err }

type serviceHealthControlError struct{}

func (serviceHealthControlError) Control(func(uintptr)) error {
	return errors.New("control unavailable")
}
func (serviceHealthControlError) Read(func(uintptr) bool) error {
	return errors.New("read unavailable")
}
func (serviceHealthControlError) Write(func(uintptr) bool) error {
	return errors.New("write unavailable")
}

func TestServiceHealthUnknownSocketModeStaysOnIPv6(t *testing.T) {
	addressOnly := serviceHealthAddrListener{addr: &net.TCPAddr{IP: net.IPv6unspecified, Port: 7655}}
	for _, listener := range []net.Listener{
		addressOnly,
		serviceHealthSocketErrorListener{serviceHealthAddrListener: addressOnly, err: errors.New("socket unavailable")},
		serviceHealthSocketErrorListener{serviceHealthAddrListener: addressOnly, raw: serviceHealthControlError{}},
	} {
		if got := localServiceHealthBaseURLs(listener, false); !reflect.DeepEqual(got, []string{"http://[::1]:7655"}) {
			t.Fatalf("uninspectable socket targets = %v", got)
		}
	}
}
