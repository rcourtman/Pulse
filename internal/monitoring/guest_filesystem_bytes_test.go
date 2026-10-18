package monitoring

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestVMFilesystemByteSummaryRejectsInvalidCounters(t *testing.T) {
	for name, bad := range map[string]proxmox.VMFileSystem{
		"overused":        {TotalBytes: 1000, UsedBytes: 1001},
		"signed-overflow": {TotalBytes: uint64(math.MaxInt64) + 1, UsedBytes: 400},
		"unsigned-limit":  {TotalBytes: math.MaxUint64, UsedBytes: math.MaxUint64},
	} {
		t.Run(name, func(t *testing.T) {
			bad.Mountpoint = "/bad"
			bad.Type = "ext4"
			bad.Disk = "bad-device"
			good := proxmox.VMFileSystem{Mountpoint: "/", Type: "ext4", Disk: "good-device", TotalBytes: 1000, UsedBytes: 400}
			summary := (&Monitor{}).summarizeVMFSInfo("fixture", proxmox.ClusterResource{}, []proxmox.VMFileSystem{bad, good})
			if summary.totalBytes != 1000 || summary.usedBytes != 400 || len(summary.individualDisks) != 1 || summary.individualDisks[0].Free != 600 || summary.individualDisks[0].Mountpoint != "/" {
				t.Fatalf("invalid counter contaminated summary or valid peer: %+v", summary)
			}
		})
	}
}

func TestVMFilesystemByteSummaryCannotOverflowSignedModel(t *testing.T) {
	for name, totals := range map[string][]uint64{
		"signed-sum":   {math.MaxInt64, 1},
		"unsigned-sum": {math.MaxInt64, math.MaxInt64, 2},
	} {
		t.Run(name, func(t *testing.T) {
			var fs []proxmox.VMFileSystem
			for i, total := range totals {
				fs = append(fs, proxmox.VMFileSystem{Mountpoint: fmt.Sprintf("/disk%d", i), Type: "ext4", Disk: fmt.Sprint(i), TotalBytes: total, UsedBytes: 0})
			}
			summary := (&Monitor{}).summarizeVMFSInfo("fixture", proxmox.ClusterResource{}, fs)
			if summary.totalBytes != 0 || summary.usedBytes != 0 || len(summary.individualDisks) != 0 {
				t.Fatalf("overflow became a partial/wrapped aggregate: %+v", summary)
			}
		})
	}
	// The exact signed boundary remains usable; a duplicate mount is counted
	// only once, and distinct capacity is not clamped into this boundary.
	fs := []proxmox.VMFileSystem{
		{Mountpoint: "/a", Type: "ext4", Disk: "a", TotalBytes: math.MaxInt64 - 1, UsedBytes: math.MaxInt64 - 2},
		{Mountpoint: "/bind", Type: "ext4", Disk: "a", TotalBytes: math.MaxInt64 - 1, UsedBytes: math.MaxInt64 - 2},
		{Mountpoint: "/b", Type: "ext4", Disk: "b", TotalBytes: 1, UsedBytes: 1},
	}
	summary := (&Monitor{}).summarizeVMFSInfo("fixture", proxmox.ClusterResource{}, fs)
	if summary.totalBytes != math.MaxInt64 || summary.usedBytes != math.MaxInt64-1 || len(summary.individualDisks) != 3 {
		t.Fatalf("valid signed boundary/dedup changed: %+v", summary)
	}
}

func TestVMFilesystemInvalidWireReadingDoesNotBecomeHealthyZero(t *testing.T) {
	for name, bad := range map[string]string{
		"missing-used":       `[{"mountpoint":"/","type":"ext4","total-bytes":1000}]`,
		"negative-used":      `[{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":-1}]`,
		"overused":           `[{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":1001}]`,
		"aggregate-overflow": `[{"mountpoint":"/a","type":"ext4","total-bytes":9223372036854775807,"used-bytes":1},{"mountpoint":"/b","type":"ext4","total-bytes":1,"used-bytes":0}]`,
	} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				if !strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo") {
					t.Errorf("unexpected request: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if calls.Add(1) == 1 {
					fmt.Fprintf(w, `{"data":{"result":%s}}`, bad)
					return
				}
				fmt.Fprint(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":0}]}}`)
			}))
			defer server.Close()
			c, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"})
			if err != nil {
				t.Fatal(err)
			}
			m := &Monitor{}
			res := proxmox.ClusterResource{Node: "node", VMID: 105, MaxDisk: 1000}
			total, used, free, usage, disks, fromAgent, reason := m.updateVMDisksFromGuestAgentFSInfo(context.Background(), "fixture", res, c, 1000, 0, 0)
			if total != 1000 || used != 0 || free != 1000 || usage != -1 || len(disks) != 0 || fromAgent || reason == "" {
				t.Fatalf("invalid wire reading was published as usage: %d/%d/%d %v %+v %v %q", total, used, free, usage, disks, fromAgent, reason)
			}
			if name == "aggregate-overflow" && reason != "agent-error" {
				t.Fatalf("invalid aggregate was misreported as special mounts: %q", reason)
			}
			// A later ordinary poll with an explicit zero is genuinely current usage.
			total, used, free, usage, disks, fromAgent, reason = m.updateVMDisksFromGuestAgentFSInfo(context.Background(), "fixture", res, c, 1000, 0, 0)
			if total != 1000 || used != 0 || free != 1000 || usage != 0 || len(disks) != 1 || !fromAgent || reason != "" || calls.Load() != 2 {
				t.Fatalf("ordinary corrected reading did not recover: %d/%d/%d %v %+v %v %q calls=%d", total, used, free, usage, disks, fromAgent, reason, calls.Load())
			}
		})
	}
}
