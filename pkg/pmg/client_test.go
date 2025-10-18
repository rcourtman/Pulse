package pmg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientAuthenticatesWithJSONPayload(t *testing.T) {
	t.Parallel()

	var authCalls int
	var versionCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			authCalls++

			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for auth, got %s", r.Method)
			}

			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("expected JSON auth content-type, got %s", ct)
			}

			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed decoding auth payload: %v", err)
			}

			if payload["username"] != "api@pmg" {
				t.Fatalf("expected username api@pmg, got %s", payload["username"])
			}

			if payload["password"] != "secret" {
				t.Fatalf("expected password secret, got %s", payload["password"])
			}

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"ticket":"ticket123","CSRFPreventionToken":"csrf123"}}`)

		case "/api2/json/version":
			versionCalls++

			cookie := r.Header.Get("Cookie")
			if !strings.Contains(cookie, "PMGAuthCookie=ticket123") {
				t.Fatalf("expected auth cookie, got %s", cookie)
			}

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"version":"8.0.0","release":"1"}}`)

		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:      server.URL,
		User:      "api@pmg",
		Password:  "secret",
		VerifySSL: false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	info, err := client.GetVersion(context.Background())
	if err != nil {
		t.Fatalf("get version failed: %v", err)
	}

	if info == nil || info.Version != "8.0.0" {
		t.Fatalf("expected version 8.0.0, got %+v", info)
	}

	if authCalls != 1 {
		t.Fatalf("expected one auth call, got %d", authCalls)
	}

	if versionCalls != 1 {
		t.Fatalf("expected one version call, got %d", versionCalls)
	}
}

func TestClientAuthenticateFallsBackToForm(t *testing.T) {
	t.Parallel()

	var authCalls int
	var jsonAuthReceived bool
	var formAuthReceived bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			authCalls++

			switch authCalls {
			case 1:
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Fatalf("first auth call should use JSON content-type, got %s", ct)
				}
				jsonAuthReceived = true

				w.WriteHeader(http.StatusUnsupportedMediaType)
				fmt.Fprint(w, "use form encoding")

			case 2:
				if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
					t.Fatalf("fallback auth should use form encoding, got %s", ct)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed reading form body: %v", err)
				}

				formValues := string(body)
				if !strings.Contains(formValues, "username=api%40pmg") {
					t.Fatalf("expected username in form body, got %s", formValues)
				}

				formAuthReceived = true

				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"data":{"ticket":"ticket456","CSRFPreventionToken":"csrf456"}}`)

			default:
				t.Fatalf("unexpected auth attempt %d", authCalls)
			}

		case "/api2/json/version":
			if !strings.Contains(r.Header.Get("Cookie"), "PMGAuthCookie=ticket456") {
				t.Fatalf("expected fallback auth cookie, got %s", r.Header.Get("Cookie"))
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"version":"9.0.0"}}`)

		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:      server.URL,
		User:      "api@pmg",
		Password:  "secret",
		VerifySSL: false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client with fallback: %v", err)
	}

	if _, err := client.GetVersion(context.Background()); err != nil {
		t.Fatalf("get version after fallback failed: %v", err)
	}

	if authCalls != 2 {
		t.Fatalf("expected two auth attempts, got %d", authCalls)
	}

	if !jsonAuthReceived {
		t.Fatal("expected JSON auth request to be received")
	}

	if !formAuthReceived {
		t.Fatal("expected form-based auth fallback to be received")
	}
}

func TestClientUsesTokenAuthorizationHeader(t *testing.T) {
	t.Parallel()

	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/statistics/mail":
			if r.Header.Get("Content-Type") != "" {
				t.Fatalf("expected no explicit content-type on GET, got %s", r.Header.Get("Content-Type"))
			}

			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method %s", r.Method)
			}

			authHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"count":42}}`)

		case "/api2/json/access/ticket":
			t.Fatalf("token-based client should not request tickets")

		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		User:       "apitest@pmg",
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating token client: %v", err)
	}

	stats, err := client.GetMailStatistics(context.Background(), "")
	if err != nil {
		t.Fatalf("get mail statistics failed: %v", err)
	}

	if stats == nil || stats.Count != 42 {
		t.Fatalf("expected statistics count 42, got %+v", stats)
	}

	expected := "PMGAPIToken=apitest@pmg!apitoken:secret"
	if authHeader != expected {
		t.Fatalf("expected authorization header %q, got %q", expected, authHeader)
	}
}

func TestListBackups(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for auth, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"ticket":"ticket789","CSRFPreventionToken":"csrf789"}}`)
		case "/api2/json/nodes/node1/backup":
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET for backup listing, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"filename":"pmg-backup_2024-01-01.tgz","size":123456,"timestamp":1704096000}]}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:      server.URL,
		User:      "api@pmg",
		Password:  "secret",
		VerifySSL: false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	backups, err := client.ListBackups(context.Background(), "node1")
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	backup := backups[0]
	if backup.Filename != "pmg-backup_2024-01-01.tgz" {
		t.Fatalf("unexpected filename: %s", backup.Filename)
	}
	if backup.Size != 123456 {
		t.Fatalf("unexpected size: %d", backup.Size)
	}
	if backup.Timestamp != 1704096000 {
		t.Fatalf("unexpected timestamp: %d", backup.Timestamp)
	}
}
