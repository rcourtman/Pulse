package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Enumerations drive removal, backup-template policy and History. Exercise the
// real HTTP readers on both production routes, with no QGA commands or retries.
func TestGuestInventoryResponseAdmission(t *testing.T) {
	for _, kind := range []string{"cluster", "qemu", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			row := `{"vmid":105,"type":"qemu","node":"member","name":"fixture","status":"running"}`
			inventory := `{"data":[` + row + `]}`
			cases := []struct {
				name, body string
				incomplete bool
			}{
				{"missing-data", `{}`, false},
				{"null-data", `{"data":null}`, false},
				{"null-envelope", `null`, false},
				{"conflicting-error", `{"data":[],"errors":{"message":"fixture-private-detail"}}`, false},
				{"case-data", `{"Data":[]}`, false},
				{"duplicate-data", `{"data":[` + row + `],"data":[]}`, false},
				{"escaped-duplicate-data", `{"data":[],"\u0064ata":[` + row + `]}`, false},
				{"case-overwrite", `{"data":[` + row + `],"Data":[]}`, false},
				{"trailing-value", inventory + ` {"data":[]}`, false},
				{"trailing-garbage", inventory + `fixture-private-detail`, false},
				{"truncated-after-prefix", inventory, true},
				{"oversized-after-prefix", inventory + strings.Repeat(" ", int(maxResponseBodyBytes)), false},
				{"invalid-utf8", `{"data":[` + row + `],"metadata":"` + string([]byte{0xff}) + `"}`, false},
				{"null-row", `{"data":[` + row + `,null]}`, false},
				{"missing-guest-id", `{"data":[{"type":"qemu","node":"member"}]}`, false},
				{"zero-guest-id", `{"data":[{"vmid":0,"type":"qemu","node":"member"}]}`, false},
				{"negative-guest-id", `{"data":[{"vmid":-105,"type":"qemu","node":"member"}]}`, false},
				{"duplicate-field", `{"data":[{"vmid":106,"vmid":105,"type":"qemu","node":"member"}]}`, false},
				{"case-id", `{"data":[{"vmid":105,"VMID":106,"type":"qemu","node":"member"}]}`, false},
				{"duplicate-row", `{"data":[` + row + `,` + row + `]}`, false},
				{"bad-row-after-good", `{"data":[` + row + `,{"vmid":106,"type":"qemu","node":"member","mem":"not-a-number"}]}`, false},
			}
			if kind == "cluster" {
				cases = append(cases, struct {
					name, body string
					incomplete bool
				}{"missing-node", `{"data":[{"vmid":105,"type":"qemu"}]}`, false},
					struct {
						name, body string
						incomplete bool
					}{"missing-type", `{"data":[{"vmid":105,"node":"member"}]}`, false},
					struct {
						name, body string
						incomplete bool
					}{"duplicate-migrated-id", `{"data":[` + row + `,{"vmid":105,"type":"qemu","node":"other"}]}`, false})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var requests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						if tc.incomplete {
							w.Header().Set("Content-Length", fmt.Sprint(len(tc.body)+100))
						} else {
							w.(http.Flusher).Flush()
						}
						fmt.Fprint(w, tc.body)
					}))
					defer server.Close()
					c, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 15 * time.Second})
					if err != nil {
						t.Fatal(err)
					}
					defer c.httpClient.CloseIdleConnections()
					var values any
					switch kind {
					case "cluster":
						values, err = c.GetClusterResources(context.Background(), "vm")
					case "qemu":
						values, err = c.GetVMs(context.Background(), "member")
					case "lxc":
						values, err = c.GetContainers(context.Background(), "member")
					}
					if err == nil || !reflect.ValueOf(values).IsNil() {
						t.Errorf("unverified %s inventory admitted: rows=%+v err=%v", kind, values, err)
					}
					if err != nil && strings.Contains(err.Error(), "fixture-private-detail") {
						t.Error("private response body became an inventory diagnostic")
					}
					if requests.Load() != 1 {
						t.Errorf("enumeration was replayed: %d requests", requests.Load())
					}
				})
			}
		})
	}
}
