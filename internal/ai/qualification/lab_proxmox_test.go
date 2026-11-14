package qualification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// fakePVENode stands in for the lab node behind SSH. It answers the commands
// the driver sends and keeps the state they change: power states, files the
// faults write, and the disk use those files cause.
type fakePVENode struct {
	marker   string
	hostname string
	nodes    []string
	guests   map[int]*fakePVEGuest
	dump     map[string]bool
	storage  float64
	calls    []string
}

type fakePVEGuest struct {
	hostname string
	fixture  bool
	status   string
	files    map[string]bool
	baseDisk float64
}

func newFakePVENode() *fakePVENode {
	guest := func(name string, disk float64) *fakePVEGuest {
		return &fakePVEGuest{hostname: name, fixture: true, status: "running", files: map[string]bool{}, baseDisk: disk}
	}
	return &fakePVENode{
		marker:   pveLabMarkerPrefix + "pmxlab",
		hostname: "pmxlab",
		nodes:    []string{"pmxlab"},
		guests:   map[int]*fakePVEGuest{110: guest("pihole", 30), 111: guest("nextcloud", 40), 112: guest("gitea", 35)},
		dump:     map[string]bool{},
		storage:  40,
	}
}

func (n *fakePVENode) guestDisk(g *fakePVEGuest) float64 { return g.baseDisk + float64(len(g.files))*2 }
func (n *fakePVENode) storageUsed() float64              { return n.storage + float64(len(n.dump))*2 }

var (
	fakeVMIDArg  = regexp.MustCompile(`^pct (config|status|stop|start) (\d+)$`)
	fakeExecArg  = regexp.MustCompile(`^pct exec (\d+) -- (.*)$`)
	fakeLxcInfo  = regexp.MustCompile(`lxc-info -n (\d+) -pH`)
	fakeDDTarget = regexp.MustCompile(`of=(\S+) `)
	fakeQuoted   = regexp.MustCompile(`'([^']*)'`)
	// Both API paths arrive shell-quoted.
	fakeStorageStatus = regexp.MustCompile(`^pvesh get '/nodes/pmxlab/storage/[a-z]+/status' --output-format json$`)
	fakeGuestStatus   = regexp.MustCompile(`^pvesh get '/nodes/pmxlab/lxc/(\d+)/status/current' --output-format json$`)
)

