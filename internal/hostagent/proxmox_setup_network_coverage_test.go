package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProxmoxSetup_getIPThatReachesPulse(t *testing.T) {
	tests := []struct {
		name         string
		pulseURL     string
		dialErr      error
		localAddr    net.Addr
		want         string
		wantDialAddr string
	}{
		{
			name:     "empty pulse URL returns empty",
			pulseURL: "",
			want:     "",
		},
		{
			name:     "invalid pulse URL returns empty",
			pulseURL: "://bad-url",
			want:     "",
		},
		{
			name:         "https infers port 443",
			pulseURL:     "https://pulse.example",
			localAddr:    &net.UDPAddr{IP: net.ParseIP("10.10.0.1")},
			want:         "10.10.0.1",
			wantDialAddr: "pulse.example:443",
		},
		{
			name:         "http infers port 80",
			pulseURL:     "http://pulse.example",
			localAddr:    &net.UDPAddr{IP: net.ParseIP("10.10.0.2")},
			want:         "10.10.0.2",
			wantDialAddr: "pulse.example:80",
		},
		{
			name:         "unknown scheme falls back to pulse default port",
			pulseURL:     "custom://pulse.example",
			localAddr:    &net.UDPAddr{IP: net.ParseIP("10.10.0.3")},
			want:         "10.10.0.3",
			wantDialAddr: "pulse.example:7655",
		},
		{
			name:         "dial timeout failure returns empty",
			pulseURL:     "https://pulse.example",
			dialErr:      errors.New("route lookup failed"),
			want:         "",
			wantDialAddr: "pulse.example:443",
		},
		{
			name:         "non UDP local address is ignored",
			pulseURL:     "https://pulse.example",
			localAddr:    &net.TCPAddr{IP: net.ParseIP("10.10.0.4"), Port: 1234},
			want:         "",
			wantDialAddr: "pulse.example:443",
		},
		{
			name:         "nil local address is ignored",
			pulseURL:     "https://pulse.example",
			localAddr:    nil,
			want:         "",
			wantDialAddr: "pulse.example:443",
		},
		{
			name:         "loopback IPv4 is ignored",
			pulseURL:     "https://pulse.example",
			localAddr:    &net.UDPAddr{IP: net.ParseIP("127.0.0.1")},
			want:         "",
			wantDialAddr: "pulse.example:443",
		},
		{
			name:         "loopback IPv6 is ignored",
			pulseURL:     "https://pulse.example",
			localAddr:    &net.UDPAddr{IP: net.ParseIP("::1")},
			want:         "",
			wantDialAddr: "pulse.example:443",
		},
		{
			name:         "link local IPv6 is ignored",
			pulseURL:     "https://pulse.example",
			localAddr:    &net.UDPAddr{IP: net.ParseIP("fe80::1")},
			want:         "",
			wantDialAddr: "pulse.example:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &mockCollector{}
			var gotDialNetwork string
			var gotDialAddr string
			var gotDialTimeout time.Duration

			if tt.wantDialAddr == "" {
				mc.dialTimeoutFn = func(network, address string, timeout time.Duration) (net.Conn, error) {
					t.Fatalf("DialTimeout should not be called for %q", tt.pulseURL)
					return nil, nil
				}
			} else {
				mc.dialTimeoutFn = func(network, address string, timeout time.Duration) (net.Conn, error) {
					gotDialNetwork = network
					gotDialAddr = address
					gotDialTimeout = timeout
					if tt.dialErr != nil {
						return nil, tt.dialErr
					}
					return &mockConn{localAddr: tt.localAddr}, nil
				}
			}

			p := &ProxmoxSetup{
				logger:    zerolog.Nop(),
				collector: mc,
				pulseURL:  tt.pulseURL,
			}

			got := p.getIPThatReachesPulse()
			if got != tt.want {
				t.Fatalf("getIPThatReachesPulse() = %q, want %q", got, tt.want)
			}

			if tt.wantDialAddr != "" {
				if gotDialNetwork != "udp" {
					t.Fatalf("DialTimeout network = %q, want %q", gotDialNetwork, "udp")
				}
				if gotDialAddr != tt.wantDialAddr {
					t.Fatalf("DialTimeout address = %q, want %q", gotDialAddr, tt.wantDialAddr)
				}
				if gotDialTimeout != 500*time.Millisecond {
					t.Fatalf("DialTimeout timeout = %v, want %v", gotDialTimeout, 500*time.Millisecond)
				}
			}
		})
	}
}

