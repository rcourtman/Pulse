package proxmox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func storageInventoryReader(t *testing.T, host, kind, endpoint string) func(context.Context) ([]string, error) {
	t.Helper()
	var client interface {
		GetStorage(context.Context, string) ([]Storage, error)
		GetAllStorage(context.Context) ([]Storage, error)
		GetStorageContent(context.Context, string, string) ([]StorageContent, error)
	}
	cfg := ClientConfig{Host: host, TokenName: "fixture@pve!read", TokenValue: "fixture", Timeout: 5 * time.Second}
	if kind == "cluster" {
		cluster := NewClusterClient("storage-inventory-fixture", cfg, []string{host}, nil)
		t.Cleanup(func() {
			for _, c := range cluster.clients {
				c.httpClient.CloseIdleConnections()
			}
		})
		client = cluster
	} else {
		single, err := NewClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(single.httpClient.CloseIdleConnections)
		client = single
	}
	return func(ctx context.Context) ([]string, error) {
		var keys []string
		if endpoint == "content" {
			rows, err := client.GetStorageContent(ctx, "node", "archive")
			for _, row := range rows {
				keys = append(keys, row.Volid)
			}
			return keys, err
		}
		var rows []Storage
		var err error
		if endpoint == "all-storages" {
			rows, err = client.GetAllStorage(ctx)
		} else {
			rows, err = client.GetStorage(ctx, "node")
		}
		for _, row := range rows {
			keys = append(keys, row.Storage)
		}
		return keys, err
	}
}

func storageInventoryFixtureRow(endpoint string) string {
	if endpoint == "content" {
		return `{"volid":"archive:backup/101.vma.zst","content":"backup","vmid":101,"size":0,"ctime":0}`
	}
	return `{"storage":"archive","content":"backup","enabled":1,"active":1,"shared":0,"total":0}`
}

func TestPVEStorageInventoryRejectsAmbiguousSuccessAndRecovers(t *testing.T) {
	for _, kind := range []string{"single", "cluster"} {
		for _, endpoint := range []string{"node-storages", "all-storages", "content"} {
			row := storageInventoryFixtureRow(endpoint)
			healthy := `{"data":[` + row + `],"errors":null,"future":{"schema":2}}`
			identity, scalar := "storage", "enabled"
			if endpoint == "content" {
				identity, scalar = "volid", "size"
			}
			tests := []struct{ name, body string }{
				{"missing-data", `{}`}, {"null-data", `{"data":null}`}, {"object-data", `{"data":{}}`}, {"scalar-data", `{"data":42}`},
				{"duplicate-data", `{"data":[` + row + `],"data":[]}`},
				{"escaped-duplicate-data", `{"data":[` + row + `],"\u0064ata":[]}`},
				{"case-data", `{"data":[` + row + `],"DATA":[]}`},
				{"competing-errors", `{"data":[],"errors":{"private-proxy-detail":"not-empty"}}`},
				{"case-errors", `{"data":[],"ERRORS":{"private-proxy-detail":"not-empty"}}`},
				{"null-row", `{"data":[null]}`}, {"missing-identity", `{"data":[{}]}`},
				{"blank-identity", `{"data":[{"` + identity + `":" \t","content":"backup"}]}`},
				{"duplicate-identity", `{"data":[` + row + `,` + row + `]}`},
				{"duplicate-row-field", `{"data":[` + strings.TrimSuffix(row, "}") + `,"` + scalar + `":0}]}`},
				{"case-row-field", `{"data":[` + strings.TrimSuffix(row, "}") + `,"` + strings.ToUpper(scalar) + `":0}]}`},
				{"null-scalar", `{"data":[{"` + identity + `":"archive","content":"backup","` + scalar + `":null}]}`},
				{"invalid-utf8", "{\"data\":[{\"" + identity + "\":\"\xff\",\"content\":\"backup\"}]}"},
				{"trailing-json", healthy + `{}`}, {"trailing-garbage", healthy + `private-proxy-detail`},
				{"short-body", healthy}, {"oversized-tail", healthy + strings.Repeat(" ", int(maxResponseBodyBytes))},
			}
			for _, tc := range tests {
				t.Run(kind+"/"+endpoint+"/"+tc.name, func(t *testing.T) {
					var recovered atomic.Bool
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/api2/json/nodes" {
							fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
							return
						}
						if r.Method != http.MethodGet || strings.Contains(r.URL.Path, "/agent/") {
							t.Error("unexpected inventory method/path")
						}
						if recovered.Load() {
							fmt.Fprint(w, healthy)
							return
						}
						if tc.name == "short-body" {
							w.Header().Set("Content-Length", "1000")
						}
						fmt.Fprint(w, tc.body)
					}))
					defer server.Close()
					read := storageInventoryReader(t, server.URL, kind, endpoint)
					if rows, err := read(context.Background()); err == nil || len(rows) != 0 || strings.Contains(err.Error(), "private-proxy-detail") || isAuthError(err) {
						t.Fatalf("ambiguous inventory published rows, leaked payload or became an auth failure: count=%d err=%v", len(rows), err)
					}
					recovered.Store(true)
					if rows, err := read(context.Background()); err != nil || len(rows) != 1 {
						t.Fatalf("ordinary complete inventory did not recover: count=%d err=%v", len(rows), err)
					}
				})
			}
		}
	}
}

