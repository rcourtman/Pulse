package proxmox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testReadGuestStatus(c *Client, kind string) (string, time.Time, bool, error) {
	if kind == "qemu" {
		value, err := c.GetVMStatus(context.Background(), "member", 105)
		if value == nil {
			return "", time.Time{}, true, err
		}
		return value.Status, value.ObservedAt, false, err
	}
	value, err := c.GetContainerStatus(context.Background(), "member", 105)
	if value == nil {
		return "", time.Time{}, true, err
	}
	return value.Status, value.ObservedAt, false, err
}

// Real HTTP reads: no malformed success may become a fresh guest observation.
func TestGuestStatusResponseAdmission(t *testing.T) {
	const valid = `{"data":{"status":"running","uptime":12,"netin":10}}`
	for _, kind := range []string{"qemu", "lxc"} {
		for _, tc := range []struct {
			name, body string
			truncated  bool
		}{
			{"missing-data", `{}`, false},
			{"null-data", `{"data":null}`, false},
			{"null-envelope", `null`, false},
			{"array-data", `{"data":[]}`, false},
			{"empty-data", `{"data":{}}`, false},
			{"missing-state", `{"data":{"uptime":12}}`, false},
			{"null-state", `{"data":{"status":null}}`, false},
			{"empty-state", `{"data":{"status":" "}}`, false},
			{"wrong-state-type", `{"data":{"status":true}}`, false},
			{"case-data", `{"Data":{"status":"running"}}`, false},
			{"duplicate-data", `{"data":{"status":"stopped"},"data":{"status":"running"}}`, false},
			{"escaped-duplicate-data", `{"data":{"status":"stopped"},"\u0064ata":{"status":"running"}}`, false},
			{"case-overwrite", `{"data":{"status":"stopped"},"Data":{"status":"running"}}`, false},
			{"duplicate-state", `{"data":{"status":"stopped","status":"running"}}`, false},
			{"escaped-duplicate-state", `{"data":{"status":"stopped","\u0073tatus":"running"}}`, false},
			{"case-state", `{"data":{"status":"stopped","Status":"running"}}`, false},
			{"case-lock", `{"data":{"status":"running","Lock":"backup"}}`, false},
			{"duplicate-counter", `{"data":{"status":"running","netin":10,"netin":0}}`, false},
			{"case-counter", `{"data":{"status":"running","Netin":10}}`, false},
			{"case-memory", `{"data":{"status":"running","Meminfo":{"available":0}}}`, false},
			{"conflicting-error", `{"data":{"status":"running"},"errors":{"message":"fixture-private-detail"}}`, false},
			{"case-error", `{"data":{"status":"running"},"Errors":null}`, false},
			{"trailing-value", valid + ` {"data":{"status":"stopped"}}`, false},
			{"trailing-garbage", valid + `fixture-private-detail`, false},
			{"truncated-after-prefix", valid, true},
			{"oversized-after-prefix", valid + strings.Repeat(" ", int(maxResponseBodyBytes)), false},
			{"invalid-utf8", `{"data":{"status":"running"},"metadata":"` + string([]byte{0xff}) + `"}`, false},
			{"foreign-guest", `{"data":{"status":"running","vmid":106}}`, false},
			{"null-guest", `{"data":{"status":"running","vmid":null}}`, false},
			{"zero-guest", `{"data":{"status":"running","vmid":0}}`, false},
			{"fractional-guest", `{"data":{"status":"running","vmid":105.5}}`, false},
			{"fractional-string-guest", `{"data":{"status":"running","vmid":"105.5"}}`, false},
			{"duplicate-guest", `{"data":{"status":"running","vmid":106,"vmid":105}}`, false},
			{"invalid-value", `{"data":{"status":"running","uptime":"fixture-private-detail"}}`, false},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.URL.Path != "/api2/json/nodes/member/"+kind+"/105/status/current" {
						t.Errorf("unexpected path: %s", r.URL.Path)
					}
					if tc.truncated {
						w.Header().Set("Content-Length", fmt.Sprint(len(tc.body)+100))
					}
					fmt.Fprint(w, tc.body)
				}))
				defer server.Close()
				_, at, nilResult, err := testReadGuestStatus(backupTestClient(t, server.URL), kind)
				if err == nil || !nilResult || !at.IsZero() {
					t.Errorf("unverified guest observation admitted: nil=%t time=%s err=%v", nilResult, at, err)
				}
				if err != nil && strings.Contains(err.Error(), "fixture-private-detail") {
					t.Error("response material leaked into diagnostic")
				}
				if requests.Load() != 1 {
					t.Errorf("invalid success replayed: %d", requests.Load())
				}
			})
		}
	}
}

