package pbs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Helper to create a client connected to a test server
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
		Timeout:    1 * time.Second,
	})
	if err != nil {
		server.Close()
		t.Fatalf("Failed to create client: %v", err)
	}
	return client, server
}

func TestClient_CreateUser(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		comment     string
		handler     http.HandlerFunc
		expectError bool
	}{
		{
			name:    "success",
			userID:  "user1@pbs",
			comment: "test user",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api2/json/access/users" {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				if r.FormValue("userid") != "user1@pbs" || r.FormValue("comment") != "test user" {
					http.Error(w, "bad params", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:   "already exists",
			userID: "user1@pbs",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "user already exists", http.StatusInternalServerError) // PBS returns 500 or 400 with message
			},
			expectError: false, // Should be ignored
		},
		{
			name:   "other error",
			userID: "user1@pbs",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "something went wrong", http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			err := client.CreateUser(context.Background(), tc.userID, tc.comment)
			if tc.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestClient_SetUserACL(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api2/json/access/acl" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if r.FormValue("auth-id") != "user1@pbs" || r.FormValue("path") != "/" || r.FormValue("role") != "Audit" {
			http.Error(w, "bad params", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	err := client.SetUserACL(context.Background(), "user1@pbs", "/", "Audit")
	if err != nil {
		t.Errorf("SetUserACL failed: %v", err)
	}
}

func TestClient_SetUserACL_Error(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	})
	defer server.Close()

	err := client.SetUserACL(context.Background(), "user1@pbs", "/", "Audit")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestClient_CreateUserToken(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		expectError bool
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api2/json/access/users/user1@pbs/token/token1" {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]string{
						"tokenid": "user1@pbs!token1",
						"value":   "secret123",
					},
				})
			},
			expectError: false,
		},
		{
			name: "error response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "fail", http.StatusInternalServerError)
			},
			expectError: true,
		},
		{
			name: "bad json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("invalid json"))
			},
			expectError: true,
		},
		{
			name: "empty value",
			handler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]string{
						"tokenid": "user1@pbs!token1",
						"value":   "",
					},
				})
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			resp, err := client.CreateUserToken(context.Background(), "user1@pbs", "token1")
			if tc.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if resp != nil && resp.Value != "secret123" {
					t.Errorf("Expected token value 'secret123', got %q", resp.Value)
				}
			}
		})
	}
}

func TestClient_SetupMonitoringAccess(t *testing.T) {
	// This tests the flow: CreateUser -> SetUserACL -> CreateUserToken -> SetUserACL (token)
	steps := make(map[string]bool)

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/users":
			steps["create_user"] = true
			if r.FormValue("userid") != "pulse-monitor@pbs" {
				http.Error(w, "bad user", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/api2/json/access/acl":
			if r.FormValue("auth-id") == "pulse-monitor@pbs" {
				steps["set_acl_user"] = true
			} else if r.FormValue("auth-id") == "pulse-monitor@pbs!monitor1" {
				steps["set_acl_token"] = true
			}
			w.WriteHeader(http.StatusOK)
		case "/api2/json/access/users/pulse-monitor@pbs/token/monitor1":
			steps["create_token"] = true
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{
					"tokenid": "pulse-monitor@pbs!monitor1",
					"value":   "newsecret",
				},
			})
		default:
			http.Error(w, "unknown path "+r.URL.Path, http.StatusNotFound)
		}
	})
	defer server.Close()

	id, val, err := client.SetupMonitoringAccess(context.Background(), "monitor1")
	if err != nil {
		t.Fatalf("SetupMonitoringAccess failed: %v", err)
	}

	if id != "pulse-monitor@pbs!monitor1" {
		t.Errorf("Expected token ID 'pulse-monitor@pbs!monitor1', got %q", id)
	}
	if val != "newsecret" {
		t.Errorf("Expected token value 'newsecret', got %q", val)
	}

	if !steps["create_user"] {
		t.Error("CreateUser step not executed")
	}
	if !steps["set_acl_user"] {
		t.Error("SetUserACL (user) step not executed")
	}
	if !steps["create_token"] {
		t.Error("CreateUserToken step not executed")
	}
	// The last step is optional/warns on failure, but here it succeeds
	if !steps["set_acl_token"] {
		t.Error("SetUserACL (token) step not executed")
	}
}

