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
	"time"
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

func TestEnsureAuthReauthDoesNotDeadlock(t *testing.T) {
	t.Parallel()

	var authCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			authCalls++
			w.Header().Set("Content-Type", "application/json")
			if authCalls == 1 {
				fmt.Fprint(w, `{"data":{"ticket":"ticket-initial","CSRFPreventionToken":"csrf-initial"}}`)
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"ticket-refresh","CSRFPreventionToken":"csrf-refresh"}}`)
		case "/api2/json/version":
			if !strings.Contains(r.Header.Get("Cookie"), "PMGAuthCookie=ticket-refresh") {
				t.Fatalf("expected refreshed auth cookie, got %s", r.Header.Get("Cookie"))
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"version":"8.1.0"}}`)
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

	client.mu.Lock()
	client.auth.expiresAt = time.Now().Add(-time.Minute)
	client.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, reqErr := client.GetVersion(context.Background())
		done <- reqErr
	}()

	select {
	case reqErr := <-done:
		if reqErr != nil {
			t.Fatalf("GetVersion failed: %v", reqErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetVersion timed out; possible ensureAuth deadlock during re-authentication")
	}

	if authCalls != 2 {
		t.Fatalf("expected re-authentication call, got %d auth calls", authCalls)
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

	if stats == nil || stats.Count.Float64() != 42 {
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
	if backup.Size.Int64() != 123456 {
		t.Fatalf("unexpected size: %d", backup.Size.Int64())
	}
	if backup.Timestamp.Int64() != 1704096000 {
		t.Fatalf("unexpected timestamp: %d", backup.Timestamp.Int64())
	}
}

func TestMailEndpointsHandleNullAndStringValues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"ticket":"ticket","CSRFPreventionToken":"csrf"}}`)
		case "/api2/json/version":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"version":"9.0","release":"1"}}`)
		case "/api2/json/statistics/mail":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{
				"count":null,
				"count_in":"25",
				"count_out":"5",
				"spamcount_in":"7",
				"spamcount_out":null,
				"viruscount_in":"0",
				"viruscount_out":"0",
				"bounces_in":null,
				"bounces_out":"",
				"bytes_in":"1024",
				"bytes_out":"2048",
				"glcount":"2",
				"junk_in":"",
				"rbl_rejects":"1",
				"pregreet_rejects":null,
				"avptime":"0.75"
			}}`)
		case "/api2/json/statistics/mailcount":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{
				"index":"0",
				"time":"1713724800",
				"count":"12",
				"count_in":"8",
				"count_out":"4",
				"spamcount_in":null,
				"spamcount_out":"1",
				"viruscount_in":"",
				"viruscount_out":"0",
				"bounces_in":"1",
				"bounces_out":"0",
				"rbl_rejects":"2",
				"pregreet_rejects":"",
				"glcount":"3"
			}]}`)
		case "/api2/json/quarantine/spamstatus":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"count":null,"avgbytes":"512","mbytes":"0.5"}}`)
		case "/api2/json/quarantine/virusstatus":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"count":"4","avgbytes":"256","mbytes":"0.25"}}`)
		case "/api2/json/nodes/mail/postfix/queue":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"active":"3","deferred":null,"hold":"1","incoming":"2","oldest_age":"600"}}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
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

	ctx := context.Background()

	stats, err := client.GetMailStatistics(ctx, "")
	if err != nil {
		t.Fatalf("GetMailStatistics failed: %v", err)
	}
	if stats == nil {
		t.Fatal("expected statistics data")
	}
	if stats.Count.Float64() != 0 {
		t.Fatalf("expected count to default to 0, got %v", stats.Count.Float64())
	}
	if stats.CountIn.Float64() != 25 {
		t.Fatalf("expected CountIn 25, got %v", stats.CountIn.Float64())
	}
	if stats.BytesIn.Float64() != 1024 {
		t.Fatalf("expected BytesIn 1024, got %v", stats.BytesIn.Float64())
	}
	if stats.AvgProcessSec.Float64() != 0.75 {
		t.Fatalf("expected AvgProcessSec 0.75, got %v", stats.AvgProcessSec.Float64())
	}

	counts, err := client.GetMailCount(ctx, 12)
	if err != nil {
		t.Fatalf("GetMailCount failed: %v", err)
	}
	if len(counts) != 1 {
		t.Fatalf("expected 1 mail count entry, got %d", len(counts))
	}
	entry := counts[0]
	if entry.Time.Int64() != 1713724800 {
		t.Fatalf("expected unix time 1713724800, got %d", entry.Time.Int64())
	}
	if entry.Count.Float64() != 12 {
		t.Fatalf("expected count 12, got %v", entry.Count.Float64())
	}
	if entry.GreylistCount.Float64() != 3 {
		t.Fatalf("expected greylist 3, got %v", entry.GreylistCount.Float64())
	}

	quarantine, err := client.GetQuarantineStatus(ctx, "spam")
	if err != nil {
		t.Fatalf("GetQuarantineStatus failed: %v", err)
	}
	if quarantine.Count.Int64() != 0 {
		t.Fatalf("expected spam quarantine count default to 0, got %d", quarantine.Count.Int64())
	}

	queue, err := client.GetQueueStatus(ctx, "mail")
	if err != nil {
		t.Fatalf("GetQueueStatus failed: %v", err)
	}
	if queue.Active.Int() != 3 || queue.Deferred.Int() != 0 || queue.Hold.Int() != 1 || queue.Incoming.Int() != 2 {
		t.Fatalf("unexpected queue values: %+v", queue)
	}
	if queue.OldestAge.Int64() != 600 {
		t.Fatalf("expected oldest age 600, got %d", queue.OldestAge.Int64())
	}
}

