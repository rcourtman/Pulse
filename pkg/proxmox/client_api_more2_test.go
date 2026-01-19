package proxmox

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestClientNodeStatusAndRRD(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/status":
			writeJSON(t, w, map[string]interface{}{
				"data": NodeStatus{CPU: 0.5, KernelVersion: "6.1"},
			})
		case "/api2/json/nodes/node1/rrddata":
			if !strings.Contains(r.URL.RawQuery, "timeframe=hour") || !strings.Contains(r.URL.RawQuery, "cf=AVERAGE") {
				http.Error(w, "bad query", http.StatusBadRequest)
				return
			}
			writeJSON(t, w, map[string]interface{}{
				"data": []NodeRRDPoint{{Time: 123}},
			})
		case "/api2/json/nodes/node1/lxc/101/rrddata":
			if !strings.Contains(r.URL.RawQuery, "ds=memused") {
				http.Error(w, "bad query", http.StatusBadRequest)
				return
			}
			writeJSON(t, w, map[string]interface{}{
				"data": []GuestRRDPoint{{Time: 456}},
			})
		case "/api2/json/nodes/node1/disks/list":
			writeJSON(t, w, map[string]interface{}{
				"data": []Disk{{DevPath: "/dev/sda", Model: "Disk"}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	status, err := client.GetNodeStatus(ctx, "node1")
	if err != nil {
		t.Fatalf("GetNodeStatus error: %v", err)
	}
	if status.KernelVersion != "6.1" {
		t.Fatalf("unexpected node status: %+v", status)
	}

	rrd, err := client.GetNodeRRDData(ctx, "node1", "", "", []string{"cpu"})
	if err != nil {
		t.Fatalf("GetNodeRRDData error: %v", err)
	}
	if len(rrd) != 1 || rrd[0].Time != 123 {
		t.Fatalf("unexpected node rrd: %+v", rrd)
	}

	guestRRD, err := client.GetLXCRRDData(ctx, "node1", 101, "", "", []string{"memused"})
	if err != nil {
		t.Fatalf("GetLXCRRDData error: %v", err)
	}
	if len(guestRRD) != 1 || guestRRD[0].Time != 456 {
		t.Fatalf("unexpected guest rrd: %+v", guestRRD)
	}

	disks, err := client.GetDisks(ctx, "node1")
	if err != nil {
		t.Fatalf("GetDisks error: %v", err)
	}
	if len(disks) != 1 || disks[0].DevPath != "/dev/sda" {
		t.Fatalf("unexpected disks: %+v", disks)
	}
}

func TestClientNodeNetworkInterfaces(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/network":
			writeJSON(t, w, map[string]interface{}{
				"data": []NodeNetworkInterface{{Iface: "eth0", Active: 1}},
			})
		case "/api2/json/nodes/bad/network":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	ifaces, err := client.GetNodeNetworkInterfaces(ctx, "node1")
	if err != nil {
		t.Fatalf("GetNodeNetworkInterfaces error: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].Iface != "eth0" {
		t.Fatalf("unexpected interfaces: %+v", ifaces)
	}

	if _, err := client.GetNodeNetworkInterfaces(ctx, "bad"); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}