func TestClient_SetupMonitoringAccess_Errors(t *testing.T) {
	// 1. Fail at SetUserACL (user)
	client1, server1 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/acl") && r.FormValue("auth-id") == "pulse-monitor@pbs" {
			http.Error(w, "acl error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server1.Close()
	_, _, err1 := client1.SetupMonitoringAccess(context.Background(), "t1")
	if err1 == nil || !strings.Contains(err1.Error(), "set user ACL") {
		t.Errorf("Expected set user ACL error, got: %v", err1)
	}

	// 2. Fail at CreateToken
	client2, server2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/token/") {
			http.Error(w, "token error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server2.Close()
	_, _, err2 := client2.SetupMonitoringAccess(context.Background(), "t2")
	if err2 == nil || !strings.Contains(err2.Error(), "create token") {
		t.Errorf("Expected create token error, got: %v", err2)
	}
}

func TestClient_GetVersion_Error(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	})
	defer server.Close()

	_, err := client.GetVersion(context.Background())
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestClient_GetVersion_BadJSON(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	})
	defer server.Close()

	_, err := client.GetVersion(context.Background())
	if err == nil {
		t.Error("Expected error on bad JSON, got nil")
	}
}

func TestClient_GetNodeStatus_Error(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	})
	defer server.Close()

	_, err := client.GetNodeStatus(context.Background())
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestClient_GetNodeStatus_JsonError(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	})
	defer server.Close()

	_, err := client.GetNodeStatus(context.Background())
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestClient_GetNodeName(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes" {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"node": "pve1"},
			},
		})
	})
	defer server.Close()

	name, err := client.GetNodeName(context.Background())
	if err != nil {
		t.Fatalf("GetNodeName failed: %v", err)
	}
	if name != "pve1" {
		t.Errorf("Expected 'pve1', got %q", name)
	}
}

func TestClient_GetNodeName_Empty(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{},
		})
	})
	defer server.Close()

	_, err := client.GetNodeName(context.Background())
	if err == nil {
		t.Error("Expected error for empty node list, got nil")
	}
}

func TestClient_GetNodeStatus_Success(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/localhost/status" {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": NodeStatus{
				CPU: 0.1,
				Memory: Memory{
					Total: 1000,
					Used:  500,
				},
			},
		})
	})
	defer server.Close()

	status, err := client.GetNodeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}
	if status.CPU != 0.1 {
		t.Errorf("Expected CPU 0.1, got %f", status.CPU)
	}
}

func TestClient_GetDatastores_Full(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/admin/datastore" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{
					{"store": "store1"},
					{"store": "store2"},
				},
			})
			return
		}

		if r.URL.Path == "/api2/json/admin/datastore/store1/rrd" {
			// Returns RRD data for dedup factor
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"time": 1, "dedup_factor": 2.5},
				},
			})
			return
		}
		if r.URL.Path == "/api2/json/admin/datastore/store1/status" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"total": 1000.0,
					"used":  200.0,
					"avail": 800.0,
				},
			})
			return
		}

		if r.URL.Path == "/api2/json/admin/datastore/store2/rrd" {
			http.Error(w, "no rrd", http.StatusNotFound)
			return
		}
		if r.URL.Path == "/api2/json/admin/datastore/store2/status" {
			// Missing dedup in status
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"total": 2000.0,
				},
			})
			return
		}
		if r.URL.Path == "/api2/json/admin/datastore/store2/gc" {
			// Fallback to GC for dedup
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"index-data-bytes": 300.0,
					"disk-bytes":       100.0,
				},
			})
			return
		}

		http.NotFound(w, r)
	})
	defer server.Close()

	dss, err := client.GetDatastores(context.Background())
	if err != nil {
		t.Fatalf("GetDatastores failed: %v", err)
	}

	if len(dss) != 2 {
		t.Fatalf("Expected 2 datastores, got %d", len(dss))
	}

	// Check store1 (RRD dedup)
	if dss[0].Store != "store1" {
		t.Errorf("Expected store1, got %s", dss[0].Store)
	}
	if dss[0].DeduplicationFactor != 2.5 {
		t.Errorf("Expected store1 dedup 2.5, got %f", dss[0].DeduplicationFactor)
	}

	// Check store2 (GC dedup: 300/100 = 3.0)
	if dss[1].Store != "store2" {
		t.Errorf("Expected store2, got %s", dss[1].Store)
	}
	if dss[1].DeduplicationFactor != 3.0 {
		t.Errorf("Expected store2 dedup 3.0, got %f", dss[1].DeduplicationFactor)
	}
}

func TestClient_ListNamespaces(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/admin/datastore/store1/namespace" {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		if r.FormValue("ns") == "parent" && r.FormValue("max-depth") == "2" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []Namespace{
					{Name: "child"},
				},
			})
			return
		}
		// Default
		json.NewEncoder(w).Encode(map[string]any{
			"data": []Namespace{
				{Name: "ns1"},
			},
		})
	})
	defer server.Close()

	// Test with params
	nss, err := client.ListNamespaces(context.Background(), "store1", "parent", 2)
	if err != nil {
		t.Fatalf("ListNamespaces failed: %v", err)
	}
	if len(nss) != 1 || nss[0].Name != "child" {
		t.Errorf("Unexpected namespaces: %v", nss)
	}

	// Test 404
	client2, server2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound) // 404
	})
	defer server2.Close()

	nss2, err := client2.ListNamespaces(context.Background(), "store1", "", 0)
	if err != nil {
		t.Fatalf("Expected no error on 404, got %v", err)
	}
	if len(nss2) != 0 {
		t.Errorf("Expected empty list on 404, got %d", len(nss2))
	}
}

