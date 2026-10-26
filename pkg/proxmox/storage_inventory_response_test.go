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

func readTestStorageInventory(c *Client, kind string) (int, bool, error) {
	if kind == "content" {
		rows, err := c.GetStorageContent(context.Background(), "member", "archive")
		return len(rows), rows == nil, err
	}
	var rows []Storage
	var err error
	if kind == "node" {
		rows, err = c.GetStorage(context.Background(), "member")
	} else {
		rows, err = c.GetAllStorage(context.Background())
	}
	return len(rows), rows == nil, err
}

func TestStorageInventoryResponseAdmission(t *testing.T) {
	for _, kind := range []string{"node", "configuration", "content"} {
		t.Run(kind, func(t *testing.T) {
			row, id, endpoint := `{"storage":"archive","type":"dir","content":"backup"}`, "storage", "/api2/json/storage"
			if kind == "node" {
				endpoint = "/api2/json/nodes/member/storage"
			} else if kind == "content" {
				row, id, endpoint = `{"volid":"archive:backup/accepted","content":"backup","ctime":1700000000}`, "volid", "/api2/json/nodes/member/storage/archive/content"
			}
			inventory := `{"data":[` + row + `]}`
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
				{"missing-identity", `{"data":[{"content":"backup","type":"dir"}]}`, false},
				{"empty-identity", `{"data":[{"` + id + `":"","content":"backup","type":"dir"}]}`, false},
				{"null-identity", `{"data":[{"` + id + `":null,"content":"backup","type":"dir"}]}`, false},
				{"whitespace-identity", `{"data":[{"` + id + `":"  ","content":"backup","type":"dir"}]}`, false},
				{"duplicate-identity-field", `{"data":[{"` + id + `":"other",` + row[1:] + `]}`, false},
				{"case-identity-field", `{"data":[{"` + strings.ToUpper(id) + `":"other",` + row[1:] + `]}`, false},
				{"case-content-field", `{"data":[{"Content":"iso",` + row[1:] + `]}`, false},
				{"duplicate-row", `{"data":[` + row + `,` + row + `]}`, false},
				{"bad-row-after-good", `{"data":[` + row + `,{"` + id + `":"second","content":false}]}`, false},
			}
			if kind != "configuration" {
				for _, value := range []string{"", `,"content":null`, `,"content":""`, `,"content":" "`} {
					cases = append(cases, struct {
						name, body string
						incomplete bool
					}{"unread-content/" + value, `{"data":[{"` + id + `":"accepted","type":"dir"` + value + `}]}`, false})
				}
			}
			if kind == "node" {
				cases = append(cases, struct {
					name, body string
					incomplete bool
				}{"missing-type", `{"data":[{"storage":"archive","content":"backup"}]}`, false})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var requests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						if r.URL.Path != endpoint {
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
					count, nilRows, err := readTestStorageInventory(backupTestClient(t, server.URL), kind)
					if err == nil || !nilRows || count != 0 {
						t.Errorf("unverified inventory admitted: count=%d nil=%t err=%v", count, nilRows, err)
					}
					if err != nil && strings.Contains(err.Error(), "fixture-private-detail") {
						t.Error("response body leaked into storage diagnostic")
					}
					if requests.Load() != 1 {
						t.Errorf("invalid success replayed: %d requests", requests.Load())
					}
				})
			}
		})
	}
}

func TestStorageInventoryAcceptedScopedAndExtendedResponses(t *testing.T) {
	for _, kind := range []string{"node", "configuration", "content"} {
		for _, body := range []string{`{"data":[]}`, `{"data":[],"errors":null,"total":0}`} {
			t.Run(kind+"/"+body, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
				defer server.Close()
				count, _, err := readTestStorageInventory(backupTestClient(t, server.URL), kind)
				if err != nil || count != 0 {
					t.Fatalf("scoped empty inventory refused: count=%d err=%v", count, err)
				}
			})
		}
	}
	t.Run("node-fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":[{"storage":"archive","type":"dir","content":"backup,images","active":1,"enabled":1,"shared":1,"nodes":"member,peer","pool":"tank","path":"/archive","total":100,"used":0,"avail":100,"provider-metadata":{"revision":1}}],"errors":null}`)
		}))
		defer server.Close()
		rows, err := backupTestClient(t, server.URL).GetStorage(context.Background(), "member")
		if err != nil || len(rows) != 1 || rows[0] != (Storage{Storage: "archive", Type: "dir", Content: "backup,images", Active: 1, Enabled: 1, Shared: 1, Nodes: "member,peer", Pool: "tank", Path: "/archive", Total: 100, Available: 100}) {
			t.Fatalf("accepted storage fields changed: %+v err=%v", rows, err)
		}
	})
	t.Run("configuration-defaults", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"data":[{"storage":"archive"}]}`) }))
		defer server.Close()
		count, _, err := readTestStorageInventory(backupTestClient(t, server.URL), "configuration")
		if err != nil || count != 1 {
			t.Fatalf("configuration defaults refused: count=%d err=%v", count, err)
		}
	})
	t.Run("all-content-validated-before-filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":[{"volid":"archive:backup/accepted","content":"backup","ctime":1700000000,"size":0,"vmid":101,"format":"pbs-vm","protected":1,"verified":1,"verification":{"state":"ok"},"notes":"accepted"},{"volid":"archive:backup/host","content":"backup","vmid":0},{"volid":"archive:vztmpl/template","content":"vztmpl"},{"volid":"archive:iso/image","content":"iso"},{"volid":"archive:future/item","content":"future-kind","future-metadata":1}]}`)
		}))
		defer server.Close()
		rows, err := backupTestClient(t, server.URL).GetStorageContent(context.Background(), "member", "archive")
		if err != nil || len(rows) != 3 || rows[0].CTime != 1700000000 || rows[0].VMID != 101 || rows[0].Size != 0 || rows[0].Format != "pbs-vm" || rows[0].Protected != 1 || rows[0].Verified != 1 || rows[0].Verification["state"] != "ok" || rows[0].Notes != "accepted" || rows[1].VMID != 0 || rows[2].Content != "vztmpl" {
			t.Fatalf("accepted backup/template filter or fields changed: %+v err=%v", rows, err)
		}
	})
}

func TestStorageInventoryRequiresCompletedResponse(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusPartialContent} {
		for _, content := range []bool{false, true} {
			resp := &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}
			var err error
			if content {
				_, err = readStorageContentInventory(resp)
			} else {
				_, err = readStorageCatalog(resp, true)
			}
			if err == nil {
				t.Errorf("status %d established absence (content=%t)", status, content)
			}
		}
	}
	for _, content := range []bool{false, true} {
		body := &snapshotCloseFailure{Reader: strings.NewReader(`{"data":[]}`)}
		resp := &http.Response{StatusCode: http.StatusOK, Body: body}
		var err error
		if content {
			_, err = readStorageContentInventory(resp)
		} else {
			_, err = readStorageCatalog(resp, true)
		}
		if err == nil || !body.closed {
			t.Errorf("unfinished close established absence (content=%t): err=%v closed=%t", content, err, body.closed)
		}
	}
}