func TestGuestStatusAcceptedCompatibility(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, state := range []string{"running", "stopped", "paused", "future-provider-state"} {
			for _, identity := range []string{"", `,"vmid":105`, `,"vmid":"105"`} {
				t.Run(kind+"/"+state+identity, func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						fmt.Fprintf(w, `{"data":{"status":%q%s,"provider-metadata":{"revision":1}},"errors":null,"total":1}`, state, identity)
					}))
					defer server.Close()
					before := time.Now()
					got, at, nilResult, err := testReadGuestStatus(backupTestClient(t, server.URL), kind)
					if err != nil || nilResult || got != state || at.Before(before) || at.After(time.Now()) {
						t.Fatalf("compatible state refused or incorrectly timed: %q %s nil=%t err=%v", got, at, nilResult, err)
					}
				})
			}
		}
	}
	t.Run("qemu-metrics", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"cpus":2,"mem":512,"maxmem":1024,"balloon":768,"balloon_min":256,"freemem":128,"meminfo":{"total":1024,"available":0},"ballooninfo":{"total_mem":1024,"free_mem":128},"disk":50,"maxdisk":100,"diskread":0,"diskwrite":null,"netin":32,"uptime":12,"agent":1,"lock":"backup"}}`)
		}))
		defer server.Close()
		got, err := backupTestClient(t, server.URL).GetVMStatus(context.Background(), "member", 105)
		if err != nil {
			t.Fatal(err)
		}
		if got.CPU != .25 || got.CPUs != 2 || got.Mem != 512 || got.MaxMem != 1024 || got.Balloon != 768 || got.BalloonMin != 256 || got.FreeMem != 128 || got.MemInfo == nil || !got.MemInfo.HasAvailable() || got.MemInfo.Available != 0 || got.BalloonInfo == nil || got.BalloonInfo.TotalMem != 1024 || got.Disk != 50 || got.MaxDisk != 100 || got.NetIn != 32 || got.Uptime != 12 || !got.Agent.IsEnabled() || got.Lock != "backup" {
			t.Fatalf("QEMU fields changed: %+v", got)
		}
		if got.IOCounters != (IOCounterPresence{Explicit: true, DiskRead: true, NetworkIn: true}) {
			t.Fatalf("missing/null/zero counter semantics changed: %+v", got.IOCounters)
		}
	})
	t.Run("lxc-metrics", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"status":"running","vmid":"105","cpus":"2","cpu":0.25,"mem":512,"maxmem":1024,"swap":0,"maxswap":256,"disk":50,"maxdisk":100,"diskread":0,"netout":32,"uptime":12,"ip":"192.0.2.5","network":{"eth0":{"name":"eth0","ip":"192.0.2.5/24"}},"diskinfo":{"rootfs":{"total":100,"used":50}}}}`)
		}))
		defer server.Close()
		got, err := backupTestClient(t, server.URL).GetContainerStatus(context.Background(), "member", 105)
		if err != nil {
			t.Fatal(err)
		}
		if got.VMID != 105 || got.CPUs != 2 || got.CPU != .25 || got.Mem != 512 || got.MaxMem != 1024 || got.MaxSwap != 256 || got.Disk != 50 || got.MaxDisk != 100 || got.NetOut != 32 || got.Uptime != 12 || got.IP != "192.0.2.5" || len(got.Network) != 1 || got.DiskInfo["rootfs"].Used != 50 {
			t.Fatalf("LXC fields changed: %+v", got)
		}
		if got.IOCounters != (IOCounterPresence{Explicit: true, DiskRead: true, NetworkOut: true}) {
			t.Fatalf("missing/zero counter semantics changed: %+v", got.IOCounters)
		}
	})
}

type guestStatusCloseFailure struct {
	io.Reader
	closed bool
}

func (b *guestStatusCloseFailure) Close() error {
	b.closed = true
	return errors.New("fixture close failed")
}

func TestGuestStatusRequiresCompletedResponse(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, code := range []int{http.StatusNoContent, http.StatusPartialContent} {
			resp := &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(`{"data":{"status":"running"}}`))}
			var err error
			if kind == "qemu" {
				_, err = readGuestStatus[VMStatus](resp, 105)
			} else {
				_, err = readGuestStatus[Container](resp, 105)
			}
			if err == nil {
				t.Errorf("status %d established %s observation", code, kind)
			}
		}
		body := &guestStatusCloseFailure{Reader: strings.NewReader(`{"data":{"status":"running"}}`)}
		resp := &http.Response{StatusCode: http.StatusOK, Body: body}
		var err error
		if kind == "qemu" {
			_, err = readGuestStatus[VMStatus](resp, 105)
		} else {
			_, err = readGuestStatus[Container](resp, 105)
		}
		if err == nil || !body.closed {
			t.Errorf("close failure established %s observation: closed=%t err=%v", kind, body.closed, err)
		}
	}
}
