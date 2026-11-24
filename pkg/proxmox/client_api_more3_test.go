package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestClientVMFSInfoParsing(t *testing.T) {
	isolateGuestAgentCapacity(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
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

func TestClientVMFSInfoParsingCentOSXFSResult(t *testing.T) {
	isolateGuestAgentCapacity(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		switch r.URL.Path {
		case "/api2/json/nodes/pve1/qemu/1376/agent/get-fsinfo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"result":[
				{"name":"/dev/mapper/cs-root","type":"xfs","mountpoint":"/","total-bytes":"75151441920","used-bytes":"32212254720","disk":{"dev":"/dev/dm-0","bus-type":"scsi"}},
				{"name":"/dev/sda2","type":"xfs","mountpoint":"/boot","total-bytes":1072693248,"used-bytes":268435456,"disk":[{"dev":"/dev/sda2","bus-type":"scsi"}]}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	})

	filesystems, err := client.GetVMFSInfo(context.Background(), "pve1", 1376)
	if err != nil {
		t.Fatalf("GetVMFSInfo error: %v", err)
	}
	if len(filesystems) != 2 {
		t.Fatalf("expected CentOS root and boot filesystems, got %+v", filesystems)
	}
	if root := filesystems[0]; root.Type != "xfs" || root.Mountpoint != "/" ||
		root.TotalBytes != 75151441920 || root.UsedBytes != 32212254720 || root.Disk != "/dev/dm-0" {
		t.Fatalf("unexpected CentOS root filesystem: %+v", root)
	}
	if boot := filesystems[1]; boot.Type != "xfs" || boot.Mountpoint != "/boot" ||
		boot.TotalBytes != 1072693248 || boot.UsedBytes != 268435456 || boot.Disk != "/dev/sda2" {
		t.Fatalf("unexpected CentOS boot filesystem: %+v", boot)
	}
}

func TestClientVMFSInfoParsingPrivilegedCapacityFallback(t *testing.T) {
	isolateGuestAgentCapacity(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
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
	isolateGuestAgentCapacity(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
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
	isolateGuestAgentCapacity(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
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
	isolateGuestAgentCapacity(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
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
	if err == nil {
		t.Fatal("malformed peer must prevent a complete filesystem inventory")
	}
	if len(filesystems) != 1 {
		t.Fatalf("expected malformed entry to be skipped, got %+v", filesystems)
	}
	if filesystems[0].Disk != "root-filesystem" {
		t.Fatalf("expected valid filesystem to survive malformed peer entry, got %+v", filesystems[0])
	}
}

func TestClientVMFSInfoObjectResult(t *testing.T) {
	isolateGuestAgentCapacity(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
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
	if GuestAgentDeferredReason(err) != "agent-completion-unverified" || len(filesystems) != 0 {
		t.Fatalf("HTTP 200 error payload became a current empty inventory: %+v / %v", filesystems, err)
	}
}

func TestClientVMFSInfoObjectFilesystemResult(t *testing.T) {
	isolateGuestAgentCapacity(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
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

func TestClientGuestAgentListsAreBounded(t *testing.T) {
	isolateGuestAgentCapacity(t)
	list := func(entry string, count int) string {
		return "[" + strings.TrimSuffix(strings.Repeat(entry+",", count), ",") + "]"
	}
	filesystem := `{"name":"data","type":"ext4","mountpoint":"/data","total-bytes":100,"used-bytes":10}`
	replies := map[string]string{
		"/api2/json/nodes/node1/qemu/100/agent/get-fsinfo":             list(filesystem, maxGuestAgentListEntries),
		"/api2/json/nodes/node1/qemu/101/agent/get-fsinfo":             list(filesystem, maxGuestAgentListEntries+1),
		"/api2/json/nodes/node1/qemu/102/agent/get-fsinfo":             list("0", maxGuestAgentListEntries+1),
		"/api2/json/nodes/node1/qemu/100/agent/network-get-interfaces": list(`{"name":"eth0"}`, maxGuestAgentListEntries),
		"/api2/json/nodes/node1/qemu/101/agent/network-get-interfaces": list(`{"name":"eth0"}`, maxGuestAgentListEntries+1),
		"/api2/json/nodes/node1/qemu/102/agent/network-get-interfaces": `[{"name":"eth0","ip-addresses":` + list(`{"ip-address":"10.0.0.5"}`, maxGuestAgentListEntries) + `},` +
			`{"name":"eth1","ip-addresses":` + list("0", maxGuestAgentListEntries+1) + `}]`,
	}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		reply, ok := replies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"data":{"result":`+reply+`}}`)
	})

	ctx := context.Background()
	if filesystems, err := client.GetVMFSInfo(ctx, "node1", 100); err != nil || len(filesystems) != maxGuestAgentListEntries {
		t.Fatalf("filesystems at the limit = %d, %v", len(filesystems), err)
	}
	for _, vmid := range []int{101, 102} {
		if filesystems, err := client.GetVMFSInfo(ctx, "node1", vmid); !errors.Is(err, errGuestAgentListTooLong) || filesystems != nil {
			t.Fatalf("vm %d: filesystems beyond the limit = %d, %v", vmid, len(filesystems), err)
		}
	}

	if interfaces, err := client.GetVMNetworkInterfaces(ctx, "node1", 100); err != nil || len(interfaces) != maxGuestAgentListEntries {
		t.Fatalf("interfaces at the limit = %d, %v", len(interfaces), err)
	}
	if interfaces, err := client.GetVMNetworkInterfaces(ctx, "node1", 101); !errors.Is(err, errGuestAgentListTooLong) || interfaces != nil {
		t.Fatalf("interfaces beyond the limit = %d, %v", len(interfaces), err)
	}
	interfaces, err := client.GetVMNetworkInterfaces(ctx, "node1", 102)
	if err != nil {
		t.Fatalf("GetVMNetworkInterfaces error: %v", err)
	}
	if len(interfaces) != 1 || interfaces[0].Name != "eth0" || len(interfaces[0].IPAddresses) != maxGuestAgentListEntries {
		t.Fatalf("expected only the interface with a bounded address list, got %d interfaces", len(interfaces))
	}
}

func TestClientGuestAgentRepliesDecodeOnlyReadFields(t *testing.T) {
	isolateGuestAgentCapacity(t)
	// What a guest can put where Pulse reads a few scalars: each nested object
	// is five bytes of the reply and a map of its own in a generic decode.
	nested := strings.Repeat(`{"":`, 400) + "0" + strings.Repeat("}", 400)
	hostile := "[" + strings.TrimSuffix(strings.Repeat(nested+",", 32), ",") + "]"
	disk := `{"bus-type":"scsi","bus":0,"unit":0,"target":1,"serial":"QM00005","pci-controller":{"bus":0,"slot":5,"domain":0,"function":0},"dev":"/dev/sda2","extra":` + hostile + `}`
	replies := map[string]string{
		"/api2/json/nodes/node1/qemu/100/agent/get-fsinfo": `[` +
			`{"name":"sda2","type":"xfs","mountpoint":"/boot","total-bytes":100,"used-bytes":10,"disk":[` + disk + `,` + nested + `]},` +
			`{"name":"sdb1","type":"xfs","mountpoint":"/srv","total-bytes":100,"used-bytes":10,"disk":{"bus-type":"scsi","target":2,"dev":` + hostile + `,"serial":null}},` +
			`{"name":"dm-0","type":"xfs","mountpoint":"/","total-bytes":100,"used-bytes":10,"disk":` + hostile + `}]`,
		"/api2/json/nodes/node1/qemu/101/agent/get-fsinfo": `{"name":"sda2","type":"xfs","mountpoint":"/boot","total-bytes":100,"used-bytes":10,"disk":[` + disk + `],"extra":` + hostile + `}`,
		"/api2/json/nodes/node1/qemu/100/agent/network-get-interfaces": `[` +
			`{"name":"eth0","hardware-address":"aa:bb:cc:dd:ee:ff","statistics":{"rx-bytes":1024,"tx-bytes":"512","rx-packets":7,"tx-errs":` + hostile + `},` +
			`"ip-addresses":[{"ip-address":"10.0.0.5","ip-address-type":"ipv4","prefix":24,"extra":` + hostile + `},` + nested + `]},` +
			`{"name":` + hostile + `,"hardware-address":"aa:bb:cc:dd:ee:00","has-ipv4-synth-gateway":` + hostile + `,"statistics":` + hostile + `,"ip-addresses":{"ip-address":` + hostile + `}}]`,
		"/api2/json/nodes/node1/qemu/101/agent/network-get-interfaces": `{"name":"eth0","statistics":{"rx-bytes":1024,"tx-bytes":512},"extra":` + hostile + `}`,
		"/api2/json/nodes/node1/qemu/100/agent/get-osinfo":             `{"id":"debian","name":"Debian GNU/Linux","version-id":"12","kernel-release":` + hostile + `,"extra":` + hostile + `}`,
		"/api2/json/nodes/node1/qemu/100/agent/info":                   `{"version":"8.2.0","supported_commands":` + hostile + `}`,
		"/api2/json/nodes/node1/qemu/101/agent/info":                   `{"version":` + hostile + `,"qemu-ga":{"version":"8.2.1","extra":` + hostile + `}}`,
	}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		reply, ok := replies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"data":{"result":`+reply+`}}`)
	})
	ctx := context.Background()
	replyBytes := 0
	for _, reply := range replies {
		replyBytes += len(reply)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	filesystems, err := client.GetVMFSInfo(ctx, "node1", 100)
	if err != nil || len(filesystems) != 3 {
		t.Fatalf("GetVMFSInfo = %d filesystems, %v", len(filesystems), err)
	}
	firstDisk := []interface{}{map[string]interface{}{"dev": "/dev/sda2", "serial": "QM00005", "bus-type": "scsi", "target": float64(1)}}
	if !reflect.DeepEqual(filesystems[0].DiskRaw, firstDisk) || filesystems[0].Disk != "/dev/sda2" {
		t.Fatalf("expected the first disk's identifying fields only, got %q from %#v", filesystems[0].Disk, filesystems[0].DiskRaw)
	}
	objectDisk := []interface{}{map[string]interface{}{"bus-type": "scsi", "target": float64(2)}}
	if !reflect.DeepEqual(filesystems[1].DiskRaw, objectDisk) || filesystems[1].Disk != "scsi-2" {
		t.Fatalf("expected the scalar fields of a disk object only, got %q from %#v", filesystems[1].Disk, filesystems[1].DiskRaw)
	}
	if !reflect.DeepEqual(filesystems[2].DiskRaw, []interface{}{map[string]interface{}{}}) || filesystems[2].Disk != "root-filesystem" {
		t.Fatalf("expected nothing from a disk without identifying fields, got %q from %#v", filesystems[2].Disk, filesystems[2].DiskRaw)
	}
	filesystems, err = client.GetVMFSInfo(ctx, "node1", 101)
	if err != nil || len(filesystems) != 1 || !reflect.DeepEqual(filesystems[0].DiskRaw, firstDisk) || filesystems[0].Disk != "/dev/sda2" {
		t.Fatalf("object-style GetVMFSInfo = %#v, %v", filesystems, err)
	}

	interfaces, err := client.GetVMNetworkInterfaces(ctx, "node1", 100)
	if err != nil || len(interfaces) != 2 {
		t.Fatalf("GetVMNetworkInterfaces = %d interfaces, %v", len(interfaces), err)
	}
	eth0 := VMNetworkInterface{
		Name:         "eth0",
		HardwareAddr: "aa:bb:cc:dd:ee:ff",
		IPAddresses:  []VMIPAddress{{Address: "10.0.0.5", Prefix: 24}},
		Statistics:   map[string]interface{}{"rx-bytes": float64(1024), "tx-bytes": "512"},
	}
	if !reflect.DeepEqual(interfaces[0], eth0) {
		t.Fatalf("expected addresses and byte counters only, got %#v", interfaces[0])
	}
	if !reflect.DeepEqual(interfaces[1], VMNetworkInterface{HardwareAddr: "aa:bb:cc:dd:ee:00"}) {
		t.Fatalf("expected nothing from fields that are not scalars, got %#v", interfaces[1])
	}
	interfaces, err = client.GetVMNetworkInterfaces(ctx, "node1", 101)
	eth0 = VMNetworkInterface{Name: "eth0", Statistics: map[string]interface{}{"rx-bytes": float64(1024), "tx-bytes": float64(512)}}
	if err != nil || len(interfaces) != 1 || !reflect.DeepEqual(interfaces[0], eth0) {
		t.Fatalf("object-style GetVMNetworkInterfaces = %#v, %v", interfaces, err)
	}

	osInfo, err := client.GetVMAgentInfo(ctx, "node1", 100)
	debian := map[string]interface{}{"result": map[string]interface{}{"id": "debian", "name": "Debian GNU/Linux", "version-id": "12"}}
	if err != nil || !reflect.DeepEqual(osInfo, debian) {
		t.Fatalf("expected the operating system strings only, got %#v, %v", osInfo, err)
	}
	for vmid, want := range map[int]string{100: "8.2.0", 101: "8.2.1"} {
		if version, err := client.GetVMAgentVersion(ctx, "node1", vmid); err != nil || version != want {
			t.Fatalf("vm %d: agent version = %q, %v", vmid, version, err)
		}
	}

	// A generic decode of any of these values allocates for each nested
	// object, more than once per five bytes. Reading and validating a reply
	// copies it in a few large pieces instead.
	runtime.ReadMemStats(&after)
	if allocations := after.Mallocs - before.Mallocs; allocations > uint64(replyBytes)/20 {
		t.Fatalf("decoding %d reply bytes took %d allocations", replyBytes, allocations)
	}
}
