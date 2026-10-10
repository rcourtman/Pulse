package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A malformed row is not an absent volume. Keep the valid peers, but make the
// completeness failure visible through both ordinary client paths. This is a
// completed synthetic QGA reply, not a native freeze/recovery experiment.
func TestVMFilesystemIncompleteInventoryKeepsPeersAndSingleAttempt(t *testing.T) {
	for _, path := range []string{"single", "cluster"} {
		for name, bad := range map[string]string{
			"missing-used":   `{"mountpoint":"/data","type":"ext4","total-bytes":9000}`,
			"overused":       `{"mountpoint":"/data","type":"ext4","total-bytes":9000,"used-bytes":9001}`,
			"duplicate":      `{"mountpoint":"/data","type":"ext4","total-bytes":9000,"used-bytes":8800,"used-bytes":0}`,
			"null-record":    `null`,
			"scalar-record":  `42`,
			"empty-record":   `{}`,
			"unknown-record": `{"unknown":"value"}`,
		} {
			t.Run(path+"/"+name, func(t *testing.T) {
				var commands, configs atomic.Int32
				var corrected atomic.Bool
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case strings.HasSuffix(r.URL.Path, "/nodes"):
						fmt.Fprint(w, `{"data":[]}`)
					case strings.HasSuffix(r.URL.Path, "/config"):
						configs.Add(1)
						fmt.Fprint(w, `{"data":{}}`)
					case strings.Contains(r.URL.Path, "/agent/"):
						commands.Add(1)
						if strings.HasSuffix(r.URL.Path, "/get-fsinfo") {
							peer := `{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":0}`
							if corrected.Load() {
								fmt.Fprintf(w, `{"data":{"result":[%s]}}`, peer)
							} else {
								fmt.Fprintf(w, `{"data":{"result":[%s,%s]}}`, bad, peer)
							}
						} else {
							backupAgentPayload(w, r)
						}
					default:
						t.Errorf("unexpected request: %s", r.URL.Path)
						http.NotFound(w, r)
					}
				})
				a, b := httptest.NewServer(handler), httptest.NewServer(handler)
				defer a.Close()
				defer b.Close()
				var client interface {
					GetVMFSInfo(context.Context, string, int) ([]VMFileSystem, error)
					GetVMAgentInfo(context.Context, string, int) (map[string]interface{}, error)
				}
				if path == "cluster" {
					client = NewClusterClient("partial-filesystems", ClientConfig{Host: a.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}, []string{a.URL, b.URL}, nil)
				} else {
					client = backupTestClient(t, a.URL)
				}
				peers, err := client.GetVMFSInfo(context.Background(), "node", 105)
				if err == nil || errors.Is(err, ErrGuestAgentDeferred) || len(peers) != 1 || peers[0].TotalBytes != 1000 || peers[0].UsedBytes != 0 || peers[0].Disk != "root-filesystem" {
					t.Fatalf("partial inventory lost failure or valid peer: %+v / %v", peers, err)
				}
				if commands.Load() != 1 || configs.Load() != 2 {
					t.Fatalf("incomplete payload repeated/bypassed guest admission: commands=%d config=%d", commands.Load(), configs.Load())
				}
				// Parsing failure is not uncertain command completion. Another
				// ordinary metadata read remains eligible, not a forced probe.
				if _, err := client.GetVMAgentInfo(context.Background(), "node", 105); err != nil {
					t.Fatalf("completed malformed reading started a command cooldown: %v", err)
				}
				corrected.Store(true)
				peers, err = client.GetVMFSInfo(context.Background(), "migrated", 105)
				if err != nil || len(peers) != 1 || peers[0].UsedBytes != 0 || commands.Load() != 3 || configs.Load() != 6 {
					t.Fatalf("complete ordinary removal/zero did not recover once: %+v / %v / commands=%d config=%d", peers, err, commands.Load(), configs.Load())
				}
			})
		}
	}
}