func TestClientTokenNameIncludesUserAndRealm(t *testing.T) {
	t.Parallel()

	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/statistics/mail":
			authHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"count":1}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apiuser@custom!apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	stats, err := client.GetMailStatistics(context.Background(), "")
	if err != nil {
		t.Fatalf("get mail statistics failed: %v", err)
	}
	if stats == nil || stats.Count.Float64() != 1 {
		t.Fatalf("expected statistics count 1, got %+v", stats)
	}

	expected := "PMGAPIToken=apiuser@custom!apitoken:secret"
	if authHeader != expected {
		t.Fatalf("expected authorization header %q, got %q", expected, authHeader)
	}
}

func TestClientRequestAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/statistics/mail":
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "unauthorized")
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	_, err = client.GetMailStatistics(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "authentication error") {
		t.Fatalf("expected authentication error, got %v", err)
	}
}

func TestClientRequestNonAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/statistics/mail":
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "boom")
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	_, err = client.GetMailStatistics(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "API error 500") {
		t.Fatalf("expected API error 500, got %q", msg)
	}
	if strings.Contains(strings.ToLower(msg), "authentication error") {
		t.Fatalf("did not expect authentication error, got %q", msg)
	}
}

func TestClientGetVersionInvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/version":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if _, err := client.GetVersion(context.Background()); err == nil || !strings.Contains(err.Error(), "failed to decode response") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestClientMailCountTimespanParam(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/statistics/mailcount" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("timespan"); got != "3600" {
			t.Fatalf("expected timespan=3600, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if _, err := client.GetMailCount(context.Background(), 3600); err != nil {
		t.Fatalf("GetMailCount failed: %v", err)
	}
}

func TestClientClusterStatusListSingle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/config/cluster/status" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("list_single_node"); got != "1" {
			t.Fatalf("expected list_single_node=1, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if _, err := client.GetClusterStatus(context.Background(), true); err != nil {
		t.Fatalf("GetClusterStatus failed: %v", err)
	}
}

func TestClientListBackupsEscapesNode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/node/1/backup" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if r.URL.EscapedPath() != "/api2/json/nodes/node%2F1/backup" {
			t.Fatalf("expected escaped path, got %s", r.URL.EscapedPath())
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if _, err := client.ListBackups(context.Background(), "node/1"); err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
}

func TestClientGetSpamScores(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/statistics/spamscores" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"level":"high","count":"2","ratio":"0.5"}]}`)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	scores, err := client.GetSpamScores(context.Background())
	if err != nil {
		t.Fatalf("GetSpamScores failed: %v", err)
	}
	if len(scores) != 1 || scores[0].Level != "high" || scores[0].Count.Int() != 2 {
		t.Fatalf("unexpected spam scores: %+v", scores)
	}
}

func TestClientMailCountNoTimespanParam(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/statistics/mailcount" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("timespan"); got != "" {
			t.Fatalf("expected no timespan param, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if _, err := client.GetMailCount(context.Background(), 0); err != nil {
		t.Fatalf("GetMailCount failed: %v", err)
	}
}

func TestClientClusterStatusNoParam(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/config/cluster/status" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if len(r.URL.Query()) != 0 {
			t.Fatalf("expected no query params, got %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if _, err := client.GetClusterStatus(context.Background(), false); err != nil {
		t.Fatalf("GetClusterStatus failed: %v", err)
	}
}