func (n *fakePVENode) Run(_ context.Context, name string, args ...string) (CommandResult, error) {
	if name != "ssh" || len(args) < 6 {
		return CommandResult{}, fmt.Errorf("unexpected command %s %v", name, args)
	}
	script := args[len(args)-1]
	n.calls = append(n.calls, script)
	ok := func(out string) (CommandResult, error) { return CommandResult{Stdout: out}, nil }
	switch {
	case strings.HasPrefix(script, "cat "+pveLabMarkerPath):
		nodes, _ := json.Marshal(func() []map[string]string {
			var out []map[string]string
			for _, node := range n.nodes {
				out = append(out, map[string]string{"node": node})
			}
			return out
		}())
		return ok(n.marker + "\n---\n" + n.hostname + "\n---\n" + string(nodes) + "\n")
	case strings.HasPrefix(script, "pvesh get /storage/"):
		return ok(`{"path":"/var/lib/vz","type":"dir"}`)
	case fakeStorageStatus.MatchString(script):
		// Proxmox's own used and total, never df's Use%.
		return ok(fmt.Sprintf(`{"active":1,"total":100000,"used":%d,"avail":%d,"type":"dir"}`, int(n.storageUsed()*1000), 100000-int(n.storageUsed()*1000)))
	case fakeGuestStatus.MatchString(script):
		vmid, _ := strconv.Atoi(fakeGuestStatus.FindStringSubmatch(script)[1])
		guest := n.guests[vmid]
		return ok(fmt.Sprintf(`{"status":%q,"disk":%d,"maxdisk":100000}`, guest.status, int(n.guestDisk(guest)*1000)))
	case strings.HasPrefix(script, "mkdir -p "):
		return ok("")
	case strings.HasPrefix(script, "fallocate "):
		for _, quoted := range fakeQuoted.FindAllStringSubmatch(script, -1) {
			if strings.HasSuffix(quoted[1], ".tar.zst") || strings.HasSuffix(quoted[1], ".log") {
				n.dump[filepath.Base(quoted[1])] = true
			}
		}
		return ok("")
	case strings.HasPrefix(script, "rm -f "):
		// The leftover clear quotes the folder and globs outside the quotes.
		if strings.Contains(script, "/vzdump-lxc-*-02_00_*") {
			n.dump = map[string]bool{}
			return ok("")
		}
		for _, quoted := range fakeQuoted.FindAllStringSubmatch(script, -1) {
			delete(n.dump, filepath.Base(quoted[1]))
		}
		return ok("")
	case strings.HasPrefix(script, "pct list"):
		var lines []string
		for vmid, guest := range n.guests {
			lines = append(lines, fmt.Sprintf("%d:%s", vmid, guest.status))
		}
		sort.Strings(lines)
		var archives []string
		for file := range n.dump {
			archives = append(archives, file)
		}
		sort.Strings(archives)
		return ok(strings.Join(lines, "\n") + "\n---\n" + strings.Join(archives, "\n") + "\n")
	case fakeLxcInfo.MatchString(script):
		vmid, _ := strconv.Atoi(fakeLxcInfo.FindStringSubmatch(script)[1])
		n.guests[vmid].status = "stopped"
		return ok("")
	}
	if match := fakeVMIDArg.FindStringSubmatch(script); match != nil {
		vmid, _ := strconv.Atoi(match[2])
		guest, exists := n.guests[vmid]
		if !exists {
			return CommandResult{ExitCode: 2}, fmt.Errorf("container %d does not exist", vmid)
		}
		switch match[1] {
		case "config":
			description := "web app"
			if guest.fixture {
				description = pveFixtureMarker + " for Patrol"
			}
			return ok("arch: amd64\ndescription: " + description + "\nhostname: " + guest.hostname + "\n")
		case "status":
			return ok("status: " + guest.status + "\n")
		case "stop":
			guest.status = "stopped"
		case "start":
			guest.status = "running"
		}
		return ok("")
	}
	if match := fakeExecArg.FindStringSubmatch(script); match != nil {
		vmid, _ := strconv.Atoi(match[1])
		guest := n.guests[vmid]
		if guest.status != "running" {
			return CommandResult{ExitCode: 1}, errors.New("container is not running")
		}
		command := match[2]
		if strings.HasPrefix(command, "sh -c '") && strings.HasSuffix(command, "'") {
			command = strings.TrimSuffix(strings.TrimPrefix(command, "sh -c '"), "'")
		}
		switch {
		case strings.HasPrefix(command, "mkdir -p "):
			return ok("")
		case strings.Contains(command, "dd if=/dev/urandom"):
			guest.files[fakeDDTarget.FindStringSubmatch(command)[1]] = true
			return ok("")
		case strings.HasPrefix(command, "rm -f "):
			if strings.Contains(command, "*") {
				guest.files = map[string]bool{}
				return ok("")
			}
			for _, file := range strings.Fields(strings.TrimPrefix(command, "rm -f ")) {
				delete(guest.files, file)
			}
			return ok("")
		}
	}
	return CommandResult{}, fmt.Errorf("fake node has no answer for %q", script)
}

func loadPVEManifest(t *testing.T, id string) Manifest {
	t.Helper()
	catalog, err := LoadCatalog(filepath.Join("..", "..", "..", "tests", "qualification", "patrol", "scenarios"))
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	for _, manifest := range catalog.Manifests {
		if manifest.ID == id {
			return manifest
		}
	}
	t.Fatalf("catalog has no scenario %s", id)
	return Manifest{}
}

var pveTarget = ProxmoxTarget{SSHHost: "root@192.0.2.23", Node: "pmxlab"}

// The driver refuses any node that has not proved it is the lab: the marker
// names this node, the host name matches and it is the only node. A
// production node carries no marker, so nothing there is ever touched.
func TestProxmoxLabRefusesANodeThatIsNotTheLab(t *testing.T) {
	manifest := loadPVEManifest(t, "response.pve-guest-died")
	for name, change := range map[string]func(*fakePVENode){
		"no marker":          func(n *fakePVENode) { n.marker = "cat: /etc/pulse-qualification-lab: No such file" },
		"marker for another": func(n *fakePVENode) { n.marker = pveLabMarkerPrefix + "delly" },
		"other host name":    func(n *fakePVENode) { n.hostname = "delly" },
		"cluster member":     func(n *fakePVENode) { n.nodes = []string{"pmxlab", "delly"} },
	} {
		t.Run(name, func(t *testing.T) {
			node := newFakePVENode()
			change(node)
			lab := NewProxmoxLab(node, pveTarget)
			if _, err := lab.Prepare(context.Background(), manifest, "run-1"); err == nil {
				t.Fatal("Prepare went ahead on a node that is not the lab")
			}
			for _, call := range node.calls {
				if strings.HasPrefix(call, "pct stop") || strings.HasPrefix(call, "pct start") || strings.Contains(call, "rm -f") || strings.Contains(call, "kill") {
					t.Fatalf("a refused node received %q", call)
				}
			}
		})
	}
}

