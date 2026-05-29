package proxmox

import (
	"context"
	"net/http"
	"testing"
)

func TestClientVMFSInfoParsing(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{
						{
							"name":        "root",
							"type":        "ext4",
							"mountpoint":  "/",
							"total-bytes": 100,
							"used-bytes":  10,
							"disk": []map[string]interface{}{
								{"dev": "/dev/sda"},
							},
						},
						{
							"name":        "windows",
							"type":        "ntfs",
							"mountpoint":  "C:\\Windows",
							"total-bytes": 200,
							"used-bytes":  20,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	filesystems, err := client.GetVMFSInfo(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 2 {
		t.Fatalf("expected 2 filesystems, got %d", len(filesystems))
	}
	if filesystems[0].Disk != "/dev/sda" {
		t.Fatalf("expected disk from metadata, got %q", filesystems[0].Disk)
	}
	if filesystems[1].Disk != "C:" {
		t.Fatalf("expected windows drive disk, got %q", filesystems[1].Disk)
	}
}

func TestClientVMFSInfoParsingPrivilegedCapacityFallback(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{
						{
							"name":                   "windows",
							"type":                   "ntfs",
							"mountpoint":             "C:\\Windows",
							"total-bytes":            0,
							"total-bytes-privileged": 500,
							"used-bytes":             200,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	filesystems, err := client.GetVMFSInfo(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 1 {
		t.Fatalf("expected 1 filesystem, got %d", len(filesystems))
	}
	if filesystems[0].TotalBytes != 500 {
		t.Fatalf("expected privileged total-bytes fallback, got %d", filesystems[0].TotalBytes)
	}
	if filesystems[0].Disk != "C:" {
		t.Fatalf("expected windows drive disk, got %q", filesystems[0].Disk)
	}
}

func TestClientVMFSInfoParsingWindowsNameMountpointFallback(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{
						{
							"name":        "C:\\Windows",
							"type":        "ntfs",
							"mountpoint":  "",
							"total-bytes": 500,
							"used-bytes":  200,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	filesystems, err := client.GetVMFSInfo(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 1 {
		t.Fatalf("expected 1 filesystem, got %d", len(filesystems))
	}
	if filesystems[0].Mountpoint != "C:\\Windows" {
		t.Fatalf("expected windows mountpoint fallback, got %q", filesystems[0].Mountpoint)
	}
	if filesystems[0].Disk != "C:" {
		t.Fatalf("expected windows drive disk, got %q", filesystems[0].Disk)
	}
}

func TestClientVMFSInfoParsingWindowsVolumeGUIDMountpointFallback(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{
						{
							"name":        "C:\\",
							"type":        "ntfs",
							"mountpoint":  "\\\\?\\Volume{1234-5678}\\",
							"total-bytes": 500,
							"used-bytes":  200,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	filesystems, err := client.GetVMFSInfo(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 1 {
		t.Fatalf("expected 1 filesystem, got %d", len(filesystems))
	}
	if filesystems[0].Mountpoint != "C:\\" {
		t.Fatalf("expected windows drive-letter mountpoint fallback, got %q", filesystems[0].Mountpoint)
	}
	if filesystems[0].Disk != "C:" {
		t.Fatalf("expected windows drive disk, got %q", filesystems[0].Disk)
	}
}

func TestClientVMFSInfoParsingIssue1319WindowsVolumePayload(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve7/qemu/116/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{
						{
							"disk":       []map[string]interface{}{{"dev": `\\.\PhysicalDrive0`, "serial": "QM00005"}},
							"mountpoint": "System Reserved",
							"name":       `\\?\Volume{cd7c4fae-8d0e-4f80-846b-f7121e12c38d}\`,
							"type":       "FAT32",
						},
						{
							"disk":        []map[string]interface{}{{"dev": `\\.\PhysicalDrive2`, "serial": "QM00009"}},
							"mountpoint":  `F:\`,
							"name":        `\\?\Volume{80ab7d1d-af7b-48fa-86d7-d37aef6ed2ad}\`,
							"total-bytes": 3298516004864,
							"type":        "NTFS",
							"used-bytes":  2671768784896,
						},
						{
							"disk":        []map[string]interface{}{{"dev": `\\.\PhysicalDrive1`, "serial": "QM00007"}},
							"mountpoint":  `E:\`,
							"name":        `\\?\Volume{5743c199-8613-4953-94a2-574d75a27bfc}\`,
							"total-bytes": 9565733122048,
							"type":        "NTFS",
							"used-bytes":  8126376873984,
						},
						{
							"disk":        []map[string]interface{}{{"dev": `\\.\PhysicalDrive0`, "serial": "QM00005"}},
							"mountpoint":  `C:\`,
							"name":        `\\?\Volume{c65410ae-abf1-4829-8d8e-81d4b7581949}\`,
							"total-bytes": 267789529088,
							"type":        "NTFS",
							"used-bytes":  195096502272,
						},
						{
							"disk":       []map[string]interface{}{{"dev": `\\.\PhysicalDrive0`, "serial": "QM00005"}},
							"mountpoint": "System Reserved",
							"name":       `\\?\Volume{f1b9529f-ff6e-434b-a53a-81687141c733}\`,
							"type":       "NTFS",
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	filesystems, err := client.GetVMFSInfo(ctx, "pve7", 116)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 5 {
		t.Fatalf("expected 5 filesystem records, got %d", len(filesystems))
	}

	want := map[string]string{
		`C:\`: `\\.\PhysicalDrive0`,
		`E:\`: `\\.\PhysicalDrive1`,
		`F:\`: `\\.\PhysicalDrive2`,
	}
	for _, fs := range filesystems {
		if disk, ok := want[fs.Mountpoint]; ok && fs.Disk != disk {
			t.Fatalf("filesystem %q Disk = %q, want %q", fs.Mountpoint, fs.Disk, disk)
		}
	}
}

func TestClientVMFSInfoObjectResult(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": map[string]interface{}{"error": "no info"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	filesystems, err := client.GetVMFSInfo(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 0 {
		t.Fatalf("expected empty filesystem list, got %d", len(filesystems))
	}
}

func TestClientVMFSInfoSingleFilesystemObjectResult(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": map[string]interface{}{
						"name":                   "C:\\",
						"type":                   "ntfs",
						"mountpoint":             "",
						"total-bytes":            500,
						"used-bytes":             200,
						"total-bytes-privileged": 500,
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	filesystems, err := client.GetVMFSInfo(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 1 {
		t.Fatalf("expected 1 filesystem, got %d", len(filesystems))
	}
	if filesystems[0].Mountpoint != "C:\\" {
		t.Fatalf("expected windows mountpoint fallback, got %q", filesystems[0].Mountpoint)
	}
	if filesystems[0].Disk != "C:" {
		t.Fatalf("expected windows drive disk, got %q", filesystems[0].Disk)
	}
	if filesystems[0].TotalBytes != 500 || filesystems[0].UsedBytes != 200 {
		t.Fatalf("unexpected filesystem totals: %+v", filesystems[0])
	}
}

func TestClientVMFSInfoSkipsMalformedEntries(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"result": [
						{
							"name": "windows",
							"type": "ntfs",
							"mountpoint": "C:\\Windows",
							"total-bytes": 200,
							"used-bytes": 20
						},
						{
							"name": "broken",
							"type": "ntfs",
							"mountpoint": "D:\\Data",
							"total-bytes": true,
							"used-bytes": 10
						}
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	filesystems, err := client.GetVMFSInfo(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 1 {
		t.Fatalf("expected 1 valid filesystem after skipping malformed entry, got %d", len(filesystems))
	}
	if filesystems[0].Mountpoint != "C:\\Windows" {
		t.Fatalf("expected valid filesystem to remain, got mountpoint %q", filesystems[0].Mountpoint)
	}
	if filesystems[0].Disk != "C:" {
		t.Fatalf("expected windows drive disk, got %q", filesystems[0].Disk)
	}
}

func TestClientContainerInterfacesError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/lxc/101/interfaces":
			http.Error(w, "boom", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	if _, err := client.GetContainerInterfaces(ctx, "node1", 101); err == nil {
		t.Fatal("expected error for non-200 interface response")
	}
}
