package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClusterClient_GetCephDF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/cluster/ceph/df" {
			fmt.Fprint(w, `{"data":{"data":{"stats":{"total_bytes":100,"total_used_bytes":20}}}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	df, err := cc.GetCephDF(context.Background())
	if err != nil {
		t.Fatalf("GetCephDF failed: %v", err)
	}
	if df == nil || df.Data.Stats.TotalBytes != 100 {
		t.Fatalf("unexpected ceph df: %+v", df)
	}
}

func TestClusterClient_GetReplicationStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
		case "/api2/json/cluster/replication":
			fmt.Fprint(w, `{"data":[{"id":"job1","guest":"vm/100","source":"node1"}]}`)
		case "/api2/json/nodes/node1/replication/job1/status":
			fmt.Fprint(w, `{"data":{"last_sync":1700000000,"duration":10,"fail_count":0,"state":"ok"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	jobs, err := cc.GetReplicationStatus(context.Background())
	if err != nil {
		t.Fatalf("GetReplicationStatus failed: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "job1" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
	if jobs[0].LastSyncUnix == 0 || jobs[0].State == "" {
		t.Fatalf("expected enriched status fields, got %+v", jobs[0])
	}
}

func TestClusterClient_IsClusterMemberFalseOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/cluster/status" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"data":null}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	member, err := cc.IsClusterMember(context.Background())
	if err != nil {
		t.Fatalf("IsClusterMember returned error: %v", err)
	}
	if member {
		t.Fatal("expected IsClusterMember to return false when status fetch fails")
	}
}