// A guest the rig did not mark as a fixture is never stopped, filled or killed.
func TestProxmoxLabRefusesAGuestThatIsNotAFixture(t *testing.T) {
	node := newFakePVENode()
	node.guests[112].fixture = false
	lab := NewProxmoxLab(node, pveTarget)
	if _, err := lab.Prepare(context.Background(), loadPVEManifest(t, "response.pve-guest-died"), "run-1"); err == nil || !strings.Contains(err.Error(), "not a qualification fixture") {
		t.Fatalf("Prepare on a non-fixture guest: %v", err)
	}
}

// Each fault reaches its oracle, reverts to its revert oracle, and the lab is
// left as it was found, with cleanup a no-op the second time.
func TestProxmoxLabFaultsApplyRevertAndCleanUp(t *testing.T) {
	for _, id := range []string{"response.pve-guest-stopped-by-person", "response.pve-guest-died", "response.pve-guest-disk-one-folder", "response.pve-storage-old-backups"} {
		t.Run(id, func(t *testing.T) {
			manifest := loadPVEManifest(t, id)
			node := newFakePVENode()
			lab := NewProxmoxLab(node, pveTarget)
			ctx := context.Background()
			prepared, err := lab.Prepare(ctx, manifest, "run-1")
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			if prepared.PVENode != "pmxlab" {
				t.Fatalf("prepared node = %q", prepared.PVENode)
			}
			fault := manifest.Faults[0]
			if err := lab.ApplyFault(ctx, manifest, prepared, fault); err != nil {
				t.Fatalf("ApplyFault: %v", err)
			}
			for i := range fault.Oracle {
				fault.Oracle[i].Timeout = "1ms"
			}
			if _, err := lab.Observe(ctx, manifest, prepared, fault.Oracle); err != nil {
				t.Fatalf("oracle: %v", err)
			}
			if err := lab.RevertFault(ctx, manifest, prepared, fault); err != nil {
				t.Fatalf("RevertFault: %v", err)
			}
			for i := range fault.RevertOracle {
				fault.RevertOracle[i].Timeout = "1ms"
			}
			if _, err := lab.Observe(ctx, manifest, prepared, fault.RevertOracle); err != nil {
				t.Fatalf("revert oracle: %v", err)
			}
			result := lab.Cleanup(ctx, manifest, prepared)
			if !result.Passed || !result.SecondCleanupNoop || !result.InventoryUnchanged {
				t.Fatalf("cleanup = %+v", result)
			}
		})
	}
}

