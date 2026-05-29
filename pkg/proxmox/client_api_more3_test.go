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
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"result":[
				{"disk":[{"dev":"\\\\.\\PhysicalDrive0","serial":"QM00005"}],"mountpoint":"System Reserved","name":"\\\\?\\Volume{cd7c4fae-8d0e-4f80-846b-f7121e12c38d}\\","type":"FAT32"},
				{"disk":[{"dev":"\\\\.\\PhysicalDrive2","serial":"QM00009"}],"mountpoint":"F:\\","name":"\\\\?\\Volume{80ab7d1d-af7b-48fa-86d7-d37aef6ed2ad}\\","total-bytes":3298516004864,"type":"NTFS","used-bytes":2671768784896},
				{"disk":[{"dev":"\\\\.\\PhysicalDrive1","serial":"QM00007"}],"mountpoint":"E:\\","name":"\\\\?\\Volume{5743c199-8613-4953-94a2-574d75a27bfc}\\","total-bytes":9565733122048,"type":"NTFS","used-bytes":8126376873984},
				{"disk":[{"dev":"\\\\.\\PhysicalDrive0","serial":"QM00005"}],"mountpoint":"C:\\","name":"\\\\?\\Volume{c65410ae-abf1-4829-8d8e-81d4b7581949}\\","total-bytes":267789529088,"type":"NTFS","used-bytes":195096502272},
				{"disk":[{"dev":"\\\\.\\PhysicalDrive0","serial":"QM00005"}],"mountpoint":"System Reserved","name":"\\\\?\\Volume{f1b9529f-ff6e-434b-a53a-81687141c733}\\","type":"NTFS"}
			]}}`))
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
		t.Fatalf("expected 5 filesystems, got %d", len(filesystems))
	}
	if filesystems[1].Mountpoint != "F:\\" || filesystems[1].Disk != "\\\\.\\PhysicalDrive2" {
		t.Fatalf("expected F: volume with physical disk metadata, got %+v", filesystems[1])
	}
	if filesystems[2].Mountpoint != "E:\\" || filesystems[2].Disk != "\\\\.\\PhysicalDrive1" {
		t.Fatalf("expected E: volume with physical disk metadata, got %+v", filesystems[2])
	}
	if filesystems[3].Mountpoint != "C:\\" || filesystems[3].Disk != "\\\\.\\PhysicalDrive0" {
		t.Fatalf("expected C: volume with physical disk metadata, got %+v", filesystems[3])
	}
}

func TestClientVMFSInfoSkipsMalformedEntries(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": []interface{}{
						map[string]interface{}{
							"name":        "root",
							"type":        "ext4",
							"mountpoint":  "/",
							"total-bytes": 100,
							"used-bytes":  10,
						},
						map[string]interface{}{
							"name":        "broken",
							"type":        "ext4",
							"mountpoint":  "/broken",
							"total-bytes": true,
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
	if len(filesystems) != 1 {
		t.Fatalf("expected malformed entry to be skipped, got %+v", filesystems)
	}
	if filesystems[0].Disk != "root-filesystem" {
		t.Fatalf("expected valid filesystem to survive malformed peer entry, got %+v", filesystems[0])
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

func TestClientVMFSInfoObjectFilesystemResult(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": map[string]interface{}{
						"name":        "root",
						"type":        "ext4",
						"mountpoint":  "/",
						"total-bytes": 512,
						"used-bytes":  256,
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
		t.Fatalf("expected single filesystem, got %+v", filesystems)
	}
	if filesystems[0].Disk != "root-filesystem" {
		t.Fatalf("expected synthesized root disk identifier, got %+v", filesystems[0])
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