func TestPVEStorageInventoryRequiresHTTP200(t *testing.T) {
	for _, endpoint := range []string{"node-storages", "all-storages", "content"} {
		for _, status := range []int{http.StatusCreated, http.StatusNoContent, http.StatusPartialContent} {
			t.Run(fmt.Sprintf("%s/%d", endpoint, status), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(status)
					fmt.Fprint(w, `{"data":[]}`)
				}))
				defer server.Close()
				if rows, err := storageInventoryReader(t, server.URL, "single", endpoint)(context.Background()); err == nil || len(rows) != 0 {
					t.Fatal("non-200 became authoritative empty inventory")
				}
			})
		}
	}
}

func TestPVEStorageInventoryWaitsForCompleteBody(t *testing.T) {
	for _, endpoint := range []string{"node-storages", "all-storages", "content"} {
		t.Run(endpoint, func(t *testing.T) {
			entered, finish := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"data":[]}`)
				w.(http.Flusher).Flush()
				close(entered)
				<-finish
				fmt.Fprint(w, " ")
			}))
			defer server.Close()
			defer close(finish)
			read := storageInventoryReader(t, server.URL, "single", endpoint)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := read(ctx); done <- err }()
			<-entered
			select {
			case err := <-done:
				t.Fatalf("JSON prefix escaped before body completion: %v", err)
			case <-time.After(30 * time.Millisecond):
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("body read lost caller cancellation: %v", err)
			}
		})
	}
}

type storageInventoryCloseFailure struct{ io.Reader }

func (storageInventoryCloseFailure) Close() error { return errors.New("synthetic-close-failure") }

func TestPVEStorageInventoryPreservesEmptySparseAndAdditiveData(t *testing.T) {
	for _, body := range []string{`{"data":[]}`, `{"data":[{"storage":"archive"}],"errors":null,"future":1}`, `{"data":[{"storage":"archive","enabled":0,"active":0,"shared":0,"total":0,"used":0,"avail":0,"path":null}]}`} {
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
		if _, err := readStorageList(resp); err != nil {
			t.Fatalf("supported optional/zero/empty storage metadata rejected: %v", err)
		}
	}
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"volid":"archive:backup/101.vma.zst","content":"backup","size":0,"vmid":0,"ctime":0,"notes":null,"verification":null,"future":{"version":2}},{"volid":"archive:vztmpl/ubuntu.tar","content":"vztmpl"},{"volid":"archive:iso/ubuntu.iso","content":"iso"}]}`))}
	rows, err := readStorageContentInventory(resp)
	if err != nil || len(rows) != 2 || rows[0].Size != 0 || rows[0].VMID != 0 || rows[1].Content != "vztmpl" {
		t.Fatalf("zero measurements/host backup/template filtering changed: %+v / %v", rows, err)
	}
	// Even a filtered ISO participates in inventory integrity checks.
	resp = &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"content":"iso"}]}`))}
	if _, err := readStorageContentInventory(resp); err == nil {
		t.Fatal("filtered invalid row hid an incomplete inventory")
	}
	for _, content := range []bool{false, true} {
		resp = &http.Response{StatusCode: http.StatusOK, Body: storageInventoryCloseFailure{strings.NewReader(`{"data":[]}`)}}
		if content {
			_, err = readStorageContentInventory(resp)
		} else {
			_, err = readStorageList(resp)
		}
		if err == nil {
			t.Fatal("close failure established a complete empty inventory")
		}
	}
	// Duplicate spellings of every consumed field must be covered as types grow.
	for _, value := range []any{Storage{}, StorageContent{}} {
		typ := reflect.TypeOf(value)
		for i := 0; i < typ.NumField(); i++ {
			field := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
			body := `{"data":[{"storage":"archive","volid":"archive:backup/101","content":"backup","` + strings.ToUpper(field) + `":null}]}`
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
			if typ == reflect.TypeOf(Storage{}) {
				_, err = readStorageList(resp)
			} else {
				_, err = readStorageContentInventory(resp)
			}
			if err == nil {
				t.Errorf("case-insensitive %s overwrite was accepted", field)
			}
		}
	}
}