func TestClient_ListBackupGroups(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/admin/datastore/store1/groups" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []BackupGroup{
				{BackupID: "vm/100"},
			},
		})
	})
	defer server.Close()

	groups, err := client.ListBackupGroups(context.Background(), "store1", "ns1")
	if err != nil {
		t.Fatalf("ListBackupGroups failed: %v", err)
	}
	if len(groups) != 1 || groups[0].BackupID != "vm/100" {
		t.Errorf("Unexpected groups: %v", groups)
	}
}

func TestClient_ListBackupSnapshots(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/admin/datastore/store1/snapshots" {
			http.NotFound(w, r)
			return
		}
		if r.FormValue("backup-type") != "vm" || r.FormValue("backup-id") != "100" {
			http.Error(w, "bad params", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []BackupSnapshot{
				{BackupTime: 12345},
			},
		})
	})
	defer server.Close()

	snaps, err := client.ListBackupSnapshots(context.Background(), "store1", "ns1", "vm", "100")
	if err != nil {
		t.Fatalf("ListBackupSnapshots failed: %v", err)
	}
	if len(snaps) != 1 || snaps[0].BackupTime != 12345 {
		t.Errorf("Unexpected snapshots: %v", snaps)
	}
}

func TestClient_ListAllBackups(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Mock groups list for ns1 and ns2
		if strings.HasSuffix(r.URL.Path, "/groups") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []BackupGroup{
					{BackupType: "vm", BackupID: "100"},
				},
			})
			return
		}
		// Mock snapshots
		if strings.HasSuffix(r.URL.Path, "/snapshots") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []BackupSnapshot{
					{BackupTime: 12345},
				},
			})
			return
		}
		http.NotFound(w, r)
	})
	defer server.Close()

	results, err := client.ListAllBackups(context.Background(), "store1", []string{"ns1", "ns2"})
	if err != nil {
		t.Fatalf("ListAllBackups failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 namespaces, got %d", len(results))
	}
	if len(results["ns1"]) != 1 {
		t.Errorf("Expected 1 snapshot in ns1, got %d", len(results["ns1"]))
	}
}

func TestClient_ListAllBackups_Error(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	})
	defer server.Close()

	_, err := client.ListAllBackups(context.Background(), "store1", []string{"ns1"})
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestNewClient_TokenParsing(t *testing.T) {
	// Test user@realm!tokenname format
	cfg1 := ClientConfig{
		Host:       "http://example.com",
		TokenName:  "user@realm!token1",
		TokenValue: "secret",
	}
	client1, err := NewClient(cfg1)
	if err != nil {
		t.Fatalf("Failed to create client with complex token name: %v", err)
	}
	if client1.auth.user != "user" || client1.auth.realm != "realm" || client1.auth.tokenName != "token1" {
		t.Errorf("Incorrect parsing: user=%q realm=%q token=%q", client1.auth.user, client1.auth.realm, client1.auth.tokenName)
	}

	// Test user provided separately
	cfg2 := ClientConfig{
		Host:       "http://example.com",
		User:       "u2@r2",
		TokenName:  "token2",
		TokenValue: "secret",
	}
	client2, err := NewClient(cfg2)
	if err != nil {
		t.Fatalf("Failed to create client with user+token: %v", err)
	}
	if client2.auth.user != "u2" || client2.auth.realm != "r2" || client2.auth.tokenName != "token2" {
		t.Errorf("Incorrect parsing: user=%q realm=%q token=%q", client2.auth.user, client2.auth.realm, client2.auth.tokenName)
	}

	// Test user no realm (default pbs)
	cfg3 := ClientConfig{
		Host:       "http://example.com",
		User:       "u3",
		TokenName:  "token3",
		TokenValue: "secret",
	}
	client3, err := NewClient(cfg3)
	if err != nil {
		t.Fatalf("Failed to create client with user no realm: %v", err)
	}
	if client3.auth.user != "u3" || client3.auth.realm != "pbs" {
		t.Errorf("Incorrect parsing: user=%q realm=%q", client3.auth.user, client3.auth.realm)
	}
}

func TestNewClient_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "auth failed", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := ClientConfig{
		Host:     server.URL,
		User:     "u@r",
		Password: "p",
		Timeout:  1 * time.Second,
	}
	_, err := NewClient(cfg)
	if err == nil {
		t.Error("Expected auth error, got nil")
	}
}

