package proxmox

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClient_GetAllStorageSetsTimeoutWhenNoDeadline(t *testing.T) {
	client, err := NewClient(ClientConfig{
		Host:       "http://example.invalid",
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	var sawDeadline bool
	client.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api2/json/storage" {
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found"))}, nil
			}
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Error("expected request deadline to be set")
			} else if time.Until(deadline) < 20*time.Second {
				t.Errorf("expected ~30s deadline, got %v", time.Until(deadline))
			} else {
				sawDeadline = true
			}
			body := `{"data":[{"storage":"local"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	storage, err := client.GetAllStorage(context.Background())
	if err != nil {
		t.Fatalf("GetAllStorage failed: %v", err)
	}
	if len(storage) != 1 {
		t.Fatalf("expected 1 storage entry, got %d", len(storage))
	}
	if !sawDeadline {
		t.Fatal("expected to observe deadline in request context")
	}
}

func TestClient_GetAllStorageRespectsExistingDeadline(t *testing.T) {
	client, err := NewClient(ClientConfig{
		Host:       "http://example.invalid",
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	client.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api2/json/storage" {
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found"))}, nil
			}
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Error("expected request deadline to be set")
			} else if time.Until(deadline) > 2*time.Second {
				t.Errorf("expected short deadline to be preserved, got %v", time.Until(deadline))
			}
			body := `{"data":[{"storage":"local"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	storage, err := client.GetAllStorage(ctx)
	if err != nil {
		t.Fatalf("GetAllStorage failed: %v", err)
	}
	if len(storage) != 1 {
		t.Fatalf("expected 1 storage entry, got %d", len(storage))
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
