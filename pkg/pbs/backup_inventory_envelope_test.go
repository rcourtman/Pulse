package pbs

import (
	"context"
	"net/http"
	"testing"
)

// A successful HTTP status without an inventory array is an unread scope, not
// an accepted empty listing. In particular, decoding JSON null is not a parse
// error in Go and must not silently authorize backup-history reconciliation.
func TestBackupInventoryRequiresExplicitArray(t *testing.T) {
	readers := []struct {
		name string
		read func(*Client) (int, error)
	}{
		{"namespaces", func(c *Client) (int, error) {
			rows, err := c.ListNamespaces(context.Background(), "archive", "tenant", 2)
			return len(rows), err
		}},
		{"groups", func(c *Client) (int, error) {
			rows, err := c.ListBackupGroups(context.Background(), "archive", "tenant")
			return len(rows), err
		}},
		{"snapshots", func(c *Client) (int, error) {
			rows, err := c.ListBackupSnapshots(context.Background(), "archive", "tenant", "vm", "101")
			return len(rows), err
		}},
	}
	for _, reader := range readers {
		for _, body := range []string{`null`, `{}`, `{"data":null}`, `{"unrelated":[]}`, `{"data":{}}`, `{"data":false}`, `{"data":`, `{"data":[]}`} {
			t.Run(reader.name+"/"+body, func(t *testing.T) {
				reads := 0
				client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					reads++
					if r.Method != http.MethodGet {
						t.Errorf("inventory read made a %s request", r.Method)
					}
					_, _ = w.Write([]byte(body))
				})
				defer server.Close()
				count, err := reader.read(client)
				if body == `{"data":[]}` {
					if err != nil || count != 0 {
						t.Fatalf("explicit empty array rejected: count=%d err=%v", count, err)
					}
				} else if err == nil || count != 0 {
					t.Fatalf("unread scope accepted: count=%d err=%v", count, err)
				}
				if reads != 1 {
					t.Fatalf("inventory admission caused %d requests, want one", reads)
				}
			})
		}
	}
}