func TestClient_GetDatastores_PartialErrors(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/admin/datastore" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{
					{"store": "store1"}, // Bad status JSON
					{"store": "store2"}, // Status HTTP error
				},
			})
			return
		}

		if strings.Contains(r.URL.Path, "rrd") {
			http.Error(w, "no rrd", http.StatusNotFound)
			return
		}

		if r.URL.Path == "/api2/json/admin/datastore/store1/status" {
			w.Write([]byte("invalid json"))
			return
		}

		if r.URL.Path == "/api2/json/admin/datastore/store2/status" {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		http.NotFound(w, r)
	})
	defer server.Close()

	dss, err := client.GetDatastores(context.Background())
	if err != nil {
		t.Fatalf("GetDatastores failed: %v", err)
	}

	if len(dss) != 2 {
		t.Fatalf("Expected 2 datastores (even with errors), got %d", len(dss))
	}
	if !strings.Contains(dss[0].Error, "parse status") {
		t.Errorf("Expected parse error for store1, got field: %v", dss[0].Error)
	}
	if !strings.Contains(dss[1].Error, "get status") {
		t.Errorf("Expected get status error for store2, got field: %v", dss[1].Error)
	}
}

func TestClient_Request_Reauthentication(t *testing.T) {
	var authCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			authCalls++
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{
					"ticket":              "newticket",
					"CSRFPreventionToken": "newcsrf",
				},
			})
			return
		}
		if r.URL.Path == "/api2/json/test" {
			if r.Header.Get("Cookie") != "PBSAuthCookie=newticket" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:     server.URL,
		User:     "u@r",
		Password: "p", // Password auth enables re-auth
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Manually set expired auth
	client.auth.ticket = "oldticket"
	client.auth.expiresAt = time.Now().Add(-1 * time.Hour)

	// Make request
	_, err = client.request(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if authCalls < 1 {
		t.Error("Expected re-authentication to occur")
	}
	if client.auth.ticket != "newticket" {
		t.Error("Expected ticket to be updated")
	}
}

func TestClient_Request_Reauthentication_Failure(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			calls++
			if calls == 1 {
				// Initial auth success
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]string{
						"ticket":              "ticket1",
						"CSRFPreventionToken": "csrf1",
					},
				})
				return
			}
			// Re-auth fail
			http.Error(w, "auth fail", http.StatusUnauthorized)
			return
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:     server.URL,
		User:     "u@r",
		Password: "p",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Force expiration
	client.auth.expiresAt = time.Now().Add(-1 * time.Hour)

	_, err = client.request(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Error("Expected error on re-auth failure, got nil")
	}
}

func TestClient_Authenticate_AuthResponseError(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// Success for NewClient
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{
					"ticket":              "t",
					"CSRFPreventionToken": "c",
				},
			})
			return
		}
		// Failure for manual call
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:     server.URL,
		User:     "u@r",
		Password: "p",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// manually call authenticateJSON which writes to client
	err = client.authenticateJSON(context.Background(), "u", "p")
	if err == nil {
		t.Error("Expected JSON error on bad auth response, got nil")
	}
}

