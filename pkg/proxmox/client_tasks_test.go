package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetNodeTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes/node1/tasks" {
			fmt.Fprint(w, `{"data":[{"upid":"upid:1","type":"vzdump","status":"ok"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	tasks, err := client.GetNodeTasks(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetNodeTasks failed: %v", err)
	}
	if len(tasks) != 1 || tasks[0].UPID != "upid:1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}

func TestClient_GetBackupTasksFiltersAndSkipsErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"},{"node":"node2","status":"offline"},{"node":"node3","status":"online"}]}`)
		case "/api2/json/nodes/node1/tasks":
			fmt.Fprint(w, `{"data":[{"upid":"upid:1","type":"vzdump"},{"upid":"upid:2","type":"qmstart"}]}`)
		case "/api2/json/nodes/node3/tasks":
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"errors":"boom"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	tasks, err := client.GetBackupTasks(context.Background())
	if err != nil {
		t.Fatalf("GetBackupTasks failed: %v", err)
	}
	if len(tasks) != 1 || tasks[0].UPID != "upid:1" {
		t.Fatalf("unexpected backup tasks: %+v", tasks)
	}
}

func TestClient_GetBackupTasks_NodeListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/nodes" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"errors":"boom"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if _, err := client.GetBackupTasks(context.Background()); err == nil {
		t.Fatal("expected error when node list fails")
	}
}
