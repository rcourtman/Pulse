package ai

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchURL(t *testing.T) {
	os.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")
	defer os.Unsetenv("PULSE_AI_ALLOW_LOOPBACK")

	// Start a local test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, world")
	}))
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

	tests := []struct {
		url     string
		wantErr bool
		errSub  string
	}{
		{"http://example.com", false, ""},
		{"https://example.com/path", false, ""},
		{"  http://example.com  ", false, ""},
		{"", true, "url is required"},
		{"http://localhost", true, "blocked"},
		{"http://127.0.0.1", true, "blocked"},
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

func TestIsBlockedFetchIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"169.254.1.1", true},
		{"192.168.1.1", false}, // Private is allowed
		{"8.8.8.8", false},     // Global is allowed
		{"224.0.0.1", true},    // Multicast
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