func TestClient_SetupMonitoringAccess_ErrorLastStep(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "token/") {
			// Token creation success
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{
					"tokenid": "t1",
					"value":   "secret",
				},
			})
			return
		}
		if r.URL.Path == "/api2/json/access/acl" && r.FormValue("auth-id") != "pulse-monitor@pbs" {
			// Second ACL call (for token) -> fail
			http.Error(w, "acl error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	// Should succeed but log warning
	_, _, err := client.SetupMonitoringAccess(context.Background(), "t1")
	if err != nil {
		t.Errorf("Expected success (with warning), got error: %v", err)
	}
}

// Custom transport to simulate network errors
type errorTransport struct{}

func (t *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("network error")
}

func TestClient_Authenticate_NetworkError(t *testing.T) {
	// Use token auth to avoid NewClient trying to authenticate immediately
	client, err := NewClient(ClientConfig{
		Host:       "http://example.com",
		TokenName:  "u@r!t",
		TokenValue: "v",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Inject error client
	client.httpClient = &http.Client{Transport: &errorTransport{}}

	// Manually call authenticateJSON
	err = client.authenticateJSON(context.Background(), "u", "p")
	if err == nil {
		t.Error("Expected network error, got nil")
	}
}

func TestClient_AuthenticateForm_NetworkError(t *testing.T) {
	// Use token auth to avoid NewClient trying to authenticate immediately
	client, err := NewClient(ClientConfig{
		Host:       "http://example.com",
		TokenName:  "u@r!t",
		TokenValue: "v",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Inject error client
	client.httpClient = &http.Client{Transport: &errorTransport{}}

	err = client.authenticateForm(context.Background(), "u", "p")
	if err == nil {
		t.Error("Expected network error, got nil")
	}
}

func TestClient_GetDatastores_HTMLResponseOnHTTPS(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>Error</html>"))
	})
	defer server.Close()

	// Ensure config is HTTPS (newTestClient uses httptest URL which is http, but we can override host string for this check logic)
	// Actually NewClient sets baseURL from config.Host.
	// The html check looks at c.config.Host.
	client.config.Host = "https://pbs.example.com"

	_, err := client.GetDatastores(context.Background())
	if err == nil {
		t.Error("Expected error on HTML response, got nil")
	}
	if !strings.Contains(err.Error(), "PBS returned HTML instead of JSON") {
		t.Errorf("Unexpected error message: %v", err)
	}
	if strings.Contains(err.Error(), "Try changing your URL") {
		t.Error("Should not suggest changing URL when already HTTPS")
	}
}

func TestClient_Request_NewRequestError(t *testing.T) {
	// Use token auth to skip initial auth
	client, err := NewClient(ClientConfig{
		Host:       "http://example.com",
		TokenName:  "u@r!t",
		TokenValue: "v",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.request(context.Background(), "INVALID\nMETHOD", "/", nil)
	if err == nil {
		t.Error("Expected error on invalid method, got nil")
	}
}

func TestClient_GetDatastores_AdvancedDedup(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "rrd") {
			if strings.Contains(r.URL.Path, "store1") {
				// Bad JSON for RRD
				w.Write([]byte("invalid json"))
				return
			}
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "gc") {
			if strings.Contains(r.URL.Path, "store2") {
				// Good GC data
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"index-data-bytes": 300.0,
						"disk-bytes":       100.0,
					},
				})
				return
			}
			if strings.Contains(r.URL.Path, "store3") {
				// Bad GC JSON
				w.Write([]byte("invalid json"))
				return
			}
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "status") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"total": 100.0,
				},
			})
			return
		}
		// List
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"store": "store1"}, // Bad RRD
				{"store": "store2"}, // Good GC
				{"store": "store3"}, // Bad GC
			},
		})
	})
	defer server.Close()

	dss, err := client.GetDatastores(context.Background())
	if err != nil {
		t.Fatalf("GetDatastores failed: %v", err)
	}
	if len(dss) != 3 {
		t.Errorf("Expected 3 datastores, got %d", len(dss))
	}
	// Store 2 should have dedup 3.0
	if dss[1].Store == "store2" && dss[1].DeduplicationFactor != 3.0 {
		t.Errorf("Expected store2 dedup 3.0, got %f", dss[1].DeduplicationFactor)
	}
}

func TestListAllBackups_PartialFailure(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ns") == "ns1" {
			http.Error(w, "ns1 fail", http.StatusInternalServerError)
			return
		}
		// ns2 success
		if strings.HasSuffix(r.URL.Path, "/groups") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []BackupGroup{},
			})
			return
		}
	})
	defer server.Close()

	_, err := client.ListAllBackups(context.Background(), "store1", []string{"ns1", "ns2"})
	if err == nil {
		t.Error("Expected error on partial failure, got nil")
	}
}

func TestClient_ListMethods_NetworkError(t *testing.T) {
	// Use token auth to allow creation
	client, err := NewClient(ClientConfig{Host: "http://example.com", TokenName: "u@r!t", TokenValue: "v"})
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient = &http.Client{Transport: &errorTransport{}}

	if _, err := client.ListNamespaces(context.Background(), "s", "", 0); err == nil {
		t.Error("ListNamespaces: expected network error")
	}
	if _, err := client.ListBackupGroups(context.Background(), "s", ""); err == nil {
		t.Error("ListBackupGroups: expected network error")
	}
	if _, err := client.ListBackupSnapshots(context.Background(), "s", "", "vm", "1"); err == nil {
		t.Error("ListBackupSnapshots: expected network error")
	}
	if _, err := client.GetNodeName(context.Background()); err == nil {
		t.Error("GetNodeName: expected network error")
	}
}

func TestNewClient_MalformedToken(t *testing.T) {
	// Malformed token with "!" but not valid parts
	_, err := NewClient(ClientConfig{Host: "http://e.com", TokenName: "invalid!token!", TokenValue: "v"})
	if err == nil {
		t.Error("Expected error on malformed token name")
	}
}

func TestClient_GetDatastores_BadJSONList(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	})
	defer server.Close()

	_, err := client.GetDatastores(context.Background())
	if err == nil {
		t.Error("Expected error on bad datastore list JSON, got nil")
	}
}

func TestClient_SetupMonitoringAccess_UserExists(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/users") && r.Method == "POST" {
			// User exists
			http.Error(w, "user already exists", http.StatusBadRequest)
			return
		}
		// Steps continue... set acl, create token...
		if strings.Contains(r.URL.Path, "token") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{"value": "secret"},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	_, _, err := client.SetupMonitoringAccess(context.Background(), "t1")
	if err != nil {
		t.Errorf("Expected success when user exists, got error: %v", err)
	}
}