func TestProxmoxSetup_getIPForHostname(t *testing.T) {
	tests := []struct {
		name              string
		hostname          string
		collectorHostname string
		lookupErr         error
		lookupIPs         []net.IP
		want              string
		wantLookupHost    string
		wantLookupInvoked bool
	}{
		{
			name:              "uses explicit hostname and returns first IPv4",
			hostname:          "node-a",
			lookupIPs:         []net.IP{net.ParseIP("2001:db8::1"), net.ParseIP("10.0.0.40")},
			want:              "10.0.0.40",
			wantLookupHost:    "node-a",
			wantLookupInvoked: true,
		},
		{
			name:              "falls back to collector hostname",
			collectorHostname: "node-b",
			lookupIPs:         []net.IP{net.ParseIP("10.0.0.41")},
			want:              "10.0.0.41",
			wantLookupHost:    "node-b",
			wantLookupInvoked: true,
		},
		{
			name:              "empty hostname returns empty",
			collectorHostname: "",
			want:              "",
			wantLookupInvoked: false,
		},
		{
			name:              "lookup failure returns empty",
			hostname:          "node-c",
			lookupErr:         errors.New("dns failed"),
			want:              "",
			wantLookupHost:    "node-c",
			wantLookupInvoked: true,
		},
		{
			name:              "IPv6 only lookup returns empty",
			hostname:          "node-d",
			lookupIPs:         []net.IP{net.ParseIP("2001:db8::2")},
			want:              "",
			wantLookupHost:    "node-d",
			wantLookupInvoked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &mockCollector{
				hostnameFn: func() (string, error) {
					return tt.collectorHostname, nil
				},
			}

			lookupCalled := false
			gotLookupHost := ""
			mc.lookupIPFn = func(host string) ([]net.IP, error) {
				lookupCalled = true
				gotLookupHost = host
				return tt.lookupIPs, tt.lookupErr
			}

			p := &ProxmoxSetup{
				logger:    zerolog.Nop(),
				collector: mc,
				hostname:  tt.hostname,
			}

			got := p.getIPForHostname()
			if got != tt.want {
				t.Fatalf("getIPForHostname() = %q, want %q", got, tt.want)
			}

			if lookupCalled != tt.wantLookupInvoked {
				t.Fatalf("lookup invoked = %v, want %v", lookupCalled, tt.wantLookupInvoked)
			}

			if tt.wantLookupInvoked && gotLookupHost != tt.wantLookupHost {
				t.Fatalf("lookup host = %q, want %q", gotLookupHost, tt.wantLookupHost)
			}
		})
	}
}

func TestProxmoxSetup_registerWithPulse(t *testing.T) {
	t.Run("invalid pulse URL returns request creation error", func(t *testing.T) {
		p := &ProxmoxSetup{
			pulseURL: "http://bad host",
		}

		err := p.registerWithPulse(context.Background(), "pve", "https://10.0.0.1:8006", "token-id", "secret")
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "create request") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("transport failure returns wrapped error", func(t *testing.T) {
		p := &ProxmoxSetup{
			pulseURL:      "https://pulse.example",
			retryBackoffs: []time.Duration{}, // disable retries for unit test
			httpClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("network down")
				}),
			},
		}

		err := p.registerWithPulse(context.Background(), "pve", "https://10.0.0.1:8006", "token-id", "secret")
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "request failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("server status >= 400 returns error", func(t *testing.T) {
		p := &ProxmoxSetup{
			pulseURL:      "https://pulse.example",
			retryBackoffs: []time.Duration{}, // disable retries for unit test
			httpClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusBadGateway,
						Body:       io.NopCloser(strings.NewReader("bad gateway")),
						Header:     make(http.Header),
					}, nil
				}),
			},
		}

		err := p.registerWithPulse(context.Background(), "pve", "https://10.0.0.1:8006", "token-id", "secret")
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "502") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("success sends expected request body and headers", func(t *testing.T) {
		var gotReq *http.Request
		var gotPayload map[string]string

		p := &ProxmoxSetup{
			pulseURL: "https://pulse.example",
			apiToken: "api-token",
			hostname: "node-1",
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotReq = req

					bodyBytes, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read body: %v", err)
					}

					if err := json.Unmarshal(bodyBytes, &gotPayload); err != nil {
						t.Fatalf("unmarshal body: %v", err)
					}

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("ok")),
						Header:     make(http.Header),
					}, nil
				}),
			},
		}

		err := p.registerWithPulse(context.Background(), "pbs", "https://10.0.0.2:8007", "token-id", "secret")
		if err != nil {
			t.Fatalf("registerWithPulse() error = %v", err)
		}

		if gotReq == nil {
			t.Fatalf("expected request to be sent")
		}
		if gotReq.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", gotReq.Method, http.MethodPost)
		}
		if gotReq.URL.Path != "/api/auto-register" {
			t.Fatalf("path = %q, want %q", gotReq.URL.Path, "/api/auto-register")
		}
		if gotReq.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q, want %q", gotReq.Header.Get("Content-Type"), "application/json")
		}
		if gotReq.Header.Get("X-API-Token") != "api-token" {
			t.Fatalf("X-API-Token = %q, want %q", gotReq.Header.Get("X-API-Token"), "api-token")
		}

		wantPayload := map[string]string{
			"type":       "pbs",
			"host":       "https://10.0.0.2:8007",
			"serverName": "node-1",
			"tokenId":    "token-id",
			"tokenValue": "secret",
			"source":     "agent",
		}

		for key, want := range wantPayload {
			if gotPayload[key] != want {
				t.Fatalf("payload[%q] = %q, want %q", key, gotPayload[key], want)
			}
		}
	})
}