// A run that stops half way leaves its fill behind; the next Prepare removes
// it, and Cleanup removes what a run recorded even if it never reverted.
func TestProxmoxLabNeverLeavesAFillBehind(t *testing.T) {
	ctx := context.Background()
	node := newFakePVENode()
	manifest := loadPVEManifest(t, "response.pve-guest-disk-one-folder")
	lab := NewProxmoxLab(node, pveTarget)
	prepared, err := lab.Prepare(ctx, manifest, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := lab.ApplyFault(ctx, manifest, prepared, manifest.Faults[0]); err != nil {
		t.Fatal(err)
	}
	if len(node.guests[111].files) == 0 || len(prepared.PVEFaultFiles["guest:111"]) != len(node.guests[111].files) {
		t.Fatalf("the fill wrote %d files and recorded %d", len(node.guests[111].files), len(prepared.PVEFaultFiles["guest:111"]))
	}
	for file := range node.guests[111].files {
		if !strings.HasPrefix(file, pveGuestFillDir+"/") {
			t.Fatalf("the fill wrote outside its folder: %s", file)
		}
	}
	if result := lab.Cleanup(ctx, manifest, prepared); !result.Passed || len(node.guests[111].files) != 0 {
		t.Fatalf("cleanup left %d files: %+v", len(node.guests[111].files), result)
	}

	node.guests[111].files[pveGuestFillFile(3)] = true
	node.dump["vzdump-lxc-111-2026_10_01-02_00_03.tar.zst"] = true
	if _, err := NewProxmoxLab(node, pveTarget).Prepare(ctx, manifest, "run-2"); err != nil {
		t.Fatal(err)
	}
	if len(node.guests[111].files) != 0 || len(node.dump) != 0 {
		t.Fatalf("Prepare kept leftovers: %d guest files, %d archives", len(node.guests[111].files), len(node.dump))
	}
}

// The storage fill writes a month of nightly archives of the named guest,
// dated in the past, each with a log, and nothing else.
func TestProxmoxLabStorageFillWritesOldBackupsOfOneGuest(t *testing.T) {
	ctx := context.Background()
	node := newFakePVENode()
	manifest := loadPVEManifest(t, "response.pve-storage-old-backups")
	lab := NewProxmoxLab(node, pveTarget)
	prepared, err := lab.Prepare(ctx, manifest, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := lab.ApplyFault(ctx, manifest, prepared, manifest.Faults[0]); err != nil {
		t.Fatal(err)
	}
	archive := regexp.MustCompile(`^vzdump-lxc-111-\d{4}_\d{2}_\d{2}-02_00_\d{2}\.(tar\.zst|log)$`)
	if len(node.dump) < 4 {
		t.Fatalf("the fill wrote %d files", len(node.dump))
	}
	for file := range node.dump {
		if !archive.MatchString(file) {
			t.Fatalf("unexpected file %s", file)
		}
	}
}

// The catalog's Proxmox scenarios load, and the manifest keeps each driver to
// its own resources.
type cannedPVERunner string

func (c cannedPVERunner) Run(context.Context, string, ...string) (CommandResult, error) {
	return CommandResult{Stdout: string(c)}, nil
}

// The figures the lab node gave on 14 Nov 2026, when df showed the storage at
// 91% and Pulse saw 86%. The probe must agree with Proxmox and Pulse.
func TestProxmoxUsageIsProxmoxsOwnFigure(t *testing.T) {
	lab := NewProxmoxLab(cannedPVERunner(`{"active":1,"avail":3212107776,"total":33603391488,"type":"dir","used":28974702592}`), pveTarget)
	percent, err := lab.storageUsedPercent(context.Background(), "local")
	if err != nil {
		t.Fatal(err)
	}
	if percent < 86.2 || percent > 86.3 {
		t.Fatalf("storage use = %.2f%%, want Proxmox's 86.2%% (used over total), not df's 91%%", percent)
	}
	if _, err := NewProxmoxLab(cannedPVERunner(`{"status":"stopped"}`), pveTarget).guestDiskUsedPercent(context.Background(), 111); err == nil {
		t.Fatal("a status with no disk figures must fail the probe, not read as 0%")
	}
}

func TestProxmoxManifestRules(t *testing.T) {
	catalog, err := LoadCatalog(filepath.Join("..", "..", "..", "tests", "qualification", "patrol", "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(FaultSetManifests(catalog, "pve-v1")); got != 4 {
		t.Fatalf("fault set pve-v1 has %d scenarios, want 4", got)
	}
	base := loadPVEManifest(t, "response.pve-storage-old-backups")
	for name, change := range map[string]func(*Manifest){
		"container on proxmox": func(m *Manifest) {
			m.Resources = append(m.Resources, ResourceSpec{Alias: "web", Kind: resourceKindContainer, Name: "web", Image: "busybox"})
		},
		"pve guest on docker": func(m *Manifest) { m.Lab.Driver = "docker" },
		"storage fill without a guest": func(m *Manifest) {
			m.Faults[0].Injector.Value = ""
		},
		"storage fill naming a storage": func(m *Manifest) {
			m.Faults[0].Injector.Value = "local"
		},
		"guest without vmid": func(m *Manifest) { m.Resources[0].VMID = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			manifest := base
			manifest.Resources = append([]ResourceSpec(nil), base.Resources...)
			manifest.Faults = append([]FaultSpec(nil), base.Faults...)
			change(&manifest)
			if err := manifest.Validate(); err == nil {
				t.Fatal("the manifest was accepted")
			}
		})
	}
}

// Pulse resources bind by node, VMID and storage name, exactly once each.
func TestResolvePVELabViewBindsByNodeAndVMID(t *testing.T) {
	manifest := loadPVEManifest(t, "response.pve-guest-died")
	lab := &PreparedLab{PVENode: "pmxlab", ResourceIDs: map[string]string{"gitea": "112"}}
	identity := func(id string) *ResourceCanonicalIdentity { return &ResourceCanonicalIdentity{PrimaryID: id} }
	// The identities are the ones Pulse gave the lab node on 14 Nov 2026.
	resources := []Resource{
		{ID: "node-1", Type: "agent", Name: "pmxlab", Proxmox: &ProxmoxResource{NodeName: "pmxlab"}, Agent: &AgentResource{CommandsEnabled: true}, CanonicalIdentity: identity("node:pmxlab-pmxlab")},
		{ID: "ct-110", Type: "system-container", Name: "pihole", Proxmox: &ProxmoxResource{NodeName: "pmxlab", VMID: 110}, CanonicalIdentity: identity("system-container:pmxlab:pmxlab:110")},
		{ID: "ct-111", Type: "system-container", Name: "nextcloud", Proxmox: &ProxmoxResource{NodeName: "pmxlab", VMID: 111}, CanonicalIdentity: identity("system-container:pmxlab:pmxlab:111")},
		{ID: "ct-112", Type: "system-container", Name: "gitea", Proxmox: &ProxmoxResource{NodeName: "pmxlab", VMID: 112, RuntimeStatus: "stopped"}, CanonicalIdentity: identity("system-container:pmxlab:pmxlab:112")},
		{ID: "ct-other-112", Type: "system-container", Name: "gitea", Proxmox: &ProxmoxResource{NodeName: "delly", VMID: 112}, CanonicalIdentity: identity("system-container:delly:delly:112")},
		{ID: "st-local", Type: "storage", Name: "local", Proxmox: &ProxmoxResource{NodeName: "pmxlab"}, CanonicalIdentity: identity("storage:pmxlab-local")},
		{ID: "st-local-delly", Type: "storage", Name: "local", Proxmox: &ProxmoxResource{NodeName: "delly"}, CanonicalIdentity: identity("storage:delly-local")},
	}
	view, err := resolveLabView(manifest, lab, resources)
	if err != nil {
		t.Fatal(err)
	}
	if view.resources["gitea"].ID != "ct-112" || view.resources["local"].ID != "st-local" || view.host.ID != "node-1" {
		t.Fatalf("bound %+v", view.resources)
	}
	if err := view.validateProjection(manifest, "fault", manifest.Faults[0].Oracle); err != nil {
		t.Fatalf("Pulse's own reading of the stopped guest: %v", err)
	}
	// Alert and finding IDs as Pulse wrote them in the 14 Nov runs.
	for alertID, want := range map[string]string{
		"pmxlab:pmxlab:112": "gitea",
		"pmxlab:pmxlab:111-disk-local-111-vm-111-disk-0.raw": "nextcloud",
		"pmxlab-local":       "local",
		"ct-110":             "pihole",
		"pmxlab-local-lvm":   "",
		"pmxlab:pmxlab:1121": "",
		"delly:delly:112":    "",
		"delly-local":        "",
	} {
		if alias := view.alertAlias(manifest, lab, PulseAlert{ResourceID: alertID}); alias != want {
			t.Errorf("alert on %s bound to %q, want %q", alertID, alias, want)
		}
	}
	for findingID, want := range map[string]string{
		"pmxlab:pmxlab:111": "nextcloud",
		"pmxlab-local":      "local",
		"ct-112":            "gitea",
		"delly:delly:111":   "",
		"pmxlab-pmxlab":     "",
	} {
		if alias := view.findingAlias(manifest, Finding{ResourceID: findingID}); alias != want {
			t.Errorf("finding on %s bound to %q, want %q", findingID, alias, want)
		}
		if alias := view.aliasOfResourceID(manifest, findingID); alias != want {
			t.Errorf("resource %s named %q, want %q", findingID, alias, want)
		}
	}
	duplicate := append(append([]Resource(nil), resources...), Resource{ID: "ct-112b", Type: "system-container", Proxmox: &ProxmoxResource{NodeName: "pmxlab", VMID: 112}})
	if _, err := resolveLabView(manifest, lab, duplicate); err == nil {
		t.Fatal("two resources for one lab guest were accepted")
	}
}
