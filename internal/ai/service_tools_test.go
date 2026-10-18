package ai

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestFetchURL(t *testing.T) {
	os.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")
	defer os.Unsetenv("PULSE_AI_ALLOW_LOOPBACK")

	// Start a local test server on IPv4 to avoid environments that disallow tcp6 binds.
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, world")
	}))
	ts.Listener = l
	ts.Start()
	defer ts.Close()

	svc := NewService(nil, nil)
	ctx := context.Background()

	// Test successful fetch
	result, err := svc.fetchURL(ctx, ts.URL)
	if err != nil {
		t.Fatalf("fetchURL failed: %v", err)
	}
	if !containsString(result, "Hello, world") {
		t.Errorf("Expected 'Hello, world' in result, got: %s", result)
	}

	// Test blocked host (localhost)
	_, err = svc.fetchURL(ctx, "http://localhost:8080")
	if err == nil || !containsString(err.Error(), "blocked") {
		t.Errorf("Expected blocked host error, got: %v", err)
	}

	// Test invalid URL
	_, err = svc.fetchURL(ctx, "not-a-url")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	// Test scheme check
	_, err = svc.fetchURL(ctx, "ftp://example.com")
	if err == nil || !containsString(err.Error(), "only http/https") {
		t.Errorf("Expected scheme error, got: %v", err)
	}
}

func TestParseAndValidateFetchURL(t *testing.T) {
	ctx := context.Background()
	// Exercise the actual hostname resolution/SSRF path without requiring
	// external DNS or weakening production address checks in offline CI.
	oldResolver := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: fetchURLTestDNSDial(t)}
	t.Cleanup(func() { net.DefaultResolver = oldResolver })

	tests := []struct {
		url     string
		wantErr bool
		errSub  string
	}{
		{"http://public.example.test", false, ""},
		{"https://public.example.test/path", false, ""},
		{"  http://public.example.test  ", false, ""},
		{"http://8.8.8.8/path", false, ""},
		{"http://private.example.test", true, "blocked address"},
		{"http://mixed.example.test", true, "blocked address"},
		{"http://missing.example.test", true, "failed to resolve host"},
		{"", true, "url is required"},
		{"http://localhost", true, "blocked"},
		{"http://localhost.", true, "blocked"},
		{"http://127.0.0.1", true, "blocked"},
		{"http://[::1]", true, "blocked"},
		{"ftp://example.com", true, "only http/https"},
		{"http://user:pass@example.com", true, "credentials"},
		{"http://example.com/#frag", true, "fragments"},
		{"http://", true, "host"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			_, err := parseAndValidateFetchURL(ctx, tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseAndValidateFetchURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errSub != "" && !containsString(err.Error(), tt.errSub) {
				t.Errorf("error %v does not contain %q", err, tt.errSub)
			}
		})
	}
}

// Go's resolver accepts a stream Conn from Dial even for its initial UDP
// query. These framed in-process responses cannot contact the configured DNS
// server. Both A and AAAA remain real parsed responses, including mixed sets.
func fetchURLTestDNSDial(t *testing.T) func(context.Context, string, string) (net.Conn, error) {
	t.Helper()
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		client, server := net.Pipe()
		_ = server.SetDeadline(time.Now().Add(2 * time.Second))
		stop := context.AfterFunc(ctx, func() { _ = server.Close() })
		go func() {
			defer stop()
			defer server.Close()
			var size [2]byte
			if _, err := io.ReadFull(server, size[:]); err != nil {
				return
			}
			query := make([]byte, binary.BigEndian.Uint16(size[:]))
			if _, err := io.ReadFull(server, query); err != nil {
				return
			}
			var request dnsmessage.Message
			if err := request.Unpack(query); err != nil || len(request.Questions) != 1 {
				return
			}
			q := request.Questions[0]
			response := dnsmessage.Message{Header: dnsmessage.Header{ID: request.ID, Response: true, RecursionDesired: true, RecursionAvailable: true}, Questions: request.Questions}
			ips := []string{"8.8.8.8", "2001:4860:4860::8888"}
			switch q.Name.String() {
			case "public.example.test.":
			case "private.example.test.":
				ips = []string{"10.0.0.1", "fd00::1"}
			case "mixed.example.test.":
				ips = append(ips, "10.0.0.1", "fd00::1")
			default:
				response.RCode = dnsmessage.RCodeNameError
				ips = nil
			}
			for _, raw := range ips {
				ip := net.ParseIP(raw)
				var body dnsmessage.ResourceBody
				if v4 := ip.To4(); v4 != nil && q.Type == dnsmessage.TypeA {
					var address [4]byte
					copy(address[:], v4)
					body = &dnsmessage.AResource{A: address}
				} else if ip.To4() == nil && q.Type == dnsmessage.TypeAAAA {
					var address [16]byte
					copy(address[:], ip.To16())
					body = &dnsmessage.AAAAResource{AAAA: address}
				}
				if body != nil {
					response.Answers = append(response.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: q.Type, Class: dnsmessage.ClassINET, TTL: 60}, Body: body})
				}
			}
			packet, err := response.Pack()
			if err != nil {
				return
			}
			binary.BigEndian.PutUint16(size[:], uint16(len(packet)))
			_, _ = server.Write(append(size[:], packet...))
		}()
		return client, nil
	}
}

func TestIsBlockedFetchIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"169.254.1.1", true},
		{"192.168.1.1", true}, // Private IPs are blocked by default for security (SSRF prevention)
		{"10.0.0.1", true},    // Private range 10.x.x.x blocked
		{"172.16.0.1", true},  // Private range 172.16.x.x blocked
		{"8.8.8.8", false},    // Global is allowed
		{"224.0.0.1", true},   // Multicast
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if got := isBlockedFetchIP(ip); got != tt.blocked {
			t.Errorf("isBlockedFetchIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
		}
	}

	if !isBlockedFetchIP(nil) {
		t.Error("nil IP should be blocked")
	}

	// Test that private IPs can be allowed via environment variable
	os.Setenv("PULSE_AI_ALLOW_PRIVATE_IPS", "true")
	defer os.Unsetenv("PULSE_AI_ALLOW_PRIVATE_IPS")

	privateIP := net.ParseIP("192.168.1.1")
	if isBlockedFetchIP(privateIP) {
		t.Error("Private IP should be allowed when PULSE_AI_ALLOW_PRIVATE_IPS=true")
	}

	// Test that loopback IPs can be allowed via environment variable
	os.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")
	defer os.Unsetenv("PULSE_AI_ALLOW_LOOPBACK")

	loopbackIP := net.ParseIP("127.0.0.1")
	if isBlockedFetchIP(loopbackIP) {
		t.Error("Loopback IP should be allowed when PULSE_AI_ALLOW_LOOPBACK=true")
	}
}

// TestIsBlockedFetchIP_IPv6TransitionAddresses covers the SSRF bypass where a
// NAT64, 6to4, Teredo, ISATAP or IPv4-compatible address carries an internal
// IPv4 destination past every net.IP predicate.
func TestIsBlockedFetchIP_IPv6TransitionAddresses(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"64:ff9b::a9fe:a9fe", true},       // NAT64 -> 169.254.169.254
		{"64:ff9b::7f00:1", true},          // NAT64 -> 127.0.0.1
		{"64:ff9b::a00:1", true},           // NAT64 -> 10.0.0.1
		{"64:ff9b:1::c0a8:1", true},        // NAT64 local-use -> 192.168.0.1
		{"64:ff9b:1:a9fe:a9:fe00::", true}, // NAT64 local-use /48 -> 169.254.169.254
		{"2002:ac10:1::", true},            // 6to4 -> 172.16.0.1
		{"2002:a9fe:a9fe::", true},         // 6to4 -> 169.254.169.254
		{"2001:0:a9fe:a9fe::", true},       // Teredo server -> 169.254.169.254
		{"2001:db8::5efe:c0a8:101", true},  // ISATAP -> 192.168.1.1
		{"::192.168.1.1", true},            // IPv4-compatible -> 192.168.1.1
		{"::ffff:0:10.0.0.1", true},        // IPv4-translated -> 10.0.0.1
		{"64:ff9b::808:808", false},        // NAT64 -> 8.8.8.8
		{"2002:808:808::", false},          // 6to4 -> 8.8.8.8
		{"2606:4700:4700::1111", false},    // ordinary global unicast
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("failed to parse %q", tt.ip)
		}
		if got := isBlockedFetchIP(ip); got != tt.blocked {
			t.Errorf("isBlockedFetchIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
		}
	}
}

func TestIsBlockedFetchIP_IPv6TransitionHonoursEscapeHatches(t *testing.T) {
	nat64Private := net.ParseIP("64:ff9b::c0a8:1")  // -> 192.168.0.1
	nat64Loopback := net.ParseIP("64:ff9b::7f00:1") // -> 127.0.0.1

	t.Setenv("PULSE_AI_ALLOW_PRIVATE_IPS", "true")
	if isBlockedFetchIP(nat64Private) {
		t.Error("NAT64-wrapped private IP should be allowed when PULSE_AI_ALLOW_PRIVATE_IPS=true")
	}
	if !isBlockedFetchIP(nat64Loopback) {
		t.Error("NAT64-wrapped loopback should stay blocked when only private IPs are allowed")
	}

	t.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")
	if isBlockedFetchIP(nat64Loopback) {
		t.Error("NAT64-wrapped loopback should be allowed when PULSE_AI_ALLOW_LOOPBACK=true")
	}
}

func TestFetchURL_SizeLimit(t *testing.T) {
	os.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")
	defer os.Unsetenv("PULSE_AI_ALLOW_LOOPBACK")

	// Server that returns 100KB of data
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := make([]byte, 100*1024)
		for i := range data {
			data[i] = 'a'
		}
		w.Write(data)
	}))
	defer ts.Close()

	svc := NewService(nil, nil)
	result, err := svc.fetchURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchURL failed: %v", err)
	}
	if !containsString(result, "truncated at 64KB") {
		t.Error("Expected result to be truncated")
	}
}

func TestFetchURL_RedirectLimit(t *testing.T) {
	os.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")
	defer os.Unsetenv("PULSE_AI_ALLOW_LOOPBACK")

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, ts.URL, http.StatusFound)
	}))
	defer ts.Close()

	svc := NewService(nil, nil)
	_, err := svc.fetchURL(context.Background(), ts.URL)
	if err == nil || !containsString(err.Error(), "too many redirects") {
		t.Errorf("Expected redirect limit error, got: %v", err)
	}
}