func TestClient_Authenticate_InvalidURL(t *testing.T) {
	// Bypass NewClient validation/auth by using token and manual struct
	// Control character in host
	client := &Client{
		baseURL:    "http://ex\nample.com",
		httpClient: &http.Client{},
	}

	err := client.authenticateJSON(context.Background(), "u", "p")
	if err == nil {
		t.Error("Expected error on invalid URL, got nil")
	}

	err = client.authenticateForm(context.Background(), "u", "p")
	if err == nil {
		t.Error("Expected error on invalid URL, got nil")
	}
}

// Mock failing reader
type failReader struct{}

func (f *failReader) Read(p []byte) (n int, err error) { return 0, fmt.Errorf("read error") }
func (f *failReader) Close() error                     { return nil }

type bodyErrorTransport struct{}

func (t *bodyErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &failReader{},
		Header:     make(http.Header),
	}, nil
}

type bodyErrorTransportBadStatus struct{}

func (t *bodyErrorTransportBadStatus) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusBadRequest, // trigger error paths reading body
		Body:       &failReader{},
		Header:     make(http.Header),
	}, nil
}

func TestClient_ReadBodyErrors(t *testing.T) {
	client, err := NewClient(ClientConfig{Host: "http://e.com", TokenName: "u@r!t", TokenValue: "v"})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test methods reading body on success (JSON decode usually fails on read error too)
	client.httpClient = &http.Client{Transport: &bodyErrorTransport{}}

	if _, err := client.GetDatastores(context.Background()); err == nil {
		t.Error("GetDatastores: expected read error")
	}
	// Note: GetNodeStatus, List* use json.Decode. json.Decode fails if Read fails.
	// But GetDatastores calls io.ReadAll explicitly.

	// 2. Test methods reading body on error status
	client.httpClient = &http.Client{Transport: &bodyErrorTransportBadStatus{}}

	// CreateUserToken reads body on error
	if _, err := client.CreateUserToken(context.Background(), "u", "t"); err == nil {
		t.Error("CreateUserToken: expected read error handling")
	}

	// authenticateJSON reads body on error
	if err := client.authenticateJSON(context.Background(), "u", "p"); err == nil {
		t.Error("authenticateJSON: expected error")
	}

	if _, err := client.ListBackupGroups(context.Background(), "s", "n"); err == nil {
		t.Error("ListBackupGroups: expected error")
	}
}

type sequentialFailTransport struct {
	mu           sync.Mutex
	requestCount int
	failOn       int // 0-based index of request to fail reading
}

func (t *sequentialFailTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	count := t.requestCount
	t.requestCount++
	t.mu.Unlock()

	// If this is the list request (first), succeed with valid JSON
	if strings.HasSuffix(req.URL.Path, "datastore") && !strings.Contains(req.URL.Path, "status") && !strings.Contains(req.URL.Path, "rrd") && !strings.Contains(req.URL.Path, "gc") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"data": [{"store": "store1"}]}`)),
			Header:     make(http.Header),
		}, nil
	}

	// For inner requests (rrd, status, gc)
	// Check if we should fail this one
	if count == t.failOn {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       &failReader{},
			Header:     make(http.Header),
		}, nil
	}

	// Otherwise succeed (empty JSON valid enough for decode to not error drastically or just fail decode gracefully)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"data": {}}`)),
		Header:     make(http.Header),
	}, nil
}

func TestClient_GetDatastores_InnerReadErrors(t *testing.T) {
	client, err := NewClient(ClientConfig{Host: "http://example.com", TokenName: "u@r!t", TokenValue: "v"})
	if err != nil {
		t.Fatal(err)
	}

	// Fail RRD read (request index 1: list is 0, rrd is 1)
	client.httpClient = &http.Client{Transport: &sequentialFailTransport{failOn: 1}}
	dss, err := client.GetDatastores(context.Background())
	if err != nil {
		t.Fatalf("GetDatastores failed on RRD error: %v", err)
	}
	if len(dss) != 1 {
		t.Error("Expected 1 datastore even with RRD error")
	}

	// Fail Status read (request index 2: list, rrd, status)
	// Note: Status read error logs error and appends datastore with Error field.
	// So function returns success, but datastore list has item with Error.
	client.httpClient = &http.Client{Transport: &sequentialFailTransport{failOn: 2}}
	client.httpClient.Transport.(*sequentialFailTransport).requestCount = 0 // Reset
	dss, err = client.GetDatastores(context.Background())
	if err != nil {
		t.Fatalf("GetDatastores failed on Status read error (unexpected): %v", err)
	}
	if len(dss) != 1 {
		t.Errorf("Expected 1 datastore (with error), got %d", len(dss))
	}
	if dss[0].Error == "" {
		t.Error("Expected Error field to be set on Status read failure")
	}

	// Fail GC read (request index 3: list, rrd, status, gc)
	// Code: if err := json.Decode(...); err != nil { log.Warn; continue }
	// So this should SUCCEED.
	client.httpClient = &http.Client{Transport: &sequentialFailTransport{failOn: 3}}
	client.httpClient.Transport.(*sequentialFailTransport).requestCount = 0 // Reset
	dss, err = client.GetDatastores(context.Background())
	if err != nil {
		t.Fatalf("GetDatastores failed on GC error: %v", err)
	}
	if len(dss) != 1 {
		t.Error("Expected 1 datastore even with GC error")
	}
}

