package pbs

import (
	"context"
	"net/http"
	"reflect"
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

func TestBackupInventoryDecodesIndependentSnapshotRows(t *testing.T) {
	body := `{
		"errors": null,
		"future": {"values": [true, null, {"nested": "温度 ] }"}]},
		"data"  :  [
			{"backup-type":"vm","backup-id":"101","backup-time":1795000000,
			 "comment":"first","protected":true,"owner":"fixture",
			 "verification":{"state":"ok"},"files":["index.json.blob",{"filename":"disk.img.fidx","size":1024}]},
			{"backup-type":"vm","backup-id":"101","backup-time":1794900000,
			 "files":[{"filename":"index.json.blob"}]},
			{"backup-type":"vm","backup-id":"101","backup-time":1794950000}
		],
		"total": 3
	}`
	want := []BackupSnapshot{
		{
			BackupType: "vm", BackupID: "101", BackupTime: 1795000000,
			Comment: "first", Protected: true, Owner: "fixture",
			Verification: map[string]any{"state": "ok"},
			Files:        []any{"index.json.blob", map[string]any{"filename": "disk.img.fidx", "size": float64(1024)}},
		},
		{
			BackupType: "vm", BackupID: "101", BackupTime: 1794900000,
			Files: []any{map[string]any{"filename": "index.json.blob"}},
		},
		{BackupType: "vm", BackupID: "101", BackupTime: 1794950000},
	}
	got, err := decodeBackupInventory[BackupSnapshot]([]byte(body))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot values or response order changed: got %+v, err=%v", got, err)
	}

	// A usable first row and a complete array cannot establish inventory
	// when later content makes the read incomplete or ambiguous.
	const first = `{"backup-type":"vm","backup-id":"101","backup-time":1795000000}`
	for _, tc := range []struct{ name, tail string }{
		{"late row type mismatch", `,{"backup-time":"not a number"}]}`},
		{"truncated row", `,{"backup-time":`},
		{"array trailing comma", `,]}`},
		{"duplicate data after complete array", `],"data":[]}`},
		{"case alias after complete array", `],"DATA":[]}`},
		{"duplicate metadata", `],"future":{},"future":0}`},
		{"malformed metadata", `],"future":[1,]}`},
		{"reported error after complete array", `],"errors":{"fixture":"unread"}}`},
		{"second object", `]} {"data":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"data":[` + first + tc.tail)
			got, err := decodeBackupInventory[BackupSnapshot](body)
			if got != nil || err != errInvalidBackupInventoryResponse {
				t.Fatalf("unread inventory returned %d rows; err=%v", len(got), err)
			}
		})
	}
}
