package proxmox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSnapshotInventoryResponseAdmission(t *testing.T) {
	row := `{"name":"weekly","snaptime":1700000000,"description":"fixture","parent":"base"}`
	for _, kind := range []string{"qemu", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			for _, tc := range []struct {
				name, body string
				incomplete bool
				status     int
			}{
				{"missing-data", `{}`, false, 200},
				{"null-data", `{"data":null}`, false, 200},
				{"null-envelope", `null`, false, 200},
				{"object-data", `{"data":{}}`, false, 200},
				{"conflicting-error", `{"data":[],"errors":{"message":"fixture-private-detail"}}`, false, 200},
				{"case-data", `{"Data":[]}`, false, 200},
				{"duplicate-data", `{"data":[` + row + `],"data":[]}`, false, 200},
				{"escaped-duplicate-data", `{"data":[],"\u0064ata":[` + row + `]}`, false, 200},
				{"case-overwrite", `{"data":[` + row + `],"Data":[]}`, false, 200},
				{"trailing-value", `{"data":[]} {"data":[` + row + `]}`, false, 200},
				{"trailing-garbage", `{"data":[]} fixture-private-detail`, false, 200},
				{"incomplete-after-prefix", `{"data":[]}`, true, 200},
				{"oversized-after-prefix", `{"data":[]}` + strings.Repeat(" ", int(maxResponseBodyBytes)), false, 200},
				{"invalid-utf8", `{"data":[` + row + `],"metadata":"` + string([]byte{0xff}) + `"}`, false, 200},
				{"null-row", `{"data":[` + row + `,null]}`, false, 200},
				{"missing-name", `{"data":[{"snaptime":1700000000}]}`, false, 200},
				{"null-name", `{"data":[{"name":null}]}`, false, 200},
				{"blank-name", `{"data":[{"name":"  "}]}`, false, 200},
				{"duplicate-name-field", `{"data":[{"name":"weekly","name":"current"}]}`, false, 200},
				{"case-name-field", `{"data":[{"name":"weekly","Name":"current"}]}`, false, 200},
				{"duplicate-time-field", `{"data":[{"name":"weekly","snaptime":10,"snaptime":20}]}`, false, 200},
				{"case-time-field", `{"data":[{"name":"weekly","SnapTime":20}]}`, false, 200},
				{"duplicate-row", `{"data":[` + row + `,` + row + `]}`, false, 200},
				{"duplicate-current", `{"data":[{"name":"current"},{"name":"current"}]}`, false, 200},
				{"bad-row-after-good", `{"data":[` + row + `,{"name":"other","snaptime":"invalid"}]}`, false, 200},
				{"unexpected-success-status", `{"data":[]}`, false, 206},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var requests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						if r.URL.Path != fmt.Sprintf("/api2/json/nodes/member/%s/105/snapshot", kind) || r.Method != http.MethodGet {
							t.Errorf("unexpected operation %s %s", r.Method, r.URL.Path)
						}
						if tc.incomplete {
							w.Header().Set("Content-Length", fmt.Sprint(len(tc.body)+100))
						}
						w.WriteHeader(tc.status)
						fmt.Fprint(w, tc.body)
					}))
					defer server.Close()
					client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"})
					if err != nil {
						t.Fatal(err)
					}
					defer client.httpClient.CloseIdleConnections()
					var snapshots []Snapshot
					if kind == "qemu" {
						snapshots, err = client.GetVMSnapshots(context.Background(), "member", 105)
					} else {
						snapshots, err = client.GetContainerSnapshots(context.Background(), "member", 105)
					}
					if err == nil || snapshots != nil {
						t.Errorf("unverified inventory admitted: %+v / %v", snapshots, err)
					}
					if err != nil && strings.Contains(err.Error(), "fixture-private-detail") {
						t.Error("provider contents became a diagnostic")
					}
					if requests.Load() != 1 {
						t.Errorf("request replayed: %d", requests.Load())
					}
				})
			}
		})
	}
}

func TestSnapshotInventoryPreservesSupportedFields(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"explicit-empty", `{"data":[]}`, 0},
		{"current-only", `{"data":[{"name":"current","digest":"additive"}],"errors":null}`, 0},
		{"optional-time", `{"data":[{"name":"weekly","extra":{"future":true}}],"metadata":true}`, 1},
		{"unknown-time", `{"data":[{"name":"weekly","snaptime":null}]}`, 1},
		{"future-time", `{"data":[{"name":"weekly","snaptime":4102444800}]}`, 1},
		{"ordinary", `{"data":[{"name":"current"},{"name":"weekly","snaptime":1700000000,"description":"fixture","parent":"base","vmid":999}]}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.body))}
			snapshots, err := readSnapshotInventory(resp, 105)
			if err != nil || len(snapshots) != tc.want {
				t.Fatalf("supported inventory rejected: %+v / %v", snapshots, err)
			}
			if tc.want > 0 && (snapshots[0].Name != "weekly" || snapshots[0].VMID != 105) {
				t.Fatalf("endpoint identity lost: %+v", snapshots)
			}
			if tc.name == "ordinary" && (snapshots[0].SnapTime != 1700000000 || snapshots[0].Description != "fixture" || snapshots[0].Parent != "base") {
				t.Fatalf("optional metadata changed: %+v", snapshots)
			}
		})
	}
}