func TestClient_GetDatastores_DedupFromStatus(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "rrd") {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "gc") {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "status") {
			if strings.Contains(r.URL.Path, "store1") {
				// Hyphenated key
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"deduplication-factor": 5.5,
					},
				})
				return
			}
			if strings.Contains(r.URL.Path, "store2") {
				// Underscore key
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"deduplication_factor": 6.6,
					},
				})
				return
			}
		}
		// List
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"store": "store1"},
				{"store": "store2"},
			},
		})
	})
	defer server.Close()

	dss, err := client.GetDatastores(context.Background())
	if err != nil {
		t.Fatalf("GetDatastores failed: %v", err)
	}
	if len(dss) != 2 {
		t.Fatalf("Expected 2 datastores, got %d", len(dss))
	}

	// Check store1 (hyphenated)
	if dss[0].Store == "store1" {
		if dss[0].DeduplicationFactor != 5.5 {
			t.Errorf("Store1: expected dedup 5.5, got %f", dss[0].DeduplicationFactor)
		}
	}
	// Check store2 (underscore)
	if dss[1].Store == "store2" {
		if dss[1].DeduplicationFactor != 6.6 {
			t.Errorf("Store2: expected dedup 6.6, got %f", dss[1].DeduplicationFactor)
		}
	}
}

func TestClient_SetupMonitoringAccess_SetUserACLError(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// CreateUser succeeds (ignored if fails anyway)
		if strings.HasSuffix(r.URL.Path, "users") {
			w.WriteHeader(http.StatusOK)
			return
		}
		// SetUserACL fails
		if strings.HasSuffix(r.URL.Path, "acl") {
			http.Error(w, "acl failed", http.StatusInternalServerError)
			return
		}
	})
	defer server.Close()

	_, _, err := client.SetupMonitoringAccess(context.Background(), "t1")
	if err == nil {
		t.Error("Expected error on SetUserACL failure, got nil")
	}
	if !strings.Contains(err.Error(), "set user ACL") {
		t.Errorf("Expected 'set user ACL' error, got: %v", err)
	}
}

func TestListMethods_Errors(t *testing.T) {
	// ListNamespaces errors
	client1, server1 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "namespace") {
			w.Write([]byte("invalid json"))
			return
		}
	})
	defer server1.Close()
	_, err := client1.ListNamespaces(context.Background(), "s1", "", 0)
	if err == nil {
		t.Error("Expected JSON error for ListNamespaces, got nil")
	}

	client1b, server1b := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Non-404 error
		http.Error(w, "server error", http.StatusInternalServerError)
	})
	defer server1b.Close()
	_, err = client1b.ListNamespaces(context.Background(), "s1", "", 0)
	if err == nil {
		t.Error("Expected server error for ListNamespaces, got nil")
	}

	// ListBackupGroups errors
	client2, server2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad status", http.StatusBadRequest)
	})
	defer server2.Close()
	_, err = client2.ListBackupGroups(context.Background(), "s1", "ns1")
	if err == nil {
		t.Error("Expected error for ListBackupGroups, got nil")
	}

	client3, server3 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	})
	defer server3.Close()
	_, err = client3.ListBackupGroups(context.Background(), "s1", "ns1")
	if err == nil {
		t.Error("Expected JSON error for ListBackupGroups, got nil")
	}

	// ListBackupSnapshots errors
	client4, server4 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad status", http.StatusBadRequest)
	})
	defer server4.Close()
	_, err = client4.ListBackupSnapshots(context.Background(), "s1", "ns1", "vm", "100")
	if err == nil {
		t.Error("Expected error for ListBackupSnapshots, got nil")
	}

	client5, server5 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	})
	defer server5.Close()
	_, err = client5.ListBackupSnapshots(context.Background(), "s1", "ns1", "vm", "100")
	if err == nil {
		t.Error("Expected JSON error for ListBackupSnapshots, got nil")
	}
}

func TestGetNodeName_Errors(t *testing.T) {
	client1, server1 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})
	defer server1.Close()
	_, err := client1.GetNodeName(context.Background())
	if err == nil {
		t.Error("Expected server error for GetNodeName, got nil")
	}

	client2, server2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	})
	defer server2.Close()
	_, err = client2.GetNodeName(context.Background())
	if err == nil {
		t.Error("Expected JSON error for GetNodeName, got nil")
	}
}

