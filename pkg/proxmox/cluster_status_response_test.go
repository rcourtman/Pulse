package proxmox

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestGetClusterStatusBoundsMembership(t *testing.T) {
	var entries atomic.Int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/cluster/status" {
			http.NotFound(w, r)
			return
		}
		status := []ClusterStatus{{Type: "cluster", Name: "prod", Quorate: 1}}
		for len(status) < int(entries.Load()) {
			status = append(status, ClusterStatus{Type: "node", Name: "node" + strconv.Itoa(len(status)), Online: 1})
		}
		writeJSON(t, w, map[string]interface{}{"data": status})
	})

	entries.Store(maxClusterNodes + 1) // the cluster and its members
	status, err := client.GetClusterStatus(context.Background())
	if err != nil || len(status) != maxClusterNodes+1 {
		t.Fatalf("reply at the limit: %d entries, error %v", len(status), err)
	}

	entries.Store(maxClusterNodes + 2)
	status, err = client.GetClusterStatus(context.Background())
	if err == nil || status != nil {
		t.Fatalf("reply beyond the limit: %d entries, error %v", len(status), err)
	}
	// The refusal is a failed read of a reachable endpoint: failover must not
	// take it for a connectivity, authentication or rate limit failure.
	if rateLimited, _ := isTransientRateLimitError(err); rateLimited || isAuthError(err) || isEndpointConnectivityError(err) {
		t.Fatalf("refusal misclassified: %v", err)
	}
	if member, _ := client.IsClusterMember(context.Background()); member {
		t.Fatal("a refused membership reply must not establish cluster membership")
	}
}
