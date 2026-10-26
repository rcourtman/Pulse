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

func readTestSnapshotInventory(c *Client, kind string) ([]Snapshot, error) {
	if kind == "qemu" {
		return c.GetVMSnapshots(context.Background(), "member", 105)
	}
	return c.GetContainerSnapshots(context.Background(), "member", 105)
}

// Exercise both actual HTTP readers: invalid success must return no rows,
// never a partial/empty authoritative enumeration or response-body diagnostic.
func TestSnapshotInventoryResponseAdmission(t *testing.T) {
	const row = `{"name":"weekly","snaptime":1700000000,"description":"accepted"}`
	const inventory = `{"data":[` + row + `]}`
	cases := []struct {
		name, body string
		incomplete bool
	}{
		{"missing-data", `{}`, false},
		{"null-data", `{"data":null}`, false},
		{"null-envelope", `null`, false},
		{"non-array", `{"data":{}}`, false},
		{"error-as-success", `{"errors":{"message":"fixture-private-detail"}}`, false},
		{"conflicting-error", `{"data":[],"errors":{"message":"fixture-private-detail"}}`, false},
		{"case-data", `{"Data":[]}`, false},
		{"duplicate-data", `{"data":[` + row + `],"data":[]}`, false},
		{"escaped-duplicate-data", `{"data":[],"\u0064ata":[` + row + `]}`, false},
		{"case-overwrite", `{"data":[` + row + `],"Data":[]}`, false},
		{"case-error", `{"data":[],"Errors":null}`, false},
		{"trailing-value", inventory + ` {"data":[]}`, false},
		{"trailing-garbage", inventory + `fixture-private-detail`, false},
		{"truncated-after-prefix", inventory, true},
		{"oversized-after-prefix", inventory + strings.Repeat(" ", int(maxResponseBodyBytes)), false},
		{"invalid-utf8", `{"data":[` + row + `],"metadata":"` + string([]byte{0xff}) + `"}`, false},
		{"null-row", `{"data":[` + row + `,null]}`, false},
		{"empty-name", `{"data":[{"name":"","snaptime":1700000000}]}`, false},
		{"missing-name", `{"data":[{"snaptime":1700000000}]}`, false},
		{"null-name", `{"data":[{"name":null,"snaptime":1700000000}]}`, false},
		{"whitespace-name", `{"data":[{"name":"  ","snaptime":1700000000}]}`, false},
		{"duplicate-name-field", `{"data":[{"name":"other","name":"weekly","snaptime":1700000000}]}`, false},
		{"case-name-field", `{"data":[{"name":"weekly","Name":"other","snaptime":1700000000}]}`, false},
		{"missing-time", `{"data":[{"name":"weekly"}]}`, false},
		{"null-time", `{"data":[{"name":"weekly","snaptime":null}]}`, false},
		{"zero-time", `{"data":[{"name":"weekly","snaptime":0}]}`, false},
		{"negative-time", `{"data":[{"name":"weekly","snaptime":-1}]}`, false},
		{"string-time", `{"data":[{"name":"weekly","snaptime":"1700000000"}]}`, false},
		{"case-time-field", `{"data":[{"name":"weekly","snaptime":1700000000,"SnapTime":0}]}`, false},
		{"duplicate-row", `{"data":[` + row + `,` + row + `]}`, false},
		{"bad-row-after-good", `{"data":[` + row + `,{"name":"second","snaptime":false}]}`, false},
		{"bad-marker-after-good", `{"data":[` + row + `,{"name":"current","description":42}]}`, false},
	}
	for _, kind := range []string{"qemu", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var requests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						if r.URL.Path != "/api2/json/nodes/member/"+kind+"/105/snapshot" {
							t.Errorf("unexpected path: %s", r.URL.Path)
						}
						if tc.incomplete {
							w.Header().Set("Content-Length", fmt.Sprint(len(tc.body)+100))
						} else {
							w.(http.Flusher).Flush()
						}
						fmt.Fprint(w, tc.body)
					}))
					defer server.Close()
					snapshots, err := readTestSnapshotInventory(backupTestClient(t, server.URL), kind)
					if err == nil || snapshots != nil {
						t.Errorf("unverified inventory admitted: snapshots=%+v err=%v", snapshots, err)
					}
					if err != nil && strings.Contains(err.Error(), "fixture-private-detail") {
						t.Error("response body leaked into snapshot diagnostic")
					}
					if requests.Load() != 1 {
						t.Errorf("invalid success replayed: %d requests", requests.Load())
					}
				})
			}
		})
	}
}

func TestSnapshotInventoryAcceptedScopedAndExtendedResponses(t *testing.T) {
	cases := []struct {
		body  string
		count int
	}{
		{`{"data":[]}`, 0},
		{`{"data":[],"errors":null,"total":0}`, 0},
		{`{"data":[{"name":"current","description":"You are here!","digest":"metadata"}]}`, 0},
		{`{"data":[{"name":"current"},{"name":"weekly","snaptime":1700000000,"description":"accepted","parent":"base","vmstate":1,"additional":{"version":1}}],"errors":null}`, 1},
	}
	for _, kind := range []string{"qemu", "lxc"} {
		for _, tc := range cases {
			t.Run(kind+"/"+tc.body, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
				defer server.Close()
				snapshots, err := readTestSnapshotInventory(backupTestClient(t, server.URL), kind)
				if err != nil || len(snapshots) != tc.count {
					t.Fatalf("accepted inventory refused: snapshots=%+v err=%v", snapshots, err)
				}
				if tc.count == 1 && (snapshots[0].Name != "weekly" || snapshots[0].SnapTime != 1700000000 || snapshots[0].Description != "accepted" || snapshots[0].Parent != "base" || snapshots[0].VMID != 105) {
					t.Fatalf("accepted snapshot fields changed: %+v", snapshots)
				}
			})
		}
	}
}

type snapshotCloseFailure struct {
	io.Reader
	closed bool
}

func (b *snapshotCloseFailure) Close() error {
	b.closed = true
	return io.ErrClosedPipe
}

func TestSnapshotInventoryRequiresCompletedResponse(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusPartialContent} {
		body := io.NopCloser(strings.NewReader(`{"data":[]}`))
		rows, err := readSnapshotInventory(&http.Response{StatusCode: status, Body: body}, 105)
		if rows != nil || err == nil {
			t.Errorf("status %d established absence: rows=%+v err=%v", status, rows, err)
		}
	}
	body := &snapshotCloseFailure{Reader: strings.NewReader(`{"data":[]}`)}
	rows, err := readSnapshotInventory(&http.Response{StatusCode: http.StatusOK, Body: body}, 105)
	if rows != nil || err == nil || !body.closed {
		t.Errorf("unfinished close established absence: rows=%+v err=%v closed=%t", rows, err, body.closed)
	}
}
