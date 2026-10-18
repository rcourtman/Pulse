package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Exercise the real bounded HTTP reader, not just a detached JSON parser.
func TestNodeInventoryResponseAdmission(t *testing.T) {
	const inventory = `{"data":[{"node":"member","status":"online","cpu":0.25}]}`
	cases := []struct {
		name, body string
		complete   bool
	}{
		{"missing-data", `{}`, true},
		{"error-as-success", `{"errors":{"message":"fixture-private-detail"}}`, true},
		{"conflicting-error", `{"data":[],"errors":{"message":"fixture-private-detail"}}`, true},
		{"null-inventory", `{"data":null}`, true},
		{"null-envelope", `null`, true},
		{"case-data", `{"Data":[]}`, true},
		{"duplicate-data", `{"data":{},"data":[]}`, true},
		{"escaped-duplicate-data", `{"data":{},"\u0064ata":[]}`, true},
		{"case-overwrite", `{"data":[],"Data":null}`, true},
		{"trailing-value", inventory + ` {"errors":{"message":"fixture-private-detail"}}`, true},
		{"trailing-garbage", inventory + `not-json`, true},
		{"truncated-after-prefix", inventory, false},
		{"oversized-after-prefix", inventory + strings.Repeat(" ", int(maxResponseBodyBytes)), true},
		{"invalid-utf8", `{"data":[],"metadata":"` + string([]byte{0xff}) + `"}`, true},
		{"null-row", `{"data":[null]}`, true},
		{"unnamed-row", `{"data":[{}]}`, true},
		{"empty-node", `{"data":[{"node":""}]}`, true},
		{"null-node", `{"data":[{"node":null}]}`, true},
		{"duplicate-node-field", `{"data":[{"node":"other","node":"member"}]}`, true},
		{"case-node-field", `{"data":[{"node":"member","Node":"other"}]}`, true},
		{"duplicate-node-row", `{"data":[{"node":"member"},{"node":"member"}]}`, true},
		{"wrong-node-type", `{"data":[{"node":42}]}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/api2/json/nodes" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if !tc.complete {
					w.Header().Set("Content-Length", fmt.Sprint(len(tc.body)+100))
				}
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c := backupTestClient(t, server.URL)
			nodes, err := c.GetNodes(context.Background())
			if err == nil || nodes != nil {
				t.Errorf("unverified inventory admitted: nodes=%+v err=%v", nodes, err)
			}
			if err != nil && strings.Contains(err.Error(), "fixture-private-detail") {
				t.Error("response body leaked into admission error")
			}
			if requests.Load() != 1 {
				t.Errorf("invalid success replayed: %d requests", requests.Load())
			}
		})
	}
}

func TestNodeInventoryAcceptedScopedAndExtendedResponses(t *testing.T) {
	for _, body := range []string{
		`{"data":[]}`,
		`{"data":[],"errors":null,"total":0}`,
		`{"data":[{"node":"member","status":"offline","cpu":0.25,"maxcpu":8,"mem":128,"maxmem":512,"disk":256,"maxdisk":1024,"uptime":42,"level":"c","id":"node/member","type":"node"}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			c := backupTestClient(t, server.URL)
			nodes, err := c.GetNodes(context.Background())
			if err != nil {
				t.Fatalf("accepted scoped/extended inventory refused: %v", err)
			}
			if strings.Contains(body, `"node"`) && (len(nodes) != 1 || nodes[0].Node != "member" || nodes[0].Status != "offline" || nodes[0].CPU != 0.25 || nodes[0].MaxCPU != 8 || nodes[0].Mem != 128 || nodes[0].MaxMem != 512 || nodes[0].Disk != 256 || nodes[0].MaxDisk != 1024 || nodes[0].Uptime != 42 || nodes[0].Level != "c") {
				t.Fatalf("accepted node fields changed: %+v", nodes)
			}
			if !strings.Contains(body, `"node"`) && len(nodes) != 0 {
				t.Fatalf("scoped empty inventory not empty: %+v", nodes)
			}
		})
	}
}
