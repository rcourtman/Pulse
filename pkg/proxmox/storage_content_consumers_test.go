package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// All content consumers use the real client boundary. A backup-only filter
// here makes the existing snapshot-size and container-root readers dead code.
func TestStorageContentRetainsAllValidatedKinds(t *testing.T) {
	for _, kind := range []string{"single", "cluster"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api2/json/nodes" {
					fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
					return
				}
				if r.Method != http.MethodGet || r.URL.Path != "/api2/json/nodes/node/storage/archive/content" {
					t.Errorf("unexpected read: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				fmt.Fprint(w, `{"data":[{"volid":"archive:backup/100","content":"backup"},{"volid":"archive:vztmpl/template","content":"vztmpl"},{"volid":"archive:iso/disc","content":"iso"},{"volid":"archive:vm-100-disk-0@weekly","content":"images","vmid":100,"size":1024},{"volid":"archive:subvol-200-disk-0","content":"rootdir","vmid":200,"size":4096,"used":1024},{"volid":"archive:future","content":"future-kind"}]}`)
			}))
			defer server.Close()
			keys, err := storageInventoryReader(t, server.URL, kind, "content")(context.Background())
			want := []string{"archive:backup/100", "archive:vztmpl/template", "archive:iso/disc", "archive:vm-100-disk-0@weekly", "archive:subvol-200-disk-0", "archive:future"}
			if err != nil || !reflect.DeepEqual(keys, want) {
				t.Fatalf("validated content was hidden from its consumers: got %v / %v, want %v", keys, err, want)
			}
		})
	}
}