func TestListAllBackups_SnapshotError(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/groups") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []BackupGroup{
					{BackupType: "vm", BackupID: "100"},
				},
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/snapshots") {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	})
	defer server.Close()

	results, err := client.ListAllBackups(context.Background(), "store1", []string{"ns1"})
	if err != nil {
		t.Fatalf("ListAllBackups failed (snapshot error should be swallowed): %v", err)
	}
	// Snapshots fail, so list should be empty (but existing)
	if len(results["ns1"]) != 0 {
		t.Errorf("Expected 0 snapshots, got %d", len(results["ns1"]))
	}
}

func TestListAllBackups_ContextCancel(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Delay response to allow cancellation
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.ListAllBackups(ctx, "store1", []string{"ns1"})
	if err == nil {
		t.Error("Expected cancellation error, got nil")
	}
}
func TestSetupMonitoringAccess_Error(t *testing.T) {
	// Test CreateUser fails
	client1, server1 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/users" {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
	})
	defer server1.Close()
	if _, _, err := client1.SetupMonitoringAccess(context.Background(), "test-token"); err == nil {
		t.Error("expected error when CreateUser fails")
	}

	// Test SetUserACL fails
	client2, server2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/users" {
			w.WriteHeader(200)
			return
		}
		if r.URL.Path == "/api2/json/access/acl" {
			http.Error(w, "fail acl", http.StatusInternalServerError)
			return
		}
	})
	defer server2.Close()
	if _, _, err := client2.SetupMonitoringAccess(context.Background(), "test-token"); err == nil {
		t.Error("expected error when SetUserACL fails")
	}
}

func TestListAllBackups_JSONDecodeError(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	})
	defer server.Close()

	// ListBackupGroups will fail to decode
	_, err := client.ListBackupGroups(context.Background(), "store1", "ns1")
	if err == nil {
		t.Error("expected error for invalid json in ListBackupGroups")
	}
}

func TestListBackupSnapshots_JSONDecodeError(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	})
	defer server.Close()

	_, err := client.ListBackupSnapshots(context.Background(), "store1", "ns1", "vm", "100")
	if err == nil {
		t.Error("expected error for invalid json in ListBackupSnapshots")
	}
}

func TestListBackupGroups_JSONDecodeError(t *testing.T) {
	// Covered by TestListAllBackups_JSONDecodeError effectively, but explicit test:
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	})
	defer server.Close()

	_, err := client.ListBackupGroups(context.Background(), "store1", "ns1")
	if err == nil {
		t.Error("expected error for invalid json")
	}
}

func TestListAllBackups_ContextCancellation_Inner(t *testing.T) {
	// We want to trigger ctx.Done() inside the group processing loop

	ctx, cancel := context.WithCancel(context.Background())

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "groups") {
			// Return many groups to ensure loop runs
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"backup-type": "vm", "backup-id": "100"},
					{"backup-type": "vm", "backup-id": "101"},
					{"backup-type": "vm", "backup-id": "102"},
				},
			})
			// Cancel context after getting groups but before processing all snapshots
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
			return
		}
		// Slow down snapshot listing to ensure cancellation is hit
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[]}`))
	})
	defer server.Close()
	defer cancel() // ensure cancel called

	_, err := client.ListAllBackups(ctx, "store1", []string{"ns1"})
	// We expect an error, likely context canceled
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

// Helper to test read errors
type bodyReadErrorReader struct{}

func (e *bodyReadErrorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}
func (e *bodyReadErrorReader) Close() error { return nil }

func TestCreateUserToken_ReadBodyError_JSON(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// We can't strictly force io.ReadAll to fail easily with httptest unless we do something hacky.
		// Instead we can use a small buffer and panic or something, but io.ReadAll usually works.
		// However, we can mock the client.httpClient or use a transport that returns a bad body.
		// Since we cannot mock httpClient easily via NewClient, we can set it.
		w.WriteHeader(200)
	})
	defer server.Close()

	// Replace httpClient with one that returns an errorReader
	client.httpClient.Transport = &readErrorTransport{
		transport: http.DefaultTransport,
	}

	_, err := client.CreateUserToken(context.Background(), "user1@pbs", "token")

	if err == nil {
		t.Error("expected error reading body")
	}
}

func TestListBackupSnapshots_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "token",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.ListBackupSnapshots(context.Background(), "store", "", "vm", "100")
	if err == nil {
		t.Error("Expected error for HTTP 500")
	}
}

func TestListBackupGroups_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "token",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.ListBackupGroups(context.Background(), "store", "")
	if err == nil {
		t.Error("Expected error for HTTP 500")
	}
}

type readErrorTransport struct {
	transport http.RoundTripper
}

func (et *readErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := et.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &bodyReadErrorReader{}
	return resp, nil
}

func TestGetNodeStatus_ReadBodyError(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	defer server.Close()

	client.httpClient.Transport = &readErrorTransport{
		transport: http.DefaultTransport,
	}

	_, err := client.GetNodeStatus(context.Background())
	if err == nil {
		t.Error("expected error reading body in GetNodeStatus")
	}
}
